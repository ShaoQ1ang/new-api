package openrouter

import (
	"strings"

	taskcommon "github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
)

var klingAspectRatios = map[string]struct{}{"16:9": {}, "9:16": {}, "1:1": {}}
var klingFrameTypes = map[string]struct{}{"first_frame": {}, "last_frame": {}}
var klingReferenceTypes = map[string]struct{}{"image_url": {}}
var klingSizes = map[string]struct{}{"1280x720": {}, "720x1280": {}, "720x720": {}}

func init() {
	taskcommon.RegisterVideoBillingConverter(isSupportedKlingModel, convertKlingVideoBillingParams)
}

func isSupportedKlingModel(modelName string) bool {
	switch strings.ToLower(strings.TrimSpace(modelName)) {
	case "kwaivgi/kling-v3.0-pro", "kwaivgi/kling-v3.0-std", "kwaivgi/kling-video-o1":
		return true
	default:
		return false
	}
}

func (h *KlingHandler) Validate(req *relaycommon.TaskSubmitReq) error {
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
	if _, hasSeed := requestSeed(req); hasSeed {
		return errf("kling does not support seed")
	}
	if frameImages, ok := requestFrameImages(req); ok {
		if err := validateFrameImages(frameImages, klingFrameTypes); err != nil {
			return err
		}
	}
	if inputReferences, ok := requestInputReferences(req); ok {
		if err := validateInputReferences(inputReferences, klingReferenceTypes); err != nil {
			return err
		}
	}
	duration, err := resolveRequestedDuration(req, 5)
	if err != nil {
		return err
	}
	if strings.EqualFold(strings.TrimSpace(req.Model), "kwaivgi/kling-video-o1") {
		if duration != 5 && duration != 10 {
			return errf("kling video o1 duration must be 5 or 10 seconds")
		}
	} else if duration < 3 || duration > 15 {
		return errf("kling v3 duration must be between 3 and 15 seconds")
	}
	if len(req.Images) > 9 {
		return errf("kling supports at most 9 images")
	}
	if exactSize := normalizeExactVideoSize(requestSize(req)); exactSize != "" {
		if _, ok := klingSizes[exactSize]; !ok {
			return errf("unsupported kling size: %s", requestSize(req))
		}
	}
	if raw := requestResolutionOrSize(req); raw != "" && normalizeTier(raw) != "720p" {
		return errf("unsupported kling resolution: %s", raw)
	}
	return validateAllowedValue("kling aspect_ratio", requestAspectRatio(req), klingAspectRatios)
}

func (h *KlingHandler) BuildUpstreamRequest(info *relaycommon.RelayInfo, req *relaycommon.TaskSubmitReq) (map[string]any, error) {
	if err := h.Validate(req); err != nil {
		return nil, err
	}
	duration, _ := resolveRequestedDuration(req, 5)
	audio := requestAudioWithDefault(req, true)
	body := map[string]any{
		"model":          info.UpstreamModelName,
		"duration":       duration,
		"generate_audio": *audio,
	}
	if exactSize := normalizeExactVideoSize(requestSize(req)); exactSize != "" {
		body["size"] = exactSize
	} else {
		body["resolution"] = "720p"
	}
	if prompt := strings.TrimSpace(req.Prompt); prompt != "" {
		body["prompt"] = prompt
	}
	if aspectRatio := requestAspectRatio(req); aspectRatio != "" {
		body["aspect_ratio"] = aspectRatio
	}
	addCommonRequestOptions(body, req)
	usedRequestFrameImages := 0
	if frameImages, ok := requestFrameImages(req); ok {
		body["frame_images"] = frameImages
	} else if len(req.Images) > 0 {
		usedRequestFrameImages = min(len(req.Images), 2)
		body["frame_images"] = inferFrameImages(req.Images[:usedRequestFrameImages])
	}
	if inputReferences, ok := requestInputReferences(req); ok {
		body["input_references"] = inputReferences
	} else {
		var refs []map[string]any
		if len(req.Images) > usedRequestFrameImages {
			refs = append(refs, buildImageReferences(req.Images[usedRequestFrameImages:])...)
		}
		refs = append(refs, buildReferenceURLs(req.Metadata, "reference_images", "image")...)
		if len(refs) > 0 {
			body["input_references"] = refs
		}
	}
	providerKeys := []string{"negative_prompt", "cfg_scale"}
	if strings.EqualFold(strings.TrimSpace(req.Model), "kwaivgi/kling-video-o1") {
		providerKeys = []string{"negative_prompt"}
	}
	addProviderPassthrough(body, req.Metadata, "atlas-cloud", providerKeys...)
	return body, nil
}

func (h *KlingHandler) EstimateBillingContext(req *relaycommon.TaskSubmitReq) (*VideoBillingContext, error) {
	if err := h.Validate(req); err != nil {
		return nil, err
	}
	duration, _ := resolveRequestedDuration(req, 5)
	return &VideoBillingContext{
		DurationSeconds: duration,
		ResolutionTier:  "720p",
		AudioEnabled:    requestAudioWithDefault(req, true),
	}, nil
}

func convertKlingVideoBillingParams(req relaycommon.TaskSubmitReq) (*types.VideoBillingParams, error) {
	ctx, err := (&KlingHandler{BaseHandler: NewBaseHandler("kling")}).EstimateBillingContext(&req)
	if err != nil {
		return nil, err
	}
	return &types.VideoBillingParams{
		Tier: ctx.ResolutionTier, DurationSeconds: ctx.DurationSeconds, AudioEnabled: *ctx.AudioEnabled,
	}, nil
}
