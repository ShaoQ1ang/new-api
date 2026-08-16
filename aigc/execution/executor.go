package execution

import (
	"context"

	"github.com/QuantumNous/new-api/aigc/dto"
)

type Identity struct {
	UserID  int
	TokenID int
	Group   string
}

type Spec struct {
	PublicModelID   string                `json:"public_model_id"`
	UpstreamModelID string                `json:"upstream_model_id"`
	ModelType       string                `json:"model_type"`
	Mode            string                `json:"mode"`
	Adapter         string                `json:"adapter,omitempty"`
	TaskProtocol    string                `json:"task_protocol,omitempty"`
	OutputSpecID    string                `json:"output_spec_id,omitempty"`
	ConfigVersion   int                   `json:"config_version"`
	Request         dto.GenerationRequest `json:"request"`
}

type Result struct {
	Status       string                     `json:"status"`
	Progress     int                        `json:"progress"`
	NativeTaskID string                     `json:"native_task_id,omitempty"`
	Outputs      []dto.GenerationOutputItem `json:"outputs"`
	Usage        *dto.GenerationUsage       `json:"usage,omitempty"`
	ErrorCode    string                     `json:"error_code,omitempty"`
	ErrorMessage string                     `json:"error_message,omitempty"`
	Retryable    bool                       `json:"retryable,omitempty"`
}

type Error struct {
	HTTPStatus int
	Code       string
	Message    string
	Retryable  bool
}

func (err *Error) Error() string { return err.Message }

type Executor interface {
	Execute(ctx context.Context, identity Identity, spec Spec) (Result, error)
	Poll(ctx context.Context, identity Identity, modelType, nativeTaskID string) (Result, error)
}
