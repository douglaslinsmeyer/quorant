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

// ─── Setup ───────────────────────────────────────────────────────────────────

// finTestFixture holds all resources needed by assessment repository tests.
type finTestFixture struct {
	repo   fin.AssessmentRepository
	orgID  uuid.UUID
	unitID uuid.UUID
	userID uuid.UUID
	pool   *pgxpool.Pool
}

// setupFinDB connects to the local Docker postgres and registers cleanup.
func setupFinDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, "postgres://quorant:quorant@localhost:5432/quorant_dev?sslmode=disable")
	require.NoError(t, err, "connecting to test database")

	t.Cleanup(func() {
		cleanCtx := context.Background()
		pool.Exec(cleanCtx, "DELETE FROM ledger_entries")
		pool.Exec(cleanCtx, "DELETE FROM assessments")
		pool.Exec(cleanCtx, "DELETE FROM assessment_schedules")
		pool.Exec(cleanCtx, "DELETE FROM unit_memberships")
		pool.Exec(cleanCtx, "DELETE FROM units")
		pool.Exec(cleanCtx, "DELETE FROM organizations WHERE parent_id IS NOT NULL")
		pool.Exec(cleanCtx, "DELETE FROM organizations")
		pool.Exec(cleanCtx, "DELETE FROM users")
		pool.Close()
	})

	return pool
}

// setupFinFixture creates a pool, test org, test unit, and test user.
func setupFinFixture(t *testing.T) finTestFixture {
	t.Helper()
	ctx := context.Background()
	pool := setupFinDB(t)

	// Create a test organization.
	var orgID uuid.UUID
	err := pool.QueryRow(ctx,
		`INSERT INTO organizations (type, name, slug, path, settings)
		 VALUES ('hoa', 'Fin Test HOA', $1, $2, '{}')
		 RETURNING id`,
		"fin-test-hoa-"+uuid.New().String(),
		"fin_test_hoa_"+uuid.New().String()[:8],
	).Scan(&orgID)
	require.NoError(t, err, "create test org")

	// Create a test user.
	var userID uuid.UUID
	err = pool.QueryRow(ctx,
		`INSERT INTO users (idp_user_id, email, display_name)
		 VALUES ($1, $2, $3)
		 RETURNING id`,
		"fin-test-idp-"+uuid.New().String(),
		"fin-test-"+uuid.New().String()+"@example.com",
		"Fin Test User",
	).Scan(&userID)
	require.NoError(t, err, "create test user")

	// Create a test unit.
	var unitID uuid.UUID
	err = pool.QueryRow(ctx,
		`INSERT INTO units (org_id, label, status)
		 VALUES ($1, 'Unit 101', 'occupied')
		 RETURNING id`,
		orgID,
	).Scan(&unitID)
	require.NoError(t, err, "create test unit")

	repo := fin.NewPostgresAssessmentRepository(pool)

	return finTestFixture{
		repo:   repo,
		orgID:  orgID,
		unitID: unitID,
		userID: userID,
		pool:   pool,
	}
}

// createExtraUnit inserts an additional unit in the fixture org.
func (f finTestFixture) createExtraUnit(t *testing.T) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var unitID uuid.UUID
	err := f.pool.QueryRow(ctx,
		`INSERT INTO units (org_id, label, status)
		 VALUES ($1, $2, 'occupied')
		 RETURNING id`,
		f.orgID,
		"Unit-extra-"+uuid.New().String()[:8],
	).Scan(&unitID)
	require.NoError(t, err, "create extra unit")
	return unitID
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func minimalSchedule(orgID, userID uuid.UUID) *fin.AssessmentSchedule {
	now := time.Now().UTC().Truncate(24 * time.Hour)
	return &fin.AssessmentSchedule{
		OrgID:           orgID,
		Name:            "Monthly HOA Fee",
		Frequency:       fin.AssessmentFreqMonthly,
		AmountStrategy:  fin.AmountStrategyFlat,
		BaseAmountCents: 15000,
		AmountRules:     map[string]any{},
		StartsAt:        now,
		IsActive:        true,
		CreatedBy:       userID,
	}
}

func minimalAssessment(orgID, unitID, userID uuid.UUID) *fin.Assessment {
	due := time.Now().UTC().Truncate(24 * time.Hour).AddDate(0, 0, 30)
	createdBy := userID
	return &fin.Assessment{
		OrgID:       orgID,
		UnitID:      unitID,
		Description: "Monthly HOA Assessment",
		AmountCents: 15000,
		DueDate:     due,
		IsRecurring: false,
		CreatedBy:   &createdBy,
	}
}

func chargeEntry(orgID, unitID uuid.UUID) *fin.LedgerEntry {
	now := time.Now().UTC().Truncate(24 * time.Hour)
	desc := "monthly charge"
	return &fin.LedgerEntry{
		OrgID:         orgID,
		UnitID:        unitID,
		EntryType:     fin.LedgerEntryTypeCharge,
		AmountCents:   15000,
		EffectiveDate: now,
		Description:   &desc,
	}
}

func paymentEntry(orgID, unitID uuid.UUID, amountCents int64) *fin.LedgerEntry {
	now := time.Now().UTC().Truncate(24 * time.Hour)
	desc := "payment received"
	return &fin.LedgerEntry{
		OrgID:         orgID,
		UnitID:        unitID,
		EntryType:     fin.LedgerEntryTypePayment,
		AmountCents:   -amountCents, // payments are negative amounts
		EffectiveDate: now,
		Description:   &desc,
	}
}

// ─── TestCreateSchedule ──────────────────────────────────────────────────────

func TestCreateSchedule(t *testing.T) {
	f := setupFinFixture(t)
	ctx := context.Background()

	input := minimalSchedule(f.orgID, f.userID)
	got, err := f.repo.CreateSchedule(ctx, input)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.NotEqual(t, uuid.Nil, got.ID, "should have a generated UUID")
	assert.Equal(t, f.orgID, got.OrgID)
	assert.Equal(t, "Monthly HOA Fee", got.Name)
	assert.Equal(t, fin.AssessmentFreqMonthly, got.Frequency)
	assert.Equal(t, fin.AmountStrategyFlat, got.AmountStrategy)
	assert.Equal(t, int64(15000), got.BaseAmountCents)
	assert.True(t, got.IsActive)
	assert.Equal(t, f.userID, got.CreatedBy)
	assert.False(t, got.CreatedAt.IsZero())
	assert.False(t, got.UpdatedAt.IsZero())
}

// ─── TestListSchedulesByOrg ──────────────────────────────────────────────────

func TestListSchedulesByOrg(t *testing.T) {
	f := setupFinFixture(t)
	ctx := context.Background()

	s1 := minimalSchedule(f.orgID, f.userID)
	s1.Name = "Schedule Alpha"
	_, err := f.repo.CreateSchedule(ctx, s1)
	require.NoError(t, err)

	s2 := minimalSchedule(f.orgID, f.userID)
	s2.Name = "Schedule Beta"
	_, err = f.repo.CreateSchedule(ctx, s2)
	require.NoError(t, err)

	list, err := f.repo.ListSchedulesByOrg(ctx, f.orgID)

	require.NoError(t, err)
	require.Len(t, list, 2, "should return both schedules")

	names := []string{list[0].Name, list[1].Name}
	assert.Contains(t, names, "Schedule Alpha")
	assert.Contains(t, names, "Schedule Beta")
}

func TestListSchedulesByOrg_EmptySliceWhenNone(t *testing.T) {
	f := setupFinFixture(t)
	ctx := context.Background()

	list, err := f.repo.ListSchedulesByOrg(ctx, f.orgID)

	require.NoError(t, err)
	assert.NotNil(t, list, "should return empty slice, not nil")
	assert.Empty(t, list)
}

// ─── TestDeactivateSchedule ──────────────────────────────────────────────────

func TestDeactivateSchedule(t *testing.T) {
	f := setupFinFixture(t)
	ctx := context.Background()

	created, err := f.repo.CreateSchedule(ctx, minimalSchedule(f.orgID, f.userID))
	require.NoError(t, err)
	require.True(t, created.IsActive, "newly created schedule should be active")

	err = f.repo.DeactivateSchedule(ctx, created.ID)
	require.NoError(t, err)

	found, err := f.repo.FindScheduleByID(ctx, created.ID)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.False(t, found.IsActive, "deactivated schedule should have is_active = false")
}

// ─── TestCreateAssessment ────────────────────────────────────────────────────

func TestCreateAssessment(t *testing.T) {
	f := setupFinFixture(t)
	ctx := context.Background()

	input := minimalAssessment(f.orgID, f.unitID, f.userID)
	got, err := f.repo.CreateAssessment(ctx, input)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.NotEqual(t, uuid.Nil, got.ID)
	assert.Equal(t, f.orgID, got.OrgID)
	assert.Equal(t, f.unitID, got.UnitID)
	assert.Equal(t, "Monthly HOA Assessment", got.Description)
	assert.Equal(t, int64(15000), got.AmountCents)
	assert.False(t, got.IsRecurring)
	assert.Nil(t, got.DeletedAt)
	assert.False(t, got.CreatedAt.IsZero())
	assert.False(t, got.UpdatedAt.IsZero())
}

// ─── TestListAssessmentsByUnit ───────────────────────────────────────────────

func TestListAssessmentsByUnit(t *testing.T) {
	f := setupFinFixture(t)
	ctx := context.Background()

	// Create a second unit in the same org.
	otherUnitID := f.createExtraUnit(t)

	// Create assessments for both units.
	a1 := minimalAssessment(f.orgID, f.unitID, f.userID)
	a1.Description = "Unit 101 charge"
	_, err := f.repo.CreateAssessment(ctx, a1)
	require.NoError(t, err)

	a2 := minimalAssessment(f.orgID, otherUnitID, f.userID)
	a2.Description = "Other unit charge"
	_, err = f.repo.CreateAssessment(ctx, a2)
	require.NoError(t, err)

	list, err := f.repo.ListAssessmentsByUnit(ctx, f.unitID)

	require.NoError(t, err)
	require.Len(t, list, 1, "should return only the assessment for the requested unit")
	assert.Equal(t, f.unitID, list[0].UnitID)
	assert.Equal(t, "Unit 101 charge", list[0].Description)
}

func TestListAssessmentsByUnit_EmptySliceWhenNone(t *testing.T) {
	f := setupFinFixture(t)
	ctx := context.Background()

	list, err := f.repo.ListAssessmentsByUnit(ctx, f.unitID)

	require.NoError(t, err)
	assert.NotNil(t, list, "should return empty slice, not nil")
	assert.Empty(t, list)
}

// ─── TestCreateLedgerEntry ───────────────────────────────────────────────────

func TestCreateLedgerEntry(t *testing.T) {
	f := setupFinFixture(t)
	ctx := context.Background()

	entry := chargeEntry(f.orgID, f.unitID)
	got, err := f.repo.CreateLedgerEntry(ctx, entry)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.NotEqual(t, uuid.Nil, got.ID)
	assert.Equal(t, f.orgID, got.OrgID)
	assert.Equal(t, f.unitID, got.UnitID)
	assert.Equal(t, fin.LedgerEntryTypeCharge, got.EntryType)
	assert.Equal(t, int64(15000), got.AmountCents)
	// First entry: balance = 0 + 15000 = 15000
	assert.Equal(t, int64(15000), got.BalanceCents, "balance should equal amount for first entry")
	assert.False(t, got.CreatedAt.IsZero())
}

// ─── TestLedgerEntry_RunningBalance ─────────────────────────────────────────

func TestLedgerEntry_RunningBalance(t *testing.T) {
	f := setupFinFixture(t)
	ctx := context.Background()

	// Create charge: balance = 15000
	charge, err := f.repo.CreateLedgerEntry(ctx, chargeEntry(f.orgID, f.unitID))
	require.NoError(t, err)
	assert.Equal(t, int64(15000), charge.BalanceCents)

	// Create payment of 10000: balance = 15000 + (-10000) = 5000
	payment, err := f.repo.CreateLedgerEntry(ctx, paymentEntry(f.orgID, f.unitID, 10000))
	require.NoError(t, err)
	assert.Equal(t, int64(-10000), payment.AmountCents)
	assert.Equal(t, int64(5000), payment.BalanceCents, "balance after payment should be 5000")
}

// ─── TestGetUnitBalance ──────────────────────────────────────────────────────

func TestGetUnitBalance(t *testing.T) {
	f := setupFinFixture(t)
	ctx := context.Background()

	// No entries yet — should return 0.
	balance, err := f.repo.GetUnitBalance(ctx, f.unitID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), balance, "balance with no entries should be 0")

	// Add a charge.
	_, err = f.repo.CreateLedgerEntry(ctx, chargeEntry(f.orgID, f.unitID))
	require.NoError(t, err)

	// Add a partial payment.
	_, err = f.repo.CreateLedgerEntry(ctx, paymentEntry(f.orgID, f.unitID, 5000))
	require.NoError(t, err)

	// GetUnitBalance should reflect the latest entry's balance_cents (10000).
	balance, err = f.repo.GetUnitBalance(ctx, f.unitID)
	require.NoError(t, err)
	assert.Equal(t, int64(10000), balance)
}

func TestGetUnitBalance_MatchesLatestEntry(t *testing.T) {
	f := setupFinFixture(t)
	ctx := context.Background()

	_, err := f.repo.CreateLedgerEntry(ctx, chargeEntry(f.orgID, f.unitID))
	require.NoError(t, err)

	// Get list to find the latest entry's balance.
	entries, err := f.repo.ListLedgerByUnit(ctx, f.unitID)
	require.NoError(t, err)
	require.Len(t, entries, 1)

	balance, err := f.repo.GetUnitBalance(ctx, f.unitID)
	require.NoError(t, err)
	assert.Equal(t, entries[len(entries)-1].BalanceCents, balance,
		"GetUnitBalance should match the latest entry's balance_cents")
}

// ─── TestListLedgerByUnit ────────────────────────────────────────────────────

func TestListLedgerByUnit_ReturnsInDateOrder(t *testing.T) {
	f := setupFinFixture(t)
	ctx := context.Background()

	// Insert three entries on different effective dates.
	day1 := time.Now().UTC().Truncate(24 * time.Hour).AddDate(0, 0, -2)
	day2 := time.Now().UTC().Truncate(24 * time.Hour).AddDate(0, 0, -1)
	day3 := time.Now().UTC().Truncate(24 * time.Hour)

	desc := "entry"
	for _, d := range []time.Time{day3, day1, day2} { // insert out of order
		e := &fin.LedgerEntry{
			OrgID:         f.orgID,
			UnitID:        f.unitID,
			EntryType:     fin.LedgerEntryTypeCharge,
			AmountCents:   1000,
			EffectiveDate: d,
			Description:   &desc,
		}
		_, err := f.repo.CreateLedgerEntry(ctx, e)
		require.NoError(t, err)
	}

	list, err := f.repo.ListLedgerByUnit(ctx, f.unitID)

	require.NoError(t, err)
	require.Len(t, list, 3)
	assert.True(t, list[0].EffectiveDate.Before(list[1].EffectiveDate) || list[0].EffectiveDate.Equal(list[1].EffectiveDate),
		"entries should be ordered by effective_date ASC")
	assert.True(t, list[1].EffectiveDate.Before(list[2].EffectiveDate) || list[1].EffectiveDate.Equal(list[2].EffectiveDate),
		"entries should be ordered by effective_date ASC")
}

func TestListLedgerByUnit_FiltersToUnit(t *testing.T) {
	f := setupFinFixture(t)
	ctx := context.Background()

	otherUnitID := f.createExtraUnit(t)

	// Create one entry per unit.
	_, err := f.repo.CreateLedgerEntry(ctx, chargeEntry(f.orgID, f.unitID))
	require.NoError(t, err)
	_, err = f.repo.CreateLedgerEntry(ctx, chargeEntry(f.orgID, otherUnitID))
	require.NoError(t, err)

	list, err := f.repo.ListLedgerByUnit(ctx, f.unitID)

	require.NoError(t, err)
	require.Len(t, list, 1, "should only return entries for the requested unit")
	assert.Equal(t, f.unitID, list[0].UnitID)
}

func TestListLedgerByUnit_EmptySliceWhenNone(t *testing.T) {
	f := setupFinFixture(t)
	ctx := context.Background()

	list, err := f.repo.ListLedgerByUnit(ctx, f.unitID)

	require.NoError(t, err)
	assert.NotNil(t, list, "should return empty slice, not nil")
	assert.Empty(t, list)
}
