package common

import (
	"fmt"
	"io"

	"github.com/QuantumNous/new-api/common"
)

// ValidateCoveredSynchronousBody checks the body actually sent upstream,
// including passthrough bodies and channel overrides, not just a lossy DTO.
func ValidateCoveredSynchronousBody(reader io.Reader) error {
	var mode struct {
		Background *bool `json:"background"`
	}
	if err := common.DecodeJson(reader, &mode); err != nil {
		return fmt.Errorf("invalid covered model request mode")
	}
	if mode.Background != nil && *mode.Background {
		return fmt.Errorf("background model requests are unsupported for business billing")
	}
	return nil
}
