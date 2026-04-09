package billing_test

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/billing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Test server setup ────────────────────────────────────────────────────────

type billingTestServer struct {
	server *httptest.Server
	repo   *mockBillingRepo
}

func setupBillingTestServer(t *testing.T) *billingTestServer {
	t.Helper()

	repo := newMockBillingRepo()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := billing.NewBillingService(repo, logger)
	handler := billing.NewBillingHandler(svc, logger)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/organizations/{org_id}/billing", handler.GetAccount)
	mux.HandleFunc("PUT /api/v1/organizations/{org_id}/billing", handler.UpdateAccount)
	mux.HandleFunc("GET /api/v1/organizations/{org_id}/invoices", handler.ListInvoices)
	mux.HandleFunc("GET /api/v1/organizations/{org_id}/invoices/{invoice_id}", handler.GetInvoice)
	mux.HandleFunc("POST /api/v1/webhooks/stripe", handler.StripeWebhook)

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	return &billingTestServer{server: server, repo: repo}
}

func doBillingRequest(t *testing.T, ts *billingTestServer, method, path string, body any) *http.Response {
	t.Helper()

	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		require.NoError(t, err)
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, ts.server.URL+path, bodyReader)
	require.NoError(t, err)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

func decodeBillingBody(t *testing.T, resp *http.Response, dst any) {
	t.Helper()
	defer resp.Body.Close()
	require.NoError(t, json.NewDecoder(resp.Body).Decode(dst))
}

// ─── GetAccount handler tests ─────────────────────────────────────────────────

func TestGetAccount_Handler(t *testing.T) {
	ts := setupBillingTestServer(t)

	orgID := uuid.New()
	ts.repo.accounts[orgID] = &billing.BillingAccount{
		ID:           uuid.New(),
		OrgID:        orgID,
		BillingEmail: "test@example.com",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	resp := doBillingRequest(t, ts, http.MethodGet,
		"/api/v1/organizations/"+orgID.String()+"/billing", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data *billing.BillingAccount `json:"data"`
	}
	decodeBillingBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, orgID, envelope.Data.OrgID)
	assert.Equal(t, "test@example.com", envelope.Data.BillingEmail)
}

func TestGetAccount_Handler_NotFound(t *testing.T) {
	ts := setupBillingTestServer(t)

	resp := doBillingRequest(t, ts, http.MethodGet,
		"/api/v1/organizations/"+uuid.New().String()+"/billing", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()
}

func TestGetAccount_Handler_InvalidOrgID(t *testing.T) {
	ts := setupBillingTestServer(t)

	resp := doBillingRequest(t, ts, http.MethodGet,
		"/api/v1/organizations/not-a-uuid/billing", nil)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()
}

// ─── UpdateAccount handler tests ──────────────────────────────────────────────

func TestUpdateAccount_Handler(t *testing.T) {
	ts := setupBillingTestServer(t)

	orgID := uuid.New()
	ts.repo.accounts[orgID] = &billing.BillingAccount{
		ID:           uuid.New(),
		OrgID:        orgID,
		BillingEmail: "old@example.com",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	newEmail := "new@example.com"
	resp := doBillingRequest(t, ts, http.MethodPut,
		"/api/v1/organizations/"+orgID.String()+"/billing",
		map[string]any{"billing_email": newEmail},
	)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data *billing.BillingAccount `json:"data"`
	}
	decodeBillingBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, newEmail, envelope.Data.BillingEmail)
}

// ─── ListInvoices handler tests ───────────────────────────────────────────────

func TestListInvoices_Handler(t *testing.T) {
	ts := setupBillingTestServer(t)

	orgID := uuid.New()
	now := time.Now()

	ts.repo.invoices[uuid.New()] = &billing.Invoice{
		ID: uuid.New(), OrgID: orgID, Status: "paid",
		PeriodStart: now.Add(-30 * 24 * time.Hour), PeriodEnd: now,
		CreatedAt: now, UpdatedAt: now,
	}
	ts.repo.invoices[uuid.New()] = &billing.Invoice{
		ID: uuid.New(), OrgID: orgID, Status: "draft",
		PeriodStart: now, PeriodEnd: now.Add(30 * 24 * time.Hour),
		CreatedAt: now, UpdatedAt: now,
	}

	resp := doBillingRequest(t, ts, http.MethodGet,
		"/api/v1/organizations/"+orgID.String()+"/invoices", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data []billing.Invoice `json:"data"`
	}
	decodeBillingBody(t, resp, &envelope)
	assert.Len(t, envelope.Data, 2)
}

func TestListInvoices_Handler_EmptyOrg(t *testing.T) {
	ts := setupBillingTestServer(t)

	resp := doBillingRequest(t, ts, http.MethodGet,
		"/api/v1/organizations/"+uuid.New().String()+"/invoices", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data []billing.Invoice `json:"data"`
	}
	decodeBillingBody(t, resp, &envelope)
	assert.Empty(t, envelope.Data)
}

// ─── GetInvoice handler tests ─────────────────────────────────────────────────

func TestGetInvoice_Handler(t *testing.T) {
	ts := setupBillingTestServer(t)

	orgID := uuid.New()
	invoiceID := uuid.New()
	now := time.Now()

	ts.repo.invoices[invoiceID] = &billing.Invoice{
		ID: invoiceID, OrgID: orgID, Status: "issued",
		PeriodStart: now, PeriodEnd: now.Add(30 * 24 * time.Hour),
		CreatedAt: now, UpdatedAt: now,
	}

	resp := doBillingRequest(t, ts, http.MethodGet,
		"/api/v1/organizations/"+orgID.String()+"/invoices/"+invoiceID.String(), nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data *billing.Invoice `json:"data"`
	}
	decodeBillingBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, invoiceID, envelope.Data.ID)
}

func TestGetInvoice_Handler_NotFound(t *testing.T) {
	ts := setupBillingTestServer(t)

	resp := doBillingRequest(t, ts, http.MethodGet,
		"/api/v1/organizations/"+uuid.New().String()+"/invoices/"+uuid.New().String(), nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()
}

func TestGetInvoice_Handler_InvalidInvoiceID(t *testing.T) {
	ts := setupBillingTestServer(t)

	resp := doBillingRequest(t, ts, http.MethodGet,
		"/api/v1/organizations/"+uuid.New().String()+"/invoices/not-a-uuid", nil)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()
}

// ─── StripeWebhook handler tests ──────────────────────────────────────────────

func TestStripeWebhook_Handler(t *testing.T) {
	ts := setupBillingTestServer(t)

	resp := doBillingRequest(t, ts, http.MethodPost,
		"/api/v1/webhooks/stripe",
		map[string]any{"type": "invoice.paid"},
	)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data map[string]string `json:"data"`
	}
	decodeBillingBody(t, resp, &envelope)
	assert.Equal(t, "ok", envelope.Data["status"])
}
