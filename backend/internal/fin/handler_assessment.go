package fin

import (
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/platform/api"
)

// AssessmentHandler handles HTTP requests for assessment schedules, assessments,
// and the ledger.
type AssessmentHandler struct {
	service *FinService
	logger  *slog.Logger
}

// NewAssessmentHandler constructs an AssessmentHandler backed by the given service.
func NewAssessmentHandler(service *FinService, logger *slog.Logger) *AssessmentHandler {
	return &AssessmentHandler{service: service, logger: logger}
}

// ── Assessment Schedules ──────────────────────────────────────────────────────

// CreateSchedule handles POST /organizations/{org_id}/assessment-schedules.
func (h *AssessmentHandler) CreateSchedule(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseFinOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var req CreateAssessmentScheduleRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}

	created, err := h.service.CreateSchedule(r.Context(), orgID, req)
	if err != nil {
		h.logger.Error("CreateSchedule failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusCreated, created)
}

// ListSchedules handles GET /organizations/{org_id}/assessment-schedules.
func (h *AssessmentHandler) ListSchedules(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseFinOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	schedules, err := h.service.ListSchedules(r.Context(), orgID)
	if err != nil {
		h.logger.Error("ListSchedules failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, schedules)
}

// GetSchedule handles GET /organizations/{org_id}/assessment-schedules/{schedule_id}.
func (h *AssessmentHandler) GetSchedule(w http.ResponseWriter, r *http.Request) {
	_, err := parseFinOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	scheduleID, err := parsePathUUID(r, "schedule_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	schedule, err := h.service.GetSchedule(r.Context(), scheduleID)
	if err != nil {
		h.logger.Error("GetSchedule failed", "schedule_id", scheduleID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, schedule)
}

// UpdateSchedule handles PATCH /organizations/{org_id}/assessment-schedules/{schedule_id}.
func (h *AssessmentHandler) UpdateSchedule(w http.ResponseWriter, r *http.Request) {
	_, err := parseFinOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	scheduleID, err := parsePathUUID(r, "schedule_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var req UpdateAssessmentScheduleRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}

	updated, err := h.service.UpdateSchedule(r.Context(), scheduleID, req)
	if err != nil {
		h.logger.Error("UpdateSchedule failed", "schedule_id", scheduleID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, updated)
}

// DeactivateSchedule handles POST /organizations/{org_id}/assessment-schedules/{schedule_id}/deactivate.
func (h *AssessmentHandler) DeactivateSchedule(w http.ResponseWriter, r *http.Request) {
	_, err := parseFinOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	scheduleID, err := parsePathUUID(r, "schedule_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	if err := h.service.DeactivateSchedule(r.Context(), scheduleID); err != nil {
		h.logger.Error("DeactivateSchedule failed", "schedule_id", scheduleID, "error", err)
		api.WriteError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ── Assessments ───────────────────────────────────────────────────────────────

// CreateAssessment handles POST /organizations/{org_id}/assessments.
func (h *AssessmentHandler) CreateAssessment(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseFinOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var req CreateAssessmentRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}

	created, err := h.service.CreateAssessment(r.Context(), orgID, req)
	if err != nil {
		h.logger.Error("CreateAssessment failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusCreated, created)
}

// ListAssessments handles GET /organizations/{org_id}/assessments.
func (h *AssessmentHandler) ListAssessments(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseFinOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	assessments, err := h.service.ListAssessments(r.Context(), orgID)
	if err != nil {
		h.logger.Error("ListAssessments failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, assessments)
}

// GetAssessment handles GET /organizations/{org_id}/assessments/{assessment_id}.
func (h *AssessmentHandler) GetAssessment(w http.ResponseWriter, r *http.Request) {
	_, err := parseFinOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	assessmentID, err := parsePathUUID(r, "assessment_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	assessment, err := h.service.GetAssessment(r.Context(), assessmentID)
	if err != nil {
		h.logger.Error("GetAssessment failed", "assessment_id", assessmentID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, assessment)
}

// ── Ledger ────────────────────────────────────────────────────────────────────

// GetUnitLedger handles GET /organizations/{org_id}/units/{unit_id}/ledger.
func (h *AssessmentHandler) GetUnitLedger(w http.ResponseWriter, r *http.Request) {
	_, err := parseFinOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	unitID, err := parsePathUUID(r, "unit_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	entries, err := h.service.GetUnitLedger(r.Context(), unitID)
	if err != nil {
		h.logger.Error("GetUnitLedger failed", "unit_id", unitID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, entries)
}

// GetOrgLedger handles GET /organizations/{org_id}/ledger.
func (h *AssessmentHandler) GetOrgLedger(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseFinOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	entries, err := h.service.GetOrgLedger(r.Context(), orgID)
	if err != nil {
		h.logger.Error("GetOrgLedger failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, entries)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// parseFinOrgID extracts and parses the {org_id} path value from the request.
// Returns a ValidationError if the value is missing or not a valid UUID.
func parseFinOrgID(r *http.Request) (uuid.UUID, error) {
	return parsePathUUID(r, "org_id")
}

// parsePathUUID extracts and parses a UUID path value by the given key.
// Returns a ValidationError if the value is missing or not a valid UUID.
func parsePathUUID(r *http.Request, key string) (uuid.UUID, error) {
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
