package router

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestRegisterAPIRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()

	RegisterAPIRoutes(engine, nil)

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
	} {
		assert.True(t, routes[route], route)
	}
}
