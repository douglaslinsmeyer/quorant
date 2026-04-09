package license_test

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/audit"
	"github.com/quorant/quorant/internal/license"
	"github.com/quorant/quorant/internal/platform/queue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Mock repository ─────────────────────────────────────────────────────────

// mockLicenseRepo is an in-memory LicenseRepository for service unit tests.
type mockLicenseRepo struct {
	plans         map[uuid.UUID]*license.Plan
	entitlements  map[uuid.UUID][]license.Entitlement // planID → []Entitlement
	subscriptions map[uuid.UUID]*license.OrgSubscription
	overrides     map[string]*license.EntitlementOverride // "orgID:featureKey"
	usageTotals   map[string]int64                        // "orgID:featureKey"

	createErr    error
	findErr      error
	updateErr    error
}

func newMockLicenseRepo() *mockLicenseRepo {
	return &mockLicenseRepo{
		plans:         make(map[uuid.UUID]*license.Plan),
		entitlements:  make(map[uuid.UUID][]license.Entitlement),
		subscriptions: make(map[uuid.UUID]*license.OrgSubscription),
		overrides:     make(map[string]*license.EntitlementOverride),
		usageTotals:   make(map[string]int64),
	}
}

func overrideKey(orgID uuid.UUID, featureKey string) string {
	return orgID.String() + ":" + featureKey
}

// ─── Plans ────────────────────────────────────────────────────────────────────

func (m *mockLicenseRepo) CreatePlan(_ context.Context, p *license.Plan) (*license.Plan, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	p.CreatedAt = time.Now()
	p.UpdatedAt = time.Now()
	if p.Metadata == nil {
		p.Metadata = map[string]any{}
	}
	cp := *p
	m.plans[p.ID] = &cp
	return &cp, nil
}

func (m *mockLicenseRepo) FindPlanByID(_ context.Context, id uuid.UUID) (*license.Plan, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	p, ok := m.plans[id]
	if !ok {
		return nil, nil
	}
	cp := *p
	return &cp, nil
}

func (m *mockLicenseRepo) ListPlans(_ context.Context) ([]license.Plan, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	out := make([]license.Plan, 0, len(m.plans))
	for _, p := range m.plans {
		out = append(out, *p)
	}
	return out, nil
}

func (m *mockLicenseRepo) UpdatePlan(_ context.Context, p *license.Plan) (*license.Plan, error) {
	if m.updateErr != nil {
		return nil, m.updateErr
	}
	p.UpdatedAt = time.Now()
	cp := *p
	m.plans[p.ID] = &cp
	return &cp, nil
}

func (m *mockLicenseRepo) ListEntitlementsByPlan(_ context.Context, planID uuid.UUID) ([]license.Entitlement, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	return m.entitlements[planID], nil
}

// ─── Subscriptions ────────────────────────────────────────────────────────────

func (m *mockLicenseRepo) CreateSubscription(_ context.Context, s *license.OrgSubscription) (*license.OrgSubscription, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	s.CreatedAt = time.Now()
	s.UpdatedAt = time.Now()
	cp := *s
	m.subscriptions[s.OrgID] = &cp
	return &cp, nil
}

func (m *mockLicenseRepo) FindActiveSubscription(_ context.Context, orgID uuid.UUID) (*license.OrgSubscription, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	s, ok := m.subscriptions[orgID]
	if !ok {
		return nil, nil
	}
	cp := *s
	return &cp, nil
}

func (m *mockLicenseRepo) UpdateSubscription(_ context.Context, s *license.OrgSubscription) (*license.OrgSubscription, error) {
	if m.updateErr != nil {
		return nil, m.updateErr
	}
	s.UpdatedAt = time.Now()
	cp := *s
	m.subscriptions[s.OrgID] = &cp
	return &cp, nil
}

// ─── Overrides ────────────────────────────────────────────────────────────────

func (m *mockLicenseRepo) UpsertOverride(_ context.Context, o *license.EntitlementOverride) (*license.EntitlementOverride, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	if o.ID == uuid.Nil {
		o.ID = uuid.New()
	}
	o.CreatedAt = time.Now()
	cp := *o
	m.overrides[overrideKey(o.OrgID, o.FeatureKey)] = &cp
	return &cp, nil
}

func (m *mockLicenseRepo) ListOverridesByOrg(_ context.Context, orgID uuid.UUID) ([]license.EntitlementOverride, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	var out []license.EntitlementOverride
	prefix := orgID.String() + ":"
	for k, o := range m.overrides {
		if len(k) > len(prefix) && k[:len(prefix)] == prefix {
			out = append(out, *o)
		}
	}
	return out, nil
}

func (m *mockLicenseRepo) FindOverride(_ context.Context, orgID uuid.UUID, featureKey string) (*license.EntitlementOverride, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	o, ok := m.overrides[overrideKey(orgID, featureKey)]
	if !ok {
		return nil, nil
	}
	cp := *o
	return &cp, nil
}

// ─── Usage ────────────────────────────────────────────────────────────────────

func (m *mockLicenseRepo) RecordUsage(_ context.Context, r *license.UsageRecord) (*license.UsageRecord, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	r.RecordedAt = time.Now()
	cp := *r
	return &cp, nil
}

func (m *mockLicenseRepo) GetUsageByOrg(_ context.Context, orgID uuid.UUID, featureKey string, _, _ time.Time) (int64, error) {
	if m.findErr != nil {
		return 0, m.findErr
	}
	return m.usageTotals[overrideKey(orgID, featureKey)], nil
}

// ─── Entitlement resolution ───────────────────────────────────────────────────

func (m *mockLicenseRepo) FindEntitlementFromSubscription(ctx context.Context, orgID uuid.UUID, featureKey string) (*license.Entitlement, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	sub, ok := m.subscriptions[orgID]
	if !ok {
		return nil, nil
	}
	for _, e := range m.entitlements[sub.PlanID] {
		if e.FeatureKey == featureKey {
			cp := e
			return &cp, nil
		}
	}
	return nil, nil
}

func (m *mockLicenseRepo) FindEntitlementFromFirmBundle(_ context.Context, _ uuid.UUID, _ string) (*license.Entitlement, error) {
	return nil, nil
}

// ─── Test helpers ─────────────────────────────────────────────────────────────

func newTestLicenseService(repo *mockLicenseRepo) *license.LicenseService {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	checker := license.NewPostgresEntitlementChecker(repo)
	return license.NewLicenseService(repo, checker, audit.NewNoopAuditor(), queue.NewInMemoryPublisher(), logger)
}

// ─── Plan tests ───────────────────────────────────────────────────────────────

func TestCreatePlan_Success(t *testing.T) {
	repo := newMockLicenseRepo()
	svc := newTestLicenseService(repo)

	req := license.CreatePlanRequest{
		Name:     "Pro Plan",
		PlanType: "hoa",
	}

	plan, err := svc.CreatePlan(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, plan)
	assert.Equal(t, "Pro Plan", plan.Name)
	assert.Equal(t, "hoa", plan.PlanType)
	assert.NotEqual(t, uuid.Nil, plan.ID)
}

func TestCreatePlan_ValidationError(t *testing.T) {
	repo := newMockLicenseRepo()
	svc := newTestLicenseService(repo)

	_, err := svc.CreatePlan(context.Background(), license.CreatePlanRequest{
		Name:     "Bad Plan",
		PlanType: "invalid",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "plan_type")
}

func TestGetPlan_NotFound(t *testing.T) {
	repo := newMockLicenseRepo()
	svc := newTestLicenseService(repo)

	_, err := svc.GetPlan(context.Background(), uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestListPlans_ReturnsAll(t *testing.T) {
	repo := newMockLicenseRepo()
	svc := newTestLicenseService(repo)

	// Seed two plans directly.
	repo.plans[uuid.New()] = &license.Plan{Name: "Plan A", PlanType: "hoa"}
	repo.plans[uuid.New()] = &license.Plan{Name: "Plan B", PlanType: "firm"}

	plans, err := svc.ListPlans(context.Background())
	require.NoError(t, err)
	assert.Len(t, plans, 2)
}

// ─── Subscription tests ───────────────────────────────────────────────────────

func TestCreateSubscription_Success(t *testing.T) {
	repo := newMockLicenseRepo()
	svc := newTestLicenseService(repo)

	orgID := uuid.New()
	planID := uuid.New()

	sub, err := svc.CreateSubscription(context.Background(), orgID, license.CreateSubscriptionRequest{
		OrgID:  orgID,
		PlanID: planID,
	})
	require.NoError(t, err)
	require.NotNil(t, sub)
	assert.Equal(t, orgID, sub.OrgID)
	assert.Equal(t, planID, sub.PlanID)
	assert.Equal(t, "active", sub.Status)
}

func TestGetSubscription_NotFound(t *testing.T) {
	repo := newMockLicenseRepo()
	svc := newTestLicenseService(repo)

	_, err := svc.GetSubscription(context.Background(), uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// ─── CheckEntitlements tests ──────────────────────────────────────────────────

func TestCheckEntitlements_Success(t *testing.T) {
	repo := newMockLicenseRepo()
	svc := newTestLicenseService(repo)

	orgID := uuid.New()
	planID := uuid.New()

	// Seed a plan with two entitlements.
	repo.plans[planID] = &license.Plan{ID: planID, Name: "Test Plan", PlanType: "hoa"}
	repo.entitlements[planID] = []license.Entitlement{
		{ID: uuid.New(), PlanID: planID, FeatureKey: "feature.a", LimitType: "boolean"},
		{ID: uuid.New(), PlanID: planID, FeatureKey: "feature.b", LimitType: "numeric", LimitValue: ptrInt64(10)},
	}

	// Seed active subscription.
	repo.subscriptions[orgID] = &license.OrgSubscription{
		ID:       uuid.New(),
		OrgID:    orgID,
		PlanID:   planID,
		Status:   "active",
		StartsAt: time.Now(),
	}

	results, err := svc.CheckEntitlements(context.Background(), orgID)
	require.NoError(t, err)
	assert.Len(t, results, 2)

	byKey := make(map[string]license.EntitlementResult)
	for _, r := range results {
		byKey[r.FeatureKey] = r
	}

	assert.True(t, byKey["feature.a"].Allowed)
	assert.Equal(t, -1, byKey["feature.a"].Remaining)
	assert.True(t, byKey["feature.b"].Allowed)
	assert.Equal(t, 10, byKey["feature.b"].Remaining)
}

func TestCheckEntitlements_NoSubscription(t *testing.T) {
	repo := newMockLicenseRepo()
	svc := newTestLicenseService(repo)

	// Org has no subscription and no overrides → empty results.
	results, err := svc.CheckEntitlements(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Empty(t, results)
}

// ─── SetOverride tests ────────────────────────────────────────────────────────

func TestSetOverride_Success(t *testing.T) {
	repo := newMockLicenseRepo()
	svc := newTestLicenseService(repo)

	orgID := uuid.New()
	override, err := svc.SetOverride(context.Background(), orgID, license.UpsertOverrideRequest{
		OrgID:      orgID,
		FeatureKey: "feature.x",
	})
	require.NoError(t, err)
	require.NotNil(t, override)
	assert.Equal(t, "feature.x", override.FeatureKey)
	assert.Equal(t, orgID, override.OrgID)
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func ptrInt64(v int64) *int64 { return &v }
