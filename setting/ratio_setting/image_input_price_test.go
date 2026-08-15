package ratio_setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCalculateImageInputCostUsesResolutionTiersAndFreeCount(t *testing.T) {
	saved := ImageInputPrice2JSONString()
	t.Cleanup(func() { require.NoError(t, UpdateImageInputPriceByJSONString(saved)) })
	require.NoError(t, UpdateImageInputPriceByJSONString(`{
		"image-model": {"default": 0.003, "1k": 0.01, "2k": 0.02, "free_count": 1}
	}`))

	cost, counts, freeCount, configured := CalculateImageInputCost(
		"image-model",
		"",
		[]string{"1024x1024", "1536x1024", "unknown"},
	)

	require.True(t, configured)
	assert.InDelta(t, 0.023, cost, 1e-9)
	assert.Equal(t, map[string]int{"2k": 1, "default": 1}, counts)
	assert.Equal(t, 1, freeCount)
}

func TestUpdateImageInputPriceRejectsInvalidValues(t *testing.T) {
	tests := []string{
		`{"model":{"default":-1}}`,
		`{"model":{"free_count":1.5}}`,
		`{"model":{"free_count":129}}`,
		`{"model":{"3k":0.1}}`,
	}
	for _, input := range tests {
		require.Error(t, UpdateImageInputPriceByJSONString(input), input)
	}
}
