package capability

import (
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseImageConfigReturnsUniqueUpstreamModels(t *testing.T) {
	config, err := Parse(ModelTypeImage, []byte(`{
		"image": {
			"adapter": "image-relay",
			"modes": {
				"text_to_image": {
					"upstream_model_id": "gemini-image",
					"output": {"sizes":["1024x1024"],"size_tiers":{"1024x1024":"1k"},"counts":[1,2],"default_size":"1024x1024","default_count":1}
				},
				"image_edit": {
					"upstream_model_id": "gemini-image",
					"input": {"role":"source_image","min":1,"max":4,"accept":["image/png"]},
					"output": {"sizes":["1024x1024"],"size_tiers":{"1024x1024":"1k"},"counts":[1],"default_size":"1024x1024","default_count":1}
				}
			}
		}
	}`))

	require.NoError(t, err)
	assert.Equal(t, []string{"gemini-image"}, config.UpstreamModelIDs())
}

func TestParseRejectsImageOutputDefaultOutsideAllowedValues(t *testing.T) {
	_, err := Parse(ModelTypeImage, []byte(`{
		"image": {
			"adapter": "image-relay",
			"modes": {
				"text_to_image": {
					"upstream_model_id": "image-model",
					"output": {"sizes":["1024x1024"],"counts":[1],"default_size":"2048x2048","default_count":1}
				}
			}
		}
	}`))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "default size")
}

func TestParseAssignsExactImageSizeTiers(t *testing.T) {
	tests := []struct {
		name       string
		upstreamID string
		sizes      string
		want       map[string]string
	}{
		{
			name: "studio matrix", upstreamID: "qwen-image-2.0",
			sizes: `"1024x576","2496x1664","4672x3504"`,
			want:  map[string]string{"1024x576": "1k", "2496x1664": "2k", "4672x3504": "4k"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defaultSize := strings.Split(tt.sizes, ",")[0]
			raw := `{"image":{"adapter":"openai-image","modes":{"text_to_image":{"upstream_model_id":"` + tt.upstreamID + `","output":{"sizes":[` + tt.sizes + `],"counts":[1],"default_size":` + defaultSize + `,"default_count":1}}}}}`
			config, err := Parse(ModelTypeImage, []byte(raw))
			require.NoError(t, err)
			assert.Equal(t, tt.want, config.Image.Modes["text_to_image"].Output.SizeTiers)
		})
	}
}

func TestStudioImagePresetCoversSevenRatiosAtEveryTier(t *testing.T) {
	want := map[string]map[string]string{
		"16:9": {"1k": "1024x576", "2k": "2560x1440", "4k": "5376x3024"},
		"3:2":  {"1k": "1008x672", "2k": "2496x1664", "4k": "4992x3328"},
		"4:3":  {"1k": "1024x768", "2k": "2304x1728", "4k": "4672x3504"},
		"1:1":  {"1k": "1024x1024", "2k": "2048x2048", "4k": "4096x4096"},
		"3:4":  {"1k": "768x1024", "2k": "1728x2304", "4k": "3504x4672"},
		"2:3":  {"1k": "672x1008", "2k": "1664x2496", "4k": "3328x4992"},
		"9:16": {"1k": "576x1024", "2k": "1440x2560", "4k": "3024x5376"},
	}

	assert.Equal(t, want, studioImageSizeMatrix)
	assert.Len(t, studioImageSizeTiers, 21)
	for ratio, tiers := range want {
		assert.Lenf(t, tiers, 3, "ratio %s", ratio)
		for tier, size := range tiers {
			assert.Equalf(t, tier, studioImageSizeTiers[size], "ratio %s size %s", ratio, size)
		}
		widthText, heightText, ok := strings.Cut(tiers["2k"], "x")
		require.True(t, ok)
		width, err := strconv.Atoi(widthText)
		require.NoError(t, err)
		height, err := strconv.Atoi(heightText)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, width*height, 3_686_400)
		assert.LessOrEqual(t, width*height, 2_048*2_048)

		widthText, heightText, ok = strings.Cut(tiers["4k"], "x")
		require.True(t, ok)
		width, err = strconv.Atoi(widthText)
		require.NoError(t, err)
		height, err = strconv.Atoi(heightText)
		require.NoError(t, err)
		ratioWidthText, ratioHeightText, ok := strings.Cut(ratio, ":")
		require.True(t, ok)
		ratioWidth, err := strconv.Atoi(ratioWidthText)
		require.NoError(t, err)
		ratioHeight, err := strconv.Atoi(ratioHeightText)
		require.NoError(t, err)
		assert.Equalf(t, width*ratioHeight, height*ratioWidth, "4k size %s must preserve ratio %s", tiers["4k"], ratio)
		assert.Zero(t, width%16)
		assert.Zero(t, height%16)
		assert.LessOrEqual(t, width*height, 4096*4096)
	}
}

func TestStudioImageMatrixAppliesToAnyModel(t *testing.T) {
	config, err := Parse(ModelTypeImage, []byte(`{"image":{"adapter":"openai-image","modes":{"text_to_image":{"upstream_model_id":"custom-provider/model","output":{"sizes":["1024x576","2304x1728","3024x5376"],"counts":[1],"default_size":"1024x576","default_count":1}}}}}`))
	require.NoError(t, err)
	assert.Equal(t, map[string]string{
		"1024x576":  "1k",
		"2304x1728": "2k",
		"3024x5376": "4k",
	}, config.Image.Modes["text_to_image"].Output.SizeTiers)
}

func TestParseRequiresTierForUnknownImageSize(t *testing.T) {
	_, err := Parse(ModelTypeImage, []byte(`{"image":{"adapter":"openai-image","modes":{"text_to_image":{"upstream_model_id":"custom-image","output":{"sizes":["2304x1792"],"counts":[1],"default_size":"2304x1792","default_count":1}}}}}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "has no resolution tier")

	config, err := Parse(ModelTypeImage, []byte(`{"image":{"adapter":"openai-image","modes":{"text_to_image":{"upstream_model_id":"custom-image","output":{"sizes":["2304x1792"],"size_tiers":{"2304x1792":"2K"},"counts":[1],"default_size":"2304x1792","default_count":1}}}}}`))
	require.NoError(t, err)
	assert.Equal(t, "2k", config.Image.Modes["text_to_image"].Output.SizeTiers["2304x1792"])

	_, err = Parse(ModelTypeImage, []byte(`{"image":{"adapter":"openai-image","modes":{"text_to_image":{"upstream_model_id":"custom-image","output":{"sizes":["1024x1024"],"size_tiers":{"1024x1024":"2k","2048x2048":"2k"},"counts":[1],"default_size":"1024x1024","default_count":1}}}}}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "image size tiers must match output sizes exactly")

	_, err = Parse(ModelTypeImage, []byte(`{"image":{"adapter":"openai-image","modes":{"text_to_image":{"upstream_model_id":"custom-image","output":{"sizes":["1024x1024"],"size_tiers":{"1024x1024":"3k"},"counts":[1],"default_size":"1024x1024","default_count":1}}}}}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid resolution tier")
}

func TestParseRejectsOverlappingVideoOutputRoutes(t *testing.T) {
	_, err := Parse(ModelTypeVideo, []byte(`{
		"video": {
			"adapter": "video-task",
			"task_protocol": "newapi-video",
			"modes": {
				"text_to_video": {"upstream_model_id":"wan2.7-t2v"}
			},
			"output_specs": [
				{"id":"base","modes":["text_to_video"],"resolutions":["720p"],"aspect_ratios":["16:9"],"durations":[5],"generate_audio":{"supported":false,"default":false}},
				{"id":"duplicate","modes":["text_to_video"],"resolutions":["720p"],"aspect_ratios":["16:9"],"durations":[5],"generate_audio":{"supported":false,"default":false}}
			]
		}
	}`))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "overlapping video route")
}

func TestParseVideoConfigIncludesModeAndOutputTargets(t *testing.T) {
	config, err := Parse(ModelTypeVideo, []byte(`{
		"video": {
			"adapter": "video-task",
			"task_protocol": "newapi-video",
			"modes": {
				"text_to_video": {"upstream_model_id":"wan2.7-t2v"},
				"first_frame": {"upstream_model_id":"wan2.7-i2v","inputs":{"first_frame":{"min":1,"max":1}}}
			},
			"output_specs": [
				{"id":"standard","modes":["text_to_video","first_frame"],"resolutions":["720p"],"aspect_ratios":["16:9"],"durations":[5],"generate_audio":{"supported":true,"default":true}},
				{"id":"1080p","modes":["text_to_video"],"resolutions":["1080p"],"aspect_ratios":["16:9"],"durations":[5],"generate_audio":{"supported":true,"default":true},"target":{"upstream_model_id":"wan2.7-t2v-hq"}}
			]
		}
	}`))

	require.NoError(t, err)
	assert.Equal(t, []string{"wan2.7-i2v", "wan2.7-t2v", "wan2.7-t2v-hq"}, config.UpstreamModelIDs())
}

func TestParseRejectsConfigurationForAnotherModelType(t *testing.T) {
	_, err := Parse(ModelTypeMusic, []byte(`{
		"image": {
			"adapter": "image-relay",
			"modes": {}
		}
	}`))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "music configuration is required")
}

func TestParseTextAndMusicConfigurations(t *testing.T) {
	tests := []struct {
		name      string
		modelType ModelType
		raw       string
		upstreams []string
	}{
		{
			name:      "text",
			modelType: ModelTypeText,
			raw:       `{"text":{"upstream_model_id":"gpt-5","max_output_tokens":8192}}`,
			upstreams: []string{"gpt-5"},
		},
		{
			name:      "music",
			modelType: ModelTypeMusic,
			raw:       `{"music":{"adapter":"music-task","modes":{"text_to_music":{"upstream_model_id":"suno-v4","parameters":{"instrumental":{"supported":true,"default":false}},"output":{"min_tracks":1,"max_tracks":2}}}}}`,
			upstreams: []string{"suno-v4"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := Parse(tt.modelType, []byte(tt.raw))
			require.NoError(t, err)
			assert.Equal(t, tt.upstreams, config.UpstreamModelIDs())
		})
	}
}

func TestParseCompleteSunoAPIMusicConfiguration(t *testing.T) {
	config, err := Parse(ModelTypeMusic, []byte(`{
		"music":{"adapter":"sunoapi-music","task_protocol":"sunoapi-v1","modes":{"text_to_music":{
			"upstream_model_id":"V5_5","parameters":{
				"instrumental":{"supported":true,"default":false,"configurable":true},
				"exact_lyrics":{"supported":true,"max_length":5000},
				"style":{"supported":true,"max_length":1000},
				"title":{"supported":true,"max_length":100},
				"persona":{"supported":true,"voice_persona_supported":true},
				"duration":{"supported":true,"min":10,"max":360},
				"negative_tags":{"supported":true,"max_length":1000},
				"vocal_gender":{"supported":true},"advanced_weights":{"supported":true}
			},"output":{"min_tracks":2,"max_tracks":2}
		}}}
	}`))

	require.NoError(t, err)
	require.NotNil(t, config.Music)
	assert.Equal(t, "sunoapi-v1", config.Music.TaskProtocol)
	assert.Equal(t, 360, config.Music.Modes["text_to_music"].Parameters.Duration.Max)
}

func TestParseRejectsInconsistentSunoAPIMusicConfiguration(t *testing.T) {
	tests := []struct {
		name      string
		musicJSON string
		message   string
	}{
		{
			name:      "wrong adapter",
			musicJSON: `{"adapter":"music-task","task_protocol":"sunoapi-v1","modes":{"text_to_music":{"upstream_model_id":"V5","parameters":{"instrumental":{"supported":true}},"output":{"min_tracks":2,"max_tracks":2}}}}`,
			message:   "requires the sunoapi-music adapter",
		},
		{
			name:      "wrong output count",
			musicJSON: `{"adapter":"sunoapi-music","task_protocol":"sunoapi-v1","modes":{"text_to_music":{"upstream_model_id":"V5","parameters":{"instrumental":{"supported":true}},"output":{"min_tracks":1,"max_tracks":2}}}}`,
			message:   "requires exactly two output tracks",
		},
		{
			name:      "duration on V5",
			musicJSON: `{"adapter":"sunoapi-music","task_protocol":"sunoapi-v1","modes":{"text_to_music":{"upstream_model_id":"V5","parameters":{"instrumental":{"supported":true},"duration":{"supported":true,"min":10,"max":360}},"output":{"min_tracks":2,"max_tracks":2}}}}`,
			message:   "duration is only supported by V5_5",
		},
		{
			name:      "voice persona on V4",
			musicJSON: `{"adapter":"sunoapi-music","task_protocol":"sunoapi-v1","modes":{"text_to_music":{"upstream_model_id":"V4","parameters":{"instrumental":{"supported":true},"persona":{"supported":true,"voice_persona_supported":true}},"output":{"min_tracks":2,"max_tracks":2}}}}`,
			message:   "voice persona is only supported by V5 and V5_5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(ModelTypeMusic, []byte(`{"music":`+tt.musicJSON+`}`))

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.message)
		})
	}
}
