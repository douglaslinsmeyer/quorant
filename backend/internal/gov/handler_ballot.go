package gov

import (
	"log/slog"
	"net/http"

	"github.com/quorant/quorant/internal/platform/api"
	"github.com/quorant/quorant/internal/platform/middleware"
)

// BallotHandler handles HTTP requests for the ballots sub-domain.
type BallotHandler struct {
	service Service
	logger  *slog.Logger
}

// NewBallotHandler constructs a BallotHandler backed by the given service.
func NewBallotHandler(service Service, logger *slog.Logger) *BallotHandler {
	return &BallotHandler{service: service, logger: logger}
}

// CreateBallot handles POST /organizations/{org_id}/ballots.
func (h *BallotHandler) CreateBallot(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseGovOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var req CreateBallotRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}

	created, err := h.service.CreateBallot(r.Context(), orgID, req, middleware.UserIDFromContext(r.Context()))
	if err != nil {
		h.logger.Error("CreateBallot failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusCreated, created)
}

// ListBallots handles GET /organizations/{org_id}/ballots.
func (h *BallotHandler) ListBallots(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseGovOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	ballots, err := h.service.ListBallots(r.Context(), orgID)
	if err != nil {
		h.logger.Error("ListBallots failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, ballots)
}

// GetBallot handles GET /organizations/{org_id}/ballots/{ballot_id}.
func (h *BallotHandler) GetBallot(w http.ResponseWriter, r *http.Request) {
	_, err := parseGovOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	ballotID, err := parseGovPathUUID(r, "ballot_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	ballot, err := h.service.GetBallot(r.Context(), ballotID)
	if err != nil {
		h.logger.Error("GetBallot failed", "ballot_id", ballotID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, ballot)
}

// UpdateBallot handles PATCH /organizations/{org_id}/ballots/{ballot_id}.
func (h *BallotHandler) UpdateBallot(w http.ResponseWriter, r *http.Request) {
	_, err := parseGovOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	ballotID, err := parseGovPathUUID(r, "ballot_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var ballot Ballot
	if err := api.ReadJSON(r, &ballot); err != nil {
		api.WriteError(w, err)
		return
	}

	updated, err := h.service.UpdateBallot(r.Context(), ballotID, &ballot)
	if err != nil {
		h.logger.Error("UpdateBallot failed", "ballot_id", ballotID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, updated)
}

// CastVote handles POST /organizations/{org_id}/ballots/{ballot_id}/votes.
func (h *BallotHandler) CastVote(w http.ResponseWriter, r *http.Request) {
	_, err := parseGovOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	ballotID, err := parseGovPathUUID(r, "ballot_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var req CastBallotVoteRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}

	vote, err := h.service.CastBallotVote(r.Context(), ballotID, req, middleware.UserIDFromContext(r.Context()))
	if err != nil {
		h.logger.Error("CastVote failed", "ballot_id", ballotID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusCreated, vote)
}

// FileProxy handles POST /organizations/{org_id}/ballots/{ballot_id}/proxy.
func (h *BallotHandler) FileProxy(w http.ResponseWriter, r *http.Request) {
	_, err := parseGovOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	ballotID, err := parseGovPathUUID(r, "ballot_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var req FileProxyRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}

	proxy, err := h.service.FileProxy(r.Context(), ballotID, req, middleware.UserIDFromContext(r.Context()))
	if err != nil {
		h.logger.Error("FileProxy failed", "ballot_id", ballotID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusCreated, proxy)
}

// RevokeProxy handles DELETE /organizations/{org_id}/ballots/{ballot_id}/proxy/{proxy_id}.
func (h *BallotHandler) RevokeProxy(w http.ResponseWriter, r *http.Request) {
	_, err := parseGovOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	_, err = parseGovPathUUID(r, "ballot_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	proxyID, err := parseGovPathUUID(r, "proxy_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	if err := h.service.RevokeProxy(r.Context(), proxyID); err != nil {
		h.logger.Error("RevokeProxy failed", "proxy_id", proxyID, "error", err)
		api.WriteError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
