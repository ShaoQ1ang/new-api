package handler

import (
	"context"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/aigc/service"
	"github.com/gin-gonic/gin"
)

type PricingCatalog interface {
	List(ctx context.Context, group string) (service.PublicPricingDocument, error)
}

type PricingHandler struct {
	catalog PricingCatalog
}

func NewPricingHandler(catalog PricingCatalog) *PricingHandler {
	return &PricingHandler{catalog: catalog}
}

func (handler *PricingHandler) List(c *gin.Context) {
	document, err := handler.catalog.List(c.Request.Context(), c.GetString("group"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "AIGC pricing is unavailable"})
		return
	}
	etag := `"` + document.PricingVersion + `"`
	c.Header("ETag", etag)
	c.Header("Cache-Control", "private, max-age=60")
	if strings.TrimSpace(c.GetHeader("If-None-Match")) == etag {
		c.Status(http.StatusNotModified)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": document})
}
