package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
)

// mockWriter implements logWriter for testing without a real Kafka broker.
type mockWriter struct {
	writeErr error
	closeErr error
	gotMsg   *kafka.Message
}

func (m *mockWriter) WriteMessages(ctx context.Context, msgs ...kafka.Message) error {
	if m.writeErr != nil {
		// If the mock error wraps DeadlineExceeded, respect the context model.
		if errors.Is(m.writeErr, context.DeadlineExceeded) {
			// Simulate deadline exceeded: tick past parent expiry
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
		}
		return m.writeErr
	}
	if len(msgs) > 0 {
		m.gotMsg = &msgs[0]
	}
	return nil
}

func (m *mockWriter) Close() error {
	return m.closeErr
}

func TestHandleIngest_Success(t *testing.T) {
	mw := &mockWriter{}
	srv := NewServer(mw)

	body := `{"service":"test-svc","level":"info","message":"hello world"}`
	req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d — body: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["status"] != "log received" {
		t.Errorf("expected status 'log received', got %q", resp["status"])
	}

	if mw.gotMsg == nil {
		t.Fatal("expected a Kafka message to be written")
	}
	if string(mw.gotMsg.Key) != "test-svc" {
		t.Errorf("expected key 'test-svc', got %q", string(mw.gotMsg.Key))
	}
}

func TestHandleIngest_InvalidJSON(t *testing.T) {
	mw := &mockWriter{}
	srv := NewServer(mw)

	body := `not json at all`
	req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}

	if mw.gotMsg != nil {
		t.Error("expected no Kafka message to be written on bad request")
	}
}

func TestHandleIngest_MissingService(t *testing.T) {
	mw := &mockWriter{}
	srv := NewServer(mw)

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

	if mw.gotMsg != nil {
		t.Error("expected no Kafka message on missing service")
	}
}

func TestHandleIngest_MissingLevel(t *testing.T) {
	mw := &mockWriter{}
	srv := NewServer(mw)

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

	if mw.gotMsg != nil {
		t.Error("expected no Kafka message on missing level")
	}
}

func TestHandleIngest_KafkaWriteError(t *testing.T) {
	mw := &mockWriter{writeErr: errors.New("kafka unavailable")}
	srv := NewServer(mw)

	body := `{"service":"test-svc","level":"info","message":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}

	var resp map[string]string
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["error"] != "failed to send to kafka" {
		t.Errorf("expected 'failed to send to kafka', got %q", resp["error"])
	}
}

func TestHandleIngest_KafkaTimeout(t *testing.T) {
	mw := &mockWriter{writeErr: context.DeadlineExceeded}
	srv := NewServer(mw)

	body := `{"service":"test-svc","level":"info","message":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusGatewayTimeout {
		t.Fatalf("expected 504, got %d", rec.Code)
	}

	var resp map[string]string
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["error"] != "kafka write timed out" {
		t.Errorf("expected 'kafka write timed out', got %q", resp["error"])
	}
}

func TestHandleIngest_RequestBodyTooLarge(t *testing.T) {
	mw := &mockWriter{}
	srv := NewServer(mw)

	// Build a valid JSON payload larger than 1 MB.
	// A 1 MB string value inside JSON makes the total body exceed the limit.
	largeMsg := strings.Repeat("x", defaultMaxBodySize+1024) // 1 MB + 1 KB string
	body := fmt.Sprintf(`{"service":"test-svc","level":"info","message":"%s"}`, largeMsg)
	req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d — body length: %d", rec.Code, len(body))
	}

	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "request body too large" {
		t.Errorf("expected 'request body too large', got %q", resp["error"])
	}

	if mw.gotMsg != nil {
		t.Error("expected no Kafka message to be written when body exceeds limit")
	}
}

func TestHealthz(t *testing.T) {
	mw := &mockWriter{}
	srv := NewServer(mw)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp map[string]string
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["status"] != "ok" {
		t.Errorf("expected status 'ok', got %q", resp["status"])
	}
}

func TestTimestampAutoFill(t *testing.T) {
	mw := &mockWriter{}
	srv := NewServer(mw)

	before := time.Now().Unix()
	body := `{"service":"test-svc","level":"info","message":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var entry LogEntry
	if err := json.Unmarshal(mw.gotMsg.Value, &entry); err != nil {
		t.Fatalf("failed to decode written message: %v", err)
	}
	if entry.Timestamp < before {
		t.Errorf("expected timestamp >= %d (before request), got %d", before, entry.Timestamp)
	}
}
