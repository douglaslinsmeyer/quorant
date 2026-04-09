package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/quorant/quorant/internal/platform/middleware"
)

// nextHandlerSpy records whether the inner handler was called and what status it wrote.
func nextHandlerSpy(called *bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*called = true
		w.WriteHeader(http.StatusOK)
	})
}

func TestCORS_WildcardOriginMatchesAnyRequestOrigin(t *testing.T) {
	var called bool
	handler := middleware.CORS([]string{"*"}, nextHandlerSpy(&called))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "http://example.com")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://example.com" {
		t.Errorf("expected Access-Control-Allow-Origin %q, got %q", "http://example.com", got)
	}
}

func TestCORS_SpecificOriginMatchGrantsHeaders(t *testing.T) {
	allowedOrigin := "http://localhost:3000"
	var called bool
	handler := middleware.CORS([]string{allowedOrigin, "http://app.example.com"}, nextHandlerSpy(&called))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", allowedOrigin)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != allowedOrigin {
		t.Errorf("expected Access-Control-Allow-Origin %q, got %q", allowedOrigin, got)
	}
	if rec.Header().Get("Access-Control-Allow-Methods") == "" {
		t.Error("expected Access-Control-Allow-Methods to be set")
	}
	if rec.Header().Get("Access-Control-Allow-Headers") == "" {
		t.Error("expected Access-Control-Allow-Headers to be set")
	}
}

func TestCORS_NonMatchingOriginGetsNoCORSHeaders(t *testing.T) {
	var called bool
	handler := middleware.CORS([]string{"http://localhost:3000"}, nextHandlerSpy(&called))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "http://evil.example.com")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("expected no Access-Control-Allow-Origin header, got %q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); got != "" {
		t.Errorf("expected no Access-Control-Allow-Methods header, got %q", got)
	}
}

func TestCORS_OptionsPreflightReturns204(t *testing.T) {
	var called bool
	handler := middleware.CORS([]string{"http://localhost:3000"}, nextHandlerSpy(&called))

	req := httptest.NewRequest(http.MethodOptions, "/", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", rec.Code)
	}
	if called {
		t.Error("expected inner handler NOT to be called for OPTIONS preflight")
	}
}

func TestCORS_NonOptionsRequestPassesThroughToHandler(t *testing.T) {
	var called bool
	handler := middleware.CORS([]string{"http://localhost:3000"}, nextHandlerSpy(&called))

	req := httptest.NewRequest(http.MethodGet, "/some-resource", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("expected inner handler to be called for non-OPTIONS request")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

func TestCORS_RequestWithNoOriginHeaderPassesThrough(t *testing.T) {
	var called bool
	handler := middleware.CORS([]string{"http://localhost:3000"}, nextHandlerSpy(&called))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	// No Origin header set
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("expected inner handler to be called when no Origin header present")
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("expected no Access-Control-Allow-Origin header when no Origin sent, got %q", got)
	}
}
