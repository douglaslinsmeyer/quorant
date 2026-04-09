package org

import (
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/platform/api"
)

// OrgHandler handles organization HTTP requests.
type OrgHandler struct {
	service *OrgService
	logger  *slog.Logger
}

// NewOrgHandler constructs an OrgHandler backed by the given service.
func NewOrgHandler(service *OrgService, logger *slog.Logger) *OrgHandler {
	return &OrgHandler{service: service, logger: logger}
}

// CreateOrg handles POST /api/v1/organizations.
func (h *OrgHandler) CreateOrg(w http.ResponseWriter, r *http.Request) {
	var req CreateOrgRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}

	created, err := h.service.CreateOrganization(r.Context(), req)
	if err != nil {
		h.logger.Error("CreateOrg failed", "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusCreated, created)
}

// ListOrgs handles GET /api/v1/organizations.
func (h *OrgHandler) ListOrgs(w http.ResponseWriter, r *http.Request) {
	orgs, err := h.service.ListOrganizations(r.Context())
	if err != nil {
		h.logger.Error("ListOrgs failed", "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, orgs)
}

// GetOrg handles GET /api/v1/organizations/{org_id}.
func (h *OrgHandler) GetOrg(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	org, err := h.service.GetOrganization(r.Context(), orgID)
	if err != nil {
		h.logger.Error("GetOrg failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, org)
}

// UpdateOrg handles PATCH /api/v1/organizations/{org_id}.
func (h *OrgHandler) UpdateOrg(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var req UpdateOrgRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}

	updated, err := h.service.UpdateOrganization(r.Context(), orgID, req)
	if err != nil {
		h.logger.Error("UpdateOrg failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, updated)
}

// DeleteOrg handles DELETE /api/v1/organizations/{org_id}.
func (h *OrgHandler) DeleteOrg(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	if err := h.service.DeleteOrganization(r.Context(), orgID); err != nil {
		h.logger.Error("DeleteOrg failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListChildren handles GET /api/v1/organizations/{org_id}/children.
func (h *OrgHandler) ListChildren(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	children, err := h.service.ListChildren(r.Context(), orgID)
	if err != nil {
		h.logger.Error("ListChildren failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, children)
}

// ConnectManagement handles POST /api/v1/organizations/{org_id}/management.
func (h *OrgHandler) ConnectManagement(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var req ConnectManagementRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}

	mgmt, err := h.service.ConnectManagement(r.Context(), orgID, req)
	if err != nil {
		h.logger.Error("ConnectManagement failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusCreated, mgmt)
}

// DisconnectManagement handles DELETE /api/v1/organizations/{org_id}/management.
func (h *OrgHandler) DisconnectManagement(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	if err := h.service.DisconnectManagement(r.Context(), orgID); err != nil {
		h.logger.Error("DisconnectManagement failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetManagementHistory handles GET /api/v1/organizations/{org_id}/management/history.
func (h *OrgHandler) GetManagementHistory(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	history, err := h.service.GetManagementHistory(r.Context(), orgID)
	if err != nil {
		h.logger.Error("GetManagementHistory failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, history)
}

// parseOrgID extracts and parses the {org_id} path value from the request.
// Returns a ValidationError if the value is missing or not a valid UUID.
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
