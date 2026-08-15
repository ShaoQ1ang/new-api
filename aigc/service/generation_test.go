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
}

func newGenerationRequestStoreStub() *generationRequestStoreStub {
	return &generationRequestStoreStub{byKey: map[string]*entity.AigcRequest{}, byGeneration: map[string]*entity.AigcRequest{}}
}

func (stub *generationRequestStoreStub) CreateOrGetRequest(_ context.Context, request *entity.AigcRequest) (*entity.AigcRequest, bool, error) {
	key := requestKey(request.UserID, request.RequestID)
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

func (stub *generationRequestStoreStub) UpdateRequestState(_ context.Context, id int64, expectedStatus string, update entity.RequestStateUpdate) (*entity.AigcRequest, error) {
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

func (stub *generationRequestStoreStub) GetRequestByUserRequestID(_ context.Context, userID int, requestID string) (*entity.AigcRequest, error) {
	request := stub.byKey[requestKey(userID, requestID)]
	if request == nil {
		return nil, entity.ErrGenerationNotFound
	}
	return request, nil
}

type executorStub struct {
	executeResult execution.Result
	pollResult    execution.Result
	executeCalls  int
	pollCalls     int
}

func (stub *executorStub) Execute(_ context.Context, _ execution.Identity, _ execution.Spec) (execution.Result, error) {
	stub.executeCalls++
	return stub.executeResult, nil
}

func (stub *executorStub) Poll(_ context.Context, _ execution.Identity, _ execution.Spec, _ string) (execution.Result, error) {
	stub.pollCalls++
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
	request := dto.GenerationRequest{RequestID: "turn-1", Model: "writer-pro", Type: "text", Prompt: "write"}

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

func TestGenerationServicePollsQueuedExecution(t *testing.T) {
	store := newGenerationRequestStoreStub()
	idempotency := NewIdempotencyService(store, func() (string, error) { return "aigc_gen_async", nil })
	executor := &executorStub{
		executeResult: execution.Result{Status: entity.RequestStatusQueued, Progress: 0, NativeTaskID: "task-native"},
		pollResult:    execution.Result{Status: entity.RequestStatusCompleted, Progress: 100, NativeTaskID: "task-native", Outputs: []dto.GenerationOutputItem{{ID: "video-1", Type: "video", URL: "https://result.test/video.mp4"}}},
	}
	service := NewGenerationService(&generationResolverStub{spec: &execution.Spec{
		PublicModelID: "video-pro", UpstreamModelID: "video-upstream", ModelType: "video", Mode: "text_to_video", ConfigVersion: 2,
	}}, idempotency, store, executor)
	identity := execution.Identity{UserID: 7, TokenID: 11, Group: "default"}

	queued, err := service.Submit(context.Background(), identity, dto.GenerationRequest{RequestID: "turn-2", Model: "video-pro", Type: "video", Prompt: "move", Mode: "text_to_video"})
	require.NoError(t, err)
	assert.Equal(t, entity.RequestStatusQueued, queued.Status)

	completed, err := service.Get(context.Background(), identity, queued.ID)
	require.NoError(t, err)
	assert.Equal(t, entity.RequestStatusCompleted, completed.Status)
	assert.Equal(t, 1, executor.pollCalls)
}

func requestKey(userID int, requestID string) string {
	return fmt.Sprintf("%d:%s", userID, requestID)
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
