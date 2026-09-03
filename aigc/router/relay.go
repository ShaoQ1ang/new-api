package router

import (
	"net/http"

	"github.com/QuantumNous/new-api/aigc/handler"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/gin-gonic/gin"
)

func RegisterRelayRoutes(engine *gin.Engine, modelHandler *handler.ModelHandler, generationHandler *handler.GenerationHandler) {
	aigcRouter := engine.Group("/v1/aigc")
	aigcRouter.Use(middleware.RouteTag("relay"))
	aigcRouter.Use(middleware.SystemPerformanceCheck())
	aigcRouter.Use(middleware.TokenAuth())
	aigcRouter.Use(middleware.ModelRequestRateLimit())
	listModels := func(c *gin.Context) {
		if modelHandler == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": gin.H{"code": "AIGC_NOT_CONFIGURED", "retryable": true}})
			return
		}
		modelHandler.List(c)
	}
	aigcRouter.GET("/models", listModels)
	invokeGeneration := func(action func(*handler.GenerationHandler, *gin.Context)) gin.HandlerFunc {
		return func(c *gin.Context) {
			if generationHandler == nil {
				c.JSON(http.StatusServiceUnavailable, gin.H{"error": gin.H{"code": "AIGC_NOT_CONFIGURED", "retryable": true}})
				return
			}
			action(generationHandler, c)
		}
	}
	aigcRouter.POST("/generations", invokeGeneration(func(h *handler.GenerationHandler, c *gin.Context) { h.Submit(c) }))
	aigcRouter.GET("/generations/:id", invokeGeneration(func(h *handler.GenerationHandler, c *gin.Context) { h.Get(c) }))
}
