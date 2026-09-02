package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
)

// mockWriter implements logWriter for testing without a real Kafka broker.
type mockWriter struct {
	mu         sync.Mutex
	writeErr   error
	pingErr    error
	closeErr   error
	writeCount int
	gotMsgs    []kafka.Message
	onWrite    func()
}

func (m *mockWriter) WriteMessages(ctx context.Context, msgs ...kafka.Message) error {
	m.mu.Lock()
	m.writeCount++
	writeErr := m.writeErr
	onWrite := m.onWrite
	if len(msgs) > 0 {
		m.gotMsgs = append(m.gotMsgs, msgs...)
	}
	m.mu.Unlock()

	if onWrite != nil {
		onWrite()
	}

	if writeErr != nil {
		if errors.Is(writeErr, context.DeadlineExceeded) {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
		}
		return writeErr
	}
	return nil
}

func (m *mockWriter) Ping(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.pingErr
}

func (m *mockWriter) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.closeErr
}

func (m *mockWriter) getWriteCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.writeCount
}

func (m *mockWriter) getMsgs() []kafka.Message {
	m.mu.Lock()
	defer m.mu.Unlock()
	dst := make([]kafka.Message, len(m.gotMsgs))
	copy(dst, m.gotMsgs)
	return dst
}

func TestHandleIngest_Success(t *testing.T) {
	mw := &mockWriter{}
	srv := NewServer(mw)
	defer srv.Shutdown(context.Background())

	body := `{"service":"test-svc","level":"info","message":"hello world"}`
	req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202 Accepted, got %d — body: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["status"] != "accepted" {
		t.Errorf("expected status 'accepted', got %q", resp["status"])
	}
}

func TestHandleIngest_InvalidJSON(t *testing.T) {
	mw := &mockWriter{}
	srv := NewServer(mw)
	defer srv.Shutdown(context.Background())

	body := `not json at all`
	req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestHandleIngest_MissingService(t *testing.T) {
	mw := &mockWriter{}
	srv := NewServer(mw)
	defer srv.Shutdown(context.Background())

	body := `{"level":"info","message":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}

	var resp map[string]string
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["error"] != "service is required" {
		t.Errorf("expected 'service is required', got %q", resp["error"])
	}
}

func TestHandleIngest_MissingLevel(t *testing.T) {
	mw := &mockWriter{}
	srv := NewServer(mw)
	defer srv.Shutdown(context.Background())

	body := `{"service":"test-svc","message":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}

	var resp map[string]string
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["error"] != "level is required" {
		t.Errorf("expected 'level is required', got %q", resp["error"])
	}
}

func TestHandleIngest_QueueFull_429(t *testing.T) {
	t.Setenv("INGEST_QUEUE_CAPACITY", "1")
	t.Setenv("INGEST_WORKERS", "1")

	blockCh := make(chan struct{})
	blockingWriter := &mockWriter{
		onWrite: func() {
			<-blockCh
		},
	}
	srv := NewServer(blockingWriter)
	defer func() {
		close(blockCh)
		srv.Shutdown(context.Background())
	}()

	body := `{"service":"test-svc","level":"info","message":"hello"}`

	// 1st request: worker consumes and blocks on WriteMessages
	req1 := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(body))
	req1.Header.Set("Content-Type", "application/json")
	rec1 := httptest.NewRecorder()
	srv.router.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusAccepted {
		t.Fatalf("expected 202 on req1, got %d", rec1.Code)
	}

	// Wait until worker is blocked inside onWrite
	for i := 0; i < 50; i++ {
		if blockingWriter.getWriteCount() > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// 2nd request: fills queue of capacity 1
	req2 := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	srv.router.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusAccepted {
		t.Fatalf("expected 202 on req2 (queue slot 1), got %d", rec2.Code)
	}

	// 3rd request: queue is 100% full, returns 429
	req3 := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(body))
	req3.Header.Set("Content-Type", "application/json")
	rec3 := httptest.NewRecorder()
	srv.router.ServeHTTP(rec3, req3)

	if rec3.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 Too Many Requests on full queue, got %d", rec3.Code)
	}

	var resp map[string]string
	json.Unmarshal(rec3.Body.Bytes(), &resp)
	if resp["error"] != "ingestion queue full, retry later" {
		t.Errorf("expected 'ingestion queue full, retry later', got %q", resp["error"])
	}
}

func TestWorkerCountConfigValidation(t *testing.T) {
	tests := []struct {
		envVal   string
		expected int
	}{
		{"0", defaultWorkerCount},
		{"-1", defaultWorkerCount},
		{"invalid", defaultWorkerCount},
		{"2", 2},
	}

	for _, tt := range tests {
		t.Setenv("INGEST_WORKERS", tt.envVal)
		count := getWorkerCount()
		if count != tt.expected {
			t.Errorf("INGEST_WORKERS=%q: expected worker count %d, got %d", tt.envVal, tt.expected, count)
		}
	}
}

func TestAsyncIngestion_EndToEnd(t *testing.T) {
	mw := &mockWriter{}
	srv := NewServer(mw)
	defer srv.Shutdown(context.Background())

	body := `{"service":"e2e-svc","level":"warn","message":"end to end test"}`
	req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202 Accepted, got %d", rec.Code)
	}

	// Wait deterministically for worker loop to process item
	var msg kafka.Message
	for i := 0; i < 50; i++ {
		msgs := mw.getMsgs()
		if len(msgs) > 0 {
			msg = msgs[0]
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if string(msg.Key) != "e2e-svc" {
		t.Fatalf("expected message key 'e2e-svc', got %q", string(msg.Key))
	}
	if !strings.Contains(string(msg.Value), "end to end test") {
		t.Errorf("expected message value to contain 'end to end test', got %s", string(msg.Value))
	}
}

func TestWorker_RetryExhaustion_DLQSuccess(t *testing.T) {
	mainWriter := &mockWriter{writeErr: errors.New("kafka connection failure")}
	dlqWriter := &mockWriter{}

	srv := NewServer(mainWriter, dlqWriter)

	var delays []time.Duration
	var delaysMu sync.Mutex
	srv.sleepFn = func(d time.Duration) {
		delaysMu.Lock()
		delays = append(delays, d)
		delaysMu.Unlock()
	}

	job := LogJob{
		Entry: LogEntry{Service: "test-svc", Level: "error", Message: "broker down", Timestamp: 1000},
		Raw:   []byte(`{"service":"test-svc","level":"error","message":"broker down"}`),
	}

	srv.processJob(job)

	// Verify total 4 attempts on main writer (1 initial + 3 retries)
	if mainWriter.getWriteCount() != 4 {
		t.Errorf("expected 4 total write attempts (1 initial + 3 retries), got %d", mainWriter.getWriteCount())
	}

	// Verify backoff delays: 100ms, 200ms, 400ms
	delaysMu.Lock()
	defer delaysMu.Unlock()
	expectedDelays := []time.Duration{100 * time.Millisecond, 200 * time.Millisecond, 400 * time.Millisecond}
	if len(delays) != len(expectedDelays) {
		t.Fatalf("expected %d backoff delays, got %d", len(expectedDelays), len(delays))
	}
	for i, d := range delays {
		if d != expectedDelays[i] {
			t.Errorf("delay %d: expected %v, got %v", i, expectedDelays[i], d)
		}
	}

	// Verify DLQ write succeeded
	dlqMsgs := dlqWriter.getMsgs()
	if len(dlqMsgs) != 1 {
		t.Fatalf("expected 1 message in DLQ, got %d", len(dlqMsgs))
	}
	if string(dlqMsgs[0].Key) != "test-svc" {
		t.Errorf("expected DLQ key 'test-svc', got %q", string(dlqMsgs[0].Key))
	}

	var payload DLQPayload
	if err := json.Unmarshal(dlqMsgs[0].Value, &payload); err != nil {
		t.Fatalf("failed to unmarshal DLQ payload: %v", err)
	}
	if payload.RetryCount != 3 {
		t.Errorf("expected DLQ retry_count 3, got %d", payload.RetryCount)
	}
	if payload.OriginalTopic != "raw-logs" {
		t.Errorf("expected original_topic 'raw-logs', got %q", payload.OriginalTopic)
	}
}

func TestWorker_DLQFailureHandled(t *testing.T) {
	mainWriter := &mockWriter{writeErr: errors.New("kafka down")}
	dlqWriter := &mockWriter{writeErr: errors.New("dlq broker also down")}

	srv := NewServer(mainWriter, dlqWriter)
	srv.sleepFn = func(d time.Duration) {}

	job := LogJob{
		Entry: LogEntry{Service: "test-svc", Level: "error", Message: "unreachable"},
		Raw:   []byte(`{"service":"test-svc","level":"error","message":"unreachable"}`),
	}

	srv.processJob(job)

	if mainWriter.getWriteCount() != 4 {
		t.Errorf("expected 4 main writer attempts, got %d", mainWriter.getWriteCount())
	}
	if dlqWriter.getWriteCount() != 1 {
		t.Errorf("expected 1 DLQ write attempt, got %d", dlqWriter.getWriteCount())
	}
}

func TestHandleIngest_RequestBodyTooLarge(t *testing.T) {
	mw := &mockWriter{}
	srv := NewServer(mw)
	defer srv.Shutdown(context.Background())

	largeMsg := strings.Repeat("x", defaultMaxBodySize+1024)
	body := fmt.Sprintf(`{"service":"test-svc","level":"info","message":"%s"}`, largeMsg)
	req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d — body length: %d", rec.Code, len(body))
	}
}

func TestHealthz_Healthy(t *testing.T) {
	mw := &mockWriter{}
	srv := NewServer(mw)
	defer srv.Shutdown(context.Background())

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("expected status 'ok', got %q", resp["status"])
	}
}

func TestHealthz_Unhealthy(t *testing.T) {
	mw := &mockWriter{pingErr: errors.New("broker unavailable")}
	srv := NewServer(mw)
	defer srv.Shutdown(context.Background())

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

func TestHealthz_NilWriter(t *testing.T) {
	srv := NewServer(nil)
	defer srv.Shutdown(context.Background())

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

func TestTimestampAutoFill(t *testing.T) {
	mw := &mockWriter{}
	srv := NewServer(mw)
	defer srv.Shutdown(context.Background())

	body := `{"service":"test-svc","level":"info","message":"no timestamp"}`
	req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rec.Code)
	}

	var msg kafka.Message
	for i := 0; i < 50; i++ {
		msgs := mw.getMsgs()
		if len(msgs) > 0 {
			msg = msgs[0]
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if string(msg.Key) != "test-svc" {
		t.Fatalf("expected key 'test-svc', got %q", string(msg.Key))
	}

	var entry LogEntry
	if err := json.Unmarshal(msg.Value, &entry); err != nil {
		t.Fatalf("failed to unmarshal message: %v", err)
	}
	if entry.Timestamp == 0 {
		t.Error("expected non-zero timestamp auto-filled")
	}
}

func TestShutdown_NoSendOnClosedChannel(t *testing.T) {
	mw := &mockWriter{}
	srv := NewServer(mw)

	// Shutdown server
	if err := srv.Shutdown(context.Background()); err != nil {
		t.Fatalf("unexpected error during shutdown: %v", err)
	}

	// Incoming HTTP request during/after shutdown returns 503 Service Unavailable without panic
	body := `{"service":"test-svc","level":"info","message":"during shutdown"}`
	req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 Service Unavailable after shutdown, got %d", rec.Code)
	}
}
