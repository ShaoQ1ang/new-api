package taskcommon_test

import (
	"testing"

	_ "github.com/QuantumNous/new-api/relay/channel/task/openrouter"
	"github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

func TestOpenRouterVideoBillingConverterResolvesTierAndAudio(t *testing.T) {
	req := relaycommon.TaskSubmitReq{
		Model:    "bytedance/seedance-2.0",
		Duration: 8,
		Size:     "720p",
		Metadata: map[string]any{
			"generate_audio": false,
		},
	}
	params, err := taskcommon.ConvertVideoBillingParams(nil, req)
	if err != nil {
		t.Fatalf("convert video billing params failed: %v", err)
	}
	if params.Tier != "720p" {
		t.Fatalf("expected 720p, got %s", params.Tier)
	}
	if params.DurationSeconds != 8 {
		t.Fatalf("expected duration 8, got %d", params.DurationSeconds)
	}
	if params.AudioEnabled {
		t.Fatal("expected audio disabled")
	}
}

func TestOpenRouterVeoBillingConverterNormalizesDuration(t *testing.T) {
	req := relaycommon.TaskSubmitReq{
		Model:   "google/veo-3.1-lite",
		Seconds: "7",
		Size:    "720p",
	}
	params, err := taskcommon.ConvertVideoBillingParams(nil, req)
	if err != nil {
		t.Fatalf("convert video billing params failed: %v", err)
	}
	if params.Tier != "720p" {
		t.Fatalf("expected 720p, got %s", params.Tier)
	}
	if params.DurationSeconds != 6 {
		t.Fatalf("expected normalized duration 6, got %d", params.DurationSeconds)
	}
}
