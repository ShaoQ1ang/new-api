package capability

import "strings"

var studioImageSizeMatrix = map[string]map[string]string{
	"16:9": {"1k": "1024x576", "2k": "2560x1440", "4k": "5376x3024"},
	"3:2":  {"1k": "1008x672", "2k": "2496x1664", "4k": "4992x3328"},
	"4:3":  {"1k": "1024x768", "2k": "2304x1728", "4k": "4672x3504"},
	"1:1":  {"1k": "1024x1024", "2k": "2048x2048", "4k": "4096x4096"},
	"3:4":  {"1k": "768x1024", "2k": "1728x2304", "4k": "3504x4672"},
	"2:3":  {"1k": "672x1008", "2k": "1664x2496", "4k": "3328x4992"},
	"9:16": {"1k": "576x1024", "2k": "1440x2560", "4k": "3024x5376"},
}

var studioImageSizeTiers = flattenImageSizeMatrix(studioImageSizeMatrix)

func normalizeImageSizeTiers(config *Config) {
	if config == nil || config.Image == nil {
		return
	}
	for modeName, mode := range config.Image.Modes {
		if mode.Output.SizeTiers == nil {
			mode.Output.SizeTiers = make(map[string]string, len(mode.Output.Sizes))
		}
		for _, size := range mode.Output.Sizes {
			size = strings.ToLower(strings.TrimSpace(size))
			if configured, ok := mode.Output.SizeTiers[size]; ok {
				mode.Output.SizeTiers[size] = strings.ToLower(strings.TrimSpace(configured))
				continue
			}
			if tier, ok := studioImageSizeTiers[size]; ok {
				mode.Output.SizeTiers[size] = tier
			}
		}
		config.Image.Modes[modeName] = mode
	}
}

func flattenImageSizeMatrix(matrix map[string]map[string]string) map[string]string {
	result := make(map[string]string)
	for _, tiers := range matrix {
		for tier, size := range tiers {
			result[size] = tier
		}
	}
	return result
}
