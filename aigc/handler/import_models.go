package handler

import (
	"context"

	"github.com/QuantumNous/new-api/aigc/service"
	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

type ProfileImporter interface {
	Import(ctx context.Context, items []service.LegacyProfileInput) (*service.ProfileImportReport, error)
}

type ProfileImportHandler struct {
	importer ProfileImporter
}

func NewProfileImportHandler(importer ProfileImporter) *ProfileImportHandler {
	return &ProfileImportHandler{importer: importer}
}

func (handler *ProfileImportHandler) Import(c *gin.Context) {
	var request struct {
		Items []service.LegacyProfileInput `json:"items"`
	}
	if err := c.ShouldBindJSON(&request); err != nil || len(request.Items) == 0 {
		common.ApiErrorMsg(c, "AIGC model import items are required")
		return
	}
	report, err := handler.importer.Import(c.Request.Context(), request.Items)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, report)
}
