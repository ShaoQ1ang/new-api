package service

import (
	"bytes"
	"context"
	"encoding/json"
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
	walletCallbackTickInterval  = time.Second
	walletCallbackProbeInterval = 5 * time.Second
	walletCallbackLeaseDuration = 30 * time.Second
	walletCallbackSafetyWindow  = 2 * time.Second
	walletCallbackBatchSize     = 100
	walletCallbackConcurrency   = 8
	walletAmountScale           = int64(1_000_000)

	walletFailureUnavailable         = "WALLET_UNAVAILABLE"
	walletFailureInsufficientFunds   = "INSUFFICIENT_FUNDS"
	walletFailureWalletNotFound      = "WALLET_NOT_FOUND"
	walletFailureIdempotencyConflict = "IDEMPOTENCY_CONFLICT"
	walletFailureInvalidState        = "INVALID_STATE"
	walletFailureInvalidPayload      = "INVALID_PAYLOAD"
	walletFailureRejected            = "WALLET_REJECTED"
)

var (
	walletCallbackOnce    sync.Once
	walletCallbackRunning atomic.Bool
	walletCallbackClient  = &http.Client{}
	walletCallbackWake    = make(chan struct{}, 1)
	walletBreaker         walletCallbackCircuitBreaker
)

type walletCallbackCircuitBreaker struct {
	mu          sync.Mutex
	open        bool
	nextProbeAt time.Time
}

func (b *walletCallbackCircuitBreaker) scanLimit(now time.Time) (int, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.open {
		return walletCallbackBatchSize, true
	}
	if now.Before(b.nextProbeAt) {
		return 0, false
	}
	b.nextProbeAt = now.Add(walletCallbackProbeInterval)
	return 1, true
}

func (b *walletCallbackCircuitBreaker) recordUnavailable(now time.Time) {
	b.mu.Lock()
	b.open = true
	b.nextProbeAt = now.Add(walletCallbackProbeInterval)
	b.mu.Unlock()
}

func (b *walletCallbackCircuitBreaker) recordAvailable() {
	b.mu.Lock()
	wasOpen := b.open
	b.open = false
	b.nextProbeAt = time.Time{}
	b.mu.Unlock()
	if wasOpen {
		wakeWalletCallbackWorker()
	}
}

type walletCallbackConfig struct {
	Enabled          bool
	FailClosed       bool
	AutoRetryEnabled bool
	BaseURL          string
	Token            string
	Timeout          time.Duration
}

type walletUsageCallbackSession struct {
	apiRequestID string
	mu           sync.Mutex
}

type walletReserveRequest struct {
	APIPlatformUserID int                   `json:"api_platform_user_id,string"`
	APIRequestID      string                `json:"api_request_id"`
	EstimateAmount    string                `json:"estimate_amount"`
	UsageAtMS         int64                 `json:"usage_at_ms"`
	BusinessOrderNo   *string               `json:"business_order_no,omitempty"`
	Extra             *walletOperationExtra `json:"extra,omitempty"`
}

type walletAPIUsageDetail struct {
	ModelName  string `json:"model_name"`
	APIKeyName string `json:"api_key_name"`
}

type walletOperationExtra struct {
	APIUsageDetail *walletAPIUsageDetail `json:"api_usage_detail,omitempty"`
}

type walletConfirmRequest struct {
	BusinessIncluded  bool   `json:"-"`
	APIPlatformUserID int    `json:"api_platform_user_id,string"`
	APIRequestID      string `json:"api_request_id"`
	FinalAmount       string `json:"final_amount"`
	UsageDetail       string `json:"usage_detail,omitempty"`
}

type walletCancelRequest struct {
	BusinessIncluded  bool   `json:"-"`
	APIPlatformUserID int    `json:"api_platform_user_id,string"`
	APIRequestID      string `json:"api_request_id"`
}

type walletCallbackHTTPError struct {
	StatusCode int
	Code       int
	Message    string
}

func (e *walletCallbackHTTPError) Error() string {
	return fmt.Sprintf("wallet callback returned HTTP %d: %s", e.StatusCode, e.Message)
}

func walletCallbackProtectedUntil(config walletCallbackConfig) int64 {
	return time.Now().Add(config.Timeout + walletCallbackSafetyWindow).UnixMilli()
}

func loadWalletCallbackConfig() walletCallbackConfig {
	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("WALLET_CALLBACK_BASE_URL")), "/")
	timeoutMS := common.GetEnvOrDefault("WALLET_CALLBACK_TIMEOUT_MS", 3000)
	if timeoutMS <= 0 {
		timeoutMS = 3000
	}
	return walletCallbackConfig{
		Enabled:          common.GetEnvOrDefaultBool("WALLET_CALLBACK_ENABLED", baseURL != "") && baseURL != "",
		FailClosed:       common.GetEnvOrDefaultBool("WALLET_CALLBACK_FAIL_CLOSED", false),
		AutoRetryEnabled: common.GetEnvOrDefaultBool("WALLET_CALLBACK_AUTO_RETRY_ENABLED", false),
		BaseURL:          baseURL,
		Token:            strings.TrimSpace(os.Getenv("WALLET_CALLBACK_TOKEN")),
		Timeout:          time.Duration(timeoutMS) * time.Millisecond,
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
	if relayInfo == nil {
		return nil, nil
	}
	if estimatedQuota < 0 {
		return nil, fmt.Errorf("wallet estimated quota cannot be negative")
	}
	covered := relayInfo.BusinessOrderNo != ""
	if covered {
		if err := validateCoveredWalletConfig(config); err != nil {
			return nil, err
		}
	}
	if !config.Enabled || (!covered && estimatedQuota == 0) {
		return nil, nil
	}
	if strings.TrimSpace(relayInfo.OriginModelName) == "" || strings.TrimSpace(relayInfo.TokenName) == "" {
		return nil, fmt.Errorf("wallet API usage detail is incomplete")
	}

	exchangeRate := decimal.NewFromFloat(operation_setting.USDExchangeRate)
	estimatedAmount, err := quotaToWalletAmount(int64(estimatedQuota), exchangeRate)
	if err != nil {
		return nil, err
	}
	// A positive reserve is required by Wallet even for a zero-cost model.
	// BUSINESS_INCLUDED records this amount but never debits the balance.
	if covered && estimatedAmount == 0 {
		estimatedAmount = 1
	}
	usageAtMS := relayInfo.StartTime.UnixMilli()
	if relayInfo.StartTime.IsZero() {
		usageAtMS = time.Now().UnixMilli()
	}
	var businessOrderNo *string
	if relayInfo.BusinessOrderNo != "" {
		businessOrderNo = &relayInfo.BusinessOrderNo
	}
	record := &model.WalletUsageCallback{
		APIRequestID:      relayInfo.RequestId,
		APIPlatformUserID: relayInfo.UserId,
		BusinessOrderNo:   businessOrderNo,
		UsageAtMS:         usageAtMS,
		ModelName:         relayInfo.OriginModelName,
		APIKeyName:        relayInfo.TokenName,
		ReservedQuota:     int64(estimatedQuota),
		ReservedAmount:    estimatedAmount,
		ExchangeRate:      exchangeRate.StringFixed(8),
		NextRetryAtMS:     walletCallbackProtectedUntil(config),
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
	s.mu.Lock()
	defer s.mu.Unlock()
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
	config := loadWalletCallbackConfig()
	if err := model.PrepareWalletUsageConfirm(s.apiRequestID, int64(actualQuota), finalAmount, walletCallbackProtectedUntil(config)); err != nil {
		return err
	}
	record, err = model.GetWalletUsageCallback(s.apiRequestID)
	if err != nil {
		return err
	}
	return processWalletUsageCallback(context.Background(), record, config)
}

func (s *walletUsageCallbackSession) Cancel() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	config := loadWalletCallbackConfig()
	if err := model.PrepareWalletUsageCancel(s.apiRequestID, walletCallbackProtectedUntil(config)); err != nil {
		return err
	}
	record, err := model.GetWalletUsageCallback(s.apiRequestID)
	if err != nil {
		return err
	}
	return processWalletUsageCallback(context.Background(), record, config)
}

func processWalletUsageCallback(ctx context.Context, record *model.WalletUsageCallback, config walletCallbackConfig) error {
	if record == nil {
		return nil
	}
	if !config.Enabled {
		return fmt.Errorf("wallet callback is disabled; reconciliation is pending")
	}
	switch record.Status {
	case model.WalletCallbackStatusReserved,
		model.WalletCallbackStatusConfirmed,
		model.WalletCallbackStatusCancelled,
		model.WalletCallbackStatusRejected:
		return nil
	}
	if record.ReservedAtMS == 0 {
		var extra *walletOperationExtra
		if record.ModelName != "" && record.APIKeyName != "" {
			extra = &walletOperationExtra{APIUsageDetail: &walletAPIUsageDetail{
				ModelName: record.ModelName, APIKeyName: record.APIKeyName,
			}}
		}
		request := walletReserveRequest{
			APIPlatformUserID: record.APIPlatformUserID,
			APIRequestID:      record.APIRequestID,
			EstimateAmount:    strconv.FormatInt(record.ReservedAmount, 10),
			UsageAtMS:         record.UsageAtMS,
			BusinessOrderNo:   record.BusinessOrderNo,
			Extra:             extra,
		}
		if err := sendWalletCallback(ctx, config, "/api/v1/callback/wallet/api-platform/usage/reserve", request); err != nil {
			handleWalletCallbackFailure(record, config, err)
			return err
		}
		walletBreaker.recordAvailable()
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
			BusinessIncluded:  record.BusinessOrderNo != nil,
			APIPlatformUserID: record.APIPlatformUserID,
			APIRequestID:      record.APIRequestID,
			FinalAmount:       strconv.FormatInt(*record.FinalAmount, 10),
			UsageDetail:       fmt.Sprintf("newapi_quota=%d;exchange_rate=%s", *record.FinalQuota, record.ExchangeRate),
		}
		if err := sendWalletCallback(ctx, config, "/api/v1/callback/wallet/api-platform/usage/confirm", request); err != nil {
			handleWalletCallbackFailure(record, config, err)
			return err
		}
		walletBreaker.recordAvailable()
		return model.MarkWalletUsageFinalized(record.APIRequestID, model.WalletCallbackStatusConfirmed)
	case model.WalletCallbackStatusCancelPending:
		request := walletCancelRequest{APIPlatformUserID: record.APIPlatformUserID, APIRequestID: record.APIRequestID, BusinessIncluded: record.BusinessOrderNo != nil}
		if err := sendWalletCallback(ctx, config, "/api/v1/callback/wallet/api-platform/usage/cancel", request); err != nil {
			handleWalletCallbackFailure(record, config, err)
			return err
		}
		walletBreaker.recordAvailable()
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
	// Never forward the callback credential to a redirect target, including a
	// same-origin redirect. Copy the client to retain any injected TLS transport.
	client := *walletCallbackClient
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	response, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("wallet callback request failed: %w", err)
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 16*1024+1))
	if err != nil || len(responseBody) > 16*1024 {
		return fmt.Errorf("wallet callback response unreadable or oversized")
	}
	var envelope struct {
		Code    int             `json:"code"`
		Success *bool           `json:"success"`
		Data    json.RawMessage `json:"data"`
	}
	decodeErr := common.Unmarshal(responseBody, &envelope)
	if response.StatusCode != http.StatusOK || envelope.Code != 0 || (envelope.Success != nil && !*envelope.Success) {
		return &walletCallbackHTTPError{StatusCode: response.StatusCode, Code: envelope.Code, Message: "wallet callback rejected"}
	}
	if decodeErr != nil || len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return fmt.Errorf("wallet callback returned invalid success envelope")
	}
	return validateWalletCallbackResponse(envelope.Data, payload)
}

// Validate the actual Facade contract, including decimal strings and the
// covered funding source. A HTTP 200 alone must never authorize a model call.
func validateWalletCallbackResponse(data []byte, payload any) error {
	var response struct {
		FundingSource      *int    `json:"funding_source"`
		ReservedAmount     *string `json:"reserved_amount"`
		APIRequestID       string  `json:"api_request_id"`
		FinalAmount        *string `json:"final_amount"`
		BalanceDeltaAmount *string `json:"balance_delta_amount"`
		ReturnedAmount     *string `json:"returned_amount"`
	}
	if err := common.Unmarshal(data, &response); err != nil {
		return fmt.Errorf("wallet callback returned invalid data")
	}
	switch request := payload.(type) {
	case walletReserveRequest:
		if response.FundingSource == nil || response.ReservedAmount == nil || *response.ReservedAmount != request.EstimateAmount || *response.FundingSource < 1 || *response.FundingSource > 3 || ((request.BusinessOrderNo != nil) != (*response.FundingSource == 3)) {
			return fmt.Errorf("wallet reserve response did not confirm the requested funding source and amount")
		}
	case walletConfirmRequest:
		if response.APIRequestID != request.APIRequestID || response.FinalAmount == nil || *response.FinalAmount != request.FinalAmount || response.BalanceDeltaAmount == nil {
			return fmt.Errorf("wallet confirm response did not match the request")
		}
		delta, err := strconv.ParseInt(*response.BalanceDeltaAmount, 10, 64)
		if err != nil || (request.BusinessIncluded && delta != 0) {
			return fmt.Errorf("wallet confirm returned invalid balance delta")
		}
	case walletCancelRequest:
		if response.APIRequestID != request.APIRequestID || response.ReturnedAmount == nil {
			return fmt.Errorf("wallet cancel response did not match the request")
		}
		returned, err := strconv.ParseInt(*response.ReturnedAmount, 10, 64)
		if err != nil || returned < 0 || (request.BusinessIncluded && returned != 0) {
			return fmt.Errorf("wallet cancel returned invalid amount")
		}
	default:
		return fmt.Errorf("unsupported wallet callback payload")
	}
	return nil
}

func handleWalletCallbackFailure(record *model.WalletUsageCallback, config walletCallbackConfig, callbackErr error) {
	failureCode, retryable := classifyWalletCallbackFailure(callbackErr)
	if retryable {
		walletBreaker.recordUnavailable(time.Now())
		var err error
		if record.Status == model.WalletCallbackStatusReservePending && config.FailClosed {
			err = model.MarkWalletUsageCancelPendingAfterReserveFailure(record.APIRequestID, failureCode, callbackErr.Error())
		} else {
			err = model.MarkWalletUsageRetry(record.APIRequestID, record.Status, time.Now().UnixMilli(), failureCode, callbackErr.Error())
		}
		if err != nil {
			common.SysLog(fmt.Sprintf("record wallet callback retry failed (request_id=%s): %v", record.APIRequestID, err))
		}
		if config.AutoRetryEnabled {
			wakeWalletCallbackWorker()
		}
		return
	}
	walletBreaker.recordAvailable()
	if err := model.MarkWalletUsageRejected(record.APIRequestID, record.Status, failureCode, callbackErr.Error()); err != nil {
		common.SysLog(fmt.Sprintf("record wallet callback rejection failed (request_id=%s): %v", record.APIRequestID, err))
	}
}

func classifyWalletCallbackFailure(callbackErr error) (string, bool) {
	httpErr, ok := callbackErr.(*walletCallbackHTTPError)
	if !ok {
		return walletFailureUnavailable, true
	}
	if httpErr.StatusCode >= http.StatusInternalServerError || httpErr.StatusCode == http.StatusRequestTimeout || httpErr.StatusCode == http.StatusTooManyRequests {
		return walletFailureUnavailable, true
	}
	switch httpErr.Code {
	case 210008:
		return walletFailureInsufficientFunds, false
	case 210002:
		return walletFailureWalletNotFound, false
	case 210007:
		return walletFailureIdempotencyConflict, false
	case 210009, 210015:
		return walletFailureInvalidState, false
	case 210001:
		return walletFailureInvalidPayload, false
	default:
		return walletFailureRejected, false
	}
}

func wakeWalletCallbackWorker() {
	select {
	case walletCallbackWake <- struct{}{}:
	default:
	}
}

func claimWalletUsageCallback(record *model.WalletUsageCallback, now time.Time, config walletCallbackConfig) (bool, error) {
	leaseDuration := walletCallbackLeaseDuration
	if minimum := config.Timeout + walletCallbackSafetyWindow; leaseDuration < minimum {
		leaseDuration = minimum
	}
	return model.ClaimWalletUsageCallback(record.ID, record.Status, now.UnixMilli(), now.Add(leaseDuration).UnixMilli())
}

// ReconcileWalletUsageCallback performs one explicitly requested accounting
// recovery. It does not call a model or choose a new financial intent. Callers
// must enforce operator authorization; no public HTTP route exposes this helper.
func ReconcileWalletUsageCallback(ctx context.Context, apiRequestID string) error {
	record, err := model.GetWalletUsageCallback(apiRequestID)
	if err != nil {
		return err
	}
	if record.Status == model.WalletCallbackStatusConfirmed || record.Status == model.WalletCallbackStatusCancelled || record.Status == model.WalletCallbackStatusRejected {
		return nil
	}
	if record.Status != model.WalletCallbackStatusConfirmPending && record.Status != model.WalletCallbackStatusCancelPending {
		return fmt.Errorf("wallet callback has no persisted finalization intent")
	}
	config := loadWalletCallbackConfig()
	if !config.Enabled {
		return fmt.Errorf("wallet callback is disabled")
	}
	if record.BusinessOrderNo != nil {
		if err := validateCoveredWalletConfig(config); err != nil {
			return err
		}
	}
	claimed, err := claimWalletUsageCallback(record, time.Now(), config)
	if err != nil {
		return err
	}
	if !claimed {
		return fmt.Errorf("wallet callback reconciliation is in progress or protected")
	}
	return processWalletUsageCallback(ctx, record, config)
}

func logWalletCallbackFailure(record *model.WalletUsageCallback, callbackErr error) {
	if callbackErr != nil {
		logger.LogWarn(context.Background(), fmt.Sprintf("wallet callback failed (request_id=%s status=%s): %v", record.APIRequestID, record.Status, callbackErr))
	}
}

func processClaimedWalletUsageCallbacks(callbacks []*model.WalletUsageCallback, config walletCallbackConfig) {
	semaphore := make(chan struct{}, walletCallbackConcurrency)
	var waitGroup sync.WaitGroup
	for _, callback := range callbacks {
		callback := callback
		waitGroup.Add(1)
		semaphore <- struct{}{}
		gopool.Go(func() {
			defer waitGroup.Done()
			defer func() { <-semaphore }()
			logWalletCallbackFailure(callback, processWalletUsageCallback(context.Background(), callback, config))
		})
	}
	waitGroup.Wait()
}

func claimDueWalletUsageCallbacks(callbacks []*model.WalletUsageCallback, now time.Time, config walletCallbackConfig) []*model.WalletUsageCallback {
	claimed := make([]*model.WalletUsageCallback, 0, len(callbacks))
	for _, callback := range callbacks {
		won, err := claimWalletUsageCallback(callback, now, config)
		if err != nil {
			logger.LogWarn(context.Background(), fmt.Sprintf("claim wallet callback failed (request_id=%s): %v", callback.APIRequestID, err))
			continue
		}
		if won {
			claimed = append(claimed, callback)
		}
	}
	return claimed
}

func StartWalletCallbackTask() {
	walletCallbackOnce.Do(func() {
		if !common.IsMasterNode || !loadWalletCallbackConfig().AutoRetryEnabled {
			return
		}
		gopool.Go(func() {
			ticker := time.NewTicker(walletCallbackTickInterval)
			defer ticker.Stop()
			runWalletCallbackTaskOnce()
			for {
				select {
				case <-ticker.C:
				case <-walletCallbackWake:
				}
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
	if !config.Enabled || !config.AutoRetryEnabled {
		return
	}
	now := time.Now()
	limit, allowed := walletBreaker.scanLimit(now)
	if !allowed {
		return
	}
	callbacks, err := model.GetDueWalletUsageCallbacks(now.UnixMilli(), limit)
	if err != nil {
		logger.LogWarn(context.Background(), fmt.Sprintf("query due wallet callbacks failed: %v", err))
		return
	}
	claimed := claimDueWalletUsageCallbacks(callbacks, now, config)
	processClaimedWalletUsageCallbacks(claimed, config)
}
