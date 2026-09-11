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
	if normalized == "" || normalized == "auto" {
		return "2k", true
	}
	normalized = strings.NewReplacer("*", "x", "×", "x").Replace(normalized)
	if tier, ok := studioImageResolutionTiers[normalized]; ok {
		return tier, true
	}
	parts := strings.Split(normalized, "x")
	if len(parts) != 2 {
		return "2k", true
	}
	width, widthErr := strconv.Atoi(strings.TrimSpace(parts[0]))
	height, heightErr := strconv.Atoi(strings.TrimSpace(parts[1]))
	if widthErr != nil || heightErr != nil || width <= 0 || height <= 0 {
		return "2k", true
	}
	maxEdge := width
	if height > maxEdge {
		maxEdge = height
	}
	if maxEdge <= 1024 {
		return "1k", true
	}
	if maxEdge <= 2048 {
		return "2k", true
	}
	return "4k", true
}

var studioImageResolutionTiers = map[string]string{
	"1024x576": "1k", "1008x672": "1k", "1024x768": "1k", "1024x1024": "1k",
	"768x1024": "1k", "672x1008": "1k", "576x1024": "1k",
	"1456x624":  "1k",
	"2560x1440": "2k", "2496x1664": "2k", "2304x1728": "2k", "2048x2048": "2k",
	"1728x2304": "2k", "1664x2496": "2k", "1440x2560": "2k",
	"3024x1296": "2k",
	"5376x3024": "4k", "4992x3328": "4k", "4672x3504": "4k", "4096x4096": "4k",
	"3504x4672": "4k", "3328x4992": "4k", "3024x5376": "4k",
	"6160x2640": "4k",
}

func GetImageResolutionPriceCopy() ImageResolutionPriceMap {
	return imageResolutionPriceMap.ReadAll()
}
