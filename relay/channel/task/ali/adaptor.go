package ali

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"github.com/samber/lo"
)

type AliMediaItem struct {
	Type              string  `json:"type"`
	URL               string  `json:"url"`
	KeepOriginalSound *string `json:"keep_original_sound,omitempty"`
	ReferenceVoice    string  `json:"reference_voice,omitempty"`
}

// AliVideoMedia is the public name used by the Wan2.7 media request contract.
type AliVideoMedia = AliMediaItem

type AliMultiPromptItem struct {
	Index    int    `json:"index"`
	Prompt   string `json:"prompt"`
	Duration int    `json:"duration"`
}

type AliElementItem struct {
	ElementID int `json:"element_id"`
}

type AliVideoRequest struct {
	Model      string              `json:"model"`
	Input      AliVideoInput       `json:"input"`
	Parameters *AliVideoParameters `json:"parameters,omitempty"`
}

type AliVideoInput struct {
	Prompt         string               `json:"prompt,omitempty"`
	ImgURL         string               `json:"img_url,omitempty"`
	FirstFrameURL  string               `json:"first_frame_url,omitempty"`
	LastFrameURL   string               `json:"last_frame_url,omitempty"`
	AudioURL       string               `json:"audio_url,omitempty"`
	NegativePrompt string               `json:"negative_prompt,omitempty"`
	Template       string               `json:"template,omitempty"`
	Media          []AliMediaItem       `json:"media,omitempty"`
	MultiShot      *bool                `json:"multi_shot,omitempty"`
	ShotType       *string              `json:"shot_type,omitempty"`
	MultiPrompt    []AliMultiPromptItem `json:"multi_prompt,omitempty"`
	ElementList    []AliElementItem     `json:"element_list,omitempty"`
}

type AliVideoParameters struct {
	Resolution   string  `json:"resolution,omitempty"`
	Size         string  `json:"size,omitempty"`
	Duration     int     `json:"duration,omitempty"`
	Ratio        *string `json:"ratio,omitempty"`
	Mode         *string `json:"mode,omitempty"`
	AspectRatio  *string `json:"aspect_ratio,omitempty"`
	AudioSetting *string `json:"audio_setting,omitempty"`
	PromptExtend bool    `json:"prompt_extend,omitempty"`
	Watermark    *bool   `json:"watermark,omitempty"`
	Audio        *bool   `json:"audio,omitempty"`
	Seed         int     `json:"seed,omitempty"`
}

type AliVideoResponse struct {
	Output    AliVideoOutput `json:"output"`
	RequestID string         `json:"request_id"`
	Code      string         `json:"code,omitempty"`
	Message   string         `json:"message,omitempty"`
	Usage     *AliUsage      `json:"usage,omitempty"`
}

type AliVideoOutput struct {
	TaskID        string `json:"task_id"`
	TaskStatus    string `json:"task_status"`
	SubmitTime    string `json:"submit_time,omitempty"`
	ScheduledTime string `json:"scheduled_time,omitempty"`
	EndTime       string `json:"end_time,omitempty"`
	OrigPrompt    string `json:"orig_prompt,omitempty"`
	ActualPrompt  string `json:"actual_prompt,omitempty"`
	VideoURL      string `json:"video_url,omitempty"`
	WatermarkURL  string `json:"watermark_video_url,omitempty"`
	Code          string `json:"code,omitempty"`
	Message       string `json:"message,omitempty"`
}

type AliUsage struct {
	Duration            any    `json:"duration,omitempty"`
	InputVideoDuration  any    `json:"input_video_duration,omitempty"`
	OutputVideoDuration any    `json:"output_video_duration,omitempty"`
	VideoCount          any    `json:"video_count,omitempty"`
	SR                  any    `json:"SR,omitempty"`
	Ratio               string `json:"ratio,omitempty"`
	Size                string `json:"size,omitempty"`
	FPS                 any    `json:"fps,omitempty"`
	Audio               any    `json:"audio,omitempty"`
}

func init() {
	taskcommon.RegisterVideoBillingConverter(isWan27Model, convertAliWan27VideoBillingParams)
}

func convertAliWan27VideoBillingParams(req relaycommon.TaskSubmitReq) (*types.VideoBillingParams, error) {
	tier := strings.ToLower(strings.TrimSpace(req.Resolution))
	if tier == "" {
		tier = strings.ToLower(strings.TrimSpace(req.Size))
	}
	if tier != "720p" && tier != "1080p" {
		tier = "1080p"
	}
	duration := req.Duration
	if duration <= 0 && strings.TrimSpace(req.Seconds) != "" {
		duration, _ = strconv.Atoi(strings.TrimSpace(req.Seconds))
	}
	if duration <= 0 {
		duration = 5
	}
	audioEnabled := true
	if req.GenerateAudio != nil {
		audioEnabled = *req.GenerateAudio
	}
	return &types.VideoBillingParams{Tier: tier, DurationSeconds: duration, AudioEnabled: audioEnabled}, nil
}

type AliMetadata struct {
	AudioURL       string `json:"audio_url,omitempty"`
	ImgURL         string `json:"img_url,omitempty"`
	FirstFrameURL  string `json:"first_frame_url,omitempty"`
	LastFrameURL   string `json:"last_frame_url,omitempty"`
	NegativePrompt string `json:"negative_prompt,omitempty"`
	Template       string `json:"template,omitempty"`

	Resolution   *string `json:"resolution,omitempty"`
	Size         *string `json:"size,omitempty"`
	Duration     *int    `json:"duration,omitempty"`
	PromptExtend *bool   `json:"prompt_extend,omitempty"`
	Watermark    *bool   `json:"watermark,omitempty"`
	Audio        *bool   `json:"audio,omitempty"`
	Seed         *int    `json:"seed,omitempty"`
}

type TaskAdaptor struct {
	taskcommon.BaseBilling
	ChannelType int
	apiKey      string
	baseURL     string
}

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	a.ChannelType = info.ChannelType
	a.baseURL = info.ChannelBaseUrl
	a.apiKey = info.ApiKey
}

func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) (taskErr *dto.TaskError) {
	req, err := parseAliTaskRequest(c)
	if err != nil {
		return newAliTaskError(err, "invalid_json", http.StatusBadRequest)
	}
	if info.TaskRelayInfo == nil {
		info.TaskRelayInfo = &relaycommon.TaskRelayInfo{}
	}
	if strings.TrimSpace(req.Model) == "" {
		return newAliTaskError(fmt.Errorf("model field is required"), "missing_model", http.StatusBadRequest)
	}
	if req.InputReference != "" && len(req.Images) == 0 {
		req.Images = []string{req.InputReference}
	}
	action, err := validateAndInferAliAction(req)
	if err != nil {
		return newAliTaskError(err, "invalid_request", http.StatusBadRequest)
	}
	info.Action = action
	c.Set("task_request", req)
	return nil
}

func (a *TaskAdaptor) BuildRequestURL(info *relaycommon.RelayInfo) (string, error) {
	return fmt.Sprintf("%s/api/v1/services/aigc/video-generation/video-synthesis", a.baseURL), nil
}

func (a *TaskAdaptor) BuildRequestHeader(c *gin.Context, req *http.Request, info *relaycommon.RelayInfo) error {
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-DashScope-Async", "enable")
	return nil
}

func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	taskReq, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil, errors.Wrap(err, "get_task_request_failed")
	}

	aliReq, err := a.convertToAliRequest(info, taskReq)
	if err != nil {
		return nil, errors.Wrap(err, "convert_to_ali_request_failed")
	}
	logger.LogJson(c, "ali video request body", aliReq)

	bodyBytes, err := common.Marshal(aliReq)
	if err != nil {
		return nil, errors.Wrap(err, "marshal_ali_request_failed")
	}
	return bytes.NewReader(bodyBytes), nil
}

var (
	size480p  = []string{"832*480", "480*832", "624*624"}
	size720p  = []string{"1280*720", "720*1280", "960*960", "1088*832", "832*1088"}
	size1080p = []string{"1920*1080", "1080*1920", "1440*1440", "1632*1248", "1248*1632"}
)

func sizeToResolution(size string) (string, error) {
	if lo.Contains(size480p, size) {
		return "480P", nil
	}
	if lo.Contains(size720p, size) {
		return "720P", nil
	}
	if lo.Contains(size1080p, size) {
		return "1080P", nil
	}
	return "", fmt.Errorf("invalid size: %s", size)
}

func ProcessAliOtherRatios(aliReq *AliVideoRequest) (map[string]float64, error) {
	otherRatios := make(map[string]float64)
	aliRatios := map[string]map[string]float64{
		"wan2.6-i2v":         {"720P": 1, "1080P": 1 / 0.6},
		"wan2.5-t2v-preview": {"480P": 1, "720P": 2, "1080P": 1 / 0.3},
		"wan2.2-t2v-plus":    {"480P": 1, "1080P": 0.7 / 0.14},
		"wan2.5-i2v-preview": {"480P": 1, "720P": 2, "1080P": 1 / 0.3},
		"wan2.2-i2v-plus":    {"480P": 1, "1080P": 0.7 / 0.14},
		"wan2.2-kf2v-flash":  {"480P": 1, "720P": 2, "1080P": 4.8},
		"wan2.2-i2v-flash":   {"480P": 1, "720P": 2},
		"wan2.2-s2v":         {"480P": 1, "720P": 0.9 / 0.5},
	}
	if aliReq.Parameters == nil {
		return otherRatios, nil
	}
	resolution := aliReq.Parameters.Resolution
	if aliReq.Parameters.Size != "" {
		var err error
		resolution, err = sizeToResolution(aliReq.Parameters.Size)
		if err != nil {
			return nil, err
		}
	} else if resolution != "" && !strings.HasSuffix(strings.ToUpper(resolution), "P") {
		resolution = strings.ToUpper(resolution) + "P"
	}
	if otherRatio, ok := aliRatios[aliReq.Model]; ok {
		if ratio, ok := otherRatio[strings.ToUpper(resolution)]; ok {
			otherRatios[fmt.Sprintf("resolution-%s", strings.ToUpper(resolution))] = ratio
		}
	}
	return otherRatios, nil
}

func (a *TaskAdaptor) convertToAliRequest(info *relaycommon.RelayInfo, req relaycommon.TaskSubmitReq) (*AliVideoRequest, error) {
	upstreamModel := req.Model
	if info != nil && info.ChannelMeta != nil && info.IsModelMapped {
		upstreamModel = info.UpstreamModelName
	}
	switch {
	case isHappyHorseModel(upstreamModel):
		return a.buildHappyHorseRequest(upstreamModel, req), nil
	case isBailianKlingModel(upstreamModel):
		return a.buildKlingRequest(upstreamModel, req)
	case isWan27Model(upstreamModel):
		return a.buildWan27Request(upstreamModel, req)
	}

	aliReq := &AliVideoRequest{
		Model: upstreamModel,
		Input: AliVideoInput{
			Prompt: req.Prompt,
			ImgURL: firstTaskImage(req),
		},
		Parameters: &AliVideoParameters{
			PromptExtend: true,
			Watermark:    lo.ToPtr(false),
		},
	}

	if req.Size != "" {
		if strings.Contains(req.Model, "t2v") && !strings.Contains(req.Size, "*") {
			return nil, fmt.Errorf("invalid size: %s, example: 1920*1080", req.Size)
		}
		if strings.Contains(req.Size, "*") {
			aliReq.Parameters.Size = req.Size
		} else {
			aliReq.Parameters.Resolution = defaultAliResolution(req.Size, "")
		}
	} else {
		if strings.Contains(req.Model, "t2v") {
			if strings.HasPrefix(req.Model, "wan2.5") || strings.HasPrefix(req.Model, "wan2.2") {
				aliReq.Parameters.Size = "1920*1080"
			} else {
				aliReq.Parameters.Size = "1280*720"
			}
		} else {
			switch {
			case strings.HasPrefix(req.Model, "wan2.6"), strings.HasPrefix(req.Model, "wan2.5"), strings.HasPrefix(req.Model, "wan2.2-i2v-plus"):
				aliReq.Parameters.Resolution = "1080P"
			case strings.HasPrefix(req.Model, "wan2.2-i2v-flash"):
				aliReq.Parameters.Resolution = "720P"
			default:
				aliReq.Parameters.Resolution = "720P"
			}
		}
	}

	aliReq.Parameters.Duration = resolveTaskDuration(req, 5)

	if req.Metadata != nil {
		if metadataBytes, err := common.Marshal(req.Metadata); err == nil {
			if err := common.Unmarshal(metadataBytes, aliReq); err != nil {
				return nil, errors.Wrap(err, "unmarshal metadata failed")
			}
		} else {
			return nil, errors.Wrap(err, "marshal metadata failed")
		}
	}

	if aliReq.Model != upstreamModel {
		return nil, errors.New("can't change model with metadata")
	}
	if err := normalizeWan27I2VInput(aliReq, req); err != nil {
		return nil, err
	}

	return aliReq, nil
}

func (a *TaskAdaptor) buildWan27Request(upstreamModel string, req relaycommon.TaskSubmitReq) (*AliVideoRequest, error) {
	defaultDuration := 5
	if isWan27VideoEditModel(upstreamModel) {
		defaultDuration = 0
	}
	aliReq := &AliVideoRequest{
		Model: upstreamModel,
		Input: AliVideoInput{Prompt: req.Prompt},
		Parameters: &AliVideoParameters{
			Resolution: defaultAliResolution(firstNonEmptyString(req.Resolution, req.Size), "1080P"),
			Duration:   resolveTaskDurationAllowZero(req, defaultDuration), PromptExtend: true, Watermark: lo.ToPtr(false),
		},
	}
	if req.Metadata != nil {
		if metadataBytes, err := common.Marshal(req.Metadata); err == nil {
			if err := common.Unmarshal(metadataBytes, aliReq); err != nil {
				return nil, errors.Wrap(err, "unmarshal metadata failed")
			}
		} else {
			return nil, errors.Wrap(err, "marshal metadata failed")
		}
	}
	if aliReq.Model != upstreamModel {
		return nil, errors.New("can't change model with metadata")
	}
	applyWan27FlatMetadata(aliReq, req.Metadata)
	if req.Seed != nil {
		aliReq.Parameters.Seed = int(*req.Seed)
	}
	if ratio := strings.TrimSpace(req.AspectRatio); ratio != "" && !isWan27I2VModel(upstreamModel) {
		aliReq.Parameters.Ratio = lo.ToPtr(ratio)
	}
	if isWan27VideoEditModel(upstreamModel) && req.GenerateAudio != nil {
		setting := "origin"
		if *req.GenerateAudio {
			setting = "auto"
		}
		aliReq.Parameters.AudioSetting = &setting
	}
	if err := normalizeWan27Input(aliReq, req); err != nil {
		return nil, err
	}
	if err := validateWan27Parameters(aliReq); err != nil {
		return nil, err
	}
	return aliReq, nil
}

func validateWan27Parameters(req *AliVideoRequest) error {
	if req == nil || req.Parameters == nil {
		return fmt.Errorf("wan2.7 parameters are required")
	}
	resolution := strings.ToUpper(strings.TrimSpace(req.Parameters.Resolution))
	if resolution != "720P" && resolution != "1080P" {
		return fmt.Errorf("wan2.7 resolution must be 720P or 1080P")
	}
	if req.Parameters.Ratio != nil {
		ratio := strings.TrimSpace(*req.Parameters.Ratio)
		if !lo.Contains([]string{"16:9", "9:16", "1:1", "4:3", "3:4"}, ratio) {
			return fmt.Errorf("unsupported wan2.7 ratio: %s", ratio)
		}
	}
	duration := req.Parameters.Duration
	switch {
	case isWan27VideoEditModel(req.Model):
		if duration != 0 && (duration < 2 || duration > 10) {
			return fmt.Errorf("wan2.7-videoedit duration must be 0 or 2-10")
		}
	default:
		if duration < 2 || duration > 15 {
			return fmt.Errorf("wan2.7 duration must be between 2 and 15 seconds")
		}
	}
	if req.Parameters.Seed < 0 || int64(req.Parameters.Seed) > 2147483647 {
		return fmt.Errorf("wan2.7 seed must be between 0 and 2147483647")
	}
	if req.Parameters.AudioSetting != nil && *req.Parameters.AudioSetting != "auto" && *req.Parameters.AudioSetting != "origin" {
		return fmt.Errorf("wan2.7 audio_setting must be auto or origin")
	}
	return nil
}

func applyWan27FlatMetadata(aliReq *AliVideoRequest, metadata map[string]any) {
	if aliReq == nil || aliReq.Parameters == nil || metadata == nil {
		return
	}
	if value, ok := metadata["negative_prompt"].(string); ok {
		aliReq.Input.NegativePrompt = value
	}
	if value, ok := getBoolMetadata(metadata, "prompt_extend"); ok {
		aliReq.Parameters.PromptExtend = value
	}
	if value, ok := getBoolMetadata(metadata, "watermark"); ok {
		aliReq.Parameters.Watermark = lo.ToPtr(value)
	}
	if value, ok := getIntMetadata(metadata, "seed"); ok {
		aliReq.Parameters.Seed = value
	}
	if value, ok := metadata["audio_setting"].(string); ok && strings.TrimSpace(value) != "" {
		aliReq.Parameters.AudioSetting = lo.ToPtr(strings.TrimSpace(value))
	}
}

func normalizeWan27Input(aliReq *AliVideoRequest, req relaycommon.TaskSubmitReq) error {
	switch {
	case isWan27T2VModel(aliReq.Model):
		if len(req.Audios) > 1 {
			return fmt.Errorf("wan2.7-t2v supports at most one driving audio")
		}
		if len(req.Audios) == 1 {
			aliReq.Input.AudioURL = strings.TrimSpace(req.Audios[0])
		}
		return nil
	case isWan27I2VModel(aliReq.Model):
		if len(aliReq.Input.Media) == 0 {
			aliReq.Input.Media = buildWan27I2VMedia(req)
		}
		if err := validateWan27I2VMedia(aliReq.Input.Media); err != nil {
			return err
		}
	case isWan27R2VModel(aliReq.Model):
		if len(aliReq.Input.Media) == 0 {
			var err error
			aliReq.Input.Media, err = buildWan27R2VMedia(req)
			if err != nil {
				return err
			}
		}
		if err := validateWan27R2VMedia(aliReq.Input.Media, aliReq.Parameters.Duration); err != nil {
			return err
		}
	case isWan27VideoEditModel(aliReq.Model):
		if len(aliReq.Input.Media) == 0 {
			aliReq.Input.Media = buildWan27VideoEditMedia(req)
		}
		if err := validateWan27VideoEditMedia(aliReq.Input.Media); err != nil {
			return err
		}
	}
	aliReq.Input.ImgURL, aliReq.Input.FirstFrameURL, aliReq.Input.LastFrameURL, aliReq.Input.AudioURL = "", "", "", ""
	return nil
}

func buildWan27I2VMedia(req relaycommon.TaskSubmitReq) []AliVideoMedia {
	if len(req.ImageRoles) == 0 && len(req.VideoRoles) == 0 && len(req.AudioRoles) == 0 && len(req.Videos) == 0 && len(req.Audios) == 0 {
		media := []AliVideoMedia{}
		if first := firstTaskImage(req); first != "" {
			media = append(media, AliVideoMedia{Type: "first_frame", URL: first})
		}
		if last := secondTaskImage(req); last != "" {
			media = append(media, AliVideoMedia{Type: "last_frame", URL: last})
		}
		return media
	}
	media := make([]AliVideoMedia, 0, len(req.Images)+len(req.Videos)+len(req.Audios))
	for index, raw := range req.Videos {
		role := roleAt(req.VideoRoles, index, "first_clip")
		media = append(media, AliVideoMedia{Type: role, URL: strings.TrimSpace(raw)})
	}
	for index, raw := range req.Images {
		fallback := "first_frame"
		if index > 0 {
			fallback = "last_frame"
		}
		media = append(media, AliVideoMedia{Type: roleAt(req.ImageRoles, index, fallback), URL: strings.TrimSpace(raw)})
	}
	for index, raw := range req.Audios {
		media = append(media, AliVideoMedia{Type: roleAt(req.AudioRoles, index, "driving_audio"), URL: strings.TrimSpace(raw)})
	}
	if len(media) == 0 {
		if first := firstTaskImage(req); first != "" {
			media = append(media, AliVideoMedia{Type: "first_frame", URL: first})
		}
		if last := secondTaskImage(req); last != "" {
			media = append(media, AliVideoMedia{Type: "last_frame", URL: last})
		}
	}
	return media
}

func validateWan27I2VMedia(media []AliVideoMedia) error {
	counts := map[string]int{}
	for _, item := range media {
		if strings.TrimSpace(item.URL) == "" {
			return fmt.Errorf("wan2.7-i2v media url is required")
		}
		counts[item.Type]++
	}
	valid := counts["first_frame"] == 1 && counts["first_clip"] == 0 && counts["last_frame"] <= 1 && counts["driving_audio"] <= 1 || counts["first_clip"] == 1 && counts["first_frame"] == 0 && counts["last_frame"] <= 1 && counts["driving_audio"] == 0
	if !valid || counts["first_frame"]+counts["first_clip"]+counts["last_frame"]+counts["driving_audio"] != len(media) {
		return fmt.Errorf("wan2.7-i2v requires image or first_clip with a valid media combination")
	}
	return nil
}

func buildWan27R2VMedia(req relaycommon.TaskSubmitReq) ([]AliVideoMedia, error) {
	media := make([]AliVideoMedia, 0, len(req.Images)+len(req.Videos))
	for index, raw := range req.Images {
		role := roleAt(req.ImageRoles, index, "reference_image")
		if role == "general_reference" {
			role = "reference_image"
		}
		media = append(media, AliVideoMedia{Type: role, URL: strings.TrimSpace(raw)})
	}
	for index, raw := range req.Videos {
		role := roleAt(req.VideoRoles, index, "reference_video")
		if role == "general_reference" {
			role = "reference_video"
		}
		media = append(media, AliVideoMedia{Type: role, URL: strings.TrimSpace(raw)})
	}
	referenceIndexes := make([]int, 0, len(media))
	for index := range media {
		if media[index].Type == "reference_image" || media[index].Type == "reference_video" {
			referenceIndexes = append(referenceIndexes, index)
		}
	}
	if len(req.Audios) > len(referenceIndexes) {
		return nil, fmt.Errorf("reference_voice must attach to a reference image or video")
	}
	for index, raw := range req.Audios {
		media[referenceIndexes[index]].ReferenceVoice = strings.TrimSpace(raw)
	}
	return media, nil
}

func validateWan27R2VMedia(media []AliVideoMedia, duration int) error {
	if len(media) < 1 || len(media) > 5 {
		return fmt.Errorf("wan2.7-r2v requires 1-5 visual references")
	}
	firstFrames, videos := 0, 0
	for _, item := range media {
		if strings.TrimSpace(item.URL) == "" {
			return fmt.Errorf("wan2.7-r2v media url is required")
		}
		switch item.Type {
		case "first_frame":
			firstFrames++
		case "reference_video":
			videos++
		case "reference_image":
		default:
			return fmt.Errorf("unsupported wan2.7-r2v media type: %s", item.Type)
		}
	}
	if firstFrames > 1 {
		return fmt.Errorf("wan2.7-r2v supports at most one first_frame")
	}
	if videos > 0 && duration > 10 {
		return fmt.Errorf("wan2.7-r2v duration must be at most 10 seconds with video references")
	}
	return nil
}

func buildWan27VideoEditMedia(req relaycommon.TaskSubmitReq) []AliVideoMedia {
	media := make([]AliVideoMedia, 0, len(req.Videos)+len(req.Images))
	for _, raw := range req.Videos {
		media = append(media, AliVideoMedia{Type: "video", URL: strings.TrimSpace(raw)})
	}
	for _, raw := range req.Images {
		media = append(media, AliVideoMedia{Type: "reference_image", URL: strings.TrimSpace(raw)})
	}
	return media
}

func validateWan27VideoEditMedia(media []AliVideoMedia) error {
	videos, images := 0, 0
	for _, item := range media {
		switch item.Type {
		case "video":
			videos++
		case "reference_image":
			images++
		default:
			return fmt.Errorf("unsupported wan2.7-videoedit media type: %s", item.Type)
		}
	}
	if videos != 1 || images > 4 {
		return fmt.Errorf("wan2.7-videoedit requires one video and at most four reference images")
	}
	return nil
}

func roleAt(roles []string, index int, fallback string) string {
	if index < len(roles) && strings.TrimSpace(roles[index]) != "" {
		return strings.TrimSpace(roles[index])
	}
	return fallback
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func resolveTaskDurationAllowZero(req relaycommon.TaskSubmitReq, fallback int) int {
	if req.Duration > 0 {
		return req.Duration
	}
	if req.Seconds != "" {
		if seconds, err := strconv.Atoi(req.Seconds); err == nil && seconds >= 0 {
			return seconds
		}
	}
	return fallback
}

func firstTaskImage(req relaycommon.TaskSubmitReq) string {
	if image := strings.TrimSpace(req.Image); image != "" {
		return image
	}
	for _, image := range req.Images {
		if image = strings.TrimSpace(image); image != "" {
			return image
		}
	}
	return strings.TrimSpace(req.InputReference)
}

func secondTaskImage(req relaycommon.TaskSubmitReq) string {
	count := 0
	for _, image := range req.Images {
		if image = strings.TrimSpace(image); image != "" {
			count++
			if count == 2 {
				return image
			}
		}
	}
	return ""
}

func normalizeWan27I2VInput(aliReq *AliVideoRequest, req relaycommon.TaskSubmitReq) error {
	if !isWan27I2VModel(aliReq.Model) {
		return nil
	}
	if len(aliReq.Input.Media) == 0 {
		if first := firstTaskImage(req); first != "" {
			aliReq.Input.Media = append(aliReq.Input.Media, AliVideoMedia{Type: "first_frame", URL: first})
		}
		if last := secondTaskImage(req); last != "" {
			aliReq.Input.Media = append(aliReq.Input.Media, AliVideoMedia{Type: "last_frame", URL: last})
		}
	}
	if len(aliReq.Input.Media) == 0 {
		return fmt.Errorf("wan2.7-i2v requires image, images, input_reference, or input.media")
	}
	aliReq.Input.ImgURL = ""
	aliReq.Input.FirstFrameURL = ""
	aliReq.Input.LastFrameURL = ""
	aliReq.Input.AudioURL = ""
	return nil
}

func (a *TaskAdaptor) EstimateBilling(c *gin.Context, info *relaycommon.RelayInfo) map[string]float64 {
	taskReq, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil
	}

	aliReq, err := a.convertToAliRequest(info, taskReq)
	if err != nil || aliReq.Parameters == nil {
		return nil
	}

	otherRatios := map[string]float64{
		"seconds": float64(aliReq.Parameters.Duration),
	}
	ratios, err := ProcessAliOtherRatios(aliReq)
	if err != nil {
		return otherRatios
	}
	for k, v := range ratios {
		otherRatios[k] = v
	}
	return otherRatios
}

func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	return channel.DoTaskApiRequest(a, c, info, requestBody)
}

func (a *TaskAdaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (taskID string, taskData []byte, publicResponse any, taskErr *dto.TaskError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		taskErr = service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
		return
	}
	_ = resp.Body.Close()

	var aliResp AliVideoResponse
	if err := common.Unmarshal(responseBody, &aliResp); err != nil {
		taskErr = service.TaskErrorWrapper(errors.Wrapf(err, "body: %s", responseBody), "unmarshal_response_body_failed", http.StatusInternalServerError)
		return
	}

	if aliResp.Code != "" {
		taskErr = service.TaskErrorWrapper(fmt.Errorf("%s: %s", aliResp.Code, aliResp.Message), "ali_api_error", resp.StatusCode)
		return
	}
	if aliResp.Output.TaskID == "" {
		taskErr = service.TaskErrorWrapper(fmt.Errorf("task_id is empty"), "invalid_response", http.StatusInternalServerError)
		return
	}

	openAIResp := dto.NewOpenAIVideo()
	openAIResp.ID = info.PublicTaskID
	openAIResp.TaskID = info.PublicTaskID
	openAIResp.Model = c.GetString("model")
	if openAIResp.Model == "" && info != nil {
		openAIResp.Model = info.OriginModelName
	}
	openAIResp.Status = convertAliStatus(aliResp.Output.TaskStatus)
	openAIResp.CreatedAt = common.GetTimestamp()
	return aliResp.Output.TaskID, responseBody, openAIResp, nil
}

func (a *TaskAdaptor) FetchTask(baseUrl, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, ok := body["task_id"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid task_id")
	}

	uri := fmt.Sprintf("%s/api/v1/tasks/%s", baseUrl, taskID)
	req, err := http.NewRequest(http.MethodGet, uri, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+key)

	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, fmt.Errorf("new proxy http client failed: %w", err)
	}
	return client.Do(req)
}

func (a *TaskAdaptor) GetModelList() []string {
	return ModelList
}

func (a *TaskAdaptor) GetChannelName() string {
	return ChannelName
}

func (a *TaskAdaptor) ParseTaskResult(respBody []byte) (*relaycommon.TaskInfo, error) {
	var aliResp AliVideoResponse
	if err := common.Unmarshal(respBody, &aliResp); err != nil {
		return nil, errors.Wrap(err, "unmarshal task result failed")
	}

	taskResult := relaycommon.TaskInfo{Code: 0}
	switch aliResp.Output.TaskStatus {
	case "PENDING":
		taskResult.Status = model.TaskStatusQueued
	case "RUNNING":
		taskResult.Status = model.TaskStatusInProgress
	case "SUCCEEDED":
		taskResult.Status = model.TaskStatusSuccess
		taskResult.Url = aliResp.Output.VideoURL
	case "FAILED", "CANCELED", "UNKNOWN":
		taskResult.Status = model.TaskStatusFailure
		if aliResp.Message != "" {
			taskResult.Reason = aliResp.Message
		} else if aliResp.Output.Message != "" {
			taskResult.Reason = fmt.Sprintf("task failed, code: %s , message: %s", aliResp.Output.Code, aliResp.Output.Message)
		} else {
			taskResult.Reason = "task failed"
		}
	default:
		taskResult.Status = model.TaskStatusQueued
	}
	return &taskResult, nil
}

func (a *TaskAdaptor) ConvertToOpenAIVideo(task *model.Task) ([]byte, error) {
	var aliResp AliVideoResponse
	if err := common.Unmarshal(task.Data, &aliResp); err != nil {
		return nil, errors.Wrap(err, "unmarshal ali response failed")
	}

	openAIResp := dto.NewOpenAIVideo()
	openAIResp.ID = task.TaskID
	openAIResp.Status = convertAliStatus(aliResp.Output.TaskStatus)
	openAIResp.Model = task.Properties.OriginModelName
	openAIResp.SetProgressStr(task.Progress)
	openAIResp.CreatedAt = task.CreatedAt
	openAIResp.CompletedAt = task.UpdatedAt
	openAIResp.SetMetadata("url", aliResp.Output.VideoURL)
	if aliResp.Output.WatermarkURL != "" {
		openAIResp.SetMetadata("watermark_url", aliResp.Output.WatermarkURL)
	}

	if aliResp.Code != "" {
		openAIResp.Error = &dto.OpenAIVideoError{Code: aliResp.Code, Message: aliResp.Message}
	} else if aliResp.Output.Code != "" {
		openAIResp.Error = &dto.OpenAIVideoError{Code: aliResp.Output.Code, Message: aliResp.Output.Message}
	}
	return common.Marshal(openAIResp)
}

func convertAliStatus(aliStatus string) string {
	switch aliStatus {
	case "PENDING":
		return dto.VideoStatusQueued
	case "RUNNING":
		return dto.VideoStatusInProgress
	case "SUCCEEDED":
		return dto.VideoStatusCompleted
	case "FAILED", "CANCELED", "UNKNOWN":
		return dto.VideoStatusFailed
	default:
		return dto.VideoStatusUnknown
	}
}

func newAliTaskError(err error, code string, statusCode int) *dto.TaskError {
	return &dto.TaskError{
		Code:       code,
		Message:    err.Error(),
		StatusCode: statusCode,
		LocalError: true,
		Error:      err,
	}
}

func parseAliTaskRequest(c *gin.Context) (relaycommon.TaskSubmitReq, error) {
	var req relaycommon.TaskSubmitReq
	if err := common.UnmarshalBodyReusable(c, &req); err != nil {
		return req, err
	}
	if len(req.Images) == 0 && strings.TrimSpace(req.Image) != "" {
		req.Images = []string{req.Image}
	}
	return req, nil
}

func validateAndInferAliAction(req relaycommon.TaskSubmitReq) (string, error) {
	switch {
	case isHappyHorseModel(req.Model):
		return inferHappyHorseAction(req)
	case isBailianKlingModel(req.Model):
		return inferBailianKlingAction(req)
	default:
		if strings.TrimSpace(req.Prompt) == "" {
			return "", fmt.Errorf("prompt is required")
		}
		if req.HasImage() {
			return constant.TaskActionGenerate, nil
		}
		return constant.TaskActionTextGenerate, nil
	}
}

func inferHappyHorseAction(req relaycommon.TaskSubmitReq) (string, error) {
	modelName := strings.ToLower(strings.TrimSpace(req.Model))
	switch {
	case strings.Contains(modelName, "-t2v"):
		if strings.TrimSpace(req.Prompt) == "" {
			return "", fmt.Errorf("prompt is required")
		}
		return constant.TaskActionTextGenerate, nil
	case strings.Contains(modelName, "-i2v"):
		if len(req.Images) != 1 {
			return "", fmt.Errorf("happyhorse i2v requires exactly 1 first-frame image, got %d", len(req.Images))
		}
		return constant.TaskActionGenerate, nil
	case strings.Contains(modelName, "-r2v"):
		if strings.TrimSpace(req.Prompt) == "" {
			return "", fmt.Errorf("prompt is required")
		}
		if len(req.Images) < 1 || len(req.Images) > 9 {
			return "", fmt.Errorf("happyhorse r2v requires 1-9 reference images, got %d", len(req.Images))
		}
		return constant.TaskActionReferenceGenerate, nil
	case strings.Contains(modelName, "-video-edit"):
		if strings.TrimSpace(req.Prompt) == "" {
			return "", fmt.Errorf("prompt is required")
		}
		if len(req.Videos) != 1 {
			return "", fmt.Errorf("happyhorse video-edit requires exactly 1 video, got %d", len(req.Videos))
		}
		if len(req.Images) > 5 {
			return "", fmt.Errorf("happyhorse video-edit supports at most 5 reference images, got %d", len(req.Images))
		}
		return constant.TaskActionRemix, nil
	default:
		return "", fmt.Errorf("unsupported happyhorse model: %s", req.Model)
	}
}

func inferBailianKlingAction(req relaycommon.TaskSubmitReq) (string, error) {
	var media []AliMediaItem
	var multiShot bool
	var shotType string
	var multiPrompt []AliMultiPromptItem
	if req.Metadata != nil {
		if mediaValue, ok := req.Metadata["media"]; ok {
			if err := decodeMetadataInto(mediaValue, &media); err != nil {
				return "", errors.Wrap(err, "decode kling media failed")
			}
		}
		if value, ok := getBoolMetadata(req.Metadata, "multi_shot"); ok {
			multiShot = value
		}
		if value, ok := getStringMetadata(req.Metadata, "shot_type"); ok {
			shotType = value
		}
		if value, ok := req.Metadata["multi_prompt"]; ok {
			if err := decodeMetadataInto(value, &multiPrompt); err != nil {
				return "", errors.Wrap(err, "decode kling multi_prompt failed")
			}
		}
	}
	if strings.TrimSpace(req.Prompt) == "" && !(multiShot && shotType == "customize" && len(multiPrompt) > 0) {
		return "", fmt.Errorf("prompt is required")
	}
	if len(media) == 0 {
		switch {
		case len(req.Videos) > 0:
			media = append(media, AliMediaItem{Type: "base", URL: req.Videos[0]})
			for _, url := range req.Images {
				media = append(media, AliMediaItem{Type: "refer", URL: url})
			}
		case len(req.Images) == 1:
			media = []AliMediaItem{{Type: "first_frame", URL: req.Images[0]}}
		case len(req.Images) >= 2:
			media = []AliMediaItem{{Type: "first_frame", URL: req.Images[0]}, {Type: "last_frame", URL: req.Images[1]}}
		}
	}

	if len(media) == 0 {
		return constant.TaskActionTextGenerate, nil
	}
	if containsMediaType(media, "base") {
		return constant.TaskActionRemix, nil
	}
	if containsMediaType(media, "feature") || containsMediaType(media, "refer") {
		return constant.TaskActionReferenceGenerate, nil
	}
	if containsMediaType(media, "last_frame") {
		return constant.TaskActionFirstTailGenerate, nil
	}
	return constant.TaskActionGenerate, nil
}

func decodeMetadataInto(value any, out any) error {
	raw, err := common.Marshal(value)
	if err != nil {
		return err
	}
	return common.Unmarshal(raw, out)
}

func getStringMetadata(metadata map[string]any, key string) (string, bool) {
	value, ok := metadata[key]
	if !ok {
		return "", false
	}
	str, ok := value.(string)
	if !ok || strings.TrimSpace(str) == "" {
		return "", false
	}
	return str, true
}

func getBoolMetadata(metadata map[string]any, key string) (bool, bool) {
	value, ok := metadata[key]
	if !ok {
		return false, false
	}
	boolValue, ok := value.(bool)
	return boolValue, ok
}

func getIntMetadata(metadata map[string]any, key string) (int, bool) {
	value, ok := metadata[key]
	if !ok {
		return 0, false
	}
	switch typed := value.(type) {
	case int:
		return typed, true
	case int32:
		return int(typed), true
	case int64:
		return int(typed), true
	case float64:
		return int(typed), true
	default:
		return 0, false
	}
}

func resolveTaskDuration(req relaycommon.TaskSubmitReq, fallback int) int {
	if req.Duration > 0 {
		return req.Duration
	}
	if req.Seconds != "" {
		if seconds, err := strconv.Atoi(req.Seconds); err == nil && seconds > 0 {
			return seconds
		}
	}
	return fallback
}

func isWan27I2VModel(modelName string) bool {
	return strings.Contains(strings.ToLower(strings.TrimSpace(modelName)), "wan2.7-i2v")
}

func isWan27Model(modelName string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(modelName)), "wan2.7-")
}

func isWan27T2VModel(modelName string) bool {
	return strings.Contains(strings.ToLower(strings.TrimSpace(modelName)), "wan2.7-t2v")
}

func isWan27R2VModel(modelName string) bool {
	return strings.Contains(strings.ToLower(strings.TrimSpace(modelName)), "wan2.7-r2v")
}

func isWan27VideoEditModel(modelName string) bool {
	return strings.Contains(strings.ToLower(strings.TrimSpace(modelName)), "wan2.7-videoedit")
}

func defaultAliResolution(size string, fallback string) string {
	if strings.TrimSpace(size) == "" {
		return fallback
	}
	resolution := strings.ToUpper(strings.TrimSpace(size))
	if strings.Contains(resolution, "*") {
		if converted, err := sizeToResolution(resolution); err == nil {
			return converted
		}
		return fallback
	}
	if !strings.HasSuffix(resolution, "P") {
		resolution += "P"
	}
	return resolution
}

func isHappyHorseModel(modelName string) bool {
	normalized := strings.ToLower(strings.TrimSpace(modelName))
	return strings.HasPrefix(normalized, "happyhorse-1.0") || strings.HasPrefix(normalized, "happyhorse-1.1")
}

func isBailianKlingModel(modelName string) bool {
	normalized := strings.ToLower(strings.TrimSpace(modelName))
	return strings.HasPrefix(normalized, "kling/kling-v3-")
}

func modeFromSize(size string) string {
	switch strings.ToUpper(strings.TrimSpace(size)) {
	case "720P":
		return "std"
	default:
		return "pro"
	}
}

func containsMediaType(media []AliMediaItem, target string) bool {
	for _, item := range media {
		if item.Type == target {
			return true
		}
	}
	return false
}

func countMediaType(media []AliMediaItem, target string) int {
	count := 0
	for _, item := range media {
		if item.Type == target {
			count++
		}
	}
	return count
}

func validateBailianKlingPayload(modelName string, req *AliVideoRequest) error {
	isOmni := strings.Contains(strings.ToLower(modelName), "omni")
	media := req.Input.Media

	if req.Input.MultiShot != nil && *req.Input.MultiShot {
		if req.Input.ShotType == nil || strings.TrimSpace(*req.Input.ShotType) == "" {
			return fmt.Errorf("kling multi_shot requires shot_type")
		}
		if *req.Input.ShotType == "customize" && len(req.Input.MultiPrompt) == 0 {
			return fmt.Errorf("kling shot_type customize requires multi_prompt")
		}
	}

	for _, item := range media {
		switch item.Type {
		case "first_frame", "last_frame":
		case "refer", "base", "feature":
			if !isOmni {
				return fmt.Errorf("kling v3 standard model does not support media type %q", item.Type)
			}
		default:
			return fmt.Errorf("unsupported kling media type %q", item.Type)
		}
	}

	hasRefer := countMediaType(media, "refer") > 0
	hasBase := countMediaType(media, "base") > 0
	hasFeature := countMediaType(media, "feature") > 0
	hasFirstFrame := countMediaType(media, "first_frame") > 0
	elementCount := len(req.Input.ElementList)
	if elementCount > 0 {
		switch {
		case hasRefer && (hasBase || hasFeature):
			if countMediaType(media, "refer")+elementCount > 4 {
				return fmt.Errorf("kling refer images and element_list total must be <= 4 when combined with base or feature")
			}
		case hasRefer:
			if countMediaType(media, "refer")+elementCount > 7 {
				return fmt.Errorf("kling refer images and element_list total must be <= 7")
			}
		case hasFirstFrame:
			if elementCount > 3 {
				return fmt.Errorf("kling first-frame generation supports at most 3 elements")
			}
		}
	}
	if req.Parameters != nil && req.Parameters.Audio != nil && *req.Parameters.Audio && (hasBase || hasFeature) {
		return fmt.Errorf("kling audio must be false when media includes base or feature video")
	}

	if !isOmni {
		switch {
		case len(media) == 0:
			return nil
		case countMediaType(media, "first_frame") == 1 && len(media) == 1:
			return nil
		case countMediaType(media, "first_frame") == 1 && countMediaType(media, "last_frame") == 1 && len(media) == 2:
			return nil
		default:
			return fmt.Errorf("kling v3 standard model only supports text, first_frame, or first_frame+last_frame")
		}
	}

	switch {
	case len(media) == 0:
		return nil
	case countMediaType(media, "first_frame") == 1 && len(media) == 1:
		return nil
	case countMediaType(media, "first_frame") == 1 && countMediaType(media, "last_frame") == 1 && len(media) == 2:
		return nil
	case countMediaType(media, "refer") == len(media):
		return nil
	case countMediaType(media, "feature") == 1 && len(media) == 1:
		return nil
	case countMediaType(media, "feature") == 1 && countMediaType(media, "refer")+countMediaType(media, "feature") == len(media):
		return nil
	case countMediaType(media, "feature") == 1 && countMediaType(media, "first_frame") == 1 && len(media) == 2:
		return nil
	case countMediaType(media, "base") == 1 && len(media) == 1:
		return nil
	case countMediaType(media, "base") == 1 && countMediaType(media, "refer")+countMediaType(media, "base") == len(media):
		return nil
	default:
		return fmt.Errorf("unsupported kling omni media combination")
	}
}
