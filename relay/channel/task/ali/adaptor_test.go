package ali

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testRelayInfo() *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{},
	}
}

func TestConvertToAliRequestWan27I2VBuildsMediaFromImage(t *testing.T) {
	adaptor := &TaskAdaptor{}
	req := relaycommon.TaskSubmitReq{
		Model:    "wan2.7-i2v",
		Prompt:   "animate the first frame",
		Image:    "https://example.com/first.png",
		Size:     "720p",
		Duration: 10,
	}

	aliReq, err := adaptor.convertToAliRequest(testRelayInfo(), req)

	require.NoError(t, err)
	require.Equal(t, "wan2.7-i2v", aliReq.Model)
	require.Equal(t, "720P", aliReq.Parameters.Resolution)
	require.NotNil(t, aliReq.Parameters.Duration)
	require.Equal(t, 10, *aliReq.Parameters.Duration)
	require.Equal(t, []AliVideoMedia{
		{Type: "first_frame", URL: "https://example.com/first.png"},
	}, aliReq.Input.Media)
	require.Empty(t, aliReq.Input.ImgURL)

	body, err := common.Marshal(aliReq)
	require.NoError(t, err)
	require.Contains(t, string(body), `"media"`)
	require.NotContains(t, string(body), `"img_url"`)
}

func TestConvertToAliRequestWan27I2VBuildsFirstAndLastFrameFromImages(t *testing.T) {
	adaptor := &TaskAdaptor{}
	req := relaycommon.TaskSubmitReq{
		Model:  "wan2.7-i2v",
		Prompt: "interpolate between frames",
		Images: []string{
			"https://example.com/first.png",
			"https://example.com/last.png",
		},
	}

	aliReq, err := adaptor.convertToAliRequest(testRelayInfo(), req)

	require.NoError(t, err)
	require.Equal(t, []AliVideoMedia{
		{Type: "first_frame", URL: "https://example.com/first.png"},
		{Type: "last_frame", URL: "https://example.com/last.png"},
	}, aliReq.Input.Media)
}

func TestConvertToAliRequestWan27I2VPrefersImageBeforeImagesAndInputReference(t *testing.T) {
	adaptor := &TaskAdaptor{}
	req := relaycommon.TaskSubmitReq{
		Model:          "wan2.7-i2v",
		Prompt:         "use the direct image",
		Image:          " https://example.com/direct.png ",
		Images:         []string{"https://example.com/images-first.png", " https://example.com/images-last.png "},
		InputReference: "https://example.com/input-reference.png",
	}

	aliReq, err := adaptor.convertToAliRequest(testRelayInfo(), req)

	require.NoError(t, err)
	require.Equal(t, []AliVideoMedia{
		{Type: "first_frame", URL: "https://example.com/direct.png"},
		{Type: "last_frame", URL: "https://example.com/images-last.png"},
	}, aliReq.Input.Media)
}

func TestConvertToAliRequestWan27I2VFallsBackToFirstNonEmptyImage(t *testing.T) {
	adaptor := &TaskAdaptor{}
	req := relaycommon.TaskSubmitReq{
		Model:  "wan2.7-i2v",
		Prompt: "skip blank images",
		Image:  " ",
		Images: []string{
			" ",
			" https://example.com/first.png ",
			" https://example.com/last.png ",
		},
		InputReference: "https://example.com/input-reference.png",
	}

	aliReq, err := adaptor.convertToAliRequest(testRelayInfo(), req)

	require.NoError(t, err)
	require.Equal(t, []AliVideoMedia{
		{Type: "first_frame", URL: "https://example.com/first.png"},
		{Type: "last_frame", URL: "https://example.com/last.png"},
	}, aliReq.Input.Media)
}

func TestConvertToAliRequestWan27I2VKeepsExplicitMetadataMedia(t *testing.T) {
	adaptor := &TaskAdaptor{}
	req := relaycommon.TaskSubmitReq{
		Model:          "wan2.7-i2v",
		Prompt:         "continue the clip",
		Image:          "https://example.com/direct.png",
		Images:         []string{"https://example.com/images-first.png", "https://example.com/images-last.png"},
		InputReference: "https://example.com/input-reference.png",
		Metadata: map[string]interface{}{
			"input": map[string]interface{}{
				"media": []interface{}{
					map[string]interface{}{
						"type": "first_clip",
						"url":  "https://example.com/input.mp4",
					},
				},
			},
		},
	}

	aliReq, err := adaptor.convertToAliRequest(testRelayInfo(), req)

	require.NoError(t, err)
	require.Equal(t, []AliVideoMedia{
		{Type: "first_clip", URL: "https://example.com/input.mp4"},
	}, aliReq.Input.Media)
	require.Empty(t, aliReq.Input.ImgURL)

	body, err := common.Marshal(aliReq)
	require.NoError(t, err)
	require.Contains(t, string(body), `"media"`)
	require.NotContains(t, string(body), `"img_url"`)
}

func TestConvertToAliRequestWan27I2VRequiresMedia(t *testing.T) {
	adaptor := &TaskAdaptor{}
	req := relaycommon.TaskSubmitReq{
		Model:  "wan2.7-i2v",
		Prompt: "animate without a frame",
	}

	_, err := adaptor.convertToAliRequest(testRelayInfo(), req)

	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "requires image"))
}

func TestConvertToAliRequestWan25I2VKeepsLegacyImgURL(t *testing.T) {
	adaptor := &TaskAdaptor{}
	req := relaycommon.TaskSubmitReq{
		Model:  "wan2.5-i2v-preview",
		Prompt: "animate the first frame",
		Image:  "https://example.com/first.png",
	}

	aliReq, err := adaptor.convertToAliRequest(testRelayInfo(), req)

	require.NoError(t, err)
	require.Equal(t, "https://example.com/first.png", aliReq.Input.ImgURL)
	require.Empty(t, aliReq.Input.Media)

	body, err := common.Marshal(aliReq)
	require.NoError(t, err)
	require.Contains(t, string(body), `"img_url"`)
	require.NotContains(t, string(body), `"media"`)
}

func TestWan27T2VUsesNativeResolutionRatioAudioAndFlatParameters(t *testing.T) {
	watermark := false
	seed := int64(42)
	aliReq, err := (&TaskAdaptor{}).convertToAliRequest(testRelayInfo(), relaycommon.TaskSubmitReq{
		Model: "wan2.7-t2v", Prompt: "a detective story", Audios: []string{"https://example.com/dialogue.mp3"},
		Resolution: "720p", AspectRatio: "4:3", Duration: 15, Seed: &seed,
		Metadata: map[string]any{"negative_prompt": "blur", "prompt_extend": false, "watermark": watermark},
	})
	require.NoError(t, err)
	assert.Equal(t, "https://example.com/dialogue.mp3", aliReq.Input.AudioURL)
	assert.Equal(t, "blur", aliReq.Input.NegativePrompt)
	assert.Equal(t, "720P", aliReq.Parameters.Resolution)
	require.NotNil(t, aliReq.Parameters.Ratio)
	assert.Equal(t, "4:3", *aliReq.Parameters.Ratio)
	require.NotNil(t, aliReq.Parameters.Duration)
	assert.Equal(t, 15, *aliReq.Parameters.Duration)
	assert.False(t, aliReq.Parameters.PromptExtend)
	require.NotNil(t, aliReq.Parameters.Watermark)
	assert.False(t, *aliReq.Parameters.Watermark)
	assert.Equal(t, 42, aliReq.Parameters.Seed)
}

func TestWan27I2VSupportsEveryOfficialMediaCombination(t *testing.T) {
	tests := []struct {
		name, mode                                                 string
		images, videos, audios, imageRoles, videoRoles, audioRoles []string
		expected                                                   []AliVideoMedia
	}{
		{name: "first", mode: "first_frame", images: []string{"first"}, imageRoles: []string{"first_frame"}, expected: []AliVideoMedia{{Type: "first_frame", URL: "first"}}},
		{name: "first audio", mode: "first_frame", images: []string{"first"}, audios: []string{"voice"}, imageRoles: []string{"first_frame"}, audioRoles: []string{"driving_audio"}, expected: []AliVideoMedia{{Type: "first_frame", URL: "first"}, {Type: "driving_audio", URL: "voice"}}},
		{name: "first last", mode: "first_last_frame", images: []string{"first", "last"}, imageRoles: []string{"first_frame", "last_frame"}, expected: []AliVideoMedia{{Type: "first_frame", URL: "first"}, {Type: "last_frame", URL: "last"}}},
		{name: "first last audio", mode: "first_last_frame", images: []string{"first", "last"}, audios: []string{"voice"}, imageRoles: []string{"first_frame", "last_frame"}, audioRoles: []string{"driving_audio"}, expected: []AliVideoMedia{{Type: "first_frame", URL: "first"}, {Type: "last_frame", URL: "last"}, {Type: "driving_audio", URL: "voice"}}},
		{name: "clip", mode: "video_extension", videos: []string{"clip"}, videoRoles: []string{"first_clip"}, expected: []AliVideoMedia{{Type: "first_clip", URL: "clip"}}},
		{name: "clip last", mode: "video_extension", images: []string{"last"}, videos: []string{"clip"}, imageRoles: []string{"last_frame"}, videoRoles: []string{"first_clip"}, expected: []AliVideoMedia{{Type: "first_clip", URL: "clip"}, {Type: "last_frame", URL: "last"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			aliReq, err := (&TaskAdaptor{}).convertToAliRequest(testRelayInfo(), relaycommon.TaskSubmitReq{Model: "wan2.7-i2v", Mode: tt.mode, Images: tt.images, Videos: tt.videos, Audios: tt.audios, ImageRoles: tt.imageRoles, VideoRoles: tt.videoRoles, AudioRoles: tt.audioRoles, Resolution: "720p", Duration: 10})
			require.NoError(t, err)
			assert.Equal(t, tt.expected, aliReq.Input.Media)
			require.Nil(t, aliReq.Parameters.Ratio, "I2V must follow source aspect ratio")
		})
	}
}

func TestWan27R2VAttachesReferenceVoicesAndEnforcesVideoDuration(t *testing.T) {
	req := relaycommon.TaskSubmitReq{Model: "wan2.7-r2v", Mode: "reference", Prompt: "Image 1 and Video 1 perform", Images: []string{"image"}, Videos: []string{"video"}, Audios: []string{"image-voice", "video-voice"}, ImageRoles: []string{"general_reference"}, VideoRoles: []string{"general_reference"}, Resolution: "1080p", AspectRatio: "16:9", Duration: 10}
	aliReq, err := (&TaskAdaptor{}).convertToAliRequest(testRelayInfo(), req)
	require.NoError(t, err)
	assert.Equal(t, []AliVideoMedia{{Type: "reference_image", URL: "image", ReferenceVoice: "image-voice"}, {Type: "reference_video", URL: "video", ReferenceVoice: "video-voice"}}, aliReq.Input.Media)
	req.Duration = 11
	_, err = (&TaskAdaptor{}).convertToAliRequest(testRelayInfo(), req)
	require.ErrorContains(t, err, "at most 10 seconds")
}

func TestWan27VideoEditUsesSourceAndReferences(t *testing.T) {
	generateAudio := false
	aliReq, err := (&TaskAdaptor{}).convertToAliRequest(testRelayInfo(), relaycommon.TaskSubmitReq{Model: "wan2.7-videoedit", Mode: "video_edit", Prompt: "replace the coat", Videos: []string{"source"}, Images: []string{"coat"}, Resolution: "720p", Duration: 0, GenerateAudio: &generateAudio})
	require.NoError(t, err)
	assert.Equal(t, []AliVideoMedia{{Type: "video", URL: "source"}, {Type: "reference_image", URL: "coat"}}, aliReq.Input.Media)
	require.NotNil(t, aliReq.Parameters.AudioSetting)
	assert.Equal(t, "origin", *aliReq.Parameters.AudioSetting)
	require.NotNil(t, aliReq.Parameters.Duration)
	assert.Equal(t, 0, *aliReq.Parameters.Duration)
	body, err := common.Marshal(aliReq)
	require.NoError(t, err)
	assert.Contains(t, string(body), `"duration":0`)
}

func TestWan27BillingConverter(t *testing.T) {
	generateAudio := false
	params, err := convertAliWan27VideoBillingParams(relaycommon.TaskSubmitReq{
		Model: "wan2.7-r2v", Resolution: "720P", Duration: 10, GenerateAudio: &generateAudio,
	})
	require.NoError(t, err)
	assert.Equal(t, "720p", params.Tier)
	assert.Equal(t, 10, params.DurationSeconds)
	assert.False(t, params.AudioEnabled)

	params, err = convertAliWan27VideoBillingParams(relaycommon.TaskSubmitReq{
		Model: "wan2.7-videoedit", Resolution: "1080p", Duration: 0,
	})
	require.NoError(t, err)
	assert.Equal(t, "1080p", params.Tier)
	assert.Equal(t, 5, params.DurationSeconds)
	assert.True(t, params.AudioEnabled)
}

func TestHappyHorseMapsUnifiedVideoOptions(t *testing.T) {
	generateAudio := false
	tests := []struct {
		name        string
		model       string
		images      []string
		expectRatio bool
	}{
		{name: "text to video", model: "happyhorse-1.1-t2v", expectRatio: true},
		{name: "image to video", model: "happyhorse-1.1-i2v", images: []string{"https://example.com/first.png"}},
		{name: "reference to video", model: "happyhorse-1.1-r2v", images: []string{"https://example.com/reference.png"}, expectRatio: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := relaycommon.TaskSubmitReq{
				Model:         test.model,
				Prompt:        "generate a video",
				Images:        test.images,
				Resolution:    "720p",
				AspectRatio:   "16:9",
				Duration:      5,
				GenerateAudio: &generateAudio,
			}

			aliReq := (&TaskAdaptor{}).buildHappyHorseRequest(test.model, req)
			body, err := common.Marshal(aliReq)
			require.NoError(t, err)

			require.NotNil(t, aliReq.Parameters.Audio)
			assert.False(t, *aliReq.Parameters.Audio)
			require.NotNil(t, aliReq.Parameters.Watermark)
			assert.False(t, *aliReq.Parameters.Watermark)
			assert.Contains(t, string(body), `"audio":false`)
			assert.Contains(t, string(body), `"watermark":false`)
			if test.expectRatio {
				require.NotNil(t, aliReq.Parameters.Ratio)
				assert.Equal(t, "16:9", *aliReq.Parameters.Ratio)
				assert.Nil(t, aliReq.Parameters.AspectRatio)
				assert.Contains(t, string(body), `"ratio":"16:9"`)
				assert.NotContains(t, string(body), `"aspect_ratio"`)
			} else {
				require.NotNil(t, aliReq.Parameters.AspectRatio)
				assert.Equal(t, "16:9", *aliReq.Parameters.AspectRatio)
				assert.Nil(t, aliReq.Parameters.Ratio)
				assert.Contains(t, string(body), `"aspect_ratio":"16:9"`)
				assert.NotContains(t, string(body), `"ratio"`)
			}
		})
	}
}

func TestKlingMapsCanonicalVideoOptions(t *testing.T) {
	audio := false
	req := relaycommon.TaskSubmitReq{
		Model: "kling/kling-v3-video-generation", Mode: "text_to_video", Prompt: "move",
		Resolution: "720p", AspectRatio: "9:16", Duration: 3, GenerateAudio: &audio,
		Metadata: map[string]any{"aspect_ratio": "1:1", "audio": true},
	}

	aliReq, err := (&TaskAdaptor{}).buildKlingRequest(req.Model, req)

	require.NoError(t, err)
	require.NotNil(t, aliReq.Parameters.Mode)
	assert.Equal(t, "pro", *aliReq.Parameters.Mode)
	require.NotNil(t, aliReq.Parameters.AspectRatio)
	assert.Equal(t, "9:16", *aliReq.Parameters.AspectRatio)
	require.NotNil(t, aliReq.Parameters.Audio)
	assert.False(t, *aliReq.Parameters.Audio)
}

func TestHappyHorseUnifiedOptionsOverrideLegacyMetadata(t *testing.T) {
	generateAudio := false
	req := relaycommon.TaskSubmitReq{
		Model:         "happyhorse-1.1-t2v",
		AspectRatio:   "16:9",
		GenerateAudio: &generateAudio,
		Metadata: map[string]any{
			"ratio":         "9:16",
			"generateAudio": true,
			"watermark":     true,
		},
	}

	aliReq := (&TaskAdaptor{}).buildHappyHorseRequest(req.Model, req)

	require.NotNil(t, aliReq.Parameters.Ratio)
	assert.Equal(t, "16:9", *aliReq.Parameters.Ratio)
	require.NotNil(t, aliReq.Parameters.Audio)
	assert.False(t, *aliReq.Parameters.Audio)
	require.NotNil(t, aliReq.Parameters.Watermark)
	assert.True(t, *aliReq.Parameters.Watermark)
}

func TestHappyHorseSupportsLegacyI2VMetadata(t *testing.T) {
	req := relaycommon.TaskSubmitReq{
		Model:  "happyhorse-1.1-i2v",
		Images: []string{"https://example.com/first.png"},
		Metadata: map[string]any{
			"aspectRatio":   "9:16",
			"generateAudio": false,
		},
	}

	aliReq := (&TaskAdaptor{}).buildHappyHorseRequest(req.Model, req)

	require.NotNil(t, aliReq.Parameters.AspectRatio)
	assert.Equal(t, "9:16", *aliReq.Parameters.AspectRatio)
	require.NotNil(t, aliReq.Parameters.Audio)
	assert.False(t, *aliReq.Parameters.Audio)
}
