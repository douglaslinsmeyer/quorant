package middleware_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/platform/middleware"
)

// ─── Mock PermissionChecker ──────────────────────────────────────────────────

type mockPermissionChecker struct {
	allowed bool
	err     error
}

func (m *mockPermissionChecker) HasPermission(_ context.Context, _, _ uuid.UUID, _ string) (bool, error) {
	return m.allowed, m.err
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// staticUserIDResolver returns a resolver that always produces the given user ID.
func staticUserIDResolver(id uuid.UUID) func(context.Context) (uuid.UUID, error) {
	return func(_ context.Context) (uuid.UUID, error) {
		return id, nil
	}
}

// failingUserIDResolver returns a resolver that always fails.
func failingUserIDResolver(err error) func(context.Context) (uuid.UUID, error) {
	return func(_ context.Context) (uuid.UUID, error) {
		return uuid.Nil, err
	}
}

// handlerCalled is a minimal http.Handler that records whether it was invoked.
type handlerCalled struct {
	called bool
}

func (h *handlerCalled) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	h.called = true
	w.WriteHeader(http.StatusOK)
}

// buildRequest creates a GET request with optional org ID and user ID in context.
func buildRequest(orgID uuid.UUID) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	if orgID != uuid.Nil {
		r = r.WithContext(middleware.WithOrgID(r.Context(), orgID))
	}
	return r
}

// ─── OrgID context helpers ───────────────────────────────────────────────────

func TestWithOrgID_StoresAndRetrieves(t *testing.T) {
	id := uuid.New()
	ctx := middleware.WithOrgID(context.Background(), id)
	got := middleware.OrgIDFromContext(ctx)
	if got != id {
		t.Errorf("expected %v, got %v", id, got)
	}
}

func TestOrgIDFromContext_MissingReturnsNil(t *testing.T) {
	got := middleware.OrgIDFromContext(context.Background())
	if got != uuid.Nil {
		t.Errorf("expected uuid.Nil, got %v", got)
	}
}

// ─── RequirePermission unit tests ────────────────────────────────────────────

func TestRequirePermission_Allowed(t *testing.T) {
	checker := &mockPermissionChecker{allowed: true}
	next := &handlerCalled{}

	orgID := uuid.New()
	userID := uuid.New()
	req := buildRequest(orgID)
	rec := httptest.NewRecorder()

	mw := middleware.RequirePermission(checker, "org.organization.read", staticUserIDResolver(userID))
	mw(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
	if !next.called {
		t.Error("expected next handler to be called")
	}
}

func TestRequirePermission_Denied(t *testing.T) {
	checker := &mockPermissionChecker{allowed: false}
	next := &handlerCalled{}

	orgID := uuid.New()
	userID := uuid.New()
	req := buildRequest(orgID)
	rec := httptest.NewRecorder()

	mw := middleware.RequirePermission(checker, "org.organization.read", staticUserIDResolver(userID))
	mw(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d", rec.Code)
	}
	if next.called {
		t.Error("expected next handler NOT to be called")
	}
	resp := decodeErrorResponse(t, rec.Body.Bytes())
	if len(resp.Errors) == 0 {
		t.Fatal("expected at least one error in response body")
	}
	if resp.Errors[0].Code != "FORBIDDEN" {
		t.Errorf("expected error code FORBIDDEN, got %q", resp.Errors[0].Code)
	}
}

func TestRequirePermission_NoOrgContext(t *testing.T) {
	checker := &mockPermissionChecker{allowed: true}
	next := &handlerCalled{}

	userID := uuid.New()
	// Request with NO org ID in context (uuid.Nil guard inside buildRequest is
	// deliberately bypassed by passing uuid.Nil directly, which skips WithOrgID).
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	mw := middleware.RequirePermission(checker, "org.organization.read", staticUserIDResolver(userID))
	mw(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d", rec.Code)
	}
	if next.called {
		t.Error("expected next handler NOT to be called")
	}
	resp := decodeErrorResponse(t, rec.Body.Bytes())
	if len(resp.Errors) == 0 {
		t.Fatal("expected at least one error in response body")
	}
	if resp.Errors[0].Code != "FORBIDDEN" {
		t.Errorf("expected error code FORBIDDEN, got %q", resp.Errors[0].Code)
	}
}

func TestRequirePermission_UserIDResolverError(t *testing.T) {
	checker := &mockPermissionChecker{allowed: true}
	next := &handlerCalled{}

	orgID := uuid.New()
	req := buildRequest(orgID)
	rec := httptest.NewRecorder()

	mw := middleware.RequirePermission(checker, "org.organization.read", failingUserIDResolver(errors.New("no claims")))
	mw(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rec.Code)
	}
	if next.called {
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

func TestRequirePermission_CheckerError(t *testing.T) {
	checker := &mockPermissionChecker{err: errors.New("db connection lost")}
	next := &handlerCalled{}

	orgID := uuid.New()
	userID := uuid.New()
	req := buildRequest(orgID)
	rec := httptest.NewRecorder()

	mw := middleware.RequirePermission(checker, "org.organization.read", staticUserIDResolver(userID))
	mw(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rec.Code)
	}
	if next.called {
		t.Error("expected next handler NOT to be called")
	}
	resp := decodeErrorResponse(t, rec.Body.Bytes())
	if len(resp.Errors) == 0 {
		t.Fatal("expected at least one error in response body")
	}
	if resp.Errors[0].Code != "INTERNAL_ERROR" {
		t.Errorf("expected error code INTERNAL_ERROR, got %q", resp.Errors[0].Code)
	}
}
