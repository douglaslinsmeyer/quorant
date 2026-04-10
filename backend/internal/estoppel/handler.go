package estoppel

import (
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/platform/api"
	"github.com/quorant/quorant/internal/platform/middleware"
)

// Handler handles HTTP requests for the Estoppel module.
type Handler struct {
	service *EstoppelService
	logger  *slog.Logger
}

// NewHandler creates a Handler backed by the given service.
func NewHandler(service *EstoppelService, logger *slog.Logger) *Handler {
	return &Handler{service: service, logger: logger}
}

// defaultRules returns a minimal EstoppelRules until PolicyResolver is wired in.
func defaultEstoppelRules() *EstoppelRules {
	return &EstoppelRules{
		StandardFeeCents:               29900,
		StandardTurnaroundBusinessDays: 10,
	}
}

// CreateRequest handles POST /organizations/{org_id}/estoppel/requests.
func (h *Handler) CreateRequest(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseEstoppelPathUUID(r, "org_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	userID := middleware.UserIDFromContext(r.Context())
	if userID == uuid.Nil {
		api.WriteError(w, api.NewUnauthenticatedError("user identity required"))
		return
	}

	var dto CreateEstoppelRequestDTO
	if err := api.ReadJSON(r, &dto); err != nil {
		api.WriteError(w, err)
		return
	}

	rules := defaultEstoppelRules()
	created, err := h.service.CreateRequest(r.Context(), orgID, dto, rules, userID)
	if err != nil {
		h.logger.Error("CreateRequest failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusCreated, created)
}

// ListRequests handles GET /organizations/{org_id}/estoppel/requests.
func (h *Handler) ListRequests(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseEstoppelPathUUID(r, "org_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var statusFilter *string
	if s := r.URL.Query().Get("status"); s != "" {
		statusFilter = &s
	}

	requests, hasMore, err := h.service.ListRequests(r.Context(), orgID, statusFilter, 20, nil)
	if err != nil {
		h.logger.Error("ListRequests failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	var meta *api.Meta
	if hasMore && len(requests) > 0 {
		meta = &api.Meta{
			Cursor:  requests[len(requests)-1].ID.String(),
			HasMore: true,
		}
	}

	api.WriteJSONWithMeta(w, http.StatusOK, requests, meta)
}

// GetRequest handles GET /organizations/{org_id}/estoppel/requests/{id}.
func (h *Handler) GetRequest(w http.ResponseWriter, r *http.Request) {
	id, err := parseEstoppelPathUUID(r, "id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	req, err := h.service.GetRequest(r.Context(), id)
	if err != nil {
		h.logger.Error("GetRequest failed", "id", id, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, req)
}

// ApproveRequest handles POST /organizations/{org_id}/estoppel/requests/{id}/approve.
func (h *Handler) ApproveRequest(w http.ResponseWriter, r *http.Request) {
	id, err := parseEstoppelPathUUID(r, "id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	userID := middleware.UserIDFromContext(r.Context())
	if userID == uuid.Nil {
		api.WriteError(w, api.NewUnauthenticatedError("user identity required"))
		return
	}

	var dto ApproveRequestDTO
	if err := api.ReadJSON(r, &dto); err != nil {
		api.WriteError(w, err)
		return
	}

	updated, err := h.service.ApproveRequest(r.Context(), id, dto, userID)
	if err != nil {
		h.logger.Error("ApproveRequest failed", "id", id, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, updated)
}

// RejectRequest handles POST /organizations/{org_id}/estoppel/requests/{id}/reject.
func (h *Handler) RejectRequest(w http.ResponseWriter, r *http.Request) {
	id, err := parseEstoppelPathUUID(r, "id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	userID := middleware.UserIDFromContext(r.Context())
	if userID == uuid.Nil {
		api.WriteError(w, api.NewUnauthenticatedError("user identity required"))
		return
	}

	var dto RejectRequestDTO
	if err := api.ReadJSON(r, &dto); err != nil {
		api.WriteError(w, err)
		return
	}

	updated, err := h.service.RejectRequest(r.Context(), id, dto, userID)
	if err != nil {
		h.logger.Error("RejectRequest failed", "id", id, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, updated)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// parseEstoppelPathUUID extracts and parses a UUID path value by key from the
// request, returning a ValidationError if missing or malformed.
func parseEstoppelPathUUID(r *http.Request, key string) (uuid.UUID, error) {
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
