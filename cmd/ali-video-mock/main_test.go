package main

import (
	"bytes"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	taskali "github.com/QuantumNous/new-api/relay/channel/task/ali"
	"github.com/abema/go-mp4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSupportedAliVideoModelsCompleteLifecycle(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		family     string
		resolution int
		audio      bool
		watermark  bool
	}{
		{
			name:       "wan text to video",
			body:       `{"model":"wan2.7-t2v","input":{"prompt":"a sunrise"},"parameters":{"size":"1280*720","duration":5}}`,
			family:     "wan",
			resolution: 720,
			audio:      true,
		},
		{
			name:       "wan first and last frame",
			body:       `{"model":"wan2.7-i2v","input":{"prompt":"interpolate","media":[{"type":"first_frame","url":"https://example.com/first.jpg"},{"type":"last_frame","url":"https://example.com/last.jpg"}]},"parameters":{"resolution":"1080P","duration":10}}`,
			family:     "wan",
			resolution: 1080,
			audio:      true,
		},
		{
			name:       "happyhorse text to video",
			body:       `{"model":"happyhorse-1.1-t2v","input":{"prompt":"a running horse"},"parameters":{"resolution":"1080P","duration":5}}`,
			family:     "happyhorse",
			resolution: 1080,
			audio:      true,
		},
		{
			name:       "happyhorse image to video",
			body:       `{"model":"happyhorse-1.1-i2v","input":{"media":[{"type":"first_frame","url":"https://example.com/first.jpg"}]},"parameters":{"resolution":"720P","duration":5}}`,
			family:     "happyhorse",
			resolution: 720,
			audio:      true,
		},
		{
			name:       "happyhorse reference to video",
			body:       `{"model":"happyhorse-1.1-r2v","input":{"prompt":"preserve the subject","media":[{"type":"reference_image","url":"https://example.com/ref.jpg"}]},"parameters":{"resolution":"1080P","duration":10}}`,
			family:     "happyhorse",
			resolution: 1080,
			audio:      true,
		},
		{
			name:       "kling standard first and last frame",
			body:       `{"model":"kling/kling-v3-video-generation","input":{"prompt":"walk forward","media":[{"type":"first_frame","url":"https://example.com/first.jpg"},{"type":"last_frame","url":"https://example.com/last.jpg"}]},"parameters":{"mode":"std","duration":5,"audio":true}}`,
			family:     "kling",
			resolution: 720,
			audio:      true,
		},
		{
			name:       "kling omni video edit",
			body:       `{"model":"kling/kling-v3-omni-video-generation","input":{"prompt":"change the background","media":[{"type":"base","url":"https://example.com/base.mp4"},{"type":"refer","url":"https://example.com/ref.jpg"}],"element_list":[{"element_id":1}]},"parameters":{"mode":"pro","duration":10,"audio":false,"watermark":true}}`,
			family:     "kling",
			resolution: 1080,
			audio:      false,
			watermark:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := newMockServerWithConfig(mockConfig{CompleteAfterPoll: 2})
			submitted := submitTask(t, server, tt.body)

			server.mu.Lock()
			stored := *server.tasks[submitted.Output.TaskID]
			server.mu.Unlock()
			assert.Equal(t, tt.family, stored.Family)

			first := fetchTask(t, server, submitted.Output.TaskID)
			assert.Equal(t, mockTaskRunning, first.Output.TaskStatus)
			completed := fetchTask(t, server, submitted.Output.TaskID)
			require.Equal(t, mockTaskSuccess, completed.Output.TaskStatus)
			require.NotNil(t, completed.Usage)
			assert.Equal(t, tt.resolution, intValue(completed.Usage.SR))
			assert.Equal(t, tt.audio, completed.Usage.Audio)
			assert.Equal(t, 1, intValue(completed.Usage.VideoCount))
			assert.NotEmpty(t, completed.Output.VideoURL)
			if tt.watermark {
				assert.NotEmpty(t, completed.Output.WatermarkURL)
			} else {
				assert.Empty(t, completed.Output.WatermarkURL)
			}
		})
	}
}

func TestEveryConfiguredWanModelIsRecognized(t *testing.T) {
	for _, model := range taskali.ModelList {
		family, ok := detectModelFamily(model)
		assert.True(t, ok, model)
		assert.Equal(t, "wan", family, model)
	}
}

func TestMockVideoIsPlayableMP4AndSupportsRanges(t *testing.T) {
	server := newMockServer()
	handler := server.routes()

	fullReq := httptest.NewRequest(http.MethodGet, "/mock-assets/videos/sample.mp4", nil)
	fullResp := httptest.NewRecorder()
	handler.ServeHTTP(fullResp, fullReq)
	require.Equal(t, http.StatusOK, fullResp.Code)
	assert.Equal(t, "video/mp4", fullResp.Header().Get("Content-Type"))
	assert.Equal(t, strconv.Itoa(fullResp.Body.Len()), fullResp.Header().Get("Content-Length"))
	assert.Greater(t, fullResp.Body.Len(), 1000)
	probe, err := mp4.Probe(bytes.NewReader(fullResp.Body.Bytes()))
	require.NoError(t, err)
	assert.Greater(t, probe.Duration, uint64(0))
	assert.Greater(t, probe.Timescale, uint32(0))

	rangeReq := httptest.NewRequest(http.MethodGet, "/mock-assets/videos/sample.mp4", nil)
	rangeReq.Header.Set("Range", "bytes=0-31")
	rangeResp := httptest.NewRecorder()
	handler.ServeHTTP(rangeResp, rangeReq)
	require.Equal(t, http.StatusPartialContent, rangeResp.Code)
	assert.Len(t, rangeResp.Body.Bytes(), 32)
	assert.Equal(t, "bytes 0-31/"+strconv.Itoa(fullResp.Body.Len()), rangeResp.Header().Get("Content-Range"))

	headReq := httptest.NewRequest(http.MethodHead, "/mock-assets/videos/sample.mp4", nil)
	headResp := httptest.NewRecorder()
	handler.ServeHTTP(headResp, headReq)
	require.Equal(t, http.StatusOK, headResp.Code)
	assert.Empty(t, headResp.Body.Bytes())
	assert.Equal(t, strconv.Itoa(fullResp.Body.Len()), headResp.Header().Get("Content-Length"))
}

func TestRequestHistoryCapturesCompleteUpstreamInput(t *testing.T) {
	server := newMockServerWithConfig(mockConfig{CompleteAfterPoll: 1})
	handler := server.routes()
	body := `{"model":"google/veo-3.1-lite","prompt":"ocean waves","duration":4}`
	req := httptest.NewRequest(http.MethodPost, "/v1/videos?trace=converted&trace=second", strings.NewReader(body))
	req.Host = "video-mock:8080"
	req.Header.Set("Authorization", "Bearer mock-debug-key")
	req.Header.Set("X-Newapi-Request-Id", "req-123")
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())

	historyResp := performRequest(t, handler, http.MethodGet, "/api/mock/history", "")
	require.Equal(t, http.StatusOK, historyResp.Code, historyResp.Body.String())
	var history mockHistoryResponse
	require.NoError(t, common.Unmarshal(historyResp.Body.Bytes(), &history))
	require.Equal(t, 1, history.Count)
	require.Len(t, history.Records, 1)
	record := history.Records[0]
	assert.Equal(t, int64(1), record.ID)
	assert.Equal(t, http.MethodPost, record.Method)
	assert.Equal(t, "/v1/videos", record.Path)
	assert.Equal(t, "trace=converted&trace=second", record.RawQuery)
	assert.Equal(t, []string{"converted", "second"}, record.Query["trace"])
	assert.Equal(t, "Bearer mock-debug-key", record.Headers.Get("Authorization"))
	assert.Equal(t, "req-123", record.Headers.Get("X-Newapi-Request-Id"))
	assert.Equal(t, body, record.Body)
	assert.Equal(t, int64(len(body)), record.ContentLength)
	assert.Equal(t, "video-mock:8080", record.Host)
	assert.Equal(t, "OpenRouter", record.Protocol)
	assert.Equal(t, http.StatusOK, record.Status)
}

func TestRequestHistoryCapturesErrorsAndExcludesInspectorTraffic(t *testing.T) {
	server := newMockServer()
	handler := server.routes()

	invalid := performRequest(t, handler, http.MethodPost, "/v1/videos", `{"model":"unsupported","prompt":"test"}`)
	require.Equal(t, http.StatusBadRequest, invalid.Code)
	assert.Equal(t, http.StatusOK, performRequest(t, handler, http.MethodGet, "/healthz", "").Code)
	assert.Equal(t, http.StatusOK, performRequest(t, handler, http.MethodGet, "/history", "").Code)
	assert.Equal(t, http.StatusOK, performRequest(t, handler, http.MethodGet, "/api/mock/history", "").Code)
	assert.Equal(t, http.StatusNotFound, performRequest(t, handler, http.MethodGet, "/favicon.ico", "").Code)
	assert.Equal(t, http.StatusOK, performRequest(t, handler, http.MethodGet, "/mock-assets/videos/sample.mp4", "").Code)

	historyResp := performRequest(t, handler, http.MethodGet, "/api/mock/history", "")
	var history mockHistoryResponse
	require.NoError(t, common.Unmarshal(historyResp.Body.Bytes(), &history))
	require.Len(t, history.Records, 1)
	assert.Equal(t, http.StatusBadRequest, history.Records[0].Status)

	clearResp := performRequest(t, handler, http.MethodDelete, "/api/mock/history", "")
	require.Equal(t, http.StatusOK, clearResp.Code)
	emptyResp := performRequest(t, handler, http.MethodGet, "/api/mock/history", "")
	require.NoError(t, common.Unmarshal(emptyResp.Body.Bytes(), &history))
	assert.Empty(t, history.Records)
}

func TestRequestHistoryKeepsNewestRecordsWithinCapacity(t *testing.T) {
	server := newMockServer()
	for index := 0; index < mockHistoryLimit+2; index++ {
		server.storeRequestRecord(mockRequestRecord{Path: "/request/" + strconv.Itoa(index)})
	}

	require.Len(t, server.history, mockHistoryLimit)
	assert.Equal(t, "/request/2", server.history[0].Path)
	assert.Equal(t, int64(mockHistoryLimit+2), server.history[mockHistoryLimit-1].ID)
}

func TestInvalidModelRequestsAreRejected(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "unsupported model", body: `{"model":"unknown-video","input":{"prompt":"test"}}`},
		{name: "wan image missing", body: `{"model":"wan2.7-i2v","input":{"prompt":"test"}}`},
		{name: "happyhorse references missing", body: `{"model":"happyhorse-1.1-r2v","input":{"prompt":"test"}}`},
		{name: "kling standard reference", body: `{"model":"kling/kling-v3-video-generation","input":{"prompt":"test","media":[{"type":"refer","url":"https://example.com/ref.jpg"}]}}`},
		{name: "kling omni video audio", body: `{"model":"kling/kling-v3-omni-video-generation","input":{"prompt":"test","media":[{"type":"base","url":"https://example.com/base.mp4"}]},"parameters":{"audio":true}}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := newMockServer()
			req := httptest.NewRequest(http.MethodPost, "/api/v1/services/aigc/video-generation/video-synthesis", strings.NewReader(tt.body))
			resp := httptest.NewRecorder()
			server.routes().ServeHTTP(resp, req)
			assert.Equal(t, http.StatusBadRequest, resp.Code)
		})
	}
}

func TestTaskLifecycleCanFailAtConfiguredPoll(t *testing.T) {
	server := newMockServerWithConfig(mockConfig{FailRate: 1, CompleteAfterPoll: 3})
	submitted := submitTask(t, server, `{"model":"happyhorse-1.1-t2v","input":{"prompt":"a running horse"},"parameters":{"duration":5}}`)

	assert.Equal(t, mockTaskRunning, fetchTask(t, server, submitted.Output.TaskID).Output.TaskStatus)
	assert.Equal(t, mockTaskRunning, fetchTask(t, server, submitted.Output.TaskID).Output.TaskStatus)
	failed := fetchTask(t, server, submitted.Output.TaskID)
	assert.Equal(t, mockTaskFailed, failed.Output.TaskStatus)
	assert.Empty(t, failed.Output.VideoURL)
	assert.Contains(t, failed.Output.Message, "mock upstream random failure")
}

func TestPublicBaseURLOverridesRequestHost(t *testing.T) {
	server := newMockServerWithConfig(mockConfig{PublicBaseURL: "https://mock.example.test/base/"})
	submitted := submitTask(t, server, `{"model":"wan2.7-t2v","input":{"prompt":"test"}}`)

	server.mu.Lock()
	videoURL := server.tasks[submitted.Output.TaskID].VideoURL
	server.mu.Unlock()
	assert.Equal(t, "https://mock.example.test/base/mock-assets/videos/"+submitted.Output.TaskID+".mp4", videoURL)
}

func TestSeedanceMultimodalLifecycleReturnsVideo(t *testing.T) {
	server := newMockServerWithConfig(mockConfig{CompleteAfterPoll: 2})
	handler := server.routes()
	body := `{"model":"doubao-seedance-2-0-260128","content":[{"type":"text","text":"cinematic city"},{"type":"image_url","image_url":{"url":"https://example.com/ref.png"}},{"type":"video_url","video_url":{"url":"https://example.com/motion.mp4"}},{"type":"audio_url","audio_url":{"url":"https://example.com/music.mp3"}}],"generate_audio":true,"resolution":"1080p","ratio":"16:9","duration":10,"seed":123}`

	submitResp := performRequest(t, handler, http.MethodPost, "/api/v3/contents/generations/tasks", body)
	require.Equal(t, http.StatusOK, submitResp.Code, submitResp.Body.String())
	var submitted struct {
		ID string `json:"id"`
	}
	require.NoError(t, common.Unmarshal(submitResp.Body.Bytes(), &submitted))
	require.NotEmpty(t, submitted.ID)

	first := fetchSeedance(t, handler, submitted.ID)
	assert.Equal(t, "running", first.Status)
	completed := fetchSeedance(t, handler, submitted.ID)
	require.Equal(t, "succeeded", completed.Status)
	assert.Equal(t, "1080p", completed.Resolution)
	assert.Equal(t, 10, completed.Duration)
	assert.Equal(t, 1000, completed.Usage.TotalTokens)
	assert.Contains(t, completed.Content.VideoURL, "/mock-assets/videos/")

	videoResp := performRequest(t, handler, http.MethodGet, completed.Content.VideoURL, "")
	require.Equal(t, http.StatusOK, videoResp.Code)
	_, err := mp4.Probe(bytes.NewReader(videoResp.Body.Bytes()))
	require.NoError(t, err)
}

func TestSeedanceFailureLifecycle(t *testing.T) {
	server := newMockServerWithConfig(mockConfig{FailRate: 1, CompleteAfterPoll: 1})
	handler := server.routes()
	submitResp := performRequest(t, handler, http.MethodPost, "/api/v3/contents/generations/tasks", `{"model":"doubao-seedance-2-0-fast-260128","content":[{"type":"text","text":"test"}]}`)
	var submitted struct {
		ID string `json:"id"`
	}
	require.NoError(t, common.Unmarshal(submitResp.Body.Bytes(), &submitted))
	failed := fetchSeedance(t, handler, submitted.ID)
	assert.Equal(t, "failed", failed.Status)
	assert.Equal(t, "MockFailure", failed.Error.Code)
	assert.Empty(t, failed.Content.VideoURL)
}

func TestGeminiVeoTextAndImageLifecycle(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "text", body: `{"instances":[{"prompt":"ocean waves"}],"parameters":{"durationSeconds":8,"aspectRatio":"16:9","resolution":"1080p","generateAudio":true}}`},
		{name: "image", body: `{"instances":[{"prompt":"animate","image":{"bytesBase64Encoded":"aW1hZ2U=","mimeType":"image/png"}}],"parameters":{"durationSeconds":6,"resolution":"720p"}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := newMockServerWithConfig(mockConfig{CompleteAfterPoll: 2})
			handler := server.routes()
			submitResp := performRequest(t, handler, http.MethodPost, "/v1beta/models/veo-3.1-generate-preview:predictLongRunning", tt.body)
			require.Equal(t, http.StatusOK, submitResp.Code, submitResp.Body.String())
			var submitted struct {
				Name string `json:"name"`
			}
			require.NoError(t, common.Unmarshal(submitResp.Body.Bytes(), &submitted))
			assert.Contains(t, submitted.Name, "models/veo-3.1-generate-preview/operations/")

			first := fetchGeminiVeo(t, handler, submitted.Name)
			assert.False(t, first.Done)
			completed := fetchGeminiVeo(t, handler, submitted.Name)
			require.True(t, completed.Done)
			require.Len(t, completed.Response.GenerateVideoResponse.GeneratedVideos, 1)
			assert.Contains(t, completed.Response.GenerateVideoResponse.GeneratedVideos[0].Video.URI, "/mock-assets/videos/")
		})
	}
}

func TestGeminiVeoFailureOperation(t *testing.T) {
	server := newMockServerWithConfig(mockConfig{FailRate: 1, CompleteAfterPoll: 1})
	handler := server.routes()
	submitResp := performRequest(t, handler, http.MethodPost, "/v1beta/models/veo-3.0-fast-generate-001:predictLongRunning", `{"instances":[{"prompt":"test"}]}`)
	var submitted struct {
		Name string `json:"name"`
	}
	require.NoError(t, common.Unmarshal(submitResp.Body.Bytes(), &submitted))
	failed := fetchGeminiVeo(t, handler, submitted.Name)
	assert.True(t, failed.Done)
	assert.Contains(t, failed.Error.Message, "mock upstream random failure")
}

func TestOpenRouterVeoCompleteLifecycle(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "text to video",
			body: `{"model":"google/veo-3.1-lite","prompt":"ocean waves","duration":8,"resolution":"1080p","aspect_ratio":"16:9","generate_audio":true}`,
		},
		{
			name: "first last and references",
			body: `{"model":"google/veo-3.1-quality","prompt":"interpolate naturally","duration":6,"resolution":"720p","aspect_ratio":"9:16","frame_images":[{"frame_type":"first_frame","image_url":{"url":"https://example.com/first.png"}},{"frame_type":"last_frame","image_url":{"url":"https://example.com/last.png"}}],"input_references":[{"type":"image","url":"https://example.com/ref.png"}]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := newMockServerWithConfig(mockConfig{CompleteAfterPoll: 2})
			handler := server.routes()
			submitResp := performRequest(t, handler, http.MethodPost, "/v1/videos", tt.body)
			require.Equal(t, http.StatusOK, submitResp.Code, submitResp.Body.String())
			var submitted openRouterVideoResponse
			require.NoError(t, common.Unmarshal(submitResp.Body.Bytes(), &submitted))
			require.NotEmpty(t, submitted.ID)
			assert.Equal(t, "queued", submitted.Status)
			assert.Contains(t, submitted.PollingURL, "/v1/videos/"+submitted.ID)

			processing := fetchOpenRouterVeo(t, handler, submitted.ID)
			assert.Equal(t, "processing", processing.Status)
			completed := fetchOpenRouterVeo(t, handler, submitted.ID)
			require.Equal(t, "completed", completed.Status)
			assert.Positive(t, completed.Usage.VideoTokens)
			assert.Equal(t, completed.Usage.VideoTokens, completed.Usage.TotalTokens)
			assert.Contains(t, completed.Output.VideoURL, "/mock-assets/videos/")

			videoResp := performRequest(t, handler, http.MethodGet, completed.Output.VideoURL, "")
			require.Equal(t, http.StatusOK, videoResp.Code)
			_, err := mp4.Probe(bytes.NewReader(videoResp.Body.Bytes()))
			require.NoError(t, err)
		})
	}
}

func TestOpenRouterSeedanceCompleteLifecycle(t *testing.T) {
	server := newMockServerWithConfig(mockConfig{CompleteAfterPoll: 2})
	handler := server.routes()
	body := `{
		"model":"bytedance/seedance-2.0",
		"prompt":"cinematic character sequence",
		"duration":15,
		"resolution":"4K",
		"aspect_ratio":"21:9",
		"generate_audio":true,
		"frame_images":[
			{"frame_type":"first_frame","image_url":{"url":"https://example.com/first.png"}},
			{"frame_type":"last_frame","image_url":{"url":"https://example.com/last.png"}}
		],
		"input_references":[
			{"type":"image","url":"https://example.com/character.png"},
			{"type":"video","url":"https://example.com/motion.mp4"},
			{"type":"audio","url":"https://example.com/dialogue.mp3"}
		],
		"watermark":true,
		"req_key":"mock-request",
		"seed":123,
		"provider":{"order":["ByteDance"]},
		"callback_url":"https://example.com/callback"
	}`
	submitResp := performRequest(t, handler, http.MethodPost, "/v1/videos", body)
	require.Equal(t, http.StatusOK, submitResp.Code, submitResp.Body.String())
	var submitted openRouterVideoResponse
	require.NoError(t, common.Unmarshal(submitResp.Body.Bytes(), &submitted))
	assert.True(t, strings.HasPrefix(submitted.ID, "mock-openrouter-seedance-"))
	assert.Equal(t, "queued", submitted.Status)

	processing := fetchOpenRouterVeo(t, handler, submitted.ID)
	assert.Equal(t, "processing", processing.Status)
	completed := fetchOpenRouterVeo(t, handler, submitted.ID)
	require.Equal(t, "completed", completed.Status)
	assert.Positive(t, completed.Usage.VideoTokens)
	assert.Contains(t, completed.Output.VideoURL, "/mock-assets/videos/")

	videoResp := performRequest(t, handler, http.MethodGet, completed.Output.VideoURL, "")
	require.Equal(t, http.StatusOK, videoResp.Code)
	_, err := mp4.Probe(bytes.NewReader(videoResp.Body.Bytes()))
	require.NoError(t, err)
}

func TestOpenRouterHappyHorseKlingAndMiniMaxLifecycle(t *testing.T) {
	tests := []struct {
		name   string
		family string
		body   string
	}{
		{name: "happyhorse", family: "happyhorse", body: `{"model":"alibaba/happyhorse-1.1","prompt":"a running horse","duration":5,"resolution":"1080p"}`},
		{name: "kling", family: "kling", body: `{"model":"kwaivgi/kling-v3.0-std","prompt":"camera push in","duration":5,"resolution":"720p","generate_audio":false}`},
		{name: "minimax", family: "minimax", body: `{"model":"minimax/hailuo-3","prompt":"cinematic","duration":5,"resolution":"2K","frame_images":[{"type":"image_url","frame_type":"first_frame","image_url":{"url":"https://example.com/first.png"}}]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := newMockServerWithConfig(mockConfig{CompleteAfterPoll: 1})
			handler := server.routes()
			submitResp := performRequest(t, handler, http.MethodPost, "/v1/videos", tt.body)
			require.Equal(t, http.StatusOK, submitResp.Code, submitResp.Body.String())
			var submitted openRouterVideoResponse
			require.NoError(t, common.Unmarshal(submitResp.Body.Bytes(), &submitted))
			assert.True(t, strings.HasPrefix(submitted.ID, "mock-openrouter-"+tt.family+"-"))
			completed := fetchOpenRouterVeo(t, handler, submitted.ID)
			assert.Equal(t, "completed", completed.Status)
		})
	}
}

func TestOpenRouterVideoModelsEndpoint(t *testing.T) {
	server := newMockServer()
	response := performRequest(t, server.routes(), http.MethodGet, "/v1/videos/models", "")
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	var payload struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &payload))
	ids := make([]string, 0, len(payload.Data))
	for _, item := range payload.Data {
		ids = append(ids, item.ID)
	}
	assert.Contains(t, ids, "alibaba/happyhorse-1.1")
	assert.Contains(t, ids, "kwaivgi/kling-v3.0-std")
	assert.Contains(t, ids, "minimax/hailuo-3")
}

func TestOpenRouterSeedanceRejectsInvalidOptions(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "duration below minimum", body: `{"model":"bytedance/seedance-2.0","prompt":"test","duration":3}`},
		{name: "duration above maximum", body: `{"model":"bytedance/seedance-2.0","prompt":"test","duration":16}`},
		{name: "resolution", body: `{"model":"bytedance/seedance-2.0","prompt":"test","resolution":"2k"}`},
		{name: "aspect ratio", body: `{"model":"bytedance/seedance-2.0","prompt":"test","aspect_ratio":"2:1"}`},
		{name: "reference type", body: `{"model":"bytedance/seedance-2.0","prompt":"test","input_references":[{"type":"document","url":"https://example.com/ref.pdf"}]}`},
		{name: "too many videos", body: `{"model":"bytedance/seedance-2.0","prompt":"test","input_references":[{"type":"video","url":"https://example.com/1.mp4"},{"type":"video","url":"https://example.com/2.mp4"},{"type":"video","url":"https://example.com/3.mp4"},{"type":"video","url":"https://example.com/4.mp4"}]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := newMockServer()
			resp := performRequest(t, server.routes(), http.MethodPost, "/v1/videos", tt.body)
			assert.Equal(t, http.StatusBadRequest, resp.Code, resp.Body.String())
			assert.Contains(t, resp.Body.String(), `"error"`)
		})
	}
}

func TestOpenRouterVeoRejectsUnsupportedRequests(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "wrong family", body: `{"model":"openai/sora-2","prompt":"test"}`},
		{name: "missing prompt", body: `{"model":"google/veo-3.1-lite"}`},
		{name: "duration", body: `{"model":"google/veo-3.1-lite","prompt":"test","duration":5}`},
		{name: "resolution", body: `{"model":"google/veo-3.1-lite","prompt":"test","resolution":"4K"}`},
		{name: "aspect ratio", body: `{"model":"google/veo-3.1-lite","prompt":"test","aspect_ratio":"1:1"}`},
		{name: "video reference", body: `{"model":"google/veo-3.1-lite","prompt":"test","input_references":[{"type":"video","url":"https://example.com/ref.mp4"}]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := newMockServer()
			resp := performRequest(t, server.routes(), http.MethodPost, "/v1/videos", tt.body)
			assert.Equal(t, http.StatusBadRequest, resp.Code, resp.Body.String())
			assert.Contains(t, resp.Body.String(), `"error"`)
		})
	}
}

func TestOpenRouterVideoFailureLifecycle(t *testing.T) {
	requests := []string{
		`{"model":"google/veo-3.1-lite","prompt":"test","duration":4}`,
		`{"model":"bytedance/seedance-2.0","prompt":"test","duration":4}`,
	}
	for _, body := range requests {
		server := newMockServerWithConfig(mockConfig{FailRate: 1, CompleteAfterPoll: 1})
		handler := server.routes()
		submitResp := performRequest(t, handler, http.MethodPost, "/v1/videos", body)
		var submitted openRouterVideoResponse
		require.NoError(t, common.Unmarshal(submitResp.Body.Bytes(), &submitted))
		failed := fetchOpenRouterVeo(t, handler, submitted.ID)
		assert.Equal(t, "failed", failed.Status)
		assert.Equal(t, "mock_failure", failed.Error.Code)
		assert.Empty(t, failed.Output.VideoURL)
	}
}

func TestVertexVeoLifecycleReturnsEmbeddedPlayableVideo(t *testing.T) {
	server := newMockServerWithConfig(mockConfig{CompleteAfterPoll: 2})
	handler := server.routes()
	submitPath := "/v1/projects/demo/locations/global/publishers/google/models/veo-3.1-fast-generate-preview:predictLongRunning"
	submitResp := performRequest(t, handler, http.MethodPost, submitPath, `{"instances":[{"prompt":"test"}],"parameters":{"resolution":"720p"}}`)
	require.Equal(t, http.StatusOK, submitResp.Code, submitResp.Body.String())
	var submitted struct {
		Name string `json:"name"`
	}
	require.NoError(t, common.Unmarshal(submitResp.Body.Bytes(), &submitted))
	assert.True(t, strings.HasPrefix(submitted.Name, "projects/demo/locations/global/"))

	fetchPath := "/v1/projects/demo/locations/global/publishers/google/models/veo-3.1-fast-generate-preview:fetchPredictOperation"
	firstResp := performRequest(t, handler, http.MethodPost, fetchPath, `{"operationName":"`+submitted.Name+`"}`)
	var first vertexOperation
	require.NoError(t, common.Unmarshal(firstResp.Body.Bytes(), &first))
	assert.False(t, first.Done)
	completedResp := performRequest(t, handler, http.MethodPost, fetchPath, `{"operationName":"`+submitted.Name+`"}`)
	var completed vertexOperation
	require.NoError(t, common.Unmarshal(completedResp.Body.Bytes(), &completed))
	require.True(t, completed.Done)
	require.Len(t, completed.Response.Videos, 1)
	decoded, err := base64.StdEncoding.DecodeString(completed.Response.Videos[0].BytesBase64Encoded)
	require.NoError(t, err)
	_, err = mp4.Probe(bytes.NewReader(decoded))
	require.NoError(t, err)
}

type seedanceTaskResponse struct {
	Status     string `json:"status"`
	Resolution string `json:"resolution"`
	Duration   int    `json:"duration"`
	Content    struct {
		VideoURL string `json:"video_url"`
	} `json:"content"`
	Usage struct {
		TotalTokens int `json:"total_tokens"`
	} `json:"usage"`
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type geminiOperation struct {
	Done     bool `json:"done"`
	Response struct {
		GenerateVideoResponse struct {
			GeneratedVideos []struct {
				Video struct {
					URI string `json:"uri"`
				} `json:"video"`
			} `json:"generatedVideos"`
		} `json:"generateVideoResponse"`
	} `json:"response"`
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

type vertexOperation struct {
	Done     bool `json:"done"`
	Response struct {
		Videos []struct {
			BytesBase64Encoded string `json:"bytesBase64Encoded"`
		} `json:"videos"`
	} `json:"response"`
}

type openRouterVideoResponse struct {
	ID         string `json:"id"`
	Status     string `json:"status"`
	PollingURL string `json:"polling_url"`
	Output     struct {
		VideoURL string `json:"video_url"`
	} `json:"output"`
	Usage struct {
		VideoTokens int `json:"video_tokens"`
		TotalTokens int `json:"total_tokens"`
	} `json:"usage"`
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func fetchSeedance(t *testing.T, handler http.Handler, taskID string) seedanceTaskResponse {
	t.Helper()
	resp := performRequest(t, handler, http.MethodGet, "/api/v3/contents/generations/tasks/"+taskID, "")
	require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
	var out seedanceTaskResponse
	require.NoError(t, common.Unmarshal(resp.Body.Bytes(), &out))
	return out
}

func fetchGeminiVeo(t *testing.T, handler http.Handler, operationName string) geminiOperation {
	t.Helper()
	resp := performRequest(t, handler, http.MethodGet, "/v1beta/"+operationName, "")
	require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
	var out geminiOperation
	require.NoError(t, common.Unmarshal(resp.Body.Bytes(), &out))
	return out
}

func fetchOpenRouterVeo(t *testing.T, handler http.Handler, taskID string) openRouterVideoResponse {
	t.Helper()
	resp := performRequest(t, handler, http.MethodGet, "/v1/videos/"+taskID, "")
	require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
	var out openRouterVideoResponse
	require.NoError(t, common.Unmarshal(resp.Body.Bytes(), &out))
	return out
}

func performRequest(t *testing.T, handler http.Handler, method string, target string, body string) *httptest.ResponseRecorder {
	t.Helper()
	if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
		requestURL := target
		if idx := strings.Index(requestURL, "://"); idx >= 0 {
			if slash := strings.Index(requestURL[idx+3:], "/"); slash >= 0 {
				target = requestURL[idx+3+slash:]
			}
		}
	}
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Host = "video-mock:8080"
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	return resp
}

func submitTask(t *testing.T, server *mockServer, body string) taskali.AliVideoResponse {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/services/aigc/video-generation/video-synthesis", strings.NewReader(body))
	req.Host = "ali-video-mock:8080"
	resp := httptest.NewRecorder()
	server.routes().ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
	var submitted taskali.AliVideoResponse
	require.NoError(t, common.Unmarshal(resp.Body.Bytes(), &submitted))
	require.Equal(t, mockTaskPending, submitted.Output.TaskStatus)
	require.NotEmpty(t, submitted.Output.TaskID)
	return submitted
}

func fetchTask(t *testing.T, server *mockServer, taskID string) taskali.AliVideoResponse {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks/"+taskID, nil)
	resp := httptest.NewRecorder()
	server.routes().ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
	var out taskali.AliVideoResponse
	require.NoError(t, common.Unmarshal(resp.Body.Bytes(), &out))
	return out
}

func intValue(v any) int {
	switch typed := v.(type) {
	case int:
		return typed
	case float64:
		return int(typed)
	default:
		return 0
	}
}
