package com

import (
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/platform/api"
)

// CalendarHandler handles calendar event HTTP requests.
type CalendarHandler struct {
	service *ComService
	logger  *slog.Logger
}

// NewCalendarHandler constructs a CalendarHandler backed by the given service.
func NewCalendarHandler(service *ComService, logger *slog.Logger) *CalendarHandler {
	return &CalendarHandler{service: service, logger: logger}
}

// Create handles POST /api/v1/organizations/{org_id}/calendar-events.
func (h *CalendarHandler) Create(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseComOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var req CreateCalendarEventRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}

	// TODO: extract real user ID from auth context
	created, err := h.service.CreateCalendarEvent(r.Context(), orgID, req, uuid.Nil)
	if err != nil {
		h.logger.Error("CreateCalendarEvent failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusCreated, created)
}

// List handles GET /api/v1/organizations/{org_id}/calendar-events.
func (h *CalendarHandler) List(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseComOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	events, err := h.service.ListCalendarEvents(r.Context(), orgID)
	if err != nil {
		h.logger.Error("ListCalendarEvents failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, events)
}

// Get handles GET /api/v1/organizations/{org_id}/calendar-events/{event_id}.
func (h *CalendarHandler) Get(w http.ResponseWriter, r *http.Request) {
	eventID, err := parseComPathUUID(r, "event_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	event, err := h.service.GetCalendarEvent(r.Context(), eventID)
	if err != nil {
		h.logger.Error("GetCalendarEvent failed", "event_id", eventID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, event)
}

// Update handles PATCH /api/v1/organizations/{org_id}/calendar-events/{event_id}.
func (h *CalendarHandler) Update(w http.ResponseWriter, r *http.Request) {
	eventID, err := parseComPathUUID(r, "event_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var body CalendarEvent
	if err := api.ReadJSON(r, &body); err != nil {
		api.WriteError(w, err)
		return
	}

	updated, err := h.service.UpdateCalendarEvent(r.Context(), eventID, &body)
	if err != nil {
		h.logger.Error("UpdateCalendarEvent failed", "event_id", eventID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, updated)
}

// Delete handles DELETE /api/v1/organizations/{org_id}/calendar-events/{event_id}.
func (h *CalendarHandler) Delete(w http.ResponseWriter, r *http.Request) {
	eventID, err := parseComPathUUID(r, "event_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	if err := h.service.DeleteCalendarEvent(r.Context(), eventID); err != nil {
		h.logger.Error("DeleteCalendarEvent failed", "event_id", eventID, "error", err)
		api.WriteError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// RSVP handles POST /api/v1/organizations/{org_id}/calendar-events/{event_id}/rsvp.
func (h *CalendarHandler) RSVP(w http.ResponseWriter, r *http.Request) {
	eventID, err := parseComPathUUID(r, "event_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var req RSVPRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}

	// TODO: extract real user ID from auth context
	rsvp, err := h.service.RSVPToEvent(r.Context(), eventID, req, uuid.Nil)
	if err != nil {
		h.logger.Error("RSVPToEvent failed", "event_id", eventID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusCreated, rsvp)
}

// ListTemplates handles GET /api/v1/organizations/{org_id}/message-templates.
func (h *CalendarHandler) ListTemplates(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseComOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	templates, err := h.service.ListTemplates(r.Context(), orgID)
	if err != nil {
		h.logger.Error("ListTemplates failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, templates)
}

// CreateTemplate handles POST /api/v1/organizations/{org_id}/message-templates.
func (h *CalendarHandler) CreateTemplate(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseComOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var req CreateTemplateRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}

	created, err := h.service.CreateTemplate(r.Context(), orgID, req)
	if err != nil {
		h.logger.Error("CreateTemplate failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusCreated, created)
}

// UpdateTemplate handles PATCH /api/v1/organizations/{org_id}/message-templates/{template_id}.
func (h *CalendarHandler) UpdateTemplate(w http.ResponseWriter, r *http.Request) {
	templateID, err := parseComPathUUID(r, "template_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var body MessageTemplate
	if err := api.ReadJSON(r, &body); err != nil {
		api.WriteError(w, err)
		return
	}

	updated, err := h.service.UpdateTemplate(r.Context(), templateID, &body)
	if err != nil {
		h.logger.Error("UpdateTemplate failed", "template_id", templateID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, updated)
}

// DeleteTemplate handles DELETE /api/v1/organizations/{org_id}/message-templates/{template_id}.
func (h *CalendarHandler) DeleteTemplate(w http.ResponseWriter, r *http.Request) {
	templateID, err := parseComPathUUID(r, "template_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	if err := h.service.DeleteTemplate(r.Context(), templateID); err != nil {
		h.logger.Error("DeleteTemplate failed", "template_id", templateID, "error", err)
		api.WriteError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetDirectoryPrefs handles GET /api/v1/organizations/{org_id}/directory/preferences.
func (h *CalendarHandler) GetDirectoryPrefs(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseComOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	// TODO: extract real user ID from auth context
	prefs, err := h.service.GetDirectoryPreferences(r.Context(), uuid.Nil, orgID)
	if err != nil {
		h.logger.Error("GetDirectoryPreferences failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, prefs)
}

// UpdateDirectoryPrefs handles PUT /api/v1/organizations/{org_id}/directory/preferences.
func (h *CalendarHandler) UpdateDirectoryPrefs(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseComOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var req UpdateDirectoryPreferenceRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}

	// TODO: extract real user ID from auth context
	updated, err := h.service.UpdateDirectoryPreferences(r.Context(), uuid.Nil, orgID, req)
	if err != nil {
		h.logger.Error("UpdateDirectoryPreferences failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, updated)
}
