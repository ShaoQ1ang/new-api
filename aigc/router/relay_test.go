package router

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestRegisterRelayRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()

	RegisterRelayRoutes(engine, nil, nil)

	routes := make(map[string]bool)
	for _, route := range engine.Routes() {
		routes[route.Method+" "+route.Path] = true
	}
	assert.True(t, routes[http.MethodGet+" /v1/aigc/models"])
	assert.True(t, routes[http.MethodPost+" /v1/aigc/generations"])
	assert.True(t, routes[http.MethodGet+" /v1/aigc/generations/:id"])
}
