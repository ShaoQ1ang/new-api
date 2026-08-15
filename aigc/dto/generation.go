package dto

type MediaInput struct {
	Role string `json:"role"`
	URL  string `json:"url"`
}

type GenerationInputs struct {
	Images []MediaInput `json:"images,omitempty"`
	Videos []MediaInput `json:"videos,omitempty"`
	Audios []MediaInput `json:"audios,omitempty"`
}

type GenerationOutput struct {
	Size          string `json:"size,omitempty"`
	Count         int    `json:"count,omitempty"`
	Resolution    string `json:"resolution,omitempty"`
	AspectRatio   string `json:"aspect_ratio,omitempty"`
	Duration      int    `json:"duration,omitempty"`
	GenerateAudio *bool  `json:"generate_audio,omitempty"`
}

type GenerationOptions struct {
	NegativePrompt string `json:"negative_prompt,omitempty"`
	Enhance        bool   `json:"enhance,omitempty"`
	Private        bool   `json:"private,omitempty"`
	AIMark         bool   `json:"ai_mark,omitempty"`
}

type GenerationParameters struct {
	Instrumental bool `json:"instrumental,omitempty"`
}

type GenerationRequest struct {
	RequestID  string               `json:"request_id"`
	Model      string               `json:"model"`
	Type       string               `json:"type"`
	Prompt     string               `json:"prompt"`
	Mode       string               `json:"mode,omitempty"`
	Inputs     GenerationInputs     `json:"inputs,omitempty"`
	Output     GenerationOutput     `json:"output,omitempty"`
	Options    GenerationOptions    `json:"options,omitempty"`
	Parameters GenerationParameters `json:"parameters,omitempty"`
}
