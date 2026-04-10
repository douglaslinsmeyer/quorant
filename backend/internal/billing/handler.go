package billing

import (
	"io"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/platform/api"
)

// BillingHandler handles HTTP requests for the billing domain.
type BillingHandler struct {
	service             Service
	logger              *slog.Logger
	stripeWebhookSecret string
}

// NewBillingHandler constructs a BillingHandler backed by the given service.
func NewBillingHandler(service Service, logger *slog.Logger) *BillingHandler {
	return &BillingHandler{service: service, logger: logger}
}

// NewBillingHandlerWithSecret constructs a BillingHandler with a Stripe webhook secret for signature verification.
func NewBillingHandlerWithSecret(service Service, logger *slog.Logger, stripeWebhookSecret string) *BillingHandler {
	return &BillingHandler{service: service, logger: logger, stripeWebhookSecret: stripeWebhookSecret}
}

// GetAccount handles GET /api/v1/organizations/{org_id}/billing.
func (h *BillingHandler) GetAccount(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	acct, err := h.service.GetBillingAccount(r.Context(), orgID)
	if err != nil {
		h.logger.Error("GetAccount failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, acct)
}

// UpdateAccount handles PUT /api/v1/organizations/{org_id}/billing.
func (h *BillingHandler) UpdateAccount(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var req UpdateBillingAccountRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}

	updated, err := h.service.UpdateBillingAccount(r.Context(), orgID, req)
	if err != nil {
		h.logger.Error("UpdateAccount failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, updated)
}

// ListInvoices handles GET /api/v1/organizations/{org_id}/invoices.
func (h *BillingHandler) ListInvoices(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	invoices, err := h.service.ListInvoices(r.Context(), orgID)
	if err != nil {
		h.logger.Error("ListInvoices failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, invoices)
}

// GetInvoice handles GET /api/v1/organizations/{org_id}/invoices/{invoice_id}.
func (h *BillingHandler) GetInvoice(w http.ResponseWriter, r *http.Request) {
	invoiceID, err := parseInvoiceID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	inv, err := h.service.GetInvoice(r.Context(), invoiceID)
	if err != nil {
		h.logger.Error("GetInvoice failed", "invoice_id", invoiceID, "error", err)
		api.WriteError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, inv)
}

// StripeWebhook handles POST /api/v1/webhooks/stripe.
func (h *BillingHandler) StripeWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		api.WriteError(w, api.NewInternalError(err))
		return
	}

	sigHeader := r.Header.Get("Stripe-Signature")
	if sigHeader == "" {
		api.WriteError(w, api.NewUnauthenticatedError("auth.missing_header", api.P("header", "Stripe-Signature")))
		return
	}

	// TODO: implement full Stripe signature verification using HMAC-SHA256 with
	// stripeWebhookSecret once the Stripe signing scheme is fully integrated.
	// See: https://stripe.com/docs/webhooks/signatures
	h.logger.Warn("Stripe webhook signature verification not fully implemented",
		"signature_present", true,
		"body_bytes", len(body))

	api.WriteJSON(w, http.StatusOK, map[string]string{"status": "received"})
}

// ─── Path value helpers ───────────────────────────────────────────────────────

func parseOrgID(r *http.Request) (uuid.UUID, error) {
	raw := r.PathValue("org_id")
	if raw == "" {
		return uuid.Nil, api.NewValidationError("validation.required", "org_id", api.P("field", "org_id"))
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, api.NewValidationError("validation.invalid_uuid", "org_id", api.P("field", "org_id"))
	}
	return id, nil
}

func parseInvoiceID(r *http.Request) (uuid.UUID, error) {
	raw := r.PathValue("invoice_id")
	if raw == "" {
		return uuid.Nil, api.NewValidationError("validation.required", "invoice_id", api.P("field", "invoice_id"))
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, api.NewValidationError("validation.invalid_uuid", "invoice_id", api.P("field", "invoice_id"))
	}
	return id, nil
}
