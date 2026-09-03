package gemini

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildRequestBodyMapsVeoFirstAndLastFrames(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(nil)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", nil)
	c.Set("task_request", relaycommon.TaskSubmitReq{
		Model:  "veo-3.1-generate-preview",
		Prompt: "bridge the frames",
		Images: []string{
			"data:image/png;base64,iVBORw0KGgo=",
			"data:image/jpeg;base64,/9j/",
		},
	})
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}, TaskRelayInfo: &relaycommon.TaskRelayInfo{}}

	body, err := (&TaskAdaptor{}).BuildRequestBody(c, info)
	require.NoError(t, err)
	raw, err := io.ReadAll(body)
	require.NoError(t, err)
	var payload VeoRequestPayload
	require.NoError(t, common.Unmarshal(raw, &payload))
	require.Len(t, payload.Instances, 1)
	require.NotNil(t, payload.Instances[0].Image)
	require.NotNil(t, payload.Instances[0].LastFrame)
	assert.Equal(t, "image/png", payload.Instances[0].Image.MimeType)
	assert.Equal(t, "image/jpeg", payload.Instances[0].LastFrame.MimeType)
	assert.Equal(t, constant.TaskActionFirstTailGenerate, info.Action)
}

func TestValidateRequestRejectsMoreThanTwoVeoFrames(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(nil)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", strings.NewReader(`{
		"model":"veo-3.1-generate-preview","prompt":"animate","images":["a","b","c"]
	}`))
	c.Request.Header.Set("Content-Type", "application/json")
	taskErr := (&TaskAdaptor{}).ValidateRequestAndSetAction(c, &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}, TaskRelayInfo: &relaycommon.TaskRelayInfo{}})
	require.NotNil(t, taskErr)
	assert.Equal(t, http.StatusBadRequest, taskErr.StatusCode)
	assert.Contains(t, taskErr.Message, "at most first and last frame")
}

func TestBuildRequestBodyPrefersCanonicalVeoOutput(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(nil)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", nil)
	c.Set("task_request", relaycommon.TaskSubmitReq{
		Model: "veo-3.1-generate-preview", Prompt: "move", Resolution: "1080p", AspectRatio: "9:16", Duration: 6,
		Metadata: map[string]any{"resolution": "720p", "aspectRatio": "16:9"},
	})
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}, TaskRelayInfo: &relaycommon.TaskRelayInfo{}}

	body, err := (&TaskAdaptor{}).BuildRequestBody(c, info)
	require.NoError(t, err)
	raw, err := io.ReadAll(body)
	require.NoError(t, err)
	var payload VeoRequestPayload
	require.NoError(t, common.Unmarshal(raw, &payload))
	assert.Equal(t, "1080p", payload.Parameters.Resolution)
	assert.Equal(t, "9:16", payload.Parameters.AspectRatio)
	assert.Equal(t, 6, payload.Parameters.DurationSeconds)
}
