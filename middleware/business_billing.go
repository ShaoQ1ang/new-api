package middleware

import (
	"net/http"

	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

func BusinessBilling() gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := service.AuthenticateBusinessBilling(c); err != nil {
			abortWithOpenAiMessage(c, http.StatusForbidden, err.Error())
			return
		}
		c.Next()
	}
}
