//go:build integration

package billing_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/quorant/quorant/internal/billing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Test DB setup ────────────────────────────────────────────────────────────

func setupBillingTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, "postgres://quorant:quorant@localhost:5432/quorant_dev?sslmode=disable")
	require.NoError(t, err, "connecting to test database")

	t.Cleanup(func() {
		cleanCtx := context.Background()
		pool.Exec(cleanCtx, "DELETE FROM invoice_line_items")
		pool.Exec(cleanCtx, "DELETE FROM invoices")
		pool.Exec(cleanCtx, "DELETE FROM billing_accounts")
		pool.Exec(cleanCtx, "DELETE FROM organizations")
		pool.Close()
	})

	return pool
}

// createBillingTestOrg inserts a minimal organization for use in tests.
func createBillingTestOrg(t *testing.T, pool *pgxpool.Pool, orgType, name string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	suffix := uuid.New().String()
	safeSuffix := strings.ReplaceAll(suffix, "-", "_")
	safeName := strings.ToLower(strings.ReplaceAll(name, " ", "_"))
	path := safeName + "_" + safeSuffix
	slug := safeName + "-" + suffix
	err := pool.QueryRow(context.Background(),
		`INSERT INTO organizations (type, name, slug, path) VALUES ($1, $2, $3, $4) RETURNING id`,
		orgType, name, slug, path,
	).Scan(&id)
	require.NoError(t, err)
	return id
}

// ─── BillingAccount tests ─────────────────────────────────────────────────────

func TestCreateAccount_StoresAndFindsByOrg(t *testing.T) {
	pool := setupBillingTestDB(t)
	repo := billing.NewPostgresBillingRepository(pool)
	ctx := context.Background()

	orgID := createBillingTestOrg(t, pool, "hoa", "Create Account HOA")

	input := &billing.BillingAccount{
		OrgID:        orgID,
		BillingEmail: "billing@create-test.com",
	}

	created, err := repo.CreateAccount(ctx, input)

	require.NoError(t, err)
	require.NotNil(t, created)
	assert.NotEqual(t, uuid.Nil, created.ID)
	assert.Equal(t, orgID, created.OrgID)
	assert.Equal(t, "billing@create-test.com", created.BillingEmail)
	assert.Nil(t, created.StripeCustomerID)
	assert.Nil(t, created.BillingName)
	assert.False(t, created.CreatedAt.IsZero())

	// FindAccountByOrg should return the same account.
	found, err := repo.FindAccountByOrg(ctx, orgID)

	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, created.ID, found.ID)
	assert.Equal(t, created.BillingEmail, found.BillingEmail)
}

func TestFindAccountByOrg_ReturnsNilWhenNotFound(t *testing.T) {
	pool := setupBillingTestDB(t)
	repo := billing.NewPostgresBillingRepository(pool)
	ctx := context.Background()

	found, err := repo.FindAccountByOrg(ctx, uuid.New())

	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestUpdateAccount_UpdatesFieldsAndReturnsUpdated(t *testing.T) {
	pool := setupBillingTestDB(t)
	repo := billing.NewPostgresBillingRepository(pool)
	ctx := context.Background()

	orgID := createBillingTestOrg(t, pool, "hoa", "Update Account HOA")
	created, err := repo.CreateAccount(ctx, &billing.BillingAccount{
		OrgID:        orgID,
		BillingEmail: "old@example.com",
	})
	require.NoError(t, err)

	stripeID := "cus_test_updated"
	newName := "Updated HOA Name"
	created.BillingEmail = "new@example.com"
	created.StripeCustomerID = &stripeID
	created.BillingName = &newName

	updated, err := repo.UpdateAccount(ctx, created)

	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, created.ID, updated.ID)
	assert.Equal(t, "new@example.com", updated.BillingEmail)
	require.NotNil(t, updated.StripeCustomerID)
	assert.Equal(t, stripeID, *updated.StripeCustomerID)
	require.NotNil(t, updated.BillingName)
	assert.Equal(t, newName, *updated.BillingName)
	assert.True(t, updated.UpdatedAt.After(created.UpdatedAt) || updated.UpdatedAt.Equal(created.UpdatedAt))
}

// ─── Invoice tests ────────────────────────────────────────────────────────────

func TestCreateInvoice_StoresFindsByIDAndListsByOrg(t *testing.T) {
	pool := setupBillingTestDB(t)
	repo := billing.NewPostgresBillingRepository(pool)
	ctx := context.Background()

	orgID := createBillingTestOrg(t, pool, "hoa", "Invoice HOA")
	account, err := repo.CreateAccount(ctx, &billing.BillingAccount{
		OrgID:        orgID,
		BillingEmail: "billing@invoice-test.com",
	})
	require.NoError(t, err)

	now := time.Now().UTC().Truncate(time.Second)
	input := &billing.Invoice{
		BillingAccountID: account.ID,
		OrgID:            orgID,
		Status:           "draft",
		SubtotalCents:    10000,
		TaxCents:         1000,
		TotalCents:       11000,
		PeriodStart:      now.Add(-30 * 24 * time.Hour),
		PeriodEnd:        now,
	}

	created, err := repo.CreateInvoice(ctx, input)

	require.NoError(t, err)
	require.NotNil(t, created)
	assert.NotEqual(t, uuid.Nil, created.ID)
	assert.Equal(t, account.ID, created.BillingAccountID)
	assert.Equal(t, orgID, created.OrgID)
	assert.Equal(t, "draft", created.Status)
	assert.Equal(t, int64(10000), created.SubtotalCents)
	assert.Equal(t, int64(1000), created.TaxCents)
	assert.Equal(t, int64(11000), created.TotalCents)
	assert.Nil(t, created.StripeInvoiceID)
	assert.Nil(t, created.DueDate)
	assert.Nil(t, created.PaidAt)

	// FindInvoiceByID should return the same invoice.
	found, err := repo.FindInvoiceByID(ctx, created.ID)

	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, created.ID, found.ID)
	assert.Equal(t, created.Status, found.Status)

	// ListInvoicesByOrg should include the created invoice.
	list, err := repo.ListInvoicesByOrg(ctx, orgID)

	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(list), 1)
	ids := make([]uuid.UUID, len(list))
	for i, inv := range list {
		ids[i] = inv.ID
	}
	assert.Contains(t, ids, created.ID)
}

func TestFindInvoiceByID_ReturnsNilWhenNotFound(t *testing.T) {
	pool := setupBillingTestDB(t)
	repo := billing.NewPostgresBillingRepository(pool)
	ctx := context.Background()

	found, err := repo.FindInvoiceByID(ctx, uuid.New())

	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestUpdateInvoice_UpdatesStatusAndOptionalFields(t *testing.T) {
	pool := setupBillingTestDB(t)
	repo := billing.NewPostgresBillingRepository(pool)
	ctx := context.Background()

	orgID := createBillingTestOrg(t, pool, "hoa", "Update Invoice HOA")
	account, err := repo.CreateAccount(ctx, &billing.BillingAccount{
		OrgID:        orgID,
		BillingEmail: "billing@update-inv-test.com",
	})
	require.NoError(t, err)

	now := time.Now().UTC().Truncate(time.Second)
	created, err := repo.CreateInvoice(ctx, &billing.Invoice{
		BillingAccountID: account.ID,
		OrgID:            orgID,
		Status:           "draft",
		SubtotalCents:    5000,
		TaxCents:         0,
		TotalCents:       5000,
		PeriodStart:      now.Add(-30 * 24 * time.Hour),
		PeriodEnd:        now,
	})
	require.NoError(t, err)

	stripeInvID := "in_test_stripe"
	paidAt := now
	created.Status = "paid"
	created.StripeInvoiceID = &stripeInvID
	created.PaidAt = &paidAt

	updated, err := repo.UpdateInvoice(ctx, created)

	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, "paid", updated.Status)
	require.NotNil(t, updated.StripeInvoiceID)
	assert.Equal(t, stripeInvID, *updated.StripeInvoiceID)
	require.NotNil(t, updated.PaidAt)
}

// ─── InvoiceLineItem tests ────────────────────────────────────────────────────

func TestCreateLineItem_StoresAndListsByInvoice(t *testing.T) {
	pool := setupBillingTestDB(t)
	repo := billing.NewPostgresBillingRepository(pool)
	ctx := context.Background()

	orgID := createBillingTestOrg(t, pool, "hoa", "LineItem HOA")
	account, err := repo.CreateAccount(ctx, &billing.BillingAccount{
		OrgID:        orgID,
		BillingEmail: "billing@lineitem-test.com",
	})
	require.NoError(t, err)

	now := time.Now().UTC().Truncate(time.Second)
	invoice, err := repo.CreateInvoice(ctx, &billing.Invoice{
		BillingAccountID: account.ID,
		OrgID:            orgID,
		Status:           "draft",
		PeriodStart:      now.Add(-30 * 24 * time.Hour),
		PeriodEnd:        now,
	})
	require.NoError(t, err)

	item1 := &billing.InvoiceLineItem{
		InvoiceID:      invoice.ID,
		Description:    "Starter Plan - Monthly",
		Quantity:       1,
		UnitPriceCents: 4999,
		TotalCents:     4999,
		LineType:       "subscription",
		Metadata:       map[string]any{"plan": "starter"},
	}
	item2 := &billing.InvoiceLineItem{
		InvoiceID:      invoice.ID,
		Description:    "API Overage - 50 extra calls",
		Quantity:       50,
		UnitPriceCents: 10,
		TotalCents:     500,
		LineType:       "overage",
		Metadata:       map[string]any{},
	}

	created1, err := repo.CreateLineItem(ctx, item1)

	require.NoError(t, err)
	require.NotNil(t, created1)
	assert.NotEqual(t, uuid.Nil, created1.ID)
	assert.Equal(t, invoice.ID, created1.InvoiceID)
	assert.Equal(t, "Starter Plan - Monthly", created1.Description)
	assert.Equal(t, 1, created1.Quantity)
	assert.Equal(t, int64(4999), created1.UnitPriceCents)
	assert.Equal(t, int64(4999), created1.TotalCents)
	assert.Equal(t, "subscription", created1.LineType)
	assert.Equal(t, "starter", created1.Metadata["plan"])

	created2, err := repo.CreateLineItem(ctx, item2)
	require.NoError(t, err)
	require.NotNil(t, created2)

	// ListLineItemsByInvoice should return both items.
	items, err := repo.ListLineItemsByInvoice(ctx, invoice.ID)

	require.NoError(t, err)
	assert.Len(t, items, 2)
	itemIDs := []uuid.UUID{items[0].ID, items[1].ID}
	assert.Contains(t, itemIDs, created1.ID)
	assert.Contains(t, itemIDs, created2.ID)
}

func TestListLineItemsByInvoice_ReturnsEmptySliceWhenNone(t *testing.T) {
	pool := setupBillingTestDB(t)
	repo := billing.NewPostgresBillingRepository(pool)
	ctx := context.Background()

	items, err := repo.ListLineItemsByInvoice(ctx, uuid.New())

	require.NoError(t, err)
	assert.Empty(t, items)
}
