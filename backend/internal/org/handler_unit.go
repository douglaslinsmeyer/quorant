package org

import (
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/platform/api"
)

// UnitHandler handles unit, property, and unit membership HTTP requests.
type UnitHandler struct {
	service *OrgService
	logger  *slog.Logger
}

// NewUnitHandler constructs a UnitHandler backed by the given service.
func NewUnitHandler(service *OrgService, logger *slog.Logger) *UnitHandler {
	return &UnitHandler{service: service, logger: logger}
}

// ─── Unit CRUD ────────────────────────────────────────────────────────────────

// CreateUnit handles POST /api/v1/organizations/{org_id}/units.
func (h *UnitHandler) CreateUnit(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var req CreateUnitRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}

	created, err := h.service.CreateUnit(r.Context(), orgID, req)
	if err != nil {
		h.logger.Error("CreateUnit failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusCreated, created)
}

// ListUnits handles GET /api/v1/organizations/{org_id}/units.
func (h *UnitHandler) ListUnits(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	page := api.ParsePageRequest(r)

	afterID, err := parseCursorID(page.Cursor)
	if err != nil {
		api.WriteError(w, api.NewValidationError("invalid cursor", "cursor"))
		return
	}

	units, hasMore, err := h.service.ListUnits(r.Context(), orgID, page.Limit, afterID)
	if err != nil {
		h.logger.Error("ListUnits failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	var meta *api.Meta
	if hasMore && len(units) > 0 {
		meta = &api.Meta{
			Cursor:  api.EncodeCursor(map[string]string{"id": units[len(units)-1].ID.String()}),
			HasMore: true,
		}
	}

	api.WriteJSONWithMeta(w, http.StatusOK, units, meta)
}

// GetUnit handles GET /api/v1/organizations/{org_id}/units/{unit_id}.
func (h *UnitHandler) GetUnit(w http.ResponseWriter, r *http.Request) {
	unitID, err := parseUnitID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	unit, err := h.service.GetUnit(r.Context(), unitID)
	if err != nil {
		h.logger.Error("GetUnit failed", "unit_id", unitID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, unit)
}

// UpdateUnit handles PATCH /api/v1/organizations/{org_id}/units/{unit_id}.
func (h *UnitHandler) UpdateUnit(w http.ResponseWriter, r *http.Request) {
	unitID, err := parseUnitID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var req UpdateUnitRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}

	updated, err := h.service.UpdateUnit(r.Context(), unitID, req)
	if err != nil {
		h.logger.Error("UpdateUnit failed", "unit_id", unitID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, updated)
}

// DeleteUnit handles DELETE /api/v1/organizations/{org_id}/units/{unit_id}.
func (h *UnitHandler) DeleteUnit(w http.ResponseWriter, r *http.Request) {
	unitID, err := parseUnitID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	if err := h.service.DeleteUnit(r.Context(), unitID); err != nil {
		h.logger.Error("DeleteUnit failed", "unit_id", unitID, "error", err)
		api.WriteError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ─── Property ─────────────────────────────────────────────────────────────────

// GetProperty handles GET /api/v1/organizations/{org_id}/units/{unit_id}/property.
func (h *UnitHandler) GetProperty(w http.ResponseWriter, r *http.Request) {
	unitID, err := parseUnitID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	prop, err := h.service.GetProperty(r.Context(), unitID)
	if err != nil {
		h.logger.Error("GetProperty failed", "unit_id", unitID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, prop)
}

// SetProperty handles PUT /api/v1/organizations/{org_id}/units/{unit_id}/property.
func (h *UnitHandler) SetProperty(w http.ResponseWriter, r *http.Request) {
	unitID, err := parseUnitID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var req SetPropertyRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}

	prop := &Property{
		ParcelNumber: req.ParcelNumber,
		SquareFeet:   req.SquareFeet,
		Bedrooms:     req.Bedrooms,
		Bathrooms:    req.Bathrooms,
		YearBuilt:    req.YearBuilt,
		Metadata:     req.Metadata,
	}
	if prop.Metadata == nil {
		prop.Metadata = map[string]any{}
	}

	saved, err := h.service.SetProperty(r.Context(), unitID, prop)
	if err != nil {
		h.logger.Error("SetProperty failed", "unit_id", unitID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, saved)
}

// ─── Unit Memberships ─────────────────────────────────────────────────────────

// CreateUnitMembership handles POST /api/v1/organizations/{org_id}/units/{unit_id}/memberships.
func (h *UnitHandler) CreateUnitMembership(w http.ResponseWriter, r *http.Request) {
	unitID, err := parseUnitID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var req CreateUnitMembershipRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}

	created, err := h.service.CreateUnitMembership(r.Context(), unitID, req)
	if err != nil {
		h.logger.Error("CreateUnitMembership failed", "unit_id", unitID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusCreated, created)
}

// ListUnitMemberships handles GET /api/v1/organizations/{org_id}/units/{unit_id}/memberships.
func (h *UnitHandler) ListUnitMemberships(w http.ResponseWriter, r *http.Request) {
	unitID, err := parseUnitID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	memberships, err := h.service.ListUnitMemberships(r.Context(), unitID)
	if err != nil {
		h.logger.Error("ListUnitMemberships failed", "unit_id", unitID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, memberships)
}

// UpdateUnitMembership handles PATCH /api/v1/organizations/{org_id}/units/{unit_id}/memberships/{id}.
func (h *UnitHandler) UpdateUnitMembership(w http.ResponseWriter, r *http.Request) {
	unitMembershipID, err := parseUnitMembershipID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var req UpdateUnitMembershipRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}

	updated, err := h.service.UpdateUnitMembership(r.Context(), unitMembershipID, req)
	if err != nil {
		h.logger.Error("UpdateUnitMembership failed", "unit_membership_id", unitMembershipID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, updated)
}

// EndUnitMembership handles DELETE /api/v1/organizations/{org_id}/units/{unit_id}/memberships/{id}.
func (h *UnitHandler) EndUnitMembership(w http.ResponseWriter, r *http.Request) {
	unitMembershipID, err := parseUnitMembershipID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	if err := h.service.EndUnitMembership(r.Context(), unitMembershipID); err != nil {
		h.logger.Error("EndUnitMembership failed", "unit_membership_id", unitMembershipID, "error", err)
		api.WriteError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ─── Path value helpers ───────────────────────────────────────────────────────

// parseUnitID extracts and parses the {unit_id} path value from the request.
// Returns a ValidationError if the value is missing or not a valid UUID.
func parseUnitID(r *http.Request) (uuid.UUID, error) {
	raw := r.PathValue("unit_id")
	if raw == "" {
		return uuid.Nil, api.NewValidationError("unit_id is required", "unit_id")
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, api.NewValidationError("unit_id must be a valid UUID", "unit_id")
	}
	return id, nil
}

// parseUnitMembershipID extracts and parses the {id} path value for unit memberships.
// Returns a ValidationError if the value is missing or not a valid UUID.
func parseUnitMembershipID(r *http.Request) (uuid.UUID, error) {
	raw := r.PathValue("id")
	if raw == "" {
		return uuid.Nil, api.NewValidationError("id is required", "id")
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, api.NewValidationError("id must be a valid UUID", "id")
	}
	return id, nil
}
