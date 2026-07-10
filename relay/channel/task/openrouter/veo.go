package openrouter

import (
	"strings"

	taskcommon "github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
)

type veoNormalizedRequest struct {
	Prompt          string
	DurationSeconds int
	Resolution      string
	AspectRatio     string
	GenerateAudio   *bool
	Seed            any
	Provider        any
	CallbackURL     string
	FrameImages     []map[string]any
	InputReferences []map[string]any
	RawMetadata     map[string]any
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
	for key, value := range normalized.RawMetadata {
		if _, exists := body[key]; exists {
			continue
		}
		body[key] = value
	}
	return body, nil
}

func (h *VeoHandler) EstimateBillingContext(req *relaycommon.TaskSubmitReq) (*VideoBillingContext, error) {
	normalized, err := h.normalizeRequest(req)
	if err != nil {
		return nil, err
	}
	tier := normalizeTier(normalized.Resolution)
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
	duration := req.Duration
	if duration <= 0 {
		duration = parsePositiveInt(req.Seconds)
	}
	duration = normalizeSupportedDuration(duration, veoSupportedDurations, 8)
	resolution := normalizeVeoResolution(firstNonEmpty(stringMetadata(req.Metadata, "resolution"), req.Size))
	if raw := firstNonEmpty(stringMetadata(req.Metadata, "resolution"), req.Size); strings.TrimSpace(raw) != "" && resolution == "" {
		return nil, errf("unsupported veo resolution: %s", raw)
	}
	aspectRatio := firstNonEmpty(stringMetadata(req.Metadata, "aspect_ratio"), stringMetadata(req.Metadata, "ratio"))
	if aspectRatio != "" {
		if _, ok := veoSupportedAspectRatios[strings.TrimSpace(aspectRatio)]; !ok {
			return nil, errf("unsupported veo aspect_ratio: %s", aspectRatio)
		}
	}
	normalized := &veoNormalizedRequest{
		Prompt:          strings.TrimSpace(req.Prompt),
		DurationSeconds: duration,
		Resolution:      resolution,
		AspectRatio:     aspectRatio,
		GenerateAudio:   resolveAudioEnabled(req.Metadata),
		Seed:            req.Metadata["seed"],
		Provider:        req.Metadata["provider"],
		CallbackURL:     stringMetadata(req.Metadata, "callback_url"),
		RawMetadata:     cloneMetadataExcludingKnown(req.Metadata),
	}
	if frameImages, ok := metadataMapSlice(req.Metadata, "frame_images"); ok && len(frameImages) > 0 {
		normalized.FrameImages = frameImages
	} else {
		normalized.FrameImages = inferFrameImages(req.Images)
	}
	if inputReferences, ok := metadataMapSlice(req.Metadata, "input_references"); ok && len(inputReferences) > 0 {
		normalized.InputReferences = inputReferences
	} else {
		normalized.InputReferences = inferVeoInputReferences(req.Images, len(normalized.FrameImages), req.Metadata)
	}
	return normalized, nil
}

func inferVeoInputReferences(images []string, usedFrameImages int, metadata map[string]any) []map[string]any {
	var refs []map[string]any
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
	bestDistance := absInt(best - value)
	for _, current := range allowed[1:] {
		currentDistance := absInt(current - value)
		if currentDistance < bestDistance || (currentDistance == bestDistance && current < best) {
			best = current
			bestDistance = currentDistance
		}
	}
	return best
}

func normalizeVeoResolution(value string) string {
	normalized := normalizeTier(value)
	if _, ok := veoSupportedResolutions[normalized]; ok {
		return normalized
	}
	return ""
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
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
