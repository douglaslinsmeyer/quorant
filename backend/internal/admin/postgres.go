package admin

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresAdminRepository implements AdminRepository using pgxpool.
type PostgresAdminRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresAdminRepository creates a new PostgresAdminRepository backed by pool.
func NewPostgresAdminRepository(pool *pgxpool.Pool) *PostgresAdminRepository {
	return &PostgresAdminRepository{pool: pool}
}

// ─── Feature Flags ────────────────────────────────────────────────────────────

// CreateFlag inserts a new feature flag and returns the persisted record.
func (r *PostgresAdminRepository) CreateFlag(ctx context.Context, f *FeatureFlag) (*FeatureFlag, error) {
	const q = `
		INSERT INTO feature_flags (key, description, enabled)
		VALUES ($1, $2, $3)
		RETURNING id, key, description, enabled, created_at, updated_at`

	row := r.pool.QueryRow(ctx, q, f.Key, f.Description, f.Enabled)
	result, err := scanFlag(row)
	if err != nil {
		return nil, fmt.Errorf("admin: CreateFlag: %w", err)
	}
	return result, nil
}

// FindFlagByID looks up a feature flag by its UUID.
// Returns nil, nil if not found.
func (r *PostgresAdminRepository) FindFlagByID(ctx context.Context, id uuid.UUID) (*FeatureFlag, error) {
	const q = `
		SELECT id, key, description, enabled, created_at, updated_at
		FROM feature_flags
		WHERE id = $1`

	row := r.pool.QueryRow(ctx, q, id)
	result, err := scanFlag(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("admin: FindFlagByID: %w", err)
	}
	return result, nil
}

// ListFlags returns all feature flags ordered by key.
func (r *PostgresAdminRepository) ListFlags(ctx context.Context) ([]FeatureFlag, error) {
	const q = `
		SELECT id, key, description, enabled, created_at, updated_at
		FROM feature_flags
		ORDER BY key`

	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("admin: ListFlags: %w", err)
	}
	defer rows.Close()

	var flags []FeatureFlag
	for rows.Next() {
		f, err := scanFlag(rows)
		if err != nil {
			return nil, fmt.Errorf("admin: ListFlags scan: %w", err)
		}
		flags = append(flags, *f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("admin: ListFlags rows: %w", err)
	}
	return flags, nil
}

// UpdateFlag saves changes to an existing feature flag and returns the updated record.
func (r *PostgresAdminRepository) UpdateFlag(ctx context.Context, f *FeatureFlag) (*FeatureFlag, error) {
	const q = `
		UPDATE feature_flags
		SET description = $1,
		    enabled     = $2,
		    updated_at  = now()
		WHERE id = $3
		RETURNING id, key, description, enabled, created_at, updated_at`

	row := r.pool.QueryRow(ctx, q, f.Description, f.Enabled, f.ID)
	result, err := scanFlag(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("admin: UpdateFlag: %w", err)
	}
	return result, nil
}

// SetOverride inserts or updates an org-level override for a feature flag.
func (r *PostgresAdminRepository) SetOverride(ctx context.Context, o *FeatureFlagOverride) (*FeatureFlagOverride, error) {
	const q = `
		INSERT INTO feature_flag_overrides (flag_id, org_id, enabled)
		VALUES ($1, $2, $3)
		ON CONFLICT (flag_id, org_id) DO UPDATE
			SET enabled = EXCLUDED.enabled
		RETURNING id, flag_id, org_id, enabled, created_at`

	row := r.pool.QueryRow(ctx, q, o.FlagID, o.OrgID, o.Enabled)
	result, err := scanOverride(row)
	if err != nil {
		return nil, fmt.Errorf("admin: SetOverride: %w", err)
	}
	return result, nil
}

// ListOverridesByFlag returns all overrides for the given flag ID.
func (r *PostgresAdminRepository) ListOverridesByFlag(ctx context.Context, flagID uuid.UUID) ([]FeatureFlagOverride, error) {
	const q = `
		SELECT id, flag_id, org_id, enabled, created_at
		FROM feature_flag_overrides
		WHERE flag_id = $1`

	rows, err := r.pool.Query(ctx, q, flagID)
	if err != nil {
		return nil, fmt.Errorf("admin: ListOverridesByFlag: %w", err)
	}
	defer rows.Close()

	var overrides []FeatureFlagOverride
	for rows.Next() {
		o, err := scanOverride(rows)
		if err != nil {
			return nil, fmt.Errorf("admin: ListOverridesByFlag scan: %w", err)
		}
		overrides = append(overrides, *o)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("admin: ListOverridesByFlag rows: %w", err)
	}
	return overrides, nil
}

// IsFlagEnabled checks if a flag is enabled for an org.
// Override takes precedence over the global flag default.
// Returns false if the flag does not exist.
func (r *PostgresAdminRepository) IsFlagEnabled(ctx context.Context, flagKey string, orgID uuid.UUID) (bool, error) {
	const q = `
		SELECT COALESCE(ffo.enabled, ff.enabled)
		FROM feature_flags ff
		LEFT JOIN feature_flag_overrides ffo
		       ON ffo.flag_id = ff.id AND ffo.org_id = $2
		WHERE ff.key = $1`

	var enabled bool
	err := r.pool.QueryRow(ctx, q, flagKey, orgID).Scan(&enabled)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("admin: IsFlagEnabled: %w", err)
	}
	return enabled, nil
}

// ─── Tenant Activity ──────────────────────────────────────────────────────────

// RecordActivity inserts a new activity record and returns it.
func (r *PostgresAdminRepository) RecordActivity(ctx context.Context, a *TenantActivity) (*TenantActivity, error) {
	const q = `
		INSERT INTO tenant_activity_log (org_id, metric_type, value, period_start, period_end)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, org_id, metric_type, value, period_start, period_end, created_at`

	row := r.pool.QueryRow(ctx, q, a.OrgID, a.MetricType, a.Value, a.PeriodStart, a.PeriodEnd)
	result, err := scanActivity(row)
	if err != nil {
		return nil, fmt.Errorf("admin: RecordActivity: %w", err)
	}
	return result, nil
}

// ListActivityByOrg returns all activity records for an org, most recent first.
func (r *PostgresAdminRepository) ListActivityByOrg(ctx context.Context, orgID uuid.UUID) ([]TenantActivity, error) {
	const q = `
		SELECT id, org_id, metric_type, value, period_start, period_end, created_at
		FROM tenant_activity_log
		WHERE org_id = $1
		ORDER BY period_start DESC`

	rows, err := r.pool.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("admin: ListActivityByOrg: %w", err)
	}
	defer rows.Close()

	var activities []TenantActivity
	for rows.Next() {
		a, err := scanActivity(rows)
		if err != nil {
			return nil, fmt.Errorf("admin: ListActivityByOrg scan: %w", err)
		}
		activities = append(activities, *a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("admin: ListActivityByOrg rows: %w", err)
	}
	return activities, nil
}

// ─── Tenant listing ───────────────────────────────────────────────────────────

// ListTenants returns all organizations as simplified maps.
func (r *PostgresAdminRepository) ListTenants(ctx context.Context) ([]map[string]any, error) {
	const q = `
		SELECT id, type, name, slug, created_at
		FROM organizations
		ORDER BY created_at DESC`

	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("admin: ListTenants: %w", err)
	}
	defer rows.Close()

	var tenants []map[string]any
	for rows.Next() {
		var id uuid.UUID
		var orgType, name, slug string
		var createdAt any
		if err := rows.Scan(&id, &orgType, &name, &slug, &createdAt); err != nil {
			return nil, fmt.Errorf("admin: ListTenants scan: %w", err)
		}
		tenants = append(tenants, map[string]any{
			"id":         id,
			"type":       orgType,
			"name":       name,
			"slug":       slug,
			"created_at": createdAt,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("admin: ListTenants rows: %w", err)
	}
	return tenants, nil
}

// ─── scan helpers ─────────────────────────────────────────────────────────────

// scanFlag scans a single row into a FeatureFlag.
func scanFlag(row pgx.Row) (*FeatureFlag, error) {
	var f FeatureFlag
	err := row.Scan(&f.ID, &f.Key, &f.Description, &f.Enabled, &f.CreatedAt, &f.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &f, nil
}

// scanOverride scans a single row into a FeatureFlagOverride.
func scanOverride(row pgx.Row) (*FeatureFlagOverride, error) {
	var o FeatureFlagOverride
	err := row.Scan(&o.ID, &o.FlagID, &o.OrgID, &o.Enabled, &o.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &o, nil
}

// scanActivity scans a single row into a TenantActivity.
func scanActivity(row pgx.Row) (*TenantActivity, error) {
	var a TenantActivity
	err := row.Scan(&a.ID, &a.OrgID, &a.MetricType, &a.Value, &a.PeriodStart, &a.PeriodEnd, &a.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &a, nil
}
