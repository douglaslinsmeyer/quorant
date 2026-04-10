package fin

import (
	"net/http"

	"github.com/quorant/quorant/internal/platform/api"
	"github.com/quorant/quorant/internal/platform/middleware"
)

// VoidPayment handles POST /organizations/{org_id}/payments/{payment_id}/void.
func (h *PaymentHandler) VoidPayment(w http.ResponseWriter, r *http.Request) {
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

	userID := middleware.UserIDFromContext(r.Context())
	if err := h.service.VoidPayment(r.Context(), paymentID, userID); err != nil {
		h.logger.Error("VoidPayment failed", "payment_id", paymentID, "error", err)
		api.WriteError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
