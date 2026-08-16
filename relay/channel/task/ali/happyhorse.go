package ali

import (
	"strings"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/samber/lo"
)

func (a *TaskAdaptor) buildHappyHorseRequest(upstreamModel string, req relaycommon.TaskSubmitReq) *AliVideoRequest {
	aliReq := &AliVideoRequest{
		Model: upstreamModel,
		Input: AliVideoInput{
			Prompt: req.Prompt,
		},
		Parameters: &AliVideoParameters{
			Resolution: normalizeHappyHorseResolution(firstNonEmptyString(req.Resolution, req.Size), "1080P"),
			Watermark:  lo.ToPtr(false),
		},
	}
	isI2V := strings.Contains(upstreamModel, "-i2v")
	isT2VOrR2V := strings.Contains(upstreamModel, "-t2v") || strings.Contains(upstreamModel, "-r2v")

	switch {
	case isI2V:
		aliReq.Input.Media = []AliMediaItem{{Type: "first_frame", URL: req.Images[0]}}
	case strings.Contains(upstreamModel, "-r2v"):
		for _, url := range req.Images {
			aliReq.Input.Media = append(aliReq.Input.Media, AliMediaItem{Type: "reference_image", URL: url})
		}
	case strings.Contains(upstreamModel, "-video-edit"):
		aliReq.Input.Media = append(aliReq.Input.Media, AliMediaItem{Type: "video", URL: req.Videos[0]})
		for _, url := range req.Images {
			aliReq.Input.Media = append(aliReq.Input.Media, AliMediaItem{Type: "reference_image", URL: url})
		}
	default:
		aliReq.Parameters.Duration = lo.ToPtr(resolveTaskDuration(req, 5))
	}
	if !strings.Contains(upstreamModel, "-video-edit") {
		aliReq.Parameters.Duration = lo.ToPtr(resolveTaskDuration(req, 5))
	}
	ratio := strings.TrimSpace(req.AspectRatio)
	if ratio == "" && req.Metadata != nil {
		if isI2V {
			for _, key := range []string{"aspectRatio", "aspect_ratio", "ratio"} {
				if value, ok := getStringMetadata(req.Metadata, key); ok {
					ratio = value
					break
				}
			}
		} else if value, ok := getStringMetadata(req.Metadata, "ratio"); ok {
			ratio = value
		}
	}
	if ratio != "" {
		if isI2V {
			aliReq.Parameters.AspectRatio = lo.ToPtr(ratio)
		} else if isT2VOrR2V {
			aliReq.Parameters.Ratio = lo.ToPtr(ratio)
		}
	}

	audio := req.GenerateAudio
	if audio == nil && req.Metadata != nil {
		for _, key := range []string{"generateAudio", "generate_audio", "audio"} {
			if value, ok := getBoolMetadata(req.Metadata, key); ok {
				audio = lo.ToPtr(value)
				break
			}
		}
	}
	aliReq.Parameters.Audio = audio

	if req.Metadata != nil {
		if watermark, ok := getBoolMetadata(req.Metadata, "watermark"); ok {
			aliReq.Parameters.Watermark = lo.ToPtr(watermark)
		}
		if audioSetting, ok := getStringMetadata(req.Metadata, "audio_setting"); ok && strings.Contains(upstreamModel, "-video-edit") {
			aliReq.Parameters.AudioSetting = lo.ToPtr(audioSetting)
		}
		if seed, ok := getIntMetadata(req.Metadata, "seed"); ok {
			aliReq.Parameters.Seed = seed
		}
	}
	return aliReq
}

func normalizeHappyHorseResolution(size string, fallback string) string {
	if strings.TrimSpace(size) == "" {
		return fallback
	}
	resolution := strings.ToUpper(strings.TrimSpace(size))
	if strings.Contains(resolution, "*") {
		if converted, err := sizeToResolution(resolution); err == nil {
			return converted
		}
		return fallback
	}
	if !strings.HasSuffix(resolution, "P") && resolution != "2K" && resolution != "4K" {
		resolution += "P"
	}
	return resolution
}
