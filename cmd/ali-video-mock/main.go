package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	taskali "github.com/QuantumNous/new-api/relay/channel/task/ali"
	taskdoubao "github.com/QuantumNous/new-api/relay/channel/task/doubao"
)

const (
	defaultListenAddr = ":8080"
	mockTaskPending   = "PENDING"
	mockTaskRunning   = "RUNNING"
	mockTaskSuccess   = "SUCCEEDED"
	mockTaskFailed    = "FAILED"
)

type mockServer struct {
	mu            sync.Mutex
	nextID        int64
	nextHistoryID int64
	tasks         map[string]*mockTask
	history       []mockRequestRecord
	config        mockConfig
	rng           *rand.Rand
	videoBytes    []byte
}

type mockConfig struct {
	FailRate          float64
	CompleteAfterPoll int
	PublicBaseURL     string
}

type mockTask struct {
	ID            string
	Provider      string
	OperationName string
	Model         string
	Family        string
	Duration      int
	Resolution    string
	SR            int
	Audio         bool
	CreatedAt     time.Time
	ScheduledAt   time.Time
	CompletedAt   time.Time
	PollCount     int
	ShouldFail    bool
	FailReason    string
	VideoURL      string
	WatermarkURL  string
	Ratio         string
	Seed          int
}

type seedanceRequest struct {
	Model   string `json:"model"`
	Content []struct {
		Type     string `json:"type"`
		Text     string `json:"text"`
		ImageURL any    `json:"image_url"`
		VideoURL any    `json:"video_url"`
		AudioURL any    `json:"audio_url"`
	} `json:"content"`
	GenerateAudio *bool  `json:"generate_audio,omitempty"`
	Resolution    string `json:"resolution,omitempty"`
	Ratio         string `json:"ratio,omitempty"`
	Duration      int    `json:"duration,omitempty"`
	Seed          int    `json:"seed,omitempty"`
}

type veoRequest struct {
	Instances []struct {
		Prompt string `json:"prompt"`
		Image  any    `json:"image,omitempty"`
	} `json:"instances"`
	Parameters struct {
		DurationSeconds int    `json:"durationSeconds,omitempty"`
		AspectRatio     string `json:"aspectRatio,omitempty"`
		Resolution      string `json:"resolution,omitempty"`
		Seed            *int   `json:"seed,omitempty"`
		GenerateAudio   *bool  `json:"generateAudio,omitempty"`
	} `json:"parameters"`
}

type openRouterVideoRequest struct {
	Model           string                     `json:"model"`
	Prompt          string                     `json:"prompt"`
	Duration        *int                       `json:"duration,omitempty"`
	Resolution      string                     `json:"resolution,omitempty"`
	AspectRatio     string                     `json:"aspect_ratio,omitempty"`
	GenerateAudio   *bool                      `json:"generate_audio,omitempty"`
	FrameImages     []openRouterFrameImage     `json:"frame_images,omitempty"`
	InputReferences []openRouterInputReference `json:"input_references,omitempty"`
	Seed            any                        `json:"seed,omitempty"`
}

type openRouterFrameImage struct {
	FrameType string `json:"frame_type"`
	Type      string `json:"type"`
	ImageURL  struct {
		URL string `json:"url"`
	} `json:"image_url"`
}

type openRouterInputReference struct {
	Type     string `json:"type"`
	URL      string `json:"url"`
	ImageURL struct {
		URL string `json:"url"`
	} `json:"image_url"`
	VideoURL struct {
		URL string `json:"url"`
	} `json:"video_url"`
	AudioURL struct {
		URL string `json:"url"`
	} `json:"audio_url"`
}

func (r openRouterInputReference) mediaTypeAndURL() (string, string) {
	switch r.Type {
	case "image_url":
		return "image", r.ImageURL.URL
	case "video_url":
		return "video", r.VideoURL.URL
	case "audio_url":
		return "audio", r.AudioURL.URL
	default:
		return r.Type, r.URL
	}
}

type requestSummary struct {
	Model      string
	Family     string
	Prompt     string
	MediaCount int
	Duration   int
	Resolution string
	SR         int
	Audio      bool
}

func newMockServer() *mockServer {
	return newMockServerWithConfig(loadMockConfig())
}

func newMockServerWithConfig(cfg mockConfig) *mockServer {
	if cfg.CompleteAfterPoll <= 0 {
		cfg.CompleteAfterPoll = 2
	}
	videoBytes, err := decodeEmbeddedMockVideo()
	if err != nil {
		panic(fmt.Sprintf("decode embedded mock video: %v", err))
	}
	return &mockServer{
		tasks:      make(map[string]*mockTask),
		history:    make([]mockRequestRecord, 0, mockHistoryLimit),
		config:     cfg,
		rng:        rand.New(rand.NewSource(time.Now().UnixNano())),
		videoBytes: videoBytes,
	}
}

func main() {
	addr := strings.TrimSpace(getenv("ALI_VIDEO_MOCK_LISTEN", defaultListenAddr))
	srv := newMockServer()
	log.Printf("ali-video-mock listening on %s fail_rate=%.2f complete_after_poll=%d public_base_url=%q video_bytes=%d",
		addr, srv.config.FailRate, srv.config.CompleteAfterPoll, srv.config.PublicBaseURL, len(srv.videoBytes))
	if err := http.ListenAndServe(addr, srv.routes()); err != nil {
		log.Fatalf("ali-video-mock listen failed: %v", err)
	}
}

func (s *mockServer) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/history", s.handleHistoryPage)
	mux.HandleFunc("/api/mock/history", s.handleRequestHistory)
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/api/v1/services/aigc/video-generation/video-synthesis", s.handleSubmit)
	mux.HandleFunc("/api/v1/tasks/", s.handleFetchTask)
	mux.HandleFunc("/api/v3/contents/generations/tasks", s.handleSeedanceSubmit)
	mux.HandleFunc("/api/v3/contents/generations/tasks/", s.handleSeedanceFetch)
	mux.HandleFunc("/v1/videos", s.handleOpenRouterVideoSubmit)
	mux.HandleFunc("/v1/videos/models", s.handleOpenRouterVideoModels)
	mux.HandleFunc("/v1/videos/", s.handleOpenRouterVideoFetch)
	mux.HandleFunc("/mock-assets/videos/", s.handleMockVideo)
	mux.HandleFunc("/", s.handleVeo)
	return s.captureRequests(mux)
}

func (s *mockServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	log.Printf("healthz check remote=%s", r.RemoteAddr)
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":             true,
		"video_bytes":    len(s.videoBytes),
		"model_families": []string{"wan", "happyhorse", "kling", "minimax", "seedance", "veo"},
		"providers":      []string{"alibaba", "doubao", "gemini", "vertex", "openrouter"},
	})
}

func (s *mockServer) handleOpenRouterVideoSubmit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeOpenRouterError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	var req openRouterVideoRequest
	if err := common.DecodeJson(r.Body, &req); err != nil {
		writeOpenRouterError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	family, err := validateOpenRouterVideoRequest(req)
	if err != nil {
		writeOpenRouterError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	duration := 5
	if family == "veo" {
		duration = 8
	} else if req.Model == "minimax/hailuo-2.3" {
		duration = 6
	}
	if req.Duration != nil {
		duration = *req.Duration
	}
	resolution := strings.ToLower(strings.TrimSpace(req.Resolution))
	if resolution == "" {
		switch req.Model {
		case "alibaba/happyhorse-1.0", "alibaba/happyhorse-1.1":
			resolution = "1080p"
		case "minimax/hailuo-3":
			resolution = "2k"
		case "minimax/hailuo-2.3":
			resolution = "1080p"
		default:
			resolution = "720p"
		}
	}
	ratio := strings.TrimSpace(req.AspectRatio)
	if ratio == "" {
		ratio = "16:9"
	}
	audio := req.GenerateAudio != nil && *req.GenerateAudio
	taskID := strings.Replace(s.nextTaskID(), "mock-task", "mock-openrouter-"+family, 1)
	now := time.Now()
	task := &mockTask{
		ID: taskID, Provider: "openrouter", Model: req.Model, Family: family,
		Duration: duration, Resolution: resolution, SR: resolutionValue(resolution), Audio: audio, Ratio: ratio,
		CreatedAt: now, ScheduledAt: now.Add(800 * time.Millisecond), CompletedAt: now.Add(1600 * time.Millisecond),
		ShouldFail: s.shouldFail(), FailReason: "mock upstream random failure",
		VideoURL: s.buildAssetURL(r, "/mock-assets/videos/"+taskID+".mp4"),
	}
	s.storeTask(task)
	writeJSON(w, http.StatusOK, map[string]any{
		"id": task.ID, "status": "queued", "created_at": task.CreatedAt.Unix(),
		"polling_url": s.buildAssetURL(r, "/v1/videos/"+task.ID),
		"model":       task.Model,
	})
}

func (s *mockServer) handleOpenRouterVideoModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeOpenRouterError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": []map[string]any{
		{"id": "alibaba/happyhorse-1.1", "supported_durations": integerRange(3, 15), "supported_resolutions": []string{"720p", "1080p"}, "pricing_skus": map[string]string{"duration_seconds_720p": "0.0988", "duration_seconds_1080p": "0.1278"}},
		{"id": "kwaivgi/kling-v3.0-std", "supported_durations": integerRange(3, 15), "supported_resolutions": []string{"720p"}, "pricing_skus": map[string]string{"duration_seconds": "0.084", "duration_seconds_with_audio": "0.126"}},
		{"id": "kwaivgi/kling-v3.0-pro", "supported_durations": integerRange(3, 15), "supported_resolutions": []string{"720p"}, "pricing_skus": map[string]string{"duration_seconds": "0.112", "duration_seconds_with_audio": "0.168"}},
		{"id": "kwaivgi/kling-video-o1", "supported_durations": []int{5, 10}, "supported_resolutions": []string{"720p"}, "pricing_skus": map[string]string{"duration_seconds": "0.112"}},
		{"id": "minimax/hailuo-3", "supported_durations": integerRange(5, 15), "supported_resolutions": []string{"2K"}, "pricing_skus": map[string]string{"duration_seconds": "0.13", "reference_images": "0.04"}},
		{"id": "minimax/hailuo-2.3", "supported_durations": []int{6, 10}, "supported_resolutions": []string{"1080p"}, "pricing_skus": map[string]string{"duration_seconds": "0.0817"}},
	}})
}

func integerRange(first, last int) []int {
	values := make([]int, 0, last-first+1)
	for value := first; value <= last; value++ {
		values = append(values, value)
	}
	return values
}

func (s *mockServer) handleOpenRouterVideoFetch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeOpenRouterError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	taskID := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/v1/videos/"))
	task, ok := s.pollTask(taskID)
	if !ok || task.Provider != "openrouter" {
		writeOpenRouterError(w, http.StatusNotFound, "video_not_found", "video task not found")
		return
	}

	status := "processing"
	response := map[string]any{
		"id": task.ID, "model": task.Model, "status": status,
		"duration": task.Duration, "resolution": task.Resolution, "aspect_ratio": task.Ratio,
		"created_at": task.CreatedAt.Unix(),
	}
	if task.PollCount < s.config.CompleteAfterPoll {
		writeJSON(w, http.StatusOK, response)
		return
	}
	response["completed_at"] = task.CompletedAt.Unix()
	if task.ShouldFail {
		response["status"] = "failed"
		response["error"] = map[string]string{"code": "mock_failure", "message": task.FailReason}
		writeJSON(w, http.StatusOK, response)
		return
	}
	tokens := task.Duration * 100
	response["status"] = "completed"
	response["output"] = map[string]any{"video_url": task.VideoURL, "unsigned_urls": []string{task.VideoURL}}
	response["usage"] = map[string]int{"video_tokens": tokens, "total_tokens": tokens}
	response["provider_cost"] = map[string]float64{"usd": float64(task.Duration) * 0.01}
	writeJSON(w, http.StatusOK, response)
}

func validateOpenRouterVideoRequest(req openRouterVideoRequest) (string, error) {
	model := strings.ToLower(strings.TrimSpace(req.Model))
	if strings.TrimSpace(req.Prompt) == "" && len(req.FrameImages) == 0 && len(req.InputReferences) == 0 {
		return "", fmt.Errorf("prompt is required")
	}
	switch {
	case strings.HasPrefix(model, "google/veo-"):
		return "veo", validateOpenRouterVeoOptions(req)
	case strings.HasPrefix(model, "bytedance/seedance-"):
		return "seedance", validateOpenRouterSeedanceOptions(req)
	case strings.HasPrefix(model, "alibaba/happyhorse-1."):
		return "happyhorse", validateOpenRouterHappyHorseOptions(req)
	case strings.HasPrefix(model, "kwaivgi/kling-v3.0-"), model == "kwaivgi/kling-video-o1":
		return "kling", validateOpenRouterKlingOptions(req)
	case model == "minimax/hailuo-3", model == "minimax/hailuo-2.3":
		return "minimax", validateOpenRouterMiniMaxOptions(req)
	default:
		return "", fmt.Errorf("unsupported OpenRouter video model %s", req.Model)
	}
}

func validateOpenRouterHappyHorseOptions(req openRouterVideoRequest) error {
	if req.Duration != nil && (*req.Duration < 3 || *req.Duration > 15) {
		return fmt.Errorf("duration must be between 3 and 15 seconds")
	}
	resolution := strings.ToLower(strings.TrimSpace(req.Resolution))
	if resolution != "" && resolution != "720p" && resolution != "1080p" {
		return fmt.Errorf("resolution must be 720p or 1080p")
	}
	return validateOpenRouterAspectRatio(req.AspectRatio, []string{"16:9", "9:16", "1:1", "4:3", "3:4", "21:9", "9:21"})
}

func validateOpenRouterKlingOptions(req openRouterVideoRequest) error {
	if strings.EqualFold(req.Model, "kwaivgi/kling-video-o1") {
		if req.Duration != nil && *req.Duration != 5 && *req.Duration != 10 {
			return fmt.Errorf("duration must be 5 or 10 seconds")
		}
	} else if req.Duration != nil && (*req.Duration < 3 || *req.Duration > 15) {
		return fmt.Errorf("duration must be between 3 and 15 seconds")
	}
	if resolution := strings.ToLower(strings.TrimSpace(req.Resolution)); resolution != "" && resolution != "720p" {
		return fmt.Errorf("resolution must be 720p")
	}
	return validateOpenRouterAspectRatio(req.AspectRatio, []string{"16:9", "9:16", "1:1"})
}

func validateOpenRouterMiniMaxOptions(req openRouterVideoRequest) error {
	isHailuo23 := strings.EqualFold(req.Model, "minimax/hailuo-2.3")
	if isHailuo23 {
		if req.Duration != nil && *req.Duration != 6 && *req.Duration != 10 {
			return fmt.Errorf("duration must be 6 or 10 seconds")
		}
		if resolution := strings.ToLower(strings.TrimSpace(req.Resolution)); resolution != "" && resolution != "1080p" {
			return fmt.Errorf("resolution must be 1080p")
		}
		return validateOpenRouterAspectRatio(req.AspectRatio, []string{"16:9"})
	}
	if req.Duration != nil && (*req.Duration < 5 || *req.Duration > 15) {
		return fmt.Errorf("duration must be between 5 and 15 seconds")
	}
	if resolution := strings.ToLower(strings.TrimSpace(req.Resolution)); resolution != "" && resolution != "2k" {
		return fmt.Errorf("resolution must be 2K")
	}
	return validateOpenRouterAspectRatio(req.AspectRatio, []string{"21:9", "16:9", "4:3", "1:1", "3:4", "9:16"})
}

func validateOpenRouterAspectRatio(value string, allowed []string) error {
	ratio := strings.TrimSpace(value)
	if ratio != "" && !slices.Contains(allowed, ratio) {
		return fmt.Errorf("unsupported aspect_ratio")
	}
	return nil
}

func validateOpenRouterVeoOptions(req openRouterVideoRequest) error {
	if req.Duration != nil && *req.Duration != 4 && *req.Duration != 6 && *req.Duration != 8 {
		return fmt.Errorf("duration must be one of 4, 6, or 8")
	}
	resolution := strings.ToLower(strings.TrimSpace(req.Resolution))
	if resolution != "" && resolution != "720p" && resolution != "1080p" {
		return fmt.Errorf("resolution must be 720p or 1080p")
	}
	ratio := strings.TrimSpace(req.AspectRatio)
	if ratio != "" && ratio != "16:9" && ratio != "9:16" {
		return fmt.Errorf("aspect_ratio must be 16:9 or 9:16")
	}
	if len(req.FrameImages) > 2 {
		return fmt.Errorf("at most two frame_images are supported")
	}
	seenFrames := map[string]bool{}
	for _, frame := range req.FrameImages {
		if frame.FrameType != "first_frame" && frame.FrameType != "last_frame" {
			return fmt.Errorf("frame_type must be first_frame or last_frame")
		}
		if seenFrames[frame.FrameType] || strings.TrimSpace(frame.ImageURL.URL) == "" {
			return fmt.Errorf("frame_images require unique frame types and non-empty image URLs")
		}
		seenFrames[frame.FrameType] = true
	}
	for _, reference := range req.InputReferences {
		mediaType, mediaURL := reference.mediaTypeAndURL()
		if mediaType != "image" || strings.TrimSpace(mediaURL) == "" {
			return fmt.Errorf("Veo input_references only support non-empty image references")
		}
	}
	return nil
}

func validateOpenRouterSeedanceOptions(req openRouterVideoRequest) error {
	if req.Duration != nil && (*req.Duration < 4 || *req.Duration > 15) {
		return fmt.Errorf("duration must be between 4 and 15 seconds")
	}
	resolution := strings.ToLower(strings.TrimSpace(req.Resolution))
	if resolution != "" && !slices.Contains([]string{"480p", "720p", "1080p", "4k"}, resolution) {
		return fmt.Errorf("resolution must be 480p, 720p, 1080p, or 4k")
	}
	ratio := strings.TrimSpace(req.AspectRatio)
	if ratio != "" && !slices.Contains([]string{"1:1", "3:4", "9:16", "4:3", "16:9", "21:9", "9:21"}, ratio) {
		return fmt.Errorf("unsupported aspect_ratio")
	}
	if len(req.FrameImages) > 2 {
		return fmt.Errorf("at most two frame_images are supported")
	}
	seenFrames := map[string]bool{}
	imageCount := 0
	for _, frame := range req.FrameImages {
		if frame.FrameType != "first_frame" && frame.FrameType != "last_frame" {
			return fmt.Errorf("frame_type must be first_frame or last_frame")
		}
		if seenFrames[frame.FrameType] || strings.TrimSpace(frame.ImageURL.URL) == "" {
			return fmt.Errorf("frame_images require unique frame types and non-empty image URLs")
		}
		seenFrames[frame.FrameType] = true
		imageCount++
	}
	videoCount, audioCount := 0, 0
	for _, reference := range req.InputReferences {
		mediaType, mediaURL := reference.mediaTypeAndURL()
		if strings.TrimSpace(mediaURL) == "" {
			return fmt.Errorf("input_references require non-empty URLs")
		}
		switch mediaType {
		case "image":
			imageCount++
		case "video":
			videoCount++
		case "audio":
			audioCount++
		default:
			return fmt.Errorf("input reference type must be image, video, or audio")
		}
	}
	if imageCount > 9 || videoCount > 3 || audioCount > 3 || imageCount+videoCount+audioCount > 12 {
		return fmt.Errorf("Seedance supports at most 9 images, 3 videos, 3 audios, and 12 media inputs total")
	}
	return nil
}

func (s *mockServer) handleSubmit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAliError(w, http.StatusMethodNotAllowed, "MethodNotAllowed", "method not allowed")
		return
	}

	var req taskali.AliVideoRequest
	if err := common.DecodeJson(r.Body, &req); err != nil {
		writeAliError(w, http.StatusBadRequest, "InvalidJSON", err.Error())
		return
	}

	family, ok := detectModelFamily(req.Model)
	if !ok {
		writeAliError(w, http.StatusBadRequest, "UnsupportedModel", fmt.Sprintf("mock does not support model %s", req.Model))
		return
	}
	if err := validateMockRequest(req, family); err != nil {
		writeAliError(w, http.StatusBadRequest, "InvalidParameter", err.Error())
		return
	}

	now := time.Now()
	duration := 5
	if req.Parameters != nil && req.Parameters.Duration > 0 {
		duration = req.Parameters.Duration
	}
	resolution, sr := resolveResolution(req)
	audio := resolveAudio(req, family)
	summary := summarizeRequest(req, family, duration, resolution, sr, audio)
	log.Printf("submit request remote=%s summary=%s", r.RemoteAddr, formatRequestSummary(summary))

	taskID := s.nextTaskID()
	videoURL := s.buildAssetURL(r, fmt.Sprintf("/mock-assets/videos/%s.mp4", taskID))
	watermarkURL := ""
	if req.Parameters != nil && req.Parameters.Watermark != nil && *req.Parameters.Watermark {
		watermarkURL = s.buildAssetURL(r, fmt.Sprintf("/mock-assets/videos/%s-watermark.mp4", taskID))
	}

	task := &mockTask{
		ID:           taskID,
		Provider:     "ali",
		Model:        req.Model,
		Family:       family,
		Duration:     duration,
		Resolution:   resolution,
		SR:           sr,
		Audio:        audio,
		CreatedAt:    now,
		ScheduledAt:  now.Add(800 * time.Millisecond),
		CompletedAt:  now.Add(1600 * time.Millisecond),
		ShouldFail:   s.shouldFail(),
		FailReason:   "mock upstream random failure",
		VideoURL:     videoURL,
		WatermarkURL: watermarkURL,
	}

	s.mu.Lock()
	s.tasks[taskID] = task
	s.mu.Unlock()
	log.Printf("task created id=%s model=%s family=%s duration=%ds resolution=%s sr=%d audio=%t should_fail=%t watermark=%t video_url=%s",
		task.ID, task.Model, task.Family, task.Duration, task.Resolution, task.SR, task.Audio, task.ShouldFail, task.WatermarkURL != "", task.VideoURL)

	resp := taskali.AliVideoResponse{
		RequestID: taskID + "-request",
		Output: taskali.AliVideoOutput{
			TaskID:        taskID,
			TaskStatus:    mockTaskPending,
			SubmitTime:    formatAliTime(now),
			ScheduledTime: formatAliTime(task.ScheduledAt),
		},
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *mockServer) handleSeedanceSubmit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": map[string]string{"code": "MethodNotAllowed", "message": "method not allowed"}})
		return
	}
	var req seedanceRequest
	if err := common.DecodeJson(r.Body, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]string{"code": "InvalidJSON", "message": err.Error()}})
		return
	}
	if !isSeedanceModel(req.Model) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]string{"code": "UnsupportedModel", "message": "unsupported Seedance model"}})
		return
	}
	hasInput := false
	for _, item := range req.Content {
		if strings.TrimSpace(item.Text) != "" || item.ImageURL != nil || item.VideoURL != nil || item.AudioURL != nil {
			hasInput = true
			break
		}
	}
	if !hasInput {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]string{"code": "InvalidParameter", "message": "content must include text, image, video, or audio input"}})
		return
	}

	now := time.Now()
	taskID := strings.Replace(s.nextTaskID(), "mock-task", "mock-seedance", 1)
	duration := req.Duration
	if duration <= 0 {
		duration = 5
	}
	resolution := strings.ToLower(strings.TrimSpace(req.Resolution))
	if resolution == "" {
		resolution = "720p"
	}
	ratio := strings.TrimSpace(req.Ratio)
	if ratio == "" {
		ratio = "16:9"
	}
	audio := req.GenerateAudio != nil && *req.GenerateAudio
	task := &mockTask{
		ID: taskID, Provider: "seedance", Model: req.Model, Family: "seedance",
		Duration: duration, Resolution: resolution, SR: resolutionValue(resolution), Audio: audio,
		CreatedAt: now, ScheduledAt: now.Add(800 * time.Millisecond), CompletedAt: now.Add(1600 * time.Millisecond),
		ShouldFail: s.shouldFail(), FailReason: "mock upstream random failure",
		VideoURL: s.buildAssetURL(r, "/mock-assets/videos/"+taskID+".mp4"), Ratio: ratio, Seed: req.Seed,
	}
	s.storeTask(task)
	writeJSON(w, http.StatusOK, map[string]any{"id": taskID})
}

func (s *mockServer) handleSeedanceFetch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": map[string]string{"code": "MethodNotAllowed", "message": "method not allowed"}})
		return
	}
	taskID := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/api/v3/contents/generations/tasks/"))
	task, ok := s.pollTask(taskID)
	if !ok || task.Provider != "seedance" {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]string{"code": "TaskNotFound", "message": "task not found"}})
		return
	}
	status := "running"
	videoURL := ""
	errorCode, errorMessage := "", ""
	if task.PollCount >= s.config.CompleteAfterPoll {
		if task.ShouldFail {
			status, errorCode, errorMessage = "failed", "MockFailure", task.FailReason
		} else {
			status, videoURL = "succeeded", task.VideoURL
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id": task.ID, "model": task.Model, "status": status,
		"content": map[string]string{"video_url": videoURL}, "seed": task.Seed,
		"resolution": task.Resolution, "duration": task.Duration, "ratio": task.Ratio, "framespersecond": 24,
		"usage":      map[string]int{"completion_tokens": task.Duration * 100, "total_tokens": task.Duration * 100},
		"error":      map[string]string{"code": errorCode, "message": errorMessage},
		"created_at": task.CreatedAt.Unix(), "updated_at": time.Now().Unix(),
	})
}

func (s *mockServer) handleVeo(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, ":predictLongRunning") {
		s.handleVeoSubmit(w, r)
		return
	}
	if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, ":fetchPredictOperation") {
		s.handleVertexVeoFetch(w, r)
		return
	}
	if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/operations/") {
		s.handleGeminiVeoFetch(w, r)
		return
	}
	http.NotFound(w, r)
}

func (s *mockServer) handleVeoSubmit(w http.ResponseWriter, r *http.Request) {
	var req veoRequest
	if err := common.DecodeJson(r.Body, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]string{"message": err.Error()}})
		return
	}
	model := extractVeoModel(r.URL.Path)
	if model == "" || len(req.Instances) == 0 || (strings.TrimSpace(req.Instances[0].Prompt) == "" && req.Instances[0].Image == nil) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]string{"message": "Veo requires a supported model and prompt or image input"}})
		return
	}
	provider := "gemini"
	operationName := "models/" + model + "/operations/" + strings.Replace(s.nextTaskID(), "mock-task", "mock-veo", 1)
	if strings.Contains(r.URL.Path, "/projects/") {
		provider = "vertex"
		prefix := strings.Split(r.URL.Path, ":predictLongRunning")[0]
		prefix = strings.TrimPrefix(prefix, "/")
		if slash := strings.Index(prefix, "/"); slash >= 0 && strings.HasPrefix(prefix, "v") {
			prefix = prefix[slash+1:]
		}
		operationName = prefix + "/operations/" + strings.Replace(s.nextTaskID(), "mock-task", "mock-veo", 1)
	}
	now := time.Now()
	duration := req.Parameters.DurationSeconds
	if duration <= 0 {
		duration = 8
	}
	resolution := strings.ToLower(strings.TrimSpace(req.Parameters.Resolution))
	if resolution == "" {
		resolution = "720p"
	}
	audio := req.Parameters.GenerateAudio != nil && *req.Parameters.GenerateAudio
	seed := 0
	if req.Parameters.Seed != nil {
		seed = *req.Parameters.Seed
	}
	task := &mockTask{
		ID: path.Base(operationName), Provider: provider, OperationName: operationName, Model: model, Family: "veo",
		Duration: duration, Resolution: resolution, SR: resolutionValue(resolution), Audio: audio, Seed: seed,
		Ratio: req.Parameters.AspectRatio, CreatedAt: now, ScheduledAt: now.Add(800 * time.Millisecond), CompletedAt: now.Add(1600 * time.Millisecond),
		ShouldFail: s.shouldFail(), FailReason: "mock upstream random failure",
		VideoURL: s.buildAssetURL(r, "/mock-assets/videos/"+path.Base(operationName)+".mp4"),
	}
	s.storeTask(task)
	writeJSON(w, http.StatusOK, map[string]string{"name": operationName})
}

func (s *mockServer) handleGeminiVeoFetch(w http.ResponseWriter, r *http.Request) {
	operationName := strings.TrimPrefix(r.URL.Path, "/")
	if slash := strings.Index(operationName, "/models/"); slash >= 0 {
		operationName = operationName[slash+1:]
	}
	task, ok := s.pollOperation(operationName)
	if !ok || task.Provider != "gemini" {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]string{"message": "operation not found"}})
		return
	}
	s.writeVeoOperation(w, task, false)
}

func (s *mockServer) handleVertexVeoFetch(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		OperationName string `json:"operationName"`
	}
	if err := common.DecodeJson(r.Body, &payload); err != nil || strings.TrimSpace(payload.OperationName) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]string{"message": "operationName is required"}})
		return
	}
	task, ok := s.pollOperation(payload.OperationName)
	if !ok || task.Provider != "vertex" {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]string{"message": "operation not found"}})
		return
	}
	s.writeVeoOperation(w, task, true)
}

func (s *mockServer) writeVeoOperation(w http.ResponseWriter, task mockTask, vertex bool) {
	if task.PollCount < s.config.CompleteAfterPoll {
		writeJSON(w, http.StatusOK, map[string]any{"name": task.OperationName, "done": false})
		return
	}
	if task.ShouldFail {
		writeJSON(w, http.StatusOK, map[string]any{"name": task.OperationName, "done": true, "error": map[string]string{"message": task.FailReason}})
		return
	}
	if vertex {
		writeJSON(w, http.StatusOK, map[string]any{"name": task.OperationName, "done": true, "response": map[string]any{
			"videos": []map[string]string{{"mimeType": "video/mp4", "bytesBase64Encoded": base64.StdEncoding.EncodeToString(s.videoBytes), "encoding": "mp4"}},
		}})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"name": task.OperationName, "done": true, "response": map[string]any{
		"generateVideoResponse": map[string]any{"generatedVideos": []map[string]any{{"video": map[string]string{"uri": task.VideoURL}}}},
	}})
}

func (s *mockServer) handleFetchTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAliError(w, http.StatusMethodNotAllowed, "MethodNotAllowed", "method not allowed")
		return
	}

	taskID := strings.TrimPrefix(r.URL.Path, "/api/v1/tasks/")
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		writeAliError(w, http.StatusBadRequest, "InvalidTaskID", "task id is required")
		return
	}

	s.mu.Lock()
	storedTask, ok := s.tasks[taskID]
	if ok {
		storedTask.PollCount++
	}
	var task mockTask
	if ok {
		task = *storedTask
	}
	s.mu.Unlock()
	if !ok {
		writeAliError(w, http.StatusNotFound, "TaskNotFound", "task not found")
		return
	}

	status := mockTaskPending
	videoURL := ""
	watermarkURL := ""
	failReason := ""
	endTime := ""
	switch {
	case task.PollCount >= s.config.CompleteAfterPoll:
		if task.ShouldFail {
			status = mockTaskFailed
			failReason = task.FailReason
		} else {
			status = mockTaskSuccess
			videoURL = task.VideoURL
			watermarkURL = task.WatermarkURL
		}
		endTime = formatAliTime(task.CompletedAt)
	case task.PollCount >= 1:
		status = mockTaskRunning
	default:
		status = mockTaskPending
	}
	log.Printf("fetch task remote=%s id=%s poll=%d status=%s model=%s resolution=%s sr=%d audio=%t should_fail=%t fail_reason=%q",
		r.RemoteAddr, task.ID, task.PollCount, status, task.Model, task.Resolution, task.SR, task.Audio, task.ShouldFail, failReason)

	resp := taskali.AliVideoResponse{
		RequestID: task.ID + "-request",
		Output: taskali.AliVideoOutput{
			TaskID:        task.ID,
			TaskStatus:    status,
			SubmitTime:    formatAliTime(task.CreatedAt),
			ScheduledTime: formatAliTime(task.ScheduledAt),
			EndTime:       endTime,
			VideoURL:      videoURL,
			WatermarkURL:  watermarkURL,
			Message:       failReason,
		},
		Usage: &taskali.AliUsage{
			Duration:            task.Duration,
			OutputVideoDuration: task.Duration,
			VideoCount:          1,
			SR:                  task.SR,
			Audio:               task.Audio,
			Size:                task.Resolution,
		},
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *mockServer) handleMockVideo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !strings.HasSuffix(strings.ToLower(r.URL.Path), ".mp4") {
		http.NotFound(w, r)
		return
	}
	log.Printf("mock asset request remote=%s method=%s path=%s", r.RemoteAddr, r.Method, r.URL.Path)
	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", path.Base(r.URL.Path)))
	http.ServeContent(w, r, path.Base(r.URL.Path), time.Unix(1, 0), bytes.NewReader(s.videoBytes))
}

func (s *mockServer) nextTaskID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextID++
	return fmt.Sprintf("mock-task-%06d", s.nextID)
}

func (s *mockServer) storeTask(task *mockTask) {
	s.mu.Lock()
	s.tasks[task.ID] = task
	s.mu.Unlock()
}

func (s *mockServer) pollTask(taskID string) (mockTask, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	task, ok := s.tasks[taskID]
	if !ok {
		return mockTask{}, false
	}
	task.PollCount++
	return *task, true
}

func (s *mockServer) pollOperation(operationName string) (mockTask, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, task := range s.tasks {
		if task.OperationName == operationName {
			task.PollCount++
			return *task, true
		}
	}
	return mockTask{}, false
}

func isSeedanceModel(model string) bool {
	return slices.Contains(taskdoubao.ModelList, strings.ToLower(strings.TrimSpace(model)))
}

func extractVeoModel(requestPath string) string {
	marker := "/models/"
	idx := strings.LastIndex(requestPath, marker)
	if idx < 0 {
		return ""
	}
	model := requestPath[idx+len(marker):]
	model = strings.TrimSuffix(model, ":predictLongRunning")
	switch model {
	case "veo-3.0-generate-001", "veo-3.0-fast-generate-001", "veo-3.1-generate-preview", "veo-3.1-fast-generate-preview":
		return model
	default:
		return ""
	}
}

func resolutionValue(resolution string) int {
	_, value, ok := normalizeResolution(resolution)
	if ok {
		return value
	}
	return 720
}

func detectModelFamily(model string) (string, bool) {
	name := strings.ToLower(strings.TrimSpace(model))
	switch {
	case slices.Contains(taskali.ModelList, name):
		return "wan", true
	case strings.HasPrefix(name, "happyhorse-1.0"), strings.HasPrefix(name, "happyhorse-1.1"):
		return "happyhorse", true
	case strings.HasPrefix(name, "kling/kling-v3-"):
		return "kling", true
	default:
		return "", false
	}
}

func validateMockRequest(req taskali.AliVideoRequest, family string) error {
	model := strings.ToLower(strings.TrimSpace(req.Model))
	switch family {
	case "wan":
		if strings.Contains(model, "-t2v") && strings.TrimSpace(req.Input.Prompt) == "" {
			return fmt.Errorf("wan text-to-video requires prompt")
		}
		if strings.Contains(model, "-i2v") && len(req.Input.Media) == 0 && strings.TrimSpace(req.Input.ImgURL) == "" && strings.TrimSpace(req.Input.FirstFrameURL) == "" {
			return fmt.Errorf("wan image-to-video requires image input")
		}
		return nil
	case "happyhorse":
		switch {
		case strings.Contains(model, "-t2v"):
			if strings.TrimSpace(req.Input.Prompt) == "" {
				return fmt.Errorf("happyhorse text-to-video requires prompt")
			}
		case strings.Contains(model, "-i2v"):
			if countMockMedia(req.Input.Media, "first_frame") != 1 || len(req.Input.Media) != 1 {
				return fmt.Errorf("happyhorse image-to-video requires exactly one first_frame")
			}
		case strings.Contains(model, "-r2v"):
			references := countMockMedia(req.Input.Media, "reference_image")
			if strings.TrimSpace(req.Input.Prompt) == "" || references < 1 || references > 9 || references != len(req.Input.Media) {
				return fmt.Errorf("happyhorse reference-to-video requires prompt and 1-9 reference_image items")
			}
		case strings.Contains(model, "-video-edit"):
			if strings.TrimSpace(req.Input.Prompt) == "" || countMockMedia(req.Input.Media, "video") != 1 || countMockMedia(req.Input.Media, "reference_image") > 5 {
				return fmt.Errorf("happyhorse video edit requires prompt, one video, and at most five reference_image items")
			}
		default:
			return fmt.Errorf("unsupported happyhorse model %s", req.Model)
		}
		return nil
	case "kling":
		return validateKlingMockRequest(req)
	default:
		return fmt.Errorf("unsupported model family %s", family)
	}
}

func validateKlingMockRequest(req taskali.AliVideoRequest) error {
	customShots := req.Input.MultiShot != nil && *req.Input.MultiShot && req.Input.ShotType != nil && *req.Input.ShotType == "customize" && len(req.Input.MultiPrompt) > 0
	if strings.TrimSpace(req.Input.Prompt) == "" && !customShots {
		return fmt.Errorf("kling requires prompt or customized multi_prompt")
	}
	if req.Input.MultiShot != nil && *req.Input.MultiShot {
		if req.Input.ShotType == nil || strings.TrimSpace(*req.Input.ShotType) == "" {
			return fmt.Errorf("kling multi_shot requires shot_type")
		}
		if *req.Input.ShotType == "customize" && len(req.Input.MultiPrompt) == 0 {
			return fmt.Errorf("kling customized shots require multi_prompt")
		}
	}

	isOmni := strings.Contains(strings.ToLower(req.Model), "omni")
	for _, media := range req.Input.Media {
		switch media.Type {
		case "first_frame", "last_frame":
		case "refer", "base", "feature":
			if !isOmni {
				return fmt.Errorf("kling standard does not support media type %s", media.Type)
			}
		default:
			return fmt.Errorf("unsupported kling media type %s", media.Type)
		}
	}

	firstFrames := countMockMedia(req.Input.Media, "first_frame")
	lastFrames := countMockMedia(req.Input.Media, "last_frame")
	references := countMockMedia(req.Input.Media, "refer")
	bases := countMockMedia(req.Input.Media, "base")
	features := countMockMedia(req.Input.Media, "feature")
	if req.Parameters != nil && req.Parameters.Audio != nil && *req.Parameters.Audio && (bases > 0 || features > 0) {
		return fmt.Errorf("kling audio must be false for base or feature video input")
	}

	valid := len(req.Input.Media) == 0 ||
		(firstFrames == 1 && len(req.Input.Media) == 1) ||
		(firstFrames == 1 && lastFrames == 1 && len(req.Input.Media) == 2)
	if isOmni {
		valid = valid || references == len(req.Input.Media) ||
			(features == 1 && len(req.Input.Media) == 1) ||
			(features == 1 && references+features == len(req.Input.Media)) ||
			(features == 1 && firstFrames == 1 && len(req.Input.Media) == 2) ||
			(bases == 1 && len(req.Input.Media) == 1) ||
			(bases == 1 && references+bases == len(req.Input.Media))
	}
	if !valid {
		return fmt.Errorf("unsupported kling media combination")
	}

	elements := len(req.Input.ElementList)
	switch {
	case references > 0 && (bases > 0 || features > 0) && references+elements > 4:
		return fmt.Errorf("kling base/feature reference and element total must be at most 4")
	case references > 0 && references+elements > 7:
		return fmt.Errorf("kling reference and element total must be at most 7")
	case firstFrames > 0 && elements > 3:
		return fmt.Errorf("kling frame generation supports at most 3 elements")
	}
	return nil
}

func countMockMedia(media []taskali.AliMediaItem, mediaType string) int {
	count := 0
	for _, item := range media {
		if item.Type == mediaType {
			count++
		}
	}
	return count
}

func resolveResolution(req taskali.AliVideoRequest) (string, int) {
	if req.Parameters != nil {
		if resolution, sr, ok := normalizeResolution(req.Parameters.Resolution); ok {
			return resolution, sr
		}
		if resolution, sr, ok := sizeToResolution(req.Parameters.Size); ok {
			return resolution, sr
		}
		if req.Parameters.Mode != nil {
			if strings.EqualFold(strings.TrimSpace(*req.Parameters.Mode), "std") {
				return "720P", 720
			}
			return "1080P", 1080
		}
	}
	return "1080P", 1080
}

func normalizeResolution(raw string) (string, int, bool) {
	value := strings.ToUpper(strings.TrimSpace(raw))
	switch value {
	case "480P":
		return "480P", 480, true
	case "720P":
		return "720P", 720, true
	case "1080P":
		return "1080P", 1080, true
	case "2K":
		return "2K", 2000, true
	case "4K":
		return "4K", 4000, true
	default:
		return "", 0, false
	}
}

func sizeToResolution(size string) (string, int, bool) {
	switch strings.TrimSpace(size) {
	case "832*480", "480*832", "624*624":
		return "480P", 480, true
	case "1280*720", "720*1280", "960*960", "1088*832", "832*1088":
		return "720P", 720, true
	case "1920*1080", "1080*1920", "1440*1440", "1632*1248", "1248*1632":
		return "1080P", 1080, true
	default:
		return "", 0, false
	}
}

func resolveAudio(req taskali.AliVideoRequest, family string) bool {
	if req.Parameters != nil && req.Parameters.Audio != nil {
		return *req.Parameters.Audio
	}
	if family == "wan" {
		model := strings.ToLower(req.Model)
		return !strings.HasPrefix(model, "wan2.2") && !strings.HasPrefix(model, "wanx2.1")
	}
	return true
}

func formatAliTime(t time.Time) string {
	return t.Format("2006-01-02 15:04:05.000")
}

func buildAbsoluteURL(r *http.Request, path string) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if forwardedProto := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")); forwardedProto != "" {
		scheme = strings.Split(forwardedProto, ",")[0]
	}
	host := r.Host
	if forwardedHost := strings.TrimSpace(r.Header.Get("X-Forwarded-Host")); forwardedHost != "" {
		host = strings.Split(forwardedHost, ",")[0]
	}
	return scheme + "://" + host + path
}

func (s *mockServer) buildAssetURL(r *http.Request, assetPath string) string {
	if s.config.PublicBaseURL != "" {
		return strings.TrimRight(s.config.PublicBaseURL, "/") + assetPath
	}
	return buildAbsoluteURL(r, assetPath)
}

func writeAliError(w http.ResponseWriter, status int, code string, message string) {
	log.Printf("mock error status=%d code=%s message=%s", status, code, message)
	writeJSON(w, status, taskali.AliVideoResponse{
		Code:      code,
		Message:   message,
		RequestID: "mock-error",
	})
}

func writeOpenRouterError(w http.ResponseWriter, status int, code string, message string) {
	log.Printf("openrouter mock error status=%d code=%s message=%s", status, code, message)
	writeJSON(w, status, map[string]any{
		"error": map[string]string{"code": code, "message": message},
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	body, err := common.Marshal(payload)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func getenv(key string, fallback string) string {
	if env := strings.TrimSpace(os.Getenv(key)); env != "" {
		return env
	}
	return fallback
}

func loadMockConfig() mockConfig {
	failRate := 0.0
	if raw := strings.TrimSpace(os.Getenv("ALI_VIDEO_MOCK_FAIL_RATE")); raw != "" {
		if parsed, err := strconv.ParseFloat(raw, 64); err == nil {
			switch {
			case parsed < 0:
				failRate = 0
			case parsed > 1:
				failRate = 1
			default:
				failRate = parsed
			}
		}
	}
	completeAfterPoll := 2
	if raw := strings.TrimSpace(os.Getenv("ALI_VIDEO_MOCK_COMPLETE_AFTER_POLL")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			completeAfterPoll = parsed
		}
	}
	return mockConfig{
		FailRate:          failRate,
		CompleteAfterPoll: completeAfterPoll,
		PublicBaseURL:     strings.TrimRight(strings.TrimSpace(os.Getenv("ALI_VIDEO_MOCK_PUBLIC_BASE_URL")), "/"),
	}
}

func (s *mockServer) shouldFail() bool {
	if s.config.FailRate <= 0 {
		return false
	}
	if s.config.FailRate >= 1 {
		return true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rng.Float64() < s.config.FailRate
}

func summarizeRequest(req taskali.AliVideoRequest, family string, duration int, resolution string, sr int, audio bool) requestSummary {
	mediaCount := len(req.Input.Media)
	if req.Input.ImgURL != "" {
		mediaCount++
	}
	if req.Input.FirstFrameURL != "" {
		mediaCount++
	}
	if req.Input.LastFrameURL != "" {
		mediaCount++
	}
	return requestSummary{
		Model:      req.Model,
		Family:     family,
		Prompt:     truncateText(firstNonEmpty(req.Input.Prompt, req.Input.Template), 80),
		MediaCount: mediaCount,
		Duration:   duration,
		Resolution: resolution,
		SR:         sr,
		Audio:      audio,
	}
}

func formatRequestSummary(summary requestSummary) string {
	return fmt.Sprintf("model=%s family=%s duration=%ds resolution=%s sr=%d audio=%t media=%d prompt=%q",
		summary.Model, summary.Family, summary.Duration, summary.Resolution, summary.SR, summary.Audio, summary.MediaCount, summary.Prompt)
}

func truncateText(value string, limit int) string {
	value = strings.TrimSpace(value)
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "..."
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
