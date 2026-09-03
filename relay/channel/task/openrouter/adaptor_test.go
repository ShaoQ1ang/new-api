package openrouter

import (
	"bytes"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskAdaptorDoResponseReturnsPublicResponseWithoutWritingHTTP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", nil)
	context.Set("task_request", relaycommon.TaskSubmitReq{Model: "google/veo-3.1-lite"})
	info := &relaycommon.RelayInfo{
		OriginModelName: "video-public",
		TaskRelayInfo:   &relaycommon.TaskRelayInfo{PublicTaskID: "task_public"},
	}
	response := &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewBufferString(`{"id":"upstream_1","status":"queued"}`))}

	taskID, _, publicResponse, taskErr := (&TaskAdaptor{}).DoResponse(context, response, info)

	require.Nil(t, taskErr)
	assert.Equal(t, "upstream_1", taskID)
	video, ok := publicResponse.(*dto.OpenAIVideo)
	require.True(t, ok)
	assert.Equal(t, "task_public", video.ID)
	assert.Empty(t, recorder.Body.String())
}

func TestSelectHandlerUsesSupportedOpenRouterVideoFamilies(t *testing.T) {
	tests := []struct {
		model  string
		family string
	}{
		{model: "bytedance/seedance-2.0", family: "seedance"},
		{model: "google/veo-3.1-lite", family: "veo"},
		{model: "alibaba/happyhorse-1.1", family: "happyhorse"},
		{model: "kwaivgi/kling-v3.0-std", family: "kling"},
		{model: "kwaivgi/kling-v3.0-pro", family: "kling"},
		{model: "kwaivgi/kling-video-o1", family: "kling"},
		{model: "minimax/hailuo-3", family: "minimax"},
		{model: "minimax/hailuo-2.3", family: "minimax"},
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
	assert.Equal(t, "16:9", body["aspect_ratio"])
	assert.Equal(t, true, body["generate_audio"])
	frames, ok := body["frame_images"].([]map[string]any)
	require.True(t, ok)
	require.Len(t, frames, 2)
	assert.Equal(t, "first_frame", frames[0]["frame_type"])
	assert.Equal(t, "last_frame", frames[1]["frame_type"])
	references, ok := body["input_references"].([]map[string]any)
	require.True(t, ok)
	require.Len(t, references, 1)
	assert.Equal(t, "image_url", references[0]["type"])

	billing, err := handler.EstimateBillingContext(req)
	require.NoError(t, err)
	assert.Equal(t, 6, billing.DurationSeconds)
	assert.Equal(t, "1080p", billing.ResolutionTier)
	require.NotNil(t, billing.AudioEnabled)
	assert.True(t, *billing.AudioEnabled)
}

func TestVeoHandlerConvertsLegacyMetadataToCanonicalOpenRouterPayload(t *testing.T) {
	frames := []map[string]any{
		buildFrameImage("first_frame", "https://example.com/first.png"),
		buildFrameImage("last_frame", "https://example.com/last.png"),
	}
	handler := &VeoHandler{BaseHandler: NewBaseHandler("veo")}

	for _, modelName := range []string{"google/veo-3.1-lite", "google/veo-3.1", "google/veo-3.1-fast"} {
		t.Run(modelName, func(t *testing.T) {
			info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: modelName}}
			req := &relaycommon.TaskSubmitReq{
				Model: modelName, Prompt: "cinematic", Resolution: "720p", FrameImages: frames,
				Metadata: map[string]any{
					"aspectRatio":     "16:9",
					"durationSeconds": float64(4),
					"watermark":       false,
				},
			}

			body, err := handler.BuildUpstreamRequest(info, req)

			require.NoError(t, err)
			assert.Equal(t, map[string]any{
				"model": modelName, "prompt": "cinematic", "duration": 4,
				"resolution": "720p", "aspect_ratio": "16:9", "frame_images": frames,
			}, body)
		})
	}
}

func TestVeoHandlerPreservesCanonicalOpenRouterPayload(t *testing.T) {
	frames := []map[string]any{
		buildFrameImage("first_frame", "data:image/jpeg;base64,AAAA"),
		buildFrameImage("last_frame", "data:image/jpeg;base64,BBBB"),
	}
	handler := &VeoHandler{BaseHandler: NewBaseHandler("veo")}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "google/veo-3.1-lite"}}
	req := &relaycommon.TaskSubmitReq{
		Model: "google/veo-3.1-lite", Prompt: "cinematic", Duration: 6,
		Size: "1280x720", FrameImages: frames,
	}

	body, err := handler.BuildUpstreamRequest(info, req)

	require.NoError(t, err)
	assert.Equal(t, map[string]any{
		"model": "google/veo-3.1-lite", "prompt": "cinematic", "duration": 6,
		"size": "1280x720", "frame_images": frames,
	}, body)
}

func TestHappyHorseHandlerBuildsReferenceRequestAndBilling(t *testing.T) {
	handler := &HappyHorseHandler{BaseHandler: NewBaseHandler("happyhorse")}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "alibaba/happyhorse-1.1"}}
	req := &relaycommon.TaskSubmitReq{
		Model: "alibaba/happyhorse-1.1", Prompt: "preserve the subject", Seconds: "10",
		Images:   []string{"https://example.com/a.png", "https://example.com/b.png"},
		Metadata: map[string]any{"resolution": "720p", "aspect_ratio": "1:1", "audio": false},
	}

	body, err := handler.BuildUpstreamRequest(info, req)
	require.NoError(t, err)
	assert.Equal(t, 10, body["duration"])
	assert.Equal(t, "720p", body["resolution"])
	assert.NotContains(t, body, "generate_audio")
	frames, ok := body["frame_images"].([]map[string]any)
	require.True(t, ok)
	require.Len(t, frames, 1)
	assert.Equal(t, "first_frame", frames[0]["frame_type"])
	references, ok := body["input_references"].([]map[string]any)
	require.True(t, ok)
	require.Len(t, references, 1)
	assert.Equal(t, "image_url", references[0]["type"])

	billing, err := handler.EstimateBillingContext(req)
	require.NoError(t, err)
	assert.Equal(t, "720p", billing.ResolutionTier)
	assert.Equal(t, 10, billing.DurationSeconds)
	require.NotNil(t, billing.AudioEnabled)
	assert.False(t, *billing.AudioEnabled)
}

func TestOpenRouterHandlersAcceptCanonicalTopLevelVideoParameters(t *testing.T) {
	audio := false
	seed := int64(0)
	handler := &HappyHorseHandler{BaseHandler: NewBaseHandler("happyhorse")}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "alibaba/happyhorse-1.1"}}
	req := &relaycommon.TaskSubmitReq{
		Model:         "alibaba/happyhorse-1.1",
		Prompt:        "cinematic",
		Duration:      5,
		Size:          "1280x720",
		GenerateAudio: &audio,
		Seed:          &seed,
		CallbackURL:   "https://example.com/video-callback",
		FrameImages: []map[string]any{
			buildFrameImage("first_frame", "https://example.com/first.png"),
		},
		InputReferences: []map[string]any{
			buildInputReference("image", "https://example.com/reference.png"),
		},
		Provider: map[string]any{"order": []string{"atlas-cloud"}},
	}

	body, err := handler.BuildUpstreamRequest(info, req)
	require.NoError(t, err)
	assert.Equal(t, "1280x720", body["size"])
	assert.NotContains(t, body, "resolution")
	assert.Equal(t, int64(0), body["seed"])
	assert.Equal(t, "https://example.com/video-callback", body["callback_url"])
	assert.Equal(t, req.FrameImages, body["frame_images"])
	assert.Equal(t, req.InputReferences, body["input_references"])
	assert.Equal(t, req.Provider, body["provider"])
}

func TestHappyHorseBuildsNativeOpenRouterPayload(t *testing.T) {
	var req relaycommon.TaskSubmitReq
	require.NoError(t, common.Unmarshal([]byte(`{
		"model":"alibaba/happyhorse-1.1",
		"prompt":"a slow cinematic push-in",
		"duration":5,
		"size":"1280x720",
		"frame_images":[{
			"type":"image_url",
			"image_url":{"url":"data:image/png;base64,AAAA"},
			"frame_type":"first_frame"
		}]
	}`), &req))

	handler := &HappyHorseHandler{BaseHandler: NewBaseHandler("happyhorse")}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: req.Model}}
	body, err := handler.BuildUpstreamRequest(info, &req)
	require.NoError(t, err)

	assert.Equal(t, 5, body["duration"])
	assert.Equal(t, "1280x720", body["size"])
	assert.NotContains(t, body, "resolution")
	assert.Equal(t, req.FrameImages, body["frame_images"])
	assert.NotContains(t, body, "input_references")
}

func TestKlingHandlerPreservesExplicitAudioFalseAndProviderOptions(t *testing.T) {
	handler := &KlingHandler{BaseHandler: NewBaseHandler("kling")}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "kwaivgi/kling-v3.0-std"}}
	req := &relaycommon.TaskSubmitReq{
		Model: "kwaivgi/kling-v3.0-std", Prompt: "camera push in", Duration: 5,
		Images:   []string{"https://example.com/first.png", "https://example.com/last.png"},
		Metadata: map[string]any{"audio": false, "negative_prompt": "blur"},
	}

	body, err := handler.BuildUpstreamRequest(info, req)
	require.NoError(t, err)
	assert.Equal(t, false, body["generate_audio"])
	frames, ok := body["frame_images"].([]map[string]any)
	require.True(t, ok)
	require.Len(t, frames, 2)
	assert.Equal(t, "image_url", frames[0]["type"])
	provider, ok := body["provider"].(map[string]any)
	require.True(t, ok)
	options := provider["options"].(map[string]any)["atlas-cloud"].(map[string]any)
	assert.Equal(t, "blur", options["negative_prompt"])

	billing, err := handler.EstimateBillingContext(req)
	require.NoError(t, err)
	assert.False(t, *billing.AudioEnabled)
	assert.Equal(t, "720p", billing.ResolutionTier)
}

func TestKlingHandlerKeepsGenericReferencesWithExplicitFrames(t *testing.T) {
	handler := &KlingHandler{BaseHandler: NewBaseHandler("kling")}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "kwaivgi/kling-v3.0-std"}}
	req := &relaycommon.TaskSubmitReq{
		Model: "kwaivgi/kling-v3.0-std", Prompt: "cinematic", Duration: 5,
		Images: []string{"https://example.com/reference.png"},
		FrameImages: []map[string]any{
			buildFrameImage("first_frame", "https://example.com/first.png"),
		},
	}

	body, err := handler.BuildUpstreamRequest(info, req)
	require.NoError(t, err)
	references, ok := body["input_references"].([]map[string]any)
	require.True(t, ok)
	require.Len(t, references, 1)
	assert.Equal(t, "https://example.com/reference.png", references[0]["image_url"].(map[string]any)["url"])
}

func TestKlingO1OnlyPassesSupportedProviderOptions(t *testing.T) {
	handler := &KlingHandler{BaseHandler: NewBaseHandler("kling")}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "kwaivgi/kling-video-o1"}}
	req := &relaycommon.TaskSubmitReq{
		Model: "kwaivgi/kling-video-o1", Prompt: "cinematic", Duration: 5,
		Metadata: map[string]any{"negative_prompt": "blur", "cfg_scale": 0.7},
	}

	body, err := handler.BuildUpstreamRequest(info, req)
	require.NoError(t, err)
	provider := body["provider"].(map[string]any)
	options := provider["options"].(map[string]any)["atlas-cloud"].(map[string]any)
	assert.Equal(t, "blur", options["negative_prompt"])
	assert.NotContains(t, options, "cfg_scale")
}

func TestOpenRouterHandlersRejectMalformedCanonicalInputs(t *testing.T) {
	handler := &MiniMaxHandler{BaseHandler: NewBaseHandler("minimax")}
	base := relaycommon.TaskSubmitReq{Model: "minimax/hailuo-2.3", Prompt: "cinematic", Duration: 6}

	invalidFrame := base
	invalidFrame.FrameImages = []map[string]any{buildFrameImage("last_frame", "https://example.com/last.png")}
	require.ErrorContains(t, handler.Validate(&invalidFrame), "unsupported frame_type")

	invalidReference := base
	invalidReference.InputReferences = []map[string]any{{"type": "image_url"}}
	require.ErrorContains(t, handler.Validate(&invalidReference), "image_url.url is required")

	invalidCallback := base
	invalidCallback.CallbackURL = "http://example.com/callback"
	require.ErrorContains(t, handler.Validate(&invalidCallback), "valid HTTPS URL")
}

func TestMiniMaxHandlersUseModelCapabilitiesForRequestAndBilling(t *testing.T) {
	tests := []struct {
		name       string
		model      string
		duration   int
		resolution string
		audio      bool
	}{
		{name: "hailuo 3", model: "minimax/hailuo-3", duration: 5, resolution: "2K", audio: true},
		{name: "hailuo 2.3", model: "minimax/hailuo-2.3", duration: 6, resolution: "1080p", audio: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := &MiniMaxHandler{BaseHandler: NewBaseHandler("minimax")}
			info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: tt.model}}
			req := &relaycommon.TaskSubmitReq{Model: tt.model, Prompt: "cinematic", Duration: tt.duration}

			body, err := handler.BuildUpstreamRequest(info, req)
			require.NoError(t, err)
			assert.Equal(t, tt.resolution, body["resolution"])
			assert.Equal(t, tt.audio, body["generate_audio"])

			billing, err := handler.EstimateBillingContext(req)
			require.NoError(t, err)
			assert.Equal(t, strings.ToLower(tt.resolution), billing.ResolutionTier)
			assert.Equal(t, tt.audio, *billing.AudioEnabled)
		})
	}
}

func TestMiniMaxH3KeepsGenericImagesAsReferencesWithExplicitFrames(t *testing.T) {
	handler := &MiniMaxHandler{BaseHandler: NewBaseHandler("minimax")}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "minimax/hailuo-3"}}
	req := &relaycommon.TaskSubmitReq{
		Model: "minimax/hailuo-3", Prompt: "cinematic", Duration: 5,
		Images: []string{"https://example.com/reference.png"},
		Metadata: map[string]any{
			"frame_images": []map[string]any{
				buildFrameImage("first_frame", "https://example.com/first.png"),
			},
		},
	}

	body, err := handler.BuildUpstreamRequest(info, req)
	require.NoError(t, err)
	references, ok := body["input_references"].([]map[string]any)
	require.True(t, ok)
	require.Len(t, references, 1)
	assert.Equal(t, "image_url", references[0]["type"])
	assert.Equal(t, "https://example.com/reference.png", references[0]["image_url"].(map[string]any)["url"])
}

func TestOpenRouterVideoHandlersRejectUnsupportedDurations(t *testing.T) {
	tests := []struct {
		handler ModelHandler
		req     relaycommon.TaskSubmitReq
	}{
		{handler: &HappyHorseHandler{BaseHandler: NewBaseHandler("happyhorse")}, req: relaycommon.TaskSubmitReq{Model: "alibaba/happyhorse-1.1", Prompt: "x", Duration: 16}},
		{handler: &KlingHandler{BaseHandler: NewBaseHandler("kling")}, req: relaycommon.TaskSubmitReq{Model: "kwaivgi/kling-video-o1", Prompt: "x", Duration: 7}},
		{handler: &MiniMaxHandler{BaseHandler: NewBaseHandler("minimax")}, req: relaycommon.TaskSubmitReq{Model: "minimax/hailuo-2.3", Prompt: "x", Duration: 8}},
	}
	for _, tt := range tests {
		require.Error(t, tt.handler.Validate(&tt.req))
	}
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
	assert.Equal(t, "video_url", references[0]["type"])
	assert.Equal(t, "image_url", references[1]["type"])
	assert.Equal(t, "audio_url", references[2]["type"])

	req.Duration = 16
	require.ErrorContains(t, handler.Validate(req), "between 4 and 15")
}

func TestSeedanceHandlerConvertsLegacyMetadataToCanonicalOpenRouterPayload(t *testing.T) {
	handler := &SeedanceHandler{BaseHandler: NewBaseHandler("seedance")}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "bytedance/seedance-2.0"}}
	req := &relaycommon.TaskSubmitReq{
		Model: "bytedance/seedance-2.0", Prompt: "cinematic", Resolution: "480p",
		Metadata: map[string]any{
			"aspectRatio":     "16:9",
			"durationSeconds": float64(5),
			"watermark":       false,
		},
	}

	body, err := handler.BuildUpstreamRequest(info, req)

	require.NoError(t, err)
	assert.Equal(t, "480p", body["resolution"])
	assert.Equal(t, "16:9", body["aspect_ratio"])
	assert.Equal(t, 5, body["duration"])
	assert.Equal(t, false, body["watermark"])
	assert.NotContains(t, body, "size")
	assert.NotContains(t, body, "aspectRatio")
	assert.NotContains(t, body, "durationSeconds")
}

func TestVeoHandlerUsesModelSpecificResolutionCapabilities(t *testing.T) {
	handler := &VeoHandler{BaseHandler: NewBaseHandler("veo")}
	for _, modelName := range []string{"google/veo-3.1", "google/veo-3.1-fast"} {
		t.Run(modelName, func(t *testing.T) {
			info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: modelName}}
			body, err := handler.BuildUpstreamRequest(info, &relaycommon.TaskSubmitReq{
				Model: modelName, Prompt: "cinematic", Duration: 4, Size: "3840x2160",
			})

			require.NoError(t, err)
			assert.Equal(t, "3840x2160", body["size"])
		})
	}

	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "google/veo-3.1"}}
	body, err := handler.BuildUpstreamRequest(info, &relaycommon.TaskSubmitReq{
		Model: "google/veo-3.1", Prompt: "cinematic", Duration: 4, Resolution: "4k", AspectRatio: "16:9",
	})
	require.NoError(t, err)
	assert.Equal(t, "4K", body["resolution"])

	info = &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "google/veo-3.1-lite"}}
	_, err = handler.BuildUpstreamRequest(info, &relaycommon.TaskSubmitReq{
		Model: "google/veo-3.1-lite", Prompt: "cinematic", Duration: 4, Size: "3840x2160",
	})
	require.ErrorContains(t, err, "unsupported veo resolution")
}

func TestOpenRouterLegacyDurationSecondsRejectsUnboundedBillingMultiplier(t *testing.T) {
	requests := []struct {
		name    string
		handler ModelHandler
		model   string
	}{
		{name: "veo", handler: &VeoHandler{BaseHandler: NewBaseHandler("veo")}, model: "google/veo-3.1"},
		{name: "seedance", handler: &SeedanceHandler{BaseHandler: NewBaseHandler("seedance")}, model: "bytedance/seedance-2.0"},
	}
	for _, tt := range requests {
		t.Run(tt.name, func(t *testing.T) {
			info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: tt.model}}
			req := &relaycommon.TaskSubmitReq{
				Model: tt.model, Prompt: "test",
				Metadata: map[string]any{"durationSeconds": float64(relaycommon.MaxTaskDurationSeconds + 1)},
			}

			_, err := tt.handler.BuildUpstreamRequest(info, req)

			require.ErrorContains(t, err, "durationSeconds must be between")
		})
	}
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

func TestTaskAdaptorCheckedBillingIncludesFixedVideoPrice(t *testing.T) {
	adaptor := &TaskAdaptor{}
	task := &model.Task{PrivateData: model.TaskPrivateData{BillingContext: &model.TaskBillingContext{
		VideoSecondsUnitPrice: 0.13,
		VideoFixedPrice:       0.08,
		VideoDurationSeconds:  5,
		GroupRatio:            1,
	}}}

	quota, clamp := adaptor.AdjustBillingOnCompleteChecked(task, &relaycommon.TaskInfo{DurationSeconds: 5})

	assert.Nil(t, clamp)
	assert.Equal(t, common.QuotaFromFloat(0.73*common.QuotaPerUnit), quota)
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
