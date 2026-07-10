package openrouter

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	taskcommon "github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
)

type seedanceNormalizedRequest struct {
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

func (h *SeedanceHandler) BuildUpstreamRequest(info *relaycommon.RelayInfo, req *relaycommon.TaskSubmitReq) (map[string]any, error) {
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

func (h *SeedanceHandler) EstimateBillingContext(req *relaycommon.TaskSubmitReq) (*VideoBillingContext, error) {
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

func init() {
	taskcommon.RegisterVideoBillingConverter(func(modelName string) bool {
		return strings.HasPrefix(modelName, "bytedance/seedance")
	}, convertSeedanceVideoBillingParams)
}

func (h *SeedanceHandler) normalizeRequest(req *relaycommon.TaskSubmitReq) (*seedanceNormalizedRequest, error) {
	duration := req.Duration
	if duration <= 0 {
		duration = parsePositiveInt(req.Seconds)
	}
	if duration <= 0 {
		duration = 5
	}
	normalized := &seedanceNormalizedRequest{
		Prompt:          strings.TrimSpace(req.Prompt),
		DurationSeconds: duration,
		Resolution:      firstNonEmpty(stringMetadata(req.Metadata, "resolution"), req.Size),
		AspectRatio:     firstNonEmpty(stringMetadata(req.Metadata, "aspect_ratio"), stringMetadata(req.Metadata, "ratio")),
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
		normalized.InputReferences = inferInputReferences(req.Images, req.Videos, req.Metadata, len(normalized.FrameImages))
	}

	return normalized, nil
}

func inferFrameImages(images []string) []map[string]any {
	switch len(images) {
	case 0:
		return nil
	case 1:
		return []map[string]any{buildFrameImage("first_frame", images[0])}
	default:
		return []map[string]any{
			buildFrameImage("first_frame", images[0]),
			buildFrameImage("last_frame", images[1]),
		}
	}
}

func buildFrameImage(frameType, url string) map[string]any {
	return map[string]any{
		"frame_type": frameType,
		"image_url": map[string]any{
			"url": url,
		},
	}
}

func inferInputReferences(images []string, videos []string, metadata map[string]any, usedFrameImages int) []map[string]any {
	var refs []map[string]any
	for _, image := range images[usedFrameImages:] {
		refs = append(refs, buildInputReference("image", image))
	}
	for _, video := range videos {
		refs = append(refs, buildInputReference("video", video))
	}
	refs = append(refs, buildReferenceURLs(metadata, "reference_images", "image")...)
	refs = append(refs, buildReferenceURLs(metadata, "reference_videos", "video")...)
	refs = append(refs, buildReferenceURLs(metadata, "reference_audios", "audio")...)
	return refs
}

func buildInputReference(refType, url string) map[string]any {
	return map[string]any{
		"type": refType,
		"url":  url,
	}
}

func buildReferenceURLs(metadata map[string]any, key, refType string) []map[string]any {
	values, ok := metadataStringSlice(metadata, key)
	if !ok {
		return nil
	}
	refs := make([]map[string]any, 0, len(values))
	for _, value := range values {
		refs = append(refs, buildInputReference(refType, value))
	}
	return refs
}

func metadataMapSlice(metadata map[string]any, key string) ([]map[string]any, bool) {
	if metadata == nil {
		return nil, false
	}
	value, ok := metadata[key]
	if !ok {
		return nil, false
	}
	raw, err := common.Marshal(value)
	if err != nil {
		return nil, false
	}
	var items []map[string]any
	if err := common.Unmarshal(raw, &items); err != nil {
		return nil, false
	}
	return items, len(items) > 0
}

func metadataStringSlice(metadata map[string]any, key string) ([]string, bool) {
	if metadata == nil {
		return nil, false
	}
	value, ok := metadata[key]
	if !ok {
		return nil, false
	}
	raw, err := common.Marshal(value)
	if err != nil {
		return nil, false
	}
	var items []string
	if err := common.Unmarshal(raw, &items); err != nil {
		return nil, false
	}
	filtered := make([]string, 0, len(items))
	for _, item := range items {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			filtered = append(filtered, trimmed)
		}
	}
	return filtered, len(filtered) > 0
}

func parsePositiveInt(raw string) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	value := firstInt64(map[string]any{"value": raw}, "value")
	if value > 0 {
		return int(value)
	}
	return 0
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func cloneMetadataExcludingKnown(metadata map[string]any) map[string]any {
	if metadata == nil {
		return nil
	}
	cloned := map[string]any{}
	for key, value := range metadata {
		switch key {
		case "resolution", "aspect_ratio", "ratio", "audio", "generate_audio",
			"seed", "provider", "callback_url", "frame_images", "input_references",
			"reference_images", "reference_videos", "reference_audios":
			continue
		default:
			cloned[key] = value
		}
	}
	return cloned
}

func convertSeedanceVideoBillingParams(req relaycommon.TaskSubmitReq) (*types.VideoBillingParams, error) {
	handler := &SeedanceHandler{BaseHandler: NewBaseHandler("seedance")}
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
