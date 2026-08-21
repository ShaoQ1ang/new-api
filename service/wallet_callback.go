package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/bytedance/gopkg/util/gopool"
	"github.com/shopspring/decimal"
)

const (
	walletCallbackTickInterval = 10 * time.Second
	walletCallbackBatchSize    = 100
	walletAmountScale          = int64(1_000_000)
)

var (
	walletCallbackOnce    sync.Once
	walletCallbackRunning atomic.Bool
	walletCallbackClient  = &http.Client{}
)

type walletCallbackConfig struct {
	Enabled    bool
	FailClosed bool
	BaseURL    string
	Token      string
	Timeout    time.Duration
}

type walletUsageCallbackSession struct {
	apiRequestID string
}

type walletReserveRequest struct {
	UserID          int     `json:"user_id"`
	APIRequestID    string  `json:"api_request_id"`
	EstimateAmount  string  `json:"estimate_amount"`
	UsageAtMS       int64   `json:"usage_at_ms"`
	BusinessOrderNo *string `json:"business_order_no,omitempty"`
}

type walletConfirmRequest struct {
	UserID       int    `json:"user_id"`
	APIRequestID string `json:"api_request_id"`
	FinalAmount  string `json:"final_amount"`
	UsageDetail  string `json:"usage_detail,omitempty"`
}

type walletCancelRequest struct {
	UserID       int    `json:"user_id"`
	APIRequestID string `json:"api_request_id"`
}

func loadWalletCallbackConfig() walletCallbackConfig {
	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("WALLET_CALLBACK_BASE_URL")), "/")
	timeoutMS := common.GetEnvOrDefault("WALLET_CALLBACK_TIMEOUT_MS", 3000)
	if timeoutMS <= 0 {
		timeoutMS = 3000
	}
	return walletCallbackConfig{
		Enabled:    common.GetEnvOrDefaultBool("WALLET_CALLBACK_ENABLED", baseURL != "") && baseURL != "",
		FailClosed: common.GetEnvOrDefaultBool("WALLET_CALLBACK_FAIL_CLOSED", false),
		BaseURL:    baseURL,
		Token:      strings.TrimSpace(os.Getenv("WALLET_CALLBACK_TOKEN")),
		Timeout:    time.Duration(timeoutMS) * time.Millisecond,
	}
}

func quotaToWalletAmount(quota int64, exchangeRate decimal.Decimal) (int64, error) {
	if quota < 0 {
		return 0, fmt.Errorf("wallet quota cannot be negative: %d", quota)
	}
	if quota == 0 {
		return 0, nil
	}
	if exchangeRate.LessThanOrEqual(decimal.Zero) || common.QuotaPerUnit <= 0 {
		return 0, fmt.Errorf("invalid wallet exchange rate or quota unit")
	}
	amount := decimal.NewFromInt(quota).
		Mul(exchangeRate).
		Mul(decimal.NewFromInt(walletAmountScale)).
		Div(decimal.NewFromFloat(common.QuotaPerUnit)).
		Round(0)
	if !amount.IsInteger() || amount.GreaterThan(decimal.NewFromInt(int64(^uint64(0)>>1))) {
		return 0, fmt.Errorf("wallet amount overflow for quota %d", quota)
	}
	value := amount.IntPart()
	if value == 0 {
		value = 1
	}
	return value, nil
}

func newWalletUsageCallbackSession(relayInfo *relaycommon.RelayInfo, estimatedQuota int) (*walletUsageCallbackSession, error) {
	config := loadWalletCallbackConfig()
	if !config.Enabled || relayInfo == nil || estimatedQuota <= 0 {
		return nil, nil
	}

	exchangeRate := decimal.NewFromFloat(operation_setting.USDExchangeRate)
	estimatedAmount, err := quotaToWalletAmount(int64(estimatedQuota), exchangeRate)
	if err != nil {
		return nil, err
	}
	usageAtMS := relayInfo.StartTime.UnixMilli()
	if relayInfo.StartTime.IsZero() {
		usageAtMS = time.Now().UnixMilli()
	}
	record := &model.WalletUsageCallback{
		APIRequestID:   relayInfo.RequestId,
		UserID:         relayInfo.UserId,
		UsageAtMS:      usageAtMS,
		ReservedQuota:  int64(estimatedQuota),
		ReservedAmount: estimatedAmount,
		ExchangeRate:   exchangeRate.StringFixed(8),
	}
	if err := model.CreateWalletUsageCallback(record); err != nil {
		return nil, fmt.Errorf("create wallet callback record: %w", err)
	}

	session := &walletUsageCallbackSession{apiRequestID: relayInfo.RequestId}
	if err := processWalletUsageCallback(context.Background(), record, config); err != nil {
		return session, err
	}
	return session, nil
}

func (s *walletUsageCallbackSession) Confirm(actualQuota int) error {
	if s == nil {
		return nil
	}
	record, err := model.GetWalletUsageCallback(s.apiRequestID)
	if err != nil {
		return err
	}
	exchangeRate, err := decimal.NewFromString(record.ExchangeRate)
	if err != nil {
		return fmt.Errorf("parse wallet exchange rate: %w", err)
	}
	finalAmount, err := quotaToWalletAmount(int64(actualQuota), exchangeRate)
	if err != nil {
		return err
	}
	if err := model.PrepareWalletUsageConfirm(s.apiRequestID, int64(actualQuota), finalAmount); err != nil {
		return err
	}
	record, err = model.GetWalletUsageCallback(s.apiRequestID)
	if err != nil {
		return err
	}
	return processWalletUsageCallback(context.Background(), record, loadWalletCallbackConfig())
}

func (s *walletUsageCallbackSession) Cancel() error {
	if s == nil {
		return nil
	}
	if err := model.PrepareWalletUsageCancel(s.apiRequestID); err != nil {
		return err
	}
	record, err := model.GetWalletUsageCallback(s.apiRequestID)
	if err != nil {
		return err
	}
	return processWalletUsageCallback(context.Background(), record, loadWalletCallbackConfig())
}

func processWalletUsageCallback(ctx context.Context, record *model.WalletUsageCallback, config walletCallbackConfig) error {
	if record == nil || !config.Enabled {
		return nil
	}
	if record.ReservedAtMS == 0 {
		request := walletReserveRequest{
			UserID:          record.UserID,
			APIRequestID:    record.APIRequestID,
			EstimateAmount:  strconv.FormatInt(record.ReservedAmount, 10),
			UsageAtMS:       record.UsageAtMS,
			BusinessOrderNo: record.BusinessOrderNo,
		}
		if err := sendWalletCallback(ctx, config, "/internal/callbacks/new-api/usage/reserve", request); err != nil {
			recordWalletCallbackRetry(record, err)
			return err
		}
		completed := record.Status == model.WalletCallbackStatusReservePending
		if err := model.MarkWalletUsageReserved(record.APIRequestID, completed); err != nil {
			return err
		}
		record.ReservedAtMS = time.Now().UnixMilli()
		if completed {
			record.Status = model.WalletCallbackStatusReserved
		}
	}

	switch record.Status {
	case model.WalletCallbackStatusConfirmPending:
		if record.FinalAmount == nil || record.FinalQuota == nil {
			return fmt.Errorf("wallet confirm amount is missing for request %s", record.APIRequestID)
		}
		request := walletConfirmRequest{
			UserID:       record.UserID,
			APIRequestID: record.APIRequestID,
			FinalAmount:  strconv.FormatInt(*record.FinalAmount, 10),
			UsageDetail:  fmt.Sprintf("newapi_quota=%d;exchange_rate=%s", *record.FinalQuota, record.ExchangeRate),
		}
		if err := sendWalletCallback(ctx, config, "/internal/callbacks/new-api/usage/confirm", request); err != nil {
			recordWalletCallbackRetry(record, err)
			return err
		}
		return model.MarkWalletUsageFinalized(record.APIRequestID, model.WalletCallbackStatusConfirmed)
	case model.WalletCallbackStatusCancelPending:
		request := walletCancelRequest{UserID: record.UserID, APIRequestID: record.APIRequestID}
		if err := sendWalletCallback(ctx, config, "/internal/callbacks/new-api/usage/cancel", request); err != nil {
			recordWalletCallbackRetry(record, err)
			return err
		}
		return model.MarkWalletUsageFinalized(record.APIRequestID, model.WalletCallbackStatusCancelled)
	default:
		return nil
	}
}

func sendWalletCallback(ctx context.Context, config walletCallbackConfig, path string, payload any) error {
	body, err := common.Marshal(payload)
	if err != nil {
		return err
	}
	requestCtx, cancel := context.WithTimeout(ctx, config.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, config.BaseURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if config.Token != "" {
		req.Header.Set("Authorization", "Bearer "+config.Token)
	}
	response, err := walletCallbackClient.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode >= http.StatusOK && response.StatusCode < http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, response.Body)
		return nil
	}
	responseBody, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
	return fmt.Errorf("wallet callback returned HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(responseBody)))
}

func recordWalletCallbackRetry(record *model.WalletUsageCallback, callbackErr error) {
	delays := [...]time.Duration{time.Second, 5 * time.Second, 30 * time.Second, 2 * time.Minute, 10 * time.Minute}
	index := record.RetryCount
	if index >= len(delays) {
		index = len(delays) - 1
	}
	nextRetryAtMS := time.Now().Add(delays[index]).UnixMilli()
	if err := model.MarkWalletUsageRetry(record.APIRequestID, nextRetryAtMS, callbackErr.Error()); err != nil {
		common.SysLog(fmt.Sprintf("record wallet callback retry failed (request_id=%s): %v", record.APIRequestID, err))
	}
}

func StartWalletCallbackTask() {
	walletCallbackOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		gopool.Go(func() {
			ticker := time.NewTicker(walletCallbackTickInterval)
			defer ticker.Stop()
			runWalletCallbackTaskOnce()
			for range ticker.C {
				runWalletCallbackTaskOnce()
			}
		})
	})
}

func runWalletCallbackTaskOnce() {
	if !walletCallbackRunning.CompareAndSwap(false, true) {
		return
	}
	defer walletCallbackRunning.Store(false)
	config := loadWalletCallbackConfig()
	if !config.Enabled {
		return
	}
	callbacks, err := model.GetDueWalletUsageCallbacks(time.Now().UnixMilli(), walletCallbackBatchSize)
	if err != nil {
		logger.LogWarn(context.Background(), fmt.Sprintf("query due wallet callbacks failed: %v", err))
		return
	}
	for _, callback := range callbacks {
		if err := processWalletUsageCallback(context.Background(), callback, config); err != nil {
			logger.LogWarn(context.Background(), fmt.Sprintf("wallet callback failed (request_id=%s status=%s): %v", callback.APIRequestID, callback.Status, err))
		}
	}
}
