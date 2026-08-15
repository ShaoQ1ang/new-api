package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/aigc/dto"
	"github.com/QuantumNous/new-api/aigc/execution"
	"github.com/QuantumNous/new-api/aigc/service"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type generationCatalogStub struct {
	identity execution.Identity
	request  dto.GenerationRequest
	response *dto.GenerationResponse
	err      error
}

func (stub *generationCatalogStub) Submit(_ context.Context, identity execution.Identity, request dto.GenerationRequest) (*dto.GenerationResponse, error) {
	stub.identity, stub.request = identity, request
	return stub.response, stub.err
}

func (stub *generationCatalogStub) Get(_ context.Context, identity execution.Identity, _ string) (*dto.GenerationResponse, error) {
	stub.identity = identity
	return stub.response, stub.err
}

func TestGenerationSubmitUsesAuthenticatedIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	catalog := &generationCatalogStub{response: &dto.GenerationResponse{ID: "aigc_gen_1", RequestID: "turn-1", Status: "queued", Outputs: []dto.GenerationOutputItem{}}}
	handler := NewGenerationHandler(catalog)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/aigc/generations", strings.NewReader(`{"request_id":"turn-1","model":"writer-pro","type":"text","prompt":"hello"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("Idempotency-Key", "turn-1")
	c.Set("id", 7)
	c.Set("token_id", 11)
	common.SetContextKey(c, constant.ContextKeyUsingGroup, "vip")

	handler.Submit(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, execution.Identity{UserID: 7, TokenID: 11, Group: "vip"}, catalog.identity)
	assert.Equal(t, "turn-1", catalog.request.RequestID)
}

func TestGenerationSubmitRejectsMismatchedIdempotencyKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewGenerationHandler(&generationCatalogStub{})
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/aigc/generations", strings.NewReader(`{"request_id":"turn-body","model":"writer-pro","type":"text","prompt":"hello"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("Idempotency-Key", "turn-header")

	handler.Submit(c)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "INVALID_REQUEST")
}

func TestGenerationHandlerWritesProtocolErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	catalog := &generationCatalogStub{err: &service.GenerationError{HTTPStatus: http.StatusConflict, Code: "IDEMPOTENCY_CONFLICT", Message: "conflict", Retryable: false}}
	handler := NewGenerationHandler(catalog)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/aigc/generations", strings.NewReader(`{"request_id":"turn-1","model":"writer-pro","type":"text","prompt":"hello"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("Idempotency-Key", "turn-1")
	c.Set("id", 7)
	c.Set("token_id", 11)

	handler.Submit(c)

	require.Equal(t, http.StatusConflict, recorder.Code)
	var response struct {
		Error struct {
			Code      string `json:"code"`
			RequestID string `json:"request_id"`
		} `json:"error"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, "IDEMPOTENCY_CONFLICT", response.Error.Code)
	assert.Equal(t, "turn-1", response.Error.RequestID)
}
