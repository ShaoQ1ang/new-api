package openai

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetRequestURLForOpenRouterImageGeneration(t *testing.T) {
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:    constant.ChannelTypeOpenRouter,
			ChannelBaseUrl: "https://openrouter.ai/api",
		},
		RelayMode:      relayconstant.RelayModeImagesGenerations,
		RequestURLPath: "/v1/images/generations",
	}

	requestURL, err := adaptor.GetRequestURL(info)

	require.NoError(t, err)
	assert.Equal(t, "https://openrouter.ai/api/v1/images", requestURL)
}

func TestConvertImageRequestPreservesOpenRouterSeedreamParameters(t *testing.T) {
	seed := int64(0)
	request := dto.ImageRequest{
		Model:           "bytedance-seed/seedream-4.5",
		Prompt:          "a landscape photo",
		Resolution:      "2K",
		AspectRatio:     "16:9",
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
		"resolution":"2K",
		"aspect_ratio":"16:9",
		"seed":0,
		"input_references":[{"type":"image_url","image_url":{"url":"https://example.com/reference.png"}}],
		"provider":{"only":["seed"],"allow_fallbacks":false}
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
