package handler

import (
	"context"
	"net/http"

	"github.com/QuantumNous/new-api/aigc/service"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/gin-gonic/gin"
)

type ModelCatalog interface {
	List(ctx context.Context, group, modelType string) ([]service.PublicModel, error)
}

type ModelHandler struct {
	catalog ModelCatalog
}

func NewModelHandler(catalog ModelCatalog) *ModelHandler {
	return &ModelHandler{catalog: catalog}
}

func (handler *ModelHandler) List(c *gin.Context) {
	group := common.GetContextKeyString(c, constant.ContextKeyTokenGroup)
	if group == "" {
		group = common.GetContextKeyString(c, constant.ContextKeyUserGroup)
	}
	items, err := handler.catalog.List(c.Request.Context(), group, c.Query("type"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{
			"code": "AIGC_CATALOG_UNAVAILABLE", "message": err.Error(), "retryable": true,
		}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"object": "list", "data": items})
}
