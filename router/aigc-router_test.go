package router

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSetAigcRouterRegistersServiceCatalog(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()

	SetAigcRouter(engine)

	routes := make(map[string]bool)
	for _, route := range engine.Routes() {
		routes[route.Method+" "+route.Path] = true
	}
	require.True(t, routes[http.MethodGet+" /v1/aigc/models"])
}
