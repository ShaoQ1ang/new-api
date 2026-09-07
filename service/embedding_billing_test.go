package service

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Uses the same reserve, embedding usage settlement, and failure cleanup calls
// as Controller.Relay -> EmbeddingHelper, with a TLS Wallet contract fixture.
func TestCoveredEmbeddingWalletLifecycle(t *testing.T) {
	for _, tc := range []struct {
		name string
		fail bool
	}{{"success", false}, {"provider_failure", true}} {
		t.Run(tc.name, func(t *testing.T) {
			useWalletCallbackTestDB(t, &model.User{}, &model.Token{}, &model.Channel{}, &model.WalletUsageCallback{})
			require.NoError(t, model.DB.Create(&model.User{Id: 7, Username: "embedding-user", Quota: 500}).Error)
			require.NoError(t, model.DB.Create(&model.Token{Id: 93, UserId: 7, Key: "embedding-key", RemainQuota: 100}).Error)
			oldLog := common.LogConsumeEnabled
			common.LogConsumeEnabled = false
			t.Cleanup(func() { common.LogConsumeEnabled = oldLog })
			var mu sync.Mutex
			var paths []string
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body map[string]any
				require.NoError(t, common.DecodeJson(r.Body, &body))
				assert.Equal(t, "Bearer "+strings.Repeat("c", 32), r.Header.Get("Authorization"))
				assert.Equal(t, "7", body["api_platform_user_id"])
				assert.Equal(t, "embedding-"+tc.name, body["api_request_id"])
				if strings.HasSuffix(r.URL.Path, "/reserve") {
					assert.Equal(t, "br-100105-7", body["business_order_no"])
					assert.Equal(t, "1", body["estimate_amount"], "zero estimate still reserves the parent order")
				}
				mu.Lock()
				paths = append(paths, r.URL.Path)
				mu.Unlock()
				writeWalletCallbackSuccess(t, w, r, body)
			}))
			defer server.Close()
			t.Setenv("WALLET_CALLBACK_BASE_URL", server.URL)
			t.Setenv("WALLET_CALLBACK_ENABLED", "true")
			t.Setenv("WALLET_CALLBACK_FAIL_CLOSED", "true")
			t.Setenv("WALLET_CALLBACK_TOKEN", strings.Repeat("c", 32))
			t.Setenv("BUSINESS_BILLING_CALLER_TOKEN", strings.Repeat("b", 32))
			t.Setenv("WALLET_CALLBACK_AUTO_RETRY_ENABLED", "false")
			previous := walletCallbackClient
			walletCallbackClient = server.Client()
			t.Cleanup(func() { walletCallbackClient = previous })
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/embeddings", nil)
			c.Request.Header.Set(common.BusinessBillingOrderHeader, "br-100105-7")
			c.Request.Header.Set(common.BusinessBillingAuthorizationHeader, "Bearer "+strings.Repeat("b", 32))
			c.Set("id", 7)
			c.Set("token_id", 93)
			require.NoError(t, AuthenticateBusinessBilling(c))
			info := &relaycommon.RelayInfo{RequestId: "embedding-" + tc.name, UserId: 7, TokenId: 93, TokenKey: "embedding-key", TokenName: "business-reporting", StartTime: time.Now(), BusinessOrderNo: c.GetString(common.BusinessBillingOrderContextKey), OriginModelName: "text-embedding-3-small", RelayFormat: types.RelayFormatEmbedding, ChannelMeta: &relaycommon.ChannelMeta{}, PriceData: types.PriceData{ModelRatio: 1, CompletionRatio: 1, GroupRatioInfo: types.GroupRatioInfo{GroupRatio: 1}}}
			require.Nil(t, PreConsumeBilling(c, 0, info))
			require.NotNil(t, info.Billing)
			if tc.fail {
				info.Billing.Refund(c)
				info.Billing.Refund(c)
			} else {
				PostTextConsumeQuota(c, info, &dto.Usage{PromptTokens: 10, TotalTokens: 10}, nil)
				require.NoError(t, info.Billing.Settle(10))
			}
			var user model.User
			var token model.Token
			require.NoError(t, model.DB.First(&user, 7).Error)
			require.NoError(t, model.DB.First(&token, 93).Error)
			assert.Equal(t, 500, user.Quota)
			assert.Equal(t, 100, token.RemainQuota)
			assert.Zero(t, token.UsedQuota)
			record, err := model.GetWalletUsageCallback(info.RequestId)
			require.NoError(t, err)
			want := "/confirm"
			status := model.WalletCallbackStatusConfirmed
			if tc.fail {
				want = "/cancel"
				status = model.WalletCallbackStatusCancelled
			}
			assert.Equal(t, status, record.Status)
			mu.Lock()
			defer mu.Unlock()
			require.Len(t, paths, 2, "exactly one reserve and one immutable terminal call")
			assert.True(t, strings.HasSuffix(paths[0], "/reserve"))
			assert.True(t, strings.HasSuffix(paths[1], want))
		})
	}
}

func TestCoveredEmbeddingCannotSettleThroughLegacyQuota(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	require.Error(t, SettleBilling(c, &relaycommon.RelayInfo{BusinessOrderNo: "br-100105-7"}, 10))
}
