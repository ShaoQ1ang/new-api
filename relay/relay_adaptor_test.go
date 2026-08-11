package relay

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	openroutertask "github.com/QuantumNous/new-api/relay/channel/task/openrouter"
	"github.com/stretchr/testify/require"
)

func TestGetTaskAdaptorUsesOpenRouterVideoAdaptor(t *testing.T) {
	adaptor := GetTaskAdaptor(constant.TaskPlatform("20"))
	require.IsType(t, &openroutertask.TaskAdaptor{}, adaptor)
}
