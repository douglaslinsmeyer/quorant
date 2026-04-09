package webhook

import (
	"net/http"

	"github.com/quorant/quorant/internal/platform/auth"
	"github.com/quorant/quorant/internal/platform/middleware"
)

// RegisterRoutes registers all webhook routes on the mux.
// All routes are auth-protected and include tenant context.
//
// IMPORTANT: event-types must be registered before the {webhook_id} catch-all
// routes, otherwise the literal string "event-types" would be matched as a
// webhook_id path value.
func RegisterRoutes(mux *http.ServeMux, handler *WebhookHandler, validator auth.TokenValidator) {
	orgMw := func(h http.HandlerFunc) http.Handler {
		return middleware.Auth(validator, middleware.TenantContext(http.HandlerFunc(h)))
	}

	// Collection endpoints
	mux.Handle("POST /api/v1/organizations/{org_id}/webhooks", orgMw(handler.Create))
	mux.Handle("GET /api/v1/organizations/{org_id}/webhooks", orgMw(handler.List))

	// Static sub-route — must precede {webhook_id} pattern.
	mux.Handle("GET /api/v1/organizations/{org_id}/webhooks/event-types", orgMw(handler.ListEventTypes))

	// Item endpoints
	mux.Handle("GET /api/v1/organizations/{org_id}/webhooks/{webhook_id}", orgMw(handler.Get))
	mux.Handle("PATCH /api/v1/organizations/{org_id}/webhooks/{webhook_id}", orgMw(handler.Update))
	mux.Handle("DELETE /api/v1/organizations/{org_id}/webhooks/{webhook_id}", orgMw(handler.Delete))
	mux.Handle("GET /api/v1/organizations/{org_id}/webhooks/{webhook_id}/deliveries", orgMw(handler.ListDeliveries))
	mux.Handle("POST /api/v1/organizations/{org_id}/webhooks/{webhook_id}/test", orgMw(handler.TestEvent))
}
