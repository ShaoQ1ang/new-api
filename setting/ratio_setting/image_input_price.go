package ratio_setting

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/types"
)

const imageInputFreeCountKey = "free_count"

type ImageInputPriceMap map[string]map[string]float64

var imageInputPriceMap = types.NewRWMap[string, map[string]float64]()

func ImageInputPrice2JSONString() string {
	return imageInputPriceMap.MarshalJSONString()
}

func UpdateImageInputPriceByJSONString(jsonStr string) error {
	var parsed ImageInputPriceMap
	if err := common.UnmarshalJsonStr(jsonStr, &parsed); err != nil {
		return err
	}

	normalized := make(ImageInputPriceMap, len(parsed))
	for modelName, config := range parsed {
		modelName = FormatMatchingModelName(modelName)
		if modelName == "" || config == nil {
			return fmt.Errorf("image input price model and config must not be empty")
		}
		normalizedConfig := make(map[string]float64, len(config))
		for rawTier, value := range config {
			tier := strings.ToLower(strings.TrimSpace(rawTier))
			if tier == imageInputFreeCountKey {
				if value < 0 || value > 128 || value != math.Trunc(value) {
					return fmt.Errorf("image input free_count for %s must be an integer between 0 and 128", modelName)
				}
				normalizedConfig[tier] = value
				continue
			}
			tier = NormalizeImageInputTier(tier)
			if tier == "" {
				return fmt.Errorf("unsupported image input price tier %q for %s", rawTier, modelName)
			}
			if value < 0 || math.IsNaN(value) || math.IsInf(value, 0) {
				return fmt.Errorf("image input price for %s tier %s must be a finite non-negative number", modelName, tier)
			}
			normalizedConfig[tier] = value
		}
		normalized[modelName] = normalizedConfig
	}

	normalizedJSON, err := common.Marshal(normalized)
	if err != nil {
		return err
	}
	return types.LoadFromJsonStringWithCallback(imageInputPriceMap, string(normalizedJSON), InvalidateExposedDataCache)
}

func NormalizeImageInputTier(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "default" {
		return normalized
	}
	switch normalized {
	case "1k", "2k", "4k", "8k":
		return normalized
	}

	normalized = strings.ReplaceAll(normalized, "×", "x")
	normalized = strings.ReplaceAll(normalized, "*", "x")
	parts := strings.Split(normalized, "x")
	if len(parts) != 2 {
		return ""
	}
	width, errWidth := strconv.Atoi(strings.TrimSpace(parts[0]))
	height, errHeight := strconv.Atoi(strings.TrimSpace(parts[1]))
	if errWidth != nil || errHeight != nil || width <= 0 || height <= 0 {
		return ""
	}
	maxDimension := max(width, height)
	switch {
	case maxDimension <= 1024:
		return "1k"
	case maxDimension <= 2048:
		return "2k"
	case maxDimension <= 4096:
		return "4k"
	default:
		return "8k"
	}
}

func GetImageInputPriceConfig(modelName, upstreamModelName string) (map[string]float64, bool) {
	for _, candidate := range []string{modelName, upstreamModelName} {
		candidate = FormatMatchingModelName(candidate)
		if candidate == "" {
			continue
		}
		if config, ok := imageInputPriceMap.Get(candidate); ok && config != nil {
			copyConfig := make(map[string]float64, len(config))
			for key, value := range config {
				copyConfig[key] = value
			}
			return copyConfig, true
		}
	}
	return nil, false
}

func CalculateImageInputCost(modelName, upstreamModelName string, tiers []string) (float64, map[string]int, int, bool) {
	config, ok := GetImageInputPriceConfig(modelName, upstreamModelName)
	if !ok || len(tiers) == 0 {
		return 0, nil, 0, ok
	}
	freeCount := min(int(config[imageInputFreeCountKey]), len(tiers))
	counts := make(map[string]int)
	total := 0.0
	for _, rawTier := range tiers[freeCount:] {
		tier := NormalizeImageInputTier(rawTier)
		if tier == "" {
			tier = "default"
		}
		unitPrice, found := config[tier]
		if !found {
			unitPrice, found = config["default"]
			tier = "default"
		}
		if !found {
			continue
		}
		counts[tier]++
		total += unitPrice
	}
	return total, counts, freeCount, true
}

func GetImageInputPriceCopy() ImageInputPriceMap {
	return imageInputPriceMap.ReadAll()
}
