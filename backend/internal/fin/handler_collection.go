package fin

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/quorant/quorant/internal/platform/api"
)

// CollectionHandler handles HTTP requests for collection cases, actions,
// and payment plans.
type CollectionHandler struct {
	service *FinService
	logger  *slog.Logger
}

// NewCollectionHandler constructs a CollectionHandler backed by the given service.
func NewCollectionHandler(service *FinService, logger *slog.Logger) *CollectionHandler {
	return &CollectionHandler{service: service, logger: logger}
}

// ── Collection Cases ──────────────────────────────────────────────────────────

// ListCollections handles GET /organizations/{org_id}/collections.
func (h *CollectionHandler) ListCollections(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseFinOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	cases, err := h.service.ListCollections(r.Context(), orgID)
	if err != nil {
		h.logger.Error("ListCollections failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, cases)
}

// GetCollection handles GET /organizations/{org_id}/collections/{case_id}.
func (h *CollectionHandler) GetCollection(w http.ResponseWriter, r *http.Request) {
	_, err := parseFinOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	caseID, err := parsePathUUID(r, "case_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	c, err := h.service.GetCollection(r.Context(), caseID)
	if err != nil {
		h.logger.Error("GetCollection failed", "case_id", caseID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, c)
}

// UpdateCollection handles PATCH /organizations/{org_id}/collections/{case_id}.
func (h *CollectionHandler) UpdateCollection(w http.ResponseWriter, r *http.Request) {
	_, err := parseFinOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	caseID, err := parsePathUUID(r, "case_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var req UpdateCollectionRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}

	// Fetch the existing case and apply partial updates.
	existing, err := h.service.GetCollection(r.Context(), caseID)
	if err != nil {
		h.logger.Error("UpdateCollection fetch failed", "case_id", caseID, "error", err)
		api.WriteError(w, err)
		return
	}

	if req.Status != nil {
		existing.Status = *req.Status
		if *req.Status != "" && existing.ClosedAt == nil &&
			(*req.Status == "closed" || *req.Status == "resolved") {
			now := time.Now()
			existing.ClosedAt = &now
		}
	}
	if req.EscalationPaused != nil {
		existing.EscalationPaused = *req.EscalationPaused
	}
	if req.PauseReason != nil {
		existing.PauseReason = req.PauseReason
	}
	if req.AssignedTo != nil {
		existing.AssignedTo = req.AssignedTo
	}
	if req.ClosedReason != nil {
		existing.ClosedReason = req.ClosedReason
	}

	updated, err := h.service.UpdateCollection(r.Context(), caseID, existing)
	if err != nil {
		h.logger.Error("UpdateCollection failed", "case_id", caseID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, updated)
}

// ── Collection Actions ────────────────────────────────────────────────────────

// AddCollectionAction handles POST /organizations/{org_id}/collections/{case_id}/actions.
func (h *CollectionHandler) AddCollectionAction(w http.ResponseWriter, r *http.Request) {
	_, err := parseFinOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	caseID, err := parsePathUUID(r, "case_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var req CreateCollectionActionRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}

	created, err := h.service.AddCollectionAction(r.Context(), caseID, req)
	if err != nil {
		h.logger.Error("AddCollectionAction failed", "case_id", caseID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusCreated, created)
}

// ── Payment Plans ─────────────────────────────────────────────────────────────

// CreatePaymentPlan handles POST /organizations/{org_id}/collections/{case_id}/payment-plans.
func (h *CollectionHandler) CreatePaymentPlan(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseFinOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	caseID, err := parsePathUUID(r, "case_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var req CreatePaymentPlanRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}

	// Fetch the case to obtain the unit ID required for the payment plan.
	c, err := h.service.GetCollection(r.Context(), caseID)
	if err != nil {
		h.logger.Error("CreatePaymentPlan: fetch case failed", "case_id", caseID, "error", err)
		api.WriteError(w, err)
		return
	}

	created, err := h.service.CreatePaymentPlan(r.Context(), caseID, orgID, c.UnitID, req)
	if err != nil {
		h.logger.Error("CreatePaymentPlan failed", "case_id", caseID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusCreated, created)
}

// UpdatePaymentPlan handles PATCH /organizations/{org_id}/payment-plans/{plan_id}.
func (h *CollectionHandler) UpdatePaymentPlan(w http.ResponseWriter, r *http.Request) {
	_, err := parseFinOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	planID, err := parsePathUUID(r, "plan_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var p PaymentPlan
	if err := api.ReadJSON(r, &p); err != nil {
		api.WriteError(w, err)
		return
	}

	updated, err := h.service.UpdatePaymentPlan(r.Context(), planID, &p)
	if err != nil {
		h.logger.Error("UpdatePaymentPlan failed", "plan_id", planID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, updated)
}

// ListPaymentPlans handles GET /organizations/{org_id}/collections/{case_id}/payment-plans.
func (h *CollectionHandler) ListPaymentPlans(w http.ResponseWriter, r *http.Request) {
	_, err := parseFinOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	caseID, err := parsePathUUID(r, "case_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	plans, err := h.service.ListPaymentPlans(r.Context(), caseID)
	if err != nil {
		h.logger.Error("ListPaymentPlans failed", "case_id", caseID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, plans)
}

// ── Unit Collection Status ────────────────────────────────────────────────────

// GetUnitCollectionStatus handles GET /organizations/{org_id}/units/{unit_id}/collection-status.
func (h *CollectionHandler) GetUnitCollectionStatus(w http.ResponseWriter, r *http.Request) {
	_, err := parseFinOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	unitID, err := parsePathUUID(r, "unit_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	c, err := h.service.GetUnitCollectionStatus(r.Context(), unitID)
	if err != nil {
		h.logger.Error("GetUnitCollectionStatus failed", "unit_id", unitID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, c)
}
