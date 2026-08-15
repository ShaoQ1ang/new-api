package openrouter

import (
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKlingV3ModelsUseSharedOpenRouterContract(t *testing.T) {
	for _, modelName := range []string{"kwaivgi/kling-v3.0-std", "kwaivgi/kling-v3.0-pro"} {
		t.Run(modelName, func(t *testing.T) {
			handler := &KlingHandler{BaseHandler: NewBaseHandler("kling")}
			info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: modelName}}
			generateAudio := false
			req := &relaycommon.TaskSubmitReq{
				Model:         modelName,
				Prompt:        "camera push in",
				Duration:      15,
				AspectRatio:   "9:16",
				GenerateAudio: &generateAudio,
				FrameImages: []map[string]any{
					buildFrameImage("first_frame", "https://example.com/first.png"),
					buildFrameImage("last_frame", "https://example.com/last.png"),
				},
				InputReferences: []map[string]any{
					{"type": "image_url", "image_url": map[string]any{"url": "https://example.com/reference.png"}},
				},
				Metadata: map[string]any{"negative_prompt": "blur", "cfg_scale": 0.7},
			}

			body, err := handler.BuildUpstreamRequest(info, req)
			require.NoError(t, err)
			assert.Equal(t, modelName, body["model"])
			assert.Equal(t, 15, body["duration"])
			assert.Equal(t, "720p", body["resolution"])
			assert.Equal(t, "9:16", body["aspect_ratio"])
			assert.Equal(t, false, body["generate_audio"])
			assert.Equal(t, req.FrameImages, body["frame_images"])
			assert.Equal(t, req.InputReferences, body["input_references"])

			provider := body["provider"].(map[string]any)
			options := provider["options"].(map[string]any)["atlas-cloud"].(map[string]any)
			assert.Equal(t, "blur", options["negative_prompt"])
			assert.Equal(t, 0.7, options["cfg_scale"])

			billing, err := handler.EstimateBillingContext(req)
			require.NoError(t, err)
			assert.Equal(t, 15, billing.DurationSeconds)
			assert.Equal(t, "720p", billing.ResolutionTier)
			assert.False(t, *billing.AudioEnabled)
		})
	}
}

func TestKlingHandlerMatchesOnlySupportedModels(t *testing.T) {
	handler := &KlingHandler{BaseHandler: NewBaseHandler("kling")}
	assert.True(t, handler.Match("kwaivgi/kling-v3.0-std"))
	assert.True(t, handler.Match("kwaivgi/kling-v3.0-pro"))
	assert.True(t, handler.Match("kwaivgi/kling-video-o1"))
	assert.False(t, handler.Match("kwaivgi/kling-v3.0-unknown"))
}
