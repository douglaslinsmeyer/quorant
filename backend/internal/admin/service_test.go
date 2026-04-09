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
	flags            map[uuid.UUID]*admin.FeatureFlag
	overrides        map[uuid.UUID][]admin.FeatureFlagOverride // keyed by flag ID
	activities       map[uuid.UUID][]admin.TenantActivity
	tenants          []map[string]any
	userSearchResult []admin.UserSearchResult
	createErr        error
	findErr          error
	listErr          error
	overrideErr      error
	activityErr      error
	suspendErr       error
	reactivateErr    error
	searchUsersErr   error
	unlockErr        error
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

func (m *mockAdminRepository) SuspendTenant(_ context.Context, _ uuid.UUID) error {
	return m.suspendErr
}

func (m *mockAdminRepository) ReactivateTenant(_ context.Context, _ uuid.UUID) error {
	return m.reactivateErr
}

func (m *mockAdminRepository) SearchUsers(_ context.Context, _ string) ([]admin.UserSearchResult, error) {
	if m.searchUsersErr != nil {
		return nil, m.searchUsersErr
	}
	return m.userSearchResult, nil
}

func (m *mockAdminRepository) UnlockAccount(_ context.Context, _ uuid.UUID) error {
	return m.unlockErr
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

// ─── TestSuspendTenant ────────────────────────────────────────────────────────

func TestSuspendTenant_CallsRepo(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	result, err := svc.SuspendTenant(ctx, uuid.New())

	require.NoError(t, err)
	assert.Equal(t, "suspended", result["status"])
}

func TestSuspendTenant_RepoError(t *testing.T) {
	svc, repo := newTestService(t)
	repo.suspendErr = errors.New("db error")
	ctx := context.Background()

	_, err := svc.SuspendTenant(ctx, uuid.New())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "db error")
}

// ─── TestReactivateTenant ─────────────────────────────────────────────────────

func TestReactivateTenant_CallsRepo(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	result, err := svc.ReactivateTenant(ctx, uuid.New())

	require.NoError(t, err)
	assert.Equal(t, "active", result["status"])
}

// ─── TestSearchUsers ──────────────────────────────────────────────────────────

func TestSearchUsers_ReturnsResults(t *testing.T) {
	svc, repo := newTestService(t)
	ctx := context.Background()
	repo.userSearchResult = []admin.UserSearchResult{
		{ID: uuid.New(), Email: "alice@example.com", DisplayName: "Alice", IsActive: true},
		{ID: uuid.New(), Email: "bob@example.com", DisplayName: "Bob", IsActive: true},
	}

	results, err := svc.SearchUsers(ctx, "alice")

	require.NoError(t, err)
	assert.Len(t, results, 2)
	assert.Equal(t, "Alice", results[0].DisplayName)
}

func TestSearchUsers_EmptyQuery_ReturnsAll(t *testing.T) {
	svc, repo := newTestService(t)
	ctx := context.Background()
	repo.userSearchResult = []admin.UserSearchResult{
		{ID: uuid.New(), Email: "user@example.com", DisplayName: "User One", IsActive: true},
	}

	results, err := svc.SearchUsers(ctx, "")

	require.NoError(t, err)
	assert.Len(t, results, 1)
}

func TestSearchUsers_RepoError(t *testing.T) {
	svc, repo := newTestService(t)
	repo.searchUsersErr = errors.New("db error")
	ctx := context.Background()

	_, err := svc.SearchUsers(ctx, "query")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "db error")
}

// ─── TestUnlockAccount ────────────────────────────────────────────────────────

func TestUnlockAccount_CallsRepoAndAuditor(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	result, err := svc.UnlockAccount(ctx, uuid.New())

	require.NoError(t, err)
	assert.Equal(t, "unlocked", result["status"])
}

func TestUnlockAccount_RepoError(t *testing.T) {
	svc, repo := newTestService(t)
	repo.unlockErr = errors.New("db error")
	ctx := context.Background()

	_, err := svc.UnlockAccount(ctx, uuid.New())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "db error")
}

// ─── TestZitadelStubs ─────────────────────────────────────────────────────────

func TestStartImpersonation_ReturnsUnprocessable(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	result, err := svc.StartImpersonation(ctx, uuid.New())

	assert.Nil(t, result)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not yet implemented")
}

func TestStopImpersonation_ReturnsUnprocessable(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	result, err := svc.StopImpersonation(ctx)

	assert.Nil(t, result)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not yet implemented")
}

func TestResetPassword_ReturnsUnprocessable(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	result, err := svc.ResetPassword(ctx, uuid.New())

	assert.Nil(t, result)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not yet implemented")
}
