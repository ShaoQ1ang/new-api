package openrouter

import (
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSelectWan27Handler(t *testing.T) {
	assert.Equal(t, "wan27", SelectHandler("alibaba/wan-2.7").Family())
}

func TestWan27BuildsEverySupportedMode(t *testing.T) {
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "alibaba/wan-2.7"}}
	seed := int64(7)
	tests := []struct {
		name  string
		req   relaycommon.TaskSubmitReq
		check func(*testing.T, map[string]any)
	}{
		{name: "text with audio", req: relaycommon.TaskSubmitReq{Mode: "text_to_video", Audios: []string{"voice"}}, check: func(t *testing.T, body map[string]any) {
			assert.Equal(t, "voice", body["audio"])
			assert.NotContains(t, body, "frame_images")
		}},
		{name: "first", req: relaycommon.TaskSubmitReq{Mode: "first_frame", Images: []string{"first"}}, check: func(t *testing.T, body map[string]any) { require.Len(t, body["frame_images"], 1) }},
		{name: "first last", req: relaycommon.TaskSubmitReq{Mode: "first_last_frame", Images: []string{"first", "last"}}, check: func(t *testing.T, body map[string]any) { require.Len(t, body["frame_images"], 2) }},
		{name: "references", req: relaycommon.TaskSubmitReq{Mode: "reference", Images: []string{"image", "opening"}, ImageRoles: []string{"general_reference", "first_frame"}, Videos: []string{"video"}, Audios: []string{"voice"}}, check: func(t *testing.T, body map[string]any) {
			require.Len(t, body["frame_images"], 1)
			require.Len(t, body["input_references"], 3)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.req.Model, tt.req.Prompt, tt.req.Duration, tt.req.Resolution, tt.req.AspectRatio, tt.req.Seed = "alibaba/wan-2.7", "test", 10, "1080p", "4:3", &seed
			tt.req.Metadata = map[string]any{"negative_prompt": "blur", "prompt_extend": false}
			body, err := (&Wan27Handler{BaseHandler: NewBaseHandler("wan27")}).BuildUpstreamRequest(info, &tt.req)
			require.NoError(t, err)
			assert.Equal(t, "alibaba/wan-2.7", body["model"])
			assert.Equal(t, 10, body["duration"])
			assert.Equal(t, "1080p", body["resolution"])
			assert.Equal(t, "4:3", body["aspect_ratio"])
			assert.Equal(t, int64(7), body["seed"])
			assert.Equal(t, true, body["generate_audio"])
			assert.Equal(t, "blur", body["negative_prompt"])
			assert.NotContains(t, body, "mode")
			tt.check(t, body)
		})
	}
}

func TestWan27RejectsUnsupportedModeAndDuration(t *testing.T) {
	handler := &Wan27Handler{BaseHandler: NewBaseHandler("wan27")}
	req := relaycommon.TaskSubmitReq{Model: "alibaba/wan-2.7", Prompt: "test", Mode: "video_edit", Duration: 10}
	require.ErrorContains(t, handler.Validate(&req), "unsupported wan2.7 mode")
	req.Mode, req.Duration = "text_to_video", 11
	require.ErrorContains(t, handler.Validate(&req), "between 2 and 10")
}

func TestWan27BillingUsesActualAudioState(t *testing.T) {
	audio := false
	params, err := convertWan27VideoBillingParams(relaycommon.TaskSubmitReq{Model: "alibaba/wan-2.7", Prompt: "test", Mode: "text_to_video", Duration: 6, Resolution: "720p", GenerateAudio: &audio})
	require.NoError(t, err)
	assert.Equal(t, "720p", params.Tier)
	assert.Equal(t, 6, params.DurationSeconds)
	assert.False(t, params.AudioEnabled)
}
