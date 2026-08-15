package common

import (
	"fmt"
	"math"
	"strings"

	"github.com/QuantumNous/new-api/dto"
)

func TaskInputImageTiers(req TaskSubmitReq) ([]string, error) {
	tiers := make([]string, 0)
	if len(req.Images) > 0 {
		for _, image := range req.Images {
			if strings.TrimSpace(image) != "" {
				tiers = append(tiers, "default")
			}
		}
	} else if strings.TrimSpace(req.Image) != "" {
		tiers = append(tiers, "default")
	}

	frameImages := req.FrameImages
	if len(frameImages) == 0 {
		frameImages = taskMetadataMapSlice(req.Metadata, "frame_images")
	}
	for _, frame := range frameImages {
		tiers = append(tiers, taskImageTier(frame))
	}

	inputReferences := req.InputReferences
	if len(inputReferences) == 0 {
		inputReferences = taskMetadataMapSlice(req.Metadata, "input_references")
	}
	for _, reference := range inputReferences {
		referenceType, _ := reference["type"].(string)
		if referenceType == "image" || referenceType == "image_url" || referenceType == "input_image" {
			tiers = append(tiers, taskImageTier(reference))
		}
	}

	if len(inputReferences) == 0 {
		for _, reference := range taskMetadataStringSlice(req.Metadata, "reference_images") {
			if strings.TrimSpace(reference) != "" {
				tiers = append(tiers, "default")
			}
		}
	}
	if len(tiers) == 0 && strings.TrimSpace(req.InputReference) != "" {
		tiers = append(tiers, "default")
	}

	if len(tiers) > dto.MaxImageN {
		return nil, fmt.Errorf("input image count must not exceed %d", dto.MaxImageN)
	}
	return tiers, nil
}

func taskImageTier(value map[string]any) string {
	for _, key := range []string{"resolution", "size"} {
		if tier, ok := value[key].(string); ok && strings.TrimSpace(tier) != "" {
			return tier
		}
	}
	width, widthOK := taskImageDimension(value["width"])
	height, heightOK := taskImageDimension(value["height"])
	if widthOK && heightOK {
		return fmt.Sprintf("%dx%d", width, height)
	}
	return "default"
}

func taskImageDimension(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, typed > 0
	case float64:
		if typed > 0 && typed <= math.MaxInt32 && typed == math.Trunc(typed) {
			return int(typed), true
		}
	}
	return 0, false
}

func taskMetadataMapSlice(metadata map[string]interface{}, key string) []map[string]any {
	if metadata == nil {
		return nil
	}
	items, _ := metadata[key].([]map[string]any)
	if len(items) > 0 {
		return items
	}
	rawItems, ok := metadata[key].([]any)
	if !ok {
		return nil
	}
	result := make([]map[string]any, 0, len(rawItems))
	for _, item := range rawItems {
		if typed, ok := item.(map[string]any); ok {
			result = append(result, typed)
		}
	}
	return result
}

func taskMetadataStringSlice(metadata map[string]interface{}, key string) []string {
	if metadata == nil {
		return nil
	}
	items, _ := metadata[key].([]string)
	if len(items) > 0 {
		return items
	}
	rawItems, ok := metadata[key].([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(rawItems))
	for _, item := range rawItems {
		if typed, ok := item.(string); ok {
			result = append(result, typed)
		}
	}
	return result
}
