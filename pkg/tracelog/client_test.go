package tracelog

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDisabledClientHasNoSideEffects(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "traces")
	t.Setenv("TRACE_LOG_ENABLED", "false")
	t.Setenv("TRACE_LOG_DIR", directory)
	client := NewFromEnv("newapi")
	require.False(t, client.Enabled())

	called := false
	client.LogValue(context.Background(), "newapi.input", "server-request", func() any {
		called = true
		return map[string]string{"prompt": "unused"}
	})
	request, err := http.NewRequest(http.MethodPost, "http://upstream.example/v1/videos", strings.NewReader(`{"model":"test"}`))
	require.NoError(t, err)
	request.GetBody = func() (io.ReadCloser, error) {
		called = true
		return io.NopCloser(strings.NewReader(`{"model":"test"}`)), nil
	}
	client.LogHTTPRequest(context.Background(), "newapi.upstream.request", request)

	assert.False(t, called)
	assert.Empty(t, request.Header.Get(Header))
	_, err = os.Stat(directory)
	assert.True(t, os.IsNotExist(err))
}

func TestEnabledClientWritesRequestAndResponse(t *testing.T) {
	directory := t.TempDir()
	client := New("newapi", directory, 8)
	ctx := client.WithTraceID(context.Background(), "turn-test")
	request, err := http.NewRequest(http.MethodPost, "http://mock.example/v1/videos", strings.NewReader(`{"model":"video-test"}`))
	require.NoError(t, err)
	client.LogHTTPRequest(ctx, "newapi.upstream.request", request)
	client.LogHTTPResponse(ctx, "newapi.upstream.response", &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}}, []byte(`{"id":"task-test"}`))
	client.Close()

	assert.Equal(t, "turn-test", request.Header.Get(Header))
	content, err := os.ReadFile(filepath.Join(directory, "newapi.jsonl"))
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	require.Len(t, lines, 2)
	var event Event
	require.NoError(t, common.Unmarshal([]byte(lines[0]), &event))
	assert.Equal(t, "turn-test", event.TraceID)
	assert.Equal(t, `{"model":"video-test"}`, event.Request.Body)
}
