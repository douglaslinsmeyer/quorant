//go:build integration

package org_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/quorant/quorant/internal/org"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestDB connects to the local Docker postgres and registers a cleanup
// function to purge test data in the correct FK order.
func setupTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, "postgres://quorant:quorant@localhost:5432/quorant_dev?sslmode=disable")
	require.NoError(t, err, "connecting to test database")

	t.Cleanup(func() {
		cleanCtx := context.Background()
		pool.Exec(cleanCtx, "DELETE FROM organizations_management")
		pool.Exec(cleanCtx, "DELETE FROM memberships")
		pool.Exec(cleanCtx, "DELETE FROM organizations WHERE parent_id IS NOT NULL")
		pool.Exec(cleanCtx, "DELETE FROM organizations")
		pool.Close()
	})

	return pool
}

// newFirmOrg returns an Organization of type "firm" for testing.
func newFirmOrg(name string) *org.Organization {
	return &org.Organization{
		Type:     "firm",
		Name:     name,
		Settings: map[string]any{},
	}
}

// newHOAOrg returns an Organization of type "hoa" for testing.
func newHOAOrg(name string) *org.Organization {
	return &org.Organization{
		Type:     "hoa",
		Name:     name,
		Settings: map[string]any{},
	}
}

// ─── Create ──────────────────────────────────────────────────────────────────

func TestCreate_NewOrg(t *testing.T) {
	pool := setupTestDB(t)
	repo := org.NewPostgresOrgRepository(pool)
	ctx := context.Background()

	input := newFirmOrg("Acme Management")
	got, err := repo.Create(ctx, input)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.NotEqual(t, uuid.Nil, got.ID, "should have a generated UUID")
	assert.Equal(t, "acme-management", got.Slug)
	assert.NotEmpty(t, got.Path, "path should be set")
	assert.False(t, got.CreatedAt.IsZero())
	assert.False(t, got.UpdatedAt.IsZero())
}

func TestCreate_ChildOrg(t *testing.T) {
	pool := setupTestDB(t)
	repo := org.NewPostgresOrgRepository(pool)
	ctx := context.Background()

	parent, err := repo.Create(ctx, newFirmOrg("Parent Firm"))
	require.NoError(t, err)
	require.NotNil(t, parent)

	childInput := &org.Organization{
		Type:     "firm",
		Name:     "Child Firm",
		ParentID: &parent.ID,
		Settings: map[string]any{},
	}
	child, err := repo.Create(ctx, childInput)

	require.NoError(t, err)
	require.NotNil(t, child)
	assert.True(t, len(child.Path) > len(parent.Path),
		"child path (%q) should be longer than parent path (%q)", child.Path, parent.Path)
	assert.Contains(t, child.Path, parent.Path,
		"child path should contain the parent path as a prefix")
}

// ─── FindByID ────────────────────────────────────────────────────────────────

func TestFindByID_Found(t *testing.T) {
	pool := setupTestDB(t)
	repo := org.NewPostgresOrgRepository(pool)
	ctx := context.Background()

	created, err := repo.Create(ctx, newFirmOrg("Find Me Firm"))
	require.NoError(t, err)

	got, err := repo.FindByID(ctx, created.ID)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, created.ID, got.ID)
	assert.Equal(t, "Find Me Firm", got.Name)
}

func TestFindByID_NotFound(t *testing.T) {
	pool := setupTestDB(t)
	repo := org.NewPostgresOrgRepository(pool)
	ctx := context.Background()

	got, err := repo.FindByID(ctx, uuid.New())

	require.NoError(t, err)
	assert.Nil(t, got, "should return nil for unknown ID")
}

// ─── FindBySlug ──────────────────────────────────────────────────────────────

func TestFindBySlug_Found(t *testing.T) {
	pool := setupTestDB(t)
	repo := org.NewPostgresOrgRepository(pool)
	ctx := context.Background()

	created, err := repo.Create(ctx, newFirmOrg("Slug Test Firm"))
	require.NoError(t, err)

	got, err := repo.FindBySlug(ctx, created.Slug)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, created.ID, got.ID)
	assert.Equal(t, created.Slug, got.Slug)
}

func TestFindBySlug_NotFound(t *testing.T) {
	pool := setupTestDB(t)
	repo := org.NewPostgresOrgRepository(pool)
	ctx := context.Background()

	got, err := repo.FindBySlug(ctx, "does-not-exist-slug")

	require.NoError(t, err)
	assert.Nil(t, got, "should return nil for unknown slug")
}

// ─── Update ──────────────────────────────────────────────────────────────────

func TestUpdate_ChangesName(t *testing.T) {
	pool := setupTestDB(t)
	repo := org.NewPostgresOrgRepository(pool)
	ctx := context.Background()

	created, err := repo.Create(ctx, newFirmOrg("Original Name"))
	require.NoError(t, err)

	created.Name = "Updated Name"
	updated, err := repo.Update(ctx, created)

	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, created.ID, updated.ID)
	assert.Equal(t, "Updated Name", updated.Name)
	assert.True(t, updated.UpdatedAt.After(created.CreatedAt) || updated.UpdatedAt.Equal(created.UpdatedAt),
		"updated_at should be >= created_at")
}

// ─── SoftDelete ──────────────────────────────────────────────────────────────

func TestSoftDelete_SetsDeletedAt(t *testing.T) {
	pool := setupTestDB(t)
	repo := org.NewPostgresOrgRepository(pool)
	ctx := context.Background()

	created, err := repo.Create(ctx, newFirmOrg("Delete Me Firm"))
	require.NoError(t, err)

	err = repo.SoftDelete(ctx, created.ID)
	require.NoError(t, err)

	// FindByID excludes soft-deleted rows.
	got, err := repo.FindByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Nil(t, got, "soft-deleted org should not be returned by FindByID")
}

// ─── ListChildren ────────────────────────────────────────────────────────────

func TestListChildren_ReturnsDirectChildren(t *testing.T) {
	pool := setupTestDB(t)
	repo := org.NewPostgresOrgRepository(pool)
	ctx := context.Background()

	parent, err := repo.Create(ctx, newFirmOrg("Parent For Children"))
	require.NoError(t, err)

	// Create two children.
	for _, name := range []string{"Child Alpha", "Child Beta"} {
		_, err := repo.Create(ctx, &org.Organization{
			Type:     "firm",
			Name:     name,
			ParentID: &parent.ID,
			Settings: map[string]any{},
		})
		require.NoError(t, err)
	}

	// Create an unrelated org to ensure it is not returned.
	_, err = repo.Create(ctx, newFirmOrg("Unrelated Firm"))
	require.NoError(t, err)

	children, err := repo.ListChildren(ctx, parent.ID)

	require.NoError(t, err)
	require.Len(t, children, 2, "expected exactly two direct children")
	assert.Equal(t, "Child Alpha", children[0].Name)
	assert.Equal(t, "Child Beta", children[1].Name)
}

// ─── Management ──────────────────────────────────────────────────────────────

func TestConnectManagement_LinksOrgidAndHoa(t *testing.T) {
	pool := setupTestDB(t)
	repo := org.NewPostgresOrgRepository(pool)
	ctx := context.Background()

	firm, err := repo.Create(ctx, newFirmOrg("Connect Firm"))
	require.NoError(t, err)
	hoa, err := repo.Create(ctx, newHOAOrg("Connect HOA"))
	require.NoError(t, err)

	mgmt, err := repo.ConnectManagement(ctx, firm.ID, hoa.ID)

	require.NoError(t, err)
	require.NotNil(t, mgmt)
	assert.NotEqual(t, uuid.Nil, mgmt.ID)
	assert.Equal(t, firm.ID, mgmt.FirmOrgID)
	assert.Equal(t, hoa.ID, mgmt.HOAOrgID)
	assert.Nil(t, mgmt.EndedAt, "new management link should not be ended")
	assert.False(t, mgmt.StartedAt.IsZero())
}

func TestDisconnectManagement_SetsEndedAt(t *testing.T) {
	pool := setupTestDB(t)
	repo := org.NewPostgresOrgRepository(pool)
	ctx := context.Background()

	firm, err := repo.Create(ctx, newFirmOrg("Disconnect Firm"))
	require.NoError(t, err)
	hoa, err := repo.Create(ctx, newHOAOrg("Disconnect HOA"))
	require.NoError(t, err)

	_, err = repo.ConnectManagement(ctx, firm.ID, hoa.ID)
	require.NoError(t, err)

	err = repo.DisconnectManagement(ctx, hoa.ID)
	require.NoError(t, err)

	active, err := repo.FindActiveManagement(ctx, hoa.ID)
	require.NoError(t, err)
	assert.Nil(t, active, "no active management should exist after disconnect")

	history, err := repo.ListManagementHistory(ctx, hoa.ID)
	require.NoError(t, err)
	require.Len(t, history, 1)
	assert.NotNil(t, history[0].EndedAt, "ended_at should be set after disconnect")
}

func TestConnectManagement_RejectsDoubleConnect(t *testing.T) {
	pool := setupTestDB(t)
	repo := org.NewPostgresOrgRepository(pool)
	ctx := context.Background()

	firm, err := repo.Create(ctx, newFirmOrg("Double Firm"))
	require.NoError(t, err)
	hoa, err := repo.Create(ctx, newHOAOrg("Double HOA"))
	require.NoError(t, err)

	_, err = repo.ConnectManagement(ctx, firm.ID, hoa.ID)
	require.NoError(t, err, "first connect should succeed")

	// Second connect should be rejected by the unique partial index.
	_, err = repo.ConnectManagement(ctx, firm.ID, hoa.ID)
	assert.Error(t, err, "second active connect should be rejected by unique index")
}
