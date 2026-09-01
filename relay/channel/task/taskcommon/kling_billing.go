package taskcommon

import (
	"strings"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
)

func convertAliKlingVideoBillingParams(req relaycommon.TaskSubmitReq) (*types.VideoBillingParams, error) {
	tier := resolveKlingBillingTier(req)
	audioEnabled := false
	if req.GenerateAudio != nil {
		audioEnabled = *req.GenerateAudio
	} else if audio, ok := resolveMetadataBool(req.Metadata, "audio"); ok {
		audioEnabled = audio
	}
	params := &types.VideoBillingParams{
		Tier:            tier,
		DurationSeconds: resolveVideoBillingDuration(req),
		AudioEnabled:    audioEnabled,
	}
	if len(req.Videos) > 0 || metadataContainsReferenceVideo(req.Metadata) {
		key := "reference_video"
		if !audioEnabled {
			key = "reference_video_silent"
		}
		params.PriceKey = key
	}
	return params, nil
}

func resolveKlingBillingTier(req relaycommon.TaskSubmitReq) string {
	for _, value := range []string{req.Resolution, req.Size} {
		switch strings.ToLower(strings.ReplaceAll(strings.TrimSpace(value), " ", "")) {
		case "720p", "1280x720", "720x1280", "720x720":
			return "720p"
		case "1080p", "1920x1080", "1080x1920", "1080x1080":
			return "1080p"
		case "4k", "2160p", "3840x2160", "2160x3840", "2160x2160":
			return "4k"
		}
	}
	if mode, ok := resolveMetadataString(req.Metadata, "mode"); ok {
		if strings.EqualFold(mode, "std") {
			return "720p"
		}
		return "1080p"
	}
	if mode := strings.TrimSpace(req.Mode); mode == "std" || mode == "pro" {
		if strings.EqualFold(mode, "std") {
			return "720p"
		}
		return "1080p"
	}
	return "1080p"
}

func metadataContainsReferenceVideo(metadata map[string]interface{}) bool {
	media, ok := metadata["media"].([]interface{})
	if !ok {
		return false
	}
	for _, item := range media {
		entry, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		typeName, _ := entry["type"].(string)
		if typeName == "base" || typeName == "feature" || typeName == "reference_video" {
			return true
		}
	}
	return false
}
