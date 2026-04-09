package estoppel

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/platform/auth"
	"github.com/quorant/quorant/internal/platform/middleware"
)

// RegisterRoutes registers all Estoppel module routes on the mux.
//
// Middleware chain per route:
//
//	Auth → TenantContext → RequireEntitlement("estoppel") → RequirePermission
func RegisterRoutes(
	mux *http.ServeMux,
	handler *Handler,
	validator auth.TokenValidator,
	checker middleware.PermissionChecker,
	entitlements middleware.EntitlementChecker,
	resolveUserID func(context.Context) (uuid.UUID, error),
) {
	permMw := func(perm string, h http.HandlerFunc) http.Handler {
		return middleware.Auth(validator,
			middleware.TenantContext(
				middleware.RequireEntitlement(entitlements, "estoppel")(
					middleware.RequirePermission(checker, perm, resolveUserID)(
						http.HandlerFunc(h)))))
	}

	mux.Handle("POST /api/v1/organizations/{org_id}/estoppel/requests",
		permMw("estoppel.request.create", handler.CreateRequest))

	mux.Handle("GET /api/v1/organizations/{org_id}/estoppel/requests",
		permMw("estoppel.request.list", handler.ListRequests))

	mux.Handle("GET /api/v1/organizations/{org_id}/estoppel/requests/{id}",
		permMw("estoppel.request.read", handler.GetRequest))

	mux.Handle("POST /api/v1/organizations/{org_id}/estoppel/requests/{id}/approve",
		permMw("estoppel.request.approve", handler.ApproveRequest))

	mux.Handle("POST /api/v1/organizations/{org_id}/estoppel/requests/{id}/reject",
		permMw("estoppel.request.approve", handler.RejectRequest))
}
