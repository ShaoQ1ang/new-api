package execution

import (
	"testing"

	aigcdto "github.com/QuantumNous/new-api/aigc/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildTaskRequestPreservesResolvedVideoContract(t *testing.T) {
	audio := false
	spec := Spec{
		PublicModelID: "video-public", UpstreamModelID: "alibaba/wan-2.7", ModelType: "video",
		Mode: "reference", Adapter: "openrouter-video", TaskProtocol: "newapi-video", OutputSpecID: "standard",
		Request: aigcdto.GenerationRequest{
			Prompt: "animate the references", Mode: "reference",
			Inputs: aigcdto.GenerationInputs{
				Images: []aigcdto.MediaInput{{Role: "general_reference", URL: "https://cdn.test/image.png"}, {Role: "first_frame", URL: "https://cdn.test/opening.png"}},
				Videos: []aigcdto.MediaInput{{Role: "general_reference", URL: "https://cdn.test/video.mp4"}},
				Audios: []aigcdto.MediaInput{{Role: "general_reference", URL: "https://cdn.test/audio.mp3"}},
			},
			Output:  aigcdto.GenerationOutput{Resolution: "1080p", AspectRatio: "16:9", Duration: 10, GenerateAudio: &audio},
			Options: aigcdto.GenerationOptions{NegativePrompt: "blur", Enhance: true, Private: true, AIMark: true},
		},
	}

	request, err := BuildTaskRequest(spec)

	require.NoError(t, err)
	assert.Equal(t, "alibaba/wan-2.7", request.Model)
	assert.Equal(t, "reference", request.Mode)
	assert.Equal(t, []string{"https://cdn.test/image.png", "https://cdn.test/opening.png"}, request.Images)
	assert.Equal(t, []string{"general_reference", "first_frame"}, request.ImageRoles)
	assert.Equal(t, []string{"https://cdn.test/video.mp4"}, request.Videos)
	assert.Equal(t, []string{"general_reference"}, request.VideoRoles)
	assert.Equal(t, []string{"https://cdn.test/audio.mp3"}, request.Audios)
	assert.Equal(t, []string{"general_reference"}, request.AudioRoles)
	assert.Equal(t, "1080p", request.Resolution)
	assert.Equal(t, "16:9", request.AspectRatio)
	assert.Equal(t, 10, request.Duration)
	require.NotNil(t, request.GenerateAudio)
	assert.False(t, *request.GenerateAudio)
	assert.Equal(t, map[string]any{
		"negative_prompt": "blur", "enhance": true, "private": true, "ai_mark": true,
	}, request.Metadata)
}

func TestBuildTaskRequestRejectsUnsupportedExecutionSpec(t *testing.T) {
	tests := []struct {
		name string
		spec Spec
	}{
		{name: "model type", spec: Spec{ModelType: "image", UpstreamModelID: "image", TaskProtocol: "newapi-video"}},
		{name: "task protocol", spec: Spec{ModelType: "video", UpstreamModelID: "video", TaskProtocol: "vendor-private"}},
		{name: "upstream model", spec: Spec{ModelType: "video", TaskProtocol: "newapi-video"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := BuildTaskRequest(test.spec)
			require.Error(t, err)
		})
	}
}

func TestBuildMusicTaskRequestPreservesResolvedContract(t *testing.T) {
	spec := Spec{
		UpstreamModelID: "chirp-v4", ModelType: "music", Mode: "text_to_music", Adapter: "music-task",
		Request: aigcdto.GenerationRequest{Prompt: "bright synthwave", Parameters: aigcdto.GenerationParameters{Instrumental: true}},
	}

	request, err := BuildMusicTaskRequest(spec)

	require.NoError(t, err)
	assert.Equal(t, "bright synthwave", request.GptDescriptionPrompt)
	assert.Equal(t, "chirp-v4", request.Mv)
	assert.True(t, request.MakeInstrumental)
}

func TestBuildMusicTaskRequestRejectsUnsupportedSpec(t *testing.T) {
	for _, spec := range []Spec{
		{ModelType: "video", Mode: "text_to_music", UpstreamModelID: "chirp-v4"},
		{ModelType: "music", Mode: "lyrics", UpstreamModelID: "chirp-v4"},
		{ModelType: "music", Mode: "text_to_music"},
	} {
		_, err := BuildMusicTaskRequest(spec)
		require.Error(t, err)
	}
}

func TestBuildSunoAPIV1TaskRequestPreservesResolvedCustomContract(t *testing.T) {
	zero := 0.0
	duration := 120
	spec := Spec{
		UpstreamModelID: "V5_5", ModelType: "music", Mode: "text_to_music", TaskProtocol: TaskProtocolSunoAPIV1, MusicCustomMode: true,
		Request: aigcdto.GenerationRequest{Prompt: "night pop", Parameters: aigcdto.GenerationParameters{
			Lyrics: "exact lyrics", Style: "ambient pop", Title: "Night", Duration: &duration, AudioWeight: &zero,
		}},
	}

	request, err := BuildSunoAPIV1TaskRequest(spec, "https://app.test/api/sunoapi/callback")
	require.NoError(t, err)
	assert.Equal(t, "V5_5", request.Model)
	assert.Equal(t, "exact lyrics", request.Metadata["lyrics"])
	assert.Equal(t, true, request.Metadata["customMode"])
	assert.Equal(t, 0.0, request.Metadata["audioWeight"])
	assert.Equal(t, 120, request.Metadata["duration"])
}
