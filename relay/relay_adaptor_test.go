package relay

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	openroutertask "github.com/QuantumNous/new-api/relay/channel/task/openrouter"
)

func TestGetTaskAdaptor_OpenRouterUsesDedicatedTaskAdaptor(t *testing.T) {
	adaptor := GetTaskAdaptor(constant.TaskPlatform("20"))
	if adaptor == nil {
		t.Fatal("expected non-nil adaptor")
	}
	if _, ok := adaptor.(*openroutertask.TaskAdaptor); !ok {
		t.Fatalf("expected *openrouter.TaskAdaptor, got %T", adaptor)
	}
}
