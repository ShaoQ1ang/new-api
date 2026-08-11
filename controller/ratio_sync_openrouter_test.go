package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertOpenRouterVideoToRatioData(t *testing.T) {
	input := `{
		"data": [
			{
				"id": "kwaivgi/kling-v3.0-std",
				"supported_resolutions": ["720p"],
				"pricing_skus": {
					"duration_seconds": "0.084",
					"duration_seconds_with_audio": "0.126"
				}
			},
			{
				"id": "minimax/hailuo-3",
				"supported_resolutions": ["2K"],
				"pricing_skus": {
					"duration_seconds": "0.13",
					"reference_images": "0.04"
				}
			},
			{
				"id": "resolution-skus",
				"supported_resolutions": ["1280x720", "1920x1080"],
				"pricing_skus": {
					"duration_seconds_720p": "0.1",
					"duration_seconds_with_audio_720p": "0.2",
					"duration_seconds_without_audio_1080p": "0.3",
					"duration_seconds_with_audio_1080p": "0.4",
					"prompt_tokens": "99"
				}
			}
		]
	}`

	converted, err := convertOpenRouterVideoToRatioData(strings.NewReader(input))
	require.NoError(t, err)

	prices, ok := converted["video_seconds_price"].(ratio_setting.VideoSecondsPriceMap)
	require.True(t, ok)
	assert.Equal(t, map[string]map[string]float64{
		"720p": {"silent": 0.084, "default": 0.126},
	}, prices["kwaivgi/kling-v3.0-std"])
	assert.Equal(t, map[string]map[string]float64{
		"2k": {"silent": 0.13, "default": 0.13, "reference_image": 0.04},
	}, prices["minimax/hailuo-3"])
	assert.Equal(t, map[string]map[string]float64{
		"720p":  {"silent": 0.1, "default": 0.2},
		"1080p": {"silent": 0.3, "default": 0.4},
	}, prices["resolution-skus"])

	modes := valueMap(converted[billing_setting.BillingModeField])
	assert.Equal(t, billing_setting.BillingModeVideoSeconds, modes["kwaivgi/kling-v3.0-std"])
	assert.Equal(t, billing_setting.BillingModeVideoSeconds, modes["minimax/hailuo-3"])
	assert.Equal(t, billing_setting.BillingModeVideoSeconds, modes["resolution-skus"])
}

func TestFetchOpenRouterPricingDataUsesGeneralAndVideoEndpoints(t *testing.T) {
	var mu sync.Mutex
	requestedPaths := make([]string, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		mu.Lock()
		requestedPaths = append(requestedPaths, r.URL.Path)
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/models":
			_, _ = w.Write([]byte(`{"data":[{"id":"text-model","pricing":{"prompt":"0.000002","completion":"0.000004"}}]}`))
		case "/v1/videos/models":
			_, _ = w.Write([]byte(`{"data":[{"id":"video-model","supported_resolutions":["720p"],"pricing_skus":{"duration_seconds":"0.1"}}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	converted, err := fetchOpenRouterPricingData(context.Background(), server.Client(), server.URL, "Bearer test-key")
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"/v1/models", "/v1/videos/models"}, requestedPaths)
	assert.Equal(t, 1.0, valueMap(converted["model_ratio"])["text-model"])
	assert.Equal(t, 2.0, valueMap(converted["completion_ratio"])["text-model"])

	prices, ok := converted["video_seconds_price"].(ratio_setting.VideoSecondsPriceMap)
	require.True(t, ok)
	assert.Equal(t, map[string]map[string]float64{
		"720p": {"default": 0.1, "silent": 0.1},
	}, prices["video-model"])
}

func TestBuildDifferencesTreatsMatchingNestedVideoPricesAsEqual(t *testing.T) {
	prices := ratio_setting.VideoSecondsPriceMap{
		"video-model": {"720p": {"default": 0.1, "silent": 0.1}},
	}
	local := map[string]any{"video_seconds_price": prices}
	channels := []struct {
		name string
		data map[string]any
	}{
		{name: "OpenRouter", data: map[string]any{"video_seconds_price": prices}},
	}

	assert.Empty(t, buildDifferences(local, channels))
}
