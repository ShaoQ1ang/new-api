package execution

import (
	"fmt"
	"strings"

	aigcdto "github.com/QuantumNous/new-api/aigc/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

const TaskProtocolNewAPIVideo = "newapi-video"

func BuildTaskRequest(spec Spec) (relaycommon.TaskSubmitReq, error) {
	if strings.TrimSpace(spec.ModelType) != "video" {
		return relaycommon.TaskSubmitReq{}, fmt.Errorf("AIGC task execution does not support model type %q", spec.ModelType)
	}
	if strings.TrimSpace(spec.TaskProtocol) != TaskProtocolNewAPIVideo {
		return relaycommon.TaskSubmitReq{}, fmt.Errorf("unsupported AIGC task protocol %q", spec.TaskProtocol)
	}
	upstreamModelID := strings.TrimSpace(spec.UpstreamModelID)
	if upstreamModelID == "" {
		return relaycommon.TaskSubmitReq{}, fmt.Errorf("AIGC task execution requires an upstream model")
	}
	request := relaycommon.TaskSubmitReq{
		Prompt: strings.TrimSpace(spec.Request.Prompt), Model: upstreamModelID, Mode: strings.TrimSpace(spec.Mode),
		Resolution: strings.TrimSpace(spec.Request.Output.Resolution), AspectRatio: strings.TrimSpace(spec.Request.Output.AspectRatio),
		Duration: spec.Request.Output.Duration, GenerateAudio: copyBool(spec.Request.Output.GenerateAudio),
	}
	request.Images, request.ImageRoles = taskMedia(spec.Request.Inputs.Images)
	request.Videos, request.VideoRoles = taskMedia(spec.Request.Inputs.Videos)
	request.Audios, request.AudioRoles = taskMedia(spec.Request.Inputs.Audios)
	request.Metadata = taskOptions(spec.Request.Options)
	return request, nil
}

func taskMedia(items []aigcdto.MediaInput) ([]string, []string) {
	if len(items) == 0 {
		return nil, nil
	}
	urls := make([]string, len(items))
	roles := make([]string, len(items))
	for index, item := range items {
		urls[index] = strings.TrimSpace(item.URL)
		roles[index] = strings.TrimSpace(item.Role)
	}
	return urls, roles
}

func taskOptions(options aigcdto.GenerationOptions) map[string]any {
	metadata := make(map[string]any)
	if value := strings.TrimSpace(options.NegativePrompt); value != "" {
		metadata["negative_prompt"] = value
	}
	if options.Enhance {
		metadata["enhance"] = true
	}
	if options.Private {
		metadata["private"] = true
	}
	if options.AIMark {
		metadata["ai_mark"] = true
	}
	if len(metadata) == 0 {
		return nil
	}
	return metadata
}

func copyBool(value *bool) *bool {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}
