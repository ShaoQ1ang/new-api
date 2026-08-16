package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/aigc/dto"
	"github.com/QuantumNous/new-api/aigc/execution"
	"github.com/QuantumNous/new-api/aigc/service"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/pkg/tracelog"
	"github.com/gin-gonic/gin"
)

type GenerationCatalog interface {
	Submit(ctx context.Context, identity execution.Identity, request dto.GenerationRequest) (*dto.GenerationResponse, error)
	Get(ctx context.Context, identity execution.Identity, generationID string) (*dto.GenerationResponse, error)
}

type GenerationHandler struct {
	catalog GenerationCatalog
}

func NewGenerationHandler(catalog GenerationCatalog) *GenerationHandler {
	return &GenerationHandler{catalog: catalog}
}

func (handler *GenerationHandler) Submit(c *gin.Context) {
	var request dto.GenerationRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		tracelog.Default.LogValue(c.Request.Context(), "newapi.input", "server-request", func() any {
			return gin.H{"error": "invalid JSON body"}
		})
		writeGenerationError(c, request.IdempotencyKey, serviceError(http.StatusBadRequest, "INVALID_REQUEST", "invalid AIGC generation request", false))
		return
	}
	tracelog.Default.LogValue(c.Request.Context(), "newapi.input", "server-request", func() any { return request })
	request.IdempotencyKey = strings.TrimSpace(request.IdempotencyKey)
	if request.IdempotencyKey == "" || len(request.IdempotencyKey) > 200 {
		writeGenerationError(c, request.IdempotencyKey, serviceError(http.StatusBadRequest, "INVALID_REQUEST", "idempotency_key must contain 1 to 200 characters", false))
		return
	}
	identity, ok := generationIdentity(c)
	if !ok {
		writeGenerationError(c, request.IdempotencyKey, serviceError(http.StatusUnauthorized, "UNAUTHORIZED", "authenticated token identity is required", false))
		return
	}
	response, err := handler.catalog.Submit(execution.WithGinContext(c.Request.Context(), c), identity, request)
	if err != nil {
		writeGenerationError(c, request.IdempotencyKey, err)
		return
	}
	tracelog.Default.LogResponseValue(c.Request.Context(), "newapi.output", "server-response", func() any { return response })
	c.JSON(http.StatusOK, response)
}

func (handler *GenerationHandler) Get(c *gin.Context) {
	tracelog.Default.LogValue(c.Request.Context(), "newapi.input", "server-request", func() any {
		return gin.H{"generation_id": strings.TrimSpace(c.Param("id"))}
	})
	identity, ok := generationIdentity(c)
	if !ok {
		writeGenerationError(c, "", serviceError(http.StatusUnauthorized, "UNAUTHORIZED", "authenticated token identity is required", false))
		return
	}
	response, err := handler.catalog.Get(execution.WithGinContext(c.Request.Context(), c), identity, strings.TrimSpace(c.Param("id")))
	if err != nil {
		writeGenerationError(c, "", err)
		return
	}
	tracelog.Default.LogResponseValue(c.Request.Context(), "newapi.output", "server-response", func() any { return response })
	c.JSON(http.StatusOK, response)
}

func generationIdentity(c *gin.Context) (execution.Identity, bool) {
	identity := execution.Identity{UserID: c.GetInt("id"), TokenID: c.GetInt("token_id")}
	identity.Group = common.GetContextKeyString(c, constant.ContextKeyUsingGroup)
	if identity.Group == "" {
		identity.Group = common.GetContextKeyString(c, constant.ContextKeyTokenGroup)
	}
	if identity.Group == "" {
		identity.Group = common.GetContextKeyString(c, constant.ContextKeyUserGroup)
	}
	return identity, identity.UserID > 0 && identity.TokenID > 0
}

func writeGenerationError(c *gin.Context, idempotencyKey string, err error) {
	var protocolErr *service.GenerationError
	if !errors.As(err, &protocolErr) {
		protocolErr = serviceError(http.StatusInternalServerError, "AIGC_INTERNAL_ERROR", "AIGC request failed", true)
	}
	response := gin.H{"error": gin.H{
		"code": protocolErr.Code, "message": protocolErr.Message,
		"retryable": protocolErr.Retryable, "idempotency_key": idempotencyKey,
	}}
	tracelog.Default.LogResponseValue(c.Request.Context(), "newapi.output", "server-response", func() any { return response })
	c.JSON(protocolErr.HTTPStatus, response)
}

func serviceError(status int, code, message string, retryable bool) *service.GenerationError {
	return &service.GenerationError{HTTPStatus: status, Code: code, Message: message, Retryable: retryable}
}
