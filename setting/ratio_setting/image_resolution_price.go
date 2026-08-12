package ratio_setting

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/types"
)

type ImageResolutionPriceMap map[string]map[string]float64

var imageResolutionPriceMap = types.NewRWMap[string, map[string]float64]()

var imageResolutionTiers = map[string]string{
	"1024x1024": "1k", "848x1264": "1k", "1264x848": "1k",
	"896x1200": "1k", "1200x896": "1k", "928x1152": "1k",
	"1152x928": "1k", "768x1376": "1k", "1376x768": "1k",
	"1584x672": "1k", "1344x576": "1k", "1280x720": "1k",
	"1152x768": "1k", "1024x768": "1k", "768x1024": "1k",
	"768x1152": "1k", "720x1280": "1k",

	"2048x2048": "2k", "1696x2528": "2k", "2528x1696": "2k",
	"1792x2400": "2k", "2400x1792": "2k", "1856x2304": "2k",
	"2304x1856": "2k", "1536x2752": "2k", "2752x1536": "2k",
	"3168x1344": "2k", "2688x1152": "2k", "2048x1152": "2k",
	"2304x1536": "2k", "2048x1536": "2k", "1536x2048": "2k",
	"1536x2304": "2k", "1152x2048": "2k",

	"4096x4096": "4k", "3392x5056": "4k", "5056x3392": "4k",
	"3584x4800": "4k", "4800x3584": "4k", "3712x4608": "4k",
	"4608x3712": "4k", "3072x5504": "4k", "5504x3072": "4k",
	"6336x2688": "4k",
}

func ImageResolutionPrice2JSONString() string {
	return imageResolutionPriceMap.MarshalJSONString()
}

func UpdateImageResolutionPriceByJSONString(jsonStr string) error {
	var parsed ImageResolutionPriceMap
	if err := common.UnmarshalJsonStr(jsonStr, &parsed); err != nil {
		return err
	}
	normalized := make(ImageResolutionPriceMap, len(parsed))
	for modelName, prices := range parsed {
		if strings.TrimSpace(modelName) == "" {
			return fmt.Errorf("image resolution price contains an empty model name")
		}
		normalizedPrices := make(map[string]float64, len(prices))
		for tier, price := range prices {
			normalizedTier := strings.ToLower(strings.TrimSpace(tier))
			if normalizedTier != "1k" && normalizedTier != "2k" && normalizedTier != "4k" {
				return fmt.Errorf("unsupported image resolution tier %q for model %s", tier, modelName)
			}
			if price < 0 || math.IsNaN(price) || math.IsInf(price, 0) {
				return fmt.Errorf("invalid image resolution price for model %s tier %s", modelName, tier)
			}
			normalizedPrices[normalizedTier] = price
		}
		normalized[modelName] = normalizedPrices
	}
	data, err := common.Marshal(normalized)
	if err != nil {
		return err
	}
	return types.LoadFromJsonStringWithCallback(imageResolutionPriceMap, string(data), InvalidateExposedDataCache)
}

func GetImageResolutionPrice(modelName, size string) (float64, string, bool) {
	tier, ok := ResolveImageResolutionTier(size)
	if !ok {
		return 0, "", false
	}
	prices, ok := imageResolutionPriceMap.Get(FormatMatchingModelName(modelName))
	if !ok || prices == nil {
		return 0, tier, false
	}
	price, ok := prices[tier]
	return price, tier, ok
}

func ResolveImageResolutionTier(size string) (string, bool) {
	normalized := strings.ToLower(strings.TrimSpace(size))
	if normalized == "1k" || normalized == "2k" || normalized == "4k" {
		return normalized, true
	}
	normalized = strings.NewReplacer("*", "x", "×", "x").Replace(normalized)
	parts := strings.Split(normalized, "x")
	if len(parts) != 2 {
		return "", false
	}
	width, widthErr := strconv.Atoi(strings.TrimSpace(parts[0]))
	height, heightErr := strconv.Atoi(strings.TrimSpace(parts[1]))
	if widthErr != nil || heightErr != nil || width <= 0 || height <= 0 {
		return "", false
	}
	tier, ok := imageResolutionTiers[strconv.Itoa(width)+"x"+strconv.Itoa(height)]
	return tier, ok
}

func GetImageResolutionPriceCopy() ImageResolutionPriceMap {
	return imageResolutionPriceMap.ReadAll()
}
