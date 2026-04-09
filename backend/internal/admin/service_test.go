package admin_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/admin"
	"github.com/quorant/quorant/internal/audit"
	"github.com/quorant/quorant/internal/platform/queue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── mockAdminRepository ──────────────────────────────────────────────────────

type mockAdminRepository struct {
	flags       map[uuid.UUID]*admin.FeatureFlag
	overrides   map[uuid.UUID][]admin.FeatureFlagOverride // keyed by flag ID
	activities  map[uuid.UUID][]admin.TenantActivity
	tenants     []map[string]any
	createErr   error
	findErr     error
	listErr     error
	overrideErr error
	activityErr error
}

func newMockAdminRepo() *mockAdminRepository {
	return &mockAdminRepository{
		flags:      make(map[uuid.UUID]*admin.FeatureFlag),
		overrides:  make(map[uuid.UUID][]admin.FeatureFlagOverride),
		activities: make(map[uuid.UUID][]admin.TenantActivity),
	}
}

func (m *mockAdminRepository) CreateFlag(_ context.Context, f *admin.FeatureFlag) (*admin.FeatureFlag, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	f.ID = uuid.New()
	f.CreatedAt = time.Now()
	f.UpdatedAt = time.Now()
	m.flags[f.ID] = f
	return f, nil
}

func (m *mockAdminRepository) FindFlagByID(_ context.Context, id uuid.UUID) (*admin.FeatureFlag, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	f, ok := m.flags[id]
	if !ok {
		return nil, nil
	}
	return f, nil
}

func (m *mockAdminRepository) ListFlags(_ context.Context) ([]admin.FeatureFlag, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	result := make([]admin.FeatureFlag, 0, len(m.flags))
	for _, f := range m.flags {
		result = append(result, *f)
	}
	return result, nil
}

func (m *mockAdminRepository) UpdateFlag(_ context.Context, f *admin.FeatureFlag) (*admin.FeatureFlag, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	existing, ok := m.flags[f.ID]
	if !ok {
		return nil, nil
	}
	existing.Description = f.Description
	existing.Enabled = f.Enabled
	existing.UpdatedAt = time.Now()
	return existing, nil
}

func (m *mockAdminRepository) SetOverride(_ context.Context, o *admin.FeatureFlagOverride) (*admin.FeatureFlagOverride, error) {
	if m.overrideErr != nil {
		return nil, m.overrideErr
	}
	o.ID = uuid.New()
	o.CreatedAt = time.Now()
	// Replace existing override for same flag+org.
	existing := m.overrides[o.FlagID]
	filtered := existing[:0]
	for _, e := range existing {
		if e.OrgID != o.OrgID {
			filtered = append(filtered, e)
		}
	}
	m.overrides[o.FlagID] = append(filtered, *o)
	return o, nil
}

func (m *mockAdminRepository) ListOverridesByFlag(_ context.Context, flagID uuid.UUID) ([]admin.FeatureFlagOverride, error) {
	return m.overrides[flagID], nil
}

func (m *mockAdminRepository) IsFlagEnabled(_ context.Context, flagKey string, orgID uuid.UUID) (bool, error) {
	// Find flag by key.
	var flag *admin.FeatureFlag
	for _, f := range m.flags {
		if f.Key == flagKey {
			flag = f
			break
		}
	}
	if flag == nil {
		return false, nil
	}
	// Check org-level override.
	for _, o := range m.overrides[flag.ID] {
		if o.OrgID == orgID {
			return o.Enabled, nil
		}
	}
	return flag.Enabled, nil
}

func (m *mockAdminRepository) RecordActivity(_ context.Context, a *admin.TenantActivity) (*admin.TenantActivity, error) {
	if m.activityErr != nil {
		return nil, m.activityErr
	}
	a.ID = int64(len(m.activities[a.OrgID]) + 1)
	a.CreatedAt = time.Now()
	m.activities[a.OrgID] = append(m.activities[a.OrgID], *a)
	return a, nil
}

func (m *mockAdminRepository) ListActivityByOrg(_ context.Context, orgID uuid.UUID) ([]admin.TenantActivity, error) {
	return m.activities[orgID], nil
}

func (m *mockAdminRepository) ListTenants(_ context.Context) ([]map[string]any, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.tenants, nil
}

// newTestService returns an AdminService backed by a mock repository.
func newTestService(t *testing.T) (*admin.AdminService, *mockAdminRepository) {
	t.Helper()
	repo := newMockAdminRepo()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	svc := admin.NewAdminService(repo, audit.NewNoopAuditor(), queue.NewInMemoryPublisher(), logger)
	return svc, repo
}

// ─── TestCreateFlag_Success ───────────────────────────────────────────────────

func TestCreateFlag_Success(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	desc := "my description"
	req := admin.CreateFeatureFlagRequest{
		Key:         "new_feature",
		Description: &desc,
		Enabled:     true,
	}

	flag, err := svc.CreateFlag(ctx, req)

	require.NoError(t, err)
	require.NotNil(t, flag)
	assert.NotEqual(t, uuid.Nil, flag.ID)
	assert.Equal(t, "new_feature", flag.Key)
	assert.Equal(t, desc, *flag.Description)
	assert.True(t, flag.Enabled)
}

func TestCreateFlag_RepoError(t *testing.T) {
	svc, repo := newTestService(t)
	repo.createErr = errors.New("db error")
	ctx := context.Background()

	_, err := svc.CreateFlag(ctx, admin.CreateFeatureFlagRequest{Key: "fail_flag"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "db error")
}

// ─── TestIsFlagEnabled_WithOverride ──────────────────────────────────────────

func TestIsFlagEnabled_WithOverride(t *testing.T) {
	svc, repo := newTestService(t)
	ctx := context.Background()
	orgID := uuid.New()

	// Create a globally disabled flag.
	flag, err := svc.CreateFlag(ctx, admin.CreateFeatureFlagRequest{
		Key:     "override_flag",
		Enabled: false,
	})
	require.NoError(t, err)

	// Without override — disabled.
	enabled, err := svc.IsFlagEnabled(ctx, "override_flag", orgID)
	require.NoError(t, err)
	assert.False(t, enabled)

	// Set org override to enabled.
	_, err = repo.SetOverride(ctx, &admin.FeatureFlagOverride{
		FlagID:  flag.ID,
		OrgID:   orgID,
		Enabled: true,
	})
	require.NoError(t, err)

	// With override — enabled.
	enabled, err = svc.IsFlagEnabled(ctx, "override_flag", orgID)
	require.NoError(t, err)
	assert.True(t, enabled, "override should take precedence over global default")
}

// ─── TestGetTenantDashboard ───────────────────────────────────────────────────

func TestGetTenantDashboard_Basic(t *testing.T) {
	svc, repo := newTestService(t)
	ctx := context.Background()
	orgID := uuid.New()

	// Pre-load some activity.
	now := time.Now().UTC()
	repo.activities[orgID] = []admin.TenantActivity{
		{
			ID:          1,
			OrgID:       orgID,
			MetricType:  "active_users",
			Value:       15,
			PeriodStart: now.Add(-24 * time.Hour),
			PeriodEnd:   now,
			CreatedAt:   now,
		},
		{
			ID:          2,
			OrgID:       orgID,
			MetricType:  "storage_bytes",
			Value:       1024 * 1024,
			PeriodStart: now.Add(-24 * time.Hour),
			PeriodEnd:   now,
			CreatedAt:   now,
		},
	}

	dashboard, err := svc.GetTenantDashboard(ctx, orgID)

	require.NoError(t, err)
	require.NotNil(t, dashboard)
	assert.Equal(t, int64(15), dashboard.ActiveUsers)
	assert.Equal(t, int64(1024*1024), dashboard.StorageBytes)
	assert.Len(t, dashboard.RecentActivity, 2)
}

func TestGetTenantDashboard_NoActivity(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()
	orgID := uuid.New()

	dashboard, err := svc.GetTenantDashboard(ctx, orgID)

	require.NoError(t, err)
	require.NotNil(t, dashboard)
	assert.Equal(t, int64(0), dashboard.ActiveUsers)
	assert.Equal(t, int64(0), dashboard.StorageBytes)
	assert.Empty(t, dashboard.RecentActivity)
}
