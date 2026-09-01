package dto

import (
	"encoding/json"
)

type SunoSubmitReq struct {
	GptDescriptionPrompt string  `json:"gpt_description_prompt,omitempty"`
	Prompt               string  `json:"prompt,omitempty"`
	Mv                   string  `json:"mv,omitempty"`
	Title                string  `json:"title,omitempty"`
	Tags                 string  `json:"tags,omitempty"`
	ContinueAt           float64 `json:"continue_at,omitempty"`
	TaskID               string  `json:"task_id,omitempty"`
	ContinueClipId       string  `json:"continue_clip_id,omitempty"`
	MakeInstrumental     bool    `json:"make_instrumental"`
}

type SunoDataResponse struct {
	TaskID     string          `json:"task_id" gorm:"type:varchar(50);index"`
	Action     string          `json:"action" gorm:"type:varchar(40);index"` // 任务类型, song, lyrics, description-mode
	Status     string          `json:"status" gorm:"type:varchar(20);index"` // 任务状态, submitted, queueing, processing, success, failed
	FailReason string          `json:"fail_reason"`
	SubmitTime int64           `json:"submit_time" gorm:"index"`
	StartTime  int64           `json:"start_time" gorm:"index"`
	FinishTime int64           `json:"finish_time" gorm:"index"`
	Data       json.RawMessage `json:"data" gorm:"type:json"`
}

type SunoSong struct {
	ID                string       `json:"id"`
	VideoURL          string       `json:"video_url"`
	AudioURL          string       `json:"audio_url"`
	ImageURL          string       `json:"image_url"`
	ImageLargeURL     string       `json:"image_large_url"`
	MajorModelVersion string       `json:"major_model_version"`
	ModelName         string       `json:"model_name"`
	Status            string       `json:"status"`
	Title             string       `json:"title"`
	Text              string       `json:"text"`
	Metadata          SunoMetadata `json:"metadata"`
}

type SunoMetadata struct {
	Tags                 string      `json:"tags"`
	Prompt               string      `json:"prompt"`
	GPTDescriptionPrompt interface{} `json:"gpt_description_prompt"`
	AudioPromptID        interface{} `json:"audio_prompt_id"`
	Duration             interface{} `json:"duration"`
	ErrorType            interface{} `json:"error_type"`
	ErrorMessage         interface{} `json:"error_message"`
}

type SunoAPIGenerateRequest struct {
	CustomMode          bool     `json:"customMode"`
	Instrumental        bool     `json:"instrumental"`
	Model               string   `json:"model"`
	CallBackURL         string   `json:"callBackUrl"`
	Prompt              string   `json:"prompt,omitempty"`
	Style               string   `json:"style,omitempty"`
	Title               string   `json:"title,omitempty"`
	PersonaID           string   `json:"personaId,omitempty"`
	PersonaModel        string   `json:"personaModel,omitempty"`
	Duration            *int     `json:"duration,omitempty"`
	NegativeTags        string   `json:"negativeTags,omitempty"`
	VocalGender         string   `json:"vocalGender,omitempty"`
	StyleWeight         *float64 `json:"styleWeight,omitempty"`
	WeirdnessConstraint *float64 `json:"weirdnessConstraint,omitempty"`
	AudioWeight         *float64 `json:"audioWeight,omitempty"`
}

type SunoAPIResponse[T any] struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data T      `json:"data"`
}

type SunoAPISubmitData struct {
	TaskID string `json:"taskId"`
}

type SunoAPISong struct {
	ID             string `json:"id"`
	AudioURL       string `json:"audioUrl"`
	StreamAudioURL string `json:"streamAudioUrl"`
	ImageURL       string `json:"imageUrl"`
	Prompt         string `json:"prompt"`
	ModelName      string `json:"modelName"`
	Title          string `json:"title"`
	Tags           string `json:"tags"`
	// SunoAPI v1 has returned this field as both a JSON string and a number.
	// json.Number accepts either representation while preserving the value.
	CreateTime json.Number `json:"createTime"`
	Duration   float64     `json:"duration"`
}

type SunoAPIGenerationResponse struct {
	TaskID   string        `json:"taskId"`
	SunoData []SunoAPISong `json:"sunoData"`
}

type SunoAPIRecordData struct {
	TaskID       string                    `json:"taskId"`
	Status       string                    `json:"status"`
	ErrorCode    string                    `json:"errorCode"`
	ErrorMessage string                    `json:"errorMessage"`
	Response     SunoAPIGenerationResponse `json:"response"`
}

type SunoLyrics struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Title  string `json:"title"`
	Text   string `json:"text"`
}

type SunoGoAPISubmitReq struct {
	CustomMode bool `json:"custom_mode"`

	Input SunoGoAPISubmitReqInput `json:"input"`

	NotifyHook string `json:"notify_hook,omitempty"`
}

type SunoGoAPISubmitReqInput struct {
	GptDescriptionPrompt string  `json:"gpt_description_prompt"`
	Prompt               string  `json:"prompt"`
	Mv                   string  `json:"mv"`
	Title                string  `json:"title"`
	Tags                 string  `json:"tags"`
	ContinueAt           float64 `json:"continue_at"`
	TaskID               string  `json:"task_id"`
	ContinueClipId       string  `json:"continue_clip_id"`
	MakeInstrumental     bool    `json:"make_instrumental"`
}

type GoAPITaskResponse[T any] struct {
	Code         int    `json:"code"`
	Message      string `json:"message"`
	Data         T      `json:"data"`
	ErrorMessage string `json:"error_message,omitempty"`
}

type GoAPITaskResponseData struct {
	TaskID string `json:"task_id"`
}

type GoAPIFetchResponseData struct {
	TaskID string              `json:"task_id"`
	Status string              `json:"status"`
	Input  string              `json:"input"`
	Clips  map[string]SunoSong `json:"clips"`
}
