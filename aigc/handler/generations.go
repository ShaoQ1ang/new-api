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
		writeGenerationError(c, request.RequestID, serviceError(http.StatusBadRequest, "INVALID_REQUEST", "invalid AIGC generation request", false))
		return
	}
	request.RequestID = strings.TrimSpace(request.RequestID)
	if key := strings.TrimSpace(c.GetHeader("Idempotency-Key")); key == "" || key != request.RequestID {
		writeGenerationError(c, request.RequestID, serviceError(http.StatusBadRequest, "INVALID_REQUEST", "Idempotency-Key must match request_id", false))
		return
	}
	identity, ok := generationIdentity(c)
	if !ok {
		writeGenerationError(c, request.RequestID, serviceError(http.StatusUnauthorized, "UNAUTHORIZED", "authenticated token identity is required", false))
		return
	}
	response, err := handler.catalog.Submit(c.Request.Context(), identity, request)
	if err != nil {
		writeGenerationError(c, request.RequestID, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

func (handler *GenerationHandler) Get(c *gin.Context) {
	identity, ok := generationIdentity(c)
	if !ok {
		writeGenerationError(c, "", serviceError(http.StatusUnauthorized, "UNAUTHORIZED", "authenticated token identity is required", false))
		return
	}
	response, err := handler.catalog.Get(c.Request.Context(), identity, strings.TrimSpace(c.Param("id")))
	if err != nil {
		writeGenerationError(c, "", err)
		return
	}
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

func writeGenerationError(c *gin.Context, requestID string, err error) {
	var protocolErr *service.GenerationError
	if !errors.As(err, &protocolErr) {
		protocolErr = serviceError(http.StatusInternalServerError, "AIGC_INTERNAL_ERROR", "AIGC request failed", true)
	}
	c.JSON(protocolErr.HTTPStatus, gin.H{"error": gin.H{
		"code": protocolErr.Code, "message": protocolErr.Message,
		"retryable": protocolErr.Retryable, "request_id": requestID,
	}})
}

func serviceError(status int, code, message string, retryable bool) *service.GenerationError {
	return &service.GenerationError{HTTPStatus: status, Code: code, Message: message, Retryable: retryable}
}
