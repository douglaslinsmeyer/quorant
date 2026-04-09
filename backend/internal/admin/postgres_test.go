//go:build integration

package admin_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/quorant/quorant/internal/admin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupAdminTestDB connects to the local Docker postgres and cleans admin test
// data after each test.
func setupAdminTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, "postgres://quorant:quorant@localhost:5432/quorant_dev?sslmode=disable")
	require.NoError(t, err, "connecting to test database")

	t.Cleanup(func() {
		cleanCtx := context.Background()
		pool.Exec(cleanCtx, "DELETE FROM tenant_activity_log")
		pool.Exec(cleanCtx, "DELETE FROM feature_flag_overrides")
		pool.Exec(cleanCtx, "DELETE FROM feature_flags")
		pool.Close()
	})

	return pool
}

// insertTestOrg inserts a minimal organization row and returns its UUID.
func insertTestOrg(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	orgID := uuid.New()
	slug := "admin-test-org-" + orgID.String()[:8]
	_, err := pool.Exec(context.Background(),
		`INSERT INTO organizations (id, type, name, slug, path) VALUES ($1, 'hoa', 'Admin Test HOA', $2, $3)`,
		orgID, slug, slug,
	)
	require.NoError(t, err, "inserting test organization")
	t.Cleanup(func() {
		pool.Exec(context.Background(), "DELETE FROM organizations WHERE id = $1", orgID)
	})
	return orgID
}

// ─── CreateFlag + ListFlags ───────────────────────────────────────────────────

func TestCreateFlag_AndListFlags(t *testing.T) {
	pool := setupAdminTestDB(t)
	repo := admin.NewPostgresAdminRepository(pool)
	ctx := context.Background()

	desc := "test flag description"
	input := &admin.FeatureFlag{
		Key:         "test_flag_list_" + uuid.New().String()[:8],
		Description: &desc,
		Enabled:     false,
	}

	created, err := repo.CreateFlag(ctx, input)
	require.NoError(t, err)
	require.NotNil(t, created)
	assert.NotEqual(t, uuid.Nil, created.ID)
	assert.Equal(t, input.Key, created.Key)
	assert.Equal(t, desc, *created.Description)
	assert.False(t, created.Enabled)
	assert.False(t, created.CreatedAt.IsZero())

	flags, err := repo.ListFlags(ctx)
	require.NoError(t, err)

	found := false
	for _, f := range flags {
		if f.ID == created.ID {
			found = true
			assert.Equal(t, input.Key, f.Key)
		}
	}
	assert.True(t, found, "created flag should appear in ListFlags")
}

// ─── UpdateFlag ───────────────────────────────────────────────────────────────

func TestUpdateFlag(t *testing.T) {
	pool := setupAdminTestDB(t)
	repo := admin.NewPostgresAdminRepository(pool)
	ctx := context.Background()

	input := &admin.FeatureFlag{
		Key:     "test_update_flag_" + uuid.New().String()[:8],
		Enabled: false,
	}
	created, err := repo.CreateFlag(ctx, input)
	require.NoError(t, err)

	newDesc := "updated description"
	created.Description = &newDesc
	created.Enabled = true

	updated, err := repo.UpdateFlag(ctx, created)
	require.NoError(t, err)
	require.NotNil(t, updated)

	assert.Equal(t, created.ID, updated.ID)
	assert.Equal(t, newDesc, *updated.Description)
	assert.True(t, updated.Enabled)
}

// ─── SetOverride + IsFlagEnabled ─────────────────────────────────────────────

func TestSetOverride_AndIsFlagEnabled(t *testing.T) {
	pool := setupAdminTestDB(t)
	repo := admin.NewPostgresAdminRepository(pool)
	ctx := context.Background()
	orgID := insertTestOrg(t, pool)

	// Create a flag that is globally disabled.
	flag, err := repo.CreateFlag(ctx, &admin.FeatureFlag{
		Key:     "override_test_" + uuid.New().String()[:8],
		Enabled: false,
	})
	require.NoError(t, err)

	// Without override, flag is disabled.
	enabled, err := repo.IsFlagEnabled(ctx, flag.Key, orgID)
	require.NoError(t, err)
	assert.False(t, enabled, "flag should be disabled by default")

	// Set org-level override to enabled.
	override, err := repo.SetOverride(ctx, &admin.FeatureFlagOverride{
		FlagID:  flag.ID,
		OrgID:   orgID,
		Enabled: true,
	})
	require.NoError(t, err)
	require.NotNil(t, override)
	assert.NotEqual(t, uuid.Nil, override.ID)
	assert.Equal(t, flag.ID, override.FlagID)
	assert.Equal(t, orgID, override.OrgID)
	assert.True(t, override.Enabled)

	// Override takes precedence: flag should now be enabled for this org.
	enabled, err = repo.IsFlagEnabled(ctx, flag.Key, orgID)
	require.NoError(t, err)
	assert.True(t, enabled, "override should enable flag for this org")
}

func TestSetOverride_Upsert(t *testing.T) {
	pool := setupAdminTestDB(t)
	repo := admin.NewPostgresAdminRepository(pool)
	ctx := context.Background()
	orgID := insertTestOrg(t, pool)

	flag, err := repo.CreateFlag(ctx, &admin.FeatureFlag{
		Key:     "upsert_override_" + uuid.New().String()[:8],
		Enabled: true,
	})
	require.NoError(t, err)

	// Set override to disabled.
	_, err = repo.SetOverride(ctx, &admin.FeatureFlagOverride{FlagID: flag.ID, OrgID: orgID, Enabled: false})
	require.NoError(t, err)

	// Update same override to enabled.
	updated, err := repo.SetOverride(ctx, &admin.FeatureFlagOverride{FlagID: flag.ID, OrgID: orgID, Enabled: true})
	require.NoError(t, err)
	assert.True(t, updated.Enabled, "upsert should update existing override")
}

// ─── RecordActivity + ListActivityByOrg ─────────────────────────────────────

func TestRecordActivity_AndListByOrg(t *testing.T) {
	pool := setupAdminTestDB(t)
	repo := admin.NewPostgresAdminRepository(pool)
	ctx := context.Background()
	orgID := insertTestOrg(t, pool)

	now := time.Now().UTC().Truncate(time.Second)
	input := &admin.TenantActivity{
		OrgID:       orgID,
		MetricType:  "active_users",
		Value:       42,
		PeriodStart: now.Add(-24 * time.Hour),
		PeriodEnd:   now,
	}

	recorded, err := repo.RecordActivity(ctx, input)
	require.NoError(t, err)
	require.NotNil(t, recorded)
	assert.Greater(t, recorded.ID, int64(0))
	assert.Equal(t, orgID, recorded.OrgID)
	assert.Equal(t, "active_users", recorded.MetricType)
	assert.Equal(t, int64(42), recorded.Value)

	activities, err := repo.ListActivityByOrg(ctx, orgID)
	require.NoError(t, err)
	require.Len(t, activities, 1)
	assert.Equal(t, recorded.ID, activities[0].ID)
	assert.Equal(t, "active_users", activities[0].MetricType)
}
