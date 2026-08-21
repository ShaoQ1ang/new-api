package controller

import (
	"encoding/json"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

const maxSunoAPICallbackBytes = 1 << 20

// SunoAPICallback acknowledges provider stage notifications. Background polling
// remains the source of truth and performs all task updates and billing.
func SunoAPICallback(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxSunoAPICallbackBytes)
	var payload map[string]json.RawMessage
	if err := common.DecodeJson(c.Request.Body, &payload); err != nil || len(payload) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "invalid callback payload"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "success"})
}
