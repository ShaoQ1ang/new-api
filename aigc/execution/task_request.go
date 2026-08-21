package execution

import (
	"fmt"
	"strings"

	aigcdto "github.com/QuantumNous/new-api/aigc/dto"
	relaydto "github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

const TaskProtocolNewAPIVideo = "newapi-video"
const TaskProtocolSunoAPIV1 = "sunoapi-v1"

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

func BuildMusicTaskRequest(spec Spec) (relaydto.SunoSubmitReq, error) {
	if strings.TrimSpace(spec.ModelType) != "music" {
		return relaydto.SunoSubmitReq{}, fmt.Errorf("AIGC music task execution does not support model type %q", spec.ModelType)
	}
	if strings.TrimSpace(spec.Mode) != "text_to_music" {
		return relaydto.SunoSubmitReq{}, fmt.Errorf("AIGC music task execution does not support mode %q", spec.Mode)
	}
	upstreamModelID := strings.TrimSpace(spec.UpstreamModelID)
	if upstreamModelID == "" {
		return relaydto.SunoSubmitReq{}, fmt.Errorf("AIGC music task execution requires an upstream model")
	}
	return relaydto.SunoSubmitReq{
		GptDescriptionPrompt: strings.TrimSpace(spec.Request.Prompt), Mv: upstreamModelID,
		MakeInstrumental: spec.Request.Parameters.Instrumental,
	}, nil
}

func BuildSunoAPIV1TaskRequest(spec Spec, callbackURL string) (relaycommon.TaskSubmitReq, error) {
	if strings.TrimSpace(spec.ModelType) != "music" || strings.TrimSpace(spec.Mode) != "text_to_music" {
		return relaycommon.TaskSubmitReq{}, fmt.Errorf("unsupported AIGC SunoAPI music spec")
	}
	if strings.TrimSpace(spec.TaskProtocol) != TaskProtocolSunoAPIV1 {
		return relaycommon.TaskSubmitReq{}, fmt.Errorf("unsupported AIGC music task protocol %q", spec.TaskProtocol)
	}
	if strings.TrimSpace(spec.UpstreamModelID) == "" {
		return relaycommon.TaskSubmitReq{}, fmt.Errorf("AIGC music task execution requires an upstream model")
	}
	p := spec.Request.Parameters
	metadata := map[string]any{
		"customMode": spec.MusicCustomMode, "instrumental": p.Instrumental,
		"callBackUrl": strings.TrimSpace(callbackURL), "lyrics": strings.TrimSpace(p.Lyrics),
	}
	optional := map[string]string{
		"style": p.Style, "title": p.Title, "personaId": p.PersonaID, "personaModel": p.PersonaModel,
		"negativeTags": p.NegativeTags, "vocalGender": p.VocalGender,
	}
	for key, value := range optional {
		if value = strings.TrimSpace(value); value != "" {
			metadata[key] = value
		}
	}
	if p.Duration != nil {
		metadata["duration"] = *p.Duration
	}
	if p.StyleWeight != nil {
		metadata["styleWeight"] = *p.StyleWeight
	}
	if p.WeirdnessConstraint != nil {
		metadata["weirdnessConstraint"] = *p.WeirdnessConstraint
	}
	if p.AudioWeight != nil {
		metadata["audioWeight"] = *p.AudioWeight
	}
	return relaycommon.TaskSubmitReq{
		Prompt: strings.TrimSpace(spec.Request.Prompt), Model: strings.TrimSpace(spec.UpstreamModelID),
		Mode: strings.TrimSpace(spec.Mode), CallbackURL: strings.TrimSpace(callbackURL), Metadata: metadata,
	}, nil
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
