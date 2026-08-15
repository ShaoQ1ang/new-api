package execution

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	aigcdto "github.com/QuantumNous/new-api/aigc/dto"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaydto "github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

type SyncWorkflow interface {
	Execute(*gin.Context, types.RelayFormat) (*relay.SyncWorkflowResult, *types.NewAPIError)
}

type SyncExecutor struct {
	workflow SyncWorkflow
}

func NewSyncExecutor(workflow SyncWorkflow) *SyncExecutor {
	return &SyncExecutor{workflow: workflow}
}

func (executor *SyncExecutor) Execute(ctx context.Context, identity Identity, spec Spec) (Result, error) {
	source, ok := GinContextFrom(ctx)
	if !ok || source.Request == nil {
		return Result{}, executionError(http.StatusInternalServerError, "AIGC_EXECUTION_CONTEXT_MISSING", "authenticated AIGC execution context is missing", false)
	}
	request, path, format, err := BuildSyncRequest(spec)
	if err != nil {
		return Result{}, executionError(http.StatusBadRequest, "AIGC_EXECUTION_INVALID_SPEC", err.Error(), false)
	}
	body, err := common.Marshal(request)
	if err != nil {
		return Result{}, executionError(http.StatusInternalServerError, "AIGC_EXECUTION_REQUEST_ENCODE_FAILED", "failed to encode AIGC synchronous request", false)
	}

	relayContext := source.Copy()
	relayContext.Set(common.KeyBodyStorage, nil)
	relayContext.Set(common.KeyRequestBody, nil)
	defer common.CleanupBodyStorage(relayContext)
	relayContext.Request = source.Request.Clone(source.Request.Context())
	relayContext.Request.Method = http.MethodPost
	relayContext.Request.URL.Path = path
	relayContext.Request.URL.RawPath = ""
	relayContext.Request.URL.RawQuery = ""
	relayContext.Request.Header.Set("Content-Type", "application/json")
	relayContext.Request.Body = io.NopCloser(bytes.NewReader(body))
	relayContext.Request.ContentLength = int64(len(body))
	common.SetContextKey(relayContext, constant.ContextKeyOriginalModel, spec.UpstreamModelID)
	common.SetContextKey(relayContext, constant.ContextKeyUsingGroup, identity.Group)
	common.SetContextKey(relayContext, constant.ContextKeyUserId, identity.UserID)
	common.SetContextKey(relayContext, constant.ContextKeyTokenId, identity.TokenID)

	workflowResult, workflowErr := executor.workflow.Execute(relayContext, format)
	if workflowErr != nil {
		return Result{}, syncExecutionError(workflowErr)
	}
	if workflowResult == nil || len(workflowResult.ResponseBody) == 0 {
		return Result{}, executionError(http.StatusBadGateway, "AIGC_EXECUTION_EMPTY_RESULT", "AIGC synchronous execution returned no result", false)
	}
	return syncResult(spec, workflowResult.ResponseBody)
}

func (executor *SyncExecutor) Poll(context.Context, Identity, Spec, string) (Result, error) {
	return Result{}, executionError(http.StatusBadRequest, "AIGC_SYNC_POLL_NOT_SUPPORTED", "synchronous AIGC generation cannot be polled", false)
}

func syncResult(spec Spec, body []byte) (Result, error) {
	result := Result{Status: "completed", Progress: 100, Outputs: make([]aigcdto.GenerationOutputItem, 0)}
	switch strings.TrimSpace(spec.ModelType) {
	case "text":
		var response relaydto.OpenAITextResponse
		if err := common.Unmarshal(body, &response); err != nil || len(response.Choices) == 0 {
			return Result{}, executionError(http.StatusBadGateway, "AIGC_EXECUTION_BAD_RESPONSE", "AIGC text execution returned an invalid response", false)
		}
		text := strings.TrimSpace(response.Choices[0].Message.StringContent())
		if text == "" {
			return Result{}, executionError(http.StatusBadGateway, "AIGC_EXECUTION_EMPTY_RESULT", "AIGC text execution returned empty content", false)
		}
		result.Outputs = append(result.Outputs, aigcdto.GenerationOutputItem{ID: response.Id, Type: "text", Text: text})
	case "image":
		var response relaydto.ImageResponse
		if err := common.Unmarshal(body, &response); err != nil || len(response.Data) == 0 {
			return Result{}, executionError(http.StatusBadGateway, "AIGC_EXECUTION_BAD_RESPONSE", "AIGC image execution returned an invalid response", false)
		}
		for index, item := range response.Data {
			url := strings.TrimSpace(item.Url)
			if url == "" {
				return Result{}, executionError(http.StatusBadGateway, "AIGC_IMAGE_RESULT_NOT_REPLAYABLE", "AIGC image execution must return replayable URLs", false)
			}
			result.Outputs = append(result.Outputs, aigcdto.GenerationOutputItem{ID: fmt.Sprintf("image-%d", index+1), Type: "image", URL: url})
		}
	default:
		return Result{}, executionError(http.StatusBadRequest, "AIGC_MODEL_TYPE_NOT_SUPPORTED", "AIGC model type is not supported", false)
	}
	return result, nil
}

func syncExecutionError(err *types.NewAPIError) error {
	status := err.StatusCode
	if status == 0 {
		status = http.StatusBadGateway
	}
	code := strings.ToUpper(strings.ReplaceAll(string(err.GetErrorCode()), ":", "_"))
	if code == "" {
		code = "AIGC_EXECUTION_FAILED"
	}
	message := strings.TrimSpace(err.Error())
	if message == "" {
		message = "AIGC synchronous execution failed"
	}
	return executionError(status, code, message, status == http.StatusTooManyRequests || status >= http.StatusInternalServerError)
}
