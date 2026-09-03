package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/pkg/tracelog"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestRequestIDBindsIncomingTraceHeaderWhenEnabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := tracelog.New("newapi", t.TempDir(), 4)
	previous := tracelog.Default
	tracelog.Default = client
	defer func() {
		client.Close()
		tracelog.Default = previous
	}()

	router := gin.New()
	router.Use(RequestId())
	router.GET("/trace", func(c *gin.Context) {
		headers := http.Header{}
		tracelog.Default.InjectTraceID(c.Request.Context(), headers)
		c.String(http.StatusOK, headers.Get(tracelog.Header))
	})
	request := httptest.NewRequest(http.MethodGet, "/trace", nil)
	request.Header.Set(tracelog.Header, "turn-from-aigc")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	assert.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, "turn-from-aigc", response.Body.String())
}
