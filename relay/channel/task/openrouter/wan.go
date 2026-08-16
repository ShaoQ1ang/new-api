package openrouter

import (
	"strings"

	taskcommon "github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
)

var wan27AspectRatios = map[string]struct{}{"16:9": {}, "9:16": {}, "1:1": {}, "4:3": {}, "3:4": {}}
var wan27FrameTypes = map[string]struct{}{"first_frame": {}, "last_frame": {}}
var wan27ReferenceTypes = map[string]struct{}{"image_url": {}, "video_url": {}, "audio_url": {}}

func init() {
	taskcommon.RegisterVideoBillingConverter(func(modelName string) bool {
		return strings.EqualFold(strings.TrimSpace(modelName), "alibaba/wan-2.7")
	}, convertWan27VideoBillingParams)
}

type Wan27Handler struct{ BaseHandler }

func (h *Wan27Handler) Match(model string) bool {
	return strings.EqualFold(strings.TrimSpace(model), "alibaba/wan-2.7")
}

func (h *Wan27Handler) Validate(req *relaycommon.TaskSubmitReq) error {
	if err := validateRequestMetadata(req); err != nil {
		return err
	}
	if !h.Match(req.Model) {
		return errf("unsupported wan2.7 model: %s", req.Model)
	}
	if strings.TrimSpace(req.Prompt) == "" && len(req.Images)+len(req.Videos) == 0 && !hasRequestInputs(req) {
		return errf("prompt or media input is required")
	}
	if err := validateCallbackURL(req); err != nil {
		return err
	}
	if _, _, err := requestIntegerSeed(req); err != nil {
		return err
	}
	duration, err := resolveRequestedDuration(req, 5)
	if err != nil {
		return err
	}
	if duration < 2 || duration > 10 {
		return errf("wan2.7 duration must be between 2 and 10 seconds")
	}
	if raw := requestResolutionOrSize(req); raw != "" {
		tier := normalizeTier(raw)
		if tier != "720p" && tier != "1080p" {
			return errf("unsupported wan2.7 resolution: %s", raw)
		}
	}
	if err := validateAllowedValue("wan2.7 aspect_ratio", requestAspectRatio(req), wan27AspectRatios); err != nil {
		return err
	}
	if frameImages, ok := requestFrameImages(req); ok {
		if err := validateFrameImages(frameImages, wan27FrameTypes); err != nil {
			return err
		}
	}
	if inputReferences, ok := requestInputReferences(req); ok {
		if err := validateInputReferences(inputReferences, wan27ReferenceTypes); err != nil {
			return err
		}
	}
	return validateWan27Mode(req)
}

func validateWan27Mode(req *relaycommon.TaskSubmitReq) error {
	mode := strings.TrimSpace(req.Mode)
	if mode == "" {
		return nil
	}
	switch mode {
	case "text_to_video":
		if len(req.Images)+len(req.Videos) != 0 || len(req.Audios) > 1 {
			return errf("wan2.7 text mode accepts only one optional driving audio")
		}
	case "first_frame":
		if len(req.Images) != 1 || len(req.Videos) != 0 || len(req.Audios) > 1 {
			return errf("wan2.7 first_frame mode requires one image and at most one audio")
		}
	case "first_last_frame":
		if len(req.Images) != 2 || len(req.Videos) != 0 || len(req.Audios) > 1 {
			return errf("wan2.7 first_last_frame mode requires two images and at most one audio")
		}
	case "reference":
		visualCount := len(req.Images) + len(req.Videos)
		if visualCount < 1 || visualCount > 5 {
			return errf("wan2.7 reference mode requires 1-5 visual references")
		}
		if len(req.Audios) > visualCount {
			return errf("wan2.7 reference audio count cannot exceed visual references")
		}
	default:
		return errf("unsupported wan2.7 mode: %s", mode)
	}
	return nil
}

func (h *Wan27Handler) BuildUpstreamRequest(info *relaycommon.RelayInfo, req *relaycommon.TaskSubmitReq) (map[string]any, error) {
	if err := h.Validate(req); err != nil {
		return nil, err
	}
	duration, _ := resolveRequestedDuration(req, 5)
	resolution := normalizeTier(requestResolutionOrSize(req))
	if resolution == "" {
		resolution = "1080p"
	}
	body := map[string]any{"model": info.UpstreamModelName, "duration": duration, "resolution": resolution, "generate_audio": *requestAudioWithDefault(req, true)}
	body["prompt_extend"] = true
	if prompt := strings.TrimSpace(req.Prompt); prompt != "" {
		body["prompt"] = prompt
	}
	if ratio := requestAspectRatio(req); ratio != "" {
		body["aspect_ratio"] = ratio
	}
	if seed, ok, _ := requestIntegerSeed(req); ok {
		body["seed"] = seed
	}
	for _, key := range []string{"negative_prompt", "prompt_extend"} {
		if value, ok := req.Metadata[key]; ok {
			body[key] = value
		}
	}
	addCommonRequestOptions(body, req)
	if frameImages, ok := requestFrameImages(req); ok {
		body["frame_images"] = frameImages
	} else if frames := wan27Frames(req); len(frames) > 0 {
		body["frame_images"] = frames
	}
	if inputReferences, ok := requestInputReferences(req); ok {
		body["input_references"] = inputReferences
	} else if refs := wan27References(req); len(refs) > 0 {
		body["input_references"] = refs
	}
	if len(req.Audios) == 1 && strings.TrimSpace(req.Mode) != "reference" {
		body["audio"] = req.Audios[0]
	}
	return body, nil
}

func wan27Frames(req *relaycommon.TaskSubmitReq) []map[string]any {
	mode := strings.TrimSpace(req.Mode)
	if mode == "first_frame" && len(req.Images) >= 1 {
		return []map[string]any{buildFrameImage("first_frame", req.Images[0])}
	}
	if mode == "first_last_frame" && len(req.Images) >= 2 {
		return []map[string]any{buildFrameImage("first_frame", req.Images[0]), buildFrameImage("last_frame", req.Images[1])}
	}
	if mode == "reference" {
		frames := []map[string]any{}
		for index, image := range req.Images {
			if index < len(req.ImageRoles) && req.ImageRoles[index] == "first_frame" {
				frames = append(frames, buildFrameImage("first_frame", image))
			}
		}
		return frames
	}
	return nil
}

func wan27References(req *relaycommon.TaskSubmitReq) []map[string]any {
	if strings.TrimSpace(req.Mode) != "reference" {
		return nil
	}
	refs := make([]map[string]any, 0, len(req.Images)+len(req.Videos)+len(req.Audios))
	for index, image := range req.Images {
		if index < len(req.ImageRoles) && req.ImageRoles[index] == "first_frame" {
			continue
		}
		refs = append(refs, buildInputReference("image", image))
	}
	for _, video := range req.Videos {
		refs = append(refs, buildInputReference("video", video))
	}
	for _, audio := range req.Audios {
		refs = append(refs, buildInputReference("audio", audio))
	}
	return refs
}

func (h *Wan27Handler) EstimateBillingContext(req *relaycommon.TaskSubmitReq) (*VideoBillingContext, error) {
	if err := h.Validate(req); err != nil {
		return nil, err
	}
	duration, _ := resolveRequestedDuration(req, 5)
	tier := normalizeTier(requestResolutionOrSize(req))
	if tier == "" {
		tier = "1080p"
	}
	return &VideoBillingContext{DurationSeconds: duration, ResolutionTier: tier, AudioEnabled: requestAudioWithDefault(req, true)}, nil
}

func convertWan27VideoBillingParams(req relaycommon.TaskSubmitReq) (*types.VideoBillingParams, error) {
	ctx, err := (&Wan27Handler{BaseHandler: NewBaseHandler("wan27")}).EstimateBillingContext(&req)
	if err != nil {
		return nil, err
	}
	return &types.VideoBillingParams{Tier: ctx.ResolutionTier, DurationSeconds: ctx.DurationSeconds, AudioEnabled: *ctx.AudioEnabled}, nil
}
