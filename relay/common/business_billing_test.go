package common

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCoveredSynchronousBodyRejectsBackgroundAndKeepsNormalInputs(t *testing.T) {
	for _, input := range []string{`{"model":"test"}`, `{"background":false}`, `{"background":null}`} {
		require.NoError(t, ValidateCoveredSynchronousBody(strings.NewReader(input)))
	}
	for _, input := range []string{`{"background":true}`, `{"background":"true"}`, `{"background":false,"background":true}`} {
		require.Error(t, ValidateCoveredSynchronousBody(strings.NewReader(input)))
	}
}
