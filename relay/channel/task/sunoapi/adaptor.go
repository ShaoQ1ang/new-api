package sunoapi

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	taskcommon "github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

const (
	generatePath = "/api/v1/generate"
	recordPath   = "/api/v1/generate/record-info"
)

var modelList = []string{"V4", "V4_5", "V4_5PLUS", "V4_5ALL", "V5", "V5_5"}

type TaskAdaptor struct {
	taskcommon.BaseBilling
	baseURL string
	apiKey  string
}

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	a.baseURL = strings.TrimRight(info.ChannelBaseUrl, "/")
	a.apiKey = info.ApiKey
}

func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) *dto.TaskError {
	return relaycommon.ValidateBasicTaskRequest(c, info, constant.TaskActionGenerate)
}

func (a *TaskAdaptor) BuildRequestURL(_ *relaycommon.RelayInfo) (string, error) {
	return a.baseURL + generatePath, nil
}

func (a *TaskAdaptor) BuildRequestHeader(_ *gin.Context, req *http.Request, _ *relaycommon.RelayInfo) error {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	return nil
}

func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, _ *relaycommon.RelayInfo) (io.Reader, error) {
	value, ok := c.Get("task_request")
	if !ok {
		return nil, fmt.Errorf("task request not found")
	}
	req, ok := value.(relaycommon.TaskSubmitReq)
	if !ok {
		return nil, fmt.Errorf("invalid task request type")
	}
	var upstream dto.SunoAPIGenerateRequest
	if err := taskcommon.UnmarshalMetadata(req.Metadata, &upstream); err != nil {
		return nil, err
	}
	upstream.Model = req.Model
	upstream.Instrumental = req.Metadata != nil && req.Metadata["instrumental"] == true
	if upstream.Model == "" || strings.TrimSpace(upstream.CallBackURL) == "" {
		return nil, fmt.Errorf("model and callback URL are required")
	}
	lyrics, _ := req.Metadata["lyrics"].(string)
	if upstream.CustomMode {
		upstream.Prompt = strings.TrimSpace(lyrics)
		if upstream.Instrumental {
			upstream.Prompt = ""
		}
	} else {
		upstream.Prompt = strings.TrimSpace(req.Prompt)
	}
	body, err := common.Marshal(upstream)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(body), nil
}

func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, body io.Reader) (*http.Response, error) {
	return channel.DoTaskApiRequest(a, c, info, body)
}

func (a *TaskAdaptor) DoResponse(_ *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (string, []byte, any, *dto.TaskError) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, nil, service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
	}
	var result dto.SunoAPIResponse[dto.SunoAPISubmitData]
	if err := common.Unmarshal(body, &result); err != nil {
		return "", nil, nil, service.TaskErrorWrapper(err, "unmarshal_response_body_failed", http.StatusBadGateway)
	}
	if resp.StatusCode/100 != 2 || result.Code != http.StatusOK || strings.TrimSpace(result.Data.TaskID) == "" {
		status := resp.StatusCode
		if status/100 == 2 {
			status = http.StatusBadGateway
		}
		return "", nil, nil, service.TaskErrorWrapper(fmt.Errorf("sunoapi: %s", result.Msg), fmt.Sprintf("%d", result.Code), status)
	}
	public := dto.TaskResponse[string]{Code: "success", Message: result.Msg, Data: info.PublicTaskID}
	return result.Data.TaskID, body, public, nil
}

func (a *TaskAdaptor) FetchTask(baseURL, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, ok := body["task_id"].(string)
	if !ok || strings.TrimSpace(taskID) == "" {
		return nil, fmt.Errorf("invalid task_id")
	}
	requestURL := strings.TrimRight(baseURL, "/") + recordPath + "?taskId=" + url.QueryEscape(taskID)
	req, err := http.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)
	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, fmt.Errorf("new proxy http client failed: %w", err)
	}
	return client.Do(req)
}

func (a *TaskAdaptor) ParseTaskResult(body []byte) (*relaycommon.TaskInfo, error) {
	var response dto.SunoAPIResponse[dto.SunoAPIRecordData]
	if err := common.Unmarshal(body, &response); err != nil {
		return nil, err
	}
	if response.Code != http.StatusOK {
		return nil, fmt.Errorf("sunoapi: %s", response.Msg)
	}
	result := &relaycommon.TaskInfo{TaskID: response.Data.TaskID}
	switch response.Data.Status {
	case "PENDING":
		result.Status, result.Progress = model.TaskStatusQueued, "10%"
	case "TEXT_SUCCESS":
		result.Status, result.Progress = model.TaskStatusInProgress, "35%"
	case "FIRST_SUCCESS":
		result.Status, result.Progress = model.TaskStatusInProgress, "70%"
	case "SUCCESS":
		ready := len(response.Data.Response.SunoData) == 2
		for _, song := range response.Data.Response.SunoData {
			ready = ready && strings.TrimSpace(song.AudioURL) != ""
		}
		if ready {
			result.Status, result.Progress = model.TaskStatusSuccess, "100%"
		} else {
			result.Status, result.Progress = model.TaskStatusInProgress, "90%"
		}
	case "CALLBACK_EXCEPTION":
		result.Status, result.Progress = model.TaskStatusInProgress, "70%"
	case "CREATE_TASK_FAILED", "GENERATE_AUDIO_FAILED", "SENSITIVE_WORD_ERROR":
		result.Status, result.Progress = model.TaskStatusFailure, "100%"
		result.Reason = strings.TrimSpace(response.Data.ErrorMessage)
		if result.Reason == "" {
			result.Reason = response.Data.Status
		}
	default:
		return nil, fmt.Errorf("unknown sunoapi status %q", response.Data.Status)
	}
	return result, nil
}

func (a *TaskAdaptor) GetModelList() []string { return modelList }
func (a *TaskAdaptor) GetChannelName() string { return "sunoapi-v1" }

func (a *TaskAdaptor) ConvertToOpenAIVideo(task *model.Task) ([]byte, error) {
	return common.Marshal(map[string]any{"id": task.TaskID, "status": task.Status, "created_at": time.Unix(task.CreatedAt, 0).Unix()})
}
