package gov

import (
	"log/slog"
	"net/http"

	"github.com/quorant/quorant/internal/platform/api"
	"github.com/quorant/quorant/internal/platform/middleware"
)

// MeetingHandler handles HTTP requests for the meetings sub-domain.
type MeetingHandler struct {
	service *GovService
	logger  *slog.Logger
}

// NewMeetingHandler constructs a MeetingHandler backed by the given service.
func NewMeetingHandler(service *GovService, logger *slog.Logger) *MeetingHandler {
	return &MeetingHandler{service: service, logger: logger}
}

// ScheduleMeeting handles POST /organizations/{org_id}/meetings.
func (h *MeetingHandler) ScheduleMeeting(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseGovOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var req CreateMeetingRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}

	created, err := h.service.ScheduleMeeting(r.Context(), orgID, req, middleware.UserIDFromContext(r.Context()))
	if err != nil {
		h.logger.Error("ScheduleMeeting failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusCreated, created)
}

// ListMeetings handles GET /organizations/{org_id}/meetings.
func (h *MeetingHandler) ListMeetings(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseGovOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	meetings, err := h.service.ListMeetings(r.Context(), orgID)
	if err != nil {
		h.logger.Error("ListMeetings failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, meetings)
}

// GetMeeting handles GET /organizations/{org_id}/meetings/{meeting_id}.
func (h *MeetingHandler) GetMeeting(w http.ResponseWriter, r *http.Request) {
	_, err := parseGovOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	meetingID, err := parseGovPathUUID(r, "meeting_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	meeting, err := h.service.GetMeeting(r.Context(), meetingID)
	if err != nil {
		h.logger.Error("GetMeeting failed", "meeting_id", meetingID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, meeting)
}

// UpdateMeeting handles PATCH /organizations/{org_id}/meetings/{meeting_id}.
func (h *MeetingHandler) UpdateMeeting(w http.ResponseWriter, r *http.Request) {
	_, err := parseGovOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	meetingID, err := parseGovPathUUID(r, "meeting_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var meeting Meeting
	if err := api.ReadJSON(r, &meeting); err != nil {
		api.WriteError(w, err)
		return
	}

	updated, err := h.service.UpdateMeeting(r.Context(), meetingID, &meeting)
	if err != nil {
		h.logger.Error("UpdateMeeting failed", "meeting_id", meetingID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, updated)
}

// AddAttendee handles POST /organizations/{org_id}/meetings/{meeting_id}/attendees.
func (h *MeetingHandler) AddAttendee(w http.ResponseWriter, r *http.Request) {
	_, err := parseGovOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	meetingID, err := parseGovPathUUID(r, "meeting_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var attendee MeetingAttendee
	if err := api.ReadJSON(r, &attendee); err != nil {
		api.WriteError(w, err)
		return
	}

	added, err := h.service.AddAttendee(r.Context(), meetingID, &attendee)
	if err != nil {
		h.logger.Error("AddAttendee failed", "meeting_id", meetingID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusCreated, added)
}

// RecordMotion handles POST /organizations/{org_id}/meetings/{meeting_id}/motions.
func (h *MeetingHandler) RecordMotion(w http.ResponseWriter, r *http.Request) {
	_, err := parseGovOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	meetingID, err := parseGovPathUUID(r, "meeting_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var req RecordMotionRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}

	motion, err := h.service.RecordMotion(r.Context(), meetingID, req)
	if err != nil {
		h.logger.Error("RecordMotion failed", "meeting_id", meetingID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusCreated, motion)
}

// UpdateMotion handles PATCH /organizations/{org_id}/meetings/{meeting_id}/motions/{motion_id}.
func (h *MeetingHandler) UpdateMotion(w http.ResponseWriter, r *http.Request) {
	_, err := parseGovOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	_, err = parseGovPathUUID(r, "meeting_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	motionID, err := parseGovPathUUID(r, "motion_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var motion MeetingMotion
	if err := api.ReadJSON(r, &motion); err != nil {
		api.WriteError(w, err)
		return
	}

	updated, err := h.service.UpdateMotion(r.Context(), motionID, &motion)
	if err != nil {
		h.logger.Error("UpdateMotion failed", "motion_id", motionID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, updated)
}
