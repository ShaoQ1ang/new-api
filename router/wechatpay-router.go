package router

import (
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/gin-gonic/gin"
)

func registerWechatPayRoutes(apiRouter *gin.RouterGroup) {
	apiRouter.POST("/wechatpay/notify", controller.WechatPayNotify)

	userRoute := apiRouter.Group("/user/wechatpay")
	userRoute.Use(middleware.UserAuth())
	{
		userRoute.POST("/native", middleware.CriticalRateLimit(), controller.RequestWechatPayNative)
	}

	adminRoute := apiRouter.Group("/wechatpay/admin")
	adminRoute.Use(middleware.RootAuth(), middleware.DisableCache())
	{
		adminRoute.GET("/settings", controller.GetWechatPayAdminSettings)
		adminRoute.PUT("/settings", middleware.CriticalRateLimit(), controller.UpdateWechatPayAdminSettings)
	}
}
