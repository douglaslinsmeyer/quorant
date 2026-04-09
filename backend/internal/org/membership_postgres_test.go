//go:build integration

package org_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/iam"
	"github.com/quorant/quorant/internal/org"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Helpers ─────────────────────────────────────────────────────────────────

// createTestUser inserts a minimal user row and returns its UUID.
func createTestUser(t *testing.T, ctx context.Context, pool interface {
	Exec(ctx context.Context, sql string, args ...any) (interface{ RowsAffected() int64 }, error)
	QueryRow(ctx context.Context, sql string, args ...any) interface{ Scan(dest ...any) error }
}) uuid.UUID {
	t.Helper()
	// Use the pool directly via the setupTestDB helper.
	return uuid.Nil
}

// fetchHomeownerRoleID looks up the seeded 'homeowner' role and returns its ID.
func fetchHomeownerRoleID(t *testing.T, ctx context.Context, repo *org.PostgresMembershipRepository) uuid.UUID {
	t.Helper()
	// We need direct DB access — use pool from setupTestDB.
	return uuid.Nil
}

// ─── Setup ───────────────────────────────────────────────────────────────────

// membershipTestFixture holds all resources needed by membership tests.
type membershipTestFixture struct {
	repo       *org.PostgresMembershipRepository
	userID     uuid.UUID
	orgID      uuid.UUID
	roleID     uuid.UUID // homeowner role
	roleName   string
}

// setupMembershipFixture creates a pool, a test user, a test org, and fetches
// the homeowner role ID from the seed data.
func setupMembershipFixture(t *testing.T) membershipTestFixture {
	t.Helper()
	pool := setupTestDB(t)
	ctx := context.Background()

	// Create a test user.
	var userID uuid.UUID
	err := pool.QueryRow(ctx,
		`INSERT INTO users (idp_user_id, email, display_name)
		 VALUES ($1, $2, $3)
		 RETURNING id`,
		"test-idp-"+uuid.New().String(),
		"test-"+uuid.New().String()+"@example.com",
		"Test User",
	).Scan(&userID)
	require.NoError(t, err, "insert test user")

	// Create a test org.
	orgRepo := org.NewPostgresOrgRepository(pool)
	testOrg, err := orgRepo.Create(ctx, newHOAOrg("Membership Test HOA"))
	require.NoError(t, err, "create test org")

	// Fetch homeowner role ID from seeded data.
	var roleID uuid.UUID
	var roleName string
	err = pool.QueryRow(ctx,
		`SELECT id, name FROM roles WHERE name = 'homeowner'`,
	).Scan(&roleID, &roleName)
	require.NoError(t, err, "fetch homeowner role")

	repo := org.NewPostgresMembershipRepository(pool)

	return membershipTestFixture{
		repo:     repo,
		userID:   userID,
		orgID:    testOrg.ID,
		roleID:   roleID,
		roleName: roleName,
	}
}

// newMembership builds a minimal Membership for insertion.
func newMembership(f membershipTestFixture) *iam.Membership {
	return &iam.Membership{
		UserID: f.userID,
		OrgID:  f.orgID,
		RoleID: f.roleID,
		Status: "active",
	}
}

// ─── TestCreateMembership ────────────────────────────────────────────────────

func TestCreateMembership(t *testing.T) {
	f := setupMembershipFixture(t)
	ctx := context.Background()

	input := newMembership(f)
	got, err := f.repo.Create(ctx, input)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.NotEqual(t, uuid.Nil, got.ID, "should have a generated UUID")
	assert.Equal(t, f.userID, got.UserID)
	assert.Equal(t, f.orgID, got.OrgID)
	assert.Equal(t, f.roleID, got.RoleID)
	assert.Equal(t, f.roleName, got.RoleName, "RoleName should be populated via role lookup")
	assert.Equal(t, "active", got.Status)
	assert.False(t, got.CreatedAt.IsZero())
	assert.False(t, got.UpdatedAt.IsZero())
	assert.Nil(t, got.DeletedAt)
}

// ─── TestFindByID ────────────────────────────────────────────────────────────

func TestMembershipFindByID_Found(t *testing.T) {
	f := setupMembershipFixture(t)
	ctx := context.Background()

	created, err := f.repo.Create(ctx, newMembership(f))
	require.NoError(t, err)

	got, err := f.repo.FindByID(ctx, created.ID)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, created.ID, got.ID)
	assert.Equal(t, f.roleName, got.RoleName, "RoleName should be populated from JOIN")
}

func TestMembershipFindByID_NotFound(t *testing.T) {
	f := setupMembershipFixture(t)
	ctx := context.Background()

	got, err := f.repo.FindByID(ctx, uuid.New())

	require.NoError(t, err)
	assert.Nil(t, got, "should return nil for unknown ID")
}

// ─── TestListByOrg ───────────────────────────────────────────────────────────

func TestListByOrg_ReturnsMemberships(t *testing.T) {
	f := setupMembershipFixture(t)
	pool := setupTestDB(t)
	ctx := context.Background()

	// Create a second user for a second membership.
	var secondUserID uuid.UUID
	err := pool.QueryRow(ctx,
		`INSERT INTO users (idp_user_id, email, display_name)
		 VALUES ($1, $2, $3)
		 RETURNING id`,
		"test-idp-"+uuid.New().String(),
		"test2-"+uuid.New().String()+"@example.com",
		"Test User 2",
	).Scan(&secondUserID)
	require.NoError(t, err)

	_, err = f.repo.Create(ctx, newMembership(f))
	require.NoError(t, err)

	_, err = f.repo.Create(ctx, &iam.Membership{
		UserID: secondUserID,
		OrgID:  f.orgID,
		RoleID: f.roleID,
		Status: "active",
	})
	require.NoError(t, err)

	list, err := f.repo.ListByOrg(ctx, f.orgID)

	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(list), 2, "should return at least two memberships")
	for _, m := range list {
		assert.Equal(t, f.orgID, m.OrgID)
		assert.NotEmpty(t, m.RoleName, "RoleName should be populated")
	}
}

// ─── TestUpdate_ChangesStatus ────────────────────────────────────────────────

func TestUpdate_ChangesStatus(t *testing.T) {
	f := setupMembershipFixture(t)
	ctx := context.Background()

	created, err := f.repo.Create(ctx, newMembership(f))
	require.NoError(t, err)

	created.Status = "inactive"
	updated, err := f.repo.Update(ctx, created)

	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, created.ID, updated.ID)
	assert.Equal(t, "inactive", updated.Status)
	assert.Equal(t, f.roleName, updated.RoleName, "RoleName should still be populated after update")
	assert.True(t, updated.UpdatedAt.After(created.CreatedAt) || updated.UpdatedAt.Equal(created.CreatedAt),
		"updated_at should be >= created_at")
}

// ─── TestSoftDelete ──────────────────────────────────────────────────────────

func TestMembershipSoftDelete(t *testing.T) {
	f := setupMembershipFixture(t)
	ctx := context.Background()

	created, err := f.repo.Create(ctx, newMembership(f))
	require.NoError(t, err)

	err = f.repo.SoftDelete(ctx, created.ID)
	require.NoError(t, err)

	// FindByID should not return soft-deleted memberships.
	got, err := f.repo.FindByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Nil(t, got, "soft-deleted membership should not be returned by FindByID")

	// ListByOrg should also exclude soft-deleted memberships.
	list, err := f.repo.ListByOrg(ctx, f.orgID)
	require.NoError(t, err)
	for _, m := range list {
		assert.NotEqual(t, created.ID, m.ID, "soft-deleted membership should not appear in list")
	}
}
