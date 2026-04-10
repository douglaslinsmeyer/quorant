package fin

import (
	"log/slog"
	"net/http"

	"github.com/quorant/quorant/internal/platform/api"
)

// FundHandler handles HTTP requests for funds and fund transfers.
type FundHandler struct {
	service Service
	logger  *slog.Logger
}

// NewFundHandler constructs a FundHandler backed by the given service.
func NewFundHandler(service Service, logger *slog.Logger) *FundHandler {
	return &FundHandler{service: service, logger: logger}
}

// ── Funds ─────────────────────────────────────────────────────────────────────

// CreateFund handles POST /organizations/{org_id}/funds.
func (h *FundHandler) CreateFund(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseFinOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var req CreateFundRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}

	created, err := h.service.CreateFund(r.Context(), orgID, req)
	if err != nil {
		h.logger.Error("CreateFund failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusCreated, created)
}

// ListFunds handles GET /organizations/{org_id}/funds.
func (h *FundHandler) ListFunds(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseFinOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	funds, err := h.service.ListFunds(r.Context(), orgID)
	if err != nil {
		h.logger.Error("ListFunds failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, funds)
}

// GetFund handles GET /organizations/{org_id}/funds/{fund_id}.
func (h *FundHandler) GetFund(w http.ResponseWriter, r *http.Request) {
	_, err := parseFinOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	fundID, err := parsePathUUID(r, "fund_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	fund, err := h.service.GetFund(r.Context(), fundID)
	if err != nil {
		h.logger.Error("GetFund failed", "fund_id", fundID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, fund)
}

// UpdateFund handles PATCH /organizations/{org_id}/funds/{fund_id}.
func (h *FundHandler) UpdateFund(w http.ResponseWriter, r *http.Request) {
	_, err := parseFinOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	fundID, err := parsePathUUID(r, "fund_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var f Fund
	if err := api.ReadJSON(r, &f); err != nil {
		api.WriteError(w, err)
		return
	}

	updated, err := h.service.UpdateFund(r.Context(), fundID, &f)
	if err != nil {
		h.logger.Error("UpdateFund failed", "fund_id", fundID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, updated)
}

// GetFundTransactions handles GET /organizations/{org_id}/funds/{fund_id}/transactions.
func (h *FundHandler) GetFundTransactions(w http.ResponseWriter, r *http.Request) {
	_, err := parseFinOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	fundID, err := parsePathUUID(r, "fund_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	transactions, err := h.service.GetFundTransactions(r.Context(), fundID)
	if err != nil {
		h.logger.Error("GetFundTransactions failed", "fund_id", fundID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, transactions)
}

// ── Fund Transfers ────────────────────────────────────────────────────────────

// CreateFundTransfer handles POST /organizations/{org_id}/fund-transfers.
func (h *FundHandler) CreateFundTransfer(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseFinOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var req CreateFundTransferRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}

	created, err := h.service.CreateFundTransfer(r.Context(), orgID, req)
	if err != nil {
		h.logger.Error("CreateFundTransfer failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusCreated, created)
}

// ListFundTransfers handles GET /organizations/{org_id}/fund-transfers.
func (h *FundHandler) ListFundTransfers(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseFinOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	transfers, err := h.service.ListFundTransfers(r.Context(), orgID)
	if err != nil {
		h.logger.Error("ListFundTransfers failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, transfers)
}
