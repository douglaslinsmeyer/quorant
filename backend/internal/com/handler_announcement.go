package com

import (
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/platform/api"
	"github.com/quorant/quorant/internal/platform/middleware"
)

// AnnouncementHandler handles announcement HTTP requests.
type AnnouncementHandler struct {
	service Service
	logger  *slog.Logger
}

// NewAnnouncementHandler constructs an AnnouncementHandler backed by the given service.
func NewAnnouncementHandler(service Service, logger *slog.Logger) *AnnouncementHandler {
	return &AnnouncementHandler{service: service, logger: logger}
}

// Create handles POST /api/v1/organizations/{org_id}/announcements.
func (h *AnnouncementHandler) Create(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseComOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var req CreateAnnouncementRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}

	created, err := h.service.CreateAnnouncement(r.Context(), orgID, req, middleware.UserIDFromContext(r.Context()))
	if err != nil {
		h.logger.Error("CreateAnnouncement failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusCreated, created)
}

// List handles GET /api/v1/organizations/{org_id}/announcements.
func (h *AnnouncementHandler) List(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseComOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	page := api.ParsePageRequest(r)
	afterID, err := parseComCursorID(page.Cursor)
	if err != nil {
		api.WriteError(w, api.NewValidationError("validation.invalid_cursor", "cursor"))
		return
	}

	announcements, hasMore, err := h.service.ListAnnouncements(r.Context(), orgID, page.Limit, afterID)
	if err != nil {
		h.logger.Error("ListAnnouncements failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	var meta *api.Meta
	if hasMore && len(announcements) > 0 {
		meta = &api.Meta{
			Cursor:  api.EncodeCursor(map[string]string{"id": announcements[len(announcements)-1].ID.String()}),
			HasMore: true,
		}
	}

	api.WriteJSONWithMeta(w, http.StatusOK, announcements, meta)
}

// Get handles GET /api/v1/organizations/{org_id}/announcements/{announcement_id}.
func (h *AnnouncementHandler) Get(w http.ResponseWriter, r *http.Request) {
	announcementID, err := parseComPathUUID(r, "announcement_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	announcement, err := h.service.GetAnnouncement(r.Context(), announcementID)
	if err != nil {
		h.logger.Error("GetAnnouncement failed", "announcement_id", announcementID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, announcement)
}

// Update handles PATCH /api/v1/organizations/{org_id}/announcements/{announcement_id}.
func (h *AnnouncementHandler) Update(w http.ResponseWriter, r *http.Request) {
	announcementID, err := parseComPathUUID(r, "announcement_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var body Announcement
	if err := api.ReadJSON(r, &body); err != nil {
		api.WriteError(w, err)
		return
	}

	updated, err := h.service.UpdateAnnouncement(r.Context(), announcementID, &body)
	if err != nil {
		h.logger.Error("UpdateAnnouncement failed", "announcement_id", announcementID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, updated)
}

// Delete handles DELETE /api/v1/organizations/{org_id}/announcements/{announcement_id}.
func (h *AnnouncementHandler) Delete(w http.ResponseWriter, r *http.Request) {
	announcementID, err := parseComPathUUID(r, "announcement_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	if err := h.service.DeleteAnnouncement(r.Context(), announcementID); err != nil {
		h.logger.Error("DeleteAnnouncement failed", "announcement_id", announcementID, "error", err)
		api.WriteError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// parseComCursorID decodes a pagination cursor and returns the ID it encodes.
// Returns nil, nil when cursor is empty (first page).
func parseComCursorID(cursor string) (*uuid.UUID, error) {
	if cursor == "" {
		return nil, nil
	}
	vals, err := api.DecodeCursor(cursor)
	if err != nil {
		return nil, err
	}
	id, err := uuid.Parse(vals["id"])
	if err != nil {
		return nil, err
	}
	return &id, nil
}

// ─── Shared path helpers ──────────────────────────────────────────────────────

// parseComOrgID extracts and parses the {org_id} path value from the request.
// Returns a ValidationError if the value is missing or not a valid UUID.
func parseComOrgID(r *http.Request) (uuid.UUID, error) {
	raw := r.PathValue("org_id")
	if raw == "" {
		return uuid.Nil, api.NewValidationError("validation.required", "org_id", api.P("field", "org_id"))
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, api.NewValidationError("validation.invalid_uuid", "org_id", api.P("field", "org_id"))
	}
	return id, nil
}

// parseComPathUUID extracts and parses a named UUID path parameter from the request.
// Returns a ValidationError if the value is missing or not a valid UUID.
func parseComPathUUID(r *http.Request, param string) (uuid.UUID, error) {
	raw := r.PathValue(param)
	if raw == "" {
		return uuid.Nil, api.NewValidationError("validation.required", param, api.P("field", param))
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, api.NewValidationError("validation.invalid_uuid", param, api.P("field", param))
	}
	return id, nil
}
