package admin

import (
	"net/http"

	"github.com/quorant/quorant/internal/platform/auth"
	"github.com/quorant/quorant/internal/platform/middleware"
)

// RegisterRoutes registers all admin routes on the mux.
// All routes are auth-protected. There is no tenant context — these are platform-scoped.
func RegisterRoutes(mux *http.ServeMux, handler *AdminHandler, validator auth.TokenValidator) {
	// authMw wraps a handler with JWT authentication.
	authMw := func(h http.HandlerFunc) http.Handler {
		return middleware.Auth(validator, http.HandlerFunc(h))
	}

	// Tenant management
	mux.Handle("GET /api/v1/admin/tenants", authMw(handler.ListTenants))
	mux.Handle("GET /api/v1/admin/tenants/{org_id}", authMw(handler.GetTenantDashboard))
	mux.Handle("POST /api/v1/admin/tenants/{org_id}/suspend", authMw(handler.SuspendTenant))
	mux.Handle("POST /api/v1/admin/tenants/{org_id}/reactivate", authMw(handler.ReactivateTenant))

	// Impersonation
	mux.Handle("POST /api/v1/admin/impersonate", authMw(handler.StartImpersonation))
	mux.Handle("POST /api/v1/admin/impersonate/stop", authMw(handler.StopImpersonation))

	// User admin
	mux.Handle("GET /api/v1/admin/users", authMw(handler.SearchUsers))
	mux.Handle("POST /api/v1/admin/users/{user_id}/reset-password", authMw(handler.ResetPassword))
	mux.Handle("POST /api/v1/admin/users/{user_id}/unlock", authMw(handler.UnlockAccount))

	// Feature flags
	mux.Handle("GET /api/v1/admin/feature-flags", authMw(handler.ListFlags))
	mux.Handle("POST /api/v1/admin/feature-flags", authMw(handler.CreateFlag))
	mux.Handle("PATCH /api/v1/admin/feature-flags/{flag_id}", authMw(handler.UpdateFlag))
	mux.Handle("POST /api/v1/admin/feature-flags/{flag_id}/overrides", authMw(handler.SetOverride))
}
