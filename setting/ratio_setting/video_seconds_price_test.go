package ratio_setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenRouterVideoSecondsPricesInitializeFromOfficialSKUs(t *testing.T) {
	require.NoError(t, UpdateVideoSecondsPriceByJSONString(`{}`))
	t.Cleanup(func() { require.NoError(t, UpdateVideoSecondsPriceByJSONString(`{}`)) })
	InitRatioSettings()

	tests := []struct {
		model string
		tier  string
		audio bool
		price float64
	}{
		{model: "alibaba/happyhorse-1.1", tier: "1080p", price: 0.1278},
		{model: "kwaivgi/kling-v3.0-std", tier: "720p", price: 0.084},
		{model: "kwaivgi/kling-v3.0-std", tier: "720p", audio: true, price: 0.126},
		{model: "kwaivgi/kling-v3.0-pro", tier: "720p", audio: true, price: 0.168},
		{model: "minimax/hailuo-3", tier: "2k", audio: true, price: 0.13},
		{model: "minimax/hailuo-2.3", tier: "1080p", price: 0.0817},
	}
	for _, tt := range tests {
		price, ok := GetVideoSecondsPrice(tt.model, tt.tier, tt.audio)
		require.True(t, ok, tt.model)
		assert.InDelta(t, tt.price, price, 1e-9, tt.model)
	}
	referencePrice, ok := GetVideoSecondsExtraPrice("minimax/hailuo-3", "2k", "reference_image")
	require.True(t, ok)
	assert.InDelta(t, 0.04, referencePrice, 1e-9)
}

func TestGetVideoSecondsPriceSelectsDefault(t *testing.T) {
	t.Cleanup(func() {
		if err := UpdateVideoSecondsPriceByJSONString(`{}`); err != nil {
			t.Fatalf("cleanup video seconds price failed: %v", err)
		}
	})

	if err := UpdateVideoSecondsPriceByJSONString(`{
		"happyhorse-1.1-r2v": {
			"720p": {"default": 0.9}
		}
	}`); err != nil {
		t.Fatalf("update video seconds price failed: %v", err)
	}
	price, ok := GetVideoSecondsPrice("happyhorse-1.1-r2v", "720p", false)
	if !ok {
		t.Fatalf("expected configured price")
	}
	if price != 0.9 {
		t.Fatalf("expected 0.9, got %v", price)
	}
}

func TestGetVideoSecondsPriceSelectsSilent(t *testing.T) {
	t.Cleanup(func() {
		if err := UpdateVideoSecondsPriceByJSONString(`{}`); err != nil {
			t.Fatalf("cleanup video seconds price failed: %v", err)
		}
	})

	if err := UpdateVideoSecondsPriceByJSONString(`{
		"happyhorse-1.1-r2v": {
			"720p": {"default": 0.9, "silent": 0.6}
		}
	}`); err != nil {
		t.Fatalf("update video seconds price failed: %v", err)
	}
	price, ok := GetVideoSecondsPrice("happyhorse-1.1-r2v", "720p", false)
	if !ok || price != 0.6 {
		t.Fatalf("expected silent price 0.6, got ok=%v price=%v", ok, price)
	}
}

func TestGetVideoSecondsPriceUsesDefaultForAudioEnabledRequests(t *testing.T) {
	t.Cleanup(func() {
		if err := UpdateVideoSecondsPriceByJSONString(`{}`); err != nil {
			t.Fatalf("cleanup video seconds price failed: %v", err)
		}
	})

	if err := UpdateVideoSecondsPriceByJSONString(`{
		"kling/kling-v3-video-generation": {
			"1080p": {"default": 1.2}
		}
	}`); err != nil {
		t.Fatalf("update video seconds price failed: %v", err)
	}
	price, ok := GetVideoSecondsPrice("kling/kling-v3-video-generation", "1080p", true)
	if !ok || price != 1.2 {
		t.Fatalf("expected default price 1.2, got ok=%v price=%v", ok, price)
	}
}

func TestGetVideoSecondsPriceFallsBackToDefault(t *testing.T) {
	t.Cleanup(func() {
		if err := UpdateVideoSecondsPriceByJSONString(`{}`); err != nil {
			t.Fatalf("cleanup video seconds price failed: %v", err)
		}
	})

	if err := UpdateVideoSecondsPriceByJSONString(`{
		"kling/kling-v3-video-generation": {
			"1080p": {"default": 1.2}
		}
	}`); err != nil {
		t.Fatalf("update video seconds price failed: %v", err)
	}
	price, ok := GetVideoSecondsPrice("kling/kling-v3-video-generation", "1080p", true)
	if !ok || price != 1.2 {
		t.Fatalf("expected fallback price 1.2, got ok=%v price=%v", ok, price)
	}
}
