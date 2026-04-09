package com

import (
	"log/slog"
	"net/http"

	"github.com/quorant/quorant/internal/platform/api"
	"github.com/quorant/quorant/internal/platform/middleware"
)

// CommLogHandler handles communication log HTTP requests.
type CommLogHandler struct {
	service *ComService
	logger  *slog.Logger
}

// NewCommLogHandler constructs a CommLogHandler backed by the given service.
func NewCommLogHandler(service *ComService, logger *slog.Logger) *CommLogHandler {
	return &CommLogHandler{service: service, logger: logger}
}

// Log handles POST /api/v1/organizations/{org_id}/communications.
func (h *CommLogHandler) Log(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseComOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var req LogCommunicationRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}

	created, err := h.service.LogCommunication(r.Context(), orgID, req, middleware.UserIDFromContext(r.Context()))
	if err != nil {
		h.logger.Error("LogCommunication failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusCreated, created)
}

// List handles GET /api/v1/organizations/{org_id}/communications.
func (h *CommLogHandler) List(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseComOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	entries, err := h.service.ListCommunications(r.Context(), orgID)
	if err != nil {
		h.logger.Error("ListCommunications failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, entries)
}

// Get handles GET /api/v1/organizations/{org_id}/communications/{comm_id}.
func (h *CommLogHandler) Get(w http.ResponseWriter, r *http.Request) {
	commID, err := parseComPathUUID(r, "comm_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	entry, err := h.service.GetCommunication(r.Context(), commID)
	if err != nil {
		h.logger.Error("GetCommunication failed", "comm_id", commID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, entry)
}

// Update handles PATCH /api/v1/organizations/{org_id}/communications/{comm_id}.
func (h *CommLogHandler) Update(w http.ResponseWriter, r *http.Request) {
	commID, err := parseComPathUUID(r, "comm_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var body CommunicationLog
	if err := api.ReadJSON(r, &body); err != nil {
		api.WriteError(w, err)
		return
	}

	updated, err := h.service.UpdateCommunication(r.Context(), commID, &body)
	if err != nil {
		h.logger.Error("UpdateCommunication failed", "comm_id", commID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, updated)
}

// ListByUnit handles GET /api/v1/organizations/{org_id}/units/{unit_id}/communications.
func (h *CommLogHandler) ListByUnit(w http.ResponseWriter, r *http.Request) {
	unitID, err := parseComPathUUID(r, "unit_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	entries, err := h.service.ListUnitCommunications(r.Context(), unitID)
	if err != nil {
		h.logger.Error("ListUnitCommunications failed", "unit_id", unitID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, entries)
}
