package service

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

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
		RequestId:    "wallet-callback-equal-amount",
		UserId:       42,
		StartTime:    time.Now(),
		IsPlayground: true,
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
		"/internal/callbacks/new-api/usage/reserve",
		"/internal/callbacks/new-api/usage/confirm",
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
		APIRequestID:   "wallet-callback-cancel-race",
		UserID:         42,
		ReservedQuota:  100,
		ReservedAmount: 1460,
		ExchangeRate:   "7.30000000",
	}
	require.NoError(t, model.CreateWalletUsageCallback(record))
	require.NoError(t, model.PrepareWalletUsageCancel(record.APIRequestID))

	require.NoError(t, model.MarkWalletUsageReserved(record.APIRequestID, true))
	stored, err := model.GetWalletUsageCallback(record.APIRequestID)
	require.NoError(t, err)
	assert.Equal(t, model.WalletCallbackStatusCancelPending, stored.Status)
	assert.Positive(t, stored.ReservedAtMS)
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
