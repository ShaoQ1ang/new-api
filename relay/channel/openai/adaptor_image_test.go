package openai

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetRequestURLForOpenRouterImages(t *testing.T) {
	tests := []struct {
		name string
		mode int
		path string
	}{
		{name: "generation", mode: relayconstant.RelayModeImagesGenerations, path: "/v1/images/generations"},
		{name: "edit", mode: relayconstant.RelayModeImagesEdits, path: "/v1/images/edits"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adaptor := &Adaptor{}
			info := &relaycommon.RelayInfo{
				ChannelMeta: &relaycommon.ChannelMeta{
					ChannelType:    constant.ChannelTypeOpenRouter,
					ChannelBaseUrl: "https://openrouter.ai/api",
				},
				RelayMode:      tt.mode,
				RequestURLPath: tt.path,
			}

			requestURL, err := adaptor.GetRequestURL(info)

			require.NoError(t, err)
			assert.Equal(t, "https://openrouter.ai/api/v1/images", requestURL)
		})
	}
}

func TestConvertOpenRouterJSONImageEditRequest(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", nil)
	c.Request.Header.Set("Content-Type", "application/json")
	n := uint(10)
	request := dto.ImageRequest{
		Model:          "google/gemini-3.1-flash-image",
		Prompt:         "remove the bird",
		N:              &n,
		ResponseFormat: "url",
		Images: json.RawMessage(`[
			{"image_url":"https://example.com/first.png"},
			{"image_url":"data:image/png;base64,AAAA"}
		]`),
	}

	converted, err := (&Adaptor{}).ConvertImageRequest(c, &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{ChannelType: constant.ChannelTypeOpenRouter},
		RelayMode:   relayconstant.RelayModeImagesEdits,
	}, request)
	require.NoError(t, err)

	payload, err := json.Marshal(converted)
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"model":"google/gemini-3.1-flash-image",
		"prompt":"remove the bird",
		"n":10,
		"input_references":[
			{"type":"image_url","image_url":{"url":"https://example.com/first.png"}},
			{"type":"image_url","image_url":{"url":"data:image/png;base64,AAAA"}}
		]
	}`, string(payload))
}

func TestConvertOpenRouterImageRequestDoesNotDuplicateInputImageBilling(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", nil)
	c.Request.Header.Set("Content-Type", "application/json")
	request := dto.ImageRequest{
		Images: json.RawMessage(`[
			{"image_url":"https://example.com/first.png"},
			{"image_url":"https://example.com/second.png"}
		]`),
	}
	require.Len(t, request.GetInputImageTiers(), 2)

	converted, err := (&Adaptor{}).ConvertImageRequest(c, &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{ChannelType: constant.ChannelTypeOpenRouter},
		RelayMode:   relayconstant.RelayModeImagesEdits,
	}, request)
	require.NoError(t, err)
	convertedRequest, ok := converted.(dto.ImageRequest)
	require.True(t, ok)

	assert.Empty(t, convertedRequest.Images)
	assert.Len(t, convertedRequest.GetInputImageTiers(), 2)
	assert.Len(t, request.GetInputImageTiers(), 2)
}

func TestConvertOpenRouterImageRequestRejectsInvalidProviderInput(t *testing.T) {
	eleven := uint(11)
	tests := []struct {
		name    string
		request dto.ImageRequest
	}{
		{
			name: "conflicting image representations",
			request: dto.ImageRequest{
				Images:          json.RawMessage(`[{"image_url":"https://example.com/source.png"}]`),
				InputReferences: json.RawMessage(`[{"type":"image_url","image_url":{"url":"https://example.com/source.png"}}]`),
			},
		},
		{name: "count above provider limit", request: dto.ImageRequest{N: &eleven}},
		{name: "invalid images JSON", request: dto.ImageRequest{Images: json.RawMessage(`{"image_url":"bad"}`)}},
		{name: "empty image URL", request: dto.ImageRequest{Images: json.RawMessage(`[{"image_url":""}]`)}},
		{name: "invalid input references JSON", request: dto.ImageRequest{InputReferences: json.RawMessage(`{"type":"image_url"}`)}},
		{name: "empty input references", request: dto.ImageRequest{InputReferences: json.RawMessage(`[]`)}},
		{name: "missing edit image", request: dto.ImageRequest{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", nil)
			c.Request.Header.Set("Content-Type", "application/json")

			_, err := (&Adaptor{}).ConvertImageRequest(c, &relaycommon.RelayInfo{
				ChannelMeta: &relaycommon.ChannelMeta{ChannelType: constant.ChannelTypeOpenRouter},
				RelayMode:   relayconstant.RelayModeImagesEdits,
			}, tt.request)

			var relayErr *types.NewAPIError
			require.ErrorAs(t, err, &relayErr)
			assert.Equal(t, http.StatusBadRequest, relayErr.StatusCode)
			assert.Equal(t, types.ErrorCodeInvalidRequest, relayErr.GetErrorCode())
		})
	}
}

func TestConvertOpenRouterJSONImageEditRequestSupportsSingularImage(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", nil)
	c.Request.Header.Set("Content-Type", "application/json")
	request := dto.ImageRequest{
		Model:  "image-model",
		Prompt: "edit",
		Image:  json.RawMessage(`"https://example.com/source.png"`),
	}

	converted, err := (&Adaptor{}).ConvertImageRequest(c, &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{ChannelType: constant.ChannelTypeOpenRouter},
		RelayMode:   relayconstant.RelayModeImagesEdits,
	}, request)
	require.NoError(t, err)

	payload, err := json.Marshal(converted)
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"model":"image-model",
		"prompt":"edit",
		"input_references":[{"type":"image_url","image_url":{"url":"https://example.com/source.png"}}]
	}`, string(payload))
}

func TestConvertImageRequestPreservesOpenRouterSeedreamParameters(t *testing.T) {
	seed := int64(0)
	n := uint(1)
	request := dto.ImageRequest{
		Model:           "bytedance-seed/seedream-4.5",
		Prompt:          "a landscape photo",
		N:               &n,
		Resolution:      "2K",
		AspectRatio:     "16:9",
		ResponseFormat:  "url",
		Seed:            &seed,
		InputReferences: json.RawMessage(`[{"type":"image_url","image_url":{"url":"https://example.com/reference.png"}}]`),
		Provider:        json.RawMessage(`{"only":["seed"],"allow_fallbacks":false}`),
	}

	converted, err := (&Adaptor{}).ConvertImageRequest(gin.CreateTestContextOnly(nil, gin.New()), &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{ChannelType: constant.ChannelTypeOpenRouter},
		RelayMode:   relayconstant.RelayModeImagesGenerations,
	}, request)
	require.NoError(t, err)

	payload, err := json.Marshal(converted)
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"model":"bytedance-seed/seedream-4.5",
		"prompt":"a landscape photo",
		"n":1,
		"resolution":"2K",
		"aspect_ratio":"16:9",
		"seed":0,
		"input_references":[{"type":"image_url","image_url":{"url":"https://example.com/reference.png"}}],
		"provider":{"only":["seed"],"allow_fallbacks":false}
	}`, string(payload))
}

func TestConvertNonOpenRouterJSONImageEditRequestPreservesExistingContract(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", nil)
	c.Request.Header.Set("Content-Type", "application/json")
	request := dto.ImageRequest{
		Model:          "compatible-image-model",
		Prompt:         "edit",
		ResponseFormat: "url",
		Images:         json.RawMessage(`[{"image_url":"https://example.com/source.png"}]`),
	}

	converted, err := (&Adaptor{}).ConvertImageRequest(c, &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{ChannelType: constant.ChannelTypeOpenAI},
		RelayMode:   relayconstant.RelayModeImagesEdits,
	}, request)
	require.NoError(t, err)

	payload, err := json.Marshal(converted)
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"model":"compatible-image-model",
		"prompt":"edit",
		"response_format":"url",
		"images":[{"image_url":"https://example.com/source.png"}]
	}`, string(payload))
}

func TestConvertImageRequest_StripsResponseFormatForOpenAIGPTImage2(t *testing.T) {
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType: constant.ChannelTypeOpenAI,
		},
	}
	request := dto.ImageRequest{
		Model:          "gpt-image-2",
		ResponseFormat: "b64_json",
	}

	gotAny, err := adaptor.ConvertImageRequest(gin.CreateTestContextOnly(nil, gin.New()), info, request)
	if err != nil {
		t.Fatalf("ConvertImageRequest returned error: %v", err)
	}

	got, ok := gotAny.(dto.ImageRequest)
	if !ok {
		t.Fatalf("ConvertImageRequest returned %T, want dto.ImageRequest", gotAny)
	}
	if got.ResponseFormat != "" {
		t.Fatalf("response_format = %q, want empty", got.ResponseFormat)
	}
}

func TestConvertImageRequest_KeepsResponseFormatForOtherImageModels(t *testing.T) {
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType: constant.ChannelTypeOpenAI,
		},
	}
	request := dto.ImageRequest{
		Model:          "dall-e-3",
		ResponseFormat: "b64_json",
	}

	gotAny, err := adaptor.ConvertImageRequest(gin.CreateTestContextOnly(nil, gin.New()), info, request)
	if err != nil {
		t.Fatalf("ConvertImageRequest returned error: %v", err)
	}

	got, ok := gotAny.(dto.ImageRequest)
	if !ok {
		t.Fatalf("ConvertImageRequest returned %T, want dto.ImageRequest", gotAny)
	}
	if got.ResponseFormat != "b64_json" {
		t.Fatalf("response_format = %q, want %q", got.ResponseFormat, "b64_json")
	}
}
