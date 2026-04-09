//go:build integration

package doc_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/quorant/quorant/internal/doc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Test Database Setup ──────────────────────────────────────────────────────

func setupDocTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, "postgres://quorant:quorant@localhost:5432/quorant_dev?sslmode=disable")
	require.NoError(t, err, "connecting to test database")

	t.Cleanup(func() {
		cleanCtx := context.Background()
		pool.Exec(cleanCtx, "DELETE FROM documents")
		pool.Exec(cleanCtx, "DELETE FROM document_categories")
		pool.Exec(cleanCtx, "DELETE FROM memberships")
		pool.Exec(cleanCtx, "DELETE FROM units")
		pool.Exec(cleanCtx, "DELETE FROM organizations WHERE parent_id IS NOT NULL")
		pool.Exec(cleanCtx, "DELETE FROM organizations")
		pool.Exec(cleanCtx, "DELETE FROM users")
		pool.Close()
	})

	return pool
}

// docTestFixture holds shared test resources.
type docTestFixture struct {
	pool   *pgxpool.Pool
	orgID  uuid.UUID
	userID uuid.UUID
}

// setupDocFixture creates a pool with a test org and user.
func setupDocFixture(t *testing.T) docTestFixture {
	t.Helper()
	pool := setupDocTestDB(t)
	ctx := context.Background()

	var orgID uuid.UUID
	err := pool.QueryRow(ctx,
		`INSERT INTO organizations (type, name, slug, path, settings)
		 VALUES ('hoa', $1, $2, $3, '{}')
		 RETURNING id`,
		"Test HOA "+uuid.New().String(),
		"test-hoa-"+uuid.New().String(),
		"test_hoa_"+uuid.New().String(),
	).Scan(&orgID)
	require.NoError(t, err, "create test org")

	var userID uuid.UUID
	err = pool.QueryRow(ctx,
		`INSERT INTO users (idp_user_id, email, display_name)
		 VALUES ($1, $2, 'Test User')
		 RETURNING id`,
		"test-idp-"+uuid.New().String(),
		"test-"+uuid.New().String()+"@example.com",
	).Scan(&userID)
	require.NoError(t, err, "create test user")

	return docTestFixture{
		pool:   pool,
		orgID:  orgID,
		userID: userID,
	}
}

// newTestDocInput builds a minimal Document suitable for Create.
func newTestDocInput(orgID, userID uuid.UUID, title string) *doc.Document {
	return &doc.Document{
		OrgID:       orgID,
		UploadedBy:  userID,
		Title:       title,
		FileName:    "test.pdf",
		ContentType: "application/pdf",
		SizeBytes:   1024,
		Visibility:  "members",
		Metadata:    map[string]any{},
	}
}

// ─── Create + FindByID ────────────────────────────────────────────────────────

func TestCreateDocument(t *testing.T) {
	f := setupDocFixture(t)
	repo := doc.NewPostgresDocRepository(f.pool)
	ctx := context.Background()

	input := newTestDocInput(f.orgID, f.userID, "CC&Rs 2024")
	created, err := repo.Create(ctx, input)

	require.NoError(t, err)
	require.NotNil(t, created)
	assert.NotEqual(t, uuid.Nil, created.ID)
	assert.Equal(t, f.orgID, created.OrgID)
	assert.Equal(t, f.userID, created.UploadedBy)
	assert.Equal(t, "CC&Rs 2024", created.Title)
	assert.Equal(t, "test.pdf", created.FileName)
	assert.Equal(t, "application/pdf", created.ContentType)
	assert.Equal(t, int64(1024), created.SizeBytes)
	assert.Equal(t, "members", created.Visibility)
	assert.Equal(t, 1, created.VersionNumber)
	assert.True(t, created.IsCurrent)
	assert.Nil(t, created.ParentDocID)
	assert.Nil(t, created.DeletedAt)
	assert.NotEmpty(t, created.StorageKey)
	assert.False(t, created.CreatedAt.IsZero())
}

func TestFindByID_Found(t *testing.T) {
	f := setupDocFixture(t)
	repo := doc.NewPostgresDocRepository(f.pool)
	ctx := context.Background()

	input := newTestDocInput(f.orgID, f.userID, "Bylaws")
	created, err := repo.Create(ctx, input)
	require.NoError(t, err)

	found, err := repo.FindByID(ctx, created.ID)

	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, created.ID, found.ID)
	assert.Equal(t, "Bylaws", found.Title)
	assert.True(t, found.IsCurrent)
}

func TestFindByID_NotFound(t *testing.T) {
	f := setupDocFixture(t)
	repo := doc.NewPostgresDocRepository(f.pool)
	ctx := context.Background()

	found, err := repo.FindByID(ctx, uuid.New())

	require.NoError(t, err)
	assert.Nil(t, found)
}

// ─── ListByOrg ────────────────────────────────────────────────────────────────

func TestListByOrg_CurrentOnly(t *testing.T) {
	f := setupDocFixture(t)
	repo := doc.NewPostgresDocRepository(f.pool)
	ctx := context.Background()

	// Create two documents (both current).
	doc1, err := repo.Create(ctx, newTestDocInput(f.orgID, f.userID, "Alpha Doc"))
	require.NoError(t, err)
	_, err = repo.Create(ctx, newTestDocInput(f.orgID, f.userID, "Beta Doc"))
	require.NoError(t, err)

	// Create a v2 of doc1 — v1 should no longer be current.
	_, err = repo.CreateVersion(ctx, doc1.ID, newTestDocInput(f.orgID, f.userID, "Alpha Doc v2"))
	require.NoError(t, err)

	docs, err := repo.ListByOrg(ctx, f.orgID)

	require.NoError(t, err)
	// Alpha Doc v1 should be excluded (not current), v2 and Beta Doc should be present.
	assert.Len(t, docs, 2)
	for _, d := range docs {
		assert.True(t, d.IsCurrent, "ListByOrg should only return current versions")
		assert.Nil(t, d.DeletedAt)
	}
}

func TestListByOrg_ExcludesDeleted(t *testing.T) {
	f := setupDocFixture(t)
	repo := doc.NewPostgresDocRepository(f.pool)
	ctx := context.Background()

	d, err := repo.Create(ctx, newTestDocInput(f.orgID, f.userID, "To Be Deleted"))
	require.NoError(t, err)

	err = repo.SoftDelete(ctx, d.ID)
	require.NoError(t, err)

	docs, err := repo.ListByOrg(ctx, f.orgID)

	require.NoError(t, err)
	assert.Empty(t, docs)
}

// ─── SoftDelete ───────────────────────────────────────────────────────────────

func TestSoftDelete(t *testing.T) {
	f := setupDocFixture(t)
	repo := doc.NewPostgresDocRepository(f.pool)
	ctx := context.Background()

	d, err := repo.Create(ctx, newTestDocInput(f.orgID, f.userID, "Soft Delete Me"))
	require.NoError(t, err)

	err = repo.SoftDelete(ctx, d.ID)
	require.NoError(t, err)

	// FindByID should return nil for soft-deleted documents.
	found, err := repo.FindByID(ctx, d.ID)
	require.NoError(t, err)
	assert.Nil(t, found, "soft-deleted document should not be found")
}

// ─── CreateVersion ────────────────────────────────────────────────────────────

func TestCreateVersion(t *testing.T) {
	f := setupDocFixture(t)
	repo := doc.NewPostgresDocRepository(f.pool)
	ctx := context.Background()

	// Create v1.
	v1, err := repo.Create(ctx, newTestDocInput(f.orgID, f.userID, "Rulebook v1"))
	require.NoError(t, err)
	assert.Equal(t, 1, v1.VersionNumber)
	assert.True(t, v1.IsCurrent)

	// Create v2.
	v2Input := newTestDocInput(f.orgID, f.userID, "Rulebook v2")
	v2, err := repo.CreateVersion(ctx, v1.ID, v2Input)

	require.NoError(t, err)
	require.NotNil(t, v2)
	assert.Equal(t, 2, v2.VersionNumber)
	assert.True(t, v2.IsCurrent)
	require.NotNil(t, v2.ParentDocID)
	assert.Equal(t, v1.ID, *v2.ParentDocID)

	// v1 should no longer be current.
	v1Updated, err := repo.FindByID(ctx, v1.ID)
	require.NoError(t, err)
	// FindByID excludes deleted; v1 isn't deleted, just not current — check directly.
	var v1Current bool
	err = f.pool.QueryRow(ctx, `SELECT is_current FROM documents WHERE id = $1`, v1.ID).Scan(&v1Current)
	require.NoError(t, err)
	assert.False(t, v1Current, "v1 should no longer be current after creating v2")

	// FindByID on v1 should still return it (it's not deleted).
	assert.NotNil(t, v1Updated)
}

// ─── ListVersions ─────────────────────────────────────────────────────────────

func TestListVersions(t *testing.T) {
	f := setupDocFixture(t)
	repo := doc.NewPostgresDocRepository(f.pool)
	ctx := context.Background()

	v1, err := repo.Create(ctx, newTestDocInput(f.orgID, f.userID, "Rules v1"))
	require.NoError(t, err)

	v2, err := repo.CreateVersion(ctx, v1.ID, newTestDocInput(f.orgID, f.userID, "Rules v2"))
	require.NoError(t, err)

	v3, err := repo.CreateVersion(ctx, v2.ID, newTestDocInput(f.orgID, f.userID, "Rules v3"))
	require.NoError(t, err)

	// ListVersions from any doc in the chain should return all 3.
	versions, err := repo.ListVersions(ctx, v2.ID)

	require.NoError(t, err)
	require.Len(t, versions, 3, "expected 3 versions in the chain")

	// Should be sorted version_number DESC: v3, v2, v1.
	assert.Equal(t, v3.ID, versions[0].ID)
	assert.Equal(t, v2.ID, versions[1].ID)
	assert.Equal(t, v1.ID, versions[2].ID)

	// All version numbers should be correct.
	assert.Equal(t, 3, versions[0].VersionNumber)
	assert.Equal(t, 2, versions[1].VersionNumber)
	assert.Equal(t, 1, versions[2].VersionNumber)
}

func TestListVersions_SingleVersion(t *testing.T) {
	f := setupDocFixture(t)
	repo := doc.NewPostgresDocRepository(f.pool)
	ctx := context.Background()

	d, err := repo.Create(ctx, newTestDocInput(f.orgID, f.userID, "Only Doc"))
	require.NoError(t, err)

	versions, err := repo.ListVersions(ctx, d.ID)

	require.NoError(t, err)
	require.Len(t, versions, 1)
	assert.Equal(t, d.ID, versions[0].ID)
}

// ─── Categories ───────────────────────────────────────────────────────────────

func TestCreateCategory(t *testing.T) {
	f := setupDocFixture(t)
	repo := doc.NewPostgresDocRepository(f.pool)
	ctx := context.Background()

	input := &doc.DocumentCategory{
		OrgID:     f.orgID,
		Name:      "Legal Documents",
		SortOrder: 1,
	}
	created, err := repo.CreateCategory(ctx, input)

	require.NoError(t, err)
	require.NotNil(t, created)
	assert.NotEqual(t, uuid.Nil, created.ID)
	assert.Equal(t, f.orgID, created.OrgID)
	assert.Equal(t, "Legal Documents", created.Name)
	assert.Equal(t, 1, created.SortOrder)
	assert.Nil(t, created.ParentID)
	assert.False(t, created.CreatedAt.IsZero())
}

func TestCreateCategory_WithParent(t *testing.T) {
	f := setupDocFixture(t)
	repo := doc.NewPostgresDocRepository(f.pool)
	ctx := context.Background()

	parent, err := repo.CreateCategory(ctx, &doc.DocumentCategory{
		OrgID: f.orgID,
		Name:  "Root Category",
	})
	require.NoError(t, err)

	child, err := repo.CreateCategory(ctx, &doc.DocumentCategory{
		OrgID:    f.orgID,
		Name:     "Sub Category",
		ParentID: &parent.ID,
	})

	require.NoError(t, err)
	require.NotNil(t, child.ParentID)
	assert.Equal(t, parent.ID, *child.ParentID)
}

func TestListCategoriesByOrg(t *testing.T) {
	f := setupDocFixture(t)
	repo := doc.NewPostgresDocRepository(f.pool)
	ctx := context.Background()

	_, err := repo.CreateCategory(ctx, &doc.DocumentCategory{
		OrgID:     f.orgID,
		Name:      "Financial",
		SortOrder: 2,
	})
	require.NoError(t, err)

	_, err = repo.CreateCategory(ctx, &doc.DocumentCategory{
		OrgID:     f.orgID,
		Name:      "Legal",
		SortOrder: 1,
	})
	require.NoError(t, err)

	cats, err := repo.ListCategoriesByOrg(ctx, f.orgID)

	require.NoError(t, err)
	require.Len(t, cats, 2)
	// Ordered by sort_order, name: Legal (1) before Financial (2).
	assert.Equal(t, "Legal", cats[0].Name)
	assert.Equal(t, "Financial", cats[1].Name)
}

func TestDeleteCategory(t *testing.T) {
	f := setupDocFixture(t)
	repo := doc.NewPostgresDocRepository(f.pool)
	ctx := context.Background()

	cat, err := repo.CreateCategory(ctx, &doc.DocumentCategory{
		OrgID: f.orgID,
		Name:  "To Delete",
	})
	require.NoError(t, err)

	err = repo.DeleteCategory(ctx, cat.ID)
	require.NoError(t, err)

	cats, err := repo.ListCategoriesByOrg(ctx, f.orgID)
	require.NoError(t, err)
	assert.Empty(t, cats, "deleted category should not appear in list")
}

func TestUpdateCategory(t *testing.T) {
	f := setupDocFixture(t)
	repo := doc.NewPostgresDocRepository(f.pool)
	ctx := context.Background()

	cat, err := repo.CreateCategory(ctx, &doc.DocumentCategory{
		OrgID:     f.orgID,
		Name:      "Original Name",
		SortOrder: 0,
	})
	require.NoError(t, err)

	cat.Name = "Updated Name"
	cat.SortOrder = 5

	updated, err := repo.UpdateCategory(ctx, cat)

	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, "Updated Name", updated.Name)
	assert.Equal(t, 5, updated.SortOrder)
	assert.Equal(t, cat.ID, updated.ID)
}
