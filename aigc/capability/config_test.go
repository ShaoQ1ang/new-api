package capability

import (
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
					"output": {"sizes":["1024x1024"],"counts":[1,2],"default_size":"1024x1024","default_count":1}
				},
				"image_edit": {
					"upstream_model_id": "gemini-image",
					"input": {"role":"source_image","min":1,"max":4,"accept":["image/png"]},
					"output": {"sizes":["1024x1024"],"counts":[1],"default_size":"1024x1024","default_count":1}
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
