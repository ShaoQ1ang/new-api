package doubao

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
)

func init() {
	taskcommon.RegisterVideoBillingConverter(func(modelName string) bool {
		return strings.HasPrefix(modelName, "doubao-seedance-2-0")
	}, convertDoubaoVideoBillingParams)
}

func convertDoubaoVideoBillingParams(req relaycommon.TaskSubmitReq) (*types.VideoBillingParams, error) {
	duration, err := resolveDoubaoVideoBillingDuration(req)
	if err != nil {
		return nil, err
	}

	audioEnabled := false
	if req.GenerateAudio != nil {
		audioEnabled = *req.GenerateAudio
	} else {
		for _, key := range []string{"generate_audio", "generateAudio", "audio"} {
			if audio, ok := req.Metadata[key].(bool); ok {
				audioEnabled = audio
				break
			}
		}
	}

	resolution := strings.TrimSpace(req.Resolution)
	if resolution == "" {
		if value, ok := req.Metadata["resolution"].(string); ok {
			resolution = value
		}
	}
	if resolution == "" {
		resolution = req.Size
	}

	return &types.VideoBillingParams{
		Tier:            normalizeDoubaoVideoBillingTier(resolution),
		DurationSeconds: duration,
		AudioEnabled:    audioEnabled,
	}, nil
}

func resolveDoubaoVideoBillingDuration(req relaycommon.TaskSubmitReq) (int, error) {
	duration := req.Duration
	if duration <= 0 && strings.TrimSpace(req.Seconds) != "" {
		parsed, err := strconv.Atoi(strings.TrimSpace(req.Seconds))
		if err != nil {
			return 0, fmt.Errorf("invalid video duration %q", req.Seconds)
		}
		duration = parsed
	}
	if duration <= 0 {
		switch value := req.Metadata["durationSeconds"].(type) {
		case float64:
			if math.IsNaN(value) || math.IsInf(value, 0) || value > float64(relaycommon.MaxTaskDurationSeconds) || value < float64(-relaycommon.MaxTaskDurationSeconds) {
				return 0, fmt.Errorf("video duration is out of range")
			}
			duration = int(value)
		case int:
			duration = value
		case string:
			parsed, err := strconv.Atoi(strings.TrimSpace(value))
			if err != nil {
				return 0, fmt.Errorf("invalid video duration %q", value)
			}
			duration = parsed
		}
	}
	if duration <= 0 {
		duration = 5
	}
	if duration > relaycommon.MaxTaskDurationSeconds {
		return 0, fmt.Errorf("video duration must be at most %d seconds", relaycommon.MaxTaskDurationSeconds)
	}
	return duration, nil
}

func normalizeDoubaoVideoBillingTier(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "480p", "480", "832*480", "480*832":
		return "480p"
	case "1080p", "1080", "1920*1080", "1080*1920":
		return "1080p"
	case "2k":
		return "2k"
	case "4k":
		return "4k"
	default:
		return "720p"
	}
}
