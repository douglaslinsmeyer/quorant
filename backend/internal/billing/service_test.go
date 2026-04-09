package billing_test

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/billing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Mock repository ─────────────────────────────────────────────────────────

// mockBillingRepo is an in-memory BillingRepository for service unit tests.
type mockBillingRepo struct {
	accounts  map[uuid.UUID]*billing.BillingAccount
	invoices  map[uuid.UUID]*billing.Invoice
	lineItems map[uuid.UUID][]billing.InvoiceLineItem

	createErr error
	findErr   error
	updateErr error
}

func newMockBillingRepo() *mockBillingRepo {
	return &mockBillingRepo{
		accounts:  make(map[uuid.UUID]*billing.BillingAccount),
		invoices:  make(map[uuid.UUID]*billing.Invoice),
		lineItems: make(map[uuid.UUID][]billing.InvoiceLineItem),
	}
}

// ─── Accounts ────────────────────────────────────────────────────────────────

func (m *mockBillingRepo) CreateAccount(_ context.Context, a *billing.BillingAccount) (*billing.BillingAccount, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	a.CreatedAt = time.Now()
	a.UpdatedAt = time.Now()
	cp := *a
	m.accounts[a.OrgID] = &cp
	return &cp, nil
}

func (m *mockBillingRepo) FindAccountByOrg(_ context.Context, orgID uuid.UUID) (*billing.BillingAccount, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	a, ok := m.accounts[orgID]
	if !ok {
		return nil, nil
	}
	cp := *a
	return &cp, nil
}

func (m *mockBillingRepo) UpdateAccount(_ context.Context, a *billing.BillingAccount) (*billing.BillingAccount, error) {
	if m.updateErr != nil {
		return nil, m.updateErr
	}
	a.UpdatedAt = time.Now()
	cp := *a
	m.accounts[a.OrgID] = &cp
	return &cp, nil
}

// ─── Invoices ────────────────────────────────────────────────────────────────

func (m *mockBillingRepo) CreateInvoice(_ context.Context, inv *billing.Invoice) (*billing.Invoice, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	if inv.ID == uuid.Nil {
		inv.ID = uuid.New()
	}
	inv.CreatedAt = time.Now()
	inv.UpdatedAt = time.Now()
	cp := *inv
	m.invoices[inv.ID] = &cp
	return &cp, nil
}

func (m *mockBillingRepo) FindInvoiceByID(_ context.Context, id uuid.UUID) (*billing.Invoice, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	inv, ok := m.invoices[id]
	if !ok {
		return nil, nil
	}
	cp := *inv
	return &cp, nil
}

func (m *mockBillingRepo) ListInvoicesByOrg(_ context.Context, orgID uuid.UUID) ([]billing.Invoice, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	var out []billing.Invoice
	for _, inv := range m.invoices {
		if inv.OrgID == orgID {
			out = append(out, *inv)
		}
	}
	return out, nil
}

func (m *mockBillingRepo) UpdateInvoice(_ context.Context, inv *billing.Invoice) (*billing.Invoice, error) {
	if m.updateErr != nil {
		return nil, m.updateErr
	}
	inv.UpdatedAt = time.Now()
	cp := *inv
	m.invoices[inv.ID] = &cp
	return &cp, nil
}

// ─── Line Items ───────────────────────────────────────────────────────────────

func (m *mockBillingRepo) CreateLineItem(_ context.Context, item *billing.InvoiceLineItem) (*billing.InvoiceLineItem, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	if item.ID == uuid.Nil {
		item.ID = uuid.New()
	}
	item.CreatedAt = time.Now()
	cp := *item
	m.lineItems[item.InvoiceID] = append(m.lineItems[item.InvoiceID], cp)
	return &cp, nil
}

func (m *mockBillingRepo) ListLineItemsByInvoice(_ context.Context, invoiceID uuid.UUID) ([]billing.InvoiceLineItem, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	return m.lineItems[invoiceID], nil
}

// ─── Test helpers ─────────────────────────────────────────────────────────────

func newTestBillingService(repo *mockBillingRepo) *billing.BillingService {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return billing.NewBillingService(repo, logger)
}

// ─── GetBillingAccount tests ──────────────────────────────────────────────────

func TestGetBillingAccount_NotFound(t *testing.T) {
	repo := newMockBillingRepo()
	svc := newTestBillingService(repo)

	_, err := svc.GetBillingAccount(context.Background(), uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestGetBillingAccount_Success(t *testing.T) {
	repo := newMockBillingRepo()
	svc := newTestBillingService(repo)

	orgID := uuid.New()
	repo.accounts[orgID] = &billing.BillingAccount{
		ID:           uuid.New(),
		OrgID:        orgID,
		BillingEmail: "billing@example.com",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	acct, err := svc.GetBillingAccount(context.Background(), orgID)
	require.NoError(t, err)
	require.NotNil(t, acct)
	assert.Equal(t, orgID, acct.OrgID)
	assert.Equal(t, "billing@example.com", acct.BillingEmail)
}

// ─── UpdateBillingAccount tests ───────────────────────────────────────────────

func TestUpdateBillingAccount_Success(t *testing.T) {
	repo := newMockBillingRepo()
	svc := newTestBillingService(repo)

	orgID := uuid.New()
	repo.accounts[orgID] = &billing.BillingAccount{
		ID:           uuid.New(),
		OrgID:        orgID,
		BillingEmail: "old@example.com",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	newEmail := "new@example.com"
	updated, err := svc.UpdateBillingAccount(context.Background(), orgID, billing.UpdateBillingAccountRequest{
		BillingEmail: &newEmail,
	})
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, newEmail, updated.BillingEmail)
}

func TestUpdateBillingAccount_ValidationError(t *testing.T) {
	repo := newMockBillingRepo()
	svc := newTestBillingService(repo)

	orgID := uuid.New()
	// No fields → validation error.
	_, err := svc.UpdateBillingAccount(context.Background(), orgID, billing.UpdateBillingAccountRequest{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one field")
}

// ─── ListInvoices tests ───────────────────────────────────────────────────────

func TestListInvoices_Success(t *testing.T) {
	repo := newMockBillingRepo()
	svc := newTestBillingService(repo)

	orgID := uuid.New()
	now := time.Now()

	// Seed two invoices.
	inv1 := &billing.Invoice{
		ID:          uuid.New(),
		OrgID:       orgID,
		Status:      "paid",
		PeriodStart: now.Add(-30 * 24 * time.Hour),
		PeriodEnd:   now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	inv2 := &billing.Invoice{
		ID:          uuid.New(),
		OrgID:       orgID,
		Status:      "draft",
		PeriodStart: now,
		PeriodEnd:   now.Add(30 * 24 * time.Hour),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	repo.invoices[inv1.ID] = inv1
	repo.invoices[inv2.ID] = inv2

	invoices, err := svc.ListInvoices(context.Background(), orgID)
	require.NoError(t, err)
	assert.Len(t, invoices, 2)
}

func TestListInvoices_EmptyForUnknownOrg(t *testing.T) {
	repo := newMockBillingRepo()
	svc := newTestBillingService(repo)

	invoices, err := svc.ListInvoices(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Empty(t, invoices)
}

// ─── GetInvoice tests ─────────────────────────────────────────────────────────

func TestGetInvoice_NotFound(t *testing.T) {
	repo := newMockBillingRepo()
	svc := newTestBillingService(repo)

	_, err := svc.GetInvoice(context.Background(), uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestGetInvoice_Success(t *testing.T) {
	repo := newMockBillingRepo()
	svc := newTestBillingService(repo)

	id := uuid.New()
	repo.invoices[id] = &billing.Invoice{
		ID:          id,
		OrgID:       uuid.New(),
		Status:      "issued",
		PeriodStart: time.Now(),
		PeriodEnd:   time.Now().Add(30 * 24 * time.Hour),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	inv, err := svc.GetInvoice(context.Background(), id)
	require.NoError(t, err)
	require.NotNil(t, inv)
	assert.Equal(t, id, inv.ID)
}
