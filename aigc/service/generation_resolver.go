package service

import (
	"context"
	"net/http"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/aigc/capability"
	"github.com/QuantumNous/new-api/aigc/dto"
	"github.com/QuantumNous/new-api/aigc/entity"
	"github.com/QuantumNous/new-api/aigc/execution"
)

type GenerationError struct {
	HTTPStatus int
	Code       string
	Message    string
	Retryable  bool
}

func (err *GenerationError) Error() string {
	return err.Message
}

type GenerationResolver struct {
	profiles     ProfileStore
	availability Availability
}

func NewGenerationResolver(profiles ProfileStore, availability Availability) *GenerationResolver {
	return &GenerationResolver{profiles: profiles, availability: availability}
}

func (resolver *GenerationResolver) Resolve(ctx context.Context, group string, request dto.GenerationRequest) (*execution.Spec, error) {
	request.Model = strings.TrimSpace(request.Model)
	request.Type = strings.TrimSpace(request.Type)
	request.Mode = strings.TrimSpace(request.Mode)
	request.Prompt = strings.TrimSpace(request.Prompt)
	if request.Model == "" || request.Type == "" || request.Prompt == "" {
		return nil, generationError(http.StatusBadRequest, "INVALID_REQUEST", "model, type and prompt are required", false)
	}
	profile, err := resolver.profiles.GetProfileByPublicID(ctx, request.Model)
	if err != nil {
		return nil, err
	}
	if profile.Status != entity.ModelStatusPublished {
		return nil, generationError(http.StatusNotFound, "MODEL_NOT_FOUND", "AIGC model is not published", false)
	}
	if profile.ModelType != request.Type {
		return nil, generationError(http.StatusBadRequest, "INVALID_REQUEST", "request type does not match the AIGC model", false)
	}
	groups, err := profileGroups(profile.GroupsJSON)
	if err != nil {
		return nil, err
	}
	if !groupAllowed(groups, group) {
		return nil, generationError(http.StatusForbidden, "MODEL_NOT_AVAILABLE_FOR_GROUP", "AIGC model is not available for this group", false)
	}
	config, err := capability.Parse(capability.ModelType(profile.ModelType), []byte(profile.ConfigJSON))
	if err != nil {
		return nil, generationError(http.StatusServiceUnavailable, "MODEL_CONFIGURATION_INVALID", "AIGC model configuration is invalid", false)
	}
	available, err := resolver.availability.Available(ctx, group, config.UpstreamModelIDs())
	if err != nil {
		return nil, err
	}
	spec := &execution.Spec{
		PublicModelID: profile.PublicModelID, ModelType: profile.ModelType, ConfigVersion: profile.ConfigVersion, Request: request,
	}
	switch capability.ModelType(profile.ModelType) {
	case capability.ModelTypeText:
		err = resolveTextGeneration(spec, config.Text, available)
	case capability.ModelTypeImage:
		err = resolveImageGeneration(spec, config.Image, available)
	case capability.ModelTypeVideo:
		err = resolveVideoGeneration(spec, config.Video, available)
	case capability.ModelTypeMusic:
		err = resolveMusicGeneration(spec, config.Music, available)
	}
	if err != nil {
		return nil, err
	}
	return spec, nil
}

func resolveTextGeneration(spec *execution.Spec, config *capability.TextConfig, available map[string]bool) error {
	if spec.Request.Mode != "" && spec.Request.Mode != "text" {
		return generationError(http.StatusBadRequest, "MODEL_MODE_NOT_SUPPORTED", "text model does not support the requested mode", false)
	}
	if mediaCount(spec.Request.Inputs) != 0 {
		return generationError(http.StatusBadRequest, "INVALID_INPUT_ROLE", "text generation does not accept media inputs", false)
	}
	spec.Mode = "text"
	spec.Request.Mode = spec.Mode
	spec.UpstreamModelID = config.UpstreamModelID
	return requireAvailable(spec.UpstreamModelID, available)
}

func resolveImageGeneration(spec *execution.Spec, config *capability.ImageConfig, available map[string]bool) error {
	mode := spec.Request.Mode
	if mode == "" {
		if len(spec.Request.Inputs.Images) == 0 {
			mode = "text_to_image"
		} else {
			mode = "image_edit"
		}
	}
	configured, ok := config.Modes[mode]
	if !ok {
		return generationError(http.StatusBadRequest, "MODEL_MODE_NOT_SUPPORTED", "image model does not support the requested mode", false)
	}
	if len(spec.Request.Inputs.Videos)+len(spec.Request.Inputs.Audios) != 0 {
		return generationError(http.StatusBadRequest, "INVALID_INPUT_ROLE", "image generation only accepts image inputs", false)
	}
	if mode == "text_to_image" && len(spec.Request.Inputs.Images) != 0 {
		return generationError(http.StatusBadRequest, "INVALID_INPUT_ROLE", "text-to-image does not accept source images", false)
	}
	if mode == "image_edit" {
		if configured.Input == nil || len(spec.Request.Inputs.Images) < configured.Input.Min || len(spec.Request.Inputs.Images) > configured.Input.Max || !mediaHaveRoleAndURL(spec.Request.Inputs.Images, configured.Input.Role) {
			return generationError(http.StatusBadRequest, "INVALID_INPUT_ROLE", "image edit requires valid source_image inputs", false)
		}
	}
	if spec.Request.Output.Size == "" {
		spec.Request.Output.Size = configured.Output.DefaultSize
	}
	if spec.Request.Output.Count == 0 {
		spec.Request.Output.Count = configured.Output.DefaultCount
	}
	if !stringIn(configured.Output.Sizes, spec.Request.Output.Size) || !intIn(configured.Output.Counts, spec.Request.Output.Count) {
		return generationError(http.StatusBadRequest, "OUTPUT_NOT_SUPPORTED", "image output size or count is not supported", false)
	}
	spec.Request.Output.Resolution = configured.Output.SizeTiers[spec.Request.Output.Size]
	spec.Mode = mode
	spec.Request.Mode = mode
	spec.Adapter = config.Adapter
	spec.UpstreamModelID = configured.UpstreamModelID
	return requireAvailable(spec.UpstreamModelID, available)
}

func resolveVideoGeneration(spec *execution.Spec, config *capability.VideoConfig, available map[string]bool) error {
	mode := spec.Request.Mode
	configured, ok := config.Modes[mode]
	if mode == "" || !ok {
		return generationError(http.StatusBadRequest, "MODEL_MODE_NOT_SUPPORTED", "video model does not support the requested mode", false)
	}
	if err := validateVideoRequestInputs(mode, configured, spec.Request.Inputs); err != nil {
		return err
	}
	if err := requireAvailable(configured.UpstreamModelID, available); err != nil {
		return err
	}
	matches := make([]capability.VideoOutputSpec, 0, 1)
	for _, output := range config.OutputSpecs {
		if stringIn(output.Modes, mode) && stringIn(output.Resolutions, spec.Request.Output.Resolution) &&
			stringIn(output.AspectRatios, spec.Request.Output.AspectRatio) && intIn(output.Durations, spec.Request.Output.Duration) {
			if spec.Request.Output.GenerateAudio != nil && *spec.Request.Output.GenerateAudio && !output.GenerateAudio.Supported {
				continue
			}
			matches = append(matches, output)
		}
	}
	if len(matches) != 1 {
		return generationError(http.StatusBadRequest, "OUTPUT_NOT_SUPPORTED", "video output combination is not supported", false)
	}
	selected := matches[0]
	if spec.Request.Output.GenerateAudio == nil {
		value := selected.GenerateAudio.Default
		spec.Request.Output.GenerateAudio = &value
	}
	upstreamID := configured.UpstreamModelID
	if selected.Target != nil && strings.TrimSpace(selected.Target.UpstreamModelID) != "" {
		upstreamID = selected.Target.UpstreamModelID
	}
	if err := requireAvailable(upstreamID, available); err != nil {
		return err
	}
	spec.Mode = mode
	spec.Adapter = config.Adapter
	spec.TaskProtocol = config.TaskProtocol
	spec.OutputSpecID = selected.ID
	spec.UpstreamModelID = upstreamID
	return nil
}

func resolveMusicGeneration(spec *execution.Spec, config *capability.MusicConfig, available map[string]bool) error {
	mode := spec.Request.Mode
	if mode == "" {
		mode = "text_to_music"
	}
	configured, ok := config.Modes[mode]
	if !ok {
		return generationError(http.StatusBadRequest, "MODEL_MODE_NOT_SUPPORTED", "music model does not support the requested mode", false)
	}
	if mediaCount(spec.Request.Inputs) != 0 {
		return generationError(http.StatusBadRequest, "INVALID_INPUT_ROLE", "music generation does not accept media inputs", false)
	}
	parameters := &spec.Request.Parameters
	capabilities := configured.Parameters
	if parameters.Instrumental && !capabilities.Instrumental.Supported {
		return generationError(http.StatusBadRequest, "OUTPUT_NOT_SUPPORTED", "instrumental generation is not supported", false)
	}
	if parameters.Instrumental && parameters.Lyrics != "" {
		return generationError(http.StatusBadRequest, "INVALID_MUSIC_PARAMETERS", "instrumental generation cannot include lyrics", false)
	}
	if err := validateMusicStringParameter("lyrics", parameters.Lyrics, capabilities.ExactLyrics); err != nil {
		return err
	}
	if err := validateMusicStringParameter("style", parameters.Style, capabilities.Style); err != nil {
		return err
	}
	if err := validateMusicStringParameter("title", parameters.Title, capabilities.Title); err != nil {
		return err
	}
	if err := validateMusicStringParameter("negative_tags", parameters.NegativeTags, capabilities.NegativeTags); err != nil {
		return err
	}
	if parameters.PersonaID != "" && !capabilities.Persona.Supported {
		return generationError(http.StatusBadRequest, "OUTPUT_NOT_SUPPORTED", "music persona is not supported", false)
	}
	if parameters.PersonaID == "" && parameters.PersonaModel != "" {
		return generationError(http.StatusBadRequest, "INVALID_MUSIC_PARAMETERS", "persona_model requires persona_id", false)
	}
	if parameters.PersonaID != "" {
		if parameters.PersonaModel == "" {
			parameters.PersonaModel = "style_persona"
		}
		if parameters.PersonaModel != "style_persona" && parameters.PersonaModel != "voice_persona" {
			return generationError(http.StatusBadRequest, "INVALID_MUSIC_PARAMETERS", "persona_model is invalid", false)
		}
		if parameters.PersonaModel == "voice_persona" && !capabilities.Persona.VoicePersonaSupported {
			return generationError(http.StatusBadRequest, "OUTPUT_NOT_SUPPORTED", "voice persona is not supported", false)
		}
	}
	if parameters.Duration != nil {
		if !capabilities.Duration.Supported || *parameters.Duration < capabilities.Duration.Min || *parameters.Duration > capabilities.Duration.Max {
			return generationError(http.StatusBadRequest, "OUTPUT_NOT_SUPPORTED", "music duration is not supported", false)
		}
	}
	if parameters.VocalGender != "" {
		if !capabilities.VocalGender.Supported {
			return generationError(http.StatusBadRequest, "OUTPUT_NOT_SUPPORTED", "vocal gender is not supported", false)
		}
		if parameters.VocalGender != "m" && parameters.VocalGender != "f" {
			return generationError(http.StatusBadRequest, "INVALID_MUSIC_PARAMETERS", "vocal_gender must be m or f", false)
		}
	}
	weightsPresent := parameters.StyleWeight != nil || parameters.WeirdnessConstraint != nil || parameters.AudioWeight != nil
	if weightsPresent && !capabilities.AdvancedWeights.Supported {
		return generationError(http.StatusBadRequest, "OUTPUT_NOT_SUPPORTED", "advanced music weights are not supported", false)
	}
	for name, value := range map[string]*float64{
		"style_weight": parameters.StyleWeight, "weirdness_constraint": parameters.WeirdnessConstraint, "audio_weight": parameters.AudioWeight,
	} {
		if value != nil && (*value < 0 || *value > 1) {
			return generationError(http.StatusBadRequest, "INVALID_MUSIC_PARAMETERS", name+" must be between 0 and 1", false)
		}
	}
	customFields := parameters.Style != "" || parameters.Title != "" || parameters.PersonaID != "" || parameters.Duration != nil ||
		parameters.NegativeTags != "" || parameters.VocalGender != "" || weightsPresent
	spec.MusicCustomMode = parameters.Lyrics != "" || parameters.Instrumental && customFields
	if !parameters.Instrumental && customFields && parameters.Lyrics == "" {
		return generationError(http.StatusBadRequest, "MUSIC_LYRICS_REQUIRED", "custom vocal generation requires exact lyrics", false)
	}
	if spec.MusicCustomMode {
		if parameters.Style == "" {
			parameters.Style = spec.Request.Prompt
		}
		if parameters.Title == "" {
			parameters.Title = defaultMusicTitle(spec.Request.Prompt, capabilities.Title.MaxLength)
		}
	}
	spec.Mode = mode
	spec.Request.Mode = mode
	spec.Adapter = config.Adapter
	spec.TaskProtocol = config.TaskProtocol
	spec.UpstreamModelID = configured.UpstreamModelID
	return requireAvailable(spec.UpstreamModelID, available)
}

func validateMusicStringParameter(name, value string, parameter capability.StringCapability) error {
	if value == "" {
		return nil
	}
	if !parameter.Supported {
		return generationError(http.StatusBadRequest, "OUTPUT_NOT_SUPPORTED", "music "+name+" is not supported", false)
	}
	if parameter.MaxLength > 0 && utf8.RuneCountInString(value) > parameter.MaxLength {
		return generationError(http.StatusBadRequest, "INVALID_MUSIC_PARAMETERS", "music "+name+" is too long", false)
	}
	return nil
}

func defaultMusicTitle(prompt string, maxLength int) string {
	prompt = strings.TrimSpace(prompt)
	if maxLength <= 0 {
		maxLength = 100
	}
	runes := []rune(prompt)
	if len(runes) > maxLength {
		runes = runes[:maxLength]
	}
	return string(runes)
}

func validateVideoRequestInputs(mode string, config capability.VideoModeConfig, inputs dto.GenerationInputs) error {
	invalid := func(message string) error {
		return generationError(http.StatusBadRequest, "INVALID_INPUT_ROLE", message, false)
	}
	switch mode {
	case "text_to_video":
		if len(inputs.Images)+len(inputs.Videos) != 0 || !mediaHaveOnlyRole(inputs.Audios, "driving_audio") || !mediaCountAllowed(len(inputs.Audios), config.Inputs.DrivingAudio) {
			return invalid("text-to-video inputs are invalid")
		}
	case "first_frame":
		if !mediaHaveExactRoles(inputs.Images, []string{"first_frame"}) || len(inputs.Videos) != 0 || !mediaHaveOnlyRole(inputs.Audios, "driving_audio") || !mediaCountAllowed(len(inputs.Audios), config.Inputs.DrivingAudio) {
			return invalid("first-frame video requires one first_frame image")
		}
	case "first_last_frame":
		if !mediaHaveExactRoles(inputs.Images, []string{"first_frame", "last_frame"}) || len(inputs.Videos) != 0 || !mediaHaveOnlyRole(inputs.Audios, "driving_audio") || !mediaCountAllowed(len(inputs.Audios), config.Inputs.DrivingAudio) {
			return invalid("first-last-frame video requires first_frame and last_frame images")
		}
	case "video_extension":
		if !mediaHaveExactRoles(inputs.Videos, []string{"first_clip"}) || !mediaCountAllowed(1, config.Inputs.FirstClip) || !mediaHaveOnlyRole(inputs.Images, "last_frame") || !mediaCountAllowed(len(inputs.Images), config.Inputs.LastFrame) || len(inputs.Audios) != 0 {
			return invalid("video extension inputs are invalid")
		}
	case "video_edit":
		if !mediaHaveExactRoles(inputs.Videos, []string{"source_video"}) || !mediaCountAllowed(1, config.Inputs.SourceVideo) || !mediaHaveOnlyRole(inputs.Images, "general_reference") || !mediaCountAllowed(len(inputs.Images), config.Inputs.GeneralReferenceImage) || len(inputs.Audios) != 0 {
			return invalid("video edit inputs are invalid")
		}
	case "reference":
		if !validateReferenceInputs(config, inputs) {
			return invalid("reference generation inputs are invalid")
		}
	}
	for _, items := range [][]dto.MediaInput{inputs.Images, inputs.Videos, inputs.Audios} {
		for _, input := range items {
			if strings.TrimSpace(input.URL) == "" {
				return invalid("media input URL is required")
			}
		}
	}
	return nil
}

func validateReferenceInputs(config capability.VideoModeConfig, inputs dto.GenerationInputs) bool {
	for _, input := range inputs.Images {
		if input.Role != "general_reference" && !(input.Role == "first_frame" && config.Inputs.FirstFrame != nil) {
			return false
		}
	}
	if !mediaHaveOnlyRole(inputs.Videos, "general_reference") || !mediaHaveOnlyRole(inputs.Audios, "general_reference") {
		return false
	}
	if !mediaCountAllowed(countMediaRole(inputs.Images, "general_reference"), config.Inputs.GeneralReferenceImage) ||
		!mediaCountAllowed(countMediaRole(inputs.Images, "first_frame"), config.Inputs.FirstFrame) ||
		!mediaCountAllowed(len(inputs.Videos), config.Inputs.GeneralReferenceVideo) ||
		!mediaCountAllowed(len(inputs.Audios), config.Inputs.GeneralReferenceAudio) {
		return false
	}
	total := mediaCount(inputs)
	if config.Combination.ExcludeAudioFromTotal {
		total -= len(inputs.Audios)
		if len(inputs.Audios) > len(inputs.Images)+len(inputs.Videos) {
			return false
		}
	}
	if total < config.Combination.MinTotal || config.Combination.MaxTotal > 0 && total > config.Combination.MaxTotal {
		return false
	}
	if config.Combination.Strategy == "allowlist" && !mediaCombinationAllowed(inputs, config.Combination.AllowedCombinations) {
		return false
	}
	return true
}

func mediaCombinationAllowed(inputs dto.GenerationInputs, allowed [][]string) bool {
	actual := make([]string, 0, 3)
	if len(inputs.Images) > 0 {
		actual = append(actual, "image")
	}
	if len(inputs.Videos) > 0 {
		actual = append(actual, "video")
	}
	if len(inputs.Audios) > 0 {
		actual = append(actual, "audio")
	}
	sort.Strings(actual)
	for _, candidate := range allowed {
		copyValue := append([]string(nil), candidate...)
		sort.Strings(copyValue)
		if strings.Join(copyValue, ",") == strings.Join(actual, ",") {
			return true
		}
	}
	return false
}

func mediaCount(inputs dto.GenerationInputs) int {
	return len(inputs.Images) + len(inputs.Videos) + len(inputs.Audios)
}

func mediaCountAllowed(count int, limit *capability.MediaLimit) bool {
	if limit == nil {
		return count == 0
	}
	return count >= limit.Min && count <= limit.Max
}

func mediaHaveRoleAndURL(items []dto.MediaInput, role string) bool {
	for _, item := range items {
		if item.Role != role || strings.TrimSpace(item.URL) == "" {
			return false
		}
	}
	return true
}

func mediaHaveOnlyRole(items []dto.MediaInput, role string) bool {
	for _, item := range items {
		if item.Role != role {
			return false
		}
	}
	return true
}

func mediaHaveExactRoles(items []dto.MediaInput, roles []string) bool {
	if len(items) != len(roles) {
		return false
	}
	actual := make([]string, len(items))
	for i := range items {
		actual[i] = items[i].Role
	}
	expected := append([]string(nil), roles...)
	sort.Strings(actual)
	sort.Strings(expected)
	return strings.Join(actual, ",") == strings.Join(expected, ",")
}

func countMediaRole(items []dto.MediaInput, role string) int {
	count := 0
	for _, item := range items {
		if item.Role == role {
			count++
		}
	}
	return count
}

func requireAvailable(upstreamID string, available map[string]bool) error {
	if !available[strings.TrimSpace(upstreamID)] {
		return generationError(http.StatusServiceUnavailable, "MODEL_CHANNEL_UNAVAILABLE", "AIGC model has no available channel", true)
	}
	return nil
}

func stringIn(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func intIn(values []int, target int) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func generationError(status int, code, message string, retryable bool) *GenerationError {
	return &GenerationError{HTTPStatus: status, Code: code, Message: message, Retryable: retryable}
}
