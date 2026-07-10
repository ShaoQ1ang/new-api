package openrouter

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

func TestSelectHandler_PicksSoraFamilyHandler(t *testing.T) {
	handler := SelectHandler("openai/sora")
	if handler == nil {
		t.Fatal("expected handler")
	}
	if handler.Family() != "sora" {
		t.Fatalf("expected sora handler, got %q", handler.Family())
	}
}

func TestSelectHandler_PicksSeedanceFamilyHandler(t *testing.T) {
	handler := SelectHandler("bytedance/seedance-2.0")
	if handler == nil {
		t.Fatal("expected handler")
	}
	if handler.Family() != "seedance" {
		t.Fatalf("expected seedance handler, got %q", handler.Family())
	}
}

func TestSelectHandler_PicksVeoFamilyHandler(t *testing.T) {
	handler := SelectHandler("google/veo-3.1-lite")
	if handler == nil {
		t.Fatal("expected handler")
	}
	if handler.Family() != "veo" {
		t.Fatalf("expected veo handler, got %q", handler.Family())
	}
}

func TestSelectHandler_FallsBackToDefaultHandler(t *testing.T) {
	handler := SelectHandler("unknown/video-model")
	if handler == nil {
		t.Fatal("expected handler")
	}
	if handler.Family() != "default" {
		t.Fatalf("expected default handler, got %q", handler.Family())
	}
}

func TestDefaultHandlerEstimateBillingContext(t *testing.T) {
	handler := SelectHandler("unknown/video-model")
	ctx, err := handler.EstimateBillingContext(&relaycommon.TaskSubmitReq{
		Model:    "unknown/video-model",
		Duration: 6,
		Size:     "1080p",
		Metadata: map[string]any{
			"audio": true,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx.DurationSeconds != 6 {
		t.Fatalf("expected duration 6, got %d", ctx.DurationSeconds)
	}
	if ctx.ResolutionTier != "1080p" {
		t.Fatalf("expected 1080p, got %q", ctx.ResolutionTier)
	}
	if ctx.AudioEnabled == nil || !*ctx.AudioEnabled {
		t.Fatalf("expected audio enabled, got %+v", ctx.AudioEnabled)
	}
}

type parseFetchStubHandler struct {
	BaseHandler
	family string
	match  string
	status string
}

func (h *parseFetchStubHandler) Family() string {
	return h.family
}

func (h *parseFetchStubHandler) Match(model string) bool {
	return model == h.match
}

func (h *parseFetchStubHandler) ParseFetchResponse(_ *relaycommon.RelayInfo, _ []byte) (*relaycommon.TaskInfo, error) {
	return &relaycommon.TaskInfo{Status: h.status}, nil
}

func TestTaskAdaptorParseTaskResultForModelUsesMatchedHandler(t *testing.T) {
	originalHandlers := registeredHandlers
	registeredHandlers = []ModelHandler{
		&parseFetchStubHandler{BaseHandler: NewBaseHandler("seedance"), family: "seedance", match: "bytedance/seedance-2.0", status: string(model.TaskStatusSuccess)},
		&DefaultHandler{BaseHandler: NewBaseHandler("default")},
	}
	t.Cleanup(func() {
		registeredHandlers = originalHandlers
	})

	adaptor := &TaskAdaptor{}
	taskInfo, err := adaptor.ParseTaskResultForModel("bytedance/seedance-2.0", []byte(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if taskInfo.Status != string(model.TaskStatusSuccess) {
		t.Fatalf("expected success status from matched handler, got %q", taskInfo.Status)
	}
}
