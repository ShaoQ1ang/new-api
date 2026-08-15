package taskcommon

import (
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAliHappyHorseConverterResolves720PTier(t *testing.T) {
	req := relaycommon.TaskSubmitReq{
		Model:    "happyhorse-1.1-r2v",
		Duration: 5,
		Metadata: map[string]any{
			"resolution": "720P",
		},
	}
	params, err := ConvertVideoBillingParams(nil, req)
	if err != nil {
		t.Fatalf("convert video billing params failed: %v", err)
	}
	if params.Tier != "720p" {
		t.Fatalf("expected 720p, got %s", params.Tier)
	}
}

func TestAliHappyHorseConverterDefaultsTo1080PTier(t *testing.T) {
	req := relaycommon.TaskSubmitReq{
		Model:    "happyhorse-1.1-r2v",
		Duration: 5,
	}
	params, err := ConvertVideoBillingParams(nil, req)
	if err != nil {
		t.Fatalf("convert video billing params failed: %v", err)
	}
	if params.Tier != "1080p" {
		t.Fatalf("expected 1080p, got %s", params.Tier)
	}
}

func TestAliHappyHorseConverterResolves720PFromSize(t *testing.T) {
	req := relaycommon.TaskSubmitReq{
		Model:    "happyhorse-1.1-r2v",
		Duration: 5,
		Size:     "1280*720",
	}
	params, err := ConvertVideoBillingParams(nil, req)
	if err != nil {
		t.Fatalf("convert video billing params failed: %v", err)
	}
	if params.Tier != "720p" {
		t.Fatalf("expected 720p, got %s", params.Tier)
	}
}

func TestAliHappyHorseConverterResolves720PFromResolutionSizeEnum(t *testing.T) {
	req := relaycommon.TaskSubmitReq{
		Model:    "happyhorse-1.1-r2v",
		Duration: 5,
		Size:     "720P",
	}
	params, err := ConvertVideoBillingParams(nil, req)
	if err != nil {
		t.Fatalf("convert video billing params failed: %v", err)
	}
	if params.Tier != "720p" {
		t.Fatalf("expected 720p, got %s", params.Tier)
	}
}

func TestAliHappyHorseConverterUsesExplicitGenerateAudio(t *testing.T) {
	generateAudio := false
	req := relaycommon.TaskSubmitReq{
		Model:         "happyhorse-1.1-r2v",
		Duration:      5,
		GenerateAudio: &generateAudio,
		Metadata: map[string]any{
			"audio": true,
		},
	}

	params, err := ConvertVideoBillingParams(nil, req)

	require.NoError(t, err)
	assert.False(t, params.AudioEnabled)
}

func TestAliHappyHorseConverterSupportsLegacyGenerateAudio(t *testing.T) {
	req := relaycommon.TaskSubmitReq{
		Model:    "happyhorse-1.1-r2v",
		Duration: 5,
		Metadata: map[string]any{
			"generateAudio": false,
		},
	}

	params, err := ConvertVideoBillingParams(nil, req)

	require.NoError(t, err)
	assert.False(t, params.AudioEnabled)
}

func TestAliKlingConverterResolvesStdTo720P(t *testing.T) {
	req := relaycommon.TaskSubmitReq{
		Model:    "kling/kling-v3-video-generation",
		Duration: 5,
		Metadata: map[string]any{
			"mode": "std",
		},
	}
	params, err := ConvertVideoBillingParams(nil, req)
	if err != nil {
		t.Fatalf("convert video billing params failed: %v", err)
	}
	if params.Tier != "720p" {
		t.Fatalf("expected 720p, got %s", params.Tier)
	}
}

func TestAliKlingConverterDefaultsProTo1080P(t *testing.T) {
	req := relaycommon.TaskSubmitReq{
		Model:    "kling/kling-v3-video-generation",
		Duration: 5,
	}
	params, err := ConvertVideoBillingParams(nil, req)
	if err != nil {
		t.Fatalf("convert video billing params failed: %v", err)
	}
	if params.Tier != "1080p" {
		t.Fatalf("expected 1080p, got %s", params.Tier)
	}
}

func TestAliKlingConverterDefaultsToSilentWhenAudioOmitted(t *testing.T) {
	req := relaycommon.TaskSubmitReq{
		Model:    "kling/kling-v3-video-generation",
		Duration: 5,
	}
	params, err := ConvertVideoBillingParams(nil, req)
	if err != nil {
		t.Fatalf("convert video billing params failed: %v", err)
	}
	if params.AudioEnabled {
		t.Fatalf("expected omitted audio to default disabled")
	}
}

func TestAliKlingConverterResolvesSilentFromAudioFalse(t *testing.T) {
	req := relaycommon.TaskSubmitReq{
		Model:    "kling/kling-v3-video-generation",
		Duration: 5,
		Metadata: map[string]any{
			"audio": false,
		},
	}
	params, err := ConvertVideoBillingParams(nil, req)
	if err != nil {
		t.Fatalf("convert video billing params failed: %v", err)
	}
	if params.AudioEnabled {
		t.Fatalf("expected audio disabled")
	}
}

func TestAliKlingConverterUsesMappedUpstreamModel(t *testing.T) {
	info := &relaycommon.RelayInfo{
		OriginModelName: "kling-v3-video-generation",
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "kling/kling-v3-video-generation",
			IsModelMapped:     true,
		},
	}
	req := relaycommon.TaskSubmitReq{
		Model:   "kling-v3-video-generation",
		Seconds: "5",
		Size:    "720p",
		Metadata: map[string]any{
			"audio": false,
		},
	}

	params, err := ConvertVideoBillingParams(info, req)

	require.NoError(t, err)
	assert.Equal(t, "720p", params.Tier)
	assert.Equal(t, 5, params.DurationSeconds)
	assert.False(t, params.AudioEnabled)
}
