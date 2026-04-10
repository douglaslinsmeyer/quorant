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

// collectionTestFixture holds all resources needed by collection repository
// tests.
type collectionTestFixture struct {
	repo   fin.CollectionRepository
	orgID  uuid.UUID
	unitID uuid.UUID
	userID uuid.UUID
	pool   *pgxpool.Pool
}

// setupCollectionFixture creates a pool (reusing setupFinDB for DB connection
// and cleanup), a test org, user, and unit for collection tests.
func setupCollectionFixture(t *testing.T) collectionTestFixture {
	t.Helper()
	ctx := context.Background()
	pool := setupFinDB(t)

	var orgID uuid.UUID
	err := pool.QueryRow(ctx,
		`INSERT INTO organizations (type, name, slug, path, settings)
		 VALUES ('hoa', 'Collection Test HOA', $1, $2, '{}')
		 RETURNING id`,
		"collection-test-hoa-"+uuid.New().String(),
		"collection_test_hoa_"+uuid.New().String()[:8],
	).Scan(&orgID)
	require.NoError(t, err, "create test org")

	var userID uuid.UUID
	err = pool.QueryRow(ctx,
		`INSERT INTO users (idp_user_id, email, display_name)
		 VALUES ($1, $2, 'Collection Test User')
		 RETURNING id`,
		"collection-test-idp-"+uuid.New().String(),
		"collection-test-"+uuid.New().String()+"@example.com",
	).Scan(&userID)
	require.NoError(t, err, "create test user")

	var unitID uuid.UUID
	err = pool.QueryRow(ctx,
		`INSERT INTO units (org_id, label, status)
		 VALUES ($1, 'Unit 301', 'occupied')
		 RETURNING id`,
		orgID,
	).Scan(&unitID)
	require.NoError(t, err, "create test unit")

	repo := fin.NewPostgresCollectionRepository(pool)

	return collectionTestFixture{
		repo:   repo,
		orgID:  orgID,
		unitID: unitID,
		userID: userID,
		pool:   pool,
	}
}

// createExtraCollectionUnit inserts an additional unit in the fixture org.
func (f collectionTestFixture) createExtraCollectionUnit(t *testing.T) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var unitID uuid.UUID
	err := f.pool.QueryRow(ctx,
		`INSERT INTO units (org_id, label, status)
		 VALUES ($1, $2, 'occupied')
		 RETURNING id`,
		f.orgID,
		"Extra Unit "+uuid.New().String()[:8],
	).Scan(&unitID)
	require.NoError(t, err, "create extra unit")
	return unitID
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func minimalCollectionCase(orgID, unitID uuid.UUID) *fin.CollectionCase {
	return &fin.CollectionCase{
		OrgID:            orgID,
		UnitID:           unitID,
		Status:           fin.CollectionCaseStatusLate,
		TotalOwedCents:   150000,
		CurrentOwedCents: 150000,
		OpenedAt:         time.Now().UTC(),
		Metadata:         map[string]any{},
	}
}

func minimalCollectionAction(caseID uuid.UUID) *fin.CollectionAction {
	triggeredBy := fin.TriggeredBySystem
	return &fin.CollectionAction{
		CaseID:      caseID,
		ActionType:  fin.CollectionActionTypeNoticeSent,
		TriggeredBy: &triggeredBy,
		Metadata:    map[string]any{},
	}
}

func minimalPaymentPlan(caseID, orgID, unitID uuid.UUID) *fin.PaymentPlan {
	return &fin.PaymentPlan{
		CaseID:            caseID,
		OrgID:             orgID,
		UnitID:            unitID,
		TotalOwedCents:    150000,
		InstallmentCents:  25000,
		Frequency:         fin.PaymentPlanFreqMonthly,
		InstallmentsTotal: 6,
		InstallmentsPaid:  0,
		NextDueDate:       time.Now().UTC().AddDate(0, 1, 0).Truncate(24 * time.Hour),
		Status:            fin.PaymentPlanStatusActive,
	}
}

// ─── TestCreateCase + FindCaseByID + ListCasesByOrg ──────────────────────────

func TestCreateCase(t *testing.T) {
	f := setupCollectionFixture(t)
	ctx := context.Background()

	input := minimalCollectionCase(f.orgID, f.unitID)
	got, err := f.repo.CreateCase(ctx, input)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.NotEqual(t, uuid.Nil, got.ID, "should have a generated UUID")
	assert.Equal(t, f.orgID, got.OrgID)
	assert.Equal(t, f.unitID, got.UnitID)
	assert.Equal(t, fin.CollectionCaseStatusLate, got.Status)
	assert.Equal(t, int64(150000), got.TotalOwedCents)
	assert.Equal(t, int64(150000), got.CurrentOwedCents)
	assert.False(t, got.EscalationPaused)
	assert.Nil(t, got.ClosedAt)
	assert.NotNil(t, got.Metadata)
	assert.False(t, got.CreatedAt.IsZero())
	assert.False(t, got.UpdatedAt.IsZero())
}

func TestFindCaseByID(t *testing.T) {
	f := setupCollectionFixture(t)
	ctx := context.Background()

	created, err := f.repo.CreateCase(ctx, minimalCollectionCase(f.orgID, f.unitID))
	require.NoError(t, err)

	found, err := f.repo.FindCaseByID(ctx, created.ID)

	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, created.ID, found.ID)
	assert.Equal(t, created.UnitID, found.UnitID)
	assert.Equal(t, created.Status, found.Status)
}

func TestFindCaseByID_ReturnsNilWhenNotFound(t *testing.T) {
	f := setupCollectionFixture(t)
	ctx := context.Background()

	found, err := f.repo.FindCaseByID(ctx, uuid.New())

	require.NoError(t, err)
	assert.Nil(t, found, "should return nil for non-existent case")
}

func TestListCasesByOrg(t *testing.T) {
	f := setupCollectionFixture(t)
	ctx := context.Background()

	unit2 := f.createExtraCollectionUnit(t)

	case1 := minimalCollectionCase(f.orgID, f.unitID)
	case1.TotalOwedCents = 100000
	_, err := f.repo.CreateCase(ctx, case1)
	require.NoError(t, err)

	case2 := minimalCollectionCase(f.orgID, unit2)
	case2.TotalOwedCents = 200000
	_, err = f.repo.CreateCase(ctx, case2)
	require.NoError(t, err)

	list, err := f.repo.ListCasesByOrg(ctx, f.orgID)

	require.NoError(t, err)
	require.Len(t, list, 2, "should return both cases")

	amounts := []int64{list[0].TotalOwedCents, list[1].TotalOwedCents}
	assert.Contains(t, amounts, int64(100000))
	assert.Contains(t, amounts, int64(200000))
}

func TestListCasesByOrg_EmptySliceWhenNone(t *testing.T) {
	f := setupCollectionFixture(t)
	ctx := context.Background()

	list, err := f.repo.ListCasesByOrg(ctx, f.orgID)

	require.NoError(t, err)
	assert.NotNil(t, list, "should return empty slice, not nil")
	assert.Empty(t, list)
}

// ─── TestCreateAction + ListActionsByCase ─────────────────────────────────────

func TestCreateAction(t *testing.T) {
	f := setupCollectionFixture(t)
	ctx := context.Background()

	coll, err := f.repo.CreateCase(ctx, minimalCollectionCase(f.orgID, f.unitID))
	require.NoError(t, err)

	input := minimalCollectionAction(coll.ID)
	got, err := f.repo.CreateAction(ctx, input)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.NotEqual(t, uuid.Nil, got.ID)
	assert.Equal(t, coll.ID, got.CaseID)
	assert.Equal(t, fin.CollectionActionTypeNoticeSent, got.ActionType)
	assert.NotNil(t, got.TriggeredBy)
	assert.Equal(t, fin.TriggeredBySystem, *got.TriggeredBy)
	assert.NotNil(t, got.Metadata)
	assert.False(t, got.CreatedAt.IsZero())
}

func TestListActionsByCase(t *testing.T) {
	f := setupCollectionFixture(t)
	ctx := context.Background()

	coll, err := f.repo.CreateCase(ctx, minimalCollectionCase(f.orgID, f.unitID))
	require.NoError(t, err)

	a1 := minimalCollectionAction(coll.ID)
	a1.ActionType = fin.CollectionActionTypeNoticeSent
	_, err = f.repo.CreateAction(ctx, a1)
	require.NoError(t, err)

	a2 := minimalCollectionAction(coll.ID)
	a2.ActionType = fin.CollectionActionTypeLienFiled
	_, err = f.repo.CreateAction(ctx, a2)
	require.NoError(t, err)

	list, err := f.repo.ListActionsByCase(ctx, coll.ID)

	require.NoError(t, err)
	require.Len(t, list, 2, "should return both actions")

	types := []fin.CollectionActionType{list[0].ActionType, list[1].ActionType}
	assert.Contains(t, types, fin.CollectionActionTypeNoticeSent)
	assert.Contains(t, types, fin.CollectionActionTypeLienFiled)
}

func TestListActionsByCase_EmptySliceWhenNone(t *testing.T) {
	f := setupCollectionFixture(t)
	ctx := context.Background()

	coll, err := f.repo.CreateCase(ctx, minimalCollectionCase(f.orgID, f.unitID))
	require.NoError(t, err)

	list, err := f.repo.ListActionsByCase(ctx, coll.ID)

	require.NoError(t, err)
	assert.NotNil(t, list, "should return empty slice, not nil")
	assert.Empty(t, list)
}

// ─── TestCreatePaymentPlan + ListPaymentPlansByCase ───────────────────────────

func TestCreatePaymentPlan(t *testing.T) {
	f := setupCollectionFixture(t)
	ctx := context.Background()

	coll, err := f.repo.CreateCase(ctx, minimalCollectionCase(f.orgID, f.unitID))
	require.NoError(t, err)

	input := minimalPaymentPlan(coll.ID, f.orgID, f.unitID)
	got, err := f.repo.CreatePaymentPlan(ctx, input)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.NotEqual(t, uuid.Nil, got.ID)
	assert.Equal(t, coll.ID, got.CaseID)
	assert.Equal(t, f.orgID, got.OrgID)
	assert.Equal(t, f.unitID, got.UnitID)
	assert.Equal(t, int64(150000), got.TotalOwedCents)
	assert.Equal(t, int64(25000), got.InstallmentCents)
	assert.Equal(t, fin.PaymentPlanFreqMonthly, got.Frequency)
	assert.Equal(t, 6, got.InstallmentsTotal)
	assert.Equal(t, 0, got.InstallmentsPaid)
	assert.Equal(t, fin.PaymentPlanStatusActive, got.Status)
	assert.Nil(t, got.ApprovedBy)
	assert.False(t, got.CreatedAt.IsZero())
	assert.False(t, got.UpdatedAt.IsZero())
}

func TestListPaymentPlansByCase(t *testing.T) {
	f := setupCollectionFixture(t)
	ctx := context.Background()

	coll, err := f.repo.CreateCase(ctx, minimalCollectionCase(f.orgID, f.unitID))
	require.NoError(t, err)

	p1 := minimalPaymentPlan(coll.ID, f.orgID, f.unitID)
	p1.TotalOwedCents = 100000
	_, err = f.repo.CreatePaymentPlan(ctx, p1)
	require.NoError(t, err)

	p2 := minimalPaymentPlan(coll.ID, f.orgID, f.unitID)
	p2.TotalOwedCents = 50000
	p2.Status = fin.PaymentPlanStatusDefaulted
	_, err = f.repo.CreatePaymentPlan(ctx, p2)
	require.NoError(t, err)

	list, err := f.repo.ListPaymentPlansByCase(ctx, coll.ID)

	require.NoError(t, err)
	require.Len(t, list, 2, "should return both payment plans")

	amounts := []int64{list[0].TotalOwedCents, list[1].TotalOwedCents}
	assert.Contains(t, amounts, int64(100000))
	assert.Contains(t, amounts, int64(50000))
}

func TestListPaymentPlansByCase_EmptySliceWhenNone(t *testing.T) {
	f := setupCollectionFixture(t)
	ctx := context.Background()

	coll, err := f.repo.CreateCase(ctx, minimalCollectionCase(f.orgID, f.unitID))
	require.NoError(t, err)

	list, err := f.repo.ListPaymentPlansByCase(ctx, coll.ID)

	require.NoError(t, err)
	assert.NotNil(t, list, "should return empty slice, not nil")
	assert.Empty(t, list)
}

// ─── TestGetCollectionStatusForUnit ──────────────────────────────────────────

func TestGetCollectionStatusForUnit_ReturnsActiveCase(t *testing.T) {
	f := setupCollectionFixture(t)
	ctx := context.Background()

	created, err := f.repo.CreateCase(ctx, minimalCollectionCase(f.orgID, f.unitID))
	require.NoError(t, err)

	found, err := f.repo.GetCollectionStatusForUnit(ctx, f.unitID)

	require.NoError(t, err)
	require.NotNil(t, found, "should return the active case")
	assert.Equal(t, created.ID, found.ID)
	assert.Nil(t, found.ClosedAt, "active case should not be closed")
}

func TestGetCollectionStatusForUnit_ReturnsNilWhenNoActiveCase(t *testing.T) {
	f := setupCollectionFixture(t)
	ctx := context.Background()

	found, err := f.repo.GetCollectionStatusForUnit(ctx, f.unitID)

	require.NoError(t, err)
	assert.Nil(t, found, "should return nil when no active case exists")
}

func TestGetCollectionStatusForUnit_ReturnsNilAfterCaseClosed(t *testing.T) {
	f := setupCollectionFixture(t)
	ctx := context.Background()

	coll, err := f.repo.CreateCase(ctx, minimalCollectionCase(f.orgID, f.unitID))
	require.NoError(t, err)

	// Close the case.
	now := time.Now().UTC()
	closedReason := "paid in full"
	coll.ClosedAt = &now
	coll.ClosedReason = &closedReason
	coll.Status = fin.CollectionCaseStatusResolved
	_, err = f.repo.UpdateCase(ctx, coll)
	require.NoError(t, err)

	found, err := f.repo.GetCollectionStatusForUnit(ctx, f.unitID)

	require.NoError(t, err)
	assert.Nil(t, found, "closed case should not be returned as active")
}
