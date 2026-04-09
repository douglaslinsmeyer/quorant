package middleware_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/quorant/quorant/internal/platform/middleware"
)

func parseLogEntry(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()
	var entry map[string]any
	if err := json.NewDecoder(buf).Decode(&entry); err != nil {
		t.Fatalf("failed to parse log entry: %v", err)
	}
	return entry
}

func TestLogging_SuccessfulRequestLoggedAtInfo(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.Logging(logger, inner)

	req := httptest.NewRequest(http.MethodGet, "/api/things", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	entry := parseLogEntry(t, &buf)

	if entry["level"] != "INFO" {
		t.Errorf("expected log level INFO, got %v", entry["level"])
	}
	if entry["method"] != "GET" {
		t.Errorf("expected method GET, got %v", entry["method"])
	}
	if entry["path"] != "/api/things" {
		t.Errorf("expected path /api/things, got %v", entry["path"])
	}
	if entry["status"] == nil {
		t.Error("expected status field in log entry")
	}
	if entry["duration_ms"] == nil {
		t.Error("expected duration_ms field in log entry")
	}
}

func TestLogging_404ResponseLoggedAtWarn(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	handler := middleware.Logging(logger, inner)

	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	entry := parseLogEntry(t, &buf)

	if entry["level"] != "WARN" {
		t.Errorf("expected log level WARN for 404, got %v", entry["level"])
	}
	if entry["status"] != float64(404) {
		t.Errorf("expected status 404 in log, got %v", entry["status"])
	}
}

func TestLogging_500ResponseLoggedAtError(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	handler := middleware.Logging(logger, inner)

	req := httptest.NewRequest(http.MethodGet, "/broken", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	entry := parseLogEntry(t, &buf)

	if entry["level"] != "ERROR" {
		t.Errorf("expected log level ERROR for 500, got %v", entry["level"])
	}
	if entry["status"] != float64(500) {
		t.Errorf("expected status 500 in log, got %v", entry["status"])
	}
}

func TestLogging_DurationIsRecorded(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.Logging(logger, inner)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	entry := parseLogEntry(t, &buf)

	durationVal, ok := entry["duration_ms"]
	if !ok {
		t.Fatal("expected duration_ms field in log entry, not found")
	}

	duration, ok := durationVal.(float64)
	if !ok {
		t.Fatalf("expected duration_ms to be a number, got %T", durationVal)
	}

	if duration < 0 {
		t.Errorf("expected non-negative duration, got %f", duration)
	}
}

func TestLogging_RequestIDIncludedWhenPresent(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	const testID = "test-request-id-456"

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Chain RequestID middleware before Logging so the ID is in context
	handler := middleware.RequestID(middleware.Logging(logger, inner))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", testID)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	entry := parseLogEntry(t, &buf)

	if entry["request_id"] != testID {
		t.Errorf("expected request_id %q in log entry, got %v", testID, entry["request_id"])
	}
}
