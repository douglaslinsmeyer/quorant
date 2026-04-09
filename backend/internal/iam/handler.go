package iam

import (
	"log/slog"
	"net/http"

	"github.com/quorant/quorant/internal/platform/api"
	"github.com/quorant/quorant/internal/platform/auth"
)

// Handler handles IAM HTTP requests.
type Handler struct {
	service *UserService
	logger  *slog.Logger
}

// NewHandler constructs a Handler backed by the given service and logger.
func NewHandler(service *UserService, logger *slog.Logger) *Handler {
	return &Handler{service: service, logger: logger}
}

// GetMe handles GET /api/v1/auth/me
// Returns the current user's profile with memberships.
func (h *Handler) GetMe(w http.ResponseWriter, r *http.Request) {
	profile, err := h.service.GetCurrentUser(r.Context())
	if err != nil {
		h.logger.Error("getting current user", "error", err)
		api.WriteError(w, api.NewInternalError(err))
		return
	}
	api.WriteJSON(w, http.StatusOK, profile)
}

// UpdateMe handles PATCH /api/v1/auth/me
// Updates the current user's profile fields.
func (h *Handler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	var req UpdateProfileRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}
	if err := req.Validate(); err != nil {
		api.WriteError(w, api.NewValidationError(err.Error(), ""))
		return
	}

	profile, err := h.service.UpdateProfile(r.Context(), req)
	if err != nil {
		h.logger.Error("updating profile", "error", err)
		api.WriteError(w, api.NewInternalError(err))
		return
	}
	api.WriteJSON(w, http.StatusOK, profile)
}

// ZitadelWebhook handles POST /api/v1/webhooks/zitadel
// Syncs user data when Zitadel sends webhook events (user.created, user.changed, etc.)
func (h *Handler) ZitadelWebhook(w http.ResponseWriter, r *http.Request) {
	// For Phase 2, implement a basic handler that:
	// 1. Reads a simple JSON payload: {"user_id": "...", "email": "...", "name": "...", "event": "user.created|user.changed"}
	// 2. Upserts the user via the service
	// Full Zitadel webhook verification (HMAC signature) will be added later.

	var payload struct {
		UserID string `json:"user_id"`
		Email  string `json:"email"`
		Name   string `json:"name"`
		Event  string `json:"event"`
	}
	if err := api.ReadJSON(r, &payload); err != nil {
		api.WriteError(w, err)
		return
	}

	if payload.UserID == "" || payload.Email == "" {
		api.WriteError(w, api.NewValidationError("user_id and email are required", ""))
		return
	}

	claims := &auth.Claims{
		Subject: payload.UserID,
		Email:   payload.Email,
		Name:    payload.Name,
	}

	_, err := h.service.GetOrCreateUser(r.Context(), claims)
	if err != nil {
		h.logger.Error("webhook user sync", "error", err, "event", payload.Event)
		api.WriteError(w, api.NewInternalError(err))
		return
	}

	api.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
