package com

import (
	"log/slog"
	"net/http"

	"github.com/quorant/quorant/internal/platform/api"
	"github.com/quorant/quorant/internal/platform/middleware"
)

// ThreadHandler handles thread and message HTTP requests.
type ThreadHandler struct {
	service *ComService
	logger  *slog.Logger
}

// NewThreadHandler constructs a ThreadHandler backed by the given service.
func NewThreadHandler(service *ComService, logger *slog.Logger) *ThreadHandler {
	return &ThreadHandler{service: service, logger: logger}
}

// CreateThread handles POST /api/v1/organizations/{org_id}/threads.
func (h *ThreadHandler) CreateThread(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseComOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var req CreateThreadRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}

	created, err := h.service.CreateThread(r.Context(), orgID, req, middleware.UserIDFromContext(r.Context()))
	if err != nil {
		h.logger.Error("CreateThread failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusCreated, created)
}

// ListThreads handles GET /api/v1/organizations/{org_id}/threads.
func (h *ThreadHandler) ListThreads(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseComOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	threads, err := h.service.ListThreads(r.Context(), orgID)
	if err != nil {
		h.logger.Error("ListThreads failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, threads)
}

// GetThread handles GET /api/v1/organizations/{org_id}/threads/{thread_id}.
func (h *ThreadHandler) GetThread(w http.ResponseWriter, r *http.Request) {
	threadID, err := parseComPathUUID(r, "thread_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	thread, err := h.service.GetThread(r.Context(), threadID)
	if err != nil {
		h.logger.Error("GetThread failed", "thread_id", threadID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, thread)
}

// SendMessage handles POST /api/v1/organizations/{org_id}/threads/{thread_id}/messages.
func (h *ThreadHandler) SendMessage(w http.ResponseWriter, r *http.Request) {
	threadID, err := parseComPathUUID(r, "thread_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var req SendMessageRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}

	created, err := h.service.SendMessage(r.Context(), threadID, req, middleware.UserIDFromContext(r.Context()))
	if err != nil {
		h.logger.Error("SendMessage failed", "thread_id", threadID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusCreated, created)
}

// EditMessage handles PATCH /api/v1/organizations/{org_id}/threads/{thread_id}/messages/{message_id}.
func (h *ThreadHandler) EditMessage(w http.ResponseWriter, r *http.Request) {
	messageID, err := parseComPathUUID(r, "message_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var req SendMessageRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}

	updated, err := h.service.EditMessage(r.Context(), messageID, req.Body)
	if err != nil {
		h.logger.Error("EditMessage failed", "message_id", messageID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, updated)
}

// DeleteMessage handles DELETE /api/v1/organizations/{org_id}/threads/{thread_id}/messages/{message_id}.
func (h *ThreadHandler) DeleteMessage(w http.ResponseWriter, r *http.Request) {
	messageID, err := parseComPathUUID(r, "message_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	if err := h.service.DeleteMessage(r.Context(), messageID); err != nil {
		h.logger.Error("DeleteMessage failed", "message_id", messageID, "error", err)
		api.WriteError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
