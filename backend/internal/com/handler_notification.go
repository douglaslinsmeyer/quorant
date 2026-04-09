package com

import (
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/platform/api"
)

// NotificationHandler handles notification preference and push token HTTP requests.
type NotificationHandler struct {
	service *ComService
	logger  *slog.Logger
}

// NewNotificationHandler constructs a NotificationHandler backed by the given service.
func NewNotificationHandler(service *ComService, logger *slog.Logger) *NotificationHandler {
	return &NotificationHandler{service: service, logger: logger}
}

// GetPrefs handles GET /api/v1/notification-preferences.
func (h *NotificationHandler) GetPrefs(w http.ResponseWriter, r *http.Request) {
	// TODO: extract real user ID and org ID from auth context
	prefs, err := h.service.GetNotificationPreferences(r.Context(), uuid.Nil, uuid.Nil)
	if err != nil {
		h.logger.Error("GetNotificationPreferences failed", "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, prefs)
}

// UpdatePrefs handles PUT /api/v1/notification-preferences.
func (h *NotificationHandler) UpdatePrefs(w http.ResponseWriter, r *http.Request) {
	var prefs []NotificationPreference
	if err := api.ReadJSON(r, &prefs); err != nil {
		api.WriteError(w, err)
		return
	}

	if err := h.service.UpdateNotificationPreferences(r.Context(), prefs); err != nil {
		h.logger.Error("UpdateNotificationPreferences failed", "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, prefs)
}

// RegisterToken handles POST /api/v1/push-tokens.
func (h *NotificationHandler) RegisterToken(w http.ResponseWriter, r *http.Request) {
	var token PushToken
	if err := api.ReadJSON(r, &token); err != nil {
		api.WriteError(w, err)
		return
	}

	// TODO: extract real user ID from auth context
	token.UserID = uuid.Nil

	created, err := h.service.RegisterPushToken(r.Context(), &token)
	if err != nil {
		h.logger.Error("RegisterPushToken failed", "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusCreated, created)
}

// UnregisterToken handles DELETE /api/v1/push-tokens/{token_id}.
func (h *NotificationHandler) UnregisterToken(w http.ResponseWriter, r *http.Request) {
	tokenID, err := parseComPathUUID(r, "token_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	if err := h.service.UnregisterPushToken(r.Context(), tokenID); err != nil {
		h.logger.Error("UnregisterPushToken failed", "token_id", tokenID, "error", err)
		api.WriteError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
