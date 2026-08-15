package capability

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

var imageSizePattern = regexp.MustCompile(`^[1-9][0-9]{2,4}x[1-9][0-9]{2,4}$`)

var videoModes = map[string]struct{}{
	"text_to_video":    {},
	"first_frame":      {},
	"first_last_frame": {},
	"reference":        {},
	"video_extension":  {},
	"video_edit":       {},
}

func Parse(modelType ModelType, raw []byte) (Config, error) {
	var config Config
	if err := common.Unmarshal(raw, &config); err != nil {
		return Config{}, fmt.Errorf("invalid AIGC model configuration: %w", err)
	}
	if err := config.validate(modelType); err != nil {
		return Config{}, err
	}
	return config, nil
}

func (config Config) UpstreamModelIDs() []string {
	ids := make(map[string]struct{})
	add := func(value string) {
		if value = strings.TrimSpace(value); value != "" {
			ids[value] = struct{}{}
		}
	}
	if config.Text != nil {
		add(config.Text.UpstreamModelID)
	}
	if config.Image != nil {
		for _, mode := range config.Image.Modes {
			add(mode.UpstreamModelID)
		}
	}
	if config.Video != nil {
		for _, mode := range config.Video.Modes {
			add(mode.UpstreamModelID)
		}
		for _, spec := range config.Video.OutputSpecs {
			if spec.Target != nil {
				add(spec.Target.UpstreamModelID)
			}
		}
	}
	if config.Music != nil {
		for _, mode := range config.Music.Modes {
			add(mode.UpstreamModelID)
		}
	}
	result := make([]string, 0, len(ids))
	for id := range ids {
		result = append(result, id)
	}
	sort.Strings(result)
	return result
}

func (config Config) validate(modelType ModelType) error {
	switch modelType {
	case ModelTypeText:
		if config.Text == nil {
			return fmt.Errorf("text configuration is required")
		}
		return validateText(*config.Text)
	case ModelTypeImage:
		if config.Image == nil {
			return fmt.Errorf("image configuration is required")
		}
		return validateImage(*config.Image)
	case ModelTypeVideo:
		if config.Video == nil {
			return fmt.Errorf("video configuration is required")
		}
		return validateVideo(*config.Video)
	case ModelTypeMusic:
		if config.Music == nil {
			return fmt.Errorf("music configuration is required")
		}
		return validateMusic(*config.Music)
	default:
		return fmt.Errorf("unsupported AIGC model type %q", modelType)
	}
}

func validateText(config TextConfig) error {
	if strings.TrimSpace(config.UpstreamModelID) == "" {
		return fmt.Errorf("text upstream model is required")
	}
	if config.MaxOutputTokens < 0 {
		return fmt.Errorf("max output tokens cannot be negative")
	}
	return nil
}

func validateImage(config ImageConfig) error {
	if strings.TrimSpace(config.Adapter) == "" {
		return fmt.Errorf("image adapter is required")
	}
	if len(config.Modes) == 0 {
		return fmt.Errorf("at least one image mode is required")
	}
	for modeName, mode := range config.Modes {
		if modeName != "text_to_image" && modeName != "image_edit" {
			return fmt.Errorf("unsupported image mode %q", modeName)
		}
		if strings.TrimSpace(mode.UpstreamModelID) == "" {
			return fmt.Errorf("image mode %s upstream model is required", modeName)
		}
		if modeName == "text_to_image" && mode.Input != nil {
			return fmt.Errorf("text_to_image cannot declare an image input")
		}
		if modeName == "image_edit" {
			if mode.Input == nil || strings.TrimSpace(mode.Input.Role) != "source_image" || mode.Input.Min < 1 || mode.Input.Max < mode.Input.Min {
				return fmt.Errorf("image_edit requires a valid source_image input")
			}
		}
		if err := validateImageOutput(mode.Output); err != nil {
			return fmt.Errorf("image mode %s: %w", modeName, err)
		}
	}
	return nil
}

func validateImageOutput(output ImageOutputSpec) error {
	if len(output.Sizes) == 0 || len(output.Counts) == 0 {
		return fmt.Errorf("output sizes and counts are required")
	}
	if !containsString(output.Sizes, output.DefaultSize) {
		return fmt.Errorf("default size must be included in output sizes")
	}
	if !containsInt(output.Counts, output.DefaultCount) {
		return fmt.Errorf("default count must be included in output counts")
	}
	for _, size := range output.Sizes {
		if !imageSizePattern.MatchString(strings.TrimSpace(size)) {
			return fmt.Errorf("invalid image size %q", size)
		}
	}
	for _, count := range output.Counts {
		if count < 1 || count > 8 {
			return fmt.Errorf("image output count must be between 1 and 8")
		}
	}
	return nil
}

func validateVideo(config VideoConfig) error {
	if strings.TrimSpace(config.Adapter) == "" || strings.TrimSpace(config.TaskProtocol) == "" {
		return fmt.Errorf("video adapter and task protocol are required")
	}
	if len(config.Modes) == 0 {
		return fmt.Errorf("at least one video mode is required")
	}
	for modeName, mode := range config.Modes {
		if _, ok := videoModes[modeName]; !ok {
			return fmt.Errorf("unsupported video mode %q", modeName)
		}
		if strings.TrimSpace(mode.UpstreamModelID) == "" {
			return fmt.Errorf("video mode %s upstream model is required", modeName)
		}
		if err := validateVideoInputs(mode.Inputs); err != nil {
			return fmt.Errorf("video mode %s: %w", modeName, err)
		}
	}
	coveredModes := make(map[string]bool)
	routes := make(map[string]string)
	for _, spec := range config.OutputSpecs {
		if strings.TrimSpace(spec.ID) == "" || len(spec.Modes) == 0 || len(spec.Resolutions) == 0 || len(spec.AspectRatios) == 0 || len(spec.Durations) == 0 {
			return fmt.Errorf("video output spec requires id, modes, resolutions, aspect ratios and durations")
		}
		if spec.GenerateAudio.Default && !spec.GenerateAudio.Supported {
			return fmt.Errorf("video output spec %s cannot default unsupported audio", spec.ID)
		}
		for _, modeName := range spec.Modes {
			if _, ok := config.Modes[modeName]; !ok {
				return fmt.Errorf("video output spec %s references disabled mode %s", spec.ID, modeName)
			}
			coveredModes[modeName] = true
			for _, resolution := range spec.Resolutions {
				for _, ratio := range spec.AspectRatios {
					for _, duration := range spec.Durations {
						if duration <= 0 {
							return fmt.Errorf("video output spec %s duration must be positive", spec.ID)
						}
						audioValues := []bool{false}
						if spec.GenerateAudio.Supported {
							audioValues = append(audioValues, true)
						}
						for _, audio := range audioValues {
							key := modeName + "|" + resolution + "|" + ratio + "|" + strconv.Itoa(duration) + "|" + strconv.FormatBool(audio)
							if previous, exists := routes[key]; exists {
								return fmt.Errorf("overlapping video route in output specs %s and %s", previous, spec.ID)
							}
							routes[key] = spec.ID
						}
					}
				}
			}
		}
	}
	for modeName := range config.Modes {
		if !coveredModes[modeName] {
			return fmt.Errorf("video mode %s has no output spec", modeName)
		}
	}
	return nil
}

func validateVideoInputs(inputs VideoInputCapabilities) error {
	limits := []*MediaLimit{
		inputs.GeneralReferenceImage,
		inputs.GeneralReferenceVideo,
		inputs.GeneralReferenceAudio,
		inputs.FirstFrame,
		inputs.LastFrame,
		inputs.DrivingAudio,
		inputs.FirstClip,
		inputs.SourceVideo,
	}
	for _, limit := range limits {
		if limit != nil && (limit.Min < 0 || limit.Max <= 0 || limit.Min > limit.Max) {
			return fmt.Errorf("invalid media limit")
		}
	}
	if inputs.MaxDurationWithVideo < 0 {
		return fmt.Errorf("max duration with video cannot be negative")
	}
	return nil
}

func validateMusic(config MusicConfig) error {
	if strings.TrimSpace(config.Adapter) == "" {
		return fmt.Errorf("music adapter is required")
	}
	if len(config.Modes) != 1 {
		return fmt.Errorf("music configuration must contain text_to_music only")
	}
	mode, ok := config.Modes["text_to_music"]
	if !ok || strings.TrimSpace(mode.UpstreamModelID) == "" {
		return fmt.Errorf("text_to_music upstream model is required")
	}
	if mode.Output.MinTracks < 1 || mode.Output.MaxTracks < mode.Output.MinTracks {
		return fmt.Errorf("invalid music output track range")
	}
	return nil
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func containsInt(values []int, target int) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
