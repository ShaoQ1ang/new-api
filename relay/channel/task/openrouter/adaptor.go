package openrouter

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/tracelog"
	"github.com/QuantumNous/new-api/relay/channel"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

type TaskAdaptor struct {
	ChannelType int
	apiKey      string
	baseURL     string
}

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	a.ChannelType = info.ChannelType
	a.baseURL = strings.TrimRight(info.ChannelBaseUrl, "/")
	a.apiKey = info.ApiKey
}

func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) *dto.TaskError {
	var req relaycommon.TaskSubmitReq
	if err := common.UnmarshalBodyReusable(c, &req); err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}
	if len(req.Images) == 0 {
		switch {
		case strings.TrimSpace(req.InputReference) != "":
			req.Images = []string{req.InputReference}
		case strings.TrimSpace(req.Image) != "":
			req.Images = []string{req.Image}
		}
	}
	tracelog.Default.LogValue(c.Request.Context(), "newapi.input", "server-request", func() any { return req })
	handler := selectRequestHandler(info, req.Model)
	resolvedReq := requestForHandler(info, &req)
	if err := handler.Validate(&resolvedReq); err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}
	frameImages, _ := requestFrameImages(&resolvedReq)
	inputReferences, _ := requestInputReferences(&resolvedReq)
	hasVideoReference := false
	for _, reference := range inputReferences {
		if reference["type"] == "video_url" {
			hasVideoReference = true
			break
		}
	}
	info.Action = constant.TaskActionTextGenerate
	if len(req.Videos) > 0 || hasVideoReference {
		info.Action = constant.TaskActionRemix
	} else if len(req.Images) > 1 || len(frameImages) > 1 {
		info.Action = constant.TaskActionFirstTailGenerate
	} else if len(req.Images) == 1 || len(frameImages) == 1 || len(inputReferences) > 0 {
		info.Action = constant.TaskActionGenerate
	}
	c.Set("task_request", req)
	return nil
}

func (a *TaskAdaptor) EstimateBilling(c *gin.Context, info *relaycommon.RelayInfo) map[string]float64 {
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil
	}
	handler := selectRequestHandler(info, req.Model)
	resolvedReq := requestForHandler(info, &req)
	ctx, err := handler.EstimateBillingContext(&resolvedReq)
	if err != nil || ctx == nil {
		return nil
	}
	ratios := map[string]float64{}
	if ctx.DurationSeconds > 0 {
		ratios["seconds"] = float64(ctx.DurationSeconds)
	}
	if ctx.ResolutionTier != "" {
		ratios["resolution-"+ctx.ResolutionTier] = 1
	}
	for k, v := range ctx.OtherRatios {
		ratios[k] = v
	}
	return ratios
}

func (a *TaskAdaptor) BuildRequestURL(_ *relaycommon.RelayInfo) (string, error) {
	return fmt.Sprintf("%s/v1/videos", a.baseURL), nil
}

func (a *TaskAdaptor) BuildRequestHeader(_ *gin.Context, req *http.Request, _ *relaycommon.RelayInfo) error {
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("HTTP-Referer", "https://www.newapi.ai")
	req.Header.Set("X-OpenRouter-Title", "New API")
	return nil
}

func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil, err
	}
	handler := selectRequestHandler(info, req.Model)
	resolvedReq := requestForHandler(info, &req)
	body, err := handler.BuildUpstreamRequest(info, &resolvedReq)
	if err != nil {
		return nil, err
	}
	bodyBytes, err := common.Marshal(body)
	if err != nil {
		return nil, err
	}
	tracelog.Default.LogValue(c.Request.Context(), "newapi.upstream.request", "client-request", func() any { return body })
	return bytes.NewReader(bodyBytes), nil
}

func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	return channel.DoTaskApiRequest(a, c, info, requestBody)
}

func (a *TaskAdaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (taskID string, taskData []byte, publicResponse any, taskErr *dto.TaskError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, nil, service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
	}
	tracelog.Default.LogHTTPResponse(c.Request.Context(), "newapi.upstream.response", resp, responseBody)
	_ = resp.Body.Close()
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return "", nil, nil, service.TaskErrorWrapper(err, "request_not_found", http.StatusInternalServerError)
	}
	handler := selectRequestHandler(info, req.Model)
	result, err := handler.ParseSubmitResponse(info, responseBody)
	if err != nil {
		return "", nil, nil, service.TaskErrorWrapper(err, "unmarshal_response_body_failed", http.StatusInternalServerError)
	}
	tracelog.Default.LogResponseValue(c.Request.Context(), "newapi.output", "server-response", func() any { return result.PublicResponse })
	return result.UpstreamTaskID, responseBody, result.PublicResponse, nil
}

func (a *TaskAdaptor) FetchTask(baseURL, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, _ := body["task_id"].(string)
	if strings.TrimSpace(taskID) == "" {
		return nil, fmt.Errorf("invalid task_id")
	}
	uri := fmt.Sprintf("%s/v1/videos/%s", strings.TrimRight(baseURL, "/"), taskID)
	req, err := http.NewRequest(http.MethodGet, uri, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("HTTP-Referer", "https://www.newapi.ai")
	req.Header.Set("X-OpenRouter-Title", "New API")
	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, fmt.Errorf("new proxy http client failed: %w", err)
	}
	return client.Do(req)
}

func (a *TaskAdaptor) ParseTaskResult(respBody []byte) (*relaycommon.TaskInfo, error) {
	return a.ParseTaskResultForModel("", respBody)
}

func (a *TaskAdaptor) ParseTaskResultForModel(modelName string, respBody []byte) (*relaycommon.TaskInfo, error) {
	handler := SelectHandler(modelName)
	return handler.ParseFetchResponse(nil, respBody)
}

func (a *TaskAdaptor) GetModelList() []string {
	return nil
}

func (a *TaskAdaptor) GetChannelName() string {
	return "openrouter-video"
}

func (a *TaskAdaptor) ConvertToOpenAIVideo(originTask *model.Task) ([]byte, error) {
	handler := SelectHandler(originTask.Properties.UpstreamModelName)
	if strings.TrimSpace(originTask.Properties.UpstreamModelName) == "" {
		handler = SelectHandler(originTask.Properties.OriginModelName)
	}
	return handler.ConvertToOpenAIVideo(originTask)
}

func (a *TaskAdaptor) AdjustBillingOnSubmit(_ *relaycommon.RelayInfo, _ []byte) map[string]float64 {
	return nil
}

func (a *TaskAdaptor) AdjustBillingOnComplete(task *model.Task, taskResult *relaycommon.TaskInfo) int {
	quota, _ := a.AdjustBillingOnCompleteChecked(task, taskResult)
	return quota
}

func (a *TaskAdaptor) AdjustBillingOnCompleteChecked(task *model.Task, taskResult *relaycommon.TaskInfo) (int, *common.QuotaClamp) {
	if task == nil || taskResult == nil {
		return 0, nil
	}
	bc := task.PrivateData.BillingContext
	if bc == nil {
		return 0, nil
	}
	groupRatio := bc.GroupRatio
	if groupRatio <= 0 {
		groupRatio = 1
	}
	if bc.VideoSecondsUnitPrice > 0 {
		duration := taskResult.DurationSeconds
		if duration <= 0 {
			duration = bc.VideoDurationSeconds
		}
		if duration > 0 {
			return common.QuotaFromFloatChecked((bc.VideoSecondsUnitPrice*float64(duration) + bc.VideoFixedPrice) * groupRatio * common.QuotaPerUnit)
		}
	}
	if taskResult.TotalTokens <= 0 {
		return 0, nil
	}
	if bc.ConditionalInputPrice > 0 {
		return common.QuotaFromFloatChecked(bc.ConditionalInputPrice / 1000000 * float64(taskResult.TotalTokens) * groupRatio * common.QuotaPerUnit)
	}
	if bc.ModelRatio <= 0 {
		return 0, nil
	}
	otherMultiplier := 1.0
	for _, ratio := range bc.OtherRatios {
		if ratio > 0 && ratio != 1 {
			otherMultiplier *= ratio
		}
	}
	return common.QuotaFromFloatChecked(float64(taskResult.TotalTokens) * bc.ModelRatio * groupRatio * otherMultiplier)
}
