package admin

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/platform/api"
	"github.com/quorant/quorant/internal/platform/auth"
	"github.com/quorant/quorant/internal/platform/middleware"
)

// RegisterRoutes registers all admin routes on the mux.
// All routes are auth-protected. There is no tenant context — these are platform-scoped.
// Permission checks use uuid.Nil as the org ID; the HasPermission query grants access
// via platform roles (platform_admin, platform_support, platform_finance) globally.
func RegisterRoutes(
	mux *http.ServeMux,
	handler *AdminHandler,
	validator auth.TokenValidator,
	checker middleware.PermissionChecker,
	resolveUserID func(context.Context) (uuid.UUID, error),
) {
	// platformPermMw wraps a handler with auth + platform-scoped permission check.
	// Uses uuid.Nil for org ID since platform roles are not org-scoped.
	platformPermMw := func(perm string, h http.HandlerFunc) http.Handler {
		return middleware.Auth(validator, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, err := resolveUserID(r.Context())
			if err != nil {
				api.WriteError(w, api.NewUnauthenticatedError("auth.resolve_failed"))
				return
			}
			allowed, err := checker.HasPermission(r.Context(), userID, uuid.Nil, perm)
			if err != nil {
				api.WriteError(w, api.NewInternalError(err))
				return
			}
			if !allowed {
				api.WriteError(w, api.NewForbiddenError("access.insufficient_permissions"))
				return
			}
			h(w, r)
		}))
	}

	// Tenant management
	mux.Handle("GET /api/v1/admin/tenants", platformPermMw("admin.tenant.read", handler.ListTenants))
	mux.Handle("GET /api/v1/admin/tenants/{org_id}", platformPermMw("admin.tenant.read", handler.GetTenantDashboard))
	mux.Handle("POST /api/v1/admin/tenants/{org_id}/suspend", platformPermMw("admin.tenant.manage", handler.SuspendTenant))
	mux.Handle("POST /api/v1/admin/tenants/{org_id}/reactivate", platformPermMw("admin.tenant.manage", handler.ReactivateTenant))

	// Impersonation
	mux.Handle("POST /api/v1/admin/impersonate", platformPermMw("admin.impersonate", handler.StartImpersonation))
	mux.Handle("POST /api/v1/admin/impersonate/stop", platformPermMw("admin.impersonate", handler.StopImpersonation))

	// User admin
	mux.Handle("GET /api/v1/admin/users", platformPermMw("admin.tenant.read", handler.SearchUsers))
	mux.Handle("POST /api/v1/admin/users/{user_id}/reset-password", platformPermMw("admin.tenant.manage", handler.ResetPassword))
	mux.Handle("POST /api/v1/admin/users/{user_id}/unlock", platformPermMw("admin.tenant.manage", handler.UnlockAccount))

	// Feature flags
	mux.Handle("GET /api/v1/admin/feature-flags", platformPermMw("admin.feature_flag.manage", handler.ListFlags))
	mux.Handle("POST /api/v1/admin/feature-flags", platformPermMw("admin.feature_flag.manage", handler.CreateFlag))
	mux.Handle("PATCH /api/v1/admin/feature-flags/{flag_id}", platformPermMw("admin.feature_flag.manage", handler.UpdateFlag))
	mux.Handle("POST /api/v1/admin/feature-flags/{flag_id}/overrides", platformPermMw("admin.feature_flag.manage", handler.SetOverride))
}
