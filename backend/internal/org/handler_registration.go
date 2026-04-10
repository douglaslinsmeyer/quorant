package org

import (
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/platform/api"
	"github.com/quorant/quorant/internal/platform/middleware"
)

// RegistrationHandler handles unit registration type and registration HTTP requests.
type RegistrationHandler struct {
	service Service
	logger  *slog.Logger
}

// NewRegistrationHandler constructs a RegistrationHandler backed by the given service.
func NewRegistrationHandler(service Service, logger *slog.Logger) *RegistrationHandler {
	return &RegistrationHandler{service: service, logger: logger}
}

// ─── Registration Types ───────────────────────────────────────────────────────

// CreateRegistrationType handles POST /api/v1/organizations/{org_id}/registration-types.
func (h *RegistrationHandler) CreateRegistrationType(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var req CreateRegistrationTypeRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}

	created, err := h.service.CreateRegistrationType(r.Context(), orgID, req)
	if err != nil {
		h.logger.Error("CreateRegistrationType failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusCreated, created)
}

// ListRegistrationTypes handles GET /api/v1/organizations/{org_id}/registration-types.
func (h *RegistrationHandler) ListRegistrationTypes(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	types, err := h.service.ListRegistrationTypes(r.Context(), orgID)
	if err != nil {
		h.logger.Error("ListRegistrationTypes failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, types)
}

// UpdateRegistrationType handles PATCH /api/v1/organizations/{org_id}/registration-types/{id}.
func (h *RegistrationHandler) UpdateRegistrationType(w http.ResponseWriter, r *http.Request) {
	regTypeID, err := parseRegistrationTypeID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var req UpdateRegistrationTypeRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}

	updated, err := h.service.UpdateRegistrationType(r.Context(), regTypeID, req)
	if err != nil {
		h.logger.Error("UpdateRegistrationType failed", "reg_type_id", regTypeID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, updated)
}

// ─── Registrations ────────────────────────────────────────────────────────────

// CreateRegistration handles POST /api/v1/organizations/{org_id}/units/{unit_id}/registrations.
func (h *RegistrationHandler) CreateRegistration(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	unitID, err := parseUnitID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var req CreateRegistrationRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}

	created, err := h.service.CreateRegistration(r.Context(), orgID, unitID, req)
	if err != nil {
		h.logger.Error("CreateRegistration failed", "unit_id", unitID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusCreated, created)
}

// ListRegistrations handles GET /api/v1/organizations/{org_id}/units/{unit_id}/registrations.
func (h *RegistrationHandler) ListRegistrations(w http.ResponseWriter, r *http.Request) {
	unitID, err := parseUnitID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	registrations, err := h.service.ListRegistrations(r.Context(), unitID)
	if err != nil {
		h.logger.Error("ListRegistrations failed", "unit_id", unitID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, registrations)
}

// UpdateRegistration handles PATCH /api/v1/organizations/{org_id}/registrations/{id}.
func (h *RegistrationHandler) UpdateRegistration(w http.ResponseWriter, r *http.Request) {
	registrationID, err := parseRegistrationID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var req UpdateRegistrationRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}

	updated, err := h.service.UpdateRegistration(r.Context(), registrationID, req)
	if err != nil {
		h.logger.Error("UpdateRegistration failed", "registration_id", registrationID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, updated)
}

// ApproveRegistration handles POST /api/v1/organizations/{org_id}/registrations/{id}/approve.
func (h *RegistrationHandler) ApproveRegistration(w http.ResponseWriter, r *http.Request) {
	registrationID, err := parseRegistrationID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	approverID := middleware.UserIDFromContext(r.Context())

	updated, err := h.service.ApproveRegistration(r.Context(), registrationID, approverID)
	if err != nil {
		h.logger.Error("ApproveRegistration failed", "registration_id", registrationID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, updated)
}

// RevokeRegistration handles POST /api/v1/organizations/{org_id}/registrations/{id}/revoke.
func (h *RegistrationHandler) RevokeRegistration(w http.ResponseWriter, r *http.Request) {
	registrationID, err := parseRegistrationID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	if err := h.service.RevokeRegistration(r.Context(), registrationID); err != nil {
		h.logger.Error("RevokeRegistration failed", "registration_id", registrationID, "error", err)
		api.WriteError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ─── Path value helpers ───────────────────────────────────────────────────────

// parseRegistrationTypeID extracts and parses the {id} path value for registration types.
func parseRegistrationTypeID(r *http.Request) (uuid.UUID, error) {
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

// parseRegistrationID extracts and parses the {id} path value for registrations.
func parseRegistrationID(r *http.Request) (uuid.UUID, error) {
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
