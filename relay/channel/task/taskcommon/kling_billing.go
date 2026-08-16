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
	return &types.VideoBillingParams{
		Tier:            tier,
		DurationSeconds: resolveVideoBillingDuration(req),
		AudioEnabled:    audioEnabled,
	}, nil
}

func resolveKlingBillingTier(req relaycommon.TaskSubmitReq) string {
	if strings.EqualFold(strings.TrimSpace(req.Resolution), "720p") {
		return "720p"
	}
	if strings.TrimSpace(req.Resolution) != "" {
		return "1080p"
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
	if strings.EqualFold(strings.TrimSpace(req.Size), "720p") {
		return "720p"
	}
	return "1080p"
}
