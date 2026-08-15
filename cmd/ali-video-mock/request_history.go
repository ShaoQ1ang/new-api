package main

import (
	"bytes"
	_ "embed"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/pkg/tracelog"
)

const mockHistoryLimit = 500

//go:embed history.html
var mockHistoryPage []byte

type mockRequestRecord struct {
	ID            int64               `json:"id"`
	ReceivedAt    time.Time           `json:"received_at"`
	DurationMS    int64               `json:"duration_ms"`
	Method        string              `json:"method"`
	Path          string              `json:"path"`
	RawQuery      string              `json:"raw_query"`
	Query         map[string][]string `json:"query"`
	Headers       http.Header         `json:"headers"`
	Body          string              `json:"body"`
	ContentLength int64               `json:"content_length"`
	RemoteAddr    string              `json:"remote_addr"`
	Host          string              `json:"host"`
	Protocol      string              `json:"protocol"`
	Status        int                 `json:"status"`
}

type mockHistoryResponse struct {
	Records  []mockRequestRecord `json:"records"`
	Count    int                 `json:"count"`
	Capacity int                 `json:"capacity"`
}

type historyResponseWriter struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
}

func (w *historyResponseWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *historyResponseWriter) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	_, _ = w.body.Write(body)
	return w.ResponseWriter.Write(body)
}

func (s *mockServer) captureRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !shouldCaptureMockRequest(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		startedAt := time.Now()
		traceContext := tracelog.Default.WithTraceID(r.Context(), r.Header.Get(tracelog.Header))
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "read request body: "+err.Error(), http.StatusBadRequest)
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		tracelog.Default.LogHTTPServerRequest(traceContext, "mock.input", r, body)
		response := &historyResponseWriter{ResponseWriter: w}
		next.ServeHTTP(response, r)
		if response.status == 0 {
			response.status = http.StatusOK
		}
		tracelog.Default.LogHTTPServerResponse(traceContext, "mock.output", response.status, response.Header(), response.body.Bytes())

		record := mockRequestRecord{
			ReceivedAt:    startedAt,
			DurationMS:    time.Since(startedAt).Milliseconds(),
			Method:        r.Method,
			Path:          r.URL.Path,
			RawQuery:      r.URL.RawQuery,
			Query:         r.URL.Query(),
			Headers:       r.Header.Clone(),
			Body:          string(body),
			ContentLength: int64(len(body)),
			RemoteAddr:    r.RemoteAddr,
			Host:          r.Host,
			Protocol:      mockProtocolForPath(r.URL.Path),
			Status:        response.status,
		}
		s.storeRequestRecord(record)
	})
}

func shouldCaptureMockRequest(requestPath string) bool {
	return requestPath != "/history" &&
		requestPath != "/api/mock/history" &&
		requestPath != "/favicon.ico" &&
		requestPath != "/healthz" &&
		!strings.HasPrefix(requestPath, "/mock-assets/videos/")
}

func mockProtocolForPath(requestPath string) string {
	switch {
	case strings.HasPrefix(requestPath, "/api/v1/services/aigc/"), strings.HasPrefix(requestPath, "/api/v1/tasks/"):
		return "Alibaba"
	case strings.HasPrefix(requestPath, "/api/v3/contents/generations/tasks"):
		return "Seedance"
	case strings.HasPrefix(requestPath, "/v1/videos"):
		return "OpenRouter"
	case strings.Contains(requestPath, "/projects/"):
		return "Vertex AI"
	case strings.Contains(requestPath, "/models/veo-"):
		return "Gemini"
	default:
		return "Other"
	}
}

func (s *mockServer) storeRequestRecord(record mockRequestRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextHistoryID++
	record.ID = s.nextHistoryID
	if len(s.history) == mockHistoryLimit {
		copy(s.history, s.history[1:])
		s.history[len(s.history)-1] = record
		return
	}
	s.history = append(s.history, record)
}

func (s *mockServer) handleHistoryPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(mockHistoryPage)
}

func (s *mockServer) handleRequestHistory(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.mu.Lock()
		records := make([]mockRequestRecord, len(s.history))
		for index := range s.history {
			records[len(s.history)-1-index] = s.history[index]
		}
		s.mu.Unlock()
		writeJSON(w, http.StatusOK, mockHistoryResponse{Records: records, Count: len(records), Capacity: mockHistoryLimit})
	case http.MethodDelete:
		s.mu.Lock()
		s.history = s.history[:0]
		s.mu.Unlock()
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
