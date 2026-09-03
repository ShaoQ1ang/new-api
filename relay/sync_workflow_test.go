package relay

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSyncRelayContextIsolatesResponseAndPreservesIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	outerRecorder := httptest.NewRecorder()
	source, _ := gin.CreateTestContext(outerRecorder)
	source.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	source.Set("id", 7)
	common.SetContextKey(source, constant.ContextKeyTokenId, 11)
	common.SetContextKey(source, constant.ContextKeyUsingGroup, "vip")

	isolated, recorder := newSyncRelayContext(source)
	isolated.JSON(http.StatusOK, gin.H{"ok": true})

	assert.Empty(t, outerRecorder.Body.String())
	assert.JSONEq(t, `{"ok":true}`, recorder.Body.String())
	assert.Equal(t, 7, isolated.GetInt("id"))
	assert.Equal(t, 11, common.GetContextKeyInt(isolated, constant.ContextKeyTokenId))
	assert.Equal(t, "vip", common.GetContextKeyString(isolated, constant.ContextKeyUsingGroup))
}

func TestSyncWorkflowSelectsChannelWhenAIGCRouteHasNoDistribution(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	require.True(t, syncWorkflowNeedsChannelSelection(c, &relaycommon.RelayInfo{}))

	common.SetContextKey(c, constant.ContextKeyChannelId, 9)
	assert.False(t, syncWorkflowNeedsChannelSelection(c, &relaycommon.RelayInfo{}))
	assert.True(t, syncWorkflowNeedsChannelSelection(c, &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}}))
}
