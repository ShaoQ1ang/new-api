package service

import (
	"context"
	"testing"

	"github.com/QuantumNous/new-api/aigc/dto"
	"github.com/QuantumNous/new-api/aigc/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerationResolverSelectsVideoOutputTarget(t *testing.T) {
	profile := entity.ModelProfile{
		PublicModelID: "video-pro", DisplayName: "Video Pro", ModelType: "video", Status: entity.ModelStatusPublished,
		GroupsJSON: `["vip"]`, ConfigVersion: 4, ConfigJSON: `{
			"video":{"adapter":"video-task","task_protocol":"newapi-video",
				"modes":{"first_frame":{"upstream_model_id":"video-i2v","inputs":{"first_frame":{"min":1,"max":1}}}},
				"output_specs":[{"id":"hq","modes":["first_frame"],"resolutions":["1080p"],"aspect_ratios":["16:9"],"durations":[5],"generate_audio":{"supported":true,"default":true},"target":{"upstream_model_id":"video-i2v-hq"}}]
			}
		}`,
	}
	store := &profileStoreStub{profiles: map[string]*entity.ModelProfile{profile.PublicModelID: &profile}}
	availability := &availabilityStub{byGroup: map[string]map[string]bool{"vip": {"video-i2v": true, "video-i2v-hq": true}}}
	resolver := NewGenerationResolver(store, availability)
	audio := true

	spec, err := resolver.Resolve(context.Background(), "vip", dto.GenerationRequest{
		IdempotencyKey: "turn-1", Model: "video-pro", Type: "video", Prompt: "move",
		Mode: "first_frame", Inputs: dto.GenerationInputs{Images: []dto.MediaInput{{Role: "first_frame", URL: "https://aigc.test/first.png"}}},
		Output: dto.GenerationOutput{Resolution: "1080p", AspectRatio: "16:9", Duration: 5, GenerateAudio: &audio},
	})

	require.NoError(t, err)
	assert.Equal(t, "video-i2v-hq", spec.UpstreamModelID)
	assert.Equal(t, "video-task", spec.Adapter)
	assert.Equal(t, "newapi-video", spec.TaskProtocol)
	assert.Equal(t, "hq", spec.OutputSpecID)
	assert.Equal(t, 4, spec.ConfigVersion)
}

func TestGenerationResolverRejectsInvalidInputRole(t *testing.T) {
	profile := entity.ModelProfile{
		PublicModelID: "image-edit", DisplayName: "Image", ModelType: "image", Status: entity.ModelStatusPublished,
		GroupsJSON: `[]`, ConfigVersion: 1, ConfigJSON: `{
			"image":{"adapter":"image-relay","modes":{"image_edit":{
				"upstream_model_id":"image-upstream","input":{"role":"source_image","min":1,"max":2},
				"output":{"sizes":["1024x1024"],"counts":[1],"default_size":"1024x1024","default_count":1}
			}}}
		}`,
	}
	store := &profileStoreStub{profiles: map[string]*entity.ModelProfile{profile.PublicModelID: &profile}}
	availability := &availabilityStub{byGroup: map[string]map[string]bool{"default": {"image-upstream": true}}}
	resolver := NewGenerationResolver(store, availability)

	_, err := resolver.Resolve(context.Background(), "default", dto.GenerationRequest{
		IdempotencyKey: "turn-2", Model: "image-edit", Type: "image", Prompt: "edit", Mode: "image_edit",
		Inputs: dto.GenerationInputs{Images: []dto.MediaInput{{Role: "first_frame", URL: "https://aigc.test/image.png"}}},
		Output: dto.GenerationOutput{Size: "1024x1024", Count: 1},
	})

	assertGenerationErrorCode(t, err, "INVALID_INPUT_ROLE")
}

func TestGenerationResolverTreatsFirstFrameAsIntrinsicModeInput(t *testing.T) {
	profile := entity.ModelProfile{
		PublicModelID: "bad-video", DisplayName: "Bad Video", ModelType: "video", Status: entity.ModelStatusPublished,
		GroupsJSON: `[]`, ConfigVersion: 1, ConfigJSON: `{
			"video":{"adapter":"video-task","task_protocol":"newapi-video",
				"modes":{"first_frame":{"upstream_model_id":"video-i2v"}},
				"output_specs":[{"id":"base","modes":["first_frame"],"resolutions":["720p"],"aspect_ratios":["16:9"],"durations":[5],"generate_audio":{"supported":false,"default":false}}]
			}
		}`,
	}
	store := &profileStoreStub{profiles: map[string]*entity.ModelProfile{profile.PublicModelID: &profile}}
	resolver := NewGenerationResolver(store, &availabilityStub{byGroup: map[string]map[string]bool{"default": {"video-i2v": true}}})

	spec, err := resolver.Resolve(context.Background(), "default", dto.GenerationRequest{
		IdempotencyKey: "turn-bad", Model: profile.PublicModelID, Type: "video", Prompt: "move", Mode: "first_frame",
		Inputs: dto.GenerationInputs{Images: []dto.MediaInput{{Role: "first_frame", URL: "https://aigc.test/first.png"}}},
		Output: dto.GenerationOutput{Resolution: "720p", AspectRatio: "16:9", Duration: 5},
	})

	require.NoError(t, err)
	assert.Equal(t, "first_frame", spec.Mode)
	assert.Equal(t, "video-i2v", spec.UpstreamModelID)
}

func TestGenerationResolverDerivesAdaptiveSunoMode(t *testing.T) {
	profile := entity.ModelProfile{
		PublicModelID: "suno-v5.5", DisplayName: "Suno V5.5", ModelType: "music", Status: entity.ModelStatusPublished,
		GroupsJSON: `[]`, ConfigVersion: 2, ConfigJSON: `{
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
		}`,
	}
	resolver := NewGenerationResolver(
		&profileStoreStub{profiles: map[string]*entity.ModelProfile{profile.PublicModelID: &profile}},
		&availabilityStub{byGroup: map[string]map[string]bool{"default": {"V5_5": true}}},
	)

	quick, err := resolver.Resolve(context.Background(), "default", dto.GenerationRequest{
		IdempotencyKey: "turn-quick", Model: profile.PublicModelID, Type: "music", Prompt: "rainy night electronic pop",
	})
	require.NoError(t, err)
	assert.False(t, quick.MusicCustomMode)
	assert.Equal(t, "sunoapi-v1", quick.TaskProtocol)
	assert.Equal(t, "V5_5", quick.UpstreamModelID)

	zero := 0.0
	custom, err := resolver.Resolve(context.Background(), "default", dto.GenerationRequest{
		IdempotencyKey: "turn-custom", Model: profile.PublicModelID, Type: "music", Prompt: "rainy night electronic pop",
		Parameters: dto.GenerationParameters{Lyrics: "[Verse]\nNeon falls", AudioWeight: &zero},
	})
	require.NoError(t, err)
	assert.True(t, custom.MusicCustomMode)
	assert.Equal(t, "rainy night electronic pop", custom.Request.Parameters.Style)
	assert.Equal(t, "rainy night electronic pop", custom.Request.Parameters.Title)
	require.NotNil(t, custom.Request.Parameters.AudioWeight)
	assert.Zero(t, *custom.Request.Parameters.AudioWeight)
}

func TestGenerationResolverRequiresLyricsForCustomVocalGeneration(t *testing.T) {
	profile := entity.ModelProfile{
		PublicModelID: "suno-v5", DisplayName: "Suno V5", ModelType: "music", Status: entity.ModelStatusPublished,
		GroupsJSON: `[]`, ConfigVersion: 1, ConfigJSON: `{
			"music":{"adapter":"sunoapi-music","task_protocol":"sunoapi-v1","modes":{"text_to_music":{
				"upstream_model_id":"V5","parameters":{"instrumental":{"supported":true,"default":false},
				"exact_lyrics":{"supported":true,"max_length":5000},"style":{"supported":true,"max_length":1000},
				"title":{"supported":true,"max_length":100}},"output":{"min_tracks":2,"max_tracks":2}
			}}}
		}`,
	}
	resolver := NewGenerationResolver(
		&profileStoreStub{profiles: map[string]*entity.ModelProfile{profile.PublicModelID: &profile}},
		&availabilityStub{byGroup: map[string]map[string]bool{"default": {"V5": true}}},
	)

	_, err := resolver.Resolve(context.Background(), "default", dto.GenerationRequest{
		IdempotencyKey: "turn-invalid", Model: profile.PublicModelID, Type: "music", Prompt: "electronic pop",
		Parameters: dto.GenerationParameters{Style: "electronic"},
	})

	assertGenerationErrorCode(t, err, "MUSIC_LYRICS_REQUIRED")
}

func TestGenerationResolverRejectsUnavailableOrHiddenModel(t *testing.T) {
	profile := validTextProfile(entity.ModelStatusPublished)
	profile.GroupsJSON = `["vip"]`
	store := &profileStoreStub{profiles: map[string]*entity.ModelProfile{profile.PublicModelID: profile}}
	resolver := NewGenerationResolver(store, &availabilityStub{byGroup: map[string]map[string]bool{"vip": {"gpt-5": false}}})
	request := dto.GenerationRequest{IdempotencyKey: "turn-3", Model: profile.PublicModelID, Type: "text", Prompt: "hello"}

	_, err := resolver.Resolve(context.Background(), "default", request)
	assertGenerationErrorCode(t, err, "MODEL_NOT_AVAILABLE_FOR_GROUP")

	_, err = resolver.Resolve(context.Background(), "vip", request)
	assertGenerationErrorCode(t, err, "MODEL_CHANNEL_UNAVAILABLE")
}

func assertGenerationErrorCode(t *testing.T, err error, code string) {
	t.Helper()
	require.Error(t, err)
	var protocolErr *GenerationError
	require.ErrorAs(t, err, &protocolErr)
	assert.Equal(t, code, protocolErr.Code)
}
