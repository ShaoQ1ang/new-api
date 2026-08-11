package openrouter

import (
	"strings"

	taskcommon "github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
)

var happyHorseAspectRatios = map[string]struct{}{
	"16:9": {}, "9:16": {}, "1:1": {}, "4:3": {}, "3:4": {}, "21:9": {}, "9:21": {},
}
var happyHorseFrameTypes = map[string]struct{}{"first_frame": {}}
var happyHorseReferenceTypes = map[string]struct{}{"image_url": {}}
var happyHorseSizes = map[string]struct{}{
	"1280x720": {}, "720x1280": {}, "720x720": {}, "960x720": {}, "720x960": {}, "1680x720": {}, "720x1680": {},
	"1920x1080": {}, "1080x1920": {}, "1080x1080": {}, "1440x1080": {}, "1080x1440": {}, "2520x1080": {}, "1080x2520": {},
}

func init() {
	taskcommon.RegisterVideoBillingConverter(func(modelName string) bool {
		return strings.HasPrefix(modelName, "alibaba/happyhorse-1.")
	}, convertHappyHorseVideoBillingParams)
}

func (h *HappyHorseHandler) Validate(req *relaycommon.TaskSubmitReq) error {
	if err := validateRequestMetadata(req); err != nil {
		return err
	}
	if strings.TrimSpace(req.Model) == "" {
		return errf("model is required")
	}
	if strings.TrimSpace(req.Prompt) == "" && len(req.Images) == 0 && !hasRequestInputs(req) {
		return errf("prompt or image input is required")
	}
	if err := validateCallbackURL(req); err != nil {
		return err
	}
	if err := validateResolutionAndSize(req); err != nil {
		return err
	}
	if _, _, err := requestIntegerSeed(req); err != nil {
		return err
	}
	if frameImages, ok := requestFrameImages(req); ok {
		if err := validateFrameImages(frameImages, happyHorseFrameTypes); err != nil {
			return err
		}
	}
	if inputReferences, ok := requestInputReferences(req); ok {
		if err := validateInputReferences(inputReferences, happyHorseReferenceTypes); err != nil {
			return err
		}
	}
	duration, err := resolveRequestedDuration(req, 5)
	if err != nil {
		return err
	}
	if duration < 3 || duration > 15 {
		return errf("happyhorse duration must be between 3 and 15 seconds")
	}
	if len(req.Images) > 9 {
		return errf("happyhorse supports at most 9 images")
	}
	if audio := requestAudioEnabled(req); audio != nil && *audio {
		return errf("happyhorse does not support audio generation")
	}
	if exactSize := normalizeExactVideoSize(requestSize(req)); exactSize != "" {
		if _, ok := happyHorseSizes[exactSize]; !ok {
			return errf("unsupported happyhorse size: %s", requestSize(req))
		}
	}
	if raw := requestResolutionOrSize(req); raw != "" {
		tier := normalizeTier(raw)
		if tier != "720p" && tier != "1080p" {
			return errf("unsupported happyhorse resolution: %s", raw)
		}
	}
	return validateAllowedValue("happyhorse aspect_ratio", requestAspectRatio(req), happyHorseAspectRatios)
}

func (h *HappyHorseHandler) BuildUpstreamRequest(info *relaycommon.RelayInfo, req *relaycommon.TaskSubmitReq) (map[string]any, error) {
	if err := h.Validate(req); err != nil {
		return nil, err
	}
	duration, _ := resolveRequestedDuration(req, 5)
	resolution := normalizeTier(requestResolutionOrSize(req))
	if resolution == "" {
		resolution = "1080p"
	}
	body := map[string]any{
		"model":    info.UpstreamModelName,
		"duration": duration,
	}
	if exactSize := normalizeExactVideoSize(requestSize(req)); exactSize != "" {
		body["size"] = exactSize
	} else {
		body["resolution"] = resolution
	}
	if prompt := strings.TrimSpace(req.Prompt); prompt != "" {
		body["prompt"] = prompt
	}
	if aspectRatio := requestAspectRatio(req); aspectRatio != "" {
		body["aspect_ratio"] = aspectRatio
	}
	if seed, ok, _ := requestIntegerSeed(req); ok {
		body["seed"] = seed
	}
	addCommonRequestOptions(body, req)
	usedRequestFrameImages := 0
	if frameImages, ok := requestFrameImages(req); ok {
		body["frame_images"] = frameImages
	} else if len(req.Images) > 0 {
		usedRequestFrameImages = 1
		body["frame_images"] = []map[string]any{buildFrameImage("first_frame", req.Images[0])}
	}
	if inputReferences, ok := requestInputReferences(req); ok {
		body["input_references"] = inputReferences
	} else if len(req.Images) > usedRequestFrameImages {
		body["input_references"] = buildImageReferences(req.Images[usedRequestFrameImages:])
	}
	return body, nil
}

func (h *HappyHorseHandler) EstimateBillingContext(req *relaycommon.TaskSubmitReq) (*VideoBillingContext, error) {
	if err := h.Validate(req); err != nil {
		return nil, err
	}
	duration, _ := resolveRequestedDuration(req, 5)
	tier := normalizeTier(requestResolutionOrSize(req))
	if tier == "" {
		tier = "1080p"
	}
	audio := false
	return &VideoBillingContext{DurationSeconds: duration, ResolutionTier: tier, AudioEnabled: &audio}, nil
}

func convertHappyHorseVideoBillingParams(req relaycommon.TaskSubmitReq) (*types.VideoBillingParams, error) {
	ctx, err := (&HappyHorseHandler{BaseHandler: NewBaseHandler("happyhorse")}).EstimateBillingContext(&req)
	if err != nil {
		return nil, err
	}
	return &types.VideoBillingParams{Tier: ctx.ResolutionTier, DurationSeconds: ctx.DurationSeconds}, nil
}
