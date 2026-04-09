//go:build integration

package iam_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/quorant/quorant/internal/iam"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestDB connects to the local Docker postgres and cleans test data
// (users + memberships) after each test.
func setupTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, "postgres://quorant:quorant@localhost:5432/quorant_dev?sslmode=disable")
	require.NoError(t, err, "connecting to test database")

	t.Cleanup(func() {
		cleanCtx := context.Background()
		pool.Exec(cleanCtx, "DELETE FROM memberships")
		pool.Exec(cleanCtx, "DELETE FROM users")
		pool.Close()
	})

	return pool
}

// newTestUser returns a User populated with predictable test values.
func newTestUser(idpUserID, email, displayName string) *iam.User {
	return &iam.User{
		IDPUserID:   idpUserID,
		Email:       email,
		DisplayName: displayName,
		IsActive:    true,
	}
}

// ─── Upsert ──────────────────────────────────────────────────────────────────

func TestUpsert_CreatesNewUser(t *testing.T) {
	pool := setupTestDB(t)
	repo := iam.NewPostgresUserRepository(pool)
	ctx := context.Background()

	input := newTestUser("idp-create-001", "create@example.com", "Create User")
	got, err := repo.Upsert(ctx, input)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.NotEqual(t, uuid.Nil, got.ID, "generated UUID should be non-nil")
	assert.Equal(t, input.IDPUserID, got.IDPUserID)
	assert.Equal(t, input.Email, got.Email)
	assert.Equal(t, input.DisplayName, got.DisplayName)
	assert.True(t, got.IsActive)
	assert.False(t, got.CreatedAt.IsZero())
	assert.False(t, got.UpdatedAt.IsZero())
}

func TestUpsert_UpdatesExistingUser(t *testing.T) {
	pool := setupTestDB(t)
	repo := iam.NewPostgresUserRepository(pool)
	ctx := context.Background()

	// First upsert — create
	first := newTestUser("idp-update-001", "original@example.com", "Original Name")
	created, err := repo.Upsert(ctx, first)
	require.NoError(t, err)
	require.NotNil(t, created)

	// Second upsert — update email (same idp_user_id)
	second := newTestUser("idp-update-001", "updated@example.com", "Updated Name")
	updated, err := repo.Upsert(ctx, second)

	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, created.ID, updated.ID, "ID must not change on update")
	assert.Equal(t, "updated@example.com", updated.Email)
	assert.Equal(t, "Updated Name", updated.DisplayName)
}

// ─── FindByIDPUserID ─────────────────────────────────────────────────────────

func TestFindByIDPUserID_Found(t *testing.T) {
	pool := setupTestDB(t)
	repo := iam.NewPostgresUserRepository(pool)
	ctx := context.Background()

	inserted, err := repo.Upsert(ctx, newTestUser("idp-find-001", "find@example.com", "Find Me"))
	require.NoError(t, err)

	got, err := repo.FindByIDPUserID(ctx, "idp-find-001")

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, inserted.ID, got.ID)
	assert.Equal(t, "find@example.com", got.Email)
}

func TestFindByIDPUserID_NotFound(t *testing.T) {
	pool := setupTestDB(t)
	repo := iam.NewPostgresUserRepository(pool)
	ctx := context.Background()

	got, err := repo.FindByIDPUserID(ctx, "idp-does-not-exist")

	require.NoError(t, err)
	assert.Nil(t, got, "should return nil for unknown idp_user_id")
}

// ─── FindByID ────────────────────────────────────────────────────────────────

func TestFindByID_Found(t *testing.T) {
	pool := setupTestDB(t)
	repo := iam.NewPostgresUserRepository(pool)
	ctx := context.Background()

	inserted, err := repo.Upsert(ctx, newTestUser("idp-byid-001", "byid@example.com", "By ID"))
	require.NoError(t, err)

	got, err := repo.FindByID(ctx, inserted.ID)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, inserted.ID, got.ID)
	assert.Equal(t, "byid@example.com", got.Email)
}

func TestFindByID_NotFound(t *testing.T) {
	pool := setupTestDB(t)
	repo := iam.NewPostgresUserRepository(pool)
	ctx := context.Background()

	randomID := uuid.New()
	got, err := repo.FindByID(ctx, randomID)

	require.NoError(t, err)
	assert.Nil(t, got, "should return nil for unknown UUID")
}

// ─── UpdateLastLogin ─────────────────────────────────────────────────────────

func TestUpdateLastLogin(t *testing.T) {
	pool := setupTestDB(t)
	repo := iam.NewPostgresUserRepository(pool)
	ctx := context.Background()

	user, err := repo.Upsert(ctx, newTestUser("idp-login-001", "login@example.com", "Login User"))
	require.NoError(t, err)
	assert.Nil(t, user.LastLoginAt, "last_login_at should be nil before first login")

	before := time.Now().UTC().Add(-time.Second)
	err = repo.UpdateLastLogin(ctx, user.ID)
	require.NoError(t, err)

	updated, err := repo.FindByID(ctx, user.ID)
	require.NoError(t, err)
	require.NotNil(t, updated)
	require.NotNil(t, updated.LastLoginAt, "last_login_at should be set after UpdateLastLogin")
	assert.True(t, updated.LastLoginAt.After(before),
		"last_login_at (%v) should be after the timestamp recorded before the call (%v)",
		updated.LastLoginAt, before)
}

// ─── FindMembershipsByUserID ─────────────────────────────────────────────────

func TestFindMembershipsByUserID(t *testing.T) {
	pool := setupTestDB(t)
	repo := iam.NewPostgresUserRepository(pool)
	ctx := context.Background()

	// Create a user
	user, err := repo.Upsert(ctx, newTestUser("idp-member-001", "member@example.com", "Member User"))
	require.NoError(t, err)

	// Insert a test organization
	orgID := uuid.New()
	_, err = pool.Exec(ctx,
		`INSERT INTO organizations (id, type, name, slug, path) VALUES ($1, 'hoa', 'Test HOA', $2, $3)`,
		orgID,
		"test-hoa-"+orgID.String()[:8],
		"test_hoa_"+orgID.String()[:8],
	)
	require.NoError(t, err, "inserting test organization")

	// Insert a membership linking user to org with the 'homeowner' role
	_, err = pool.Exec(ctx,
		`INSERT INTO memberships (user_id, org_id, role_id, status)
		 VALUES ($1, $2, (SELECT id FROM roles WHERE name = 'homeowner'), 'active')`,
		user.ID,
		orgID,
	)
	require.NoError(t, err, "inserting test membership")

	memberships, err := repo.FindMembershipsByUserID(ctx, user.ID)

	require.NoError(t, err)
	require.Len(t, memberships, 1, "expected exactly one membership")

	m := memberships[0]
	assert.Equal(t, user.ID, m.UserID)
	assert.Equal(t, orgID, m.OrgID)
	assert.Equal(t, "homeowner", m.RoleName)
	assert.Equal(t, "active", m.Status)
	assert.False(t, m.CreatedAt.IsZero())
}

func TestFindMembershipsByUserID_NoMemberships(t *testing.T) {
	pool := setupTestDB(t)
	repo := iam.NewPostgresUserRepository(pool)
	ctx := context.Background()

	user, err := repo.Upsert(ctx, newTestUser("idp-nomem-001", "nomem@example.com", "No Memberships"))
	require.NoError(t, err)

	memberships, err := repo.FindMembershipsByUserID(ctx, user.ID)

	require.NoError(t, err)
	assert.Empty(t, memberships, "user with no memberships should return empty slice")
}
