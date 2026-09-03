package openrouter

import (
	"math"
	"strings"

	taskcommon "github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
)

type veoNormalizedRequest struct {
	Prompt          string
	DurationSeconds int
	Resolution      string
	Size            string
	AspectRatio     string
	GenerateAudio   *bool
	Seed            any
	Provider        any
	CallbackURL     string
	FrameImages     []map[string]any
	InputReferences []map[string]any
}

var veoSupportedDurations = []int{4, 6, 8}
var veoSupportedAspectRatios = map[string]struct{}{
	"16:9": {},
	"9:16": {},
}
var veoSupportedResolutions = map[string]struct{}{
	"720p":  {},
	"1080p": {},
}
var veoFrameTypes = map[string]struct{}{"first_frame": {}, "last_frame": {}}
var veoReferenceTypes = map[string]struct{}{"image_url": {}}

func init() {
	taskcommon.RegisterVideoBillingConverter(func(modelName string) bool {
		return strings.HasPrefix(modelName, "google/veo")
	}, convertVeoVideoBillingParams)
}

func (h *VeoHandler) BuildUpstreamRequest(info *relaycommon.RelayInfo, req *relaycommon.TaskSubmitReq) (map[string]any, error) {
	normalized, err := h.normalizeRequest(req)
	if err != nil {
		return nil, err
	}
	body := map[string]any{
		"model":  info.UpstreamModelName,
		"prompt": normalized.Prompt,
	}
	if normalized.DurationSeconds > 0 {
		body["duration"] = normalized.DurationSeconds
	}
	if normalized.Resolution != "" {
		body["resolution"] = normalized.Resolution
	}
	if normalized.Size != "" {
		body["size"] = normalized.Size
	}
	if normalized.AspectRatio != "" {
		body["aspect_ratio"] = normalized.AspectRatio
	}
	if normalized.GenerateAudio != nil {
		body["generate_audio"] = *normalized.GenerateAudio
	}
	if len(normalized.FrameImages) > 0 {
		body["frame_images"] = normalized.FrameImages
	}
	if len(normalized.InputReferences) > 0 {
		body["input_references"] = normalized.InputReferences
	}
	if normalized.Seed != nil {
		body["seed"] = normalized.Seed
	}
	if normalized.Provider != nil {
		body["provider"] = normalized.Provider
	}
	if normalized.CallbackURL != "" {
		body["callback_url"] = normalized.CallbackURL
	}
	return body, nil
}

func (h *VeoHandler) EstimateBillingContext(req *relaycommon.TaskSubmitReq) (*VideoBillingContext, error) {
	normalized, err := h.normalizeRequest(req)
	if err != nil {
		return nil, err
	}
	tier := normalizeTier(normalized.Size)
	if tier == "" {
		tier = normalizeResolutionTier(req)
	}
	return &VideoBillingContext{
		DurationSeconds: normalized.DurationSeconds,
		ResolutionTier:  tier,
		AudioEnabled:    normalized.GenerateAudio,
		OtherRatios: map[string]float64{
			"seconds": float64(normalized.DurationSeconds),
		},
	}, nil
}

func (h *VeoHandler) normalizeRequest(req *relaycommon.TaskSubmitReq) (*veoNormalizedRequest, error) {
	if err := validateRequestMetadata(req); err != nil {
		return nil, err
	}
	if err := validateCallbackURL(req); err != nil {
		return nil, err
	}
	if err := validateResolutionAndSize(req); err != nil {
		return nil, err
	}
	if _, _, err := requestIntegerSeed(req); err != nil {
		return nil, err
	}
	if frameImages, ok := requestFrameImages(req); ok {
		if err := validateFrameImages(frameImages, veoFrameTypes); err != nil {
			return nil, err
		}
	}
	if inputReferences, ok := requestInputReferences(req); ok {
		if err := validateInputReferences(inputReferences, veoReferenceTypes); err != nil {
			return nil, err
		}
	}
	duration := req.Duration
	if duration <= 0 {
		duration = parsePositiveInt(req.Seconds)
	}
	if duration <= 0 {
		legacyDuration, ok, err := metadataDurationSeconds(req.Metadata)
		if err != nil {
			return nil, err
		}
		if ok {
			duration = legacyDuration
		}
	}
	duration = normalizeSupportedDuration(duration, veoSupportedDurations, 8)
	rawSize := requestSize(req)
	exactSize := normalizeExactVideoSize(rawSize)
	rawResolution := requestResolution(req)
	resolution := normalizeVeoResolution(firstNonEmpty(rawResolution, rawSize, exactSize))
	if raw := firstNonEmpty(rawResolution, rawSize, exactSize); strings.TrimSpace(raw) != "" && resolution == "" {
		return nil, errf("unsupported veo resolution: %s", raw)
	}
	if strings.EqualFold(resolution, "4K") && strings.EqualFold(strings.TrimSpace(req.Model), "google/veo-3.1-lite") {
		return nil, errf("unsupported veo resolution: %s", firstNonEmpty(rawResolution, rawSize, exactSize))
	}
	aspectRatio := requestAspectRatio(req)
	if aspectRatio == "" {
		aspectRatio = stringMetadata(req.Metadata, "aspectRatio")
	}
	if aspectRatio != "" {
		if _, ok := veoSupportedAspectRatios[strings.TrimSpace(aspectRatio)]; !ok {
			return nil, errf("unsupported veo aspect_ratio: %s", aspectRatio)
		}
	}
	if exactSize != "" {
		resolution = ""
		aspectRatio = ""
	}
	seed, _ := requestSeed(req)
	provider, _ := requestProvider(req)
	normalized := &veoNormalizedRequest{
		Prompt:          strings.TrimSpace(req.Prompt),
		DurationSeconds: duration,
		Resolution:      resolution,
		Size:            exactSize,
		AspectRatio:     aspectRatio,
		GenerateAudio:   requestAudioEnabled(req),
		Seed:            seed,
		Provider:        provider,
		CallbackURL:     requestCallbackURL(req),
	}
	if frameImages, ok := requestFrameImages(req); ok && len(frameImages) > 0 {
		normalized.FrameImages = frameImages
	} else {
		normalized.FrameImages = inferFrameImages(req.Images)
	}
	if inputReferences, ok := requestInputReferences(req); ok && len(inputReferences) > 0 {
		normalized.InputReferences = inputReferences
	} else {
		normalized.InputReferences = inferVeoInputReferences(req.Images, len(normalized.FrameImages), req.Metadata)
	}
	return normalized, nil
}

func inferVeoInputReferences(images []string, usedFrameImages int, metadata map[string]any) []map[string]any {
	var refs []map[string]any
	if usedFrameImages > len(images) {
		usedFrameImages = len(images)
	}
	for _, image := range images[usedFrameImages:] {
		refs = append(refs, buildInputReference("image", image))
	}
	refs = append(refs, buildReferenceURLs(metadata, "reference_images", "image")...)
	return refs
}

func normalizeSupportedDuration(value int, allowed []int, fallback int) int {
	if value <= 0 {
		return fallback
	}
	best := allowed[0]
	bestDistance := math.Abs(float64(best) - float64(value))
	for _, current := range allowed[1:] {
		currentDistance := math.Abs(float64(current) - float64(value))
		if currentDistance < bestDistance || (currentDistance == bestDistance && current < best) {
			best = current
			bestDistance = currentDistance
		}
	}
	return best
}

func normalizeVeoResolution(value string) string {
	normalized := normalizeTier(value)
	if normalized == "4k" {
		return "4K"
	}
	if _, ok := veoSupportedResolutions[normalized]; ok {
		return normalized
	}
	return ""
}

func convertVeoVideoBillingParams(req relaycommon.TaskSubmitReq) (*types.VideoBillingParams, error) {
	handler := &VeoHandler{BaseHandler: NewBaseHandler("veo")}
	ctx, err := handler.EstimateBillingContext(&req)
	if err != nil {
		return nil, err
	}
	audioEnabled := false
	if ctx.AudioEnabled != nil {
		audioEnabled = *ctx.AudioEnabled
	}
	return &types.VideoBillingParams{
		Tier:            ctx.ResolutionTier,
		DurationSeconds: ctx.DurationSeconds,
		AudioEnabled:    audioEnabled,
	}, nil
}
