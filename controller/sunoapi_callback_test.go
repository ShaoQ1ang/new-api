package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestSunoAPICallbackOnlyAcknowledgesValidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, test := range []struct {
		name, body string
		status     int
	}{
		{name: "valid", body: `{"data":{"task_id":"native"}}`, status: http.StatusOK},
		{name: "invalid", body: `{`, status: http.StatusBadRequest},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(response)
			context.Request = httptest.NewRequest(http.MethodPost, "/api/sunoapi/callback", strings.NewReader(test.body))
			SunoAPICallback(context)
			assert.Equal(t, test.status, response.Code)
		})
	}
}
