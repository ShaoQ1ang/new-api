package execution

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strconv"
	"strings"

	aigcdto "github.com/QuantumNous/new-api/aigc/dto"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaydto "github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

type TaskSubmitter interface {
	Submit(*gin.Context, *relaycommon.RelayInfo) (*relay.TaskWorkflowResult, *relaydto.TaskError)
}

type TaskStore interface {
	Get(userID int, taskID string) (*model.Task, bool, error)
}

type ModelTaskStore struct{}

func (ModelTaskStore) Get(userID int, taskID string) (*model.Task, bool, error) {
	return model.GetByTaskId(userID, taskID)
}

type TaskExecutor struct {
	workflow TaskSubmitter
	tasks    TaskStore
}

func NewTaskExecutor(workflow TaskSubmitter, tasks TaskStore) *TaskExecutor {
	return &TaskExecutor{workflow: workflow, tasks: tasks}
}

type ginContextKey struct{}

func WithGinContext(ctx context.Context, c *gin.Context) context.Context {
	return context.WithValue(ctx, ginContextKey{}, c)
}

func GinContextFrom(ctx context.Context) (*gin.Context, bool) {
	c, ok := ctx.Value(ginContextKey{}).(*gin.Context)
	return c, ok && c != nil
}

func (executor *TaskExecutor) Execute(ctx context.Context, identity Identity, spec Spec) (Result, error) {
	source, ok := GinContextFrom(ctx)
	if !ok || source.Request == nil {
		return Result{}, executionError(http.StatusInternalServerError, "AIGC_EXECUTION_CONTEXT_MISSING", "authenticated AIGC execution context is missing", false)
	}
	request, err := BuildTaskRequest(spec)
	if err != nil {
		return Result{}, executionError(http.StatusBadRequest, "AIGC_EXECUTION_INVALID_SPEC", err.Error(), false)
	}
	body, err := common.Marshal(request)
	if err != nil {
		return Result{}, executionError(http.StatusInternalServerError, "AIGC_EXECUTION_REQUEST_ENCODE_FAILED", "failed to encode AIGC task request", false)
	}

	taskContext := source.Copy()
	taskContext.Set(common.KeyBodyStorage, nil)
	taskContext.Set(common.KeyRequestBody, nil)
	defer common.CleanupBodyStorage(taskContext)
	taskContext.Request = source.Request.Clone(source.Request.Context())
	taskContext.Request.Method = http.MethodPost
	taskContext.Request.URL.Path = "/v1/videos"
	taskContext.Request.URL.RawPath = ""
	taskContext.Request.URL.RawQuery = ""
	taskContext.Request.Header.Set("Content-Type", "application/json")
	taskContext.Request.Body = io.NopCloser(bytes.NewReader(body))
	taskContext.Request.ContentLength = int64(len(body))
	common.SetContextKey(taskContext, constant.ContextKeyOriginalModel, spec.UpstreamModelID)
	common.SetContextKey(taskContext, constant.ContextKeyUsingGroup, identity.Group)
	common.SetContextKey(taskContext, constant.ContextKeyUserId, identity.UserID)
	common.SetContextKey(taskContext, constant.ContextKeyTokenId, identity.TokenID)

	info, err := relaycommon.GenRelayInfo(taskContext, types.RelayFormatTask, nil, nil)
	if err != nil {
		return Result{}, executionError(http.StatusInternalServerError, "AIGC_EXECUTION_RELAY_INFO_FAILED", "failed to initialize AIGC task execution", false)
	}
	workflowResult, taskErr := executor.workflow.Submit(taskContext, info)
	if taskErr != nil {
		return Result{}, taskExecutionError(taskErr)
	}
	if workflowResult == nil || workflowResult.Task == nil || strings.TrimSpace(workflowResult.Task.TaskID) == "" {
		return Result{}, executionError(http.StatusBadGateway, "AIGC_EXECUTION_EMPTY_RESULT", "AIGC task execution returned no task", false)
	}
	return taskResult(spec, workflowResult.Task), nil
}

func (executor *TaskExecutor) Poll(_ context.Context, identity Identity, spec Spec, nativeTaskID string) (Result, error) {
	task, found, err := executor.tasks.Get(identity.UserID, strings.TrimSpace(nativeTaskID))
	if err != nil {
		return Result{}, executionError(http.StatusInternalServerError, "AIGC_TASK_LOOKUP_FAILED", "failed to load AIGC task", true)
	}
	if !found || task == nil {
		return Result{}, executionError(http.StatusNotFound, "AIGC_TASK_NOT_FOUND", "AIGC task was not found", false)
	}
	return taskResult(spec, task), nil
}

func taskResult(spec Spec, task *model.Task) Result {
	result := Result{
		Status: taskStatus(task.Status), Progress: taskProgress(task.Progress), NativeTaskID: task.TaskID,
		Outputs: make([]aigcdto.GenerationOutputItem, 0), Usage: &aigcdto.GenerationUsage{Quota: task.Quota},
	}
	if task.Status == model.TaskStatusSuccess && spec.ModelType == "video" {
		output := aigcdto.GenerationOutputItem{ID: task.TaskID, Type: "video", URL: task.GetResultURL()}
		if task.PrivateData.BillingContext != nil {
			output.Duration = task.PrivateData.BillingContext.VideoDurationSeconds
		}
		result.Outputs = append(result.Outputs, output)
	}
	if task.Status == model.TaskStatusFailure {
		result.ErrorCode = "AIGC_TASK_FAILED"
		result.ErrorMessage = task.FailReason
	}
	return result
}

func taskStatus(status model.TaskStatus) string {
	switch status {
	case model.TaskStatusNotStart, model.TaskStatusSubmitted, model.TaskStatusQueued:
		return "queued"
	case model.TaskStatusInProgress:
		return "processing"
	case model.TaskStatusSuccess:
		return "completed"
	case model.TaskStatusFailure:
		return "failed"
	default:
		return "failed"
	}
}

func taskProgress(progress string) int {
	value := strings.TrimSpace(strings.TrimSuffix(progress, "%"))
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	if parsed < 0 {
		return 0
	}
	if parsed > 100 {
		return 100
	}
	return parsed
}

func taskExecutionError(taskErr *relaydto.TaskError) error {
	status := taskErr.StatusCode
	if status == 0 {
		status = http.StatusBadGateway
	}
	message := strings.TrimSpace(taskErr.Message)
	if message == "" && taskErr.Error != nil {
		message = taskErr.Error.Error()
	}
	if message == "" {
		message = "AIGC task execution failed"
	}
	code := strings.TrimSpace(taskErr.Code)
	if code == "" {
		code = "AIGC_EXECUTION_FAILED"
	}
	return executionError(status, code, message, !taskErr.LocalError && (status == http.StatusTooManyRequests || status >= http.StatusInternalServerError))
}

func executionError(status int, code, message string, retryable bool) *Error {
	return &Error{HTTPStatus: status, Code: code, Message: message, Retryable: retryable}
}
