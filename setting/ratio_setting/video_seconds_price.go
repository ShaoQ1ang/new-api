package ratio_setting

import (
	"strings"

	"github.com/QuantumNous/new-api/types"
)

type VideoSecondsPriceMap map[string]map[string]map[string]float64

var videoSecondsPriceMap = types.NewRWMap[string, map[string]map[string]float64]()

var defaultVideoSecondsPrice = VideoSecondsPriceMap{
	"alibaba/happyhorse-1.0": {
		"720p":  {"default": 0.0988},
		"1080p": {"default": 0.1694},
	},
	"alibaba/happyhorse-1.1": {
		"720p":  {"default": 0.0988},
		"1080p": {"default": 0.1278},
	},
	"kwaivgi/kling-v3.0-std": {
		"720p": {"default": 0.126, "silent": 0.084},
	},
	"kwaivgi/kling-v3.0-pro": {
		"720p": {"default": 0.168, "silent": 0.112},
	},
	"kwaivgi/kling-video-o1": {
		"720p": {"default": 0.112, "silent": 0.112},
	},
	"minimax/hailuo-3": {
		"2k": {"default": 0.13, "silent": 0.13, "reference_image": 0.04},
	},
	"minimax/hailuo-2.3": {
		"1080p": {"default": 0.0817, "silent": 0.0817},
	},
	"wan2.7-t2v": {
		"720p": {"default": 0.1, "silent": 0.1}, "1080p": {"default": 0.1, "silent": 0.1},
	},
	"wan2.7-i2v": {
		"720p": {"default": 0.1, "silent": 0.1}, "1080p": {"default": 0.1, "silent": 0.1},
	},
	"wan2.7-r2v": {
		"720p": {"default": 0.1, "silent": 0.1}, "1080p": {"default": 0.1, "silent": 0.1},
	},
	"wan2.7-videoedit": {
		"720p": {"default": 0.1, "silent": 0.1}, "1080p": {"default": 0.1, "silent": 0.1},
	},
	"alibaba/wan-2.7": {
		"720p":  {"default": 0.1, "silent": 0.1},
		"1080p": {"default": 0.1, "silent": 0.1},
	},
}

func GetVideoSecondsExtraPrice(modelName, tier, key string) (float64, bool) {
	modelMap, ok := videoSecondsPriceMap.Get(FormatMatchingModelName(modelName))
	if !ok || modelMap == nil {
		return 0, false
	}
	tierMap, ok := modelMap[strings.ToLower(strings.TrimSpace(tier))]
	if !ok || tierMap == nil {
		return 0, false
	}
	price, ok := tierMap[strings.ToLower(strings.TrimSpace(key))]
	return price, ok
}

func initVideoSecondsPrice() {
	videoSecondsPriceMap.AddAll(defaultVideoSecondsPrice)
}

func VideoSecondsPrice2JSONString() string {
	return videoSecondsPriceMap.MarshalJSONString()
}

func UpdateVideoSecondsPriceByJSONString(jsonStr string) error {
	return types.LoadFromJsonStringWithCallback(videoSecondsPriceMap, jsonStr, InvalidateExposedDataCache)
}

func GetVideoSecondsPrice(modelName, tier string, audioEnabled bool) (float64, bool) {
	if !audioEnabled {
		if price, ok := getVideoSecondsPriceByKey(modelName, tier, "silent"); ok {
			return price, true
		}
	}
	return getVideoSecondsPriceByKey(modelName, tier, "default")
}

// GetVideoSecondsPriceByKey resolves a direct per-second price variant and
// falls back to the normal audio-sensitive price when the variant is absent.
func GetVideoSecondsPriceByKey(modelName, tier, key string, audioEnabled bool) (float64, bool) {
	if strings.TrimSpace(key) != "" {
		if price, ok := getVideoSecondsPriceByKey(modelName, tier, key); ok {
			return price, true
		}
	}
	return GetVideoSecondsPrice(modelName, tier, audioEnabled)
}

func getVideoSecondsPriceByKey(modelName, tier, key string) (float64, bool) {
	modelMap, ok := videoSecondsPriceMap.Get(FormatMatchingModelName(modelName))
	if !ok || modelMap == nil {
		return 0, false
	}
	tierMap, ok := modelMap[strings.ToLower(strings.TrimSpace(tier))]
	if !ok || tierMap == nil {
		return 0, false
	}
	price, ok := tierMap[strings.ToLower(strings.TrimSpace(key))]
	return price, ok
}

func GetVideoSecondsPriceCopy() VideoSecondsPriceMap {
	return videoSecondsPriceMap.ReadAll()
}
