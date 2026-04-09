package license

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/platform/api"
	"github.com/quorant/quorant/internal/platform/auth"
	"github.com/quorant/quorant/internal/platform/middleware"
)

// RegisterRoutes registers all license module routes on the mux.
// Admin routes use auth middleware with platform-scoped permission checks;
// org-scoped routes also get tenant context.
func RegisterRoutes(
	mux *http.ServeMux,
	handler *LicenseHandler,
	validator auth.TokenValidator,
	checker middleware.PermissionChecker,
	resolveUserID func(context.Context) (uuid.UUID, error),
) {
	// platformPermMw: auth + platform-scoped permission (no tenant context, uuid.Nil org)
	platformPermMw := func(perm string, h http.HandlerFunc) http.Handler {
		return middleware.Auth(validator, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, err := resolveUserID(r.Context())
			if err != nil {
				api.WriteError(w, api.NewUnauthenticatedError("could not resolve user"))
				return
			}
			allowed, err := checker.HasPermission(r.Context(), userID, uuid.Nil, perm)
			if err != nil {
				api.WriteError(w, api.NewInternalError(err))
				return
			}
			if !allowed {
				api.WriteError(w, api.NewForbiddenError("insufficient permissions"))
				return
			}
			h(w, r)
		}))
	}

	// orgPermMw: auth + tenant context + org-scoped permission check
	orgPermMw := func(perm string, h http.HandlerFunc) http.Handler {
		return middleware.Auth(validator,
			middleware.TenantContext(
				middleware.RequirePermission(checker, perm, resolveUserID)(
					http.HandlerFunc(h))))
	}

	// Admin plan management
	mux.Handle("GET /api/v1/admin/plans", platformPermMw("license.plan.manage", handler.ListPlans))
	mux.Handle("POST /api/v1/admin/plans", platformPermMw("license.plan.manage", handler.CreatePlan))
	mux.Handle("PATCH /api/v1/admin/plans/{plan_id}", platformPermMw("license.plan.manage", handler.UpdatePlan))
	mux.Handle("GET /api/v1/admin/plans/{plan_id}/entitlements", platformPermMw("license.plan.manage", handler.ListEntitlements))

	// Org subscription management
	mux.Handle("POST /api/v1/organizations/{org_id}/subscription", orgPermMw("license.subscription.manage", handler.CreateSubscription))
	mux.Handle("GET /api/v1/organizations/{org_id}/subscription", orgPermMw("license.subscription.read", handler.GetSubscription))
	mux.Handle("PATCH /api/v1/organizations/{org_id}/subscription", orgPermMw("license.subscription.manage", handler.UpdateSubscription))
	mux.Handle("GET /api/v1/organizations/{org_id}/entitlements", orgPermMw("license.subscription.read", handler.CheckEntitlements))
	mux.Handle("POST /api/v1/admin/organizations/{org_id}/entitlement-overrides", orgPermMw("license.plan.manage", handler.SetOverride))
	mux.Handle("GET /api/v1/organizations/{org_id}/usage", orgPermMw("license.usage.read", handler.GetUsage))
}
