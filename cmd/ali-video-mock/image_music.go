package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	relaydto "github.com/QuantumNous/new-api/dto"
)

var mockImageModels = map[string]struct{}{
	"gpt-image-2": {}, "gemini-3-pro-image-preview": {},
	"gemini-3.1-flash-image": {}, "qwen-image-2.0": {},
}

var mockImageSizes = map[string]struct{}{
	"1024x1024": {}, "1024x576": {}, "576x1024": {}, "1024x768": {},
	"768x1024": {}, "1008x672": {}, "672x1008": {}, "1536x1024": {}, "1024x1536": {},
}

type mockImageRequest struct {
	Model          string `json:"model"`
	Prompt         string `json:"prompt"`
	N              *uint  `json:"n,omitempty"`
	Size           string `json:"size,omitempty"`
	ResponseFormat string `json:"response_format,omitempty"`
	Images         any    `json:"images,omitempty"`
}

type mockSunoRequest struct {
	Description  string `json:"gpt_description_prompt"`
	Prompt       string `json:"prompt"`
	Model        string `json:"mv"`
	Title        string `json:"title"`
	Instrumental bool   `json:"make_instrumental"`
}

func (s *mockServer) handleImageGeneration(w http.ResponseWriter, r *http.Request) {
	s.handleImageRequest(w, r, false)
}

func (s *mockServer) handleImageEdit(w http.ResponseWriter, r *http.Request) {
	s.handleImageRequest(w, r, true)
}

func (s *mockServer) handleImageRequest(w http.ResponseWriter, r *http.Request, edit bool) {
	if r.Method != http.MethodPost {
		writeOpenRouterError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	request, err := decodeMockImageRequest(r)
	if err != nil {
		writeOpenRouterError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if _, ok := mockImageModels[request.Model]; !ok {
		writeOpenRouterError(w, http.StatusBadRequest, "model_not_found", "unsupported image model")
		return
	}
	if strings.TrimSpace(request.Prompt) == "" {
		writeOpenRouterError(w, http.StatusBadRequest, "invalid_prompt", "prompt is required")
		return
	}
	if edit && !hasMockSourceImages(request.Images) {
		writeOpenRouterError(w, http.StatusBadRequest, "image_required", "image edit requires source images")
		return
	}
	size := strings.TrimSpace(request.Size)
	if size == "" {
		size = "1024x1024"
	}
	if _, ok := mockImageSizes[size]; !ok {
		writeOpenRouterError(w, http.StatusBadRequest, "invalid_size", "unsupported image size")
		return
	}
	count := uint(1)
	if request.N != nil {
		count = *request.N
	}
	if count < 1 || count > 8 {
		writeOpenRouterError(w, http.StatusBadRequest, "invalid_n", "n must be between 1 and 8")
		return
	}
	data := make([]map[string]string, 0, count)
	for index := uint(0); index < count; index++ {
		assetID := fmt.Sprintf("mock-image-%d-%d", time.Now().UnixNano(), index+1)
		assetPath := fmt.Sprintf("/mock-assets/images/%s-%s.png", assetID, size)
		data = append(data, map[string]string{
			"url": s.buildAssetURL(r, assetPath), "revised_prompt": request.Prompt,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"created": time.Now().Unix(), "data": data})
}

func hasMockSourceImages(images any) bool {
	switch value := images.(type) {
	case []any:
		return len(value) > 0
	case []string:
		return len(value) > 0
	case string:
		return strings.TrimSpace(value) != ""
	default:
		return false
	}
}

func decodeMockImageRequest(r *http.Request) (mockImageRequest, error) {
	if strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "multipart/form-data") {
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			return mockImageRequest{}, fmt.Errorf("parse multipart image request: %w", err)
		}
		count := uint(1)
		if raw := r.FormValue("n"); raw != "" {
			parsed, err := strconv.ParseUint(raw, 10, 32)
			if err != nil {
				return mockImageRequest{}, fmt.Errorf("invalid n")
			}
			count = uint(parsed)
		}
		images := make([]string, 0)
		for field, files := range r.MultipartForm.File {
			if strings.HasPrefix(field, "image") {
				for _, file := range files {
					images = append(images, file.Filename)
				}
			}
		}
		return mockImageRequest{Model: r.FormValue("model"), Prompt: r.FormValue("prompt"), N: &count, Size: r.FormValue("size"), Images: images}, nil
	}
	var request mockImageRequest
	if err := common.DecodeJson(r.Body, &request); err != nil {
		return mockImageRequest{}, fmt.Errorf("decode image request: %w", err)
	}
	return request, nil
}

func (s *mockServer) handleMockImage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	base := strings.TrimSuffix(path.Base(r.URL.Path), path.Ext(r.URL.Path))
	parts := strings.Split(base, "-")
	if len(parts) < 2 {
		http.NotFound(w, r)
		return
	}
	dimensions := strings.Split(parts[len(parts)-1], "x")
	if len(dimensions) != 2 {
		http.NotFound(w, r)
		return
	}
	width, widthErr := strconv.Atoi(dimensions[0])
	height, heightErr := strconv.Atoi(dimensions[1])
	if widthErr != nil || heightErr != nil || width < 1 || height < 1 || width > 4096 || height > 4096 {
		http.NotFound(w, r)
		return
	}
	canvas := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(canvas, canvas.Bounds(), &image.Uniform{C: color.RGBA{R: 28, G: 113, B: 216, A: 255}}, image.Point{}, draw.Src)
	var body bytes.Buffer
	if err := png.Encode(&body, canvas); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Header().Set("Content-Length", strconv.Itoa(body.Len()))
	if r.Method == http.MethodGet {
		_, _ = w.Write(body.Bytes())
	}
}

func (s *mockServer) handleSunoSubmit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || !strings.EqualFold(strings.TrimPrefix(r.URL.Path, "/suno/submit/"), "music") {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"code": "invalid_action", "message": "music action required", "data": ""})
		return
	}
	var request mockSunoRequest
	if err := common.DecodeJson(r.Body, &request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "invalid_request", "message": err.Error(), "data": ""})
		return
	}
	if request.Model != "chirp-v4" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "model_not_found", "message": "unsupported music model", "data": ""})
		return
	}
	prompt := strings.TrimSpace(request.Description)
	if prompt == "" {
		prompt = strings.TrimSpace(request.Prompt)
	}
	if prompt == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "invalid_prompt", "message": "prompt is required", "data": ""})
		return
	}
	taskID := strings.Replace(s.nextTaskID(), "mock-task", "mock-suno", 1)
	now := time.Now()
	task := &mockTask{
		ID: taskID, Provider: "suno", Model: request.Model, Family: "music", Prompt: prompt,
		Instrumental: request.Instrumental, CreatedAt: now, ScheduledAt: now.Add(time.Second), CompletedAt: s.completionTime(now),
		ShouldFail: s.shouldFail(), FailReason: "mock upstream random failure",
		AudioURL:  s.buildAssetURL(r, "/mock-assets/music/"+taskID+".wav"),
		PosterURL: s.buildAssetURL(r, "/mock-assets/images/"+taskID+"-1024x1024.png"),
	}
	s.storeTask(task)
	writeJSON(w, http.StatusOK, map[string]any{"code": "success", "message": "", "data": taskID})
}

func (s *mockServer) handleSunoFetch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"code": "invalid_request", "message": "method not allowed", "data": []any{}})
		return
	}
	var request struct {
		IDs []string `json:"ids"`
	}
	if err := common.DecodeJson(r.Body, &request); err != nil || len(request.IDs) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "invalid_request", "message": "ids are required", "data": []any{}})
		return
	}
	items := make([]relaydto.SunoDataResponse, 0, len(request.IDs))
	for _, taskID := range request.IDs {
		task, ok := s.pollTask(taskID)
		if !ok || task.Provider != "suno" {
			continue
		}
		status := "IN_PROGRESS"
		finishTime := int64(0)
		var data []byte
		complete := s.taskComplete(task)
		if task.ShouldFail && complete {
			status = "FAILURE"
			finishTime = time.Now().UnixMilli()
		} else if complete {
			status = "SUCCESS"
			finishTime = time.Now().UnixMilli()
			songs := []relaydto.SunoSong{
				{ID: task.ID + "-1", AudioURL: task.AudioURL, ImageURL: task.PosterURL, MajorModelVersion: task.Model, ModelName: task.Model, Status: "complete", Title: "Mock Track 1", Text: task.Prompt},
				{ID: task.ID + "-2", AudioURL: task.AudioURL, ImageURL: task.PosterURL, MajorModelVersion: task.Model, ModelName: task.Model, Status: "complete", Title: "Mock Track 2", Text: task.Prompt},
			}
			data, _ = common.Marshal(songs)
		}
		item := relaydto.SunoDataResponse{
			TaskID: task.ID, Action: "MUSIC", Status: status, SubmitTime: task.CreatedAt.UnixMilli(),
			StartTime: task.ScheduledAt.UnixMilli(), FinishTime: finishTime, Data: data,
		}
		if status == "FAILURE" {
			item.FailReason = task.FailReason
		}
		items = append(items, item)
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": "success", "message": "", "data": items})
}

func (s *mockServer) handleMockMusic(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	body := mockToneWAV(2, 22050, 440)
	w.Header().Set("Content-Type", "audio/wav")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	if r.Method == http.MethodGet {
		_, _ = w.Write(body)
	}
}

func mockToneWAV(seconds int, sampleRate int, frequency float64) []byte {
	sampleCount := seconds * sampleRate
	dataSize := sampleCount * 2
	body := make([]byte, 44+dataSize)
	copy(body[0:4], "RIFF")
	binary.LittleEndian.PutUint32(body[4:8], uint32(36+dataSize))
	copy(body[8:12], "WAVE")
	copy(body[12:16], "fmt ")
	binary.LittleEndian.PutUint32(body[16:20], 16)
	binary.LittleEndian.PutUint16(body[20:22], 1)
	binary.LittleEndian.PutUint16(body[22:24], 1)
	binary.LittleEndian.PutUint32(body[24:28], uint32(sampleRate))
	binary.LittleEndian.PutUint32(body[28:32], uint32(sampleRate*2))
	binary.LittleEndian.PutUint16(body[32:34], 2)
	binary.LittleEndian.PutUint16(body[34:36], 16)
	copy(body[36:40], "data")
	binary.LittleEndian.PutUint32(body[40:44], uint32(dataSize))
	for index := 0; index < sampleCount; index++ {
		sample := int16(math.Sin(2*math.Pi*frequency*float64(index)/float64(sampleRate)) * 4096)
		binary.LittleEndian.PutUint16(body[44+index*2:], uint16(sample))
	}
	return body
}
