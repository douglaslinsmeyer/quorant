//go:build integration

package org_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/org"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Fixtures ────────────────────────────────────────────────────────────────

// unitTestFixture holds all resources needed by unit repository tests.
type unitTestFixture struct {
	repo   *org.PostgresUnitRepository
	orgID  uuid.UUID
	userID uuid.UUID
}

// setupUnitFixture creates a pool, a test HOA org, and a test user.
func setupUnitFixture(t *testing.T) unitTestFixture {
	t.Helper()
	pool := setupTestDB(t)
	ctx := context.Background()

	// Create a test HOA org.
	orgRepo := org.NewPostgresOrgRepository(pool)
	testOrg, err := orgRepo.Create(ctx, newHOAOrg("Unit Test HOA"))
	require.NoError(t, err, "create test org")

	// Create a test user.
	var userID uuid.UUID
	err = pool.QueryRow(ctx,
		`INSERT INTO users (idp_user_id, email, display_name)
		 VALUES ($1, $2, $3)
		 RETURNING id`,
		"unit-test-idp-"+uuid.New().String(),
		"unit-test-"+uuid.New().String()+"@example.com",
		"Unit Test User",
	).Scan(&userID)
	require.NoError(t, err, "create test user")

	repo := org.NewPostgresUnitRepository(pool)

	return unitTestFixture{
		repo:   repo,
		orgID:  testOrg.ID,
		userID: userID,
	}
}

// newUnit builds a minimal Unit for insertion.
func newUnit(orgID uuid.UUID, label string) *org.Unit {
	return &org.Unit{
		OrgID:  orgID,
		Label:  label,
		Status: "occupied",
	}
}

// ─── TestCreateUnit ──────────────────────────────────────────────────────────

func TestCreateUnit(t *testing.T) {
	f := setupUnitFixture(t)
	ctx := context.Background()

	input := newUnit(f.orgID, "Unit 101")
	got, err := f.repo.CreateUnit(ctx, input)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.NotEqual(t, uuid.Nil, got.ID, "should have a generated UUID")
	assert.Equal(t, f.orgID, got.OrgID)
	assert.Equal(t, "Unit 101", got.Label)
	assert.Equal(t, 1.00, got.VotingWeight, "default voting_weight should be 1.00")
	assert.NotNil(t, got.Metadata, "metadata should not be nil")
	assert.False(t, got.CreatedAt.IsZero())
	assert.False(t, got.UpdatedAt.IsZero())
	assert.Nil(t, got.DeletedAt)
}

func TestCreateUnit_DefaultsVotingWeight(t *testing.T) {
	f := setupUnitFixture(t)
	ctx := context.Background()

	input := &org.Unit{
		OrgID:  f.orgID,
		Label:  "Unit Zero Weight",
		Status: "occupied",
		// VotingWeight intentionally left as zero
	}
	got, err := f.repo.CreateUnit(ctx, input)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, 1.00, got.VotingWeight, "zero voting_weight should default to 1.00")
}

// ─── TestFindUnitByID ────────────────────────────────────────────────────────

func TestFindUnitByID_Found(t *testing.T) {
	f := setupUnitFixture(t)
	ctx := context.Background()

	created, err := f.repo.CreateUnit(ctx, newUnit(f.orgID, "Find Me Unit"))
	require.NoError(t, err)

	got, err := f.repo.FindUnitByID(ctx, created.ID)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, created.ID, got.ID)
	assert.Equal(t, "Find Me Unit", got.Label)
}

func TestFindUnitByID_NotFound(t *testing.T) {
	f := setupUnitFixture(t)
	ctx := context.Background()

	got, err := f.repo.FindUnitByID(ctx, uuid.New())

	require.NoError(t, err)
	assert.Nil(t, got, "should return nil for unknown ID")
}

// ─── TestListUnitsByOrg ──────────────────────────────────────────────────────

func TestListUnitsByOrg_ReturnsInLabelOrder(t *testing.T) {
	f := setupUnitFixture(t)
	ctx := context.Background()

	// Insert in reverse label order to confirm sorting.
	for _, label := range []string{"Unit C", "Unit A", "Unit B"} {
		_, err := f.repo.CreateUnit(ctx, newUnit(f.orgID, label))
		require.NoError(t, err)
	}

	list, err := f.repo.ListUnitsByOrg(ctx, f.orgID)

	require.NoError(t, err)
	require.Len(t, list, 3)
	assert.Equal(t, "Unit A", list[0].Label)
	assert.Equal(t, "Unit B", list[1].Label)
	assert.Equal(t, "Unit C", list[2].Label)
}

func TestListUnitsByOrg_ExcludesOtherOrgs(t *testing.T) {
	f := setupUnitFixture(t)
	ctx := context.Background()

	// Create a second org and unit in it.
	pool := setupTestDB(t)
	orgRepo := org.NewPostgresOrgRepository(pool)
	otherOrg, err := orgRepo.Create(ctx, newHOAOrg("Other HOA"))
	require.NoError(t, err)
	otherRepo := org.NewPostgresUnitRepository(pool)
	_, err = otherRepo.CreateUnit(ctx, newUnit(otherOrg.ID, "Other Unit"))
	require.NoError(t, err)

	// Create one unit in the fixture org.
	_, err = f.repo.CreateUnit(ctx, newUnit(f.orgID, "My Unit"))
	require.NoError(t, err)

	list, err := f.repo.ListUnitsByOrg(ctx, f.orgID)

	require.NoError(t, err)
	for _, u := range list {
		assert.Equal(t, f.orgID, u.OrgID, "list should only contain units from the fixture org")
	}
}

// ─── TestUpdateUnit ──────────────────────────────────────────────────────────

func TestUpdateUnit_ChangesLabel(t *testing.T) {
	f := setupUnitFixture(t)
	ctx := context.Background()

	created, err := f.repo.CreateUnit(ctx, newUnit(f.orgID, "Original Label"))
	require.NoError(t, err)

	created.Label = "Updated Label"
	updated, err := f.repo.UpdateUnit(ctx, created)

	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, created.ID, updated.ID)
	assert.Equal(t, "Updated Label", updated.Label)
	assert.True(t, updated.UpdatedAt.After(created.CreatedAt) || updated.UpdatedAt.Equal(created.CreatedAt),
		"updated_at should be >= created_at")
}

// ─── TestSoftDeleteUnit ──────────────────────────────────────────────────────

func TestSoftDeleteUnit_HidesFromFindByID(t *testing.T) {
	f := setupUnitFixture(t)
	ctx := context.Background()

	created, err := f.repo.CreateUnit(ctx, newUnit(f.orgID, "Delete Me Unit"))
	require.NoError(t, err)

	err = f.repo.SoftDeleteUnit(ctx, created.ID)
	require.NoError(t, err)

	got, err := f.repo.FindUnitByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Nil(t, got, "soft-deleted unit should not be returned by FindUnitByID")
}

func TestSoftDeleteUnit_HidesFromList(t *testing.T) {
	f := setupUnitFixture(t)
	ctx := context.Background()

	u1, err := f.repo.CreateUnit(ctx, newUnit(f.orgID, "Keep Unit"))
	require.NoError(t, err)
	u2, err := f.repo.CreateUnit(ctx, newUnit(f.orgID, "Remove Unit"))
	require.NoError(t, err)

	err = f.repo.SoftDeleteUnit(ctx, u2.ID)
	require.NoError(t, err)

	list, err := f.repo.ListUnitsByOrg(ctx, f.orgID)
	require.NoError(t, err)

	ids := make([]uuid.UUID, len(list))
	for i, u := range list {
		ids[i] = u.ID
	}
	assert.Contains(t, ids, u1.ID, "non-deleted unit should be in list")
	assert.NotContains(t, ids, u2.ID, "soft-deleted unit should not be in list")
}

// ─── TestUpsertProperty ──────────────────────────────────────────────────────

func TestUpsertProperty_CreatesWhenMissing(t *testing.T) {
	f := setupUnitFixture(t)
	ctx := context.Background()

	unit, err := f.repo.CreateUnit(ctx, newUnit(f.orgID, "Property Unit"))
	require.NoError(t, err)

	sqft := 1200
	prop := &org.Property{
		UnitID:     unit.ID,
		SquareFeet: &sqft,
	}
	got, err := f.repo.UpsertProperty(ctx, prop)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.NotEqual(t, uuid.Nil, got.ID)
	assert.Equal(t, unit.ID, got.UnitID)
	assert.Equal(t, &sqft, got.SquareFeet)
	assert.False(t, got.CreatedAt.IsZero())
}

func TestUpsertProperty_UpdatesWhenExists(t *testing.T) {
	f := setupUnitFixture(t)
	ctx := context.Background()

	unit, err := f.repo.CreateUnit(ctx, newUnit(f.orgID, "Update Property Unit"))
	require.NoError(t, err)

	sqft1 := 1000
	_, err = f.repo.UpsertProperty(ctx, &org.Property{
		UnitID:     unit.ID,
		SquareFeet: &sqft1,
	})
	require.NoError(t, err)

	sqft2 := 1500
	parcel := "ABC-123"
	updated, err := f.repo.UpsertProperty(ctx, &org.Property{
		UnitID:       unit.ID,
		SquareFeet:   &sqft2,
		ParcelNumber: &parcel,
	})

	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, unit.ID, updated.UnitID)
	assert.Equal(t, &sqft2, updated.SquareFeet, "square_feet should be updated")
	assert.Equal(t, &parcel, updated.ParcelNumber, "parcel_number should be updated")
}

func TestGetProperty_NotFound(t *testing.T) {
	f := setupUnitFixture(t)
	ctx := context.Background()

	unit, err := f.repo.CreateUnit(ctx, newUnit(f.orgID, "No Property Unit"))
	require.NoError(t, err)

	got, err := f.repo.GetProperty(ctx, unit.ID)

	require.NoError(t, err)
	assert.Nil(t, got, "should return nil when no property exists")
}

// ─── TestCreateUnitMembership ────────────────────────────────────────────────

func TestCreateUnitMembership(t *testing.T) {
	f := setupUnitFixture(t)
	ctx := context.Background()

	unit, err := f.repo.CreateUnit(ctx, newUnit(f.orgID, "Membership Unit"))
	require.NoError(t, err)

	m := &org.UnitMembership{
		UnitID:       unit.ID,
		UserID:       f.userID,
		Relationship: "owner",
		IsVoter:      true,
		StartedAt:    time.Now(),
	}
	got, err := f.repo.CreateUnitMembership(ctx, m)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.NotEqual(t, uuid.Nil, got.ID, "should have a generated UUID")
	assert.Equal(t, unit.ID, got.UnitID)
	assert.Equal(t, f.userID, got.UserID)
	assert.Equal(t, "owner", got.Relationship)
	assert.True(t, got.IsVoter)
	assert.Nil(t, got.EndedAt)
	assert.False(t, got.CreatedAt.IsZero())
}

// ─── TestListUnitMemberships ─────────────────────────────────────────────────

func TestListUnitMemberships_ReturnsActiveMembershipsOnly(t *testing.T) {
	f := setupUnitFixture(t)
	ctx := context.Background()

	unit, err := f.repo.CreateUnit(ctx, newUnit(f.orgID, "List Memberships Unit"))
	require.NoError(t, err)

	// Create an active membership.
	active, err := f.repo.CreateUnitMembership(ctx, &org.UnitMembership{
		UnitID:       unit.ID,
		UserID:       f.userID,
		Relationship: "owner",
		IsVoter:      true,
		StartedAt:    time.Now(),
	})
	require.NoError(t, err)

	// End it, then create another user with a new active membership.
	pool := setupTestDB(t)
	ctx2 := context.Background()
	var secondUserID uuid.UUID
	err = pool.QueryRow(ctx2,
		`INSERT INTO users (idp_user_id, email, display_name)
		 VALUES ($1, $2, $3)
		 RETURNING id`,
		"list-test-idp-"+uuid.New().String(),
		"list-test-"+uuid.New().String()+"@example.com",
		"List Test User",
	).Scan(&secondUserID)
	require.NoError(t, err)

	repo2 := org.NewPostgresUnitRepository(pool)

	// End the active membership.
	err = repo2.EndUnitMembership(ctx, active.ID)
	require.NoError(t, err)

	// Create a new active membership for secondUser.
	newActive, err := f.repo.CreateUnitMembership(ctx, &org.UnitMembership{
		UnitID:       unit.ID,
		UserID:       secondUserID,
		Relationship: "owner",
		IsVoter:      true,
		StartedAt:    time.Now(),
	})
	require.NoError(t, err)

	list, err := f.repo.ListUnitMemberships(ctx, unit.ID)

	require.NoError(t, err)
	require.Len(t, list, 1, "should return only active memberships")
	assert.Equal(t, newActive.ID, list[0].ID)
	assert.Nil(t, list[0].EndedAt, "active membership should have nil ended_at")
}

// ─── TestEndUnitMembership ───────────────────────────────────────────────────

func TestEndUnitMembership_SetsEndedAt(t *testing.T) {
	f := setupUnitFixture(t)
	ctx := context.Background()

	unit, err := f.repo.CreateUnit(ctx, newUnit(f.orgID, "End Membership Unit"))
	require.NoError(t, err)

	created, err := f.repo.CreateUnitMembership(ctx, &org.UnitMembership{
		UnitID:       unit.ID,
		UserID:       f.userID,
		Relationship: "owner",
		IsVoter:      true,
		StartedAt:    time.Now(),
	})
	require.NoError(t, err)
	assert.Nil(t, created.EndedAt, "newly created membership should have nil ended_at")

	err = f.repo.EndUnitMembership(ctx, created.ID)
	require.NoError(t, err)

	// Should no longer appear in the active list.
	list, err := f.repo.ListUnitMemberships(ctx, unit.ID)
	require.NoError(t, err)
	for _, m := range list {
		assert.NotEqual(t, created.ID, m.ID, "ended membership should not appear in active list")
	}
}

func TestEndUnitMembership_IdempotentOnAlreadyEnded(t *testing.T) {
	f := setupUnitFixture(t)
	ctx := context.Background()

	unit, err := f.repo.CreateUnit(ctx, newUnit(f.orgID, "Idempotent End Unit"))
	require.NoError(t, err)

	created, err := f.repo.CreateUnitMembership(ctx, &org.UnitMembership{
		UnitID:       unit.ID,
		UserID:       f.userID,
		Relationship: "owner",
		IsVoter:      true,
		StartedAt:    time.Now(),
	})
	require.NoError(t, err)

	err = f.repo.EndUnitMembership(ctx, created.ID)
	require.NoError(t, err)

	// Calling again should not produce an error (WHERE ended_at IS NULL means it's a no-op).
	err = f.repo.EndUnitMembership(ctx, created.ID)
	require.NoError(t, err)
}
