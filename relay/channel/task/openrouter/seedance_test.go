package openrouter

import (
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

func TestSeedanceBuildUpstreamRequestUsesFrameImagesForOneImage(t *testing.T) {
	handler := &SeedanceHandler{BaseHandler: NewBaseHandler("seedance")}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "bytedance/seedance-2.0"}}
	body, err := handler.BuildUpstreamRequest(info, &relaycommon.TaskSubmitReq{
		Model:    "bytedance/seedance-2.0",
		Prompt:   "hello",
		Duration: 5,
		Images:   []string{"https://example.com/first.png"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	frameImages, ok := body["frame_images"].([]map[string]any)
	if !ok || len(frameImages) != 1 {
		t.Fatalf("expected 1 frame image, got %#v", body["frame_images"])
	}
	if frameImages[0]["frame_type"] != "first_frame" {
		t.Fatalf("expected first_frame, got %#v", frameImages[0]["frame_type"])
	}
	if _, exists := body["input_references"]; exists {
		t.Fatalf("expected no input references, got %#v", body["input_references"])
	}
}

func TestSeedanceBuildUpstreamRequestUsesFirstAndLastFrameForTwoImages(t *testing.T) {
	handler := &SeedanceHandler{BaseHandler: NewBaseHandler("seedance")}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "bytedance/seedance-2.0"}}
	body, err := handler.BuildUpstreamRequest(info, &relaycommon.TaskSubmitReq{
		Model:    "bytedance/seedance-2.0",
		Prompt:   "hello",
		Duration: 5,
		Images:   []string{"https://example.com/first.png", "https://example.com/last.png"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	frameImages, ok := body["frame_images"].([]map[string]any)
	if !ok || len(frameImages) != 2 {
		t.Fatalf("expected 2 frame images, got %#v", body["frame_images"])
	}
	if frameImages[0]["frame_type"] != "first_frame" || frameImages[1]["frame_type"] != "last_frame" {
		t.Fatalf("unexpected frame types: %#v", frameImages)
	}
}

func TestSeedanceBuildUpstreamRequestUsesExtraImagesAndVideosAsInputReferences(t *testing.T) {
	handler := &SeedanceHandler{BaseHandler: NewBaseHandler("seedance")}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "bytedance/seedance-2.0"}}
	body, err := handler.BuildUpstreamRequest(info, &relaycommon.TaskSubmitReq{
		Model:    "bytedance/seedance-2.0",
		Prompt:   "hello",
		Duration: 5,
		Images: []string{
			"https://example.com/first.png",
			"https://example.com/last.png",
			"https://example.com/ref.png",
		},
		Videos: []string{"https://example.com/ref.mp4"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	inputReferences, ok := body["input_references"].([]map[string]any)
	if !ok || len(inputReferences) != 2 {
		t.Fatalf("expected 2 input references, got %#v", body["input_references"])
	}
	if inputReferences[0]["type"] != "image" || inputReferences[1]["type"] != "video" {
		t.Fatalf("unexpected input references: %#v", inputReferences)
	}
}

func TestSeedanceBuildUpstreamRequestPreservesExplicitFrameImagesAndInputReferences(t *testing.T) {
	handler := &SeedanceHandler{BaseHandler: NewBaseHandler("seedance")}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "bytedance/seedance-2.0"}}
	body, err := handler.BuildUpstreamRequest(info, &relaycommon.TaskSubmitReq{
		Model:    "bytedance/seedance-2.0",
		Prompt:   "hello",
		Duration: 5,
		Images:   []string{"https://example.com/ignored.png"},
		Metadata: map[string]any{
			"frame_images": []map[string]any{
				{"frame_type": "first_frame", "image_url": map[string]any{"url": "https://example.com/explicit-first.png"}},
			},
			"input_references": []map[string]any{
				{"type": "audio", "url": "https://example.com/ref.mp3"},
			},
			"generate_audio": true,
			"aspect_ratio":   "16:9",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	frameImages := body["frame_images"].([]map[string]any)
	inputReferences := body["input_references"].([]map[string]any)
	if frameImages[0]["frame_type"] != "first_frame" {
		t.Fatalf("unexpected explicit frame images: %#v", frameImages)
	}
	if inputReferences[0]["type"] != "audio" {
		t.Fatalf("unexpected explicit input references: %#v", inputReferences)
	}
	if body["generate_audio"] != true {
		t.Fatalf("expected generate_audio true, got %#v", body["generate_audio"])
	}
	if body["aspect_ratio"] != "16:9" {
		t.Fatalf("expected aspect_ratio 16:9, got %#v", body["aspect_ratio"])
	}
}

func TestSeedanceEstimateBillingContextUsesNormalizedFields(t *testing.T) {
	handler := &SeedanceHandler{BaseHandler: NewBaseHandler("seedance")}
	ctx, err := handler.EstimateBillingContext(&relaycommon.TaskSubmitReq{
		Model:   "bytedance/seedance-2.0",
		Prompt:  "hello",
		Seconds: "8",
		Metadata: map[string]any{
			"resolution":     "720p",
			"generate_audio": true,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx.DurationSeconds != 8 {
		t.Fatalf("expected duration 8, got %d", ctx.DurationSeconds)
	}
	if ctx.ResolutionTier != "720p" {
		t.Fatalf("expected tier 720p, got %q", ctx.ResolutionTier)
	}
	if ctx.AudioEnabled == nil || !*ctx.AudioEnabled {
		t.Fatalf("expected audio enabled, got %#v", ctx.AudioEnabled)
	}
}

func TestSeedanceBuildUpstreamRequestPreserves4KAndPassthroughFields(t *testing.T) {
	handler := &SeedanceHandler{BaseHandler: NewBaseHandler("seedance")}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "bytedance/seedance-2.0"}}
	body, err := handler.BuildUpstreamRequest(info, &relaycommon.TaskSubmitReq{
		Model:   "bytedance/seedance-2.0",
		Prompt:  "hello",
		Seconds: "15",
		Metadata: map[string]any{
			"resolution":   "4K",
			"aspect_ratio": "21:9",
			"watermark":    true,
			"req_key":      "seedance_video_generation",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if body["duration"] != 15 {
		t.Fatalf("expected duration 15, got %#v", body["duration"])
	}
	if body["resolution"] != "4K" {
		t.Fatalf("expected resolution 4K, got %#v", body["resolution"])
	}
	if body["watermark"] != true {
		t.Fatalf("expected watermark passthrough, got %#v", body["watermark"])
	}
	if body["req_key"] != "seedance_video_generation" {
		t.Fatalf("expected req_key passthrough, got %#v", body["req_key"])
	}
}
