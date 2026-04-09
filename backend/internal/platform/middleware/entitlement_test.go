package middleware_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/platform/middleware"
)

// ─── Mock EntitlementChecker ─────────────────────────────────────────────────

type mockEntitlementChecker struct {
	allowed bool
	err     error
}

func (m *mockEntitlementChecker) Check(_ context.Context, _ uuid.UUID, _ string) (bool, int, error) {
	return m.allowed, -1, m.err
}

// ─── RequireEntitlement tests ─────────────────────────────────────────────────

func TestRequireEntitlement_Allowed_CallsNextAnd200(t *testing.T) {
	checker := &mockEntitlementChecker{allowed: true}
	next := &handlerCalled{}

	orgID := uuid.New()
	req := buildRequest(orgID)
	rec := httptest.NewRecorder()

	mw := middleware.RequireEntitlement(checker, "ai.context_lake")
	mw(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
	if !next.called {
		t.Error("expected next handler to be called")
	}
}

func TestRequireEntitlement_Denied_Returns403WithMessage(t *testing.T) {
	checker := &mockEntitlementChecker{allowed: false}
	next := &handlerCalled{}

	orgID := uuid.New()
	req := buildRequest(orgID)
	rec := httptest.NewRecorder()

	mw := middleware.RequireEntitlement(checker, "ai.context_lake")
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
	if !strings.Contains(resp.Errors[0].Message, "ai.context_lake") {
		t.Errorf("expected message to contain feature key 'ai.context_lake', got %q", resp.Errors[0].Message)
	}
}

func TestRequireEntitlement_NoOrgContext_PassesThrough(t *testing.T) {
	checker := &mockEntitlementChecker{allowed: false} // would deny if checked
	next := &handlerCalled{}

	// Request with NO org ID in context — user-scoped route
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	mw := middleware.RequireEntitlement(checker, "ai.context_lake")
	mw(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200 (pass-through), got %d", rec.Code)
	}
	if !next.called {
		t.Error("expected next handler to be called when no org context present")
	}
}

func TestRequireEntitlement_CheckerError_Returns500(t *testing.T) {
	checker := &mockEntitlementChecker{err: errors.New("db connection lost")}
	next := &handlerCalled{}

	orgID := uuid.New()
	req := buildRequest(orgID)
	rec := httptest.NewRecorder()

	mw := middleware.RequireEntitlement(checker, "ai.context_lake")
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
