package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/aigc/handler"
	"github.com/QuantumNous/new-api/aigc/service"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type upstreamRouteCatalogStub struct {
	id string
}

func (stub *upstreamRouteCatalogStub) List(_ context.Context) ([]service.UpstreamModel, error) {
	return nil, nil
}

func (stub *upstreamRouteCatalogStub) Get(_ context.Context, id string) (*service.UpstreamModel, error) {
	stub.id = id
	return &service.UpstreamModel{ID: id}, nil
}

func TestRegisterAPIRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()

	RegisterAPIRoutes(engine, nil, nil, nil)

	routes := make(map[string]bool)
	for _, route := range engine.Routes() {
		routes[route.Method+" "+route.Path] = true
	}
	for _, route := range []string{
		http.MethodGet + " /api/aigc/models",
		http.MethodGet + " /api/aigc/models/:id",
		http.MethodPost + " /api/aigc/models",
		http.MethodPut + " /api/aigc/models/:id",
		http.MethodPost + " /api/aigc/models/:id/validate",
		http.MethodPost + " /api/aigc/models/:id/publish",
		http.MethodPost + " /api/aigc/models/:id/disable",
		http.MethodDelete + " /api/aigc/models/:id",
		http.MethodPost + " /api/aigc/models/import",
		http.MethodGet + " /api/aigc/upstream-models",
		http.MethodGet + " /api/aigc/upstream-models/*path",
	} {
		assert.True(t, routes[route], route)
	}
}

func TestUpstreamModelRouteAcceptsIDsContainingSlashes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(sessions.Sessions("test-session", cookie.NewStore([]byte("test-secret"))))
	catalog := &upstreamRouteCatalogStub{}
	RegisterAPIRoutes(engine, nil, handler.NewUpstreamModelHandler(catalog), nil)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/aigc/upstream-models/openai/gpt-image-1", nil)

	engine.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
}
