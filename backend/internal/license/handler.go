package license

import (
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/platform/api"
)

// LicenseHandler handles HTTP requests for the license domain.
type LicenseHandler struct {
	service Service
	logger  *slog.Logger
}

// NewLicenseHandler constructs a LicenseHandler backed by the given service.
func NewLicenseHandler(service Service, logger *slog.Logger) *LicenseHandler {
	return &LicenseHandler{service: service, logger: logger}
}

// ─── Admin plan handlers ──────────────────────────────────────────────────────

// ListPlans handles GET /api/v1/admin/plans.
func (h *LicenseHandler) ListPlans(w http.ResponseWriter, r *http.Request) {
	plans, err := h.service.ListPlans(r.Context())
	if err != nil {
		h.logger.Error("ListPlans failed", "error", err)
		api.WriteError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, plans)
}

// CreatePlan handles POST /api/v1/admin/plans.
func (h *LicenseHandler) CreatePlan(w http.ResponseWriter, r *http.Request) {
	var req CreatePlanRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}

	plan, err := h.service.CreatePlan(r.Context(), req)
	if err != nil {
		h.logger.Error("CreatePlan failed", "error", err)
		api.WriteError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusCreated, plan)
}

// UpdatePlan handles PATCH /api/v1/admin/plans/{plan_id}.
func (h *LicenseHandler) UpdatePlan(w http.ResponseWriter, r *http.Request) {
	planID, err := parsePlanID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var p Plan
	if err := api.ReadJSON(r, &p); err != nil {
		api.WriteError(w, err)
		return
	}

	updated, err := h.service.UpdatePlan(r.Context(), planID, &p)
	if err != nil {
		h.logger.Error("UpdatePlan failed", "plan_id", planID, "error", err)
		api.WriteError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, updated)
}

// ListEntitlements handles GET /api/v1/admin/plans/{plan_id}/entitlements.
func (h *LicenseHandler) ListEntitlements(w http.ResponseWriter, r *http.Request) {
	planID, err := parsePlanID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	ents, err := h.service.ListPlanEntitlements(r.Context(), planID)
	if err != nil {
		h.logger.Error("ListEntitlements failed", "plan_id", planID, "error", err)
		api.WriteError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, ents)
}

// ─── Org subscription handlers ────────────────────────────────────────────────

// CreateSubscription handles POST /api/v1/organizations/{org_id}/subscription.
func (h *LicenseHandler) CreateSubscription(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var req CreateSubscriptionRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}

	sub, err := h.service.CreateSubscription(r.Context(), orgID, req)
	if err != nil {
		h.logger.Error("CreateSubscription failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusCreated, sub)
}

// GetSubscription handles GET /api/v1/organizations/{org_id}/subscription.
func (h *LicenseHandler) GetSubscription(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	sub, err := h.service.GetSubscription(r.Context(), orgID)
	if err != nil {
		h.logger.Error("GetSubscription failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, sub)
}

// UpdateSubscription handles PATCH /api/v1/organizations/{org_id}/subscription.
func (h *LicenseHandler) UpdateSubscription(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var s OrgSubscription
	if err := api.ReadJSON(r, &s); err != nil {
		api.WriteError(w, err)
		return
	}

	updated, err := h.service.UpdateSubscription(r.Context(), orgID, &s)
	if err != nil {
		h.logger.Error("UpdateSubscription failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, updated)
}

// CheckEntitlements handles GET /api/v1/organizations/{org_id}/entitlements.
func (h *LicenseHandler) CheckEntitlements(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	results, err := h.service.CheckEntitlements(r.Context(), orgID)
	if err != nil {
		h.logger.Error("CheckEntitlements failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, results)
}

// SetOverride handles POST /api/v1/admin/organizations/{org_id}/entitlement-overrides.
func (h *LicenseHandler) SetOverride(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var req UpsertOverrideRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}

	override, err := h.service.SetOverride(r.Context(), orgID, req)
	if err != nil {
		h.logger.Error("SetOverride failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusCreated, override)
}

// GetUsage handles GET /api/v1/organizations/{org_id}/usage.
func (h *LicenseHandler) GetUsage(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	records, err := h.service.GetUsage(r.Context(), orgID)
	if err != nil {
		h.logger.Error("GetUsage failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, records)
}

// ─── Path value helpers ───────────────────────────────────────────────────────

func parsePlanID(r *http.Request) (uuid.UUID, error) {
	raw := r.PathValue("plan_id")
	if raw == "" {
		return uuid.Nil, api.NewValidationError("plan_id is required", "plan_id")
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, api.NewValidationError("plan_id must be a valid UUID", "plan_id")
	}
	return id, nil
}

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
