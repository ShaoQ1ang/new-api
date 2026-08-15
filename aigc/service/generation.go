package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/aigc/dto"
	"github.com/QuantumNous/new-api/aigc/entity"
	"github.com/QuantumNous/new-api/aigc/execution"
	"github.com/QuantumNous/new-api/common"
)

type Resolver interface {
	Resolve(ctx context.Context, group string, request dto.GenerationRequest) (*execution.Spec, error)
}

type GenerationRequestStore interface {
	UpdateRequestState(ctx context.Context, id int64, expectedStatus string, update entity.RequestStateUpdate) (*entity.AigcRequest, error)
	GetRequestByGenerationID(ctx context.Context, userID int, generationID string) (*entity.AigcRequest, error)
}

type GenerationService struct {
	resolver    Resolver
	idempotency *IdempotencyService
	requests    GenerationRequestStore
	executor    execution.Executor
}

func NewGenerationService(resolver Resolver, idempotency *IdempotencyService, requests GenerationRequestStore, executor execution.Executor) *GenerationService {
	return &GenerationService{resolver: resolver, idempotency: idempotency, requests: requests, executor: executor}
}

func (service *GenerationService) Submit(ctx context.Context, identity execution.Identity, request dto.GenerationRequest) (*dto.GenerationResponse, error) {
	request = normalizeGenerationRequest(request)
	if request.RequestID == "" {
		return nil, generationError(http.StatusBadRequest, "INVALID_REQUEST", "request_id is required", false)
	}
	digest, err := DigestGenerationRequest(request)
	if err != nil {
		return nil, err
	}
	replayed, found, err := service.idempotency.Replay(ctx, identity.UserID, request.RequestID, digest)
	if err != nil {
		if errors.Is(err, entity.ErrIdempotencyConflict) {
			return nil, generationError(http.StatusConflict, "IDEMPOTENCY_CONFLICT", "request_id was already used with different input", false)
		}
		return nil, err
	}
	if found {
		return generationResponse(replayed)
	}
	spec, err := service.resolver.Resolve(ctx, identity.Group, request)
	if err != nil {
		return nil, err
	}
	requestJSON, err := common.Marshal(spec.Request)
	if err != nil {
		return nil, err
	}
	executionJSON, err := common.Marshal(spec)
	if err != nil {
		return nil, err
	}
	stored, created, err := service.idempotency.Begin(ctx, BeginRequest{
		RequestID: request.RequestID, UserID: identity.UserID, TokenID: identity.TokenID, GroupName: identity.Group,
		PublicModelID: spec.PublicModelID, UpstreamModelID: spec.UpstreamModelID, ModelType: spec.ModelType,
		Mode: spec.Mode, ConfigVersion: spec.ConfigVersion, RequestDigest: digest,
		RequestJSON: string(requestJSON), ExecutionJSON: string(executionJSON),
	})
	if err != nil {
		if errors.Is(err, entity.ErrIdempotencyConflict) {
			return nil, generationError(http.StatusConflict, "IDEMPOTENCY_CONFLICT", "request_id was already used with different input", false)
		}
		return nil, err
	}
	if !created {
		return generationResponse(stored)
	}
	result, executeErr := service.executor.Execute(ctx, identity, *spec)
	if executeErr != nil {
		return nil, service.persistExecutionError(ctx, stored, executeErr)
	}
	updated, err := service.persistResult(ctx, stored, result)
	if err != nil {
		return nil, err
	}
	return generationResponse(updated)
}

func normalizeGenerationRequest(request dto.GenerationRequest) dto.GenerationRequest {
	request.RequestID = strings.TrimSpace(request.RequestID)
	request.Model = strings.TrimSpace(request.Model)
	request.Type = strings.TrimSpace(request.Type)
	request.Prompt = strings.TrimSpace(request.Prompt)
	request.Mode = strings.TrimSpace(request.Mode)
	request.Output.Size = strings.TrimSpace(request.Output.Size)
	request.Output.Resolution = strings.TrimSpace(request.Output.Resolution)
	request.Output.AspectRatio = strings.TrimSpace(request.Output.AspectRatio)
	for _, items := range [][]dto.MediaInput{request.Inputs.Images, request.Inputs.Videos, request.Inputs.Audios} {
		for index := range items {
			items[index].Role = strings.TrimSpace(items[index].Role)
			items[index].URL = strings.TrimSpace(items[index].URL)
		}
	}
	return request
}

func (service *GenerationService) Get(ctx context.Context, identity execution.Identity, generationID string) (*dto.GenerationResponse, error) {
	request, err := service.requests.GetRequestByGenerationID(ctx, identity.UserID, strings.TrimSpace(generationID))
	if err != nil {
		if errors.Is(err, entity.ErrGenerationNotFound) {
			return nil, generationError(http.StatusNotFound, "GENERATION_NOT_FOUND", "AIGC generation was not found", false)
		}
		return nil, err
	}
	if terminalRequestStatus(request.Status) || request.NativeTaskID == "" {
		return generationResponse(request)
	}
	var spec execution.Spec
	if err := common.Unmarshal([]byte(request.ExecutionJSON), &spec); err != nil {
		return nil, fmt.Errorf("decode AIGC execution snapshot: %w", err)
	}
	result, pollErr := service.executor.Poll(ctx, identity, spec, request.NativeTaskID)
	if pollErr != nil {
		return nil, generationErrorFromExecution(pollErr)
	}
	updated, err := service.persistResult(ctx, request, result)
	if err != nil {
		if errors.Is(err, entity.ErrRequestStateConflict) {
			latest, loadErr := service.requests.GetRequestByGenerationID(ctx, identity.UserID, generationID)
			if loadErr != nil {
				return nil, loadErr
			}
			return generationResponse(latest)
		}
		return nil, err
	}
	return generationResponse(updated)
}

func (service *GenerationService) persistResult(ctx context.Context, request *entity.AigcRequest, result execution.Result) (*entity.AigcRequest, error) {
	if !validRequestTransition(request.Status, result.Status) {
		return nil, fmt.Errorf("invalid AIGC request transition %s -> %s", request.Status, result.Status)
	}
	if result.Progress < 0 || result.Progress > 100 {
		return nil, fmt.Errorf("invalid AIGC generation progress %d", result.Progress)
	}
	if result.Progress < request.Progress {
		return nil, fmt.Errorf("AIGC generation progress cannot regress from %d to %d", request.Progress, result.Progress)
	}
	if result.Status == entity.RequestStatusCompleted && result.Progress != 100 {
		return nil, fmt.Errorf("completed AIGC generation progress must be 100")
	}
	if result.NativeTaskID == "" {
		result.NativeTaskID = request.NativeTaskID
	}
	if (result.Status == entity.RequestStatusQueued || result.Status == entity.RequestStatusProcessing) && result.NativeTaskID == "" {
		return nil, fmt.Errorf("AIGC asynchronous result requires native task id")
	}
	if result.Outputs == nil {
		result.Outputs = make([]dto.GenerationOutputItem, 0)
	}
	resultJSON, err := common.Marshal(result)
	if err != nil {
		return nil, err
	}
	return service.requests.UpdateRequestState(ctx, request.ID, request.Status, entity.RequestStateUpdate{
		Status: result.Status, Progress: result.Progress, NativeTaskID: result.NativeTaskID,
		ResultJSON: string(resultJSON), ErrorCode: result.ErrorCode, ErrorMessage: result.ErrorMessage,
	})
}

func (service *GenerationService) persistExecutionError(ctx context.Context, request *entity.AigcRequest, executeErr error) error {
	protocolErr := generationErrorFromExecution(executeErr)
	_, persistErr := service.persistResult(ctx, request, execution.Result{
		Status: entity.RequestStatusFailed, Progress: request.Progress, ErrorCode: protocolErr.Code,
		ErrorMessage: protocolErr.Message, Retryable: protocolErr.Retryable,
	})
	if persistErr != nil {
		return persistErr
	}
	return protocolErr
}

func generationErrorFromExecution(err error) *GenerationError {
	var executionErr *execution.Error
	if errors.As(err, &executionErr) {
		return generationError(executionErr.HTTPStatus, executionErr.Code, executionErr.Message, executionErr.Retryable)
	}
	return generationError(http.StatusBadGateway, "UPSTREAM_REJECTED", "AIGC upstream rejected the request", false)
}

func generationResponse(request *entity.AigcRequest) (*dto.GenerationResponse, error) {
	result := execution.Result{Outputs: make([]dto.GenerationOutputItem, 0)}
	if strings.TrimSpace(request.ResultJSON) != "" && request.ResultJSON != "{}" {
		if err := common.Unmarshal([]byte(request.ResultJSON), &result); err != nil {
			return nil, fmt.Errorf("decode AIGC generation result: %w", err)
		}
	}
	if result.Outputs == nil {
		result.Outputs = make([]dto.GenerationOutputItem, 0)
	}
	response := &dto.GenerationResponse{
		ID: request.GenerationID, RequestID: request.RequestID, Status: request.Status, Progress: request.Progress,
		Model: request.PublicModelID, Type: request.ModelType, CreatedAt: request.CreatedTime,
		Outputs: result.Outputs, Usage: result.Usage,
	}
	if request.ErrorCode != "" || request.ErrorMessage != "" {
		response.Error = &dto.GenerationFailure{Code: request.ErrorCode, Message: request.ErrorMessage, Retryable: result.Retryable}
	}
	return response, nil
}

func validRequestTransition(from, to string) bool {
	if from == to && (from == entity.RequestStatusQueued || from == entity.RequestStatusProcessing) {
		return true
	}
	switch from {
	case entity.RequestStatusSubmitted:
		return to == entity.RequestStatusQueued || to == entity.RequestStatusProcessing || to == entity.RequestStatusCompleted || to == entity.RequestStatusFailed
	case entity.RequestStatusQueued:
		return to == entity.RequestStatusProcessing || to == entity.RequestStatusCompleted || to == entity.RequestStatusFailed || to == entity.RequestStatusCanceled
	case entity.RequestStatusProcessing:
		return to == entity.RequestStatusCompleted || to == entity.RequestStatusFailed || to == entity.RequestStatusCanceled
	default:
		return false
	}
}

func terminalRequestStatus(status string) bool {
	return status == entity.RequestStatusCompleted || status == entity.RequestStatusFailed || status == entity.RequestStatusCanceled
}
