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
	IdempotencyKey string               `json:"idempotency_key"`
	Model          string               `json:"model"`
	Type           string               `json:"type"`
	Prompt         string               `json:"prompt"`
	Mode           string               `json:"mode,omitempty"`
	Inputs         GenerationInputs     `json:"inputs,omitempty"`
	Output         GenerationOutput     `json:"output,omitempty"`
	Options        GenerationOptions    `json:"options,omitempty"`
	Parameters     GenerationParameters `json:"parameters,omitempty"`
}

type GenerationOutputItem struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Title       string `json:"title,omitempty"`
	Text        string `json:"text,omitempty"`
	URL         string `json:"url,omitempty"`
	B64JSON     string `json:"b64_json,omitempty"`
	ContentType string `json:"content_type,omitempty"`
	Width       int    `json:"width,omitempty"`
	Height      int    `json:"height,omitempty"`
	Duration    int    `json:"duration,omitempty"`
	PosterURL   string `json:"poster_url,omitempty"`
}

type GenerationUsage struct {
	Quota int `json:"quota,omitempty"`
}

type GenerationFailure struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
}

type GenerationResponse struct {
	ID             string                 `json:"id"`
	IdempotencyKey string                 `json:"idempotency_key"`
	Status         string                 `json:"status"`
	Progress       int                    `json:"progress"`
	Model          string                 `json:"model"`
	Type           string                 `json:"type"`
	CreatedAt      int64                  `json:"created_at"`
	Outputs        []GenerationOutputItem `json:"outputs"`
	Usage          *GenerationUsage       `json:"usage,omitempty"`
	Error          *GenerationFailure     `json:"error,omitempty"`
}
