//go:build integration

package gov_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/quorant/quorant/internal/gov"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Test DB Setup ───────────────────────────────────────────────────────────

// setupGovTestDB connects to the local Docker postgres and registers a cleanup
// function to purge gov test data in the correct FK order.
func setupGovTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, "postgres://quorant:quorant@localhost:5432/quorant_dev?sslmode=disable")
	require.NoError(t, err, "connecting to test database")

	t.Cleanup(func() {
		cleanCtx := context.Background()
		pool.Exec(cleanCtx, "DELETE FROM arb_votes")
		pool.Exec(cleanCtx, "DELETE FROM arb_requests")
		pool.Exec(cleanCtx, "DELETE FROM violation_actions")
		pool.Exec(cleanCtx, "DELETE FROM violations")
		pool.Exec(cleanCtx, "DELETE FROM unit_memberships")
		pool.Exec(cleanCtx, "DELETE FROM units")
		pool.Exec(cleanCtx, "DELETE FROM memberships")
		pool.Exec(cleanCtx, "DELETE FROM organizations_management")
		pool.Exec(cleanCtx, "DELETE FROM organizations")
		pool.Exec(cleanCtx, "DELETE FROM users")
		pool.Close()
	})

	return pool
}

// govTestFixture holds resources shared by gov tests.
type govTestFixture struct {
	pool   *pgxpool.Pool
	orgID  uuid.UUID
	unitID uuid.UUID
	userID uuid.UUID
}

// setupGovFixture creates a pool, a test org, test user, and test unit.
func setupGovFixture(t *testing.T) govTestFixture {
	t.Helper()
	pool := setupGovTestDB(t)
	ctx := context.Background()

	// Insert a test user.
	var userID uuid.UUID
	err := pool.QueryRow(ctx,
		`INSERT INTO users (idp_user_id, email, display_name)
		 VALUES ($1, $2, $3) RETURNING id`,
		"test-idp-"+uuid.New().String(),
		"govtest-"+uuid.New().String()+"@example.com",
		"Gov Test User",
	).Scan(&userID)
	require.NoError(t, err, "insert test user")

	// Insert a test org (type hoa).
	var orgID uuid.UUID
	err = pool.QueryRow(ctx,
		`INSERT INTO organizations (type, name, slug, path)
		 VALUES ('hoa', $1, $2, $3) RETURNING id`,
		"Gov Test HOA "+uuid.New().String(),
		"gov-test-hoa-"+uuid.New().String(),
		"gov_test_hoa_"+uuid.New().String(),
	).Scan(&orgID)
	require.NoError(t, err, "insert test org")

	// Insert a test unit.
	var unitID uuid.UUID
	err = pool.QueryRow(ctx,
		`INSERT INTO units (org_id, label, unit_type, status)
		 VALUES ($1, $2, 'single_family', 'occupied') RETURNING id`,
		orgID,
		"Unit-101-"+uuid.New().String(),
	).Scan(&unitID)
	require.NoError(t, err, "insert test unit")

	return govTestFixture{
		pool:   pool,
		orgID:  orgID,
		unitID: unitID,
		userID: userID,
	}
}

// newTestViolation builds a minimal Violation for insertion.
func newTestViolation(f govTestFixture) *gov.Violation {
	return &gov.Violation{
		OrgID:       f.orgID,
		UnitID:      f.unitID,
		ReportedBy:  f.userID,
		Title:       "Lawn not mowed",
		Description: "Grass exceeds 6 inches",
		Category:    "landscaping",
		Status:      "open",
		Severity:    2,
	}
}

// ─── TestCreateViolation + FindByID ──────────────────────────────────────────

func TestCreateViolation(t *testing.T) {
	f := setupGovFixture(t)
	repo := gov.NewPostgresViolationRepository(f.pool)
	ctx := context.Background()

	input := newTestViolation(f)
	got, err := repo.Create(ctx, input)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.NotEqual(t, uuid.Nil, got.ID)
	assert.Equal(t, f.orgID, got.OrgID)
	assert.Equal(t, f.unitID, got.UnitID)
	assert.Equal(t, f.userID, got.ReportedBy)
	assert.Equal(t, "Lawn not mowed", got.Title)
	assert.Equal(t, "landscaping", got.Category)
	assert.Equal(t, "open", got.Status)
	assert.Equal(t, int16(2), got.Severity)
	assert.NotNil(t, got.Metadata)
	assert.NotNil(t, got.EvidenceDocIDs)
	assert.False(t, got.CreatedAt.IsZero())
	assert.False(t, got.UpdatedAt.IsZero())
	assert.Nil(t, got.DeletedAt)
}

func TestFindViolationByID_Found(t *testing.T) {
	f := setupGovFixture(t)
	repo := gov.NewPostgresViolationRepository(f.pool)
	ctx := context.Background()

	created, err := repo.Create(ctx, newTestViolation(f))
	require.NoError(t, err)

	got, err := repo.FindByID(ctx, created.ID)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, created.ID, got.ID)
	assert.Equal(t, "Lawn not mowed", got.Title)
}

func TestFindViolationByID_NotFound(t *testing.T) {
	f := setupGovFixture(t)
	repo := gov.NewPostgresViolationRepository(f.pool)
	ctx := context.Background()

	got, err := repo.FindByID(ctx, uuid.New())

	require.NoError(t, err)
	assert.Nil(t, got, "should return nil for unknown ID")
}

// ─── TestListViolationsByOrg / ByUnit ────────────────────────────────────────

func TestListViolationsByOrg(t *testing.T) {
	f := setupGovFixture(t)
	repo := gov.NewPostgresViolationRepository(f.pool)
	ctx := context.Background()

	// Create two violations for our org.
	_, err := repo.Create(ctx, newTestViolation(f))
	require.NoError(t, err)
	_, err = repo.Create(ctx, newTestViolation(f))
	require.NoError(t, err)

	list, err := repo.ListByOrg(ctx, f.orgID)

	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(list), 2)
	for _, v := range list {
		assert.Equal(t, f.orgID, v.OrgID)
	}
}

func TestListViolationsByOrg_EmptyForUnknown(t *testing.T) {
	f := setupGovFixture(t)
	repo := gov.NewPostgresViolationRepository(f.pool)
	ctx := context.Background()

	list, err := repo.ListByOrg(ctx, uuid.New())

	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestListViolationsByUnit(t *testing.T) {
	f := setupGovFixture(t)
	repo := gov.NewPostgresViolationRepository(f.pool)
	ctx := context.Background()

	// Create another unit to ensure we filter correctly.
	var otherUnitID uuid.UUID
	err := f.pool.QueryRow(ctx,
		`INSERT INTO units (org_id, label, unit_type, status)
		 VALUES ($1, $2, 'single_family', 'occupied') RETURNING id`,
		f.orgID,
		"Unit-202-"+uuid.New().String(),
	).Scan(&otherUnitID)
	require.NoError(t, err)

	// Create violations for our unit and one for the other unit.
	_, err = repo.Create(ctx, newTestViolation(f))
	require.NoError(t, err)

	otherV := newTestViolation(f)
	otherV.UnitID = otherUnitID
	_, err = repo.Create(ctx, otherV)
	require.NoError(t, err)

	list, err := repo.ListByUnit(ctx, f.unitID)

	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, f.unitID, list[0].UnitID)
}

// ─── TestUpdateViolation ─────────────────────────────────────────────────────

func TestUpdateViolation_ChangeStatus(t *testing.T) {
	f := setupGovFixture(t)
	repo := gov.NewPostgresViolationRepository(f.pool)
	ctx := context.Background()

	created, err := repo.Create(ctx, newTestViolation(f))
	require.NoError(t, err)

	created.Status = "acknowledged"
	updated, err := repo.Update(ctx, created)

	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, created.ID, updated.ID)
	assert.Equal(t, "acknowledged", updated.Status)
	assert.True(t, updated.UpdatedAt.After(created.CreatedAt) || updated.UpdatedAt.Equal(created.CreatedAt))
}

func TestSoftDeleteViolation(t *testing.T) {
	f := setupGovFixture(t)
	repo := gov.NewPostgresViolationRepository(f.pool)
	ctx := context.Background()

	created, err := repo.Create(ctx, newTestViolation(f))
	require.NoError(t, err)

	err = repo.SoftDelete(ctx, created.ID)
	require.NoError(t, err)

	got, err := repo.FindByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Nil(t, got, "soft-deleted violation should not be returned by FindByID")
}

// ─── TestCreateViolationAction + ListActions ─────────────────────────────────

func TestCreateViolationAction(t *testing.T) {
	f := setupGovFixture(t)
	repo := gov.NewPostgresViolationRepository(f.pool)
	ctx := context.Background()

	violation, err := repo.Create(ctx, newTestViolation(f))
	require.NoError(t, err)

	notes := "Sent notice via email"
	action := &gov.ViolationAction{
		ViolationID: violation.ID,
		ActorID:     f.userID,
		ActionType:  "notice_sent",
		Notes:       &notes,
		Metadata:    map[string]any{"channel": "email"},
	}

	got, err := repo.CreateAction(ctx, action)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.NotEqual(t, uuid.Nil, got.ID)
	assert.Equal(t, violation.ID, got.ViolationID)
	assert.Equal(t, f.userID, got.ActorID)
	assert.Equal(t, "notice_sent", got.ActionType)
	assert.Equal(t, &notes, got.Notes)
	assert.False(t, got.CreatedAt.IsZero())
}

func TestListActionsByViolation(t *testing.T) {
	f := setupGovFixture(t)
	repo := gov.NewPostgresViolationRepository(f.pool)
	ctx := context.Background()

	violation, err := repo.Create(ctx, newTestViolation(f))
	require.NoError(t, err)

	// Create two actions.
	for _, actionType := range []string{"notice_sent", "fine_issued"} {
		_, err := repo.CreateAction(ctx, &gov.ViolationAction{
			ViolationID: violation.ID,
			ActorID:     f.userID,
			ActionType:  actionType,
		})
		require.NoError(t, err)
	}

	actions, err := repo.ListActionsByViolation(ctx, violation.ID)

	require.NoError(t, err)
	require.Len(t, actions, 2)
	assert.Equal(t, "notice_sent", actions[0].ActionType)
	assert.Equal(t, "fine_issued", actions[1].ActionType)
}

// ─── TestGetOffenseCount ─────────────────────────────────────────────────────

func TestGetOffenseCount(t *testing.T) {
	f := setupGovFixture(t)
	repo := gov.NewPostgresViolationRepository(f.pool)
	ctx := context.Background()

	// Initially zero offenses.
	count, err := repo.GetOffenseCount(ctx, f.unitID, "landscaping")
	require.NoError(t, err)
	assert.Equal(t, 0, count)

	// Create two landscaping violations for the unit.
	_, err = repo.Create(ctx, newTestViolation(f))
	require.NoError(t, err)
	_, err = repo.Create(ctx, newTestViolation(f))
	require.NoError(t, err)

	// Create a violation with a different category — should not count.
	other := newTestViolation(f)
	other.Category = "noise"
	_, err = repo.Create(ctx, other)
	require.NoError(t, err)

	count, err = repo.GetOffenseCount(ctx, f.unitID, "landscaping")
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}

func TestGetOffenseCount_ExcludesDeleted(t *testing.T) {
	f := setupGovFixture(t)
	repo := gov.NewPostgresViolationRepository(f.pool)
	ctx := context.Background()

	v, err := repo.Create(ctx, newTestViolation(f))
	require.NoError(t, err)

	// Soft-delete the violation.
	err = repo.SoftDelete(ctx, v.ID)
	require.NoError(t, err)

	count, err := repo.GetOffenseCount(ctx, f.unitID, "landscaping")
	require.NoError(t, err)
	assert.Equal(t, 0, count, "soft-deleted violations should not count as offenses")
}
