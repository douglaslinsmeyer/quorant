package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/platform/middleware"
)

// ─── TenantContext middleware tests ──────────────────────────────────────────

// orgCapture records the org ID injected into the request context by the
// tenant middleware so tests can assert it was stored correctly.
type orgCapture struct {
	orgID  uuid.UUID
	called bool
}

func (c *orgCapture) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.called = true
		c.orgID = middleware.OrgIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})
}

// buildMuxWithTenantMiddleware creates a ServeMux with TenantContext middleware
// on a route that includes {org_id} as a path parameter.
func buildMuxWithTenantMiddleware(cap *orgCapture) *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("GET /api/v1/organizations/{org_id}/test",
		middleware.TenantContext(cap.Handler()),
	)
	return mux
}

// buildMuxNoOrgID creates a ServeMux with TenantContext middleware on a route
// that does NOT include {org_id} as a path parameter.
func buildMuxNoOrgID(cap *orgCapture) *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("GET /api/v1/health",
		middleware.TenantContext(cap.Handler()),
	)
	return mux
}

func TestTenantContext_ValidUUID_StoresOrgIDInContext(t *testing.T) {
	orgID := uuid.New()
	cap := &orgCapture{}
	mux := buildMuxWithTenantMiddleware(cap)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/organizations/"+orgID.String()+"/test", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
	if !cap.called {
		t.Error("expected next handler to be called")
	}
	if cap.orgID == uuid.Nil {
		t.Fatal("expected org ID to be stored in context, got uuid.Nil")
	}
	if cap.orgID != orgID {
		t.Errorf("expected org ID %v, got %v", orgID, cap.orgID)
	}
}

func TestTenantContext_InvalidUUID_Returns400ValidationError(t *testing.T) {
	cap := &orgCapture{}
	mux := buildMuxWithTenantMiddleware(cap)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/organizations/not-a-uuid/test", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}
	if cap.called {
		t.Error("expected next handler NOT to be called")
	}
	resp := decodeErrorResponse(t, rec.Body.Bytes())
	if len(resp.Errors) == 0 {
		t.Fatal("expected at least one error in response body")
	}
	if resp.Errors[0].Code != "VALIDATION_ERROR" {
		t.Errorf("expected error code VALIDATION_ERROR, got %q", resp.Errors[0].Code)
	}
}

func TestTenantContext_NoOrgIDPathParam_PassesThroughWithoutOrgContext(t *testing.T) {
	cap := &orgCapture{}
	mux := buildMuxNoOrgID(cap)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
	if !cap.called {
		t.Error("expected next handler to be called")
	}
	if cap.orgID != uuid.Nil {
		t.Errorf("expected org ID to be uuid.Nil (no org context), got %v", cap.orgID)
	}
}
