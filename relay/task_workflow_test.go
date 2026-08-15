package relay

import (
	"errors"
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestShouldRetryTaskSubmission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name      string
		status    int
		local     bool
		remaining int
		want      bool
	}{
		{name: "rate limited", status: http.StatusTooManyRequests, remaining: 1, want: true},
		{name: "temporary redirect", status: http.StatusTemporaryRedirect, remaining: 1, want: true},
		{name: "upstream failure", status: http.StatusBadGateway, remaining: 1, want: true},
		{name: "bad request", status: http.StatusBadRequest, remaining: 1, want: false},
		{name: "timeout", status: http.StatusRequestTimeout, remaining: 1, want: false},
		{name: "local error", status: http.StatusInternalServerError, local: true, remaining: 1, want: false},
		{name: "exhausted", status: http.StatusTooManyRequests, remaining: 0, want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			context, _ := gin.CreateTestContext(nil)
			taskErr := &dto.TaskError{StatusCode: test.status, LocalError: test.local, Error: errors.New("failed")}
			assert.Equal(t, test.want, ShouldRetryTaskSubmission(context, taskErr, test.remaining))
		})
	}
}

func TestShouldRetryTaskSubmissionHonorsSpecificChannel(t *testing.T) {
	context, _ := gin.CreateTestContext(nil)
	context.Set("specific_channel_id", 9)
	taskErr := &dto.TaskError{StatusCode: http.StatusTooManyRequests, Error: errors.New("busy")}

	assert.False(t, ShouldRetryTaskSubmission(context, taskErr, 1))
}

func TestTaskModelFromSubmitPreservesExecutionAndBillingSnapshot(t *testing.T) {
	info := &relaycommon.RelayInfo{
		UserId: 7, UsingGroup: "vip", OriginModelName: "video-upstream",
		ChannelMeta:   &relaycommon.ChannelMeta{ChannelId: 3},
		TaskRelayInfo: &relaycommon.TaskRelayInfo{PublicTaskID: "task_public", Action: "generate"},
	}
	info.PriceData.ModelPrice = 1.25
	info.PriceData.ModelRatio = 2
	info.PriceData.GroupRatioInfo.GroupRatio = 1.5
	result := &TaskSubmitResult{
		UpstreamTaskID: "upstream_private", TaskData: []byte(`{"id":"upstream_private"}`),
		Platform: constant.TaskPlatform("openrouter"), Quota: 120,
	}

	task := taskModelFromSubmit(info, result)

	assert.Equal(t, "task_public", task.TaskID)
	assert.Equal(t, "upstream_private", task.PrivateData.UpstreamTaskID)
	assert.Equal(t, 7, task.UserId)
	assert.Equal(t, "vip", task.Group)
	assert.Equal(t, 3, task.ChannelId)
	assert.Equal(t, 120, task.Quota)
	assert.Equal(t, "generate", task.Action)
	assert.Equal(t, 1.25, task.PrivateData.BillingContext.ModelPrice)
	assert.Equal(t, 2.0, task.PrivateData.BillingContext.ModelRatio)
	assert.Equal(t, 1.5, task.PrivateData.BillingContext.GroupRatio)
}
