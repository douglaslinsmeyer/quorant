package middleware_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/quorant/quorant/internal/platform/auth"
	"github.com/quorant/quorant/internal/platform/middleware"
)

// claimsCapture records the claims injected into the request context by the
// auth middleware, so tests can assert they were passed through correctly.
type claimsCapture struct {
	claims *auth.Claims
	called bool
	status int
}

func (c *claimsCapture) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.called = true
		c.claims, _ = auth.ClaimsFromContext(r.Context())
		c.status = http.StatusOK
		w.WriteHeader(http.StatusOK)
	})
}

// errorResponseBody is a minimal struct for decoding the JSON error envelope
// returned by WriteError.
type errorResponseBody struct {
	Errors []struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"errors"`
}

func decodeErrorResponse(t *testing.T, body []byte) errorResponseBody {
	t.Helper()
	var resp errorResponseBody
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("failed to decode error response body: %v", err)
	}
	return resp
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestAuth_ValidBearerToken_InjectsClaimsAndCallsNext(t *testing.T) {
	want := &auth.Claims{Subject: "user-123", Email: "user@example.com", Name: "Test User"}
	validator := auth.NewStaticValidator(want)

	cap := &claimsCapture{}
	handler := middleware.Auth(validator, cap.Handler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
	if !cap.called {
		t.Error("expected next handler to be called")
	}
	if cap.claims == nil {
		t.Fatal("expected claims to be injected into context, got nil")
	}
	if cap.claims.Subject != want.Subject {
		t.Errorf("expected Subject %q, got %q", want.Subject, cap.claims.Subject)
	}
	if cap.claims.Email != want.Email {
		t.Errorf("expected Email %q, got %q", want.Email, cap.claims.Email)
	}
}

func TestAuth_MissingAuthorizationHeader_Returns401(t *testing.T) {
	validator := auth.NewStaticValidator(&auth.Claims{Subject: "user-123"})

	cap := &claimsCapture{}
	handler := middleware.Auth(validator, cap.Handler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	// No Authorization header.
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rec.Code)
	}
	if cap.called {
		t.Error("expected next handler NOT to be called")
	}
	resp := decodeErrorResponse(t, rec.Body.Bytes())
	if len(resp.Errors) == 0 {
		t.Fatal("expected at least one error in response body")
	}
	if resp.Errors[0].Code != "UNAUTHENTICATED" {
		t.Errorf("expected error code UNAUTHENTICATED, got %q", resp.Errors[0].Code)
	}
}

func TestAuth_BasicScheme_Returns401(t *testing.T) {
	validator := auth.NewStaticValidator(&auth.Claims{Subject: "user-123"})

	cap := &claimsCapture{}
	handler := middleware.Auth(validator, cap.Handler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rec.Code)
	}
	if cap.called {
		t.Error("expected next handler NOT to be called")
	}
	resp := decodeErrorResponse(t, rec.Body.Bytes())
	if len(resp.Errors) == 0 {
		t.Fatal("expected at least one error in response body")
	}
	if resp.Errors[0].Code != "UNAUTHENTICATED" {
		t.Errorf("expected error code UNAUTHENTICATED, got %q", resp.Errors[0].Code)
	}
}

func TestAuth_MalformedBearerNoSpace_Returns401(t *testing.T) {
	validator := auth.NewStaticValidator(&auth.Claims{Subject: "user-123"})

	cap := &claimsCapture{}
	handler := middleware.Auth(validator, cap.Handler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	// "Bearer" with no token after it — no space, so SplitN produces one part.
	req.Header.Set("Authorization", "Bearertoken-no-space")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rec.Code)
	}
	if cap.called {
		t.Error("expected next handler NOT to be called")
	}
	resp := decodeErrorResponse(t, rec.Body.Bytes())
	if len(resp.Errors) == 0 {
		t.Fatal("expected at least one error in response body")
	}
	if resp.Errors[0].Code != "UNAUTHENTICATED" {
		t.Errorf("expected error code UNAUTHENTICATED, got %q", resp.Errors[0].Code)
	}
}

func TestAuth_InvalidToken_ValidatorReturnsError_Returns401(t *testing.T) {
	validator := &auth.StaticValidator{Err: fmt.Errorf("bad token")}

	cap := &claimsCapture{}
	handler := middleware.Auth(validator, cap.Handler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer expired-or-bad-token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rec.Code)
	}
	if cap.called {
		t.Error("expected next handler NOT to be called")
	}
	resp := decodeErrorResponse(t, rec.Body.Bytes())
	if len(resp.Errors) == 0 {
		t.Fatal("expected at least one error in response body")
	}
	if resp.Errors[0].Code != "UNAUTHENTICATED" {
		t.Errorf("expected error code UNAUTHENTICATED, got %q", resp.Errors[0].Code)
	}
}
