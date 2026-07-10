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
	handler := SelectHandler(req.Model)
	if err := handler.Validate(&req); err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}
	info.Action = constant.TaskActionTextGenerate
	if len(req.Videos) > 0 {
		info.Action = constant.TaskActionRemix
	} else if len(req.Images) > 1 {
		info.Action = constant.TaskActionFirstTailGenerate
	} else if len(req.Images) == 1 {
		info.Action = constant.TaskActionGenerate
	}
	c.Set("task_request", req)
	return nil
}

func (a *TaskAdaptor) EstimateBilling(c *gin.Context, _ *relaycommon.RelayInfo) map[string]float64 {
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil
	}
	handler := SelectHandler(req.Model)
	ctx, err := handler.EstimateBillingContext(&req)
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
	handler := SelectHandler(req.Model)
	body, err := handler.BuildUpstreamRequest(info, &req)
	if err != nil {
		return nil, err
	}
	bodyBytes, err := common.Marshal(body)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(bodyBytes), nil
}

func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	return channel.DoTaskApiRequest(a, c, info, requestBody)
}

func (a *TaskAdaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (taskID string, taskData []byte, taskErr *dto.TaskError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
	}
	_ = resp.Body.Close()
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return "", nil, service.TaskErrorWrapper(err, "request_not_found", http.StatusInternalServerError)
	}
	handler := SelectHandler(req.Model)
	result, err := handler.ParseSubmitResponse(info, responseBody)
	if err != nil {
		return "", nil, service.TaskErrorWrapper(err, "unmarshal_response_body_failed", http.StatusInternalServerError)
	}
	c.JSON(http.StatusOK, result.PublicResponse)
	return result.UpstreamTaskID, responseBody, nil
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
	handler := SelectHandler(originTask.Properties.OriginModelName)
	return handler.ConvertToOpenAIVideo(originTask)
}

func (a *TaskAdaptor) AdjustBillingOnSubmit(_ *relaycommon.RelayInfo, _ []byte) map[string]float64 {
	return nil
}

func (a *TaskAdaptor) AdjustBillingOnComplete(task *model.Task, taskResult *relaycommon.TaskInfo) int {
	if task == nil || taskResult == nil {
		return 0
	}
	bc := task.PrivateData.BillingContext
	if bc == nil {
		return 0
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
			return int(bc.VideoSecondsUnitPrice * float64(duration) * groupRatio * common.QuotaPerUnit)
		}
	}
	if taskResult.TotalTokens <= 0 {
		return 0
	}
	if bc.ConditionalInputPrice > 0 {
		return int(bc.ConditionalInputPrice / 1000000 * float64(taskResult.TotalTokens) * groupRatio * common.QuotaPerUnit)
	}
	if bc.ModelRatio <= 0 {
		return 0
	}
	otherMultiplier := 1.0
	for _, ratio := range bc.OtherRatios {
		if ratio > 0 && ratio != 1 {
			otherMultiplier *= ratio
		}
	}
	return int(float64(taskResult.TotalTokens) * bc.ModelRatio * groupRatio * otherMultiplier)
}
