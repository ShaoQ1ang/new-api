package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBusinessBillingAuthentication(t *testing.T) {
	for _, tc := range []struct {
		name        string
		order       []string
		credential  []string
		baseURL     string
		enabled     string
		failClosed  string
		path        string
		wantAllowed bool
		wantCovered bool
	}{
		{name: "ordinary user request", wantAllowed: true},
		{name: "forged order", order: []string{"br:1"}},
		{name: "wrong workload key", order: []string{"br:1"}, credential: []string{"Bearer wrong"}},
		{name: "key without order", credential: []string{"Bearer " + strings.Repeat("b", 32)}},
		{name: "duplicate orders", order: []string{"br:1", "br:2"}, credential: []string{"Bearer " + strings.Repeat("b", 32)}},
		{name: "duplicate credentials", order: []string{"br:1"}, credential: []string{"Bearer " + strings.Repeat("b", 32), "Bearer wrong"}},
		{name: "ambiguous whitespace", order: []string{" br:1"}, credential: []string{"Bearer " + strings.Repeat("b", 32)}},
		{name: "disabled callback", order: []string{"br:1"}, credential: []string{"Bearer " + strings.Repeat("b", 32)}, enabled: "false"},
		{name: "fail open", order: []string{"br:1"}, credential: []string{"Bearer " + strings.Repeat("b", 32)}, failClosed: "false"},
		{name: "plain HTTP", order: []string{"br:1"}, credential: []string{"Bearer " + strings.Repeat("b", 32)}, baseURL: "http://wallet.internal"},
		{name: "URL userinfo", order: []string{"br:1"}, credential: []string{"Bearer " + strings.Repeat("b", 32)}, baseURL: "https://user@wallet.internal"},
		{name: "async endpoint", order: []string{"br:1"}, credential: []string{"Bearer " + strings.Repeat("b", 32)}, path: "/v1/videos"},
		{name: "covered embeddings", order: []string{"br-100105-7"}, credential: []string{"Bearer " + strings.Repeat("b", 32)}, path: "/v1/embeddings", wantAllowed: true, wantCovered: true},
		{name: "valid internal request", order: []string{"br:1"}, credential: []string{"Bearer " + strings.Repeat("b", 32)}, wantAllowed: true, wantCovered: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("BUSINESS_BILLING_CALLER_TOKEN", strings.Repeat("b", 32))
			t.Setenv("WALLET_CALLBACK_TOKEN", strings.Repeat("c", 32))
			t.Setenv("WALLET_CALLBACK_AUTO_RETRY_ENABLED", "false")
			baseURL, enabled, failClosed, path := tc.baseURL, tc.enabled, tc.failClosed, tc.path
			if baseURL == "" {
				baseURL = "https://wallet.internal"
			}
			if enabled == "" {
				enabled = "true"
			}
			if failClosed == "" {
				failClosed = "true"
			}
			if path == "" {
				path = "/v1/chat/completions"
			}
			t.Setenv("WALLET_CALLBACK_BASE_URL", baseURL)
			t.Setenv("WALLET_CALLBACK_ENABLED", enabled)
			t.Setenv("WALLET_CALLBACK_FAIL_CLOSED", failClosed)
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, path, nil)
			c.Request.Header.Set("Authorization", "Bearer real-user-token")
			for _, v := range tc.order {
				c.Request.Header.Add(common.BusinessBillingOrderHeader, v)
			}
			for _, v := range tc.credential {
				c.Request.Header.Add(common.BusinessBillingAuthorizationHeader, v)
			}
			err := AuthenticateBusinessBilling(c)
			if tc.wantAllowed {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
			assert.Equal(t, tc.wantCovered, c.GetBool(common.BusinessBillingAuthenticatedContextKey))
			assert.Empty(t, c.Request.Header.Get(common.BusinessBillingOrderHeader))
			assert.Empty(t, c.Request.Header.Get(common.BusinessBillingAuthorizationHeader))
			assert.Equal(t, "Bearer real-user-token", c.Request.Header.Get("Authorization"))
		})
	}
}

func TestCoveredBillingRejectsCallerSuppliedIdentity(t *testing.T) {
	t.Setenv("WALLET_CALLBACK_BASE_URL", "https://wallet.internal")
	t.Setenv("WALLET_CALLBACK_ENABLED", "true")
	t.Setenv("WALLET_CALLBACK_FAIL_CLOSED", "true")
	t.Setenv("WALLET_CALLBACK_TOKEN", strings.Repeat("c", 32))
	t.Setenv("WALLET_CALLBACK_AUTO_RETRY_ENABLED", "false")
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set(common.BusinessBillingOrderContextKey, "br:1")
	c.Set(common.BusinessBillingAuthenticatedContextKey, true)
	c.Set("id", 7)
	c.Set("token_id", 93)
	_, apiErr := NewBillingSession(c, &relaycommon.RelayInfo{UserId: 8, TokenId: 93, BusinessOrderNo: "br:1"}, 0)
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	c.Set(common.BusinessBillingAuthenticatedContextKey, false)
	_, apiErr = NewBillingSession(c, &relaycommon.RelayInfo{UserId: 7, TokenId: 93, BusinessOrderNo: "br:1"}, 0)
	require.NotNil(t, apiErr)
}

func TestWalletCallbackRejectsFalseSuccessAndRedirects(t *testing.T) {
	order := "br:1"
	for _, tc := range []struct {
		name     string
		status   int
		response string
		wantOK   bool
	}{
		{"valid covered response", 200, `{"data":{"funding_source":3,"reserved_amount":"1"}}`, true},
		{"empty data", 200, `{"data":{}}`, false},
		{"business error at HTTP 200", 200, `{"code":210008,"data":{"funding_source":3,"reserved_amount":"1"}}`, false},
		{"explicit false success", 200, `{"success":false,"data":{"funding_source":3,"reserved_amount":"1"}}`, false},
		{"ordinary wallet charged", 200, `{"data":{"funding_source":1,"reserved_amount":"1"}}`, false},
		{"wrong amount", 200, `{"data":{"funding_source":3,"reserved_amount":"2"}}`, false},
		{"numeric amount violates Facade", 200, `{"data":{"funding_source":3,"reserved_amount":1}}`, false},
		{"trailing JSON", 200, `{"data":{"funding_source":3,"reserved_amount":"1"}}{}`, false},
		{"empty accepted response", 202, ``, false},
		{"oversized body", 200, strings.Repeat(" ", 16*1024+1), false},
		{"redirect", 307, ``, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if tc.status == 307 {
					w.Header().Set("Location", "/other")
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.response))
			}))
			defer server.Close()
			err := sendWalletCallback(context.Background(), walletCallbackConfig{BaseURL: server.URL, Token: "test-token", Timeout: time.Second}, "/reserve", walletReserveRequest{APIPlatformUserID: 7, APIRequestID: "r1", EstimateAmount: "1", BusinessOrderNo: &order})
			if tc.wantOK {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
			assert.Equal(t, int32(1), calls.Load(), "no redirect or automatic retry")
		})
	}
}

func TestWalletCallbacksRemainPendingWithoutAutomaticRetry(t *testing.T) {
	useWalletCallbackTestDB(t, &model.WalletUsageCallback{})
	t.Setenv("WALLET_CALLBACK_AUTO_RETRY_ENABLED", "")
	t.Setenv("WALLET_CALLBACK_ENABLED", "true")
	t.Setenv("WALLET_CALLBACK_FAIL_CLOSED", "true")
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); w.WriteHeader(503) }))
	defer server.Close()
	t.Setenv("WALLET_CALLBACK_BASE_URL", server.URL)
	_, err := newWalletUsageCallbackSession(&relaycommon.RelayInfo{RequestId: "pending-one", UserId: 7, OriginModelName: "model", TokenName: "key"}, 10)
	require.Error(t, err)
	runWalletCallbackTaskOnce()
	assert.Equal(t, int32(1), calls.Load())
	record, err := model.GetWalletUsageCallback("pending-one")
	require.NoError(t, err)
	assert.Equal(t, model.WalletCallbackStatusCancelPending, record.Status)
	assert.NotEmpty(t, record.FailureCode)
	assert.NotEmpty(t, record.LastError)
}

func TestWalletTerminalIntentIsAtomicAndImmutable(t *testing.T) {
	useWalletCallbackTestDB(t, &model.WalletUsageCallback{})
	record := &model.WalletUsageCallback{APIRequestID: "terminal-race", APIPlatformUserID: 7, ReservedAmount: 1, ExchangeRate: "7.3"}
	require.NoError(t, model.CreateWalletUsageCallback(record))
	require.NoError(t, model.MarkWalletUsageReserved(record.APIRequestID, true))
	start := make(chan struct{})
	errors := make(chan error, 2)
	var group sync.WaitGroup
	group.Add(2)
	go func() {
		defer group.Done()
		<-start
		errors <- model.PrepareWalletUsageConfirm(record.APIRequestID, 1, 1, time.Now().Add(time.Minute).UnixMilli())
	}()
	go func() {
		defer group.Done()
		<-start
		errors <- model.PrepareWalletUsageCancel(record.APIRequestID, time.Now().Add(time.Minute).UnixMilli())
	}()
	close(start)
	group.Wait()
	close(errors)
	for err := range errors {
		require.NoError(t, err)
	}
	first, err := model.GetWalletUsageCallback(record.APIRequestID)
	require.NoError(t, err)
	require.Contains(t, []string{model.WalletCallbackStatusConfirmPending, model.WalletCallbackStatusCancelPending}, first.Status)
	require.NoError(t, model.PrepareWalletUsageConfirm(record.APIRequestID, 2, 2, time.Now().Add(time.Minute).UnixMilli()))
	require.NoError(t, model.PrepareWalletUsageCancel(record.APIRequestID, time.Now().Add(time.Minute).UnixMilli()))
	last, err := model.GetWalletUsageCallback(record.APIRequestID)
	require.NoError(t, err)
	assert.Equal(t, first.Status, last.Status)
	assert.Equal(t, first.FinalAmount, last.FinalAmount)
}

func TestWalletManualReconciliationUsesExclusiveLease(t *testing.T) {
	useWalletCallbackTestDB(t, &model.WalletUsageCallback{})
	t.Setenv("WALLET_CALLBACK_ENABLED", "true")
	t.Setenv("WALLET_CALLBACK_AUTO_RETRY_ENABLED", "false")
	record := &model.WalletUsageCallback{APIRequestID: "manual-exclusive", APIPlatformUserID: 7, ReservedAmount: 1, ExchangeRate: "7.3"}
	require.NoError(t, model.CreateWalletUsageCallback(record))
	require.NoError(t, model.MarkWalletUsageReserved(record.APIRequestID, true))
	require.NoError(t, model.PrepareWalletUsageCancel(record.APIRequestID, time.Now().UnixMilli()))
	started, release := make(chan struct{}), make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
		writeWalletCallbackSuccess(t, w, r, nil)
	}))
	defer server.Close()
	t.Setenv("WALLET_CALLBACK_BASE_URL", server.URL)
	completed := make(chan error, 1)
	go func() { completed <- ReconcileWalletUsageCallback(context.Background(), record.APIRequestID) }()
	<-started
	err := ReconcileWalletUsageCallback(context.Background(), record.APIRequestID)
	close(release)
	require.Error(t, err)
	require.NoError(t, <-completed)
	stored, err := model.GetWalletUsageCallback(record.APIRequestID)
	require.NoError(t, err)
	assert.Equal(t, model.WalletCallbackStatusCancelled, stored.Status)
}
