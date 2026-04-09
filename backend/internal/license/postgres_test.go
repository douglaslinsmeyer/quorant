//go:build integration

package license_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/quorant/quorant/internal/license"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Test DB setup ────────────────────────────────────────────────────────────

func setupLicenseTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, "postgres://quorant:quorant@localhost:5432/quorant_dev?sslmode=disable")
	require.NoError(t, err, "connecting to test database")

	t.Cleanup(func() {
		cleanCtx := context.Background()
		pool.Exec(cleanCtx, "DELETE FROM usage_records")
		pool.Exec(cleanCtx, "DELETE FROM org_entitlement_overrides")
		pool.Exec(cleanCtx, "DELETE FROM org_subscriptions")
		pool.Exec(cleanCtx, "DELETE FROM entitlements")
		pool.Exec(cleanCtx, "DELETE FROM plans")
		pool.Exec(cleanCtx, "DELETE FROM organizations_management")
		pool.Exec(cleanCtx, "DELETE FROM organizations")
		pool.Close()
	})

	return pool
}

// createTestOrg inserts a minimal organization for use in tests.
// ltree path labels may only contain letters, digits, and underscores.
func createTestOrg(t *testing.T, pool *pgxpool.Pool, orgType, name string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	// Build a slug-like value and strip characters invalid for ltree.
	suffix := uuid.New().String()
	// Replace hyphens with underscores for the ltree path label.
	safeSuffix := strings.ReplaceAll(suffix, "-", "_")
	safeName := strings.ToLower(strings.ReplaceAll(name, " ", "_"))
	path := safeName + "_" + safeSuffix
	slug := safeName + "-" + suffix
	err := pool.QueryRow(context.Background(),
		`INSERT INTO organizations (type, name, slug, path) VALUES ($1, $2, $3, $4) RETURNING id`,
		orgType, name, slug, path,
	).Scan(&id)
	require.NoError(t, err)
	return id
}

// createManagementLink creates an active management relationship between a firm and HOA.
func createManagementLink(t *testing.T, pool *pgxpool.Pool, firmOrgID, hoaOrgID uuid.UUID) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO organizations_management (firm_org_id, hoa_org_id) VALUES ($1, $2)`,
		firmOrgID, hoaOrgID,
	)
	require.NoError(t, err)
}

// createPlanWithEntitlement creates a plan + one entitlement and returns both.
func createPlanWithEntitlement(t *testing.T, repo license.LicenseRepository, planType, featureKey, limitType string, limitValue *int64) (*license.Plan, *license.Entitlement) {
	t.Helper()
	ctx := context.Background()

	plan, err := repo.CreatePlan(ctx, &license.Plan{
		Name:       "Test Plan " + uuid.New().String()[:8],
		PlanType:   planType,
		PriceCents: 999,
		IsActive:   true,
		Metadata:   map[string]any{},
	})
	require.NoError(t, err)

	// Insert entitlement directly; no CreateEntitlement method needed per spec.
	pool := extractPool(t, repo)
	var entID uuid.UUID
	var createdAt time.Time
	err = pool.QueryRow(ctx,
		`INSERT INTO entitlements (plan_id, feature_key, limit_type, limit_value)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, created_at`,
		plan.ID, featureKey, limitType, limitValue,
	).Scan(&entID, &createdAt)
	require.NoError(t, err)

	ent := &license.Entitlement{
		ID:         entID,
		PlanID:     plan.ID,
		FeatureKey: featureKey,
		LimitType:  limitType,
		LimitValue: limitValue,
		CreatedAt:  createdAt,
	}
	return plan, ent
}

// extractPool is a helper that type-asserts to get the pool from the repo.
func extractPool(t *testing.T, repo license.LicenseRepository) *pgxpool.Pool {
	t.Helper()
	pr, ok := repo.(*license.PostgresLicenseRepository)
	require.True(t, ok, "expected *PostgresLicenseRepository")
	return pr.Pool()
}

// ─── Plan CRUD tests ──────────────────────────────────────────────────────────

func TestCreatePlan_StoresAndReturnsPlan(t *testing.T) {
	pool := setupLicenseTestDB(t)
	repo := license.NewPostgresLicenseRepository(pool)
	ctx := context.Background()

	input := &license.Plan{
		Name:       "Starter",
		PlanType:   "hoa",
		PriceCents: 1999,
		IsActive:   true,
		Metadata:   map[string]any{"tier": "starter"},
	}

	got, err := repo.CreatePlan(ctx, input)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.NotEqual(t, uuid.Nil, got.ID)
	assert.Equal(t, "Starter", got.Name)
	assert.Equal(t, "hoa", got.PlanType)
	assert.Equal(t, int64(1999), got.PriceCents)
	assert.True(t, got.IsActive)
	assert.False(t, got.CreatedAt.IsZero())
}

func TestListPlans_ReturnsAllPlans(t *testing.T) {
	pool := setupLicenseTestDB(t)
	repo := license.NewPostgresLicenseRepository(pool)
	ctx := context.Background()

	for _, name := range []string{"Plan Alpha", "Plan Beta", "Plan Gamma"} {
		_, err := repo.CreatePlan(ctx, &license.Plan{
			Name: name, PlanType: "hoa", IsActive: true, Metadata: map[string]any{},
		})
		require.NoError(t, err)
	}

	plans, err := repo.ListPlans(ctx)

	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(plans), 3, "should return at least the 3 created plans")
}

func TestListEntitlementsByPlan_ReturnsPlanEntitlements(t *testing.T) {
	pool := setupLicenseTestDB(t)
	repo := license.NewPostgresLicenseRepository(pool)
	ctx := context.Background()

	plan, err := repo.CreatePlan(ctx, &license.Plan{
		Name: "Ent Test Plan", PlanType: "hoa", IsActive: true, Metadata: map[string]any{},
	})
	require.NoError(t, err)

	_, err = pool.Exec(ctx,
		`INSERT INTO entitlements (plan_id, feature_key, limit_type) VALUES ($1, 'docs', 'boolean'), ($1, 'units', 'numeric')`,
		plan.ID,
	)
	require.NoError(t, err)

	ents, err := repo.ListEntitlementsByPlan(ctx, plan.ID)

	require.NoError(t, err)
	assert.Len(t, ents, 2)
}

// ─── Subscription tests ───────────────────────────────────────────────────────

func TestCreateSubscription_StoresSubscription(t *testing.T) {
	pool := setupLicenseTestDB(t)
	repo := license.NewPostgresLicenseRepository(pool)
	ctx := context.Background()

	orgID := createTestOrg(t, pool, "hoa", "Sub HOA")
	plan, err := repo.CreatePlan(ctx, &license.Plan{
		Name: "Sub Plan", PlanType: "hoa", IsActive: true, Metadata: map[string]any{},
	})
	require.NoError(t, err)

	sub, err := repo.CreateSubscription(ctx, &license.OrgSubscription{
		OrgID:    orgID,
		PlanID:   plan.ID,
		Status:   "active",
		StartsAt: time.Now(),
	})

	require.NoError(t, err)
	require.NotNil(t, sub)
	assert.NotEqual(t, uuid.Nil, sub.ID)
	assert.Equal(t, orgID, sub.OrgID)
	assert.Equal(t, plan.ID, sub.PlanID)
	assert.Equal(t, "active", sub.Status)
}

func TestFindActiveSubscription_ReturnsActiveSubscription(t *testing.T) {
	pool := setupLicenseTestDB(t)
	repo := license.NewPostgresLicenseRepository(pool)
	ctx := context.Background()

	orgID := createTestOrg(t, pool, "hoa", "Active Sub HOA")
	plan, err := repo.CreatePlan(ctx, &license.Plan{
		Name: "Active Plan", PlanType: "hoa", IsActive: true, Metadata: map[string]any{},
	})
	require.NoError(t, err)

	created, err := repo.CreateSubscription(ctx, &license.OrgSubscription{
		OrgID: orgID, PlanID: plan.ID, Status: "active", StartsAt: time.Now(),
	})
	require.NoError(t, err)

	got, err := repo.FindActiveSubscription(ctx, orgID)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, created.ID, got.ID)
}

func TestFindActiveSubscription_ReturnsNilWhenNone(t *testing.T) {
	pool := setupLicenseTestDB(t)
	repo := license.NewPostgresLicenseRepository(pool)
	ctx := context.Background()

	got, err := repo.FindActiveSubscription(ctx, uuid.New())

	require.NoError(t, err)
	assert.Nil(t, got)
}

// ─── Usage tests ──────────────────────────────────────────────────────────────

func TestRecordUsage_InsertsRecord(t *testing.T) {
	pool := setupLicenseTestDB(t)
	repo := license.NewPostgresLicenseRepository(pool)
	ctx := context.Background()

	orgID := createTestOrg(t, pool, "hoa", "Usage HOA")
	now := time.Now().UTC().Truncate(time.Second)
	start := now.Add(-24 * time.Hour)
	end := now

	rec, err := repo.RecordUsage(ctx, &license.UsageRecord{
		OrgID:       orgID,
		FeatureKey:  "api_calls",
		Quantity:    100,
		PeriodStart: start,
		PeriodEnd:   end,
	})

	require.NoError(t, err)
	require.NotNil(t, rec)
	assert.NotEqual(t, uuid.Nil, rec.ID)
	assert.Equal(t, int64(100), rec.Quantity)
}

func TestGetUsageByOrg_SumsQuantities(t *testing.T) {
	pool := setupLicenseTestDB(t)
	repo := license.NewPostgresLicenseRepository(pool)
	ctx := context.Background()

	orgID := createTestOrg(t, pool, "hoa", "Sum Usage HOA")
	now := time.Now().UTC().Truncate(time.Second)
	start := now.Add(-7 * 24 * time.Hour)
	end := now

	for _, qty := range []int64{50, 75, 25} {
		_, err := repo.RecordUsage(ctx, &license.UsageRecord{
			OrgID: orgID, FeatureKey: "api_calls", Quantity: qty,
			PeriodStart: start, PeriodEnd: end,
		})
		require.NoError(t, err)
	}

	total, err := repo.GetUsageByOrg(ctx, orgID, "api_calls", start, end)

	require.NoError(t, err)
	assert.Equal(t, int64(150), total)
}

// ─── Checker integration tests ────────────────────────────────────────────────

func TestCheck_Override_ReturnsOverrideValue(t *testing.T) {
	pool := setupLicenseTestDB(t)
	repo := license.NewPostgresLicenseRepository(pool)
	checker := license.NewPostgresEntitlementChecker(repo)
	ctx := context.Background()

	orgID := createTestOrg(t, pool, "hoa", "Override HOA")
	val := int64(42)
	_, err := repo.UpsertOverride(ctx, &license.EntitlementOverride{
		OrgID:      orgID,
		FeatureKey: "units",
		LimitValue: &val,
	})
	require.NoError(t, err)

	allowed, remaining, err := checker.Check(ctx, orgID, "units")

	require.NoError(t, err)
	assert.True(t, allowed)
	assert.Equal(t, 42, remaining)
}

func TestCheck_DirectSubscription_BooleanEntitlement_Allowed(t *testing.T) {
	pool := setupLicenseTestDB(t)
	repo := license.NewPostgresLicenseRepository(pool)
	checker := license.NewPostgresEntitlementChecker(repo)
	ctx := context.Background()

	orgID := createTestOrg(t, pool, "hoa", "Direct Sub HOA")
	plan, _ := createPlanWithEntitlement(t, repo, "hoa", "document_storage", "boolean", nil)

	_, err := repo.CreateSubscription(ctx, &license.OrgSubscription{
		OrgID: orgID, PlanID: plan.ID, Status: "active", StartsAt: time.Now(),
	})
	require.NoError(t, err)

	allowed, remaining, err := checker.Check(ctx, orgID, "document_storage")

	require.NoError(t, err)
	assert.True(t, allowed)
	assert.Equal(t, -1, remaining)
}

func TestCheck_DirectSubscription_NumericEntitlement_ReturnsLimit(t *testing.T) {
	pool := setupLicenseTestDB(t)
	repo := license.NewPostgresLicenseRepository(pool)
	checker := license.NewPostgresEntitlementChecker(repo)
	ctx := context.Background()

	orgID := createTestOrg(t, pool, "hoa", "Numeric HOA")
	limit := int64(50)
	plan, _ := createPlanWithEntitlement(t, repo, "hoa", "max_units", "numeric", &limit)

	_, err := repo.CreateSubscription(ctx, &license.OrgSubscription{
		OrgID: orgID, PlanID: plan.ID, Status: "active", StartsAt: time.Now(),
	})
	require.NoError(t, err)

	allowed, remaining, err := checker.Check(ctx, orgID, "max_units")

	require.NoError(t, err)
	assert.True(t, allowed)
	assert.Equal(t, 50, remaining)
}

func TestCheck_FirmBundle_HOAInheritsFirmEntitlement(t *testing.T) {
	pool := setupLicenseTestDB(t)
	repo := license.NewPostgresLicenseRepository(pool)
	checker := license.NewPostgresEntitlementChecker(repo)
	ctx := context.Background()

	firmID := createTestOrg(t, pool, "firm", "Bundle Firm")
	hoaID := createTestOrg(t, pool, "hoa", "Bundle HOA")
	createManagementLink(t, pool, firmID, hoaID)

	plan, _ := createPlanWithEntitlement(t, repo, "firm_bundle", "advanced_reporting", "boolean", nil)
	_, err := repo.CreateSubscription(ctx, &license.OrgSubscription{
		OrgID: firmID, PlanID: plan.ID, Status: "active", StartsAt: time.Now(),
	})
	require.NoError(t, err)

	allowed, remaining, err := checker.Check(ctx, hoaID, "advanced_reporting")

	require.NoError(t, err)
	assert.True(t, allowed, "HOA should inherit entitlement from managing firm's bundle")
	assert.Equal(t, -1, remaining)
}

func TestCheck_NoEntitlement_Denied(t *testing.T) {
	pool := setupLicenseTestDB(t)
	repo := license.NewPostgresLicenseRepository(pool)
	checker := license.NewPostgresEntitlementChecker(repo)
	ctx := context.Background()

	orgID := createTestOrg(t, pool, "hoa", "No Sub HOA")

	allowed, remaining, err := checker.Check(ctx, orgID, "some_feature")

	require.NoError(t, err)
	assert.False(t, allowed, "org with no subscription or override should be denied")
	assert.Equal(t, 0, remaining)
}

func TestCheck_ExpiredOverride_FallsThroughToSubscription(t *testing.T) {
	pool := setupLicenseTestDB(t)
	repo := license.NewPostgresLicenseRepository(pool)
	checker := license.NewPostgresEntitlementChecker(repo)
	ctx := context.Background()

	orgID := createTestOrg(t, pool, "hoa", "Expired Override HOA")

	// Create expired override.
	past := time.Now().Add(-1 * time.Hour)
	expiredVal := int64(99)
	_, err := repo.UpsertOverride(ctx, &license.EntitlementOverride{
		OrgID:      orgID,
		FeatureKey: "max_units",
		LimitValue: &expiredVal,
		ExpiresAt:  &past,
	})
	require.NoError(t, err)

	// Create active subscription with a different limit.
	limit := int64(10)
	plan, _ := createPlanWithEntitlement(t, repo, "hoa", "max_units", "numeric", &limit)
	_, err = repo.CreateSubscription(ctx, &license.OrgSubscription{
		OrgID: orgID, PlanID: plan.ID, Status: "active", StartsAt: time.Now(),
	})
	require.NoError(t, err)

	allowed, remaining, err := checker.Check(ctx, orgID, "max_units")

	require.NoError(t, err)
	assert.True(t, allowed, "should fall through to subscription after expired override")
	assert.Equal(t, 10, remaining, "should use subscription limit, not expired override value")
}
