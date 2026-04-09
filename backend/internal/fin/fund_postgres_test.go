//go:build integration

package fin_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/fin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Fixture ─────────────────────────────────────────────────────────────────

// fundTestFixture holds all resources needed by fund repository tests.
type fundTestFixture struct {
	repo   fin.FundRepository
	orgID  uuid.UUID
	userID uuid.UUID
}

// setupFundFixture creates a pool (reusing setupFinDB for DB connection and
// cleanup), a test org, and a test user.
func setupFundFixture(t *testing.T) fundTestFixture {
	t.Helper()
	ctx := context.Background()
	pool := setupFinDB(t)

	var orgID uuid.UUID
	err := pool.QueryRow(ctx,
		`INSERT INTO organizations (type, name, slug, path, settings)
		 VALUES ('hoa', 'Fund Test HOA', $1, $2, '{}')
		 RETURNING id`,
		"fund-test-hoa-"+uuid.New().String(),
		"fund_test_hoa_"+uuid.New().String()[:8],
	).Scan(&orgID)
	require.NoError(t, err, "create test org")

	var userID uuid.UUID
	err = pool.QueryRow(ctx,
		`INSERT INTO users (idp_user_id, email, display_name)
		 VALUES ($1, $2, 'Fund Test User')
		 RETURNING id`,
		"fund-test-idp-"+uuid.New().String(),
		"fund-test-"+uuid.New().String()+"@example.com",
	).Scan(&userID)
	require.NoError(t, err, "create test user")

	repo := fin.NewPostgresFundRepository(pool)

	return fundTestFixture{
		repo:   repo,
		orgID:  orgID,
		userID: userID,
	}
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func minimalFund(orgID uuid.UUID) *fin.Fund {
	return &fin.Fund{
		OrgID:        orgID,
		Name:         "Operating Fund",
		FundType:     "operating",
		BalanceCents: 0,
		IsDefault:    false,
	}
}

func minimalFundTransaction(fundID, orgID uuid.UUID) *fin.FundTransaction {
	return &fin.FundTransaction{
		FundID:          fundID,
		OrgID:           orgID,
		TransactionType: "credit",
		AmountCents:     50000,
		EffectiveDate:   time.Now().UTC().Truncate(24 * time.Hour),
	}
}

func minimalFundTransfer(orgID, fromFundID, toFundID uuid.UUID) *fin.FundTransfer {
	return &fin.FundTransfer{
		OrgID:         orgID,
		FromFundID:    fromFundID,
		ToFundID:      toFundID,
		AmountCents:   10000,
		EffectiveDate: time.Now().UTC().Truncate(24 * time.Hour),
	}
}

// ─── TestCreateFund + FindFundByID + ListFundsByOrg ──────────────────────────

func TestCreateFund(t *testing.T) {
	f := setupFundFixture(t)
	ctx := context.Background()

	input := minimalFund(f.orgID)
	got, err := f.repo.CreateFund(ctx, input)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.NotEqual(t, uuid.Nil, got.ID, "should have a generated UUID")
	assert.Equal(t, f.orgID, got.OrgID)
	assert.Equal(t, "Operating Fund", got.Name)
	assert.Equal(t, "operating", got.FundType)
	assert.Equal(t, int64(0), got.BalanceCents)
	assert.False(t, got.IsDefault)
	assert.Nil(t, got.DeletedAt)
	assert.False(t, got.CreatedAt.IsZero())
	assert.False(t, got.UpdatedAt.IsZero())
}

func TestFindFundByID(t *testing.T) {
	f := setupFundFixture(t)
	ctx := context.Background()

	created, err := f.repo.CreateFund(ctx, minimalFund(f.orgID))
	require.NoError(t, err)

	found, err := f.repo.FindFundByID(ctx, created.ID)

	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, created.ID, found.ID)
	assert.Equal(t, created.Name, found.Name)
	assert.Equal(t, created.FundType, found.FundType)
}

func TestFindFundByID_ReturnsNilWhenNotFound(t *testing.T) {
	f := setupFundFixture(t)
	ctx := context.Background()

	found, err := f.repo.FindFundByID(ctx, uuid.New())

	require.NoError(t, err)
	assert.Nil(t, found, "should return nil for non-existent fund")
}

func TestListFundsByOrg(t *testing.T) {
	f := setupFundFixture(t)
	ctx := context.Background()

	fund1 := minimalFund(f.orgID)
	fund1.Name = "Operating Fund"
	fund1.FundType = "operating"
	_, err := f.repo.CreateFund(ctx, fund1)
	require.NoError(t, err)

	fund2 := minimalFund(f.orgID)
	fund2.Name = "Reserve Fund"
	fund2.FundType = "reserve"
	_, err = f.repo.CreateFund(ctx, fund2)
	require.NoError(t, err)

	list, err := f.repo.ListFundsByOrg(ctx, f.orgID)

	require.NoError(t, err)
	require.Len(t, list, 2, "should return both funds")

	names := []string{list[0].Name, list[1].Name}
	assert.Contains(t, names, "Operating Fund")
	assert.Contains(t, names, "Reserve Fund")
}

func TestListFundsByOrg_EmptySliceWhenNone(t *testing.T) {
	f := setupFundFixture(t)
	ctx := context.Background()

	list, err := f.repo.ListFundsByOrg(ctx, f.orgID)

	require.NoError(t, err)
	assert.NotNil(t, list, "should return empty slice, not nil")
	assert.Empty(t, list)
}

// ─── TestCreateTransaction ────────────────────────────────────────────────────

func TestCreateTransaction_UpdatesFundBalance(t *testing.T) {
	f := setupFundFixture(t)
	ctx := context.Background()

	fund, err := f.repo.CreateFund(ctx, minimalFund(f.orgID))
	require.NoError(t, err)
	require.Equal(t, int64(0), fund.BalanceCents)

	txn := minimalFundTransaction(fund.ID, f.orgID)
	txn.AmountCents = 75000
	got, err := f.repo.CreateTransaction(ctx, txn)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.NotEqual(t, uuid.Nil, got.ID)
	assert.Equal(t, fund.ID, got.FundID)
	assert.Equal(t, f.orgID, got.OrgID)
	assert.Equal(t, "credit", got.TransactionType)
	assert.Equal(t, int64(75000), got.AmountCents)
	assert.Equal(t, int64(75000), got.BalanceAfterCents, "balance_after_cents should equal new fund balance")
	assert.False(t, got.CreatedAt.IsZero())

	// Verify fund balance was updated.
	updatedFund, err := f.repo.FindFundByID(ctx, fund.ID)
	require.NoError(t, err)
	require.NotNil(t, updatedFund)
	assert.Equal(t, int64(75000), updatedFund.BalanceCents, "fund balance_cents should be updated")
}

func TestCreateTransaction_DebitReducesBalance(t *testing.T) {
	f := setupFundFixture(t)
	ctx := context.Background()

	fund := minimalFund(f.orgID)
	fund.BalanceCents = 100000
	created, err := f.repo.CreateFund(ctx, fund)
	require.NoError(t, err)

	// Record a debit by using a negative amount.
	txn := &fin.FundTransaction{
		FundID:          created.ID,
		OrgID:           f.orgID,
		TransactionType: "debit",
		AmountCents:     -30000,
		EffectiveDate:   time.Now().UTC().Truncate(24 * time.Hour),
	}
	got, err := f.repo.CreateTransaction(ctx, txn)

	require.NoError(t, err)
	assert.Equal(t, int64(70000), got.BalanceAfterCents)

	updatedFund, err := f.repo.FindFundByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(70000), updatedFund.BalanceCents)
}

// ─── TestListTransactionsByFund ───────────────────────────────────────────────

func TestListTransactionsByFund(t *testing.T) {
	f := setupFundFixture(t)
	ctx := context.Background()

	fund, err := f.repo.CreateFund(ctx, minimalFund(f.orgID))
	require.NoError(t, err)

	txn1 := minimalFundTransaction(fund.ID, f.orgID)
	txn1.AmountCents = 50000
	desc1 := "Initial deposit"
	txn1.Description = &desc1
	_, err = f.repo.CreateTransaction(ctx, txn1)
	require.NoError(t, err)

	txn2 := minimalFundTransaction(fund.ID, f.orgID)
	txn2.AmountCents = 25000
	desc2 := "Assessment income"
	txn2.Description = &desc2
	_, err = f.repo.CreateTransaction(ctx, txn2)
	require.NoError(t, err)

	list, err := f.repo.ListTransactionsByFund(ctx, fund.ID)

	require.NoError(t, err)
	require.Len(t, list, 2, "should return both transactions")

	descs := []string{}
	for _, tx := range list {
		if tx.Description != nil {
			descs = append(descs, *tx.Description)
		}
	}
	assert.Contains(t, descs, "Initial deposit")
	assert.Contains(t, descs, "Assessment income")
}

func TestListTransactionsByFund_EmptySliceWhenNone(t *testing.T) {
	f := setupFundFixture(t)
	ctx := context.Background()

	fund, err := f.repo.CreateFund(ctx, minimalFund(f.orgID))
	require.NoError(t, err)

	list, err := f.repo.ListTransactionsByFund(ctx, fund.ID)

	require.NoError(t, err)
	assert.NotNil(t, list, "should return empty slice, not nil")
	assert.Empty(t, list)
}

// ─── TestCreateTransfer + ListTransfersByOrg ─────────────────────────────────

func TestCreateTransfer(t *testing.T) {
	f := setupFundFixture(t)
	ctx := context.Background()

	fromFund := minimalFund(f.orgID)
	fromFund.Name = "Operating Fund"
	createdFrom, err := f.repo.CreateFund(ctx, fromFund)
	require.NoError(t, err)

	toFund := minimalFund(f.orgID)
	toFund.Name = "Reserve Fund"
	toFund.FundType = "reserve"
	createdTo, err := f.repo.CreateFund(ctx, toFund)
	require.NoError(t, err)

	input := minimalFundTransfer(f.orgID, createdFrom.ID, createdTo.ID)
	got, err := f.repo.CreateTransfer(ctx, input)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.NotEqual(t, uuid.Nil, got.ID)
	assert.Equal(t, f.orgID, got.OrgID)
	assert.Equal(t, createdFrom.ID, got.FromFundID)
	assert.Equal(t, createdTo.ID, got.ToFundID)
	assert.Equal(t, int64(10000), got.AmountCents)
	assert.Nil(t, got.ApprovedBy)
	assert.Nil(t, got.ApprovedAt)
	assert.False(t, got.CreatedAt.IsZero())
}

func TestListTransfersByOrg(t *testing.T) {
	f := setupFundFixture(t)
	ctx := context.Background()

	operatingFund := minimalFund(f.orgID)
	operatingFund.Name = "Operating Fund"
	operating, err := f.repo.CreateFund(ctx, operatingFund)
	require.NoError(t, err)

	reserveFund := minimalFund(f.orgID)
	reserveFund.Name = "Reserve Fund"
	reserveFund.FundType = "reserve"
	reserve, err := f.repo.CreateFund(ctx, reserveFund)
	require.NoError(t, err)

	capitalFund := minimalFund(f.orgID)
	capitalFund.Name = "Capital Fund"
	capitalFund.FundType = "capital"
	capital, err := f.repo.CreateFund(ctx, capitalFund)
	require.NoError(t, err)

	t1 := minimalFundTransfer(f.orgID, operating.ID, reserve.ID)
	t1.AmountCents = 5000
	_, err = f.repo.CreateTransfer(ctx, t1)
	require.NoError(t, err)

	t2 := minimalFundTransfer(f.orgID, operating.ID, capital.ID)
	t2.AmountCents = 8000
	_, err = f.repo.CreateTransfer(ctx, t2)
	require.NoError(t, err)

	list, err := f.repo.ListTransfersByOrg(ctx, f.orgID)

	require.NoError(t, err)
	require.Len(t, list, 2, "should return both transfers")

	amounts := []int64{list[0].AmountCents, list[1].AmountCents}
	assert.Contains(t, amounts, int64(5000))
	assert.Contains(t, amounts, int64(8000))
}

func TestListTransfersByOrg_EmptySliceWhenNone(t *testing.T) {
	f := setupFundFixture(t)
	ctx := context.Background()

	list, err := f.repo.ListTransfersByOrg(ctx, f.orgID)

	require.NoError(t, err)
	assert.NotNil(t, list, "should return empty slice, not nil")
	assert.Empty(t, list)
}
