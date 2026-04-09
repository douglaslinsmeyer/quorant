package health_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/quorant/quorant/internal/platform/health"
)

// mockChecker is a test double implementing the Checker interface.
type mockChecker struct {
	name string
	err  error
}

func (m *mockChecker) Name() string                    { return m.name }
func (m *mockChecker) Check(_ context.Context) error   { return m.err }

// healthResponse mirrors the JSON body returned by the handler.
type healthResponse struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks"`
}

func TestHandler_AllHealthy(t *testing.T) {
	t.Parallel()

	h := health.NewHandler(
		&mockChecker{name: "db"},
		&mockChecker{name: "redis"},
	)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Fatalf("expected Content-Type application/json, got %q", contentType)
	}

	var resp healthResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}

	if resp.Status != "healthy" {
		t.Errorf("expected status %q, got %q", "healthy", resp.Status)
	}

	if resp.Checks["db"] != "ok" {
		t.Errorf("expected db=ok, got %q", resp.Checks["db"])
	}
	if resp.Checks["redis"] != "ok" {
		t.Errorf("expected redis=ok, got %q", resp.Checks["redis"])
	}
}

func TestHandler_OneCheckerFails(t *testing.T) {
	t.Parallel()

	redisErr := fmt.Errorf("connection refused")
	h := health.NewHandler(
		&mockChecker{name: "db"},
		&mockChecker{name: "redis", err: redisErr},
	)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d", rec.Code)
	}

	var resp healthResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}

	if resp.Status != "unhealthy" {
		t.Errorf("expected status %q, got %q", "unhealthy", resp.Status)
	}
	if resp.Checks["db"] != "ok" {
		t.Errorf("expected db=ok, got %q", resp.Checks["db"])
	}
	if resp.Checks["redis"] == "" || resp.Checks["redis"] == "ok" {
		t.Errorf("expected redis to contain an error message, got %q", resp.Checks["redis"])
	}
}

func TestHandler_NoCheckers(t *testing.T) {
	t.Parallel()

	h := health.NewHandler()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resp healthResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}

	if resp.Status != "healthy" {
		t.Errorf("expected status %q, got %q", "healthy", resp.Status)
	}
	if len(resp.Checks) != 0 {
		t.Errorf("expected empty checks map, got %v", resp.Checks)
	}
}
