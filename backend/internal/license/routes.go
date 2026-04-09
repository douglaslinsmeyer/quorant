package license

import (
	"net/http"

	"github.com/quorant/quorant/internal/platform/auth"
	"github.com/quorant/quorant/internal/platform/middleware"
)

// RegisterRoutes registers all license module routes on the mux.
// Admin routes use auth middleware only; org-scoped routes also get tenant context.
func RegisterRoutes(mux *http.ServeMux, handler *LicenseHandler, validator auth.TokenValidator) {
	authMw := func(h http.HandlerFunc) http.Handler {
		return middleware.Auth(validator, http.HandlerFunc(h))
	}
	orgMw := func(h http.HandlerFunc) http.Handler {
		return middleware.Auth(validator, middleware.TenantContext(http.HandlerFunc(h)))
	}

	// Admin plan management
	mux.Handle("GET /api/v1/admin/plans", authMw(handler.ListPlans))
	mux.Handle("POST /api/v1/admin/plans", authMw(handler.CreatePlan))
	mux.Handle("PATCH /api/v1/admin/plans/{plan_id}", authMw(handler.UpdatePlan))
	mux.Handle("GET /api/v1/admin/plans/{plan_id}/entitlements", authMw(handler.ListEntitlements))

	// Org subscription management
	mux.Handle("POST /api/v1/organizations/{org_id}/subscription", orgMw(handler.CreateSubscription))
	mux.Handle("GET /api/v1/organizations/{org_id}/subscription", orgMw(handler.GetSubscription))
	mux.Handle("PATCH /api/v1/organizations/{org_id}/subscription", orgMw(handler.UpdateSubscription))
	mux.Handle("GET /api/v1/organizations/{org_id}/entitlements", orgMw(handler.CheckEntitlements))
	mux.Handle("POST /api/v1/admin/organizations/{org_id}/entitlement-overrides", orgMw(handler.SetOverride))
	mux.Handle("GET /api/v1/organizations/{org_id}/usage", orgMw(handler.GetUsage))
}
