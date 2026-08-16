package execution

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/aigc/dto"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaydto "github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type taskWorkflowStub struct {
	context *gin.Context
	info    *relaycommon.RelayInfo
	request relaycommon.TaskSubmitReq
	music   relaydto.SunoSubmitReq
	result  *relay.TaskWorkflowResult
	err     *relaydto.TaskError
	storage bool
}

func (stub *taskWorkflowStub) Submit(c *gin.Context, info *relaycommon.RelayInfo) (*relay.TaskWorkflowResult, *relaydto.TaskError) {
	stub.context, stub.info = c, info
	var contents []byte
	if stub.storage {
		body, _ := common.GetBodyStorage(c)
		contents, _ = body.Bytes()
	} else {
		contents, _ = io.ReadAll(c.Request.Body)
	}
	_ = common.Unmarshal(contents, &stub.request)
	_ = common.Unmarshal(contents, &stub.music)
	return stub.result, stub.err
}

func TestTaskExecutorSubmitsResolvedMusicThroughSharedWorkflow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	requestContext, _ := gin.CreateTestContext(httptest.NewRecorder())
	requestContext.Request = httptest.NewRequest(http.MethodPost, "/v1/aigc/generations", nil)
	workflow := &taskWorkflowStub{result: &relay.TaskWorkflowResult{Task: &model.Task{TaskID: "task_music", Status: model.TaskStatusQueued, Progress: "10%"}}}
	executor := NewTaskExecutor(workflow, &taskStoreStub{})
	spec := Spec{UpstreamModelID: "chirp-v4", ModelType: "music", Mode: "text_to_music", Adapter: "music-task",
		Request: dto.GenerationRequest{Prompt: "bright synthwave", Parameters: dto.GenerationParameters{Instrumental: true}}}

	result, err := executor.Execute(WithGinContext(context.Background(), requestContext), Identity{UserID: 7, TokenID: 11, Group: "vip"}, spec)

	require.NoError(t, err)
	assert.Equal(t, "queued", result.Status)
	assert.Equal(t, "/suno/submit/music", workflow.context.Request.URL.Path)
	assert.Equal(t, "music", workflow.context.Param("action"))
	assert.Equal(t, string(constant.TaskPlatformSuno), workflow.context.GetString("platform"))
	assert.Equal(t, "bright synthwave", workflow.music.GptDescriptionPrompt)
	assert.Equal(t, "chirp-v4", workflow.music.Mv)
	assert.True(t, workflow.music.MakeInstrumental)
}

func TestTaskExecutorPollMapsCompletedMusicTracks(t *testing.T) {
	data, err := common.Marshal([]relaydto.SunoSong{
		{ID: "song-1", Title: "First", AudioURL: "https://cdn.test/first.mp3", ImageURL: "https://cdn.test/first.jpg"},
		{ID: "song-2", Title: "Second", AudioURL: "https://cdn.test/second.mp3", ImageURL: "https://cdn.test/second.jpg"},
	})
	require.NoError(t, err)
	store := &taskStoreStub{found: true, task: &model.Task{TaskID: "task_music", Status: model.TaskStatusSuccess, Progress: "100%", Data: data}}
	executor := NewTaskExecutor(&taskWorkflowStub{}, store)

	result, err := executor.Poll(context.Background(), Identity{UserID: 7}, "music", "task_music")

	require.NoError(t, err)
	require.Len(t, result.Outputs, 2)
	assert.Equal(t, "music", result.Outputs[0].Type)
	assert.Equal(t, "song-1", result.Outputs[0].ID)
	assert.Equal(t, "First", result.Outputs[0].Title)
	assert.Equal(t, "https://cdn.test/first.mp3", result.Outputs[0].URL)
	assert.Equal(t, "https://cdn.test/first.jpg", result.Outputs[0].PosterURL)
}

type taskStoreStub struct {
	task   *model.Task
	found  bool
	err    error
	userID int
	taskID string
}

func (stub *taskStoreStub) Get(userID int, taskID string) (*model.Task, bool, error) {
	stub.userID = userID
	stub.taskID = taskID
	return stub.task, stub.found, stub.err
}

func TestTaskExecutorSubmitsResolvedVideoThroughSharedWorkflow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	requestContext, _ := gin.CreateTestContext(httptest.NewRecorder())
	requestContext.Request = httptest.NewRequest(http.MethodPost, "/v1/aigc/generations", nil)
	requestContext.Set("id", 7)
	requestContext.Set("token_id", 11)
	requestContext.Set(common.KeyRequestBody, []byte(`{"model":"public-stale"}`))
	common.SetContextKey(requestContext, constant.ContextKeyUsingGroup, "vip")
	workflow := &taskWorkflowStub{storage: true, result: &relay.TaskWorkflowResult{Task: &model.Task{TaskID: "task_public", Status: model.TaskStatusQueued, Progress: "20%", Quota: 90}}}
	executor := NewTaskExecutor(workflow, &taskStoreStub{})
	audio := true
	spec := Spec{
		PublicModelID: "video-public", UpstreamModelID: "video-upstream", ModelType: "video", Mode: "first_frame", TaskProtocol: TaskProtocolNewAPIVideo,
		Request: dto.GenerationRequest{Prompt: "move", Inputs: dto.GenerationInputs{Images: []dto.MediaInput{{Role: "first_frame", URL: "https://cdn.test/first.png"}}},
			Output: dto.GenerationOutput{Resolution: "720p", AspectRatio: "16:9", Duration: 5, GenerateAudio: &audio}},
	}

	result, err := executor.Execute(WithGinContext(context.Background(), requestContext), Identity{UserID: 7, TokenID: 11, Group: "vip"}, spec)

	require.NoError(t, err)
	assert.Equal(t, "queued", result.Status)
	assert.Equal(t, 20, result.Progress)
	assert.Equal(t, "task_public", result.NativeTaskID)
	assert.Equal(t, 90, result.Usage.Quota)
	assert.Equal(t, http.MethodPost, workflow.context.Request.Method)
	assert.Equal(t, "/v1/videos", workflow.context.Request.URL.Path)
	assert.Equal(t, "video-upstream", workflow.info.OriginModelName)
	assert.Equal(t, "video-upstream", workflow.request.Model)
	assert.Equal(t, []string{"first_frame"}, workflow.request.ImageRoles)
	assert.Equal(t, 7, workflow.info.UserId)
	assert.Equal(t, 11, workflow.info.TokenId)
	assert.Equal(t, "vip", workflow.info.UsingGroup)
}

func TestTaskExecutorRequiresAuthenticatedGinContext(t *testing.T) {
	executor := NewTaskExecutor(&taskWorkflowStub{}, &taskStoreStub{})
	_, err := executor.Execute(context.Background(), Identity{UserID: 7, TokenID: 11, Group: "vip"}, Spec{
		UpstreamModelID: "video", ModelType: "video", TaskProtocol: TaskProtocolNewAPIVideo,
	})
	require.Error(t, err)
	protocolErr, ok := err.(*Error)
	require.True(t, ok)
	assert.Equal(t, "AIGC_EXECUTION_CONTEXT_MISSING", protocolErr.Code)
}

func TestTaskExecutorPollMapsCompletedVideoTask(t *testing.T) {
	task := &model.Task{TaskID: "task_public", Status: model.TaskStatusSuccess, Progress: "100%", Quota: 120,
		PrivateData: model.TaskPrivateData{ResultURL: "https://cdn.test/video.mp4", BillingContext: &model.TaskBillingContext{VideoDurationSeconds: 8}}}
	store := &taskStoreStub{task: task, found: true}
	executor := NewTaskExecutor(&taskWorkflowStub{}, store)

	result, err := executor.Poll(context.Background(), Identity{UserID: 7}, "video", "task_public")

	require.NoError(t, err)
	assert.Equal(t, "completed", result.Status)
	assert.Equal(t, 100, result.Progress)
	require.Len(t, result.Outputs, 1)
	assert.Equal(t, "https://cdn.test/video.mp4", result.Outputs[0].URL)
	assert.Equal(t, 8, result.Outputs[0].Duration)
	assert.Equal(t, 120, result.Usage.Quota)
	assert.Equal(t, 7, store.userID)
	assert.Equal(t, "task_public", store.taskID)
}
