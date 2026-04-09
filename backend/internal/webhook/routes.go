package webhook

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/platform/auth"
	"github.com/quorant/quorant/internal/platform/middleware"
)

// RegisterRoutes registers all webhook routes on the mux.
// All routes are auth-protected and include tenant context.
// All webhook endpoints are gated by the webhooks.enabled entitlement.
//
// IMPORTANT: event-types must be registered before the {webhook_id} catch-all
// routes, otherwise the literal string "event-types" would be matched as a
// webhook_id path value.
func RegisterRoutes(
	mux *http.ServeMux,
	handler *WebhookHandler,
	validator auth.TokenValidator,
	checker middleware.PermissionChecker,
	resolveUserID func(context.Context) (uuid.UUID, error),
	entChecker middleware.EntitlementChecker,
) {
	permMw := func(perm string, h http.HandlerFunc) http.Handler {
		return middleware.Auth(validator,
			middleware.TenantContext(
				middleware.RequireEntitlement(entChecker, "webhooks.enabled")(
					middleware.RequirePermission(checker, perm, resolveUserID)(
						http.HandlerFunc(h)))))
	}

	// Collection endpoints
	mux.Handle("POST /api/v1/organizations/{org_id}/webhooks", permMw("webhook.manage", handler.Create))
	mux.Handle("GET /api/v1/organizations/{org_id}/webhooks", permMw("webhook.read", handler.List))

	// Static sub-route — must precede {webhook_id} pattern.
	mux.Handle("GET /api/v1/organizations/{org_id}/webhooks/event-types", permMw("webhook.read", handler.ListEventTypes))

	// Item endpoints
	mux.Handle("GET /api/v1/organizations/{org_id}/webhooks/{webhook_id}", permMw("webhook.read", handler.Get))
	mux.Handle("PATCH /api/v1/organizations/{org_id}/webhooks/{webhook_id}", permMw("webhook.manage", handler.Update))
	mux.Handle("DELETE /api/v1/organizations/{org_id}/webhooks/{webhook_id}", permMw("webhook.manage", handler.Delete))
	mux.Handle("GET /api/v1/organizations/{org_id}/webhooks/{webhook_id}/deliveries", permMw("webhook.read", handler.ListDeliveries))
	mux.Handle("POST /api/v1/organizations/{org_id}/webhooks/{webhook_id}/test", permMw("webhook.manage", handler.TestEvent))
}
