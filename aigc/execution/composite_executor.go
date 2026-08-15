package execution

import (
	"context"
	"net/http"
	"strings"
)

type CompositeExecutor struct {
	sync Executor
	task Executor
}

func NewCompositeExecutor(syncExecutor, taskExecutor Executor) *CompositeExecutor {
	return &CompositeExecutor{sync: syncExecutor, task: taskExecutor}
}

func (executor *CompositeExecutor) Execute(ctx context.Context, identity Identity, spec Spec) (Result, error) {
	switch strings.TrimSpace(spec.ModelType) {
	case "text", "image":
		return executor.sync.Execute(ctx, identity, spec)
	case "video", "music":
		return executor.task.Execute(ctx, identity, spec)
	default:
		return Result{}, executionError(http.StatusBadRequest, "AIGC_MODEL_TYPE_NOT_SUPPORTED", "AIGC model type is not supported", false)
	}
}

func (executor *CompositeExecutor) Poll(ctx context.Context, identity Identity, spec Spec, nativeTaskID string) (Result, error) {
	switch strings.TrimSpace(spec.ModelType) {
	case "text", "image":
		return Result{}, executionError(http.StatusBadRequest, "AIGC_SYNC_POLL_NOT_SUPPORTED", "synchronous AIGC generation cannot be polled", false)
	case "video", "music":
		return executor.task.Poll(ctx, identity, spec, nativeTaskID)
	default:
		return Result{}, executionError(http.StatusBadRequest, "AIGC_MODEL_TYPE_NOT_SUPPORTED", "AIGC model type is not supported", false)
	}
}
