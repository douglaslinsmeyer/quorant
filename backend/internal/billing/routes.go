package billing

import (
	"net/http"

	"github.com/quorant/quorant/internal/platform/auth"
	"github.com/quorant/quorant/internal/platform/middleware"
)

// RegisterRoutes registers all billing module routes on the mux.
// Org-scoped routes require auth + tenant context middleware.
// The Stripe webhook route has no auth — Stripe calls it directly.
func RegisterRoutes(mux *http.ServeMux, handler *BillingHandler, validator auth.TokenValidator) {
	orgMw := func(h http.HandlerFunc) http.Handler {
		return middleware.Auth(validator, middleware.TenantContext(http.HandlerFunc(h)))
	}

	mux.Handle("GET /api/v1/organizations/{org_id}/billing", orgMw(handler.GetAccount))
	mux.Handle("PUT /api/v1/organizations/{org_id}/billing", orgMw(handler.UpdateAccount))
	mux.Handle("GET /api/v1/organizations/{org_id}/invoices", orgMw(handler.ListInvoices))
	mux.Handle("GET /api/v1/organizations/{org_id}/invoices/{invoice_id}", orgMw(handler.GetInvoice))
	mux.Handle("POST /api/v1/webhooks/stripe", http.HandlerFunc(handler.StripeWebhook)) // no auth — Stripe calls this
}
