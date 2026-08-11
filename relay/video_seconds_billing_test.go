package relay

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRelayTaskVideoSecondsComputesQuotaBySecond(t *testing.T) {
	gin.SetMode(gin.TestMode)

	saved := map[string]string{}
	require.NoError(t, config.GlobalConfig.SaveToDB(func(key, value string) error {
		saved[key] = value
		return nil
	}))
	t.Cleanup(func() {
		require.NoError(t, config.GlobalConfig.LoadFromDB(saved))
	})

	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"billing_setting.billing_mode": `{"happyhorse-1.1-r2v":"video_seconds"}`,
	}))
	require.NoError(t, ratio_setting.UpdateVideoSecondsPriceByJSONString(`{
		"happyhorse-1.1-r2v": {
			"720p": {"default": 0.9}
		}
	}`))

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set("group", "default")
	ctx.Set("task_request", relaycommon.TaskSubmitReq{
		Model:    "happyhorse-1.1-r2v",
		Duration: 5,
		Metadata: map[string]any{
			"resolution": "720P",
		},
	})

	info := &relaycommon.RelayInfo{
		OriginModelName: "happyhorse-1.1-r2v",
		UserGroup:       "default",
		UsingGroup:      "default",
	}
	info.PriceData.GroupRatioInfo.GroupRatio = 1

	require.NoError(t, applyVideoSecondsBilling(ctx, info))
	expectedQuota := int(0.9 * 5 * common.QuotaPerUnit * 1.0)
	require.Equal(t, expectedQuota, info.PriceData.Quota)
	require.Equal(t, "720p", info.PriceData.VideoSecondsTier)
}

func TestRelayTaskVideoSecondsFailsWhenTierPriceMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	saved := map[string]string{}
	require.NoError(t, config.GlobalConfig.SaveToDB(func(key, value string) error {
		saved[key] = value
		return nil
	}))
	t.Cleanup(func() {
		require.NoError(t, config.GlobalConfig.LoadFromDB(saved))
	})

	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"billing_setting.billing_mode": `{"happyhorse-1.1-r2v":"video_seconds"}`,
	}))
	require.NoError(t, ratio_setting.UpdateVideoSecondsPriceByJSONString(`{
		"happyhorse-1.1-r2v": {
			"1080p": {"default": 1.2}
		}
	}`))

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set("group", "default")
	ctx.Set("task_request", relaycommon.TaskSubmitReq{
		Model:    "happyhorse-1.1-r2v",
		Duration: 5,
		Metadata: map[string]any{
			"resolution": "720P",
		},
	})

	info := &relaycommon.RelayInfo{
		OriginModelName: "happyhorse-1.1-r2v",
		UserGroup:       "default",
		UsingGroup:      "default",
	}
	info.PriceData.GroupRatioInfo.GroupRatio = 1

	err := applyVideoSecondsBilling(ctx, info)
	require.Error(t, err)
	require.Equal(t, billing_setting.BillingModeVideoSeconds, billing_setting.GetBillingMode(info.OriginModelName))
}

func TestRelayTaskVideoSecondsUsesMappedUpstreamBillingConverter(t *testing.T) {
	gin.SetMode(gin.TestMode)

	saved := map[string]string{}
	require.NoError(t, config.GlobalConfig.SaveToDB(func(key, value string) error {
		saved[key] = value
		return nil
	}))
	t.Cleanup(func() {
		require.NoError(t, config.GlobalConfig.LoadFromDB(saved))
	})

	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"billing_setting.billing_mode": `{"kling-v3-video-generation":"video_seconds"}`,
	}))
	require.NoError(t, ratio_setting.UpdateVideoSecondsPriceByJSONString(`{
		"kling-v3-video-generation": {
			"720p": {"default": 0.9, "silent": 0.6}
		}
	}`))

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set("group", "default")
	ctx.Set("task_request", relaycommon.TaskSubmitReq{
		Model:   "kling-v3-video-generation",
		Seconds: "5",
		Size:    "720p",
		Metadata: map[string]any{
			"aspect_ratio": "16:9",
			"audio":        false,
			"watermark":    false,
		},
	})

	info := &relaycommon.RelayInfo{
		OriginModelName: "kling-v3-video-generation",
		UserGroup:       "default",
		UsingGroup:      "default",
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "kling/kling-v3-video-generation",
			IsModelMapped:     true,
		},
	}
	info.PriceData.GroupRatioInfo.GroupRatio = 1

	require.NoError(t, applyVideoSecondsBilling(ctx, info))
	assert.Equal(t, int(0.6*5*common.QuotaPerUnit), info.PriceData.Quota)
	assert.Equal(t, "720p", info.PriceData.VideoSecondsTier)
	assert.Equal(t, 5, info.PriceData.VideoDurationSeconds)
	require.NotNil(t, info.PriceData.VideoAudioEnabled)
	assert.False(t, *info.PriceData.VideoAudioEnabled)
}
