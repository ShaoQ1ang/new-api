package taskcommon_test

import (
	"testing"

	_ "github.com/QuantumNous/new-api/relay/channel/task/openrouter"
	"github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenRouterBillingRegistryUsesMappedUpstreamModel(t *testing.T) {
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{
		UpstreamModelName: "google/veo-3.1-lite",
		IsModelMapped:     true,
	}}
	req := relaycommon.TaskSubmitReq{
		Model: "my-veo", Seconds: "7", Size: "720p",
		Metadata: map[string]any{"generate_audio": false},
	}

	params, err := taskcommon.ConvertVideoBillingParams(info, req)

	require.NoError(t, err)
	assert.Equal(t, "720p", params.Tier)
	assert.Equal(t, 6, params.DurationSeconds)
	assert.False(t, params.AudioEnabled)
}

func TestOpenRouterVideoFamilyBillingConverters(t *testing.T) {
	tests := []struct {
		name     string
		model    string
		seconds  string
		size     string
		metadata map[string]any
		tier     string
		duration int
		audio    bool
	}{
		{name: "happyhorse", model: "alibaba/happyhorse-1.1", seconds: "10", size: "1080p", tier: "1080p", duration: 10},
		{name: "kling silent", model: "kwaivgi/kling-v3.0-std", seconds: "5", metadata: map[string]any{"audio": false}, tier: "720p", duration: 5},
		{name: "kling audio default", model: "kwaivgi/kling-v3.0-pro", seconds: "5", tier: "720p", duration: 5, audio: true},
		{name: "minimax h3", model: "minimax/hailuo-3", seconds: "8", tier: "2k", duration: 8, audio: true},
		{name: "minimax hailuo 2.3", model: "minimax/hailuo-2.3", seconds: "10", tier: "1080p", duration: 10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params, err := taskcommon.ConvertVideoBillingParams(nil, relaycommon.TaskSubmitReq{
				Model: tt.model, Prompt: "test", Seconds: tt.seconds, Size: tt.size, Metadata: tt.metadata,
			})
			require.NoError(t, err)
			assert.Equal(t, tt.tier, params.Tier)
			assert.Equal(t, tt.duration, params.DurationSeconds)
			assert.Equal(t, tt.audio, params.AudioEnabled)
		})
	}
}

func TestMiniMaxH3BillingOnlyCountsInputReferenceImages(t *testing.T) {
	tests := []struct {
		name     string
		request  relaycommon.TaskSubmitReq
		expected map[string]int
	}{
		{
			name: "frame images are not reference images",
			request: relaycommon.TaskSubmitReq{
				Images: []string{"https://example.com/first.png", "https://example.com/last.png"},
			},
		},
		{
			name: "first five reference images are free",
			request: relaycommon.TaskSubmitReq{
				Metadata: map[string]any{
					"reference_images": []string{
						"https://example.com/reference-1.png",
						"https://example.com/reference-2.png",
						"https://example.com/reference-3.png",
						"https://example.com/reference-4.png",
						"https://example.com/reference-5.png",
					},
				},
			},
		},
		{
			name: "only reference images after the first five are billable",
			request: relaycommon.TaskSubmitReq{
				Metadata: map[string]any{
					"input_references": []map[string]any{
						{"type": "image_url", "image_url": map[string]any{"url": "https://example.com/reference-1.png"}},
						{"type": "image_url", "image_url": map[string]any{"url": "https://example.com/reference-2.png"}},
						{"type": "image_url", "image_url": map[string]any{"url": "https://example.com/reference-3.png"}},
						{"type": "image_url", "image_url": map[string]any{"url": "https://example.com/reference-4.png"}},
						{"type": "image_url", "image_url": map[string]any{"url": "https://example.com/reference-5.png"}},
						{"type": "image_url", "image_url": map[string]any{"url": "https://example.com/reference-6.png"}},
					},
				},
			},
			expected: map[string]int{"reference_image": 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.request.Model = "minimax/hailuo-3"
			tt.request.Prompt = "test"
			tt.request.Duration = 5
			params, err := taskcommon.ConvertVideoBillingParams(nil, tt.request)

			require.NoError(t, err)
			assert.Equal(t, tt.expected, params.ExtraUnits)
		})
	}
}
