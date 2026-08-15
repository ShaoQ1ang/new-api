package service

import (
	"context"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/aigc/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type requestStoreStub struct {
	stored *entity.AigcRequest
}

func (stub *requestStoreStub) CreateOrGetRequest(_ context.Context, request *entity.AigcRequest) (*entity.AigcRequest, bool, error) {
	if stub.stored != nil {
		return stub.stored, false, nil
	}
	copy := *request
	copy.ID = 1
	stub.stored = &copy
	return &copy, true, nil
}

func (stub *requestStoreStub) GetRequestByUserRequestID(_ context.Context, userID int, requestID string) (*entity.AigcRequest, error) {
	if stub.stored == nil || stub.stored.UserID != userID || stub.stored.RequestID != requestID {
		return nil, entity.ErrGenerationNotFound
	}
	return stub.stored, nil
}

func TestIdempotencyBeginCreatesThenReplaysSameRequest(t *testing.T) {
	store := &requestStoreStub{}
	service := NewIdempotencyService(store, func() (string, error) { return "aigc_gen_fixed", nil })
	input := BeginRequest{
		RequestID: "turn-1", UserID: 7, TokenID: 11, GroupName: "default",
		PublicModelID: "writer-pro", UpstreamModelID: "gpt-5", ModelType: "text", Mode: "text",
		ConfigVersion: 3, RequestDigest: strings.Repeat("a", 64),
	}

	request, created, err := service.Begin(context.Background(), input)
	require.NoError(t, err)
	assert.True(t, created)
	assert.Equal(t, "aigc_gen_fixed", request.GenerationID)
	assert.Equal(t, 3, request.ConfigVersion)

	replayed, created, err := service.Begin(context.Background(), input)
	require.NoError(t, err)
	assert.False(t, created)
	assert.Equal(t, request.GenerationID, replayed.GenerationID)
}

func TestIdempotencyBeginRejectsDigestMismatch(t *testing.T) {
	store := &requestStoreStub{stored: &entity.AigcRequest{
		RequestID: "turn-1", GenerationID: "aigc_gen_existing", UserID: 7, RequestDigest: strings.Repeat("a", 64),
	}}
	service := NewIdempotencyService(store, func() (string, error) { return "unused", nil })

	_, _, err := service.Begin(context.Background(), BeginRequest{
		RequestID: "turn-1", UserID: 7, TokenID: 11, PublicModelID: "writer-pro",
		UpstreamModelID: "gpt-5", ModelType: "text", Mode: "text", RequestDigest: strings.Repeat("b", 64),
	})

	assert.ErrorIs(t, err, ErrIdempotencyConflict)
}

func TestDigestGenerationRequestIsStable(t *testing.T) {
	first, err := DigestGenerationRequest(map[string]any{"prompt": "hello", "output": map[string]any{"size": "1024x1024", "count": 1}})
	require.NoError(t, err)
	second, err := DigestGenerationRequest(map[string]any{"output": map[string]any{"count": 1, "size": "1024x1024"}, "prompt": "hello"})
	require.NoError(t, err)

	assert.Equal(t, first, second)
	assert.Len(t, first, 64)
}
