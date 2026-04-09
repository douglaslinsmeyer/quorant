package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/quorant/quorant/internal/platform/middleware"
)

func TestRequestID_PropagatesExistingID(t *testing.T) {
	const existingID = "test-request-id-123"

	var capturedID string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedID = middleware.RequestIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.RequestID(inner)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", existingID)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if capturedID != existingID {
		t.Errorf("expected context request ID %q, got %q", existingID, capturedID)
	}

	if got := rec.Header().Get("X-Request-ID"); got != existingID {
		t.Errorf("expected response X-Request-ID %q, got %q", existingID, got)
	}
}

func TestRequestID_GeneratesNewIDWhenAbsent(t *testing.T) {
	var capturedID string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedID = middleware.RequestIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.RequestID(inner)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if capturedID == "" {
		t.Error("expected a generated request ID in context, got empty string")
	}

	responseID := rec.Header().Get("X-Request-ID")
	if responseID == "" {
		t.Error("expected X-Request-ID response header to be set, got empty string")
	}

	if capturedID != responseID {
		t.Errorf("context ID %q does not match response header ID %q", capturedID, responseID)
	}
}

func TestRequestID_GeneratesUUIDFormat(t *testing.T) {
	var capturedID string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedID = middleware.RequestIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.RequestID(inner)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// UUID v4 format: xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx (36 chars)
	if len(capturedID) != 36 {
		t.Errorf("expected UUID-length ID (36 chars), got %d chars: %q", len(capturedID), capturedID)
	}
}
