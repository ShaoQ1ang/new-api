package openrouter

import (
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

func TestVeoBuildUpstreamRequestNormalizesDurationAndFrameImages(t *testing.T) {
	handler := &VeoHandler{BaseHandler: NewBaseHandler("veo")}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "google/veo-3.1-lite"}}
	body, err := handler.BuildUpstreamRequest(info, &relaycommon.TaskSubmitReq{
		Model:   "google/veo-3.1-lite",
		Prompt:  "hello",
		Seconds: "5",
		Images:  []string{"https://example.com/first.png", "https://example.com/last.png"},
		Metadata: map[string]any{
			"resolution":   "1080p",
			"aspect_ratio": "16:9",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if body["duration"] != 4 {
		t.Fatalf("expected normalized duration 4, got %#v", body["duration"])
	}
	frameImages, ok := body["frame_images"].([]map[string]any)
	if !ok || len(frameImages) != 2 {
		t.Fatalf("expected 2 frame images, got %#v", body["frame_images"])
	}
	if frameImages[0]["frame_type"] != "first_frame" || frameImages[1]["frame_type"] != "last_frame" {
		t.Fatalf("unexpected frame types %#v", frameImages)
	}
}

func TestVeoBuildUpstreamRequestUsesExtraImagesAsInputReferences(t *testing.T) {
	handler := &VeoHandler{BaseHandler: NewBaseHandler("veo")}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "google/veo-3.1-lite"}}
	body, err := handler.BuildUpstreamRequest(info, &relaycommon.TaskSubmitReq{
		Model:    "google/veo-3.1-lite",
		Prompt:   "hello",
		Duration: 8,
		Images: []string{
			"https://example.com/first.png",
			"https://example.com/last.png",
			"https://example.com/ref-1.png",
			"https://example.com/ref-2.png",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	inputReferences, ok := body["input_references"].([]map[string]any)
	if !ok || len(inputReferences) != 2 {
		t.Fatalf("expected 2 input references, got %#v", body["input_references"])
	}
	if inputReferences[0]["type"] != "image" || inputReferences[1]["type"] != "image" {
		t.Fatalf("unexpected input references %#v", inputReferences)
	}
}

func TestVeoEstimateBillingContextUsesNormalizedDuration(t *testing.T) {
	handler := &VeoHandler{BaseHandler: NewBaseHandler("veo")}
	ctx, err := handler.EstimateBillingContext(&relaycommon.TaskSubmitReq{
		Model:   "google/veo-3.1-lite",
		Prompt:  "hello",
		Seconds: "7",
		Metadata: map[string]any{
			"resolution": "720p",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx.DurationSeconds != 6 {
		t.Fatalf("expected duration 6, got %d", ctx.DurationSeconds)
	}
	if ctx.ResolutionTier != "720p" {
		t.Fatalf("expected 720p, got %q", ctx.ResolutionTier)
	}
}

func TestVeoBuildUpstreamRequestRejectsUnsupportedAspectRatio(t *testing.T) {
	handler := &VeoHandler{BaseHandler: NewBaseHandler("veo")}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "google/veo-3.1-lite"}}
	_, err := handler.BuildUpstreamRequest(info, &relaycommon.TaskSubmitReq{
		Model:    "google/veo-3.1-lite",
		Prompt:   "hello",
		Duration: 8,
		Metadata: map[string]any{
			"aspect_ratio": "1:1",
		},
	})
	if err == nil {
		t.Fatal("expected unsupported aspect ratio error")
	}
}

func TestVeoBuildUpstreamRequestRejectsUnsupportedResolution(t *testing.T) {
	handler := &VeoHandler{BaseHandler: NewBaseHandler("veo")}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "google/veo-3.1-lite"}}
	_, err := handler.BuildUpstreamRequest(info, &relaycommon.TaskSubmitReq{
		Model:    "google/veo-3.1-lite",
		Prompt:   "hello",
		Duration: 8,
		Metadata: map[string]any{
			"resolution": "4K",
		},
	})
	if err == nil {
		t.Fatal("expected unsupported resolution error")
	}
}
