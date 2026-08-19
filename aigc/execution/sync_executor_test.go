package execution

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	aigcdto "github.com/QuantumNous/new-api/aigc/dto"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaydto "github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type syncWorkflowStub struct {
	context *gin.Context
	format  types.RelayFormat
	result  *relay.SyncWorkflowResult
	err     *types.NewAPIError
}

func (stub *syncWorkflowStub) Execute(c *gin.Context, format types.RelayFormat) (*relay.SyncWorkflowResult, *types.NewAPIError) {
	stub.context = c
	stub.format = format
	return stub.result, stub.err
}

func TestBuildSyncRequestPreservesResolvedTextContract(t *testing.T) {
	spec := Spec{UpstreamModelID: "gpt-upstream", ModelType: "text", Mode: "text", Request: aigcdto.GenerationRequest{Prompt: "write clearly"}}

	request, path, format, err := BuildSyncRequest(spec)

	require.NoError(t, err)
	assert.Equal(t, "/v1/chat/completions", path)
	assert.Equal(t, types.RelayFormatOpenAI, format)
	text, ok := request.(*relaydto.GeneralOpenAIRequest)
	require.True(t, ok)
	assert.Equal(t, "gpt-upstream", text.Model)
	require.Len(t, text.Messages, 1)
	assert.Equal(t, "user", text.Messages[0].Role)
	assert.Equal(t, "write clearly", text.Messages[0].StringContent())
	assert.NotNil(t, text.Stream)
	assert.False(t, *text.Stream)
}

func TestBuildSyncRequestPreservesResolvedImageContract(t *testing.T) {
	spec := Spec{UpstreamModelID: "image-upstream", ModelType: "image", Mode: "image_edit", Request: aigcdto.GenerationRequest{
		Prompt: "change the sky", Inputs: aigcdto.GenerationInputs{Images: []aigcdto.MediaInput{
			{Role: "source_image", URL: "https://cdn.test/one.png"},
			{Role: "source_image", URL: "https://cdn.test/two.png"},
		}}, Output: aigcdto.GenerationOutput{Size: "1024x1024", Count: 2},
	}}

	request, path, format, err := BuildSyncRequest(spec)

	require.NoError(t, err)
	assert.Equal(t, "/v1/images/edits", path)
	assert.Equal(t, types.RelayFormat(types.RelayFormatOpenAIImage), format)
	image, ok := request.(*relaydto.ImageRequest)
	require.True(t, ok)
	assert.Equal(t, "image-upstream", image.Model)
	assert.Equal(t, "change the sky", image.Prompt)
	assert.Equal(t, "1024x1024", image.Size)
	assert.Equal(t, "url", image.ResponseFormat)
	require.NotNil(t, image.N)
	assert.Equal(t, uint(2), *image.N)
	assert.JSONEq(t, `["https://cdn.test/one.png","https://cdn.test/two.png"]`, string(image.Images))
}

func TestSyncExecutorReturnsCompletedTextOutput(t *testing.T) {
	gin.SetMode(gin.TestMode)
	source, _ := gin.CreateTestContext(httptest.NewRecorder())
	source.Request = httptest.NewRequest(http.MethodPost, "/v1/aigc/generations", nil)
	source.Set("id", 7)
	source.Set("token_id", 11)
	common.SetContextKey(source, constant.ContextKeyUsingGroup, "vip")
	workflow := &syncWorkflowStub{result: &relay.SyncWorkflowResult{ResponseBody: []byte(`{
		"id":"chat-1","choices":[{"index":0,"message":{"role":"assistant","content":"finished text"},"finish_reason":"stop"}],
		"usage":{"prompt_tokens":3,"completion_tokens":4,"total_tokens":7}
	}`)}}
	executor := NewSyncExecutor(workflow)

	result, err := executor.Execute(WithGinContext(context.Background(), source), Identity{UserID: 7, TokenID: 11, Group: "vip"}, Spec{
		UpstreamModelID: "gpt-upstream", ModelType: "text", Mode: "text", Request: aigcdto.GenerationRequest{Prompt: "write"},
	})

	require.NoError(t, err)
	assert.Equal(t, "completed", result.Status)
	assert.Equal(t, 100, result.Progress)
	require.Len(t, result.Outputs, 1)
	assert.Equal(t, "text", result.Outputs[0].Type)
	assert.Equal(t, "finished text", result.Outputs[0].Text)
	assert.Equal(t, types.RelayFormatOpenAI, workflow.format)
	assert.Equal(t, "/v1/chat/completions", workflow.context.Request.URL.Path)
	assert.Equal(t, "gpt-upstream", common.GetContextKeyString(workflow.context, constant.ContextKeyOriginalModel))
}

func TestSyncExecutorReturnsReplayableImageURLs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	source, _ := gin.CreateTestContext(httptest.NewRecorder())
	source.Request = httptest.NewRequest(http.MethodPost, "/v1/aigc/generations", nil)
	workflow := &syncWorkflowStub{result: &relay.SyncWorkflowResult{ResponseBody: []byte(`{
		"created":1,"data":[{"url":"https://cdn.test/one.png"},{"url":"https://cdn.test/two.png"}]
	}`)}}
	executor := NewSyncExecutor(workflow)

	result, err := executor.Execute(WithGinContext(context.Background(), source), Identity{UserID: 7, TokenID: 11, Group: "vip"}, Spec{
		UpstreamModelID: "image-upstream", ModelType: "image", Mode: "text_to_image",
		Request: aigcdto.GenerationRequest{Prompt: "draw", Output: aigcdto.GenerationOutput{Size: "1024x1024", Count: 2}},
	})

	require.NoError(t, err)
	require.Len(t, result.Outputs, 2)
	assert.Equal(t, "https://cdn.test/one.png", result.Outputs[0].URL)
	assert.Equal(t, "https://cdn.test/two.png", result.Outputs[1].URL)
}

func TestSyncExecutorReturnsInlineImageBase64(t *testing.T) {
	gin.SetMode(gin.TestMode)
	source, _ := gin.CreateTestContext(httptest.NewRecorder())
	source.Request = httptest.NewRequest(http.MethodPost, "/v1/aigc/generations", nil)
	executor := NewSyncExecutor(&syncWorkflowStub{result: &relay.SyncWorkflowResult{ResponseBody: []byte(`{"data":[{"b64_json":"large-inline-data"}]}`)}})

	result, err := executor.Execute(WithGinContext(context.Background(), source), Identity{UserID: 7, TokenID: 11, Group: "vip"}, Spec{
		UpstreamModelID: "image-upstream", ModelType: "image", Mode: "text_to_image", Request: aigcdto.GenerationRequest{Prompt: "draw"},
	})

	require.NoError(t, err)
	require.Len(t, result.Outputs, 1)
	assert.Empty(t, result.Outputs[0].URL)
	assert.Equal(t, "large-inline-data", result.Outputs[0].B64JSON)
}
