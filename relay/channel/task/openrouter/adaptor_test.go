package openrouter

import (
	"math"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSelectHandlerUsesSupportedOpenRouterVideoFamilies(t *testing.T) {
	tests := []struct {
		model  string
		family string
	}{
		{model: "bytedance/seedance-2.0", family: "seedance"},
		{model: "google/veo-3.1-lite", family: "veo"},
		{model: "openai/sora-2", family: "default"},
	}
	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			assert.Equal(t, tt.family, SelectHandler(tt.model).Family())
		})
	}
}

func TestSelectRequestHandlerUsesMappedUpstreamModel(t *testing.T) {
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{
		UpstreamModelName: "google/veo-3.1-lite",
		IsModelMapped:     true,
	}}

	assert.Equal(t, "veo", selectRequestHandler(info, "my-video").Family())
}

func TestVeoHandlerBuildsFramesReferencesAndNormalizedBilling(t *testing.T) {
	handler := &VeoHandler{BaseHandler: NewBaseHandler("veo")}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "google/veo-3.1-lite"}}
	req := &relaycommon.TaskSubmitReq{
		Model:   "google/veo-3.1-lite",
		Prompt:  "animate",
		Seconds: "7",
		Images: []string{
			"https://example.com/first.png",
			"https://example.com/last.png",
			"https://example.com/reference.png",
		},
		Metadata: map[string]any{
			"resolution":     "1080p",
			"aspect_ratio":   "16:9",
			"generate_audio": true,
		},
	}

	body, err := handler.BuildUpstreamRequest(info, req)
	require.NoError(t, err)
	assert.Equal(t, "google/veo-3.1-lite", body["model"])
	assert.Equal(t, 6, body["duration"])
	assert.Equal(t, "1080p", body["resolution"])
	assert.Equal(t, true, body["generate_audio"])
	frames, ok := body["frame_images"].([]map[string]any)
	require.True(t, ok)
	require.Len(t, frames, 2)
	assert.Equal(t, "first_frame", frames[0]["frame_type"])
	assert.Equal(t, "last_frame", frames[1]["frame_type"])
	references, ok := body["input_references"].([]map[string]any)
	require.True(t, ok)
	require.Len(t, references, 1)
	assert.Equal(t, "image", references[0]["type"])

	billing, err := handler.EstimateBillingContext(req)
	require.NoError(t, err)
	assert.Equal(t, 6, billing.DurationSeconds)
	assert.Equal(t, "1080p", billing.ResolutionTier)
	require.NotNil(t, billing.AudioEnabled)
	assert.True(t, *billing.AudioEnabled)
}

func TestVeoHandlerRejectsUnsupportedOutput(t *testing.T) {
	handler := &VeoHandler{BaseHandler: NewBaseHandler("veo")}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "google/veo-3.1-lite"}}

	_, err := handler.BuildUpstreamRequest(info, &relaycommon.TaskSubmitReq{
		Model: "google/veo-3.1-lite", Prompt: "test", Metadata: map[string]any{"resolution": "4k"},
	})
	require.ErrorContains(t, err, "unsupported veo resolution")

	_, err = handler.BuildUpstreamRequest(info, &relaycommon.TaskSubmitReq{
		Model: "google/veo-3.1-lite", Prompt: "test", Metadata: map[string]any{"aspect_ratio": "1:1"},
	})
	require.ErrorContains(t, err, "unsupported veo aspect_ratio")
}

func TestSeedanceHandlerBuildsMultimodalReferences(t *testing.T) {
	handler := &SeedanceHandler{BaseHandler: NewBaseHandler("seedance")}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "bytedance/seedance-2.0"}}
	req := &relaycommon.TaskSubmitReq{
		Model: "bytedance/seedance-2.0", Prompt: "cinematic", Duration: 10,
		Images: []string{"https://example.com/first.png", "https://example.com/last.png"},
		Videos: []string{"https://example.com/motion.mp4"},
		Metadata: map[string]any{
			"resolution":       "1080p",
			"reference_images": []string{"https://example.com/ref.png"},
			"reference_audios": []string{"https://example.com/audio.mp3"},
		},
	}

	require.NoError(t, handler.Validate(req))
	body, err := handler.BuildUpstreamRequest(info, req)
	require.NoError(t, err)
	frames, ok := body["frame_images"].([]map[string]any)
	require.True(t, ok)
	assert.Len(t, frames, 2)
	references, ok := body["input_references"].([]map[string]any)
	require.True(t, ok)
	require.Len(t, references, 3)
	assert.Equal(t, "video", references[0]["type"])
	assert.Equal(t, "image", references[1]["type"])
	assert.Equal(t, "audio", references[2]["type"])

	req.Duration = 16
	require.ErrorContains(t, handler.Validate(req), "between 4 and 15")
}

func TestBaseHandlerParsesOpenRouterLifecycle(t *testing.T) {
	handler := NewBaseHandler("veo")
	info := &relaycommon.RelayInfo{
		OriginModelName: "google/veo-3.1-lite",
		TaskRelayInfo:   &relaycommon.TaskRelayInfo{PublicTaskID: "task_public"},
	}

	submitted, err := handler.ParseSubmitResponse(info, []byte(`{
		"id":"vid_123","status":"queued","polling_url":"https://openrouter.ai/v1/videos/vid_123"
	}`))
	require.NoError(t, err)
	assert.Equal(t, "vid_123", submitted.UpstreamTaskID)
	assert.Equal(t, "queued", submitted.PublicResponse.Status)
	assert.Equal(t, "https://openrouter.ai/v1/videos/vid_123", submitted.PublicResponse.Metadata["polling_url"])

	completed, err := handler.ParseFetchResponse(nil, []byte(`{
		"data":{"id":"vid_123","status":"completed","duration":8,
		"output":{"video_url":"https://cdn.example.com/video.mp4"},
		"usage":{"video_tokens":800,"total_tokens":800}}
	}`))
	require.NoError(t, err)
	assert.Equal(t, string(model.TaskStatusSuccess), completed.Status)
	assert.Equal(t, "https://cdn.example.com/video.mp4", completed.Url)
	assert.Equal(t, 8, completed.DurationSeconds)
	assert.Equal(t, 800, completed.TotalTokens)

	failed, err := handler.ParseFetchResponse(nil, []byte(`{
		"id":"vid_123","status":"failed","error":{"code":"blocked","message":"policy blocked"}
	}`))
	require.NoError(t, err)
	assert.Equal(t, string(model.TaskStatusFailure), failed.Status)
	assert.Equal(t, "policy blocked", failed.Reason)
}

func TestTaskAdaptorCheckedBillingSaturatesOversizedCharge(t *testing.T) {
	adaptor := &TaskAdaptor{}
	task := &model.Task{PrivateData: model.TaskPrivateData{BillingContext: &model.TaskBillingContext{
		ModelRatio: math.MaxFloat64, GroupRatio: math.MaxFloat64,
	}}}

	quota, clamp := adaptor.AdjustBillingOnCompleteChecked(task, &relaycommon.TaskInfo{TotalTokens: math.MaxInt32})

	assert.Equal(t, common.MaxQuota, quota)
	require.NotNil(t, clamp)
	assert.Equal(t, common.QuotaClampOverflow, clamp.Kind)
}

func TestBaseHandlerSaturatesUntrustedUsageAndDuration(t *testing.T) {
	handler := NewBaseHandler("veo")

	result, err := handler.ParseFetchResponse(nil, []byte(`{
		"status":"completed","duration":"999999999999999999999999",
		"usage":{"video_tokens":"999999999999999999999999","total_tokens":1e300}
	}`))

	require.NoError(t, err)
	assert.Equal(t, relaycommon.MaxTaskDurationSeconds, result.DurationSeconds)
	assert.Equal(t, common.MaxQuota, result.CompletionTokens)
	assert.Equal(t, common.MaxQuota, result.TotalTokens)
}
