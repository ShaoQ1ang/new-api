package capability

import (
	"fmt"

	"github.com/QuantumNous/new-api/common"
)

func (config Config) Public(modelType ModelType, available map[string]bool) ([]byte, bool, error) {
	var value any
	switch modelType {
	case ModelTypeText:
		if config.Text == nil || !available[config.Text.UpstreamModelID] {
			return nil, false, nil
		}
		value = map[string]any{"max_output_tokens": config.Text.MaxOutputTokens}
	case ModelTypeImage:
		modes := make(map[string]any)
		for name, mode := range config.Image.Modes {
			if !available[mode.UpstreamModelID] {
				continue
			}
			modes[name] = map[string]any{"input": mode.Input, "output": mode.Output}
		}
		if len(modes) == 0 {
			return nil, false, nil
		}
		value = map[string]any{"modes": modes}
	case ModelTypeVideo:
		value = publicVideo(config.Video, available)
		if value == nil {
			return nil, false, nil
		}
	case ModelTypeMusic:
		modes := make(map[string]any)
		for name, mode := range config.Music.Modes {
			if !available[mode.UpstreamModelID] {
				continue
			}
			modes[name] = map[string]any{"parameters": mode.Parameters, "output": mode.Output}
		}
		if len(modes) == 0 {
			return nil, false, nil
		}
		value = map[string]any{"modes": modes}
	default:
		return nil, false, fmt.Errorf("unsupported AIGC model type %q", modelType)
	}
	contents, err := common.Marshal(value)
	if err != nil {
		return nil, false, fmt.Errorf("encode public AIGC capabilities: %w", err)
	}
	return contents, true, nil
}

func publicVideo(config *VideoConfig, available map[string]bool) any {
	availableModes := make(map[string]VideoModeConfig)
	for name, mode := range config.Modes {
		if available[mode.UpstreamModelID] {
			availableModes[name] = mode
		}
	}
	if len(availableModes) == 0 {
		return nil
	}

	publicSpecs := make([]map[string]any, 0, len(config.OutputSpecs))
	coveredModes := make(map[string]bool)
	for _, spec := range config.OutputSpecs {
		if spec.Target != nil && spec.Target.UpstreamModelID != "" && !available[spec.Target.UpstreamModelID] {
			continue
		}
		modes := make([]string, 0, len(spec.Modes))
		for _, mode := range spec.Modes {
			if _, ok := availableModes[mode]; ok {
				modes = append(modes, mode)
				coveredModes[mode] = true
			}
		}
		if len(modes) == 0 {
			continue
		}
		publicSpecs = append(publicSpecs, map[string]any{
			"id":             spec.ID,
			"modes":          modes,
			"resolutions":    spec.Resolutions,
			"aspect_ratios":  spec.AspectRatios,
			"durations":      spec.Durations,
			"generate_audio": spec.GenerateAudio,
		})
	}

	publicModes := make(map[string]any)
	for name, mode := range availableModes {
		if coveredModes[name] {
			publicModes[name] = map[string]any{"inputs": mode.Inputs, "combination": mode.Combination}
		}
	}
	if len(publicModes) == 0 {
		return nil
	}
	return map[string]any{"modes": publicModes, "output_specs": publicSpecs}
}
