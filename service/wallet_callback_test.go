package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type walletCallbackFundingStub struct{}

// The callback fake speaks the real Facade decimal-string response contract.
func writeWalletCallbackSuccess(t *testing.T, w http.ResponseWriter, request *http.Request, decoded any) {
	t.Helper()
	var body map[string]any
	if decoded == nil {
		require.NoError(t, common.DecodeJson(request.Body, &body))
	} else {
		encoded, err := common.Marshal(decoded)
		require.NoError(t, err)
		require.NoError(t, common.Unmarshal(encoded, &body))
	}
	assert.IsType(t, "", body["api_platform_user_id"], "Facade accepts a decimal-string API platform identity, not a JSON number")
	var data map[string]any
	switch {
	case strings.HasSuffix(request.URL.Path, "/reserve"):
		funding := 1
		if body["business_order_no"] != nil {
			funding = 3
		}
		data = map[string]any{"funding_source": funding, "reserved_amount": body["estimate_amount"]}
	case strings.HasSuffix(request.URL.Path, "/confirm"):
		data = map[string]any{"api_request_id": body["api_request_id"], "final_amount": body["final_amount"], "balance_delta_amount": "0"}
	case strings.HasSuffix(request.URL.Path, "/cancel"):
		data = map[string]any{"api_request_id": body["api_request_id"], "returned_amount": "0"}
	default:
		t.Errorf("unexpected callback path %s", request.URL.Path)
		return
	}
	encoded, err := common.Marshal(map[string]any{"data": data})
	require.NoError(t, err)
	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(encoded)
	require.NoError(t, err)
}

func (walletCallbackFundingStub) Source() string       { return BillingSourceWallet }
func (walletCallbackFundingStub) PreConsume(int) error { return nil }
func (walletCallbackFundingStub) Settle(int) error     { return nil }
func (walletCallbackFundingStub) Refund() error        { return nil }

func useWalletCallbackTestDB(t *testing.T, models ...any) {
	t.Helper()
	previousDB := model.DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(models...))
	model.DB = db
	t.Cleanup(func() {
		model.DB = previousDB
		_ = sqlDB.Close()
	})
}

func TestQuotaToWalletAmount(t *testing.T) {
	tests := []struct {
		name     string
		quota    int64
		rate     string
		expected int64
	}{
		{name: "one USD", quota: 500_000, rate: "7.3", expected: 7_300_000},
		{name: "zero", quota: 0, rate: "7.3", expected: 0},
		{name: "round to nearest micro CNY", quota: 1, rate: "7.3", expected: 15},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rate, err := decimal.NewFromString(test.rate)
			require.NoError(t, err)
			amount, err := quotaToWalletAmount(test.quota, rate)
			require.NoError(t, err)
			assert.Equal(t, test.expected, amount)
		})
	}
}

func TestWalletCallbackPayloadsUseAPIPlatformUserID(t *testing.T) {
	// Exercise the actual HTTP send path: a permissive DTO-to-DTO mock can hide
	// numeric/string mismatches at the Facade boundary.
	for _, userID := range []int64{0, 42, 9007199254740993, 9223372036854775807} {
		if strconv.IntSize < 64 && userID > 2147483647 {
			continue
		}
		for _, operation := range []string{"reserve", "confirm", "cancel"} {
			t.Run(operation+"/"+strconv.FormatInt(userID, 10), func(t *testing.T) {
				expectedID := strconv.FormatInt(userID, 10)
				server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
					var raw map[string]json.RawMessage
					if !assert.NoError(t, common.DecodeJson(request.Body, &raw)) {
						http.Error(w, "invalid JSON", 400)
						return
					}
					assert.Equal(t, `"`+expectedID+`"`, string(raw["api_platform_user_id"]))
					assert.NotContains(t, raw, "user_id")
					assert.Equal(t, http.MethodPost, request.Method)
					assert.Equal(t, "/api/v1/callback/wallet/api-platform/usage/"+operation, request.URL.Path)
					var strictID string
					if !assert.NoError(t, common.Unmarshal(raw["api_platform_user_id"], &strictID)) {
						http.Error(w, "identity must be string", 400)
						return
					}
					parsed, err := strconv.ParseInt(strictID, 10, 64)
					if err != nil || parsed <= 0 {
						http.Error(w, "identity must be positive", 400)
						return
					}
					writeWalletCallbackSuccess(t, w, request, raw)
				}))
				t.Cleanup(server.Close)
				previousClient := walletCallbackClient
				walletCallbackClient = server.Client()
				t.Cleanup(func() { walletCallbackClient = previousClient })
				var payload any
				switch operation {
				case "reserve":
					payload = walletReserveRequest{APIPlatformUserID: int(userID), APIRequestID: "wire-id-contract", EstimateAmount: "1", UsageAtMS: 1}
				case "confirm":
					payload = walletConfirmRequest{APIPlatformUserID: int(userID), APIRequestID: "wire-id-contract", FinalAmount: "1"}
				case "cancel":
					payload = walletCancelRequest{APIPlatformUserID: int(userID), APIRequestID: "wire-id-contract"}
				}
				err := sendWalletCallback(context.Background(), walletCallbackConfig{BaseURL: server.URL, Timeout: time.Second}, "/api/v1/callback/wallet/api-platform/usage/"+operation, payload)
				if userID == 0 {
					var rejected *walletCallbackHTTPError
					require.ErrorAs(t, err, &rejected)
					assert.Equal(t, http.StatusBadRequest, rejected.StatusCode)
				} else {
					require.NoError(t, err)
				}
			})
		}
	}
}

func TestWalletCallbackInvalidIdentityCannotCreateOrSendReserve(t *testing.T) {
	useWalletCallbackTestDB(t, &model.WalletUsageCallback{})
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests++; http.Error(w, "unexpected request", 500) }))
	t.Cleanup(server.Close)
	t.Setenv("WALLET_CALLBACK_BASE_URL", server.URL)
	t.Setenv("WALLET_CALLBACK_ENABLED", "true")
	for _, userID := range []int{0, -1} {
		_, err := newWalletUsageCallbackSession(&relaycommon.RelayInfo{UserId: userID, RequestId: "invalid-identity", StartTime: time.Now()}, 1)
		require.Error(t, err)
	}
	assert.Zero(t, requests)
	var count int64
	require.NoError(t, model.DB.Model(&model.WalletUsageCallback{}).Count(&count).Error)
	assert.Zero(t, count)
}

func TestBillingSessionSettleEqualAmountStillConfirmsWallet(t *testing.T) {
	useWalletCallbackTestDB(t, &model.WalletUsageCallback{})

	originalRate := operation_setting.USDExchangeRate
	operation_setting.USDExchangeRate = 7.3
	t.Cleanup(func() { operation_setting.USDExchangeRate = originalRate })

	var mu sync.Mutex
	paths := make([]string, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		mu.Lock()
		paths = append(paths, request.URL.Path)
		mu.Unlock()
		writeWalletCallbackSuccess(t, w, request, nil)
	}))
	t.Cleanup(server.Close)
	t.Setenv("WALLET_CALLBACK_BASE_URL", server.URL)
	t.Setenv("WALLET_CALLBACK_ENABLED", "true")
	t.Setenv("WALLET_CALLBACK_FAIL_CLOSED", "false")

	relayInfo := &relaycommon.RelayInfo{
		RequestId:       "wallet-callback-equal-amount",
		UserId:          42,
		StartTime:       time.Now(),
		IsPlayground:    true,
		OriginModelName: "gpt-5",
		TokenName:       "desktop",
	}
	callback, err := newWalletUsageCallbackSession(relayInfo, 500_000)
	require.NoError(t, err)
	require.NotNil(t, callback)

	session := &BillingSession{
		relayInfo:        relayInfo,
		funding:          walletCallbackFundingStub{},
		preConsumedQuota: 500_000,
		walletCallback:   callback,
	}
	require.NoError(t, session.Settle(500_000))

	record, err := model.GetWalletUsageCallback(relayInfo.RequestId)
	require.NoError(t, err)
	assert.Equal(t, model.WalletCallbackStatusConfirmed, record.Status)
	require.NotNil(t, record.FinalAmount)
	assert.Equal(t, int64(7_300_000), *record.FinalAmount)
	mu.Lock()
	assert.Equal(t, []string{
		"/api/v1/callback/wallet/api-platform/usage/reserve",
		"/api/v1/callback/wallet/api-platform/usage/confirm",
	}, paths)
	mu.Unlock()
}

func TestWalletReserveFailureMode(t *testing.T) {
	tests := []struct {
		name          string
		failClosed    string
		initialQuota  int
		expectError   bool
		expectedQuota int
		expectedState string
	}{
		{name: "fail open by default", failClosed: "", initialQuota: 1000, expectError: false, expectedQuota: 900, expectedState: model.WalletCallbackStatusReservePending},
		{name: "fail closed preserves local charge", failClosed: "true", initialQuota: 1000, expectError: true, expectedQuota: 900, expectedState: model.WalletCallbackStatusCancelPending},
		{name: "fail closed allows overdraft", failClosed: "true", initialQuota: 0, expectError: true, expectedQuota: -100, expectedState: model.WalletCallbackStatusCancelPending},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			useWalletCallbackTestDB(t, &model.User{}, &model.WalletUsageCallback{})
			require.NoError(t, model.DB.Create(&model.User{Id: 42, Username: "wallet_callback_user", Quota: test.initialQuota}).Error)

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				http.Error(w, "wallet unavailable", http.StatusServiceUnavailable)
			}))
			t.Cleanup(server.Close)
			t.Setenv("WALLET_CALLBACK_BASE_URL", server.URL)
			t.Setenv("WALLET_CALLBACK_ENABLED", "true")
			t.Setenv("WALLET_CALLBACK_FAIL_CLOSED", test.failClosed)

			context, _ := gin.CreateTestContext(httptest.NewRecorder())
			relayInfo := &relaycommon.RelayInfo{
				RequestId:       "wallet-mode-" + test.name,
				UserId:          42,
				StartTime:       time.Now(),
				IsPlayground:    true,
				ForcePreConsume: true,
				OriginModelName: "gpt-5",
				TokenName:       "desktop",
				UserSetting:     dto.UserSetting{BillingPreference: "wallet_only"},
			}
			apiErr := PreConsumeBilling(context, 100, relayInfo)
			if test.expectError {
				require.NotNil(t, apiErr)
			} else {
				require.Nil(t, apiErr)
			}

			var user model.User
			require.NoError(t, model.DB.First(&user, 42).Error)
			assert.Equal(t, test.expectedQuota, user.Quota)
			record, err := model.GetWalletUsageCallback(relayInfo.RequestId)
			require.NoError(t, err)
			assert.Equal(t, test.expectedState, record.Status)
		})
	}
}

func TestWalletFailOpenStillRejectsInsufficientLocalQuota(t *testing.T) {
	useWalletCallbackTestDB(t, &model.User{}, &model.WalletUsageCallback{})
	require.NoError(t, model.DB.Create(&model.User{Id: 42, Username: "wallet_callback_user", Quota: 0}).Error)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "wallet unavailable", http.StatusServiceUnavailable)
	}))
	t.Cleanup(server.Close)
	t.Setenv("WALLET_CALLBACK_BASE_URL", server.URL)
	t.Setenv("WALLET_CALLBACK_ENABLED", "true")
	t.Setenv("WALLET_CALLBACK_FAIL_CLOSED", "false")

	ginContext, _ := gin.CreateTestContext(httptest.NewRecorder())
	relayInfo := &relaycommon.RelayInfo{
		RequestId:       "wallet-fail-open-local-insufficient",
		UserId:          42,
		StartTime:       time.Now(),
		IsPlayground:    true,
		ForcePreConsume: true,
		OriginModelName: "gpt-5",
		TokenName:       "desktop",
		UserSetting:     dto.UserSetting{BillingPreference: "wallet_only"},
	}

	apiErr := PreConsumeBilling(ginContext, 100, relayInfo)
	require.NotNil(t, apiErr)
	assert.Equal(t, types.ErrorCodeInsufficientUserQuota, apiErr.GetErrorCode())
	assert.ErrorIs(t, model.DB.First(&model.WalletUsageCallback{}, "api_request_id = ?", relayInfo.RequestId).Error, gorm.ErrRecordNotFound)

	var user model.User
	require.NoError(t, model.DB.First(&user, 42).Error)
	assert.Equal(t, 0, user.Quota)
}

func TestWalletReserveInsufficientFundsRefundsLocalCharge(t *testing.T) {
	useWalletCallbackTestDB(t, &model.User{}, &model.WalletUsageCallback{})
	require.NoError(t, model.DB.Create(&model.User{Id: 42, Username: "wallet_callback_user", Quota: 1000}).Error)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"code":210008,"message":"insufficient funds"}`))
	}))
	t.Cleanup(server.Close)
	t.Setenv("WALLET_CALLBACK_BASE_URL", server.URL)
	t.Setenv("WALLET_CALLBACK_ENABLED", "true")
	t.Setenv("WALLET_CALLBACK_FAIL_CLOSED", "true")

	ginContext, _ := gin.CreateTestContext(httptest.NewRecorder())
	relayInfo := &relaycommon.RelayInfo{
		RequestId:       "wallet-insufficient-funds-refund",
		UserId:          42,
		StartTime:       time.Now(),
		IsPlayground:    true,
		ForcePreConsume: true,
		OriginModelName: "gpt-5",
		TokenName:       "desktop",
		UserSetting:     dto.UserSetting{BillingPreference: "wallet_only"},
	}

	apiErr := PreConsumeBilling(ginContext, 100, relayInfo)
	require.NotNil(t, apiErr)

	var user model.User
	require.NoError(t, model.DB.First(&user, 42).Error)
	assert.Equal(t, 1000, user.Quota)
	record, err := model.GetWalletUsageCallback(relayInfo.RequestId)
	require.NoError(t, err)
	assert.Equal(t, model.WalletCallbackStatusRejected, record.Status)
	assert.Equal(t, walletFailureInsufficientFunds, record.FailureCode)
}

func TestWalletReserveCompletionDoesNotOverwriteNewerCancelTarget(t *testing.T) {
	useWalletCallbackTestDB(t, &model.WalletUsageCallback{})
	record := &model.WalletUsageCallback{
		APIRequestID:      "wallet-callback-cancel-race",
		APIPlatformUserID: 42,
		ReservedQuota:     100,
		ReservedAmount:    1460,
		ExchangeRate:      "7.30000000",
	}
	require.NoError(t, model.CreateWalletUsageCallback(record))
	require.NoError(t, model.PrepareWalletUsageCancel(record.APIRequestID, time.Now().Add(time.Minute).UnixMilli()))

	require.NoError(t, model.MarkWalletUsageReserved(record.APIRequestID, true))
	stored, err := model.GetWalletUsageCallback(record.APIRequestID)
	require.NoError(t, err)
	assert.Equal(t, model.WalletCallbackStatusCancelPending, stored.Status)
	assert.Positive(t, stored.ReservedAtMS)
}

func TestWalletUncertainReserveFailureRetriesReserveBeforeCancel(t *testing.T) {
	useWalletCallbackTestDB(t, &model.WalletUsageCallback{})
	t.Setenv("WALLET_CALLBACK_FAIL_CLOSED", "true")

	var mu sync.Mutex
	paths := make([]string, 0, 3)
	reserveRequests := make([]walletReserveRequest, 0, 2)
	reserveCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		mu.Lock()
		paths = append(paths, request.URL.Path)
		var decoded any
		if request.URL.Path == "/api/v1/callback/wallet/api-platform/usage/reserve" {
			var payload walletReserveRequest
			require.NoError(t, common.DecodeJson(request.Body, &payload))
			decoded = payload
			reserveRequests = append(reserveRequests, payload)
			reserveCalls++
			if reserveCalls == 1 {
				mu.Unlock()
				http.Error(w, "wallet unavailable", http.StatusServiceUnavailable)
				return
			}
		}
		mu.Unlock()
		writeWalletCallbackSuccess(t, w, request, decoded)
	}))
	t.Cleanup(server.Close)
	t.Setenv("WALLET_CALLBACK_BASE_URL", server.URL)
	t.Setenv("WALLET_CALLBACK_ENABLED", "true")

	relayInfo := &relaycommon.RelayInfo{RequestId: "wallet-reserve-then-cancel", UserId: 42, StartTime: time.Now(), OriginModelName: "gpt-5", TokenName: "desktop"}
	callback, err := newWalletUsageCallbackSession(relayInfo, 100)
	require.Error(t, err)
	require.NotNil(t, callback)

	record, err := model.GetWalletUsageCallback(relayInfo.RequestId)
	require.NoError(t, err)
	assert.Equal(t, model.WalletCallbackStatusCancelPending, record.Status)
	assert.Zero(t, record.ReservedAtMS)

	require.NoError(t, ReconcileWalletUsageCallback(context.Background(), relayInfo.RequestId))
	record, err = model.GetWalletUsageCallback(relayInfo.RequestId)
	require.NoError(t, err)
	assert.Equal(t, model.WalletCallbackStatusCancelled, record.Status)
	assert.Positive(t, record.ReservedAtMS)
	mu.Lock()
	assert.Equal(t, []string{
		"/api/v1/callback/wallet/api-platform/usage/reserve",
		"/api/v1/callback/wallet/api-platform/usage/reserve",
		"/api/v1/callback/wallet/api-platform/usage/cancel",
	}, paths)
	require.Len(t, reserveRequests, 2)
	for _, request := range reserveRequests {
		require.NotNil(t, request.Extra)
		require.NotNil(t, request.Extra.APIUsageDetail)
		assert.Equal(t, "gpt-5", request.Extra.APIUsageDetail.ModelName)
		assert.Equal(t, "desktop", request.Extra.APIUsageDetail.APIKeyName)
	}
	mu.Unlock()
}

func TestWalletLegacyCallbackOmitsExtra(t *testing.T) {
	useWalletCallbackTestDB(t, &model.WalletUsageCallback{})

	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		require.NoError(t, common.DecodeJson(request.Body, &body))
		writeWalletCallbackSuccess(t, w, request, body)
	}))
	t.Cleanup(server.Close)
	t.Setenv("WALLET_CALLBACK_BASE_URL", server.URL)
	t.Setenv("WALLET_CALLBACK_ENABLED", "true")

	record := &model.WalletUsageCallback{
		APIRequestID: "legacy-wallet-callback", APIPlatformUserID: 42,
		ReservedQuota: 100, ReservedAmount: 100, ExchangeRate: "7.30000000",
	}
	require.NoError(t, model.CreateWalletUsageCallback(record))
	require.NoError(t, processWalletUsageCallback(context.Background(), record, loadWalletCallbackConfig()))
	_, hasExtra := body["extra"]
	assert.False(t, hasExtra)
}

func TestWalletReserveBusinessRejectionDoesNotCancel(t *testing.T) {
	useWalletCallbackTestDB(t, &model.WalletUsageCallback{})
	t.Setenv("WALLET_CALLBACK_FAIL_CLOSED", "true")

	var mu sync.Mutex
	paths := make([]string, 0, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		mu.Lock()
		paths = append(paths, request.URL.Path)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"code":210008,"message":"insufficient funds"}`))
	}))
	t.Cleanup(server.Close)
	t.Setenv("WALLET_CALLBACK_BASE_URL", server.URL)
	t.Setenv("WALLET_CALLBACK_ENABLED", "true")

	relayInfo := &relaycommon.RelayInfo{RequestId: "wallet-reserve-rejected", UserId: 42, StartTime: time.Now(), OriginModelName: "gpt-5", TokenName: "desktop"}
	callback, err := newWalletUsageCallbackSession(relayInfo, 100)
	require.Error(t, err)
	require.NotNil(t, callback)
	require.NoError(t, callback.Cancel())

	record, err := model.GetWalletUsageCallback(relayInfo.RequestId)
	require.NoError(t, err)
	assert.Equal(t, model.WalletCallbackStatusRejected, record.Status)
	assert.Equal(t, walletFailureInsufficientFunds, record.FailureCode)
	assert.Zero(t, record.ReservedAtMS)
	mu.Lock()
	assert.Equal(t, []string{"/api/v1/callback/wallet/api-platform/usage/reserve"}, paths)
	mu.Unlock()
}

func TestWalletReserveOmitsEmptyBusinessOrderNo(t *testing.T) {
	useWalletCallbackTestDB(t, &model.WalletUsageCallback{})
	t.Setenv("WALLET_CALLBACK_FAIL_CLOSED", "true")

	var body map[string]any
	var decodeErr error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		decodeErr = common.DecodeJson(request.Body, &body)
		writeWalletCallbackSuccess(t, w, request, body)
	}))
	t.Cleanup(server.Close)
	t.Setenv("WALLET_CALLBACK_BASE_URL", server.URL)
	t.Setenv("WALLET_CALLBACK_ENABLED", "true")

	relayInfo := &relaycommon.RelayInfo{RequestId: "wallet-reserve-without-order", UserId: 42, StartTime: time.Now(), OriginModelName: "gpt-5", TokenName: "desktop"}
	callback, err := newWalletUsageCallbackSession(relayInfo, 100)
	require.NoError(t, err)
	require.NotNil(t, callback)
	require.NoError(t, decodeErr)

	_, hasBusinessOrderNo := body["business_order_no"]
	assert.False(t, hasBusinessOrderNo)
	extra := body["extra"].(map[string]any)
	detail := extra["api_usage_detail"].(map[string]any)
	assert.Equal(t, "gpt-5", detail["model_name"])
	assert.Equal(t, "desktop", detail["api_key_name"])
	record, err := model.GetWalletUsageCallback(relayInfo.RequestId)
	require.NoError(t, err)
	assert.Nil(t, record.BusinessOrderNo)
	assert.Equal(t, "gpt-5", record.ModelName)
	assert.Equal(t, "desktop", record.APIKeyName)
}

func TestCoveredBillingSkipsLocalQuotaAndCarriesOrderNo(t *testing.T) {
	useWalletCallbackTestDB(t, &model.User{}, &model.Token{}, &model.WalletUsageCallback{})
	require.NoError(t, model.DB.Create(&model.User{Id: 7, Username: "covered_user", Quota: 500}).Error)
	require.NoError(t, model.DB.Create(&model.Token{Id: 93, UserId: 7, Key: "covered-user-key", RemainQuota: 100}).Error)

	var mu sync.Mutex
	var bodies []map[string]any
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		var body map[string]any
		if r.URL.Path == "/api/v1/callback/wallet/api-platform/usage/reserve" {
			_ = common.DecodeJson(r.Body, &body)
			bodies = append(bodies, body)
		}
		mu.Unlock()
		if body == nil {
			writeWalletCallbackSuccess(t, w, r, nil)
		} else {
			writeWalletCallbackSuccess(t, w, r, body)
		}
	}))
	t.Cleanup(server.Close)
	t.Setenv("WALLET_CALLBACK_BASE_URL", server.URL)
	t.Setenv("WALLET_CALLBACK_ENABLED", "true")
	t.Setenv("WALLET_CALLBACK_FAIL_CLOSED", "true")
	t.Setenv("WALLET_CALLBACK_TOKEN", strings.Repeat("c", 32))
	t.Setenv("BUSINESS_BILLING_CALLER_TOKEN", strings.Repeat("b", 32))
	t.Setenv("WALLET_CALLBACK_AUTO_RETRY_ENABLED", "false")
	previousClient := walletCallbackClient
	walletCallbackClient = server.Client()
	t.Cleanup(func() { walletCallbackClient = previousClient })

	ginContext, _ := gin.CreateTestContext(httptest.NewRecorder())
	ginContext.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ginContext.Request.Header.Set("X-Business-Order", "order-monthly-001")
	ginContext.Request.Header.Set(common.BusinessBillingAuthorizationHeader, "Bearer "+strings.Repeat("b", 32))
	ginContext.Set("id", 7)
	ginContext.Set("token_id", 93)
	require.NoError(t, AuthenticateBusinessBilling(ginContext))
	relayInfo := &relaycommon.RelayInfo{
		RequestId:       "covered-request-1",
		UserId:          7,
		StartTime:       time.Now(),
		BusinessOrderNo: ginContext.GetString(common.BusinessBillingOrderContextKey),
		TokenId:         93,
		TokenKey:        "covered-user-key",
		OriginModelName: "gpt-5",
		TokenName:       "desktop",
	}

	apiErr := PreConsumeBilling(ginContext, 0, relayInfo)
	require.Nil(t, apiErr)
	require.NotNil(t, relayInfo.Billing)
	assert.Equal(t, BillingSourceBusinessIncluded, relayInfo.BillingSource)
	require.NoError(t, relayInfo.Billing.Reserve(200))
	require.NoError(t, relayInfo.Billing.Settle(200))
	require.NoError(t, relayInfo.Billing.Settle(200))
	var token model.Token
	require.NoError(t, model.DB.First(&token, 93).Error)
	assert.Equal(t, 100, token.RemainQuota)
	assert.Zero(t, token.UsedQuota)

	var user model.User
	require.NoError(t, model.DB.First(&user, 7).Error)
	assert.Equal(t, 500, user.Quota, "local quota must not change in covered mode")

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, bodies, 1)
	assert.Equal(t, "7", bodies[0]["api_platform_user_id"])
	assert.Equal(t, "order-monthly-001", bodies[0]["business_order_no"])
	assert.Equal(t, "1", bodies[0]["estimate_amount"], "zero estimate must still validate the paid parent")
	_, hasLegacyUserID := bodies[0]["user_id"]
	assert.False(t, hasLegacyUserID)
}

func TestWalletCallbackClaimUsesConditionalLease(t *testing.T) {
	useWalletCallbackTestDB(t, &model.WalletUsageCallback{})
	nowMS := time.Now().UnixMilli()
	record := &model.WalletUsageCallback{
		APIRequestID: "wallet-callback-claim", APIPlatformUserID: 42,
		ReservedQuota: 100, ReservedAmount: 100,
		ExchangeRate: "7.30000000", NextRetryAtMS: nowMS,
	}
	require.NoError(t, model.CreateWalletUsageCallback(record))

	claimed, err := model.ClaimWalletUsageCallback(record.ID, model.WalletCallbackStatusReservePending, nowMS, nowMS+30_000)
	require.NoError(t, err)
	assert.True(t, claimed)
	claimed, err = model.ClaimWalletUsageCallback(record.ID, model.WalletCallbackStatusReservePending, nowMS, nowMS+30_000)
	require.NoError(t, err)
	assert.False(t, claimed)
}

func TestWalletCallbackCircuitBreakerProbesThenRecovers(t *testing.T) {
	var breaker walletCallbackCircuitBreaker
	now := time.Now()
	limit, allowed := breaker.scanLimit(now)
	assert.True(t, allowed)
	assert.Equal(t, walletCallbackBatchSize, limit)

	breaker.recordUnavailable(now)
	limit, allowed = breaker.scanLimit(now.Add(4 * time.Second))
	assert.False(t, allowed)
	assert.Zero(t, limit)
	limit, allowed = breaker.scanLimit(now.Add(5 * time.Second))
	assert.True(t, allowed)
	assert.Equal(t, 1, limit)

	breaker.recordAvailable()
	limit, allowed = breaker.scanLimit(now.Add(5 * time.Second))
	assert.True(t, allowed)
	assert.Equal(t, walletCallbackBatchSize, limit)
}

func TestWalletCallbackConfigDefaultsToFailOpen(t *testing.T) {
	t.Setenv("WALLET_CALLBACK_BASE_URL", "http://wallet.example")
	t.Setenv("WALLET_CALLBACK_ENABLED", "true")
	t.Setenv("WALLET_CALLBACK_FAIL_CLOSED", "")

	config := loadWalletCallbackConfig()
	assert.True(t, config.Enabled)
	assert.False(t, config.FailClosed)
	assert.Equal(t, 3*time.Second, config.Timeout)
}

func TestWalletDisabledFailClosedStillRequiresLocalQuota(t *testing.T) {
	useWalletCallbackTestDB(t, &model.User{})
	require.NoError(t, model.DB.Create(&model.User{Id: 7, Username: "no-wallet", Quota: 0}).Error)
	t.Setenv("WALLET_CALLBACK_BASE_URL", "https://wallet.internal")
	t.Setenv("WALLET_CALLBACK_ENABLED", "false")
	t.Setenv("WALLET_CALLBACK_FAIL_CLOSED", "true")
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	_, apiErr := NewBillingSession(c, &relaycommon.RelayInfo{UserId: 7, IsPlayground: true, UserSetting: dto.UserSetting{BillingPreference: "wallet_only"}}, 10)
	require.NotNil(t, apiErr)
	assert.Equal(t, types.ErrorCodeInsufficientUserQuota, apiErr.GetErrorCode())
	var user model.User
	require.NoError(t, model.DB.First(&user, 7).Error)
	assert.Zero(t, user.Quota)
}
