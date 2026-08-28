package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type walletCallbackFundingStub struct{}

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
	payloads := []any{
		walletReserveRequest{APIPlatformUserID: 42},
		walletConfirmRequest{APIPlatformUserID: 42},
		walletCancelRequest{APIPlatformUserID: 42},
	}
	for _, payload := range payloads {
		encoded, err := common.Marshal(payload)
		require.NoError(t, err)
		var decoded map[string]any
		require.NoError(t, common.Unmarshal(encoded, &decoded))
		assert.Equal(t, float64(42), decoded["api_platform_user_id"])
		_, hasLegacyUserID := decoded["user_id"]
		assert.False(t, hasLegacyUserID)
	}
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
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{}}`))
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
		expectError   bool
		expectedQuota int
		expectedState string
	}{
		{name: "fail open by default", failClosed: "", expectError: false, expectedQuota: 900, expectedState: model.WalletCallbackStatusReservePending},
		{name: "fail closed compensates local charge", failClosed: "true", expectError: true, expectedQuota: 1000, expectedState: model.WalletCallbackStatusCancelPending},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			useWalletCallbackTestDB(t, &model.User{}, &model.WalletUsageCallback{})
			require.NoError(t, model.DB.Create(&model.User{Id: 42, Username: "wallet_callback_user", Quota: 1000}).Error)

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
		if request.URL.Path == "/api/v1/callback/wallet/api-platform/usage/reserve" {
			var payload walletReserveRequest
			require.NoError(t, common.DecodeJson(request.Body, &payload))
			reserveRequests = append(reserveRequests, payload)
			reserveCalls++
			if reserveCalls == 1 {
				mu.Unlock()
				http.Error(w, "wallet unavailable", http.StatusServiceUnavailable)
				return
			}
		}
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{}}`))
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

	require.NoError(t, callback.Cancel())
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
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{}}`))
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
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{}}`))
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
	useWalletCallbackTestDB(t, &model.User{}, &model.WalletUsageCallback{})
	require.NoError(t, model.DB.Create(&model.User{Id: 7, Username: "covered_user", Quota: 500}).Error)

	var mu sync.Mutex
	var bodies []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		if r.URL.Path == "/api/v1/callback/wallet/api-platform/usage/reserve" {
			var body map[string]any
			_ = common.DecodeJson(r.Body, &body)
			bodies = append(bodies, body)
		}
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{}}`))
	}))
	t.Cleanup(server.Close)
	t.Setenv("WALLET_CALLBACK_BASE_URL", server.URL)
	t.Setenv("WALLET_CALLBACK_ENABLED", "true")
	t.Setenv("WALLET_CALLBACK_FAIL_CLOSED", "true")

	ginContext, _ := gin.CreateTestContext(httptest.NewRecorder())
	ginContext.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ginContext.Request.Header.Set("X-Business-Order", "order-monthly-001")
	relayInfo := &relaycommon.RelayInfo{
		RequestId:       "covered-request-1",
		UserId:          7,
		StartTime:       time.Now(),
		BusinessOrderNo: ginContext.Request.Header.Get("X-Business-Order"),
		OriginModelName: "gpt-5",
		TokenName:       "desktop",
	}

	apiErr := PreConsumeBilling(ginContext, 100, relayInfo)
	require.Nil(t, apiErr)
	require.NotNil(t, relayInfo.Billing)
	assert.Equal(t, BillingSourceBusinessIncluded, relayInfo.BillingSource)

	var user model.User
	require.NoError(t, model.DB.First(&user, 7).Error)
	assert.Equal(t, 500, user.Quota, "local quota must not change in covered mode")

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, bodies, 1)
	assert.Equal(t, float64(7), bodies[0]["api_platform_user_id"])
	assert.Equal(t, "order-monthly-001", bodies[0]["business_order_no"])
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
