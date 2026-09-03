package openrouter

import (
	"strings"

	taskcommon "github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
)

var miniMaxH3AspectRatios = map[string]struct{}{
	"21:9": {}, "16:9": {}, "4:3": {}, "1:1": {}, "3:4": {}, "9:16": {},
}
var miniMaxHailuo23AspectRatios = map[string]struct{}{"16:9": {}}
var miniMaxH3FrameTypes = map[string]struct{}{"first_frame": {}, "last_frame": {}}
var miniMaxHailuo23FrameTypes = map[string]struct{}{"first_frame": {}}
var miniMaxReferenceTypes = map[string]struct{}{"image_url": {}}
var miniMaxHailuo23Sizes = map[string]struct{}{"1920x1080": {}}

const miniMaxH3FreeReferenceImages = 5

func init() {
	taskcommon.RegisterVideoBillingConverter(func(modelName string) bool {
		return modelName == "minimax/hailuo-3" || modelName == "minimax/hailuo-2.3"
	}, convertMiniMaxVideoBillingParams)
}

func (h *MiniMaxHandler) Validate(req *relaycommon.TaskSubmitReq) error {
	if err := validateRequestMetadata(req); err != nil {
		return err
	}
	modelName := strings.ToLower(strings.TrimSpace(req.Model))
	if modelName == "" {
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
		return errf("minimax does not support seed")
	}
	frameTypes := miniMaxH3FrameTypes
	if modelName == "minimax/hailuo-2.3" {
		frameTypes = miniMaxHailuo23FrameTypes
	}
	if frameImages, ok := requestFrameImages(req); ok {
		if err := validateFrameImages(frameImages, frameTypes); err != nil {
			return err
		}
	}
	if inputReferences, ok := requestInputReferences(req); ok {
		if err := validateInputReferences(inputReferences, miniMaxReferenceTypes); err != nil {
			return err
		}
	}
	fallback := 5
	if modelName == "minimax/hailuo-2.3" {
		fallback = 6
	}
	duration, err := resolveRequestedDuration(req, fallback)
	if err != nil {
		return err
	}
	if modelName == "minimax/hailuo-2.3" {
		if duration != 6 && duration != 10 {
			return errf("minimax hailuo 2.3 duration must be 6 or 10 seconds")
		}
		if audio := requestAudioEnabled(req); audio != nil && *audio {
			return errf("minimax hailuo 2.3 does not support audio generation")
		}
		if countMiniMaxImageInputs(req, 1) > 9 {
			return errf("minimax supports at most 9 images")
		}
		if exactSize := normalizeExactVideoSize(requestSize(req)); exactSize != "" {
			if _, ok := miniMaxHailuo23Sizes[exactSize]; !ok {
				return errf("unsupported minimax hailuo 2.3 size: %s", requestSize(req))
			}
		}
		if raw := requestResolutionOrSize(req); raw != "" && normalizeTier(raw) != "1080p" {
			return errf("unsupported minimax hailuo 2.3 resolution: %s", raw)
		}
		return validateAllowedValue("minimax hailuo 2.3 aspect_ratio", requestAspectRatio(req), miniMaxHailuo23AspectRatios)
	}
	if duration < 5 || duration > 15 {
		return errf("minimax hailuo 3 duration must be between 5 and 15 seconds")
	}
	if countMiniMaxImageInputs(req, 2) > 9 {
		return errf("minimax supports at most 9 images")
	}
	if raw := requestResolutionOrSize(req); raw != "" && normalizeTier(raw) != "2k" {
		return errf("unsupported minimax hailuo 3 resolution: %s", raw)
	}
	return validateAllowedValue("minimax hailuo 3 aspect_ratio", requestAspectRatio(req), miniMaxH3AspectRatios)
}

func (h *MiniMaxHandler) BuildUpstreamRequest(info *relaycommon.RelayInfo, req *relaycommon.TaskSubmitReq) (map[string]any, error) {
	if err := h.Validate(req); err != nil {
		return nil, err
	}
	isHailuo23 := strings.EqualFold(strings.TrimSpace(req.Model), "minimax/hailuo-2.3")
	fallback, resolution, audioDefault := 5, "2K", true
	if isHailuo23 {
		fallback, resolution, audioDefault = 6, "1080p", false
	}
	duration, _ := resolveRequestedDuration(req, fallback)
	body := map[string]any{
		"model":          info.UpstreamModelName,
		"duration":       duration,
		"resolution":     resolution,
		"generate_audio": *requestAudioWithDefault(req, audioDefault),
	}
	if isHailuo23 {
		if exactSize := normalizeExactVideoSize(requestSize(req)); exactSize != "" {
			delete(body, "resolution")
			body["size"] = exactSize
		}
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
		if isHailuo23 {
			usedRequestFrameImages = 1
		}
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
	if isHailuo23 {
		addProviderPassthrough(body, req.Metadata, "minimax", "prompt_optimizer", "fast_pretreatment")
	} else {
		addProviderPassthrough(body, req.Metadata, "minimax", "aigc_watermark")
	}
	return body, nil
}

func (h *MiniMaxHandler) EstimateBillingContext(req *relaycommon.TaskSubmitReq) (*VideoBillingContext, error) {
	if err := h.Validate(req); err != nil {
		return nil, err
	}
	isHailuo23 := strings.EqualFold(strings.TrimSpace(req.Model), "minimax/hailuo-2.3")
	fallback, tier, audioDefault := 5, "2k", true
	if isHailuo23 {
		fallback, tier, audioDefault = 6, "1080p", false
	}
	duration, _ := resolveRequestedDuration(req, fallback)
	return &VideoBillingContext{
		DurationSeconds: duration,
		ResolutionTier:  tier,
		AudioEnabled:    requestAudioWithDefault(req, audioDefault),
	}, nil
}

func convertMiniMaxVideoBillingParams(req relaycommon.TaskSubmitReq) (*types.VideoBillingParams, error) {
	ctx, err := (&MiniMaxHandler{BaseHandler: NewBaseHandler("minimax")}).EstimateBillingContext(&req)
	if err != nil {
		return nil, err
	}
	params := &types.VideoBillingParams{
		Tier: ctx.ResolutionTier, DurationSeconds: ctx.DurationSeconds, AudioEnabled: *ctx.AudioEnabled,
	}
	if strings.EqualFold(strings.TrimSpace(req.Model), "minimax/hailuo-3") {
		referenceImages := countMiniMaxReferenceImages(&req, 2)
		if billableImages := referenceImages - miniMaxH3FreeReferenceImages; billableImages > 0 {
			params.ExtraUnits = map[string]int{"reference_image": billableImages}
		}
	}
	return params, nil
}

func countMiniMaxImageInputs(req *relaycommon.TaskSubmitReq, maxFrameImages int) int {
	frameCount := 0
	usedRequestFrameImages := 0
	if frameImages, ok := requestFrameImages(req); ok {
		frameCount = len(frameImages)
	} else {
		usedRequestFrameImages = min(len(req.Images), maxFrameImages)
		frameCount = usedRequestFrameImages
	}
	if inputReferences, ok := requestInputReferences(req); ok {
		for _, ref := range inputReferences {
			if ref["type"] == "image_url" {
				frameCount++
			}
		}
		return frameCount
	}
	count := frameCount + len(req.Images) - usedRequestFrameImages
	if referenceImages, ok := metadataStringSlice(req.Metadata, "reference_images"); ok {
		count += len(referenceImages)
	}
	return count
}

func countMiniMaxReferenceImages(req *relaycommon.TaskSubmitReq, maxFrameImages int) int {
	if inputReferences, ok := requestInputReferences(req); ok {
		count := 0
		for _, ref := range inputReferences {
			if ref["type"] == "image_url" {
				count++
			}
		}
		return count
	}
	usedRequestFrameImages := 0
	if _, hasExplicitFrames := requestFrameImages(req); !hasExplicitFrames {
		usedRequestFrameImages = min(len(req.Images), maxFrameImages)
	}
	count := len(req.Images) - usedRequestFrameImages
	if referenceImages, ok := metadataStringSlice(req.Metadata, "reference_images"); ok {
		count += len(referenceImages)
	}
	return count
}
