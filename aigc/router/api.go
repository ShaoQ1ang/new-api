package router

import (
	"net/http"

	"github.com/QuantumNous/new-api/aigc/handler"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/gin-gonic/gin"
)

func RegisterAPIRoutes(engine *gin.Engine, adminHandler *handler.AdminModelHandler, upstreamHandler *handler.UpstreamModelHandler, importHandler *handler.ProfileImportHandler) {
	aigcRouter := engine.Group("/api/aigc")
	aigcRouter.Use(middleware.RouteTag("api"))
	aigcRouter.Use(middleware.GlobalAPIRateLimit())
	aigcRouter.Use(middleware.AdminAuth())

	invoke := func(action func(*handler.AdminModelHandler, *gin.Context)) gin.HandlerFunc {
		return func(c *gin.Context) {
			if adminHandler == nil {
				c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "message": "AIGC is not configured"})
				return
			}
			action(adminHandler, c)
		}
	}

	aigcRouter.GET("/models", invoke(func(h *handler.AdminModelHandler, c *gin.Context) { h.List(c) }))
	aigcRouter.GET("/models/:id", invoke(func(h *handler.AdminModelHandler, c *gin.Context) { h.Get(c) }))
	aigcRouter.POST("/models", invoke(func(h *handler.AdminModelHandler, c *gin.Context) { h.Create(c) }))
	aigcRouter.PUT("/models/:id", invoke(func(h *handler.AdminModelHandler, c *gin.Context) { h.Update(c) }))
	aigcRouter.POST("/models/:id/validate", invoke(func(h *handler.AdminModelHandler, c *gin.Context) { h.Validate(c) }))
	aigcRouter.POST("/models/:id/publish", invoke(func(h *handler.AdminModelHandler, c *gin.Context) { h.Publish(c) }))
	aigcRouter.POST("/models/:id/disable", invoke(func(h *handler.AdminModelHandler, c *gin.Context) { h.Disable(c) }))
	aigcRouter.DELETE("/models/:id", invoke(func(h *handler.AdminModelHandler, c *gin.Context) { h.Delete(c) }))
	if importHandler != nil {
		aigcRouter.POST("/models/import", importHandler.Import)
	} else {
		aigcRouter.POST("/models/import", func(c *gin.Context) {
			c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "message": "AIGC is not configured"})
		})
	}

	invokeUpstream := func(action func(*handler.UpstreamModelHandler, *gin.Context)) gin.HandlerFunc {
		return func(c *gin.Context) {
			if upstreamHandler == nil {
				c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "message": "AIGC is not configured"})
				return
			}
			action(upstreamHandler, c)
		}
	}
	aigcRouter.GET("/upstream-models", invokeUpstream(func(h *handler.UpstreamModelHandler, c *gin.Context) { h.List(c) }))
	aigcRouter.GET("/upstream-models/*path", invokeUpstream(func(h *handler.UpstreamModelHandler, c *gin.Context) { h.Get(c) }))
}
