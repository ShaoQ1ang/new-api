package tracelog

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
)

const Header = "X-Trace-ID"

type contextKey struct{}

type Payload struct {
	Method  string              `json:"method,omitempty"`
	URL     string              `json:"url,omitempty"`
	Status  int                 `json:"status,omitempty"`
	Headers map[string][]string `json:"headers,omitempty"`
	Body    string              `json:"body,omitempty"`
}

type Event struct {
	ID         string    `json:"id"`
	TraceID    string    `json:"trace_id"`
	SpanID     string    `json:"span_id,omitempty"`
	Service    string    `json:"service"`
	Name       string    `json:"name"`
	Kind       string    `json:"kind,omitempty"`
	Status     string    `json:"status"`
	StartedAt  time.Time `json:"started_at"`
	DurationMS int64     `json:"duration_ms,omitempty"`
	Request    Payload   `json:"request,omitempty"`
	Response   Payload   `json:"response,omitempty"`
	Error      string    `json:"error,omitempty"`
}

type Client struct {
	enabled bool
	service string
	events  chan Event
	stop    chan struct{}
	done    chan struct{}
	once    sync.Once
}

var Default = &Client{service: "newapi"}

func InitDefaultFromEnv(service string) {
	Default = NewFromEnv(service)
}

func NewFromEnv(service string) *Client {
	enabled := strings.EqualFold(strings.TrimSpace(os.Getenv("TRACE_LOG_ENABLED")), "true") || strings.TrimSpace(os.Getenv("TRACE_LOG_ENABLED")) == "1"
	if !enabled {
		return &Client{service: service}
	}
	directory := strings.TrimSpace(os.Getenv("TRACE_LOG_DIR"))
	if directory == "" {
		directory = "/tmp/aigc-traces"
	}
	bufferSize := 256
	if raw := strings.TrimSpace(os.Getenv("TRACE_LOG_BUFFER_SIZE")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			bufferSize = parsed
		}
	}
	return New(service, directory, bufferSize)
}

func New(service, directory string, bufferSize int) *Client {
	client := &Client{service: service}
	if bufferSize <= 0 {
		bufferSize = 256
	}
	if err := os.MkdirAll(directory, 0o755); err != nil {
		log.Printf("Trace Log 创建目录失败，采集已关闭: %v", err)
		return client
	}
	file, err := os.OpenFile(filepath.Join(directory, service+".jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		log.Printf("Trace Log 打开文件失败，采集已关闭: %v", err)
		return client
	}
	client.enabled = true
	client.events = make(chan Event, bufferSize)
	client.stop = make(chan struct{})
	client.done = make(chan struct{})
	go client.writeLoop(file)
	return client
}

func (client *Client) Enabled() bool { return client != nil && client.enabled }

func (client *Client) WithTraceID(ctx context.Context, traceID string) context.Context {
	if !client.Enabled() || strings.TrimSpace(traceID) == "" {
		return ctx
	}
	return context.WithValue(ctx, contextKey{}, strings.TrimSpace(traceID))
}

func (client *Client) InjectTraceID(ctx context.Context, headers http.Header) {
	if !client.Enabled() || headers == nil {
		return
	}
	if traceID := traceIDFromContext(ctx); traceID != "" {
		headers.Set(Header, traceID)
	}
}

func (client *Client) LogValue(ctx context.Context, name, kind string, value func() any) {
	if !client.Enabled() || value == nil {
		return
	}
	body, err := common.Marshal(value())
	event := client.newEvent(ctx, name, kind)
	if err != nil {
		event.Status = "error"
		event.Error = err.Error()
	} else {
		event.Request.Body = string(body)
	}
	client.emit(event)
}

func (client *Client) LogResponseValue(ctx context.Context, name, kind string, value func() any) {
	if !client.Enabled() || value == nil {
		return
	}
	body, err := common.Marshal(value())
	event := client.newEvent(ctx, name, kind)
	if err != nil {
		event.Status = "error"
		event.Error = err.Error()
	} else {
		event.Response.Body = string(body)
	}
	client.emit(event)
}

func (client *Client) LogHTTPRequest(ctx context.Context, name string, request *http.Request) {
	if !client.Enabled() || request == nil {
		return
	}
	client.InjectTraceID(ctx, request.Header)
	event := client.newEvent(ctx, name, "client-request")
	event.Request = Payload{Method: request.Method, URL: request.URL.String(), Headers: request.Header.Clone(), Body: requestBody(request)}
	client.emit(event)
}

func (client *Client) LogHTTPServerRequest(ctx context.Context, name string, request *http.Request, body []byte) {
	if !client.Enabled() || request == nil {
		return
	}
	event := client.newEvent(ctx, name, "server-request")
	event.Request = Payload{Method: request.Method, URL: request.URL.String(), Headers: request.Header.Clone(), Body: encodeBody(body)}
	client.emit(event)
}

func (client *Client) LogHTTPResponse(ctx context.Context, name string, response *http.Response, body []byte) {
	if !client.Enabled() || response == nil {
		return
	}
	event := client.newEvent(ctx, name, "client-response")
	event.Response = Payload{Status: response.StatusCode, Headers: response.Header.Clone(), Body: encodeBody(body)}
	if response.StatusCode >= http.StatusBadRequest {
		event.Status = "error"
	}
	client.emit(event)
}

func (client *Client) LogHTTPServerResponse(ctx context.Context, name string, status int, headers http.Header, body []byte) {
	if !client.Enabled() {
		return
	}
	event := client.newEvent(ctx, name, "server-response")
	event.Response = Payload{Status: status, Headers: headers.Clone(), Body: encodeBody(body)}
	if status >= http.StatusBadRequest {
		event.Status = "error"
	}
	client.emit(event)
}

func (client *Client) Close() {
	if !client.Enabled() {
		return
	}
	client.once.Do(func() {
		close(client.stop)
		<-client.done
	})
}

func (client *Client) newEvent(ctx context.Context, name, kind string) Event {
	return Event{ID: newID(), TraceID: traceIDFromContext(ctx), SpanID: newID(), Service: client.service, Name: name, Kind: kind, Status: "ok", StartedAt: time.Now()}
}

func (client *Client) emit(event Event) {
	if event.TraceID == "" {
		return
	}
	select {
	case client.events <- event:
	default:
	}
}

func (client *Client) writeLoop(file *os.File) {
	defer close(client.done)
	defer file.Close()
	writer := bufio.NewWriterSize(file, 64*1024)
	write := func(event Event) {
		if encoded, err := common.Marshal(event); err == nil {
			_, _ = writer.Write(encoded)
			_ = writer.WriteByte('\n')
		}
	}
	for {
		select {
		case event := <-client.events:
			write(event)
			_ = writer.Flush()
		case <-client.stop:
			for {
				select {
				case event := <-client.events:
					write(event)
				default:
					_ = writer.Flush()
					return
				}
			}
		}
	}
}

func traceIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	traceID, _ := ctx.Value(contextKey{}).(string)
	return strings.TrimSpace(traceID)
}

func requestBody(request *http.Request) string {
	if request.GetBody == nil {
		return ""
	}
	body, err := request.GetBody()
	if err != nil {
		return ""
	}
	defer body.Close()
	content, err := io.ReadAll(body)
	if err != nil {
		return ""
	}
	return encodeBody(content)
}

func encodeBody(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	if utf8.Valid(body) {
		return string(body)
	}
	return "base64:" + base64.StdEncoding.EncodeToString(body)
}

func newID() string {
	random := make([]byte, 8)
	if _, err := rand.Read(random); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 16)
	}
	return hex.EncodeToString(random)
}
