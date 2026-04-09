package org

import (
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/platform/api"
)

// MembershipHandler handles membership HTTP requests.
type MembershipHandler struct {
	service *OrgService
	logger  *slog.Logger
}

// NewMembershipHandler constructs a MembershipHandler backed by the given service.
func NewMembershipHandler(service *OrgService, logger *slog.Logger) *MembershipHandler {
	return &MembershipHandler{service: service, logger: logger}
}

// CreateMembership handles POST /api/v1/organizations/{org_id}/memberships.
func (h *MembershipHandler) CreateMembership(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var req CreateMembershipRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}

	created, err := h.service.CreateMembership(r.Context(), orgID, req)
	if err != nil {
		h.logger.Error("CreateMembership failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusCreated, created)
}

// ListMemberships handles GET /api/v1/organizations/{org_id}/memberships.
func (h *MembershipHandler) ListMemberships(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	memberships, err := h.service.ListMemberships(r.Context(), orgID)
	if err != nil {
		h.logger.Error("ListMemberships failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, memberships)
}

// GetMembership handles GET /api/v1/organizations/{org_id}/memberships/{membership_id}.
func (h *MembershipHandler) GetMembership(w http.ResponseWriter, r *http.Request) {
	membershipID, err := parseMembershipID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	membership, err := h.service.FindMembership(r.Context(), membershipID)
	if err != nil {
		h.logger.Error("GetMembership failed", "membership_id", membershipID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, membership)
}

// UpdateMembership handles PATCH /api/v1/organizations/{org_id}/memberships/{membership_id}.
func (h *MembershipHandler) UpdateMembership(w http.ResponseWriter, r *http.Request) {
	membershipID, err := parseMembershipID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var req UpdateMembershipRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}

	updated, err := h.service.UpdateMembership(r.Context(), membershipID, req.RoleID, req.Status)
	if err != nil {
		h.logger.Error("UpdateMembership failed", "membership_id", membershipID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, updated)
}

// DeleteMembership handles DELETE /api/v1/organizations/{org_id}/memberships/{membership_id}.
func (h *MembershipHandler) DeleteMembership(w http.ResponseWriter, r *http.Request) {
	membershipID, err := parseMembershipID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	if err := h.service.DeleteMembership(r.Context(), membershipID); err != nil {
		h.logger.Error("DeleteMembership failed", "membership_id", membershipID, "error", err)
		api.WriteError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// parseMembershipID extracts and parses the {membership_id} path value from the request.
// Returns a ValidationError if the value is missing or not a valid UUID.
func parseMembershipID(r *http.Request) (uuid.UUID, error) {
	raw := r.PathValue("membership_id")
	if raw == "" {
		return uuid.Nil, api.NewValidationError("membership_id is required", "membership_id")
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, api.NewValidationError("membership_id must be a valid UUID", "membership_id")
	}
	return id, nil
}
