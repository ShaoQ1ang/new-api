package handler

import (
	"context"
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/aigc/service"
	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

type UpstreamCatalog interface {
	List(ctx context.Context) ([]service.UpstreamModel, error)
	Get(ctx context.Context, id string) (*service.UpstreamModel, error)
}

type UpstreamModelHandler struct {
	catalog UpstreamCatalog
}

func NewUpstreamModelHandler(catalog UpstreamCatalog) *UpstreamModelHandler {
	return &UpstreamModelHandler{catalog: catalog}
}

func (handler *UpstreamModelHandler) List(c *gin.Context) {
	items, err := handler.catalog.List(c.Request.Context())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, items)
}

func (handler *UpstreamModelHandler) Get(c *gin.Context) {
	rawID := c.Param("id")
	if rawID == "" {
		rawID = strings.TrimPrefix(c.Param("path"), "/")
	}
	id, err := url.PathUnescape(strings.TrimSpace(rawID))
	if err != nil || id == "" {
		common.ApiErrorMsg(c, "invalid upstream model id")
		return
	}
	item, err := handler.catalog.Get(c.Request.Context(), id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, item)
}
