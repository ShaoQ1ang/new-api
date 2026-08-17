package main

import (
	"bytes"
	"image/png"
	"net/http"
	"net/http/httptest"
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
