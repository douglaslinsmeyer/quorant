package fin

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/quorant/quorant/internal/platform/api"
	"github.com/quorant/quorant/internal/platform/middleware"
)

// GLHandler handles HTTP requests for the double-entry general ledger:
// chart of accounts, journal entries, trial balance, and account balances.
type GLHandler struct {
	glService *GLService
	logger    *slog.Logger
}

// NewGLHandler constructs a GLHandler backed by the given GLService.
func NewGLHandler(glService *GLService, logger *slog.Logger) *GLHandler {
	return &GLHandler{glService: glService, logger: logger}
}

// ── Chart of Accounts ────────────────────────────────────────────────────────

// CreateAccount handles POST /organizations/{org_id}/gl/accounts.
func (h *GLHandler) CreateAccount(w http.ResponseWriter, r *http.Request) {
	orgID, err := parsePathUUID(r, "org_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var req CreateGLAccountRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}

	account, err := h.glService.CreateAccount(r.Context(), orgID, req)
	if err != nil {
		h.logger.Error("CreateAccount failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusCreated, account)
}

// ListAccounts handles GET /organizations/{org_id}/gl/accounts.
func (h *GLHandler) ListAccounts(w http.ResponseWriter, r *http.Request) {
	orgID, err := parsePathUUID(r, "org_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	accounts, err := h.glService.ListAccounts(r.Context(), orgID)
	if err != nil {
		h.logger.Error("ListAccounts failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, accounts)
}

// GetAccount handles GET /organizations/{org_id}/gl/accounts/{account_id}.
func (h *GLHandler) GetAccount(w http.ResponseWriter, r *http.Request) {
	_, err := parsePathUUID(r, "org_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	accountID, err := parsePathUUID(r, "account_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	account, err := h.glService.GetAccount(r.Context(), accountID)
	if err != nil {
		h.logger.Error("GetAccount failed", "account_id", accountID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, account)
}

// UpdateAccount handles PATCH /organizations/{org_id}/gl/accounts/{account_id}.
func (h *GLHandler) UpdateAccount(w http.ResponseWriter, r *http.Request) {
	_, err := parsePathUUID(r, "org_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	accountID, err := parsePathUUID(r, "account_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var req UpdateGLAccountRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}

	updated, err := h.glService.UpdateAccount(r.Context(), accountID, req)
	if err != nil {
		h.logger.Error("UpdateAccount failed", "account_id", accountID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, updated)
}

// DeleteAccount handles DELETE /organizations/{org_id}/gl/accounts/{account_id}.
func (h *GLHandler) DeleteAccount(w http.ResponseWriter, r *http.Request) {
	_, err := parsePathUUID(r, "org_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	accountID, err := parsePathUUID(r, "account_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	if err := h.glService.DeleteAccount(r.Context(), accountID); err != nil {
		h.logger.Error("DeleteAccount failed", "account_id", accountID, "error", err)
		api.WriteError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ── Journal Entries ──────────────────────────────────────────────────────────

// CreateJournalEntry handles POST /organizations/{org_id}/gl/journal-entries.
func (h *GLHandler) CreateJournalEntry(w http.ResponseWriter, r *http.Request) {
	orgID, err := parsePathUUID(r, "org_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var req CreateJournalEntryRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}

	postedBy := middleware.UserIDFromContext(r.Context())

	entry, err := h.glService.PostJournalEntry(r.Context(), orgID, postedBy, req)
	if err != nil {
		h.logger.Error("CreateJournalEntry failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusCreated, entry)
}

// ListJournalEntries handles GET /organizations/{org_id}/gl/journal-entries.
func (h *GLHandler) ListJournalEntries(w http.ResponseWriter, r *http.Request) {
	orgID, err := parsePathUUID(r, "org_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	entries, err := h.glService.ListJournalEntries(r.Context(), orgID)
	if err != nil {
		h.logger.Error("ListJournalEntries failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, entries)
}

// GetJournalEntry handles GET /organizations/{org_id}/gl/journal-entries/{entry_id}.
func (h *GLHandler) GetJournalEntry(w http.ResponseWriter, r *http.Request) {
	_, err := parsePathUUID(r, "org_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	entryID, err := parsePathUUID(r, "entry_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	entry, err := h.glService.GetJournalEntry(r.Context(), entryID)
	if err != nil {
		h.logger.Error("GetJournalEntry failed", "entry_id", entryID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, entry)
}

// ── Reporting ────────────────────────────────────────────────────────────────

// GetTrialBalance handles GET /organizations/{org_id}/gl/trial-balance.
func (h *GLHandler) GetTrialBalance(w http.ResponseWriter, r *http.Request) {
	orgID, err := parsePathUUID(r, "org_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	asOfStr := r.URL.Query().Get("as_of_date")
	var asOfDate time.Time
	if asOfStr != "" {
		asOfDate, err = time.Parse("2006-01-02", asOfStr)
		if err != nil {
			api.WriteError(w, api.NewValidationError("validation.constraint", "as_of_date", api.P("field", "as_of_date"), api.P("constraint", "YYYY-MM-DD format")))
			return
		}
	} else {
		asOfDate = time.Now()
	}

	rows, err := h.glService.GetTrialBalance(r.Context(), orgID, asOfDate)
	if err != nil {
		h.logger.Error("GetTrialBalance failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, rows)
}

// GetAccountBalances handles GET /organizations/{org_id}/gl/account-balances.
func (h *GLHandler) GetAccountBalances(w http.ResponseWriter, r *http.Request) {
	orgID, err := parsePathUUID(r, "org_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")

	if fromStr == "" {
		api.WriteError(w, api.NewValidationError("validation.required", "from", api.P("field", "from")))
		return
	}
	if toStr == "" {
		api.WriteError(w, api.NewValidationError("validation.required", "to", api.P("field", "to")))
		return
	}

	from, err := time.Parse("2006-01-02", fromStr)
	if err != nil {
		api.WriteError(w, api.NewValidationError("validation.constraint", "from", api.P("field", "from"), api.P("constraint", "YYYY-MM-DD format")))
		return
	}

	to, err := time.Parse("2006-01-02", toStr)
	if err != nil {
		api.WriteError(w, api.NewValidationError("validation.constraint", "to", api.P("field", "to"), api.P("constraint", "YYYY-MM-DD format")))
		return
	}

	balances, err := h.glService.GetAccountBalances(r.Context(), orgID, from, to)
	if err != nil {
		h.logger.Error("GetAccountBalances failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, balances)
}
