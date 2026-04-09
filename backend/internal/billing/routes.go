package billing

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/platform/auth"
	"github.com/quorant/quorant/internal/platform/middleware"
)

// RegisterRoutes registers all billing module routes on the mux.
// Org-scoped routes require auth + tenant context middleware.
// The Stripe webhook route has no auth — Stripe calls it directly.
func RegisterRoutes(
	mux *http.ServeMux,
	handler *BillingHandler,
	validator auth.TokenValidator,
	checker middleware.PermissionChecker,
	resolveUserID func(context.Context) (uuid.UUID, error),
) {
	permMw := func(perm string, h http.HandlerFunc) http.Handler {
		return middleware.Auth(validator,
			middleware.TenantContext(
				middleware.RequirePermission(checker, perm, resolveUserID)(
					http.HandlerFunc(h))))
	}

	mux.Handle("GET /api/v1/organizations/{org_id}/billing", permMw("billing.account.manage", handler.GetAccount))
	mux.Handle("PUT /api/v1/organizations/{org_id}/billing", permMw("billing.account.manage", handler.UpdateAccount))
	mux.Handle("GET /api/v1/organizations/{org_id}/invoices", permMw("billing.invoice.read", handler.ListInvoices))
	mux.Handle("GET /api/v1/organizations/{org_id}/invoices/{invoice_id}", permMw("billing.invoice.read", handler.GetInvoice))
	mux.Handle("POST /api/v1/webhooks/stripe", http.HandlerFunc(handler.StripeWebhook)) // no auth — Stripe calls this
}
