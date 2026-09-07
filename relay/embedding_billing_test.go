package relay

import (
	"net/http"
	"net/http/httptest"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCoveredEmbeddingRequiresReservedBillingBeforeProvider(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/embeddings", nil)
	for _, source := range []string{"", service.BillingSourceWallet, service.BillingSourceBusinessIncluded} {
		info := &relaycommon.RelayInfo{BusinessOrderNo: "br-100105-7", BillingSource: source}
		err := EmbeddingHelper(c, info)
		require.NotNil(t, err)
		assert.Equal(t, http.StatusForbidden, err.StatusCode)
		assert.Nil(t, info.ChannelMeta, "must reject before initializing or contacting a provider")
	}
}
