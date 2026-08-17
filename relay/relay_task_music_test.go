package relay

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestApplyTaskInputImagePricingSkipsSunoRequestContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Set("task_request", &dto.SunoSubmitReq{Mv: "chirp-v4"})
	info := &relaycommon.RelayInfo{}

	taskErr := applyTaskInputImagePricing(context, info, constant.TaskPlatformSuno)

	require.Nil(t, taskErr)
}

func TestApplyTaskInputImagePricingStillRejectsInvalidVideoRequestContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Set("task_request", &dto.SunoSubmitReq{Mv: "chirp-v4"})
	info := &relaycommon.RelayInfo{}

	taskErr := applyTaskInputImagePricing(context, info, constant.TaskPlatform("ali"))

	require.NotNil(t, taskErr)
	require.Equal(t, "invalid_request", taskErr.Code)
}
