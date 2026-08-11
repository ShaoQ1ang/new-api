package billing_setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOpenRouterVideoModelsDefaultToVideoSecondsBilling(t *testing.T) {
	for _, model := range []string{
		"alibaba/happyhorse-1.0",
		"alibaba/happyhorse-1.1",
		"kwaivgi/kling-v3.0-std",
		"kwaivgi/kling-v3.0-pro",
		"kwaivgi/kling-video-o1",
		"minimax/hailuo-3",
		"minimax/hailuo-2.3",
	} {
		assert.Equal(t, BillingModeVideoSeconds, GetBillingMode(model), model)
	}
}
