package gov

import (
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/platform/api"
	"github.com/quorant/quorant/internal/platform/middleware"
)

// ViolationHandler handles HTTP requests for the violations sub-domain.
type ViolationHandler struct {
	service Service
	logger  *slog.Logger
}

// NewViolationHandler constructs a ViolationHandler backed by the given service.
func NewViolationHandler(service Service, logger *slog.Logger) *ViolationHandler {
	return &ViolationHandler{service: service, logger: logger}
}

// ReportViolation handles POST /organizations/{org_id}/violations.
func (h *ViolationHandler) ReportViolation(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseGovOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var req CreateViolationRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}

	created, err := h.service.ReportViolation(r.Context(), orgID, req, middleware.UserIDFromContext(r.Context()))
	if err != nil {
		h.logger.Error("ReportViolation failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusCreated, created)
}

// ListViolations handles GET /organizations/{org_id}/violations.
func (h *ViolationHandler) ListViolations(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseGovOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	page := api.ParsePageRequest(r)
	afterID, err := parseGovCursorID(page.Cursor)
	if err != nil {
		api.WriteError(w, api.NewValidationError("invalid cursor", "cursor"))
		return
	}

	violations, hasMore, err := h.service.ListViolations(r.Context(), orgID, page.Limit, afterID)
	if err != nil {
		h.logger.Error("ListViolations failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	var meta *api.Meta
	if hasMore && len(violations) > 0 {
		meta = &api.Meta{
			Cursor:  api.EncodeCursor(map[string]string{"id": violations[len(violations)-1].ID.String()}),
			HasMore: true,
		}
	}

	api.WriteJSONWithMeta(w, http.StatusOK, violations, meta)
}

// GetViolation handles GET /organizations/{org_id}/violations/{violation_id}.
func (h *ViolationHandler) GetViolation(w http.ResponseWriter, r *http.Request) {
	_, err := parseGovOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	violationID, err := parseGovPathUUID(r, "violation_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	v, err := h.service.GetViolation(r.Context(), violationID)
	if err != nil {
		h.logger.Error("GetViolation failed", "violation_id", violationID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, v)
}

// UpdateViolation handles PATCH /organizations/{org_id}/violations/{violation_id}.
func (h *ViolationHandler) UpdateViolation(w http.ResponseWriter, r *http.Request) {
	_, err := parseGovOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	violationID, err := parseGovPathUUID(r, "violation_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var v Violation
	if err := api.ReadJSON(r, &v); err != nil {
		api.WriteError(w, err)
		return
	}

	updated, err := h.service.UpdateViolation(r.Context(), violationID, &v)
	if err != nil {
		h.logger.Error("UpdateViolation failed", "violation_id", violationID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, updated)
}

// AddAction handles POST /organizations/{org_id}/violations/{violation_id}/actions.
func (h *ViolationHandler) AddAction(w http.ResponseWriter, r *http.Request) {
	_, err := parseGovOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	violationID, err := parseGovPathUUID(r, "violation_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var req CreateViolationActionRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}

	action, err := h.service.AddViolationAction(r.Context(), violationID, req, middleware.UserIDFromContext(r.Context()))
	if err != nil {
		h.logger.Error("AddAction failed", "violation_id", violationID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusCreated, action)
}

// VerifyCure handles POST /organizations/{org_id}/violations/{violation_id}/verify-cure.
func (h *ViolationHandler) VerifyCure(w http.ResponseWriter, r *http.Request) {
	_, err := parseGovOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	violationID, err := parseGovPathUUID(r, "violation_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	updated, err := h.service.VerifyCure(r.Context(), violationID, middleware.UserIDFromContext(r.Context()))
	if err != nil {
		h.logger.Error("VerifyCure failed", "violation_id", violationID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, updated)
}

// ScheduleHearing handles POST /organizations/{org_id}/violations/{violation_id}/hearing.
func (h *ViolationHandler) ScheduleHearing(w http.ResponseWriter, r *http.Request) {
	_, err := parseGovOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	violationID, err := parseGovPathUUID(r, "violation_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var body struct {
		MeetingID uuid.UUID `json:"meeting_id"`
	}
	if err := api.ReadJSON(r, &body); err != nil {
		api.WriteError(w, err)
		return
	}
	if body.MeetingID == uuid.Nil {
		api.WriteError(w, api.NewValidationError("meeting_id is required", "meeting_id"))
		return
	}

	hearing, err := h.service.ScheduleHearing(r.Context(), violationID, body.MeetingID)
	if err != nil {
		h.logger.Error("ScheduleHearing failed", "violation_id", violationID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusCreated, hearing)
}

// GetHearing handles GET /organizations/{org_id}/violations/{violation_id}/hearing.
func (h *ViolationHandler) GetHearing(w http.ResponseWriter, r *http.Request) {
	_, err := parseGovOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	violationID, err := parseGovPathUUID(r, "violation_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	hearing, err := h.service.GetHearing(r.Context(), violationID)
	if err != nil {
		h.logger.Error("GetHearing failed", "violation_id", violationID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, hearing)
}

// UpdateHearing handles PATCH /organizations/{org_id}/violations/{violation_id}/hearing/{hearing_id}.
func (h *ViolationHandler) UpdateHearing(w http.ResponseWriter, r *http.Request) {
	_, err := parseGovOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	_, err = parseGovPathUUID(r, "violation_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	hearingID, err := parseGovPathUUID(r, "hearing_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var hearing HearingLink
	if err := api.ReadJSON(r, &hearing); err != nil {
		api.WriteError(w, err)
		return
	}

	updated, err := h.service.UpdateHearing(r.Context(), hearingID, &hearing)
	if err != nil {
		h.logger.Error("UpdateHearing failed", "hearing_id", hearingID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, updated)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// parseGovCursorID decodes a pagination cursor and returns the ID it encodes.
// Returns nil, nil when cursor is empty (first page).
func parseGovCursorID(cursor string) (*uuid.UUID, error) {
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

// parseGovOrgID extracts and parses the {org_id} path value from the request.
func parseGovOrgID(r *http.Request) (uuid.UUID, error) {
	return parseGovPathUUID(r, "org_id")
}

// parseGovPathUUID extracts and parses a UUID path value by the given key.
func parseGovPathUUID(r *http.Request, key string) (uuid.UUID, error) {
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
