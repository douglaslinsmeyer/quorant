package gov

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/quorant/quorant/internal/platform/api"
	"github.com/quorant/quorant/internal/platform/middleware"
)

// ARBHandler handles HTTP requests for the ARB (Architectural Review Board) sub-domain.
type ARBHandler struct {
	service *GovService
	logger  *slog.Logger
}

// NewARBHandler constructs an ARBHandler backed by the given service.
func NewARBHandler(service *GovService, logger *slog.Logger) *ARBHandler {
	return &ARBHandler{service: service, logger: logger}
}

// SubmitARBRequest handles POST /organizations/{org_id}/arb-requests.
func (h *ARBHandler) SubmitARBRequest(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseGovOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var req CreateARBRequestRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}

	created, err := h.service.SubmitARBRequest(r.Context(), orgID, req, middleware.UserIDFromContext(r.Context()))
	if err != nil {
		h.logger.Error("SubmitARBRequest failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusCreated, created)
}

// ListARBRequests handles GET /organizations/{org_id}/arb-requests.
func (h *ARBHandler) ListARBRequests(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseGovOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	requests, err := h.service.ListARBRequests(r.Context(), orgID)
	if err != nil {
		h.logger.Error("ListARBRequests failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, requests)
}

// GetARBRequest handles GET /organizations/{org_id}/arb-requests/{request_id}.
func (h *ARBHandler) GetARBRequest(w http.ResponseWriter, r *http.Request) {
	_, err := parseGovOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	requestID, err := parseGovPathUUID(r, "request_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	req, err := h.service.GetARBRequest(r.Context(), requestID)
	if err != nil {
		h.logger.Error("GetARBRequest failed", "request_id", requestID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, req)
}

// UpdateARBRequest handles PATCH /organizations/{org_id}/arb-requests/{request_id}.
func (h *ARBHandler) UpdateARBRequest(w http.ResponseWriter, r *http.Request) {
	_, err := parseGovOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	requestID, err := parseGovPathUUID(r, "request_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var req ARBRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}

	updated, err := h.service.UpdateARBRequest(r.Context(), requestID, &req)
	if err != nil {
		h.logger.Error("UpdateARBRequest failed", "request_id", requestID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, updated)
}

// CastARBVote handles POST /organizations/{org_id}/arb-requests/{request_id}/votes.
func (h *ARBHandler) CastARBVote(w http.ResponseWriter, r *http.Request) {
	_, err := parseGovOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	requestID, err := parseGovPathUUID(r, "request_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var req CastARBVoteRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}

	vote, err := h.service.CastARBVote(r.Context(), requestID, req, middleware.UserIDFromContext(r.Context()))
	if err != nil {
		h.logger.Error("CastARBVote failed", "request_id", requestID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusCreated, vote)
}

// VerifyCondition handles POST /organizations/{org_id}/arb-requests/{request_id}/conditions/{condition_id}/verify.
// It marks the condition with the given condition_id as verified in the JSONB conditions array.
func (h *ARBHandler) VerifyCondition(w http.ResponseWriter, r *http.Request) {
	_, err := parseGovOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	requestID, err := parseGovPathUUID(r, "request_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	conditionID, err := parseGovPathUUID(r, "condition_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	req, err := h.service.GetARBRequest(r.Context(), requestID)
	if err != nil {
		h.logger.Error("VerifyCondition fetch failed", "request_id", requestID, "error", err)
		api.WriteError(w, err)
		return
	}

	// Parse conditions JSONB array and mark the matching condition as verified.
	var conditions []map[string]any
	if len(req.Conditions) > 0 {
		if err := json.Unmarshal(req.Conditions, &conditions); err != nil {
			api.WriteError(w, api.NewValidationError("conditions field is not a valid JSON array", "conditions"))
			return
		}
	}

	found := false
	for i, c := range conditions {
		if id, ok := c["id"].(string); ok && id == conditionID.String() {
			conditions[i]["verified"] = true
			found = true
			break
		}
	}

	if !found {
		api.WriteError(w, api.NewNotFoundError("condition not found in ARB request"))
		return
	}

	updated_conditions, err := json.Marshal(conditions)
	if err != nil {
		h.logger.Error("VerifyCondition marshal failed", "request_id", requestID, "error", err)
		api.WriteError(w, err)
		return
	}
	req.Conditions = json.RawMessage(updated_conditions)

	updated, err := h.service.UpdateARBRequest(r.Context(), requestID, req)
	if err != nil {
		h.logger.Error("VerifyCondition update failed", "request_id", requestID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, updated)
}

// RequestRevision handles POST /organizations/{org_id}/arb-requests/{request_id}/request-revision.
func (h *ARBHandler) RequestRevision(w http.ResponseWriter, r *http.Request) {
	_, err := parseGovOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	requestID, err := parseGovPathUUID(r, "request_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	updated, err := h.service.RequestRevision(r.Context(), requestID)
	if err != nil {
		h.logger.Error("RequestRevision failed", "request_id", requestID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, updated)
}
