package execution

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type executorStub struct {
	result   Result
	err      error
	executes int
	polls    int
}

func (stub *executorStub) Execute(context.Context, Identity, Spec) (Result, error) {
	stub.executes++
	return stub.result, stub.err
}

func (stub *executorStub) Poll(context.Context, Identity, Spec, string) (Result, error) {
	stub.polls++
	return stub.result, stub.err
}

func TestCompositeExecutorRoutesSyncAndTaskModelTypes(t *testing.T) {
	sync := &executorStub{result: Result{Status: "completed"}}
	task := &executorStub{result: Result{Status: "queued"}}
	executor := NewCompositeExecutor(sync, task)

	for _, modelType := range []string{"text", "image"} {
		result, err := executor.Execute(context.Background(), Identity{}, Spec{ModelType: modelType})
		require.NoError(t, err)
		assert.Equal(t, "completed", result.Status)
	}
	for _, modelType := range []string{"video", "music"} {
		result, err := executor.Execute(context.Background(), Identity{}, Spec{ModelType: modelType})
		require.NoError(t, err)
		assert.Equal(t, "queued", result.Status)
	}
	assert.Equal(t, 2, sync.executes)
	assert.Equal(t, 2, task.executes)
}

func TestCompositeExecutorRejectsPollingSyncGeneration(t *testing.T) {
	executor := NewCompositeExecutor(&executorStub{}, &executorStub{})

	_, err := executor.Poll(context.Background(), Identity{}, Spec{ModelType: "image"}, "native")

	protocolErr, ok := err.(*Error)
	require.True(t, ok)
	assert.Equal(t, "AIGC_SYNC_POLL_NOT_SUPPORTED", protocolErr.Code)
}
