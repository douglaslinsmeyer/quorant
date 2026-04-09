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
	"github.com/quorant/quorant/internal/ai"
	"github.com/quorant/quorant/internal/audit"
	"github.com/quorant/quorant/internal/fin"
	"github.com/quorant/quorant/internal/platform/queue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── Test server setup ─────────────────────────────────────────────────────────

type fundTestServer struct {
	server       *httptest.Server
	mockFundRepo *mockFundRepo
}

func setupFundTestServer(t *testing.T) *fundTestServer {
	t.Helper()

	mockAssessRepo := &mockAssessmentRepo{}
	mockPaymentRepo := &mockPaymentRepo{}
	mockBudgetRepo := &mockBudgetRepo{}
	mockFundRepo := &mockFundRepo{}
	mockCollectionRepo := &mockCollectionRepo{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	service := fin.NewFinService(
		mockAssessRepo,
		mockPaymentRepo,
		mockBudgetRepo,
		mockFundRepo,
		mockCollectionRepo,
		nil,
		audit.NewNoopAuditor(),
		queue.NewInMemoryPublisher(),
		ai.NewNoopPolicyResolver(),
		logger,
	)
	fundHandler := fin.NewFundHandler(service, logger)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /organizations/{org_id}/funds", fundHandler.CreateFund)
	mux.HandleFunc("GET /organizations/{org_id}/funds", fundHandler.ListFunds)
	mux.HandleFunc("GET /organizations/{org_id}/funds/{fund_id}", fundHandler.GetFund)
	mux.HandleFunc("GET /organizations/{org_id}/funds/{fund_id}/transactions", fundHandler.GetFundTransactions)
	mux.HandleFunc("POST /organizations/{org_id}/fund-transfers", fundHandler.CreateFundTransfer)
	mux.HandleFunc("GET /organizations/{org_id}/fund-transfers", fundHandler.ListFundTransfers)

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	return &fundTestServer{
		server:       server,
		mockFundRepo: mockFundRepo,
	}
}

// ── Seed helpers ──────────────────────────────────────────────────────────────

func seedFund(t *testing.T, repo *mockFundRepo, orgID uuid.UUID) *fin.Fund {
	t.Helper()
	f := fin.Fund{
		ID:           uuid.New(),
		OrgID:        orgID,
		Name:         "Operating Fund",
		FundType:     "operating",
		BalanceCents: 0,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	repo.funds = append(repo.funds, f)
	return &repo.funds[len(repo.funds)-1]
}

// ── CreateFund tests ──────────────────────────────────────────────────────────

func TestCreateFundHandler_Success(t *testing.T) {
	ts := setupFundTestServer(t)
	orgID := uuid.New()

	body := map[string]any{
		"name":      "Operating Fund",
		"fund_type": "operating",
	}
	resp := doFinRequest(t, ts.server.URL, http.MethodPost,
		fmt.Sprintf("/organizations/%s/funds", orgID), body)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var envelope struct {
		Data *fin.Fund `json:"data"`
	}
	decodeFinBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, orgID, envelope.Data.OrgID)
	assert.Equal(t, "Operating Fund", envelope.Data.Name)
	assert.Equal(t, "operating", envelope.Data.FundType)
	assert.NotEqual(t, uuid.Nil, envelope.Data.ID)
}

func TestCreateFund_InvalidBody(t *testing.T) {
	ts := setupFundTestServer(t)
	orgID := uuid.New()

	// Missing required fund_type
	body := map[string]any{"name": "Unnamed Fund"}
	resp := doFinRequest(t, ts.server.URL, http.MethodPost,
		fmt.Sprintf("/organizations/%s/funds", orgID), body)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestCreateFund_InvalidFundType(t *testing.T) {
	ts := setupFundTestServer(t)
	orgID := uuid.New()

	body := map[string]any{
		"name":      "Bad Fund",
		"fund_type": "invalid_type",
	}
	resp := doFinRequest(t, ts.server.URL, http.MethodPost,
		fmt.Sprintf("/organizations/%s/funds", orgID), body)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// ── ListFunds tests ───────────────────────────────────────────────────────────

func TestListFunds_Success(t *testing.T) {
	ts := setupFundTestServer(t)
	orgID := uuid.New()
	seedFund(t, ts.mockFundRepo, orgID)
	seedFund(t, ts.mockFundRepo, orgID)

	resp := doFinRequest(t, ts.server.URL, http.MethodGet,
		fmt.Sprintf("/organizations/%s/funds", orgID), nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data []fin.Fund `json:"data"`
	}
	decodeFinBody(t, resp, &envelope)
	assert.Len(t, envelope.Data, 2)
}

func TestListFunds_EmptyOrg(t *testing.T) {
	ts := setupFundTestServer(t)
	orgID := uuid.New()

	resp := doFinRequest(t, ts.server.URL, http.MethodGet,
		fmt.Sprintf("/organizations/%s/funds", orgID), nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data []fin.Fund `json:"data"`
	}
	decodeFinBody(t, resp, &envelope)
	assert.Empty(t, envelope.Data)
}

// ── GetFund tests ─────────────────────────────────────────────────────────────

func TestGetFund_Success(t *testing.T) {
	ts := setupFundTestServer(t)
	orgID := uuid.New()
	fund := seedFund(t, ts.mockFundRepo, orgID)

	resp := doFinRequest(t, ts.server.URL, http.MethodGet,
		fmt.Sprintf("/organizations/%s/funds/%s", orgID, fund.ID), nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data *fin.Fund `json:"data"`
	}
	decodeFinBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, fund.ID, envelope.Data.ID)
}

func TestGetFund_NotFound(t *testing.T) {
	ts := setupFundTestServer(t)
	orgID := uuid.New()

	resp := doFinRequest(t, ts.server.URL, http.MethodGet,
		fmt.Sprintf("/organizations/%s/funds/%s", orgID, uuid.New()), nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// ── GetFundTransactions tests ─────────────────────────────────────────────────

func TestGetFundTransactions_Success(t *testing.T) {
	ts := setupFundTestServer(t)
	orgID := uuid.New()
	fund := seedFund(t, ts.mockFundRepo, orgID)

	// Seed a transaction directly.
	ts.mockFundRepo.transactions = append(ts.mockFundRepo.transactions, fin.FundTransaction{
		ID:                uuid.New(),
		FundID:            fund.ID,
		OrgID:             orgID,
		TransactionType:   "credit",
		AmountCents:       100000,
		BalanceAfterCents: 100000,
		EffectiveDate:     time.Now(),
		CreatedAt:         time.Now(),
	})

	resp := doFinRequest(t, ts.server.URL, http.MethodGet,
		fmt.Sprintf("/organizations/%s/funds/%s/transactions", orgID, fund.ID), nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data []fin.FundTransaction `json:"data"`
	}
	decodeFinBody(t, resp, &envelope)
	assert.Len(t, envelope.Data, 1)
	assert.Equal(t, int64(100000), envelope.Data[0].AmountCents)
}

// ── CreateFundTransfer tests ──────────────────────────────────────────────────

func TestCreateFundTransfer_Success(t *testing.T) {
	ts := setupFundTestServer(t)
	orgID := uuid.New()
	fromFund := seedFund(t, ts.mockFundRepo, orgID)
	toFund := seedFund(t, ts.mockFundRepo, orgID)

	body := map[string]any{
		"from_fund_id": fromFund.ID,
		"to_fund_id":   toFund.ID,
		"amount_cents": 10000,
	}
	resp := doFinRequest(t, ts.server.URL, http.MethodPost,
		fmt.Sprintf("/organizations/%s/fund-transfers", orgID), body)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var envelope struct {
		Data *fin.FundTransfer `json:"data"`
	}
	decodeFinBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, orgID, envelope.Data.OrgID)
	assert.Equal(t, fromFund.ID, envelope.Data.FromFundID)
	assert.Equal(t, toFund.ID, envelope.Data.ToFundID)
	assert.Equal(t, int64(10000), envelope.Data.AmountCents)
	assert.NotEqual(t, uuid.Nil, envelope.Data.ID)
}

func TestCreateFundTransfer_InvalidBody(t *testing.T) {
	ts := setupFundTestServer(t)
	orgID := uuid.New()

	// Missing to_fund_id and amount_cents
	body := map[string]any{"from_fund_id": uuid.New()}
	resp := doFinRequest(t, ts.server.URL, http.MethodPost,
		fmt.Sprintf("/organizations/%s/fund-transfers", orgID), body)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// ── ListFundTransfers tests ───────────────────────────────────────────────────

func TestListFundTransfers_Success(t *testing.T) {
	ts := setupFundTestServer(t)
	orgID := uuid.New()

	// Seed transfers directly.
	ts.mockFundRepo.transfers = append(ts.mockFundRepo.transfers,
		fin.FundTransfer{
			ID:            uuid.New(),
			OrgID:         orgID,
			FromFundID:    uuid.New(),
			ToFundID:      uuid.New(),
			AmountCents:   5000,
			EffectiveDate: time.Now(),
			CreatedAt:     time.Now(),
		},
	)

	resp := doFinRequest(t, ts.server.URL, http.MethodGet,
		fmt.Sprintf("/organizations/%s/fund-transfers", orgID), nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data []fin.FundTransfer `json:"data"`
	}
	decodeFinBody(t, resp, &envelope)
	assert.Len(t, envelope.Data, 1)
}
