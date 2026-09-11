package ratio_setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveImageResolutionTier(t *testing.T) {
	tests := []struct {
		name     string
		size     string
		wantTier string
		wantOK   bool
	}{
		{name: "direct tier", size: " 2K ", wantTier: "2k", wantOK: true},
		{name: "one k square", size: "1024x1024", wantTier: "1k", wantOK: true},
		{name: "studio two k landscape", size: "2560x1440", wantTier: "2k", wantOK: true},
		{name: "studio two k portrait", size: "1440x2560", wantTier: "2k", wantOK: true},
		{name: "studio two k four three", size: "2304x1728", wantTier: "2k", wantOK: true},
		{name: "large portrait", size: "1696*2528", wantTier: "4k", wantOK: true},
		{name: "four k ultrawide", size: "6336×2688", wantTier: "4k", wantOK: true},
		{name: "maximum edge decides tier", size: "1280X720", wantTier: "2k", wantOK: true},
		{name: "large custom size", size: "2688x1152", wantTier: "4k", wantOK: true},
		{name: "arbitrary size", size: "1600x1600", wantTier: "2k", wantOK: true},
		{name: "oversized image", size: "8192x8192", wantTier: "4k", wantOK: true},
		{name: "auto defaults to two k", size: "auto", wantTier: "2k", wantOK: true},
		{name: "empty defaults to two k", size: "", wantTier: "2k", wantOK: true},
		{name: "invalid defaults to two k", size: "wide", wantTier: "2k", wantOK: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tier, ok := ResolveImageResolutionTier(tt.size)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.wantTier, tier)
		})
	}
}

func TestImageResolutionPriceConfiguration(t *testing.T) {
	require.NoError(t, UpdateImageResolutionPriceByJSONString(`{}`))
	t.Cleanup(func() { require.NoError(t, UpdateImageResolutionPriceByJSONString(`{}`)) })

	require.NoError(t, UpdateImageResolutionPriceByJSONString(`{
		"image-model": {"1K": 0.04, "2k": 0.08}
	}`))

	price, tier, ok := GetImageResolutionPrice("image-model", "2048x2048")
	require.True(t, ok)
	assert.Equal(t, "2k", tier)
	assert.Equal(t, 0.08, price)

	_, tier, ok = GetImageResolutionPrice("image-model", "4096x4096")
	assert.False(t, ok, "a known size must not enable an unconfigured model SKU")
	assert.Equal(t, "4k", tier)

	err := UpdateImageResolutionPriceByJSONString(`{"image-model":{"8k":1}}`)
	require.Error(t, err)
	price, _, ok = GetImageResolutionPrice("image-model", "1024x1024")
	require.True(t, ok, "an invalid update must preserve the previous configuration")
	assert.Equal(t, 0.04, price)

	require.Error(t, UpdateImageResolutionPriceByJSONString(`{"image-model":{"1k":-1}}`))
}

func TestStudioImageResolutionUsesExactSizeBeforeLegacyFallback(t *testing.T) {
	require.NoError(t, UpdateImageResolutionPriceByJSONString(`{"seedream-4.5":{"2k":0.2,"4k":0.3}}`))
	t.Cleanup(func() { require.NoError(t, UpdateImageResolutionPriceByJSONString(`{}`)) })

	price, tier, ok := GetImageResolutionPrice("seedream-4.5", "2560x1440")
	require.True(t, ok)
	assert.Equal(t, "2k", tier)
	assert.Equal(t, 0.2, price)

	price, tier, ok = GetImageResolutionPrice("seedream-4.5", "3024x1296")
	require.True(t, ok)
	assert.Equal(t, "2k", tier)
	assert.Equal(t, 0.2, price)
}
