package fin

import (
	"log/slog"
	"net/http"

	"github.com/quorant/quorant/internal/platform/api"
	"github.com/quorant/quorant/internal/platform/middleware"
)

// PaymentHandler handles HTTP requests for payments and payment methods.
type PaymentHandler struct {
	service Service
	logger  *slog.Logger
}

// NewPaymentHandler constructs a PaymentHandler backed by the given service.
func NewPaymentHandler(service Service, logger *slog.Logger) *PaymentHandler {
	return &PaymentHandler{service: service, logger: logger}
}

// ── Payments ──────────────────────────────────────────────────────────────────

// RecordPayment handles POST /organizations/{org_id}/payments.
func (h *PaymentHandler) RecordPayment(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseFinOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var req CreatePaymentRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}

	created, err := h.service.RecordPayment(r.Context(), orgID, middleware.UserIDFromContext(r.Context()), req)
	if err != nil {
		h.logger.Error("RecordPayment failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusCreated, created)
}

// ListPayments handles GET /organizations/{org_id}/payments.
func (h *PaymentHandler) ListPayments(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseFinOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	page := api.ParsePageRequest(r)
	afterID, err := parseFinCursorID(page.Cursor)
	if err != nil {
		api.WriteError(w, api.NewValidationError("validation.invalid_cursor", "cursor"))
		return
	}

	payments, hasMore, err := h.service.ListPayments(r.Context(), orgID, page.Limit, afterID)
	if err != nil {
		h.logger.Error("ListPayments failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	var meta *api.Meta
	if hasMore && len(payments) > 0 {
		meta = &api.Meta{
			Cursor:  api.EncodeCursor(map[string]string{"id": payments[len(payments)-1].ID.String()}),
			HasMore: true,
		}
	}

	api.WriteJSONWithMeta(w, http.StatusOK, payments, meta)
}

// GetPayment handles GET /organizations/{org_id}/payments/{payment_id}.
func (h *PaymentHandler) GetPayment(w http.ResponseWriter, r *http.Request) {
	_, err := parseFinOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	paymentID, err := parsePathUUID(r, "payment_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	payment, err := h.service.GetPayment(r.Context(), paymentID)
	if err != nil {
		h.logger.Error("GetPayment failed", "payment_id", paymentID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, payment)
}

// ── Payment Methods ───────────────────────────────────────────────────────────

// AddPaymentMethod handles POST /organizations/{org_id}/payment-methods.
func (h *PaymentHandler) AddPaymentMethod(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseFinOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var m PaymentMethod
	if err := api.ReadJSON(r, &m); err != nil {
		api.WriteError(w, err)
		return
	}

	created, err := h.service.AddPaymentMethod(r.Context(), orgID, &m)
	if err != nil {
		h.logger.Error("AddPaymentMethod failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusCreated, created)
}

// ListPaymentMethods handles GET /organizations/{org_id}/payment-methods.
func (h *PaymentHandler) ListPaymentMethods(w http.ResponseWriter, r *http.Request) {
	_, err := parseFinOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	methods, err := h.service.ListPaymentMethods(r.Context(), middleware.UserIDFromContext(r.Context()))
	if err != nil {
		h.logger.Error("ListPaymentMethods failed", "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, methods)
}

// RemovePaymentMethod handles DELETE /organizations/{org_id}/payment-methods/{method_id}.
func (h *PaymentHandler) RemovePaymentMethod(w http.ResponseWriter, r *http.Request) {
	_, err := parseFinOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	methodID, err := parsePathUUID(r, "method_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	if err := h.service.RemovePaymentMethod(r.Context(), methodID); err != nil {
		h.logger.Error("RemovePaymentMethod failed", "method_id", methodID, "error", err)
		api.WriteError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
