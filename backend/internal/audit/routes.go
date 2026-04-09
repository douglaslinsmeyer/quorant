package audit

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/platform/auth"
	"github.com/quorant/quorant/internal/platform/middleware"
)

// RegisterRoutes registers audit log API routes.
func RegisterRoutes(mux *http.ServeMux, handler *Handler, validator auth.TokenValidator, checker middleware.PermissionChecker, resolveUserID func(ctx context.Context) (uuid.UUID, error)) {
	permMw := func(perm string, h http.HandlerFunc) http.Handler {
		return middleware.Auth(validator,
			middleware.TenantContext(
				middleware.RequirePermission(checker, perm, resolveUserID)(
					http.HandlerFunc(h))))
	}

	mux.Handle("GET /api/v1/organizations/{org_id}/audit-log", permMw("audit.log.read", handler.ListAuditLog))
	mux.Handle("GET /api/v1/organizations/{org_id}/audit-log/{event_id}", permMw("audit.log.read", handler.GetAuditEntry))
}
