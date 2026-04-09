package billing

import (
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/platform/api"
)

// BillingHandler handles HTTP requests for the billing domain.
type BillingHandler struct {
	service *BillingService
	logger  *slog.Logger
}

// NewBillingHandler constructs a BillingHandler backed by the given service.
func NewBillingHandler(service *BillingService, logger *slog.Logger) *BillingHandler {
	return &BillingHandler{service: service, logger: logger}
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
// This is a placeholder — Stripe webhook verification and event handling will be
// implemented in a future phase.
func (h *BillingHandler) StripeWebhook(w http.ResponseWriter, r *http.Request) {
	// TODO: implement Stripe webhook verification and event handling
	api.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ─── Path value helpers ───────────────────────────────────────────────────────

func parseOrgID(r *http.Request) (uuid.UUID, error) {
	raw := r.PathValue("org_id")
	if raw == "" {
		return uuid.Nil, api.NewValidationError("org_id is required", "org_id")
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, api.NewValidationError("org_id must be a valid UUID", "org_id")
	}
	return id, nil
}

func parseInvoiceID(r *http.Request) (uuid.UUID, error) {
	raw := r.PathValue("invoice_id")
	if raw == "" {
		return uuid.Nil, api.NewValidationError("invoice_id is required", "invoice_id")
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, api.NewValidationError("invoice_id must be a valid UUID", "invoice_id")
	}
	return id, nil
}
