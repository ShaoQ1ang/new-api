package openrouter

import (
	"fmt"
	"strconv"
	"strings"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

func errf(format string, args ...any) error {
	return fmt.Errorf(format, args...)
}

func normalizeResolutionTier(req *relaycommon.TaskSubmitReq) string {
	candidates := []string{
		req.Size,
		stringMetadata(req.Metadata, "resolution"),
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
	case "720p", "1280x720", "720x1280", "1280*720", "720*1280", "960*960", "1088*832", "832*1088", "std":
		return "720p"
	case "1080p", "1920x1080", "1080x1920", "1920*1080", "1080*1920", "1440*1440", "1632*1248", "1248*1632", "pro":
		return "1080p"
	case "2k":
		return "2k"
	case "4k":
		return "4k"
	}
	return ""
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
				return int64(typed)
			case int64:
				return typed
			case int:
				return int64(typed)
			case string:
				if parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64); err == nil {
					return parsed
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
	completion = int(firstInt64(payload,
		"usage.video_tokens",
		"data.usage.video_tokens",
		"usage.output_tokens",
		"data.usage.output_tokens",
		"usage.completion_tokens",
		"data.usage.completion_tokens",
		"usage.total_tokens",
		"data.usage.total_tokens",
	))
	total = int(firstInt64(payload,
		"usage.total_tokens",
		"data.usage.total_tokens",
		"usage.video_tokens",
		"data.usage.video_tokens",
		"usage.output_tokens",
		"data.usage.output_tokens",
	))
	if total == 0 {
		total = completion
	}
	return completion, total
}

func extractDurationSeconds(payload map[string]any) int {
	duration := int(firstInt64(payload,
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
	))
	if duration > 0 {
		return duration
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
