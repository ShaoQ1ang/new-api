package relay

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/stretchr/testify/assert"
)

func TestShouldPassThroughImageRequestRequiresOpenRouterConversion(t *testing.T) {
	settings := model_setting.GetGlobalSettings()
	originalGlobalPassThrough := settings.PassThroughRequestEnabled
	t.Cleanup(func() {
		settings.PassThroughRequestEnabled = originalGlobalPassThrough
	})
	assert.False(t, shouldPassThroughImageRequest(nil))

	tests := []struct {
		name               string
		globalPassThrough  bool
		channelType        int
		channelPassThrough bool
		want               bool
	}{
		{
			name:        "disabled passthrough stays disabled",
			channelType: constant.ChannelTypeOpenAI, want: false,
		},
		{
			name: "global passthrough cannot bypass OpenRouter edit conversion", globalPassThrough: true,
			channelType: constant.ChannelTypeOpenRouter, want: false,
		},
		{
			name: "channel passthrough cannot bypass OpenRouter edit conversion", channelPassThrough: true,
			channelType: constant.ChannelTypeOpenRouter, want: false,
		},
		{
			name: "other channel keeps global passthrough", globalPassThrough: true,
			channelType: constant.ChannelTypeOpenAI, want: true,
		},
		{
			name: "other channel keeps channel passthrough", channelPassThrough: true,
			channelType: constant.ChannelTypeOpenAI, want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings.PassThroughRequestEnabled = tt.globalPassThrough
			info := &relaycommon.RelayInfo{
				RelayMode: relayconstant.RelayModeImagesEdits,
				ChannelMeta: &relaycommon.ChannelMeta{
					ChannelType:    tt.channelType,
					ChannelSetting: dto.ChannelSettings{PassThroughBodyEnabled: tt.channelPassThrough},
				},
			}

			assert.Equal(t, tt.want, shouldPassThroughImageRequest(info))
		})
	}
}
