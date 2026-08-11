package taskcommon_test

import (
	"testing"

	_ "github.com/QuantumNous/new-api/relay/channel/task/openrouter"
	"github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenRouterBillingRegistryUsesMappedUpstreamModel(t *testing.T) {
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{
		UpstreamModelName: "google/veo-3.1-lite",
		IsModelMapped:     true,
	}}
	req := relaycommon.TaskSubmitReq{
		Model: "my-veo", Seconds: "7", Size: "720p",
		Metadata: map[string]any{"generate_audio": false},
	}

	params, err := taskcommon.ConvertVideoBillingParams(info, req)

	require.NoError(t, err)
	assert.Equal(t, "720p", params.Tier)
	assert.Equal(t, 6, params.DurationSeconds)
	assert.False(t, params.AudioEnabled)
}
