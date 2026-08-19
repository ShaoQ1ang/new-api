package service

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/aigc/dto"
	"github.com/QuantumNous/new-api/aigc/entity"
	"github.com/QuantumNous/new-api/aigc/execution"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type generationResolverStub struct {
	spec  *execution.Spec
	err   error
	calls int
}

func (stub *generationResolverStub) Resolve(_ context.Context, _ string, request dto.GenerationRequest) (*execution.Spec, error) {
	stub.calls++
	if stub.err != nil {
		return nil, stub.err
	}
	copy := *stub.spec
	copy.Request = request
	copy.Request.Mode = copy.Mode
	return &copy, nil
}

type generationRequestStoreStub struct {
	byKey        map[string]*entity.AigcRequest
	byGeneration map[string]*entity.AigcRequest
	updateCtxErr error
}

func newGenerationRequestStoreStub() *generationRequestStoreStub {
	return &generationRequestStoreStub{byKey: map[string]*entity.AigcRequest{}, byGeneration: map[string]*entity.AigcRequest{}}
}

func (stub *generationRequestStoreStub) CreateOrGetRequest(_ context.Context, request *entity.AigcRequest) (*entity.AigcRequest, bool, error) {
	key := requestKey(request.UserID, request.IdempotencyKey)
	if existing := stub.byKey[key]; existing != nil {
		return existing, false, nil
	}
	copy := *request
	copy.ID = int64(len(stub.byKey) + 1)
	copy.CreatedTime = 100
	stub.byKey[key] = &copy
	stub.byGeneration[copy.GenerationID] = &copy
	return &copy, true, nil
}

func (stub *generationRequestStoreStub) UpdateRequestState(ctx context.Context, id int64, expectedStatus string, update entity.RequestStateUpdate) (*entity.AigcRequest, error) {
	stub.updateCtxErr = ctx.Err()
	for _, request := range stub.byGeneration {
		if request.ID == id && request.Status == expectedStatus {
			request.Status = update.Status
			request.Progress = update.Progress
			request.NativeTaskID = update.NativeTaskID
			request.ResultJSON = update.ResultJSON
			request.ErrorCode = update.ErrorCode
			request.ErrorMessage = update.ErrorMessage
			return request, nil
		}
	}
	return nil, entity.ErrRequestStateConflict
}

func (stub *generationRequestStoreStub) GetRequestByGenerationID(_ context.Context, userID int, generationID string) (*entity.AigcRequest, error) {
	request := stub.byGeneration[generationID]
	if request == nil || request.UserID != userID {
		return nil, assert.AnError
	}
	return request, nil
}

func (stub *generationRequestStoreStub) GetRequestByUserIdempotencyKey(_ context.Context, userID int, idempotencyKey string) (*entity.AigcRequest, error) {
	request := stub.byKey[requestKey(userID, idempotencyKey)]
	if request == nil {
		return nil, entity.ErrGenerationNotFound
	}
	return request, nil
}

type executorStub struct {
	executeResult execution.Result
	executeErr    error
	cancelExecute context.CancelFunc
	pollResult    execution.Result
	executeCalls  int
	pollCalls     int
	pollModelType string
}

func (stub *executorStub) Execute(_ context.Context, _ execution.Identity, _ execution.Spec) (execution.Result, error) {
	stub.executeCalls++
	if stub.cancelExecute != nil {
		stub.cancelExecute()
	}
	return stub.executeResult, stub.executeErr
}

func (stub *executorStub) Poll(_ context.Context, _ execution.Identity, modelType, _ string) (execution.Result, error) {
	stub.pollCalls++
	stub.pollModelType = modelType
	return stub.pollResult, nil
}

func TestGenerationServiceExecutesOnceAndReplaysCompletedResult(t *testing.T) {
	store := newGenerationRequestStoreStub()
	idempotency := NewIdempotencyService(store, func() (string, error) { return "aigc_gen_fixed", nil })
	executor := &executorStub{executeResult: execution.Result{
		Status: entity.RequestStatusCompleted, Progress: 100,
		Outputs: []dto.GenerationOutputItem{{ID: "output-1", Type: "text", Text: "hello"}},
	}}
	resolver := &generationResolverStub{spec: &execution.Spec{
		PublicModelID: "writer-pro", UpstreamModelID: "gpt-5", ModelType: "text", Mode: "text", ConfigVersion: 3,
	}}
	service := NewGenerationService(resolver, idempotency, store, executor)
	identity := execution.Identity{UserID: 7, TokenID: 11, Group: "default"}
	request := dto.GenerationRequest{IdempotencyKey: "turn-1", Model: "writer-pro", Type: "text", Prompt: "write"}

	first, err := service.Submit(context.Background(), identity, request)
	require.NoError(t, err)
	assert.Equal(t, entity.RequestStatusCompleted, first.Status)
	require.Len(t, first.Outputs, 1)
	assert.Equal(t, "hello", first.Outputs[0].Text)

	resolver.err = assert.AnError
	replayed, err := service.Submit(context.Background(), identity, request)
	require.NoError(t, err)
	assert.Equal(t, first.ID, replayed.ID)
	assert.Equal(t, 1, executor.executeCalls)
	assert.Equal(t, 1, resolver.calls)
}

func TestGenerationServicePersistsFailureAfterRequestContextCanceled(t *testing.T) {
	store := newGenerationRequestStoreStub()
	idempotency := NewIdempotencyService(store, func() (string, error) { return "aigc_gen_canceled", nil })
	requestContext, cancel := context.WithCancel(context.Background())
	executor := &executorStub{
		executeErr:    &execution.Error{HTTPStatus: 504, Code: "UPSTREAM_TIMEOUT", Message: "upstream timed out", Retryable: true},
		cancelExecute: cancel,
	}
	resolver := &generationResolverStub{spec: &execution.Spec{
		PublicModelID: "image-public", UpstreamModelID: "image-upstream", ModelType: "image", Mode: "text_to_image", ConfigVersion: 1,
	}}
	service := NewGenerationService(resolver, idempotency, store, executor)
	request := dto.GenerationRequest{IdempotencyKey: "turn-canceled", Model: "image-public", Type: "image", Prompt: "draw"}

	_, err := service.Submit(requestContext, execution.Identity{UserID: 7, TokenID: 11, Group: "default"}, request)
	require.Error(t, err)
	require.NoError(t, store.updateCtxErr)
	stored := store.byGeneration["aigc_gen_canceled"]
	require.NotNil(t, stored)
	assert.Equal(t, entity.RequestStatusFailed, stored.Status)
	assert.Equal(t, "UPSTREAM_TIMEOUT", stored.ErrorCode)
}

func TestGenerationServicePollsQueuedExecution(t *testing.T) {
	store := newGenerationRequestStoreStub()
	idempotency := NewIdempotencyService(store, func() (string, error) { return "aigc_gen_async", nil })
	executor := &executorStub{
		executeResult: execution.Result{Status: entity.RequestStatusQueued, Progress: 0, NativeTaskID: "task-native"},
		pollResult:    execution.Result{Status: entity.RequestStatusCompleted, Progress: 100, NativeTaskID: "task-native", Outputs: []dto.GenerationOutputItem{{ID: "video-1", Type: "video", URL: "https://result.test/video.mp4"}}},
	}
	resolver := &generationResolverStub{spec: &execution.Spec{
		PublicModelID: "video-pro", UpstreamModelID: "video-upstream", ModelType: "video", Mode: "text_to_video", ConfigVersion: 2,
	}}
	service := NewGenerationService(resolver, idempotency, store, executor)
	identity := execution.Identity{UserID: 7, TokenID: 11, Group: "default"}

	queued, err := service.Submit(context.Background(), identity, dto.GenerationRequest{IdempotencyKey: "turn-2", Model: "video-pro", Type: "video", Prompt: "move", Mode: "text_to_video"})
	require.NoError(t, err)
	assert.Equal(t, entity.RequestStatusQueued, queued.Status)

	resolver.err = assert.AnError
	completed, err := service.Get(context.Background(), identity, queued.ID)
	require.NoError(t, err)
	assert.Equal(t, entity.RequestStatusCompleted, completed.Status)
	assert.Equal(t, 1, executor.pollCalls)
	assert.Equal(t, "video", executor.pollModelType)
}

func TestGenerationServiceNewIdempotencyKeyResolvesCurrentProfile(t *testing.T) {
	store := newGenerationRequestStoreStub()
	nextID := 0
	idempotency := NewIdempotencyService(store, func() (string, error) {
		nextID++
		return fmt.Sprintf("aigc_gen_%d", nextID), nil
	})
	executor := &executorStub{executeResult: execution.Result{Status: entity.RequestStatusCompleted, Progress: 100}}
	resolver := &generationResolverStub{spec: &execution.Spec{
		PublicModelID: "video-pro", UpstreamModelID: "video-v7", ModelType: "video", Mode: "text_to_video", ConfigVersion: 7,
	}}
	service := NewGenerationService(resolver, idempotency, store, executor)
	identity := execution.Identity{UserID: 7, TokenID: 11, Group: "default"}

	first, err := service.Submit(context.Background(), identity, dto.GenerationRequest{
		IdempotencyKey: "turn-1", Model: "video-pro", Type: "video", Prompt: "move", Mode: "text_to_video",
	})
	require.NoError(t, err)

	resolver.spec.UpstreamModelID = "video-v8"
	resolver.spec.ConfigVersion = 8
	second, err := service.Submit(context.Background(), identity, dto.GenerationRequest{
		IdempotencyKey: "turn-2", Model: "video-pro", Type: "video", Prompt: "move", Mode: "text_to_video",
	})
	require.NoError(t, err)

	assert.NotEqual(t, first.ID, second.ID)
	assert.Equal(t, 2, resolver.calls)
	assert.Equal(t, 2, executor.executeCalls)
	assert.Equal(t, "video-v7", store.byGeneration[first.ID].UpstreamModelID)
	assert.Equal(t, "video-v8", store.byGeneration[second.ID].UpstreamModelID)
	assert.Equal(t, 7, store.byGeneration[first.ID].ConfigVersion)
	assert.Equal(t, 8, store.byGeneration[second.ID].ConfigVersion)
}

func requestKey(userID int, idempotencyKey string) string {
	return fmt.Sprintf("%d:%s", userID, idempotencyKey)
}

func TestPersistResultRejectsProgressRegressionAndIncompleteCompletion(t *testing.T) {
	service := &GenerationService{requests: newGenerationRequestStoreStub()}
	request := &entity.AigcRequest{ID: 1, Status: entity.RequestStatusProcessing, Progress: 60}

	_, err := service.persistResult(context.Background(), request, execution.Result{Status: entity.RequestStatusProcessing, Progress: 50})
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "progress"))

	_, err = service.persistResult(context.Background(), request, execution.Result{Status: entity.RequestStatusCompleted, Progress: 99})
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "100"))
}
