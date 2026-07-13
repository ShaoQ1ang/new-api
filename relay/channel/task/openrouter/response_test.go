package openrouter

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

func TestBaseHandlerParseSubmitResponseIncludesProgressAndMetadata(t *testing.T) {
	handler := NewBaseHandler("default")
	info := &relaycommon.RelayInfo{
		OriginModelName: "google/veo-3.1-lite",
		TaskRelayInfo:   &relaycommon.TaskRelayInfo{PublicTaskID: "task_public"},
	}
	body := []byte(`{
		"id":"vid_123",
		"status":"processing",
		"created_at":1700000000,
		"polling_url":"https://openrouter.ai/api/v1/videos/vid_123",
		"output":{"unsigned_urls":["https://cdn.example.com/video.mp4"]},
		"usage":{"video_tokens":321,"total_tokens":321},
		"provider_cost":{"usd":0.42}
	}`)

	result, err := handler.ParseSubmitResponse(info, body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.UpstreamTaskID != "vid_123" {
		t.Fatalf("expected upstream task id vid_123, got %q", result.UpstreamTaskID)
	}
	if result.PublicResponse.Progress != 50 {
		t.Fatalf("expected progress 50, got %d", result.PublicResponse.Progress)
	}
	if result.PublicResponse.Metadata["polling_url"] != "https://openrouter.ai/api/v1/videos/vid_123" {
		t.Fatalf("expected polling_url metadata, got %#v", result.PublicResponse.Metadata)
	}
	if result.PublicResponse.Metadata["url"] != "https://cdn.example.com/video.mp4" {
		t.Fatalf("expected url metadata, got %#v", result.PublicResponse.Metadata)
	}
	if result.PublicResponse.Metadata["provider_cost_usd"] != 0.42 {
		t.Fatalf("expected provider_cost_usd metadata, got %#v", result.PublicResponse.Metadata["provider_cost_usd"])
	}
	usage, ok := result.PublicResponse.Metadata["usage"].(map[string]any)
	if !ok || usage["video_tokens"] != float64(321) {
		t.Fatalf("expected usage metadata, got %#v", result.PublicResponse.Metadata["usage"])
	}
}

func TestBaseHandlerParseFetchResponseExtractsSuccessURLAndUsage(t *testing.T) {
	handler := NewBaseHandler("default")
	body := []byte(`{
		"data":{
			"id":"vid_456",
			"status":"completed",
			"duration":8,
			"output":{"video_url":"https://cdn.example.com/final.mp4"},
			"usage":{"video_tokens":777,"total_tokens":777}
		}
	}`)

	taskInfo, err := handler.ParseFetchResponse(nil, body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if taskInfo.Status != model.TaskStatusSuccess {
		t.Fatalf("expected SUCCESS, got %q", taskInfo.Status)
	}
	if taskInfo.Url != "https://cdn.example.com/final.mp4" {
		t.Fatalf("expected result url, got %q", taskInfo.Url)
	}
	if taskInfo.DurationSeconds != 8 {
		t.Fatalf("expected duration 8, got %d", taskInfo.DurationSeconds)
	}
	if taskInfo.CompletionTokens != 777 || taskInfo.TotalTokens != 777 {
		t.Fatalf("expected usage tokens 777/777, got %d/%d", taskInfo.CompletionTokens, taskInfo.TotalTokens)
	}
}

func TestBaseHandlerParseFetchResponseExtractsFailureReason(t *testing.T) {
	handler := NewBaseHandler("default")
	body := []byte(`{
		"data":{
			"id":"vid_789",
			"status":"failed",
			"error":{"message":"policy blocked","code":"content_policy"}
		}
	}`)

	taskInfo, err := handler.ParseFetchResponse(nil, body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if taskInfo.Status != model.TaskStatusFailure {
		t.Fatalf("expected FAILURE, got %q", taskInfo.Status)
	}
	if taskInfo.Reason != "policy blocked" {
		t.Fatalf("expected failure reason, got %q", taskInfo.Reason)
	}
}

func TestBaseHandlerParseFetchResponseTreatsErrorBodyWithoutStatusAsFailure(t *testing.T) {
	handler := NewBaseHandler("default")
	body := []byte(`{
		"error":{"message":"provider unavailable","code":"503"}
	}`)

	taskInfo, err := handler.ParseFetchResponse(nil, body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if taskInfo.Status != model.TaskStatusFailure {
		t.Fatalf("expected FAILURE, got %q", taskInfo.Status)
	}
	if taskInfo.Reason != "provider unavailable" {
		t.Fatalf("expected failure reason, got %q", taskInfo.Reason)
	}
}

func TestBaseHandlerParseFetchResponseErrorsWhenStatusMissingWithoutError(t *testing.T) {
	handler := NewBaseHandler("default")
	_, err := handler.ParseFetchResponse(nil, []byte(`{"data":{"id":"vid_000"}}`))
	if err == nil {
		t.Fatal("expected missing status error")
	}
}

func TestBaseHandlerParseFetchResponseErrorsOnUnknownStatus(t *testing.T) {
	handler := NewBaseHandler("default")
	_, err := handler.ParseFetchResponse(nil, []byte(`{"id":"vid_1","status":"weird_state"}`))
	if err == nil {
		t.Fatal("expected unknown status error")
	}
}

func TestBaseHandlerParseFetchResponseRateLimitIsNotFailure(t *testing.T) {
	handler := NewBaseHandler("default")
	body := []byte(`{"error":{"message":"Rate limit exceeded","code":"429"}}`)
	taskInfo, err := handler.ParseFetchResponse(nil, body)
	if err == nil {
		t.Fatal("expected rate limit error")
	}
	if taskInfo != nil {
		t.Fatalf("expected nil task info on rate limit, got %+v", taskInfo)
	}
}

func TestMapStatusDoesNotDefaultUnknownToQueued(t *testing.T) {
	if got := mapStatus("weird_state"); got != "" {
		t.Fatalf("expected empty mapped status, got %q", got)
	}
	if got := mapTaskStatus("weird_state"); got != "" {
		t.Fatalf("expected empty task status, got %q", got)
	}
	if got := mapStatus("processing"); got != "in_progress" {
		t.Fatalf("expected in_progress, got %q", got)
	}
}

func TestBaseHandlerConvertToOpenAIVideoIncludesErrorAndMetadata(t *testing.T) {
	handler := NewBaseHandler("default")
	task := &model.Task{
		TaskID:     "task_public",
		Status:     model.TaskStatusFailure,
		Progress:   "100%",
		CreatedAt:  1700000000,
		UpdatedAt:  1700001234,
		Properties: model.Properties{OriginModelName: "bytedance/seedance-2.0"},
		Data: []byte(`{
			"id":"vid_999",
			"status":"failed",
			"created_at":1700000000,
			"completed_at":1700001234,
			"polling_url":"https://openrouter.ai/api/v1/videos/vid_999",
			"output":{"unsigned_urls":["https://cdn.example.com/failed.mp4"]},
			"usage":{"video_tokens":12,"total_tokens":12},
			"provider_cost":{"usd":0.1},
			"error":{"message":"policy blocked","code":"content_policy"}
		}`),
	}

	raw, err := handler.ConvertToOpenAIVideo(task)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var video dto.OpenAIVideo
	if err := common.Unmarshal(raw, &video); err != nil {
		t.Fatalf("unexpected unmarshal error: %v", err)
	}
	if video.TaskID != "task_public" {
		t.Fatalf("expected task_id task_public, got %q", video.TaskID)
	}
	if video.Error == nil || video.Error.Message != "policy blocked" || video.Error.Code != "content_policy" {
		t.Fatalf("expected video error, got %#v", video.Error)
	}
	if video.Metadata["polling_url"] != "https://openrouter.ai/api/v1/videos/vid_999" {
		t.Fatalf("expected polling_url metadata, got %#v", video.Metadata)
	}
	if video.Metadata["provider_cost_usd"] != 0.1 {
		t.Fatalf("expected provider_cost_usd metadata, got %#v", video.Metadata["provider_cost_usd"])
	}
	usage, ok := video.Metadata["usage"].(map[string]any)
	if !ok || usage["video_tokens"] != float64(12) {
		t.Fatalf("expected usage metadata, got %#v", video.Metadata["usage"])
	}
}
