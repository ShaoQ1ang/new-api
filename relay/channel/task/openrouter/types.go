package openrouter

import (
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

type VideoBillingContext struct {
	DurationSeconds int
	ResolutionTier  string
	AudioEnabled    *bool
	OtherRatios     map[string]float64
	ProviderUsage   map[string]any
	ProviderCostUSD *float64
}

type VideoSubmitResult struct {
	UpstreamTaskID string
	PublicResponse *dto.OpenAIVideo
}

type ModelHandler interface {
	Family() string
	Match(model string) bool
	Validate(req *relaycommon.TaskSubmitReq) error
	BuildUpstreamRequest(info *relaycommon.RelayInfo, req *relaycommon.TaskSubmitReq) (map[string]any, error)
	EstimateBillingContext(req *relaycommon.TaskSubmitReq) (*VideoBillingContext, error)
	ParseSubmitResponse(info *relaycommon.RelayInfo, body []byte) (*VideoSubmitResult, error)
	ParseFetchResponse(info *relaycommon.RelayInfo, body []byte) (*relaycommon.TaskInfo, error)
	ConvertToOpenAIVideo(task *model.Task) ([]byte, error)
}

type BaseHandler struct {
	family string
}

func NewBaseHandler(family string) BaseHandler {
	return BaseHandler{family: family}
}

func (h BaseHandler) Family() string {
	return h.family
}

func (h BaseHandler) Match(string) bool {
	return false
}

func (h BaseHandler) Validate(req *relaycommon.TaskSubmitReq) error {
	if strings.TrimSpace(req.Model) == "" {
		return errf("model is required")
	}
	if strings.TrimSpace(req.Prompt) == "" {
		return errf("prompt is required")
	}
	return nil
}

func (h BaseHandler) BuildUpstreamRequest(info *relaycommon.RelayInfo, req *relaycommon.TaskSubmitReq) (map[string]any, error) {
	body := map[string]any{
		"model":  info.UpstreamModelName,
		"prompt": req.Prompt,
	}
	if req.Duration > 0 {
		body["duration"] = req.Duration
		body["seconds"] = req.Duration
	} else if strings.TrimSpace(req.Seconds) != "" {
		body["seconds"] = strings.TrimSpace(req.Seconds)
	}
	if req.Size != "" {
		body["size"] = req.Size
	}
	if req.Mode != "" {
		body["mode"] = req.Mode
	}
	if req.InputReference != "" {
		body["input_reference"] = req.InputReference
	}
	switch {
	case req.Image != "":
		body["image"] = req.Image
	case len(req.Images) == 1:
		body["image"] = req.Images[0]
	case len(req.Images) > 1:
		body["images"] = req.Images
	}
	if len(req.Videos) == 1 {
		body["video"] = req.Videos[0]
	} else if len(req.Videos) > 1 {
		body["videos"] = req.Videos
	}
	for k, v := range req.Metadata {
		if _, exists := body[k]; exists || k == "model" {
			continue
		}
		body[k] = v
	}
	return body, nil
}

func (h BaseHandler) EstimateBillingContext(req *relaycommon.TaskSubmitReq) (*VideoBillingContext, error) {
	duration := req.Duration
	if duration <= 0 {
		if seconds, err := strconv.Atoi(strings.TrimSpace(req.Seconds)); err == nil && seconds > 0 {
			duration = seconds
		}
	}
	if duration <= 0 {
		duration = 5
	}
	tier := normalizeResolutionTier(req)
	audio := resolveAudioEnabled(req.Metadata)
	return &VideoBillingContext{
		DurationSeconds: duration,
		ResolutionTier:  tier,
		AudioEnabled:    audio,
		OtherRatios: map[string]float64{
			"seconds": float64(duration),
		},
	}, nil
}

func (h BaseHandler) ParseSubmitResponse(info *relaycommon.RelayInfo, body []byte) (*VideoSubmitResult, error) {
	payload, err := decodeObject(body)
	if err != nil {
		return nil, err
	}
	upstreamTaskID := firstString(payload, "id", "task_id", "data.id", "data.task_id")
	if upstreamTaskID == "" {
		return nil, errf("task id is empty")
	}
	video := dto.NewOpenAIVideo()
	video.ID = info.PublicTaskID
	video.TaskID = info.PublicTaskID
	video.Model = info.OriginModelName
	// Submit responses without a status are treated as queued once we have a task id.
	status := mapStatus(firstString(payload, "status", "state", "data.status", "data.state"))
	if status == "" {
		status = "queued"
	}
	video.Status = status
	video.SetProgressStr(mapProgress(status))
	video.CreatedAt = firstInt64(payload, "created_at", "created", "data.created_at", "data.created")
	video.SetMetadata("upstream_task_id", upstreamTaskID)
	if pollingURL := firstString(payload, "polling_url", "data.polling_url"); pollingURL != "" {
		video.SetMetadata("polling_url", pollingURL)
	}
	if url := firstURL(payload); url != "" {
		video.SetMetadata("url", url)
	}
	if usage := extractUsage(payload); usage != nil {
		video.SetMetadata("usage", usage)
	}
	if providerCostUSD, ok := extractProviderCostUSD(payload); ok {
		video.SetMetadata("provider_cost_usd", providerCostUSD)
	}
	if message, code := firstError(payload); message != "" && video.Status == "failed" {
		video.Error = &dto.OpenAIVideoError{
			Message: message,
			Code:    code,
		}
	}
	return &VideoSubmitResult{UpstreamTaskID: upstreamTaskID, PublicResponse: video}, nil
}

func (h BaseHandler) ParseFetchResponse(_ *relaycommon.RelayInfo, body []byte) (*relaycommon.TaskInfo, error) {
	payload, err := decodeObject(body)
	if err != nil {
		return nil, err
	}
	statusText := firstString(payload, "status", "state", "data.status", "data.state")
	reason, code := firstError(payload)
	taskID := firstString(payload, "id", "task_id", "data.id", "data.task_id")

	// Empty / unknown status must not be silently mapped to PENDING.
	if strings.TrimSpace(statusText) == "" || mapStatus(statusText) == "" {
		if reason != "" {
			if isRateLimitError(code, reason) {
				// Transient: let the poller keep the previous task state.
				return nil, errf("upstream rate limited: %s", reason)
			}
			return &relaycommon.TaskInfo{
				Status:   model.TaskStatusFailure,
				TaskID:   taskID,
				Reason:   reason,
				Progress: "100%",
			}, nil
		}
		if strings.TrimSpace(statusText) == "" {
			return nil, errf("task status is empty")
		}
		return nil, errf("unknown task status: %s", statusText)
	}

	status := mapTaskStatus(statusText)
	taskInfo := &relaycommon.TaskInfo{
		Status:   status,
		TaskID:   taskID,
		Progress: mapProgress(statusText),
	}
	if reason != "" && status == model.TaskStatusFailure {
		taskInfo.Reason = reason
	}
	if url := firstURL(payload); url != "" {
		taskInfo.Url = url
	}
	taskInfo.DurationSeconds = extractDurationSeconds(payload)
	taskInfo.CompletionTokens, taskInfo.TotalTokens = extractUsageTokens(payload)
	return taskInfo, nil
}

func (h BaseHandler) ConvertToOpenAIVideo(task *model.Task) ([]byte, error) {
	video := task.ToOpenAIVideo()
	video.TaskID = task.TaskID
	payload, err := decodeObject(task.Data)
	if err == nil {
		if status := firstString(payload, "status", "state", "data.status", "data.state"); status != "" {
			video.Status = mapStatus(status)
			video.SetProgressStr(mapProgress(status))
		}
		if createdAt := firstInt64(payload, "created_at", "created", "data.created_at", "data.created"); createdAt > 0 {
			video.CreatedAt = createdAt
		}
		if completedAt := firstInt64(payload, "completed_at", "completed", "data.completed_at", "data.completed"); completedAt > 0 {
			video.CompletedAt = completedAt
		}
		if pollingURL := firstString(payload, "polling_url", "data.polling_url"); pollingURL != "" {
			video.SetMetadata("polling_url", pollingURL)
		}
		if url := firstURL(payload); url != "" {
			video.SetMetadata("url", url)
		}
		if usage := extractUsage(payload); usage != nil {
			video.SetMetadata("usage", usage)
		}
		if providerCostUSD, ok := extractProviderCostUSD(payload); ok {
			video.SetMetadata("provider_cost_usd", providerCostUSD)
		}
		if message, code := firstError(payload); message != "" && video.Status == "failed" {
			video.Error = &dto.OpenAIVideoError{
				Message: message,
				Code:    code,
			}
		}
	}
	return common.Marshal(video)
}

func decodeObject(body []byte) (map[string]any, error) {
	var payload map[string]any
	if err := common.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	return payload, nil
}
