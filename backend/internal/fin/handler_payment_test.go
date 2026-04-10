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
	"github.com/quorant/quorant/internal/fin"
	"github.com/quorant/quorant/internal/platform/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── Test server setup ─────────────────────────────────────────────────────────

type paymentTestServer struct {
	server          *httptest.Server
	mockAssessRepo  *mockAssessmentRepo
	mockPaymentRepo *mockPaymentRepo
	userID          uuid.UUID
}

func setupPaymentTestServer(t *testing.T) *paymentTestServer {
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
		ai.NewNoopPolicyResolver(),
		ai.NewNoopComplianceResolver(),
		nil,
		logger,
		nil,
	)
	payHandler := fin.NewPaymentHandler(service, logger)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /organizations/{org_id}/payments", payHandler.RecordPayment)
	mux.HandleFunc("GET /organizations/{org_id}/payments", payHandler.ListPayments)
	mux.HandleFunc("GET /organizations/{org_id}/payments/{payment_id}", payHandler.GetPayment)
	mux.HandleFunc("POST /organizations/{org_id}/payment-methods", payHandler.AddPaymentMethod)
	mux.HandleFunc("GET /organizations/{org_id}/payment-methods", payHandler.ListPaymentMethods)
	mux.HandleFunc("DELETE /organizations/{org_id}/payment-methods/{method_id}", payHandler.RemovePaymentMethod)

	testUserID := uuid.New()
	handlerWithUserID := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := middleware.WithUserID(r.Context(), testUserID)
		mux.ServeHTTP(w, r.WithContext(ctx))
	})
	server := httptest.NewServer(handlerWithUserID)
	t.Cleanup(server.Close)

	return &paymentTestServer{
		server:          server,
		mockAssessRepo:  mockAssessRepo,
		mockPaymentRepo: mockPaymentRepo,
		userID:          testUserID,
	}
}

// seedPayment pre-seeds a payment into the mock payment repo.
func seedPayment(t *testing.T, repo *mockPaymentRepo, orgID, unitID uuid.UUID) *fin.Payment {
	t.Helper()
	now := time.Now()
	p := &fin.Payment{
		ID:          uuid.New(),
		OrgID:       orgID,
		UnitID:      unitID,
		UserID:      uuid.New(),
		AmountCents: 25000,
		Status:      fin.PaymentStatusCompleted,
		PaidAt:      &now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	repo.payments = append(repo.payments, *p)
	return p
}

// seedPaymentMethod pre-seeds a payment method into the mock payment repo.
func seedPaymentMethod(t *testing.T, repo *mockPaymentRepo, orgID, userID uuid.UUID) *fin.PaymentMethod {
	t.Helper()
	pm := &fin.PaymentMethod{
		ID:         uuid.New(),
		OrgID:      orgID,
		UserID:     userID,
		MethodType: fin.PaymentMethodTypeACH,
		IsDefault:  true,
		CreatedAt:  time.Now(),
	}
	repo.methods = append(repo.methods, *pm)
	return pm
}

// ── RecordPayment tests ───────────────────────────────────────────────────────

func TestRecordPayment_Success(t *testing.T) {
	ts := setupPaymentTestServer(t)
	orgID := uuid.New()
	unitID := uuid.New()

	body := map[string]any{
		"unit_id":      unitID,
		"amount_cents": 25000,
	}
	resp := doFinRequest(t, ts.server.URL, http.MethodPost,
		fmt.Sprintf("/organizations/%s/payments", orgID), body)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var envelope struct {
		Data *fin.Payment `json:"data"`
	}
	decodeFinBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, orgID, envelope.Data.OrgID)
	assert.Equal(t, unitID, envelope.Data.UnitID)
	assert.Equal(t, int64(25000), envelope.Data.AmountCents)
	assert.Equal(t, fin.PaymentStatusCompleted, envelope.Data.Status)
	assert.NotEqual(t, uuid.Nil, envelope.Data.ID)

	// Verify ledger credit was created.
	assert.Len(t, ts.mockAssessRepo.ledger, 1)
	assert.Equal(t, fin.LedgerEntryTypePayment, ts.mockAssessRepo.ledger[0].EntryType)
	assert.Equal(t, int64(-25000), ts.mockAssessRepo.ledger[0].AmountCents)
}

func TestRecordPayment_InvalidBody(t *testing.T) {
	ts := setupPaymentTestServer(t)
	orgID := uuid.New()

	// Missing required unit_id
	body := map[string]any{"amount_cents": 25000}
	resp := doFinRequest(t, ts.server.URL, http.MethodPost,
		fmt.Sprintf("/organizations/%s/payments", orgID), body)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestRecordPayment_InvalidOrgID(t *testing.T) {
	ts := setupPaymentTestServer(t)

	body := map[string]any{
		"unit_id":      uuid.New(),
		"amount_cents": 10000,
	}
	resp := doFinRequest(t, ts.server.URL, http.MethodPost,
		"/organizations/not-a-uuid/payments", body)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// ── ListPayments tests ────────────────────────────────────────────────────────

func TestListPayments_Success(t *testing.T) {
	ts := setupPaymentTestServer(t)
	orgID := uuid.New()
	unitID := uuid.New()
	seedPayment(t, ts.mockPaymentRepo, orgID, unitID)
	seedPayment(t, ts.mockPaymentRepo, orgID, unitID)

	resp := doFinRequest(t, ts.server.URL, http.MethodGet,
		fmt.Sprintf("/organizations/%s/payments", orgID), nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data []fin.Payment `json:"data"`
	}
	decodeFinBody(t, resp, &envelope)
	assert.Len(t, envelope.Data, 2)
}

func TestListPayments_EmptyOrg(t *testing.T) {
	ts := setupPaymentTestServer(t)
	orgID := uuid.New()

	resp := doFinRequest(t, ts.server.URL, http.MethodGet,
		fmt.Sprintf("/organizations/%s/payments", orgID), nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data []fin.Payment `json:"data"`
	}
	decodeFinBody(t, resp, &envelope)
	assert.Empty(t, envelope.Data)
}

// ── GetPayment tests ──────────────────────────────────────────────────────────

func TestGetPayment_Success(t *testing.T) {
	ts := setupPaymentTestServer(t)
	orgID := uuid.New()
	unitID := uuid.New()
	payment := seedPayment(t, ts.mockPaymentRepo, orgID, unitID)

	resp := doFinRequest(t, ts.server.URL, http.MethodGet,
		fmt.Sprintf("/organizations/%s/payments/%s", orgID, payment.ID), nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data *fin.Payment `json:"data"`
	}
	decodeFinBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, payment.ID, envelope.Data.ID)
}

func TestGetPayment_NotFound(t *testing.T) {
	ts := setupPaymentTestServer(t)
	orgID := uuid.New()

	resp := doFinRequest(t, ts.server.URL, http.MethodGet,
		fmt.Sprintf("/organizations/%s/payments/%s", orgID, uuid.New()), nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestGetPayment_InvalidPaymentID(t *testing.T) {
	ts := setupPaymentTestServer(t)
	orgID := uuid.New()

	resp := doFinRequest(t, ts.server.URL, http.MethodGet,
		fmt.Sprintf("/organizations/%s/payments/bad-uuid", orgID), nil)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// ── AddPaymentMethod tests ────────────────────────────────────────────────────

func TestAddPaymentMethod_Success(t *testing.T) {
	ts := setupPaymentTestServer(t)
	orgID := uuid.New()
	userID := uuid.New()

	body := map[string]any{
		"user_id":     userID,
		"method_type": "ach",
		"is_default":  true,
	}
	resp := doFinRequest(t, ts.server.URL, http.MethodPost,
		fmt.Sprintf("/organizations/%s/payment-methods", orgID), body)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var envelope struct {
		Data *fin.PaymentMethod `json:"data"`
	}
	decodeFinBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, orgID, envelope.Data.OrgID)
	assert.Equal(t, fin.PaymentMethodTypeACH, envelope.Data.MethodType)
	assert.NotEqual(t, uuid.Nil, envelope.Data.ID)
}

func TestAddPaymentMethod_InvalidBody(t *testing.T) {
	ts := setupPaymentTestServer(t)
	orgID := uuid.New()

	req, err := http.NewRequest(http.MethodPost,
		ts.server.URL+fmt.Sprintf("/organizations/%s/payment-methods", orgID),
		nil)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// ── ListPaymentMethods tests ──────────────────────────────────────────────────

func TestListPaymentMethods_Success(t *testing.T) {
	ts := setupPaymentTestServer(t)
	orgID := uuid.New()
	seedPaymentMethod(t, ts.mockPaymentRepo, orgID, ts.userID)
	seedPaymentMethod(t, ts.mockPaymentRepo, orgID, ts.userID)

	resp := doFinRequest(t, ts.server.URL, http.MethodGet,
		fmt.Sprintf("/organizations/%s/payment-methods", orgID), nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data []fin.PaymentMethod `json:"data"`
	}
	decodeFinBody(t, resp, &envelope)
	assert.Len(t, envelope.Data, 2)
}

// ── RemovePaymentMethod tests ─────────────────────────────────────────────────

func TestRemovePaymentMethod_Success(t *testing.T) {
	ts := setupPaymentTestServer(t)
	orgID := uuid.New()
	userID := uuid.New()
	method := seedPaymentMethod(t, ts.mockPaymentRepo, orgID, userID)

	resp := doFinRequest(t, ts.server.URL, http.MethodDelete,
		fmt.Sprintf("/organizations/%s/payment-methods/%s", orgID, method.ID), nil)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	resp.Body.Close()

	// Verify it was soft-deleted.
	require.NotEmpty(t, ts.mockPaymentRepo.methods)
	assert.NotNil(t, ts.mockPaymentRepo.methods[0].DeletedAt)
}

func TestRemovePaymentMethod_InvalidMethodID(t *testing.T) {
	ts := setupPaymentTestServer(t)
	orgID := uuid.New()

	resp := doFinRequest(t, ts.server.URL, http.MethodDelete,
		fmt.Sprintf("/organizations/%s/payment-methods/bad-uuid", orgID), nil)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}
