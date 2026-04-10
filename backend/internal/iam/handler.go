package iam

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	"github.com/quorant/quorant/internal/platform/api"
	"github.com/quorant/quorant/internal/platform/auth"
)

// Handler handles IAM HTTP requests.
type Handler struct {
	service               *UserService
	logger                *slog.Logger
	zitadelWebhookSecret  string
}

// NewHandler constructs a Handler backed by the given service and logger.
func NewHandler(service *UserService, logger *slog.Logger) *Handler {
	return &Handler{service: service, logger: logger}
}

// NewHandlerWithSecret constructs a Handler with a Zitadel webhook secret for signature verification.
func NewHandlerWithSecret(service *UserService, logger *slog.Logger, zitadelWebhookSecret string) *Handler {
	return &Handler{service: service, logger: logger, zitadelWebhookSecret: zitadelWebhookSecret}
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
		api.WriteError(w, api.NewValidationError("validation.invalid", "", api.P("field", "request")))
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
	body, err := io.ReadAll(r.Body)
	if err != nil {
		api.WriteError(w, api.NewInternalError(err))
		return
	}

	signature := r.Header.Get("X-Zitadel-Signature")
	if signature == "" {
		api.WriteError(w, api.NewUnauthenticatedError("auth.missing_header", api.P("header", "X-Zitadel-Signature")))
		return
	}

	// TODO: implement full HMAC-SHA256 verification using zitadelWebhookSecret
	// once the Zitadel signing scheme is fully integrated.
	h.logger.Info("zitadel webhook received", "signature_present", true, "body_bytes", len(body))

	var payload struct {
		UserID string `json:"user_id"`
		Email  string `json:"email"`
		Name   string `json:"name"`
		Event  string `json:"event"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		api.WriteError(w, api.NewValidationError("validation.invalid", "", api.P("field", "JSON payload")))
		return
	}

	if payload.UserID == "" || payload.Email == "" {
		api.WriteError(w, api.NewValidationError("validation.required", "", api.P("field", "user_id, email")))
		return
	}

	claims := &auth.Claims{
		Subject: payload.UserID,
		Email:   payload.Email,
		Name:    payload.Name,
	}

	_, err = h.service.GetOrCreateUser(r.Context(), claims)
	if err != nil {
		h.logger.Error("webhook user sync", "error", err, "event", payload.Event)
		api.WriteError(w, api.NewInternalError(err))
		return
	}

	api.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
