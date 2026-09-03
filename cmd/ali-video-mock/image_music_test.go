package main

import (
	"bytes"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	relaydto "github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestImageModelsGenerateRequestedCountAndPlayablePNG(t *testing.T) {
	for _, model := range []string{"gpt-image-2", "gemini-3-pro-image-preview", "gemini-3.1-flash-image", "qwen-image-2.0"} {
		t.Run(model, func(t *testing.T) {
			server := newMockServerWithConfig(mockConfig{CompleteAfterPoll: 1})
			handler := server.routes()
			response := performRequest(t, handler, http.MethodPost, "/v1/images/generations", `{"model":"`+model+`","prompt":"blue city","size":"1024x576","n":2,"response_format":"url"}`)
			require.Equal(t, http.StatusOK, response.Code, response.Body.String())

			var result relaydto.ImageResponse
			require.NoError(t, common.Unmarshal(response.Body.Bytes(), &result))
			require.Len(t, result.Data, 2)
			for _, item := range result.Data {
				request := httptest.NewRequest(http.MethodGet, item.Url, nil)
				asset := httptest.NewRecorder()
				handler.ServeHTTP(asset, request)
				require.Equal(t, http.StatusOK, asset.Code)
				assert.Equal(t, "image/png", asset.Header().Get("Content-Type"))
				image, err := png.Decode(bytes.NewReader(asset.Body.Bytes()))
				require.NoError(t, err)
				assert.Equal(t, 1024, image.Bounds().Dx())
				assert.Equal(t, 576, image.Bounds().Dy())
			}
		})
	}
}

func TestImageEditRequiresAndPreservesSourceImages(t *testing.T) {
	server := newMockServerWithConfig(mockConfig{CompleteAfterPoll: 1})
	handler := server.routes()

	missing := performRequest(t, handler, http.MethodPost, "/v1/images/edits", `{"model":"gpt-image-2","prompt":"edit it","size":"1024x1024","n":1}`)
	require.Equal(t, http.StatusBadRequest, missing.Code)
	empty := performRequest(t, handler, http.MethodPost, "/v1/images/edits", `{"model":"gpt-image-2","prompt":"edit it","size":"1024x1024","n":1,"images":[]}`)
	require.Equal(t, http.StatusBadRequest, empty.Code)

	body := `{"model":"gpt-image-2","prompt":"edit it","size":"1024x1024","n":1,"images":["data:image/png;base64,AAAA"]}`
	response := performRequest(t, handler, http.MethodPost, "/v1/images/edits", body)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())

	historyResponse := performRequest(t, handler, http.MethodGet, "/api/mock/history", "")
	var history mockHistoryResponse
	require.NoError(t, common.Unmarshal(historyResponse.Body.Bytes(), &history))
	require.Len(t, history.Records, 3)
	assert.Equal(t, "OpenAI Images", history.Records[0].Protocol)
	assert.Equal(t, body, history.Records[0].Body)
}

func TestSunoMusicLifecycleReturnsPlayableTracksAndCapturesProtocol(t *testing.T) {
	server := newMockServerWithConfig(mockConfig{CompleteAfterPoll: 1})
	handler := server.routes()
	submitBody := `{"gpt_description_prompt":"late night jazz","mv":"chirp-v4","make_instrumental":true}`
	submit := performRequest(t, handler, http.MethodPost, "/suno/submit/MUSIC", submitBody)
	require.Equal(t, http.StatusOK, submit.Code, submit.Body.String())
	var submitted relaydto.TaskResponse[string]
	require.NoError(t, common.Unmarshal(submit.Body.Bytes(), &submitted))
	require.Equal(t, "success", submitted.Code)
	require.NotEmpty(t, submitted.Data)

	fetch := performRequest(t, handler, http.MethodPost, "/suno/fetch", `{"ids":["`+submitted.Data+`"]}`)
	require.Equal(t, http.StatusOK, fetch.Code, fetch.Body.String())
	var result relaydto.TaskResponse[[]relaydto.SunoDataResponse]
	require.NoError(t, common.Unmarshal(fetch.Body.Bytes(), &result))
	require.Len(t, result.Data, 1)
	assert.Equal(t, "SUCCESS", result.Data[0].Status)
	var songs []relaydto.SunoSong
	require.NoError(t, common.Unmarshal(result.Data[0].Data, &songs))
	require.Len(t, songs, 2)
	assert.Equal(t, "chirp-v4", songs[0].ModelName)
	assert.Equal(t, "late night jazz", songs[0].Text)

	audioRequest := httptest.NewRequest(http.MethodGet, songs[0].AudioURL, nil)
	audio := httptest.NewRecorder()
	handler.ServeHTTP(audio, audioRequest)
	require.Equal(t, http.StatusOK, audio.Code)
	assert.Equal(t, "audio/wav", audio.Header().Get("Content-Type"))
	assert.True(t, strings.HasPrefix(audio.Body.String(), "RIFF"))
	assert.Contains(t, audio.Body.String()[:16], "WAVE")

	historyResponse := performRequest(t, handler, http.MethodGet, "/api/mock/history", "")
	var history mockHistoryResponse
	require.NoError(t, common.Unmarshal(historyResponse.Body.Bytes(), &history))
	require.Len(t, history.Records, 2)
	assert.Equal(t, "Suno", history.Records[0].Protocol)
	assert.Equal(t, "/suno/fetch", history.Records[0].Path)
	assert.Equal(t, "Suno", history.Records[1].Protocol)
	assert.Equal(t, submitBody, history.Records[1].Body)
}

func TestSunoAPIV1LifecycleReturnsExactlyTwoPlayableTracks(t *testing.T) {
	server := newMockServerWithConfig(mockConfig{CompleteAfterPoll: 2})
	handler := server.routes()
	submitBody := `{"customMode":true,"instrumental":false,"model":"V5_5","callBackUrl":"https://app.test/api/sunoapi/callback","prompt":"city lights","style":"ambient pop","title":"Night Drive","duration":180}`
	submit := performRequest(t, handler, http.MethodPost, "/api/v1/generate", submitBody)
	require.Equal(t, http.StatusOK, submit.Code, submit.Body.String())
	var submitted relaydto.SunoAPIResponse[relaydto.SunoAPISubmitData]
	require.NoError(t, common.Unmarshal(submit.Body.Bytes(), &submitted))
	assert.Equal(t, http.StatusOK, submitted.Code)
	require.NotEmpty(t, submitted.Data.TaskID)

	firstFetch := performRequest(t, handler, http.MethodGet, "/api/v1/generate/record-info?taskId="+submitted.Data.TaskID, "")
	require.Equal(t, http.StatusOK, firstFetch.Code, firstFetch.Body.String())
	var pending relaydto.SunoAPIResponse[relaydto.SunoAPIRecordData]
	require.NoError(t, common.Unmarshal(firstFetch.Body.Bytes(), &pending))
	assert.Equal(t, "PENDING", pending.Data.Status)

	secondFetch := performRequest(t, handler, http.MethodGet, "/api/v1/generate/record-info?taskId="+submitted.Data.TaskID, "")
	require.Equal(t, http.StatusOK, secondFetch.Code, secondFetch.Body.String())
	var completed relaydto.SunoAPIResponse[relaydto.SunoAPIRecordData]
	require.NoError(t, common.Unmarshal(secondFetch.Body.Bytes(), &completed))
	assert.Equal(t, "SUCCESS", completed.Data.Status)
	assert.Equal(t, submitted.Data.TaskID, completed.Data.Response.TaskID)
	require.Len(t, completed.Data.Response.SunoData, 2)
	for index, song := range completed.Data.Response.SunoData {
		assert.Equal(t, submitted.Data.TaskID+"-"+strconv.Itoa(index+1), song.ID)
		assert.Equal(t, "V5_5", song.ModelName)
		assert.Equal(t, "city lights", song.Prompt)
		assert.Equal(t, "ambient pop", song.Tags)
		assert.Equal(t, float64(180), song.Duration)
		assert.NotEmpty(t, song.StreamAudioURL)

		audioRequest := httptest.NewRequest(http.MethodGet, song.AudioURL, nil)
		audio := httptest.NewRecorder()
		handler.ServeHTTP(audio, audioRequest)
		require.Equal(t, http.StatusOK, audio.Code)
		assert.Equal(t, "audio/wav", audio.Header().Get("Content-Type"))
		assert.True(t, strings.HasPrefix(audio.Body.String(), "RIFF"))
	}

	historyResponse := performRequest(t, handler, http.MethodGet, "/api/mock/history", "")
	var history mockHistoryResponse
	require.NoError(t, common.Unmarshal(historyResponse.Body.Bytes(), &history))
	require.Len(t, history.Records, 3)
	for _, record := range history.Records {
		assert.Equal(t, "SunoAPI v1", record.Protocol)
	}
	assert.Equal(t, submitBody, history.Records[2].Body)
}

func TestSunoAPIV1GenerateRejectsInvalidRequests(t *testing.T) {
	tests := []struct {
		name string
		body string
		msg  string
	}{
		{name: "unsupported model", body: `{"model":"V6","callBackUrl":"https://app.test/callback","prompt":"music"}`, msg: "unsupported music model"},
		{name: "missing callback", body: `{"model":"V5_5","prompt":"music"}`, msg: "callBackUrl is required"},
		{name: "missing simple prompt", body: `{"model":"V5_5","callBackUrl":"https://app.test/callback"}`, msg: "prompt is required"},
		{name: "incomplete custom mode", body: `{"customMode":true,"model":"V5_5","callBackUrl":"https://app.test/callback","prompt":"lyrics","title":"Song"}`, msg: "custom mode requires style, title, and vocal prompt"},
		{name: "duration on unsupported model", body: `{"model":"V5","callBackUrl":"https://app.test/callback","prompt":"music","duration":120}`, msg: "duration requires V5_5 and must be between 10 and 360"},
		{name: "duration out of range", body: `{"model":"V5_5","callBackUrl":"https://app.test/callback","prompt":"music","duration":361}`, msg: "duration requires V5_5 and must be between 10 and 360"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := newMockServerWithConfig(mockConfig{CompleteAfterPoll: 1})
			response := performRequest(t, server.routes(), http.MethodPost, "/api/v1/generate", test.body)
			require.Equal(t, http.StatusBadRequest, response.Code, response.Body.String())
			var result relaydto.SunoAPIResponse[any]
			require.NoError(t, common.Unmarshal(response.Body.Bytes(), &result))
			assert.Equal(t, http.StatusBadRequest, result.Code)
			assert.Equal(t, test.msg, result.Msg)
		})
	}
}
