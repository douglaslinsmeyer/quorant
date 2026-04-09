package webhook

import (
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/platform/api"
	"github.com/quorant/quorant/internal/platform/middleware"
)

// WebhookHandler handles HTTP requests for the webhook module.
type WebhookHandler struct {
	service *WebhookService
	logger  *slog.Logger
}

// NewWebhookHandler constructs a WebhookHandler backed by the given service and logger.
func NewWebhookHandler(service *WebhookService, logger *slog.Logger) *WebhookHandler {
	return &WebhookHandler{service: service, logger: logger}
}

// Create handles POST /api/v1/organizations/{org_id}/webhooks.
func (h *WebhookHandler) Create(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseWebhookPathUUID(r, "org_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var req CreateSubscriptionRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}

	created, err := h.service.CreateSubscription(r.Context(), orgID, middleware.UserIDFromContext(r.Context()), req)
	if err != nil {
		h.logger.Error("Create webhook subscription failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusCreated, created)
}

// List handles GET /api/v1/organizations/{org_id}/webhooks.
func (h *WebhookHandler) List(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseWebhookPathUUID(r, "org_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	subs, err := h.service.ListSubscriptions(r.Context(), orgID)
	if err != nil {
		h.logger.Error("List webhook subscriptions failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, subs)
}

// ListEventTypes handles GET /api/v1/organizations/{org_id}/webhooks/event-types.
func (h *WebhookHandler) ListEventTypes(w http.ResponseWriter, r *http.Request) {
	types := h.service.GetEventTypes(r.Context())
	api.WriteJSON(w, http.StatusOK, types)
}

// Get handles GET /api/v1/organizations/{org_id}/webhooks/{webhook_id}.
func (h *WebhookHandler) Get(w http.ResponseWriter, r *http.Request) {
	webhookID, err := parseWebhookPathUUID(r, "webhook_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	sub, err := h.service.GetSubscription(r.Context(), webhookID)
	if err != nil {
		h.logger.Error("Get webhook subscription failed", "webhook_id", webhookID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, sub)
}

// Update handles PATCH /api/v1/organizations/{org_id}/webhooks/{webhook_id}.
func (h *WebhookHandler) Update(w http.ResponseWriter, r *http.Request) {
	webhookID, err := parseWebhookPathUUID(r, "webhook_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var req UpdateSubscriptionRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}

	updated, err := h.service.UpdateSubscription(r.Context(), webhookID, req)
	if err != nil {
		h.logger.Error("Update webhook subscription failed", "webhook_id", webhookID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, updated)
}

// Delete handles DELETE /api/v1/organizations/{org_id}/webhooks/{webhook_id}.
func (h *WebhookHandler) Delete(w http.ResponseWriter, r *http.Request) {
	webhookID, err := parseWebhookPathUUID(r, "webhook_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	if err := h.service.DeleteSubscription(r.Context(), webhookID); err != nil {
		h.logger.Error("Delete webhook subscription failed", "webhook_id", webhookID, "error", err)
		api.WriteError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListDeliveries handles GET /api/v1/organizations/{org_id}/webhooks/{webhook_id}/deliveries.
func (h *WebhookHandler) ListDeliveries(w http.ResponseWriter, r *http.Request) {
	webhookID, err := parseWebhookPathUUID(r, "webhook_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	deliveries, err := h.service.ListDeliveries(r.Context(), webhookID)
	if err != nil {
		h.logger.Error("List webhook deliveries failed", "webhook_id", webhookID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, deliveries)
}

// TestEvent handles POST /api/v1/organizations/{org_id}/webhooks/{webhook_id}/test.
func (h *WebhookHandler) TestEvent(w http.ResponseWriter, r *http.Request) {
	webhookID, err := parseWebhookPathUUID(r, "webhook_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	delivery, err := h.service.SendTestEvent(r.Context(), webhookID)
	if err != nil {
		h.logger.Error("Send webhook test event failed", "webhook_id", webhookID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, delivery)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// parseWebhookPathUUID extracts and parses a UUID path value by the given key.
func parseWebhookPathUUID(r *http.Request, key string) (uuid.UUID, error) {
	raw := r.PathValue(key)
	if raw == "" {
		return uuid.Nil, api.NewValidationError(key+" is required", key)
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, api.NewValidationError(key+" must be a valid UUID", key)
	}
	return id, nil
}
