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

type collectionTestServer struct {
	server             *httptest.Server
	mockCollectionRepo *mockCollectionRepo
}

func setupCollectionTestServer(t *testing.T) *collectionTestServer {
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
		audit.NewNoopAuditor(),
		queue.NewInMemoryPublisher(),
		ai.NewNoopPolicyResolver(),
		ai.NewNoopComplianceResolver(),
		logger,
	)
	collectionHandler := fin.NewCollectionHandler(service, logger)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /organizations/{org_id}/collections", collectionHandler.ListCollections)
	mux.HandleFunc("GET /organizations/{org_id}/collections/{case_id}", collectionHandler.GetCollection)
	mux.HandleFunc("PATCH /organizations/{org_id}/collections/{case_id}", collectionHandler.UpdateCollection)
	mux.HandleFunc("POST /organizations/{org_id}/collections/{case_id}/actions", collectionHandler.AddCollectionAction)
	mux.HandleFunc("POST /organizations/{org_id}/collections/{case_id}/payment-plans", collectionHandler.CreatePaymentPlan)
	mux.HandleFunc("PATCH /organizations/{org_id}/payment-plans/{plan_id}", collectionHandler.UpdatePaymentPlan)
	mux.HandleFunc("GET /organizations/{org_id}/collections/{case_id}/payment-plans", collectionHandler.ListPaymentPlans)
	mux.HandleFunc("GET /organizations/{org_id}/units/{unit_id}/collection-status", collectionHandler.GetUnitCollectionStatus)

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	return &collectionTestServer{
		server:             server,
		mockCollectionRepo: mockCollectionRepo,
	}
}

// ── Seed helpers ──────────────────────────────────────────────────────────────

func seedCollectionCase(t *testing.T, repo *mockCollectionRepo, orgID, unitID uuid.UUID) *fin.CollectionCase {
	t.Helper()
	c := fin.CollectionCase{
		ID:               uuid.New(),
		OrgID:            orgID,
		UnitID:           unitID,
		Status:           "active",
		TotalOwedCents:   150000,
		CurrentOwedCents: 150000,
		OpenedAt:         time.Now(),
		Metadata:         map[string]any{},
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
	repo.cases = append(repo.cases, c)
	return &repo.cases[len(repo.cases)-1]
}

// ── ListCollections tests ─────────────────────────────────────────────────────

func TestListCollections_Success(t *testing.T) {
	ts := setupCollectionTestServer(t)
	orgID := uuid.New()
	unitID := uuid.New()
	seedCollectionCase(t, ts.mockCollectionRepo, orgID, unitID)
	seedCollectionCase(t, ts.mockCollectionRepo, orgID, unitID)

	resp := doFinRequest(t, ts.server.URL, http.MethodGet,
		fmt.Sprintf("/organizations/%s/collections", orgID), nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data []fin.CollectionCase `json:"data"`
	}
	decodeFinBody(t, resp, &envelope)
	assert.Len(t, envelope.Data, 2)
}

func TestListCollections_EmptyOrg(t *testing.T) {
	ts := setupCollectionTestServer(t)
	orgID := uuid.New()

	resp := doFinRequest(t, ts.server.URL, http.MethodGet,
		fmt.Sprintf("/organizations/%s/collections", orgID), nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data []fin.CollectionCase `json:"data"`
	}
	decodeFinBody(t, resp, &envelope)
	assert.Empty(t, envelope.Data)
}

// ── GetCollection tests ───────────────────────────────────────────────────────

func TestGetCollection_Success(t *testing.T) {
	ts := setupCollectionTestServer(t)
	orgID := uuid.New()
	unitID := uuid.New()
	c := seedCollectionCase(t, ts.mockCollectionRepo, orgID, unitID)

	resp := doFinRequest(t, ts.server.URL, http.MethodGet,
		fmt.Sprintf("/organizations/%s/collections/%s", orgID, c.ID), nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data *fin.CollectionCase `json:"data"`
	}
	decodeFinBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, c.ID, envelope.Data.ID)
	assert.Equal(t, unitID, envelope.Data.UnitID)
}

func TestGetCollection_NotFound(t *testing.T) {
	ts := setupCollectionTestServer(t)
	orgID := uuid.New()

	resp := doFinRequest(t, ts.server.URL, http.MethodGet,
		fmt.Sprintf("/organizations/%s/collections/%s", orgID, uuid.New()), nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestGetCollection_InvalidCaseID(t *testing.T) {
	ts := setupCollectionTestServer(t)
	orgID := uuid.New()

	resp := doFinRequest(t, ts.server.URL, http.MethodGet,
		fmt.Sprintf("/organizations/%s/collections/bad-uuid", orgID), nil)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// ── AddCollectionAction tests ─────────────────────────────────────────────────

func TestAddCollectionAction_Success(t *testing.T) {
	ts := setupCollectionTestServer(t)
	orgID := uuid.New()
	unitID := uuid.New()
	c := seedCollectionCase(t, ts.mockCollectionRepo, orgID, unitID)

	body := map[string]any{
		"action_type": "notice_sent",
	}
	resp := doFinRequest(t, ts.server.URL, http.MethodPost,
		fmt.Sprintf("/organizations/%s/collections/%s/actions", orgID, c.ID), body)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var envelope struct {
		Data *fin.CollectionAction `json:"data"`
	}
	decodeFinBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, c.ID, envelope.Data.CaseID)
	assert.Equal(t, "notice_sent", envelope.Data.ActionType)
	assert.NotEqual(t, uuid.Nil, envelope.Data.ID)
}

func TestAddCollectionAction_InvalidBody(t *testing.T) {
	ts := setupCollectionTestServer(t)
	orgID := uuid.New()
	unitID := uuid.New()
	c := seedCollectionCase(t, ts.mockCollectionRepo, orgID, unitID)

	// Missing action_type
	body := map[string]any{"notes": "some notes"}
	resp := doFinRequest(t, ts.server.URL, http.MethodPost,
		fmt.Sprintf("/organizations/%s/collections/%s/actions", orgID, c.ID), body)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// ── CreatePaymentPlan tests ───────────────────────────────────────────────────

func TestCreatePaymentPlan_Success(t *testing.T) {
	ts := setupCollectionTestServer(t)
	orgID := uuid.New()
	unitID := uuid.New()
	c := seedCollectionCase(t, ts.mockCollectionRepo, orgID, unitID)

	body := map[string]any{
		"total_owed_cents":    150000,
		"installment_cents":   30000,
		"installments_total":  5,
		"next_due_date":       time.Now().Add(30 * 24 * time.Hour).Format(time.RFC3339),
		"frequency":           "monthly",
	}
	resp := doFinRequest(t, ts.server.URL, http.MethodPost,
		fmt.Sprintf("/organizations/%s/collections/%s/payment-plans", orgID, c.ID), body)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var envelope struct {
		Data *fin.PaymentPlan `json:"data"`
	}
	decodeFinBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, c.ID, envelope.Data.CaseID)
	assert.Equal(t, orgID, envelope.Data.OrgID)
	assert.Equal(t, unitID, envelope.Data.UnitID)
	assert.Equal(t, int64(150000), envelope.Data.TotalOwedCents)
	assert.Equal(t, int64(30000), envelope.Data.InstallmentCents)
	assert.Equal(t, 5, envelope.Data.InstallmentsTotal)
	assert.Equal(t, "active", envelope.Data.Status)
	assert.NotEqual(t, uuid.Nil, envelope.Data.ID)
}

func TestCreatePaymentPlan_InvalidBody(t *testing.T) {
	ts := setupCollectionTestServer(t)
	orgID := uuid.New()
	unitID := uuid.New()
	c := seedCollectionCase(t, ts.mockCollectionRepo, orgID, unitID)

	// Missing required fields
	body := map[string]any{"total_owed_cents": 100000}
	resp := doFinRequest(t, ts.server.URL, http.MethodPost,
		fmt.Sprintf("/organizations/%s/collections/%s/payment-plans", orgID, c.ID), body)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestCreatePaymentPlan_CaseNotFound(t *testing.T) {
	ts := setupCollectionTestServer(t)
	orgID := uuid.New()

	body := map[string]any{
		"total_owed_cents":   150000,
		"installment_cents":  30000,
		"installments_total": 5,
		"next_due_date":      time.Now().Add(30 * 24 * time.Hour).Format(time.RFC3339),
	}
	resp := doFinRequest(t, ts.server.URL, http.MethodPost,
		fmt.Sprintf("/organizations/%s/collections/%s/payment-plans", orgID, uuid.New()), body)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// ── ListPaymentPlans tests ────────────────────────────────────────────────────

func TestListPaymentPlans_Success(t *testing.T) {
	ts := setupCollectionTestServer(t)
	orgID := uuid.New()
	unitID := uuid.New()
	c := seedCollectionCase(t, ts.mockCollectionRepo, orgID, unitID)

	// Seed a payment plan directly.
	ts.mockCollectionRepo.paymentPlans = append(ts.mockCollectionRepo.paymentPlans, fin.PaymentPlan{
		ID:                uuid.New(),
		CaseID:            c.ID,
		OrgID:             orgID,
		UnitID:            unitID,
		TotalOwedCents:    150000,
		InstallmentCents:  30000,
		InstallmentsTotal: 5,
		InstallmentsPaid:  0,
		NextDueDate:       time.Now().Add(30 * 24 * time.Hour),
		Frequency:         "monthly",
		Status:            "active",
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	})

	resp := doFinRequest(t, ts.server.URL, http.MethodGet,
		fmt.Sprintf("/organizations/%s/collections/%s/payment-plans", orgID, c.ID), nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data []fin.PaymentPlan `json:"data"`
	}
	decodeFinBody(t, resp, &envelope)
	assert.Len(t, envelope.Data, 1)
	assert.Equal(t, int64(150000), envelope.Data[0].TotalOwedCents)
}

// ── GetUnitCollectionStatus tests ─────────────────────────────────────────────

func TestGetUnitCollectionStatus_Success(t *testing.T) {
	ts := setupCollectionTestServer(t)
	orgID := uuid.New()
	unitID := uuid.New()
	c := seedCollectionCase(t, ts.mockCollectionRepo, orgID, unitID)

	resp := doFinRequest(t, ts.server.URL, http.MethodGet,
		fmt.Sprintf("/organizations/%s/units/%s/collection-status", orgID, unitID), nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data *fin.CollectionCase `json:"data"`
	}
	decodeFinBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, c.ID, envelope.Data.ID)
	assert.Equal(t, unitID, envelope.Data.UnitID)
}

// ── UpdatePaymentPlan tests ───────────────────────────────────────────────────

func TestUpdatePaymentPlan_Success(t *testing.T) {
	ts := setupCollectionTestServer(t)
	orgID := uuid.New()
	unitID := uuid.New()
	c := seedCollectionCase(t, ts.mockCollectionRepo, orgID, unitID)

	// Seed a payment plan directly.
	plan := fin.PaymentPlan{
		ID:                uuid.New(),
		CaseID:            c.ID,
		OrgID:             orgID,
		UnitID:            unitID,
		TotalOwedCents:    150000,
		InstallmentCents:  30000,
		InstallmentsTotal: 5,
		InstallmentsPaid:  1,
		NextDueDate:       time.Now().Add(30 * 24 * time.Hour),
		Frequency:         "monthly",
		Status:            "active",
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}
	ts.mockCollectionRepo.paymentPlans = append(ts.mockCollectionRepo.paymentPlans, plan)

	body := map[string]any{
		"installments_paid": 2,
		"status":            "active",
	}
	resp := doFinRequest(t, ts.server.URL, http.MethodPatch,
		fmt.Sprintf("/organizations/%s/payment-plans/%s", orgID, plan.ID), body)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data *fin.PaymentPlan `json:"data"`
	}
	decodeFinBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, plan.ID, envelope.Data.ID)
}

func TestGetUnitCollectionStatus_NoActiveCase(t *testing.T) {
	ts := setupCollectionTestServer(t)
	orgID := uuid.New()
	unitID := uuid.New()

	resp := doFinRequest(t, ts.server.URL, http.MethodGet,
		fmt.Sprintf("/organizations/%s/units/%s/collection-status", orgID, unitID), nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestGetUnitCollectionStatus_InvalidUnitID(t *testing.T) {
	ts := setupCollectionTestServer(t)
	orgID := uuid.New()

	resp := doFinRequest(t, ts.server.URL, http.MethodGet,
		fmt.Sprintf("/organizations/%s/units/bad-uuid/collection-status", orgID), nil)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}
