package capability

type ModelType string

const (
	ModelTypeText  ModelType = "text"
	ModelTypeImage ModelType = "image"
	ModelTypeVideo ModelType = "video"
	ModelTypeMusic ModelType = "music"
)

type Config struct {
	Text  *TextConfig  `json:"text,omitempty"`
	Image *ImageConfig `json:"image,omitempty"`
	Video *VideoConfig `json:"video,omitempty"`
	Music *MusicConfig `json:"music,omitempty"`
}

type TextConfig struct {
	UpstreamModelID string `json:"upstream_model_id"`
	MaxOutputTokens int    `json:"max_output_tokens,omitempty"`
}

type MediaLimit struct {
	Min int `json:"min"`
	Max int `json:"max"`
}

type ImageInputSpec struct {
	Role   string   `json:"role"`
	Min    int      `json:"min"`
	Max    int      `json:"max"`
	Accept []string `json:"accept,omitempty"`
}

type ImageOutputSpec struct {
	Sizes        []string `json:"sizes"`
	Counts       []int    `json:"counts"`
	DefaultSize  string   `json:"default_size"`
	DefaultCount int      `json:"default_count"`
}

type ImageModeConfig struct {
	UpstreamModelID string          `json:"upstream_model_id"`
	Input           *ImageInputSpec `json:"input,omitempty"`
	Output          ImageOutputSpec `json:"output"`
}

type ImageConfig struct {
	Adapter string                     `json:"adapter"`
	Modes   map[string]ImageModeConfig `json:"modes"`
}

type VideoInputCapabilities struct {
	GeneralReferenceImage *MediaLimit `json:"general_reference_image,omitempty"`
	GeneralReferenceVideo *MediaLimit `json:"general_reference_video,omitempty"`
	GeneralReferenceAudio *MediaLimit `json:"general_reference_audio,omitempty"`
	FirstFrame            *MediaLimit `json:"first_frame,omitempty"`
	LastFrame             *MediaLimit `json:"last_frame,omitempty"`
	DrivingAudio          *MediaLimit `json:"driving_audio,omitempty"`
	FirstClip             *MediaLimit `json:"first_clip,omitempty"`
	SourceVideo           *MediaLimit `json:"source_video,omitempty"`
	MaxDurationWithVideo  int         `json:"max_duration_with_video,omitempty"`
}

type VideoCombinationPolicy struct {
	Strategy              string     `json:"strategy,omitempty"`
	AllowedCombinations   [][]string `json:"allowed_combinations,omitempty"`
	MinTotal              int        `json:"min_total,omitempty"`
	MaxTotal              int        `json:"max_total,omitempty"`
	ExcludeAudioFromTotal bool       `json:"exclude_audio_from_total,omitempty"`
}

type VideoModeConfig struct {
	UpstreamModelID string                 `json:"upstream_model_id"`
	Inputs          VideoInputCapabilities `json:"inputs,omitempty"`
	Combination     VideoCombinationPolicy `json:"combination,omitempty"`
}

type VideoAudioSpec struct {
	Supported    bool `json:"supported"`
	Default      bool `json:"default"`
	Configurable bool `json:"configurable,omitempty"`
}

type VideoExecutionTarget struct {
	UpstreamModelID string `json:"upstream_model_id,omitempty"`
}

type VideoOutputSpec struct {
	ID            string                `json:"id"`
	Modes         []string              `json:"modes"`
	Resolutions   []string              `json:"resolutions"`
	AspectRatios  []string              `json:"aspect_ratios"`
	Durations     []int                 `json:"durations"`
	GenerateAudio VideoAudioSpec        `json:"generate_audio"`
	Target        *VideoExecutionTarget `json:"target,omitempty"`
}

type VideoConfig struct {
	Adapter      string                     `json:"adapter"`
	TaskProtocol string                     `json:"task_protocol"`
	Modes        map[string]VideoModeConfig `json:"modes"`
	OutputSpecs  []VideoOutputSpec          `json:"output_specs"`
}

type BooleanCapability struct {
	Supported    bool `json:"supported"`
	Default      bool `json:"default"`
	Configurable bool `json:"configurable,omitempty"`
}

type StringCapability struct {
	Supported bool `json:"supported"`
	MaxLength int  `json:"max_length,omitempty"`
}

type IntegerRangeCapability struct {
	Supported bool `json:"supported"`
	Min       int  `json:"min,omitempty"`
	Max       int  `json:"max,omitempty"`
}

type PersonaCapability struct {
	Supported             bool `json:"supported"`
	VoicePersonaSupported bool `json:"voice_persona_supported,omitempty"`
}

type MusicParameters struct {
	Instrumental    BooleanCapability      `json:"instrumental"`
	ExactLyrics     StringCapability       `json:"exact_lyrics,omitempty"`
	Style           StringCapability       `json:"style,omitempty"`
	Title           StringCapability       `json:"title,omitempty"`
	Persona         PersonaCapability      `json:"persona,omitempty"`
	Duration        IntegerRangeCapability `json:"duration,omitempty"`
	NegativeTags    StringCapability       `json:"negative_tags,omitempty"`
	VocalGender     BooleanCapability      `json:"vocal_gender,omitempty"`
	AdvancedWeights BooleanCapability      `json:"advanced_weights,omitempty"`
}

type MusicOutputSpec struct {
	MinTracks int `json:"min_tracks"`
	MaxTracks int `json:"max_tracks"`
}

type MusicModeConfig struct {
	UpstreamModelID string          `json:"upstream_model_id"`
	Parameters      MusicParameters `json:"parameters"`
	Output          MusicOutputSpec `json:"output"`
}

type MusicConfig struct {
	Adapter      string                     `json:"adapter"`
	TaskProtocol string                     `json:"task_protocol,omitempty"`
	Modes        map[string]MusicModeConfig `json:"modes"`
}
