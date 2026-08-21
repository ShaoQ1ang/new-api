package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/aigc/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type pricingCatalogStub struct {
	group string
}

func (stub *pricingCatalogStub) List(_ context.Context, group string) (service.PublicPricingDocument, error) {
	stub.group = group
	return service.PublicPricingDocument{PricingVersion: "sha256:test", Models: []service.PublicModelPricing{}}, nil
}

func TestPricingHandlerReturnsETagAndSupportsConditionalRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	catalog := &pricingCatalogStub{}
	handler := NewPricingHandler(catalog)
	engine := gin.New()
	engine.Use(func(c *gin.Context) { c.Set("group", "vip") })
	engine.GET("/api/aigc/pricing", handler.List)

	response := httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/aigc/pricing", nil))

	require.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, "vip", catalog.group)
	assert.Equal(t, `"sha256:test"`, response.Header().Get("ETag"))

	conditional := httptest.NewRequest(http.MethodGet, "/api/aigc/pricing", nil)
	conditional.Header.Set("If-None-Match", `"sha256:test"`)
	conditionalResponse := httptest.NewRecorder()
	engine.ServeHTTP(conditionalResponse, conditional)
	assert.Equal(t, http.StatusNotModified, conditionalResponse.Code)
}
