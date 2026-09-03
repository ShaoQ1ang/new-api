package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskInputImageTiersPreservesRequestOrderAndResolution(t *testing.T) {
	tiers, err := TaskInputImageTiers(TaskSubmitReq{
		Images: []string{"https://example.test/frame.png"},
		InputReferences: []map[string]any{
			{"type": "image_url", "width": 2048.0, "height": 1024.0},
			{"type": "video_url", "url": "https://example.test/video.mp4"},
		},
		Metadata: map[string]interface{}{
			"reference_images": []any{"https://example.test/ignored.png"},
		},
	})

	require.NoError(t, err)
	assert.Equal(t, []string{"default", "2048x1024"}, tiers)
}
