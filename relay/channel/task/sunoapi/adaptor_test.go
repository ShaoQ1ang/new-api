package sunoapi

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildRequestBodyPreservesSunoAPIContractAndExplicitZero(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	zero := 0.0
	c.Set("task_request", relaycommon.TaskSubmitReq{
		Prompt: "creative brief", Model: "V5_5",
		Metadata: map[string]any{
			"customMode": true, "instrumental": false, "callBackUrl": "https://app.test/api/sunoapi/callback",
			"lyrics": "exact lyrics", "style": "ambient pop", "title": "Night", "audioWeight": zero,
		},
	})

	body, err := (&TaskAdaptor{}).BuildRequestBody(c, &relaycommon.RelayInfo{})
	require.NoError(t, err)
	encoded, err := io.ReadAll(body)
	require.NoError(t, err)
	assert.JSONEq(t, `{"customMode":true,"instrumental":false,"model":"V5_5","callBackUrl":"https://app.test/api/sunoapi/callback","prompt":"exact lyrics","style":"ambient pop","title":"Night","audioWeight":0}`, string(encoded))
}

func TestFetchTaskUsesAuthenticatedRecordInfoGET(t *testing.T) {
	service.InitHttpClient()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/api/v1/generate/record-info", r.URL.Path)
		assert.Equal(t, "native task", r.URL.Query().Get("taskId"))
		assert.Equal(t, "Bearer secret", r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	resp, err := (&TaskAdaptor{}).FetchTask(server.URL, "secret", map[string]any{"task_id": "native task"}, "")
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
}

func TestParseTaskResultMapsProviderLifecycle(t *testing.T) {
	adaptor := &TaskAdaptor{}
	tests := []struct {
		status   string
		expected model.TaskStatus
		progress string
	}{
		{"PENDING", model.TaskStatusQueued, "10%"},
		{"TEXT_SUCCESS", model.TaskStatusInProgress, "35%"},
		{"FIRST_SUCCESS", model.TaskStatusInProgress, "70%"},
		{"CALLBACK_EXCEPTION", model.TaskStatusInProgress, "70%"},
		{"SUCCESS", model.TaskStatusSuccess, "100%"},
		{"GENERATE_AUDIO_FAILED", model.TaskStatusFailure, "100%"},
	}
	for _, test := range tests {
		t.Run(test.status, func(t *testing.T) {
			data := map[string]any{"taskId": "native", "status": test.status}
			if test.status == "SUCCESS" {
				data["response"] = map[string]any{"sunoData": []map[string]any{{"audioUrl": "https://cdn.test/1.mp3"}, {"audioUrl": "https://cdn.test/2.mp3"}}}
			}
			body, err := common.Marshal(map[string]any{"code": 200, "msg": "success", "data": data})
			require.NoError(t, err)
			result, err := adaptor.ParseTaskResult(body)
			require.NoError(t, err)
			assert.Equal(t, test.expected, model.TaskStatus(result.Status))
			assert.Equal(t, test.progress, result.Progress)
		})
	}
}

func TestParseTaskResultAcceptsNumericCreateTime(t *testing.T) {
	adaptor := &TaskAdaptor{}
	body := []byte(`{"code":200,"msg":"success","data":{"taskId":"native","status":"SUCCESS","response":{"sunoData":[{"audioUrl":"https://cdn.test/1.mp3","createTime":1750000000000},{"audioUrl":"https://cdn.test/2.mp3","createTime":"1750000000000"}]}}}`)

	result, err := adaptor.ParseTaskResult(body)
	require.NoError(t, err)
	assert.Equal(t, string(model.TaskStatusSuccess), result.Status)
	assert.Equal(t, "100%", result.Progress)
}
