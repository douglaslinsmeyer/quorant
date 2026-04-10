//go:build integration

package fin_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/quorant/quorant/internal/fin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Fixture ─────────────────────────────────────────────────────────────────

// paymentTestFixture holds all resources needed by payment repository tests.
type paymentTestFixture struct {
	repo   fin.PaymentRepository
	orgID  uuid.UUID
	unitID uuid.UUID
	userID uuid.UUID
	pool   *pgxpool.Pool
}

// setupPaymentFixture reuses the shared DB pool helper and creates a fresh org,
// user, and unit for payment tests.
func setupPaymentFixture(t *testing.T) paymentTestFixture {
	t.Helper()
	ctx := context.Background()
	pool := setupFinDB(t)

	// Create a test organization.
	var orgID uuid.UUID
	err := pool.QueryRow(ctx,
		`INSERT INTO organizations (type, name, slug, path, settings)
		 VALUES ('hoa', 'Payment Test HOA', $1, $2, '{}')
		 RETURNING id`,
		"payment-test-hoa-"+uuid.New().String(),
		"payment_test_hoa_"+uuid.New().String()[:8],
	).Scan(&orgID)
	require.NoError(t, err, "create test org")

	// Create a test user.
	var userID uuid.UUID
	err = pool.QueryRow(ctx,
		`INSERT INTO users (idp_user_id, email, display_name)
		 VALUES ($1, $2, $3)
		 RETURNING id`,
		"payment-test-idp-"+uuid.New().String(),
		"payment-test-"+uuid.New().String()+"@example.com",
		"Payment Test User",
	).Scan(&userID)
	require.NoError(t, err, "create test user")

	// Create a test unit.
	var unitID uuid.UUID
	err = pool.QueryRow(ctx,
		`INSERT INTO units (org_id, label, status)
		 VALUES ($1, 'Unit 201', 'occupied')
		 RETURNING id`,
		orgID,
	).Scan(&unitID)
	require.NoError(t, err, "create test unit")

	repo := fin.NewPostgresPaymentRepository(pool)

	return paymentTestFixture{
		repo:   repo,
		orgID:  orgID,
		unitID: unitID,
		userID: userID,
		pool:   pool,
	}
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func minimalPayment(orgID, unitID, userID uuid.UUID) *fin.Payment {
	return &fin.Payment{
		OrgID:       orgID,
		UnitID:      unitID,
		UserID:      userID,
		AmountCents: 15000,
		Status:      fin.PaymentStatusPending,
	}
}

func minimalPaymentMethod(orgID, userID uuid.UUID) *fin.PaymentMethod {
	lastFour := "4242"
	return &fin.PaymentMethod{
		OrgID:      orgID,
		UserID:     userID,
		MethodType: fin.PaymentMethodTypeCard,
		LastFour:   &lastFour,
		IsDefault:  false,
	}
}

// ─── TestCreatePayment ────────────────────────────────────────────────────────

func TestCreatePayment(t *testing.T) {
	f := setupPaymentFixture(t)
	ctx := context.Background()

	input := minimalPayment(f.orgID, f.unitID, f.userID)
	got, err := f.repo.CreatePayment(ctx, input)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.NotEqual(t, uuid.Nil, got.ID, "should have a generated UUID")
	assert.Equal(t, f.orgID, got.OrgID)
	assert.Equal(t, f.unitID, got.UnitID)
	assert.Equal(t, f.userID, got.UserID)
	assert.Equal(t, int64(15000), got.AmountCents)
	assert.Equal(t, fin.PaymentStatusPending, got.Status)
	assert.Nil(t, got.PaidAt)
	assert.False(t, got.CreatedAt.IsZero())
	assert.False(t, got.UpdatedAt.IsZero())
}

// ─── TestFindPaymentByID ──────────────────────────────────────────────────────

func TestFindPaymentByID(t *testing.T) {
	f := setupPaymentFixture(t)
	ctx := context.Background()

	created, err := f.repo.CreatePayment(ctx, minimalPayment(f.orgID, f.unitID, f.userID))
	require.NoError(t, err)

	found, err := f.repo.FindPaymentByID(ctx, created.ID)

	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, created.ID, found.ID)
	assert.Equal(t, fin.PaymentStatusPending, found.Status)
}

func TestFindPaymentByID_NilWhenNotFound(t *testing.T) {
	f := setupPaymentFixture(t)
	ctx := context.Background()

	found, err := f.repo.FindPaymentByID(ctx, uuid.New())

	require.NoError(t, err)
	assert.Nil(t, found)
}

// ─── TestListPaymentsByOrg ────────────────────────────────────────────────────

func TestListPaymentsByOrg(t *testing.T) {
	f := setupPaymentFixture(t)
	ctx := context.Background()

	p1 := minimalPayment(f.orgID, f.unitID, f.userID)
	p1.AmountCents = 10000
	_, err := f.repo.CreatePayment(ctx, p1)
	require.NoError(t, err)

	p2 := minimalPayment(f.orgID, f.unitID, f.userID)
	p2.AmountCents = 20000
	_, err = f.repo.CreatePayment(ctx, p2)
	require.NoError(t, err)

	list, err := f.repo.ListPaymentsByOrg(ctx, f.orgID)

	require.NoError(t, err)
	require.Len(t, list, 2, "should return both payments for the org")
	for _, p := range list {
		assert.Equal(t, f.orgID, p.OrgID)
	}
}

func TestListPaymentsByOrg_EmptySliceWhenNone(t *testing.T) {
	f := setupPaymentFixture(t)
	ctx := context.Background()

	list, err := f.repo.ListPaymentsByOrg(ctx, f.orgID)

	require.NoError(t, err)
	assert.NotNil(t, list, "should return empty slice, not nil")
	assert.Empty(t, list)
}

// ─── TestListPaymentsByUnit ───────────────────────────────────────────────────

func TestListPaymentsByUnit(t *testing.T) {
	f := setupPaymentFixture(t)
	ctx := context.Background()

	// Create a second unit in the same org.
	var otherUnitID uuid.UUID
	err := f.pool.QueryRow(ctx,
		`INSERT INTO units (org_id, label, status) VALUES ($1, $2, 'occupied') RETURNING id`,
		f.orgID,
		"Unit-other-"+uuid.New().String()[:8],
	).Scan(&otherUnitID)
	require.NoError(t, err)

	// Payment for primary unit.
	_, err = f.repo.CreatePayment(ctx, minimalPayment(f.orgID, f.unitID, f.userID))
	require.NoError(t, err)

	// Payment for other unit.
	other := minimalPayment(f.orgID, otherUnitID, f.userID)
	_, err = f.repo.CreatePayment(ctx, other)
	require.NoError(t, err)

	list, err := f.repo.ListPaymentsByUnit(ctx, f.unitID)

	require.NoError(t, err)
	require.Len(t, list, 1, "should return only payments for the requested unit")
	assert.Equal(t, f.unitID, list[0].UnitID)
}

func TestListPaymentsByUnit_EmptySliceWhenNone(t *testing.T) {
	f := setupPaymentFixture(t)
	ctx := context.Background()

	list, err := f.repo.ListPaymentsByUnit(ctx, f.unitID)

	require.NoError(t, err)
	assert.NotNil(t, list, "should return empty slice, not nil")
	assert.Empty(t, list)
}

// ─── TestUpdatePaymentStatus ──────────────────────────────────────────────────

func TestUpdatePaymentStatus(t *testing.T) {
	f := setupPaymentFixture(t)
	ctx := context.Background()

	created, err := f.repo.CreatePayment(ctx, minimalPayment(f.orgID, f.unitID, f.userID))
	require.NoError(t, err)
	require.Equal(t, fin.PaymentStatusPending, created.Status)

	paidAt := time.Now().UTC().Truncate(time.Millisecond)
	err = f.repo.UpdatePaymentStatus(ctx, created.ID, fin.PaymentStatusCompleted, &paidAt)
	require.NoError(t, err)

	updated, err := f.repo.FindPaymentByID(ctx, created.ID)
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, fin.PaymentStatusCompleted, updated.Status)
	require.NotNil(t, updated.PaidAt)
	assert.WithinDuration(t, paidAt, *updated.PaidAt, time.Second)
	assert.True(t, updated.UpdatedAt.After(created.UpdatedAt) || updated.UpdatedAt.Equal(created.UpdatedAt),
		"updated_at should be updated")
}

func TestUpdatePaymentStatus_NilPaidAt(t *testing.T) {
	f := setupPaymentFixture(t)
	ctx := context.Background()

	created, err := f.repo.CreatePayment(ctx, minimalPayment(f.orgID, f.unitID, f.userID))
	require.NoError(t, err)

	err = f.repo.UpdatePaymentStatus(ctx, created.ID, fin.PaymentStatusFailed, nil)
	require.NoError(t, err)

	updated, err := f.repo.FindPaymentByID(ctx, created.ID)
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, fin.PaymentStatusFailed, updated.Status)
	assert.Nil(t, updated.PaidAt)
}

// ─── TestCreatePaymentMethod ──────────────────────────────────────────────────

func TestCreatePaymentMethod(t *testing.T) {
	f := setupPaymentFixture(t)
	ctx := context.Background()

	input := minimalPaymentMethod(f.orgID, f.userID)
	got, err := f.repo.CreatePaymentMethod(ctx, input)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.NotEqual(t, uuid.Nil, got.ID, "should have a generated UUID")
	assert.Equal(t, f.orgID, got.OrgID)
	assert.Equal(t, f.userID, got.UserID)
	assert.Equal(t, fin.PaymentMethodTypeCard, got.MethodType)
	require.NotNil(t, got.LastFour)
	assert.Equal(t, "4242", *got.LastFour)
	assert.False(t, got.IsDefault)
	assert.False(t, got.CreatedAt.IsZero())
	assert.Nil(t, got.DeletedAt)
}

// ─── TestListPaymentMethodsByUser ─────────────────────────────────────────────

func TestListPaymentMethodsByUser(t *testing.T) {
	f := setupPaymentFixture(t)
	ctx := context.Background()

	m1 := minimalPaymentMethod(f.orgID, f.userID)
	m1.MethodType = fin.PaymentMethodTypeCard
	_, err := f.repo.CreatePaymentMethod(ctx, m1)
	require.NoError(t, err)

	m2 := minimalPaymentMethod(f.orgID, f.userID)
	m2.MethodType = fin.PaymentMethodTypeACH
	_, err = f.repo.CreatePaymentMethod(ctx, m2)
	require.NoError(t, err)

	list, err := f.repo.ListPaymentMethodsByUser(ctx, f.userID)

	require.NoError(t, err)
	require.Len(t, list, 2, "should return both methods for the user")
	for _, m := range list {
		assert.Equal(t, f.userID, m.UserID)
		assert.Nil(t, m.DeletedAt)
	}
}

func TestListPaymentMethodsByUser_EmptySliceWhenNone(t *testing.T) {
	f := setupPaymentFixture(t)
	ctx := context.Background()

	list, err := f.repo.ListPaymentMethodsByUser(ctx, f.userID)

	require.NoError(t, err)
	assert.NotNil(t, list, "should return empty slice, not nil")
	assert.Empty(t, list)
}

// ─── TestSoftDeletePaymentMethod ─────────────────────────────────────────────

func TestSoftDeletePaymentMethod(t *testing.T) {
	f := setupPaymentFixture(t)
	ctx := context.Background()

	created, err := f.repo.CreatePaymentMethod(ctx, minimalPaymentMethod(f.orgID, f.userID))
	require.NoError(t, err)
	require.Nil(t, created.DeletedAt)

	// Should appear in list before deletion.
	list, err := f.repo.ListPaymentMethodsByUser(ctx, f.userID)
	require.NoError(t, err)
	require.Len(t, list, 1)

	// Soft-delete the method.
	err = f.repo.SoftDeletePaymentMethod(ctx, created.ID)
	require.NoError(t, err)

	// Should not appear in list after deletion.
	list, err = f.repo.ListPaymentMethodsByUser(ctx, f.userID)
	require.NoError(t, err)
	assert.Empty(t, list, "deleted payment method should not appear in list")
}
