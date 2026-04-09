//go:build integration

package middleware_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/quorant/quorant/internal/platform/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Test DB setup ───────────────────────────────────────────────────────────

// setupRBACTestDB connects to the local Docker postgres and registers cleanup
// that removes all test-generated rows after each test.
func setupRBACTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, "postgres://quorant:quorant@localhost:5432/quorant_dev?sslmode=disable")
	require.NoError(t, err, "connecting to test database")

	t.Cleanup(func() {
		clean := context.Background()
		// Remove test data in dependency order.
		pool.Exec(clean, "DELETE FROM organizations_management WHERE firm_org_id IN (SELECT id FROM organizations WHERE slug LIKE 'test-rbac-%')")
		pool.Exec(clean, "DELETE FROM memberships WHERE org_id IN (SELECT id FROM organizations WHERE slug LIKE 'test-rbac-%')")
		pool.Exec(clean, "DELETE FROM memberships WHERE user_id IN (SELECT id FROM users WHERE email LIKE '%@rbac-test.example.com')")
		pool.Exec(clean, "DELETE FROM organizations WHERE slug LIKE 'test-rbac-%'")
		pool.Exec(clean, "DELETE FROM users WHERE email LIKE '%@rbac-test.example.com'")
		pool.Close()
	})

	return pool
}

// createTestUser inserts a minimal user row and returns its UUID.
func createTestUser(t *testing.T, pool *pgxpool.Pool, idpID, email string) uuid.UUID {
	t.Helper()
	ctx := context.Background()

	var id uuid.UUID
	err := pool.QueryRow(ctx,
		`INSERT INTO users (idp_user_id, email, display_name)
		 VALUES ($1, $2, $2)
		 ON CONFLICT (idp_user_id) DO UPDATE SET email = EXCLUDED.email
		 RETURNING id`,
		idpID, email,
	).Scan(&id)
	require.NoError(t, err, "creating test user %s", email)
	return id
}

// createTestOrg inserts an organization row and returns its UUID.
// orgType must be a valid org_type enum value: 'platform', 'firm', 'hoa'.
func createTestOrg(t *testing.T, pool *pgxpool.Pool, slug, orgType, ltreePath string) uuid.UUID {
	t.Helper()
	ctx := context.Background()

	var id uuid.UUID
	err := pool.QueryRow(ctx,
		`INSERT INTO organizations (type, name, slug, path)
		 VALUES ($1::org_type, $2, $3, $4::ltree)
		 RETURNING id`,
		orgType, slug, slug, ltreePath,
	).Scan(&id)
	require.NoError(t, err, "creating test org %s", slug)
	return id
}

// createMembership inserts a membership for the given user/org/role.
// roleName must match a seeded role name (e.g. 'homeowner', 'firm_admin').
func createMembership(t *testing.T, pool *pgxpool.Pool, userID, orgID uuid.UUID, roleName string) {
	t.Helper()
	ctx := context.Background()

	_, err := pool.Exec(ctx,
		`INSERT INTO memberships (user_id, org_id, role_id, status)
		 VALUES ($1, $2, (SELECT id FROM roles WHERE name = $3), 'active')`,
		userID, orgID, roleName,
	)
	require.NoError(t, err, "creating membership user=%s org=%s role=%s", userID, orgID, roleName)
}

// linkFirmToHOA inserts a management relationship.
func linkFirmToHOA(t *testing.T, pool *pgxpool.Pool, firmOrgID, hoaOrgID uuid.UUID) {
	t.Helper()
	ctx := context.Background()

	_, err := pool.Exec(ctx,
		`INSERT INTO organizations_management (firm_org_id, hoa_org_id)
		 VALUES ($1, $2)`,
		firmOrgID, hoaOrgID,
	)
	require.NoError(t, err, "linking firm=%s to hoa=%s", firmOrgID, hoaOrgID)
}

// ─── Integration tests ───────────────────────────────────────────────────────

// TestHasPermission_DirectMembership verifies a homeowner can read their own org.
func TestHasPermission_DirectMembership(t *testing.T) {
	pool := setupRBACTestDB(t)
	checker := middleware.NewPostgresPermissionChecker(pool)
	ctx := context.Background()

	hoaID := createTestOrg(t, pool, "test-rbac-hoa-direct", "hoa", "test_rbac_hoa_direct")
	userID := createTestUser(t, pool, "rbac-direct-001", "direct@rbac-test.example.com")
	createMembership(t, pool, userID, hoaID, "homeowner")

	allowed, err := checker.HasPermission(ctx, userID, hoaID, "org.organization.read")
	require.NoError(t, err)
	assert.True(t, allowed, "homeowner should have org.organization.read on their own HOA")
}

// TestHasPermission_NoMembership verifies that a user with no membership is denied.
func TestHasPermission_NoMembership(t *testing.T) {
	pool := setupRBACTestDB(t)
	checker := middleware.NewPostgresPermissionChecker(pool)
	ctx := context.Background()

	hoaID := createTestOrg(t, pool, "test-rbac-hoa-nomem", "hoa", "test_rbac_hoa_nomem")
	userID := createTestUser(t, pool, "rbac-nomem-001", "nomem@rbac-test.example.com")
	// No membership created.

	allowed, err := checker.HasPermission(ctx, userID, hoaID, "org.organization.read")
	require.NoError(t, err)
	assert.False(t, allowed, "user with no membership should be denied")
}

// TestHasPermission_PlatformAdmin verifies a platform_admin has permissions globally
// even when their membership is in a different org from the target.
func TestHasPermission_PlatformAdmin(t *testing.T) {
	pool := setupRBACTestDB(t)
	checker := middleware.NewPostgresPermissionChecker(pool)
	ctx := context.Background()

	// The platform_admin is a member of a firm org (any org works for global roles),
	// not the target HOA.
	adminFirmID := createTestOrg(t, pool, "test-rbac-admin-firm", "firm", "test_rbac_admin_firm")
	hoaID := createTestOrg(t, pool, "test-rbac-hoa-platform", "hoa", "test_rbac_hoa_platform")
	adminID := createTestUser(t, pool, "rbac-admin-001", "admin@rbac-test.example.com")
	createMembership(t, pool, adminID, adminFirmID, "platform_admin")

	// Admin should have admin-only permissions on the unrelated HOA.
	allowed, err := checker.HasPermission(ctx, adminID, hoaID, "admin.tenant.manage")
	require.NoError(t, err)
	assert.True(t, allowed, "platform_admin should have admin.tenant.manage globally")
}

// TestHasPermission_FirmToHOA verifies a firm_admin can access an HOA managed
// by their firm.
func TestHasPermission_FirmToHOA(t *testing.T) {
	pool := setupRBACTestDB(t)
	checker := middleware.NewPostgresPermissionChecker(pool)
	ctx := context.Background()

	firmID := createTestOrg(t, pool, "test-rbac-firm", "firm", "test_rbac_firm")
	hoaID := createTestOrg(t, pool, "test-rbac-hoa-firm", "hoa", "test_rbac_hoa_firm")
	linkFirmToHOA(t, pool, firmID, hoaID)

	firmAdminID := createTestUser(t, pool, "rbac-firmadmin-001", "firmadmin@rbac-test.example.com")
	createMembership(t, pool, firmAdminID, firmID, "firm_admin")

	// firm_admin should have read permission on the managed HOA.
	allowed, err := checker.HasPermission(ctx, firmAdminID, hoaID, "org.organization.read")
	require.NoError(t, err)
	assert.True(t, allowed, "firm_admin should have org.organization.read on managed HOA")
}

// TestHasPermission_FirmToHOA_UnmanagedDenied verifies a firm_admin cannot
// access an HOA their firm does NOT manage.
func TestHasPermission_FirmToHOA_UnmanagedDenied(t *testing.T) {
	pool := setupRBACTestDB(t)
	checker := middleware.NewPostgresPermissionChecker(pool)
	ctx := context.Background()

	firmID := createTestOrg(t, pool, "test-rbac-firm-unmanaged", "firm", "test_rbac_firm_unmanaged")
	otherHOAID := createTestOrg(t, pool, "test-rbac-hoa-unmanaged", "hoa", "test_rbac_hoa_unmanaged")
	// No management link between firmID and otherHOAID.

	firmAdminID := createTestUser(t, pool, "rbac-firmadmin-002", "firmadmin2@rbac-test.example.com")
	createMembership(t, pool, firmAdminID, firmID, "firm_admin")

	allowed, err := checker.HasPermission(ctx, firmAdminID, otherHOAID, "org.organization.read")
	require.NoError(t, err)
	assert.False(t, allowed, "firm_admin should NOT have access to an unmanaged HOA")
}

// TestHasPermission_FirmHierarchy verifies a firm_admin at a parent firm
// has access to child firms (via ltree ancestor check).
func TestHasPermission_FirmHierarchy(t *testing.T) {
	pool := setupRBACTestDB(t)
	checker := middleware.NewPostgresPermissionChecker(pool)
	ctx := context.Background()

	// Parent firm: path = "test_rbac_parent_firm"
	// Child firm: path = "test_rbac_parent_firm.test_rbac_child_firm"
	parentFirmID := createTestOrg(t, pool, "test-rbac-parent-firm", "firm", "test_rbac_parent_firm")
	_ = createTestOrg(t, pool, "test-rbac-child-firm", "firm", "test_rbac_parent_firm.test_rbac_child_firm")

	// Look up child firm by slug to get its UUID.
	var childFirmID uuid.UUID
	err := pool.QueryRow(ctx,
		`SELECT id FROM organizations WHERE slug = 'test-rbac-child-firm'`,
	).Scan(&childFirmID)
	require.NoError(t, err)

	adminID := createTestUser(t, pool, "rbac-parentadmin-001", "parentadmin@rbac-test.example.com")
	createMembership(t, pool, adminID, parentFirmID, "firm_admin")

	allowed, err := checker.HasPermission(context.Background(), adminID, childFirmID, "org.organization.read")
	require.NoError(t, err)
	assert.True(t, allowed, "firm_admin at parent firm should have access to child firm via ltree")
}
