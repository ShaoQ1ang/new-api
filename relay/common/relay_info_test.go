package common

import (
	"testing"

	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func TestRelayInfoGetFinalRequestRelayFormatPrefersExplicitFinal(t *testing.T) {
	info := &RelayInfo{
		RelayFormat:             types.RelayFormatOpenAI,
		RequestConversionChain:  []types.RelayFormat{types.RelayFormatOpenAI, types.RelayFormatClaude},
		FinalRequestRelayFormat: types.RelayFormatOpenAIResponses,
	}

	require.Equal(t, types.RelayFormat(types.RelayFormatOpenAIResponses), info.GetFinalRequestRelayFormat())
}

func TestRelayInfoGetFinalRequestRelayFormatFallsBackToConversionChain(t *testing.T) {
	info := &RelayInfo{
		RelayFormat:            types.RelayFormatOpenAI,
		RequestConversionChain: []types.RelayFormat{types.RelayFormatOpenAI, types.RelayFormatClaude},
	}

	require.Equal(t, types.RelayFormat(types.RelayFormatClaude), info.GetFinalRequestRelayFormat())
}

func TestRelayInfoGetFinalRequestRelayFormatFallsBackToRelayFormat(t *testing.T) {
	info := &RelayInfo{
		RelayFormat: types.RelayFormatGemini,
	}

	require.Equal(t, types.RelayFormat(types.RelayFormatGemini), info.GetFinalRequestRelayFormat())
}

func TestRelayInfoGetFinalRequestRelayFormatNilReceiver(t *testing.T) {
	var info *RelayInfo
	require.Equal(t, types.RelayFormat(""), info.GetFinalRequestRelayFormat())
}

func TestTaskSubmitReqPreservesCanonicalVideoZeroValues(t *testing.T) {
	var req TaskSubmitReq
	require.NoError(t, req.UnmarshalJSON([]byte(`{
		"model":"alibaba/happyhorse-1.1",
		"prompt":"cinematic",
		"resolution":"720p",
		"aspect_ratio":"16:9",
		"generate_audio":false,
		"seed":0,
		"callback_url":"https://example.com/callback",
		"frame_images":[{"type":"image_url","frame_type":"first_frame","image_url":{"url":"https://example.com/first.png"}}],
		"input_references":[{"type":"image_url","image_url":{"url":"https://example.com/reference.png"}}],
		"provider":{"order":["atlas-cloud"]}
	}`)))

	require.Equal(t, "720p", req.Resolution)
	require.Equal(t, "16:9", req.AspectRatio)
	require.NotNil(t, req.GenerateAudio)
	require.False(t, *req.GenerateAudio)
	require.NotNil(t, req.Seed)
	require.Zero(t, *req.Seed)
	require.Len(t, req.FrameImages, 1)
	require.Len(t, req.InputReferences, 1)
	require.Equal(t, "https://example.com/callback", req.CallbackURL)
	require.Equal(t, []any{"atlas-cloud"}, req.Provider["order"])
}
