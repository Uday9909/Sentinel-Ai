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
}

func (m *mockWriter) WriteMessages(ctx context.Context, msgs ...kafka.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.writeCount++
	if m.writeErr != nil {
		if errors.Is(m.writeErr, context.DeadlineExceeded) {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
		}
		return m.writeErr
	}
	if len(msgs) > 0 {
		m.gotMsgs = append(m.gotMsgs, msgs...)
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
	t.Setenv("INGEST_WORKERS", "0")
	mw := &mockWriter{}
	srv := NewServer(mw)
	defer srv.Shutdown(context.Background())

	body := `{"service":"test-svc","level":"info","message":"hello"}`

	// Saturate queue of capacity 1
	req1 := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(body))
	req1.Header.Set("Content-Type", "application/json")
	rec1 := httptest.NewRecorder()
	srv.router.ServeHTTP(rec1, req1)

	// Second request when queue is full returns 429
	req2 := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	srv.router.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 Too Many Requests on full queue, got %d", rec2.Code)
	}

	var resp map[string]string
	json.Unmarshal(rec2.Body.Bytes(), &resp)
	if resp["error"] != "ingestion queue full, retry later" {
		t.Errorf("expected 'ingestion queue full, retry later', got %q", resp["error"])
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

	// Should process retries and attempt DLQ write without crashing or deadlocking
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
