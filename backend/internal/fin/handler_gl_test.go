package fin_test

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/audit"
	"github.com/quorant/quorant/internal/fin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── Test server setup ────────────────────────────────────────────────────────

type glTestServer struct {
	server     *httptest.Server
	mockGLRepo *mockGLRepo
}

func setupGLTestServer(t *testing.T) *glTestServer {
	t.Helper()

	glRepo := newMockGLRepo()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	glService := fin.NewGLService(glRepo, audit.NewNoopAuditor(), logger)
	glHandler := fin.NewGLHandler(glService, logger)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /organizations/{org_id}/gl/accounts", glHandler.CreateAccount)
	mux.HandleFunc("GET /organizations/{org_id}/gl/accounts", glHandler.ListAccounts)
	mux.HandleFunc("GET /organizations/{org_id}/gl/accounts/{account_id}", glHandler.GetAccount)
	mux.HandleFunc("PATCH /organizations/{org_id}/gl/accounts/{account_id}", glHandler.UpdateAccount)
	mux.HandleFunc("DELETE /organizations/{org_id}/gl/accounts/{account_id}", glHandler.DeleteAccount)
	mux.HandleFunc("POST /organizations/{org_id}/gl/journal-entries", glHandler.CreateJournalEntry)
	mux.HandleFunc("GET /organizations/{org_id}/gl/journal-entries", glHandler.ListJournalEntries)
	mux.HandleFunc("GET /organizations/{org_id}/gl/journal-entries/{entry_id}", glHandler.GetJournalEntry)
	mux.HandleFunc("GET /organizations/{org_id}/gl/trial-balance", glHandler.GetTrialBalance)
	mux.HandleFunc("GET /organizations/{org_id}/gl/account-balances", glHandler.GetAccountBalances)

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	return &glTestServer{
		server:     server,
		mockGLRepo: glRepo,
	}
}

// ── Seed helpers ─────────────────────────────────────────────────────────────

func seedGLAccount(t *testing.T, repo *mockGLRepo, orgID uuid.UUID, number int, name string, acctType fin.GLAccountType) *fin.GLAccount {
	t.Helper()
	a := &fin.GLAccount{
		ID:            uuid.New(),
		OrgID:         orgID,
		AccountNumber: number,
		Name:          name,
		AccountType:   acctType,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	repo.accounts[a.ID] = a
	return a
}

// ── CreateAccount tests ──────────────────────────────────────────────────────

func TestGLHandler_CreateAccount_201(t *testing.T) {
	ts := setupGLTestServer(t)
	orgID := uuid.New()

	body := map[string]any{
		"account_number": 1010,
		"name":           "Cash-Operating",
		"account_type":   "asset",
	}
	resp := doFinRequest(t, ts.server.URL, http.MethodPost,
		fmt.Sprintf("/organizations/%s/gl/accounts", orgID), body)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var envelope struct {
		Data *fin.GLAccount `json:"data"`
	}
	decodeFinBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, orgID, envelope.Data.OrgID)
	assert.Equal(t, "Cash-Operating", envelope.Data.Name)
	assert.Equal(t, fin.GLAccountTypeAsset, envelope.Data.AccountType)
	assert.Equal(t, 1010, envelope.Data.AccountNumber)
	assert.NotEqual(t, uuid.Nil, envelope.Data.ID)
}

func TestGLHandler_CreateAccount_400(t *testing.T) {
	ts := setupGLTestServer(t)
	orgID := uuid.New()

	// Missing name (required field)
	body := map[string]any{
		"account_number": 1010,
		"account_type":   "asset",
	}
	resp := doFinRequest(t, ts.server.URL, http.MethodPost,
		fmt.Sprintf("/organizations/%s/gl/accounts", orgID), body)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// ── ListAccounts tests ───────────────────────────────────────────────────────

func TestGLHandler_ListAccounts_200(t *testing.T) {
	ts := setupGLTestServer(t)
	orgID := uuid.New()
	seedGLAccount(t, ts.mockGLRepo, orgID, 1010, "Cash-Operating", fin.GLAccountTypeAsset)
	seedGLAccount(t, ts.mockGLRepo, orgID, 2100, "Accounts Payable", fin.GLAccountTypeLiability)

	resp := doFinRequest(t, ts.server.URL, http.MethodGet,
		fmt.Sprintf("/organizations/%s/gl/accounts", orgID), nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data []fin.GLAccount `json:"data"`
	}
	decodeFinBody(t, resp, &envelope)
	assert.Len(t, envelope.Data, 2)
}

// ── GetAccount tests ─────────────────────────────────────────────────────────

func TestGLHandler_GetAccount_200(t *testing.T) {
	ts := setupGLTestServer(t)
	orgID := uuid.New()
	acct := seedGLAccount(t, ts.mockGLRepo, orgID, 1010, "Cash-Operating", fin.GLAccountTypeAsset)

	resp := doFinRequest(t, ts.server.URL, http.MethodGet,
		fmt.Sprintf("/organizations/%s/gl/accounts/%s", orgID, acct.ID), nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data *fin.GLAccount `json:"data"`
	}
	decodeFinBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, acct.ID, envelope.Data.ID)
	assert.Equal(t, "Cash-Operating", envelope.Data.Name)
}

func TestGLHandler_GetAccount_404(t *testing.T) {
	ts := setupGLTestServer(t)
	orgID := uuid.New()

	resp := doFinRequest(t, ts.server.URL, http.MethodGet,
		fmt.Sprintf("/organizations/%s/gl/accounts/%s", orgID, uuid.New()), nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// ── DeleteAccount tests ──────────────────────────────────────────────────────

func TestGLHandler_DeleteAccount_204(t *testing.T) {
	ts := setupGLTestServer(t)
	orgID := uuid.New()
	acct := seedGLAccount(t, ts.mockGLRepo, orgID, 1110, "AR-Other", fin.GLAccountTypeAsset)

	resp := doFinRequest(t, ts.server.URL, http.MethodDelete,
		fmt.Sprintf("/organizations/%s/gl/accounts/%s", orgID, acct.ID), nil)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	resp.Body.Close()

	// Verify it was soft-deleted.
	require.NotNil(t, ts.mockGLRepo.accounts[acct.ID].DeletedAt)
}

// ── CreateJournalEntry tests ─────────────────────────────────────────────────

func TestGLHandler_CreateJournalEntry_201(t *testing.T) {
	ts := setupGLTestServer(t)
	orgID := uuid.New()
	debitAcct := seedGLAccount(t, ts.mockGLRepo, orgID, 1010, "Cash-Operating", fin.GLAccountTypeAsset)
	creditAcct := seedGLAccount(t, ts.mockGLRepo, orgID, 4010, "Assessment Revenue", fin.GLAccountTypeRevenue)

	body := map[string]any{
		"entry_date": time.Now().Format(time.RFC3339),
		"memo":       "Monthly assessment revenue",
		"lines": []map[string]any{
			{
				"account_id":  debitAcct.ID,
				"debit_cents": 50000,
			},
			{
				"account_id":   creditAcct.ID,
				"credit_cents": 50000,
			},
		},
	}
	resp := doFinRequest(t, ts.server.URL, http.MethodPost,
		fmt.Sprintf("/organizations/%s/gl/journal-entries", orgID), body)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var envelope struct {
		Data *fin.GLJournalEntry `json:"data"`
	}
	decodeFinBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, orgID, envelope.Data.OrgID)
	assert.Equal(t, "Monthly assessment revenue", envelope.Data.Memo)
	assert.Len(t, envelope.Data.Lines, 2)
	assert.NotEqual(t, uuid.Nil, envelope.Data.ID)
	assert.Equal(t, 1, envelope.Data.EntryNumber)
}

func TestGLHandler_CreateJournalEntry_400_Unbalanced(t *testing.T) {
	ts := setupGLTestServer(t)
	orgID := uuid.New()
	debitAcct := seedGLAccount(t, ts.mockGLRepo, orgID, 1010, "Cash-Operating", fin.GLAccountTypeAsset)
	creditAcct := seedGLAccount(t, ts.mockGLRepo, orgID, 4010, "Assessment Revenue", fin.GLAccountTypeRevenue)

	body := map[string]any{
		"entry_date": time.Now().Format(time.RFC3339),
		"memo":       "Unbalanced entry",
		"lines": []map[string]any{
			{
				"account_id":  debitAcct.ID,
				"debit_cents": 50000,
			},
			{
				"account_id":   creditAcct.ID,
				"credit_cents": 30000,
			},
		},
	}
	resp := doFinRequest(t, ts.server.URL, http.MethodPost,
		fmt.Sprintf("/organizations/%s/gl/journal-entries", orgID), body)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// ── GetTrialBalance tests ────────────────────────────────────────────────────

func TestGLHandler_GetTrialBalance_200(t *testing.T) {
	ts := setupGLTestServer(t)
	orgID := uuid.New()

	// The mock GetTrialBalance returns an empty slice, which is still a 200 OK
	// with an empty array.
	resp := doFinRequest(t, ts.server.URL, http.MethodGet,
		fmt.Sprintf("/organizations/%s/gl/trial-balance", orgID), nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data []fin.TrialBalanceRow `json:"data"`
	}
	decodeFinBody(t, resp, &envelope)
	assert.NotNil(t, envelope.Data)
}
