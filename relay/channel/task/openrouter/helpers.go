package openrouter

import (
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

func errf(format string, args ...any) error {
	return fmt.Errorf(format, args...)
}

func normalizeResolutionTier(req *relaycommon.TaskSubmitReq) string {
	candidates := []string{
		req.Resolution,
		stringMetadata(req.Metadata, "resolution"),
		req.Size,
		stringMetadata(req.Metadata, "size"),
		stringMetadata(req.Metadata, "quality"),
	}
	for _, candidate := range candidates {
		if tier := normalizeTier(candidate); tier != "" {
			return tier
		}
	}
	return "1080p"
}

func normalizeTier(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = strings.ReplaceAll(normalized, " ", "")
	switch normalized {
	case "480p", "854x480", "832*480", "480*832", "624*624":
		return "480p"
	case "720p", "1280x720", "720x1280", "720x720", "960x720", "720x960", "1680x720", "720x1680", "1280*720", "720*1280", "960*960", "1088*832", "832*1088", "std":
		return "720p"
	case "1080p", "1920x1080", "1080x1920", "1080x1080", "1440x1080", "1080x1440", "2520x1080", "1080x2520", "1920*1080", "1080*1920", "1440*1440", "1632*1248", "1248*1632", "pro":
		return "1080p"
	case "2k":
		return "2k"
	case "4k":
		return "4k"
	}
	return ""
}

func resolveRequestedDuration(req *relaycommon.TaskSubmitReq, fallback int) (int, error) {
	if req.Duration != 0 {
		if req.Duration < 1 || req.Duration > relaycommon.MaxTaskDurationSeconds {
			return 0, errf("duration must be between 1 and %d seconds", relaycommon.MaxTaskDurationSeconds)
		}
		return req.Duration, nil
	}
	if strings.TrimSpace(req.Seconds) == "" {
		return fallback, nil
	}
	duration, err := strconv.Atoi(strings.TrimSpace(req.Seconds))
	if err != nil || duration < 1 || duration > relaycommon.MaxTaskDurationSeconds {
		return 0, errf("seconds must be between 1 and %d", relaycommon.MaxTaskDurationSeconds)
	}
	return duration, nil
}

func requestResolution(req *relaycommon.TaskSubmitReq) string {
	if req == nil {
		return ""
	}
	return firstNonEmpty(req.Resolution, stringMetadata(req.Metadata, "resolution"))
}

func requestSize(req *relaycommon.TaskSubmitReq) string {
	if req == nil {
		return ""
	}
	return firstNonEmpty(req.Size, stringMetadata(req.Metadata, "size"))
}

func requestResolutionOrSize(req *relaycommon.TaskSubmitReq) string {
	return firstNonEmpty(requestResolution(req), requestSize(req))
}

func requestAspectRatio(req *relaycommon.TaskSubmitReq) string {
	if req == nil {
		return ""
	}
	return firstNonEmpty(req.AspectRatio, stringMetadata(req.Metadata, "aspect_ratio"), stringMetadata(req.Metadata, "ratio"))
}

func requestAudioEnabled(req *relaycommon.TaskSubmitReq) *bool {
	if req == nil {
		return nil
	}
	if req.GenerateAudio != nil {
		return req.GenerateAudio
	}
	return resolveAudioEnabled(req.Metadata)
}

func requestAudioWithDefault(req *relaycommon.TaskSubmitReq, fallback bool) *bool {
	if audio := requestAudioEnabled(req); audio != nil {
		return audio
	}
	return &fallback
}

func requestFrameImages(req *relaycommon.TaskSubmitReq) ([]map[string]any, bool) {
	if req == nil {
		return nil, false
	}
	if len(req.FrameImages) > 0 {
		return req.FrameImages, true
	}
	return metadataMapSlice(req.Metadata, "frame_images")
}

func requestInputReferences(req *relaycommon.TaskSubmitReq) ([]map[string]any, bool) {
	if req == nil {
		return nil, false
	}
	if len(req.InputReferences) > 0 {
		return req.InputReferences, true
	}
	return metadataMapSlice(req.Metadata, "input_references")
}

func requestSeed(req *relaycommon.TaskSubmitReq) (any, bool) {
	if req == nil {
		return nil, false
	}
	if req.Seed != nil {
		return *req.Seed, true
	}
	seed, ok := req.Metadata["seed"]
	return seed, ok && seed != nil
}

func requestProvider(req *relaycommon.TaskSubmitReq) (any, bool) {
	if req == nil {
		return nil, false
	}
	if len(req.Provider) > 0 {
		return req.Provider, true
	}
	provider, ok := req.Metadata["provider"]
	return provider, ok && provider != nil
}

func requestCallbackURL(req *relaycommon.TaskSubmitReq) string {
	if req == nil {
		return ""
	}
	return firstNonEmpty(req.CallbackURL, stringMetadata(req.Metadata, "callback_url"))
}

func normalizeExactVideoSize(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = strings.ReplaceAll(normalized, "*", "x")
	parts := strings.Split(normalized, "x")
	if len(parts) != 2 {
		return ""
	}
	width, widthErr := strconv.Atoi(parts[0])
	height, heightErr := strconv.Atoi(parts[1])
	if widthErr != nil || heightErr != nil || width <= 0 || height <= 0 {
		return ""
	}
	return fmt.Sprintf("%dx%d", width, height)
}

func validateResolutionAndSize(req *relaycommon.TaskSubmitReq) error {
	if requestResolution(req) != "" && normalizeExactVideoSize(requestSize(req)) != "" {
		return errf("resolution and exact size cannot be used together")
	}
	return nil
}

func requestIntegerSeed(req *relaycommon.TaskSubmitReq) (int64, bool, error) {
	value, ok := requestSeed(req)
	if !ok {
		return 0, false, nil
	}
	switch typed := value.(type) {
	case int:
		return int64(typed), true, nil
	case int64:
		return typed, true, nil
	case float64:
		if math.IsNaN(typed) || math.IsInf(typed, 0) || typed != math.Trunc(typed) || typed < math.MinInt64 || typed > math.MaxInt64 {
			return 0, false, errf("seed must be an integer")
		}
		return int64(typed), true, nil
	default:
		return 0, false, errf("seed must be an integer")
	}
}

func validateCallbackURL(req *relaycommon.TaskSubmitReq) error {
	callbackURL := requestCallbackURL(req)
	if callbackURL == "" {
		return nil
	}
	parsed, err := url.Parse(callbackURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return errf("callback_url must be a valid HTTPS URL")
	}
	return nil
}

func validateRequestMetadata(req *relaycommon.TaskSubmitReq) error {
	if req == nil || req.Metadata == nil {
		return nil
	}
	for _, key := range []string{"resolution", "size", "aspect_ratio", "ratio", "callback_url"} {
		if value, ok := req.Metadata[key]; ok {
			if _, valid := value.(string); !valid {
				return errf("metadata.%s must be a string", key)
			}
		}
	}
	for _, key := range []string{"audio", "generate_audio"} {
		if value, ok := req.Metadata[key]; ok {
			if _, valid := value.(bool); !valid {
				return errf("metadata.%s must be a boolean", key)
			}
		}
	}
	for _, key := range []string{"frame_images", "input_references"} {
		value, ok := req.Metadata[key]
		if !ok {
			continue
		}
		raw, err := common.Marshal(value)
		if err != nil {
			return errf("metadata.%s must be an array of objects", key)
		}
		var items []map[string]any
		if err := common.Unmarshal(raw, &items); err != nil {
			return errf("metadata.%s must be an array of objects", key)
		}
	}
	for _, key := range []string{"reference_images", "reference_videos", "reference_audios"} {
		value, ok := req.Metadata[key]
		if !ok {
			continue
		}
		raw, err := common.Marshal(value)
		if err != nil {
			return errf("metadata.%s must be an array of strings", key)
		}
		var items []string
		if err := common.Unmarshal(raw, &items); err != nil {
			return errf("metadata.%s must be an array of strings", key)
		}
	}
	if provider, ok := req.Metadata["provider"]; ok {
		raw, err := common.Marshal(provider)
		if err != nil {
			return errf("metadata.provider must be an object")
		}
		var object map[string]any
		if err := common.Unmarshal(raw, &object); err != nil {
			return errf("metadata.provider must be an object")
		}
	}
	return nil
}

func validateFrameImages(frameImages []map[string]any, allowedFrameTypes map[string]struct{}) error {
	seen := make(map[string]struct{}, len(frameImages))
	for _, frame := range frameImages {
		if frame["type"] != "image_url" {
			return errf("frame_images type must be image_url")
		}
		frameType, _ := frame["frame_type"].(string)
		if _, ok := allowedFrameTypes[frameType]; !ok {
			return errf("unsupported frame_type: %s", frameType)
		}
		if _, exists := seen[frameType]; exists {
			return errf("duplicate frame_type: %s", frameType)
		}
		seen[frameType] = struct{}{}
		imageURL, ok := frame["image_url"].(map[string]any)
		if !ok || strings.TrimSpace(stringValue(imageURL["url"])) == "" {
			return errf("frame_images image_url.url is required")
		}
	}
	return nil
}

func validateInputReferences(inputReferences []map[string]any, allowedTypes map[string]struct{}) error {
	for _, reference := range inputReferences {
		referenceType, _ := reference["type"].(string)
		if _, ok := allowedTypes[referenceType]; !ok {
			return errf("unsupported input reference type: %s", referenceType)
		}
		referenceURL, ok := reference[referenceType].(map[string]any)
		if !ok || strings.TrimSpace(stringValue(referenceURL["url"])) == "" {
			return errf("input_references %s.url is required", referenceType)
		}
	}
	return nil
}

func stringValue(value any) string {
	str, _ := value.(string)
	return str
}

func validateAllowedValue(field, value string, allowed map[string]struct{}) error {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	if _, ok := allowed[strings.TrimSpace(value)]; !ok {
		return errf("unsupported %s: %s", field, value)
	}
	return nil
}

func hasRequestInputs(req *relaycommon.TaskSubmitReq) bool {
	if req == nil {
		return false
	}
	if len(req.FrameImages) > 0 || len(req.InputReferences) > 0 {
		return true
	}
	if frameImages, ok := metadataMapSlice(req.Metadata, "frame_images"); ok && len(frameImages) > 0 {
		return true
	}
	if inputReferences, ok := metadataMapSlice(req.Metadata, "input_references"); ok && len(inputReferences) > 0 {
		return true
	}
	if referenceImages, ok := metadataStringSlice(req.Metadata, "reference_images"); ok && len(referenceImages) > 0 {
		return true
	}
	return false
}

func buildImageReferences(urls []string) []map[string]any {
	refs := make([]map[string]any, 0, len(urls))
	for _, url := range urls {
		if strings.TrimSpace(url) != "" {
			refs = append(refs, buildInputReference("image", url))
		}
	}
	return refs
}

func addCommonRequestOptions(body map[string]any, req *relaycommon.TaskSubmitReq) {
	if provider, ok := requestProvider(req); ok {
		body["provider"] = provider
	}
	if callbackURL := requestCallbackURL(req); callbackURL != "" {
		body["callback_url"] = callbackURL
	}
}

func addProviderPassthrough(body map[string]any, metadata map[string]any, providerSlug string, keys ...string) {
	if metadata == nil {
		return
	}
	if _, exists := body["provider"]; exists {
		return
	}
	options := make(map[string]any)
	for _, key := range keys {
		if value, ok := metadata[key]; ok {
			options[key] = value
		}
	}
	if len(options) == 0 {
		return
	}
	body["provider"] = map[string]any{
		"options": map[string]any{
			providerSlug: options,
		},
	}
}

func resolveAudioEnabled(metadata map[string]any) *bool {
	if metadata == nil {
		return nil
	}
	for _, key := range []string{"audio", "generate_audio"} {
		value, ok := metadata[key]
		if !ok {
			continue
		}
		boolean, ok := value.(bool)
		if !ok {
			continue
		}
		return &boolean
	}
	return nil
}

func stringMetadata(metadata map[string]any, key string) string {
	if metadata == nil {
		return ""
	}
	value, ok := metadata[key]
	if !ok {
		return ""
	}
	str, ok := value.(string)
	if !ok {
		return ""
	}
	return str
}

func firstString(payload map[string]any, paths ...string) string {
	for _, path := range paths {
		if value := nestedValue(payload, path); value != nil {
			switch typed := value.(type) {
			case string:
				if strings.TrimSpace(typed) != "" {
					return strings.TrimSpace(typed)
				}
			}
		}
	}
	return ""
}

func firstInt64(payload map[string]any, paths ...string) int64 {
	for _, path := range paths {
		if value := nestedValue(payload, path); value != nil {
			switch typed := value.(type) {
			case float64:
				if math.IsNaN(typed) {
					return 0
				}
				if typed >= math.MaxInt64 {
					return math.MaxInt64
				}
				if typed <= math.MinInt64 {
					return math.MinInt64
				}
				return int64(typed)
			case int64:
				return typed
			case int:
				return int64(typed)
			case string:
				if parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64); err == nil {
					return parsed
				}
				if parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64); err == nil {
					switch {
					case math.IsNaN(parsed):
						return 0
					case parsed >= math.MaxInt64:
						return math.MaxInt64
					case parsed <= math.MinInt64:
						return math.MinInt64
					default:
						return int64(parsed)
					}
				}
			}
		}
	}
	return 0
}

func firstFloat64(payload map[string]any, paths ...string) (float64, bool) {
	for _, path := range paths {
		if value := nestedValue(payload, path); value != nil {
			switch typed := value.(type) {
			case float64:
				return typed, true
			case float32:
				return float64(typed), true
			case int:
				return float64(typed), true
			case int64:
				return float64(typed), true
			case string:
				if parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64); err == nil {
					return parsed, true
				}
			}
		}
	}
	return 0, false
}

func firstMap(payload map[string]any, paths ...string) map[string]any {
	for _, path := range paths {
		if value := nestedValue(payload, path); value != nil {
			if typed, ok := value.(map[string]any); ok && len(typed) > 0 {
				return typed
			}
		}
	}
	return nil
}

func firstURL(payload map[string]any) string {
	return firstString(payload,
		"url",
		"result_url",
		"download_url",
		"video_url",
		"metadata.url",
		"metadata.video_url",
		"data.url",
		"data.video_url",
		"data.output.url",
		"data.output.video_url",
		"data.output.unsigned_url",
		"data.output.unsigned_urls.0",
		"output.url",
		"output.video_url",
		"output.unsigned_url",
		"output.unsigned_urls.0",
		"unsigned_url",
		"unsigned_urls.0",
		"data.unsigned_url",
		"data.unsigned_urls.0",
	)
}

func firstError(payload map[string]any) (message string, code string) {
	message = firstString(payload,
		"error.message",
		"error",
		"message",
		"data.error.message",
		"data.error",
		"data.message",
	)
	code = firstString(payload, "error.code", "data.error.code")
	return message, code
}

func extractUsage(payload map[string]any) map[string]any {
	return firstMap(payload, "usage", "data.usage", "meta.usage", "data.meta.usage")
}

func extractProviderCostUSD(payload map[string]any) (float64, bool) {
	return firstFloat64(payload,
		"provider_cost.usd",
		"data.provider_cost.usd",
		"usage.cost",
		"data.usage.cost",
		"cost",
		"data.cost",
	)
}

func extractUsageTokens(payload map[string]any) (completion int, total int) {
	completion = common.QuotaFromFloat(float64(firstInt64(payload,
		"usage.video_tokens",
		"data.usage.video_tokens",
		"usage.output_tokens",
		"data.usage.output_tokens",
		"usage.completion_tokens",
		"data.usage.completion_tokens",
		"usage.total_tokens",
		"data.usage.total_tokens",
	)))
	total = common.QuotaFromFloat(float64(firstInt64(payload,
		"usage.total_tokens",
		"data.usage.total_tokens",
		"usage.video_tokens",
		"data.usage.video_tokens",
		"usage.output_tokens",
		"data.usage.output_tokens",
	)))
	if total == 0 {
		total = completion
	}
	return completion, total
}

func extractDurationSeconds(payload map[string]any) int {
	duration := firstInt64(payload,
		"duration",
		"seconds",
		"duration_seconds",
		"data.duration",
		"data.seconds",
		"data.duration_seconds",
		"output.duration",
		"output.seconds",
		"output.duration_seconds",
		"data.output.duration",
		"data.output.seconds",
		"data.output.duration_seconds",
	)
	if duration > 0 {
		return int(min(duration, int64(relaycommon.MaxTaskDurationSeconds)))
	}
	return 0
}

func nestedValue(payload map[string]any, path string) any {
	parts := strings.Split(path, ".")
	var current any = payload
	for _, part := range parts {
		switch typed := current.(type) {
		case map[string]any:
			current = typed[part]
		case []any:
			index, err := strconv.Atoi(part)
			if err != nil || index < 0 || index >= len(typed) {
				return nil
			}
			current = typed[index]
		default:
			return nil
		}
	}
	return current
}

func mapStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "queued", "pending", "created":
		return "queued"
	case "processing", "running", "in_progress", "generating":
		return "in_progress"
	case "completed", "succeeded", "success", "done":
		return "completed"
	case "failed", "error", "canceled", "cancelled":
		return "failed"
	default:
		// Unknown / empty: do not pretend the task is still queued.
		return ""
	}
}

func mapTaskStatus(status string) string {
	switch mapStatus(status) {
	case "queued":
		return "PENDING"
	case "in_progress":
		return "IN_PROGRESS"
	case "completed":
		return "SUCCESS"
	case "failed":
		return "FAILURE"
	default:
		return ""
	}
}

func mapProgress(status string) string {
	switch mapStatus(status) {
	case "queued":
		return "20%"
	case "in_progress":
		return "50%"
	case "completed", "failed":
		return "100%"
	default:
		return ""
	}
}

func isRateLimitError(code, message string) bool {
	normalizedCode := strings.ToLower(strings.TrimSpace(code))
	normalizedMessage := strings.ToLower(strings.TrimSpace(message))
	switch normalizedCode {
	case "429", "rate_limit_error", "rate_limit_exceeded", "too_many_requests":
		return true
	}
	if strings.Contains(normalizedMessage, "rate limit") ||
		strings.Contains(normalizedMessage, "too many requests") {
		return true
	}
	return false
}
