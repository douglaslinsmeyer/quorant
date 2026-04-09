package license

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresLicenseRepository implements LicenseRepository using a pgxpool.
type PostgresLicenseRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresLicenseRepository creates a new PostgresLicenseRepository.
func NewPostgresLicenseRepository(pool *pgxpool.Pool) *PostgresLicenseRepository {
	return &PostgresLicenseRepository{pool: pool}
}

// Pool returns the underlying pgxpool.Pool for test helpers.
func (r *PostgresLicenseRepository) Pool() *pgxpool.Pool { return r.pool }

// ─── Plans ────────────────────────────────────────────────────────────────────

func (r *PostgresLicenseRepository) CreatePlan(ctx context.Context, p *Plan) (*Plan, error) {
	metaJSON, err := marshalJSON(p.Metadata)
	if err != nil {
		return nil, fmt.Errorf("license: CreatePlan marshal metadata: %w", err)
	}

	const q = `
		INSERT INTO plans (name, description, plan_type, price_cents, is_active, metadata)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, name, description, plan_type, price_cents, is_active, metadata, created_at, updated_at`

	row := r.pool.QueryRow(ctx, q,
		p.Name, p.Description, p.PlanType, p.PriceCents, p.IsActive, metaJSON,
	)
	result, err := scanPlan(row)
	if err != nil {
		return nil, fmt.Errorf("license: CreatePlan: %w", err)
	}
	return result, nil
}

func (r *PostgresLicenseRepository) FindPlanByID(ctx context.Context, id uuid.UUID) (*Plan, error) {
	const q = `
		SELECT id, name, description, plan_type, price_cents, is_active, metadata, created_at, updated_at
		FROM plans WHERE id = $1`

	row := r.pool.QueryRow(ctx, q, id)
	result, err := scanPlan(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("license: FindPlanByID: %w", err)
	}
	return result, nil
}

func (r *PostgresLicenseRepository) ListPlans(ctx context.Context) ([]Plan, error) {
	const q = `
		SELECT id, name, description, plan_type, price_cents, is_active, metadata, created_at, updated_at
		FROM plans ORDER BY name`

	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("license: ListPlans: %w", err)
	}
	defer rows.Close()

	var plans []Plan
	for rows.Next() {
		p, err := scanPlanRow(rows)
		if err != nil {
			return nil, fmt.Errorf("license: ListPlans scan: %w", err)
		}
		plans = append(plans, *p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("license: ListPlans rows: %w", err)
	}
	return plans, nil
}

func (r *PostgresLicenseRepository) UpdatePlan(ctx context.Context, p *Plan) (*Plan, error) {
	metaJSON, err := marshalJSON(p.Metadata)
	if err != nil {
		return nil, fmt.Errorf("license: UpdatePlan marshal metadata: %w", err)
	}

	const q = `
		UPDATE plans SET
			name        = $1,
			description = $2,
			price_cents = $3,
			is_active   = $4,
			metadata    = $5,
			updated_at  = now()
		WHERE id = $6
		RETURNING id, name, description, plan_type, price_cents, is_active, metadata, created_at, updated_at`

	row := r.pool.QueryRow(ctx, q,
		p.Name, p.Description, p.PriceCents, p.IsActive, metaJSON, p.ID,
	)
	result, err := scanPlan(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("license: UpdatePlan: plan %s not found", p.ID)
	}
	if err != nil {
		return nil, fmt.Errorf("license: UpdatePlan: %w", err)
	}
	return result, nil
}

func (r *PostgresLicenseRepository) ListEntitlementsByPlan(ctx context.Context, planID uuid.UUID) ([]Entitlement, error) {
	const q = `
		SELECT id, plan_id, feature_key, limit_type, limit_value, created_at
		FROM entitlements WHERE plan_id = $1 ORDER BY feature_key`

	rows, err := r.pool.Query(ctx, q, planID)
	if err != nil {
		return nil, fmt.Errorf("license: ListEntitlementsByPlan: %w", err)
	}
	defer rows.Close()

	var ents []Entitlement
	for rows.Next() {
		var e Entitlement
		if err := rows.Scan(&e.ID, &e.PlanID, &e.FeatureKey, &e.LimitType, &e.LimitValue, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("license: ListEntitlementsByPlan scan: %w", err)
		}
		ents = append(ents, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("license: ListEntitlementsByPlan rows: %w", err)
	}
	return ents, nil
}

// ─── Subscriptions ────────────────────────────────────────────────────────────

func (r *PostgresLicenseRepository) CreateSubscription(ctx context.Context, s *OrgSubscription) (*OrgSubscription, error) {
	const q = `
		INSERT INTO org_subscriptions (org_id, plan_id, status, starts_at, ends_at, trial_ends_at, stripe_sub_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, org_id, plan_id, status, starts_at, ends_at, trial_ends_at, stripe_sub_id, created_at, updated_at`

	row := r.pool.QueryRow(ctx, q,
		s.OrgID, s.PlanID, s.Status, s.StartsAt, s.EndsAt, s.TrialEndsAt, s.StripeSubID,
	)
	result, err := scanSubscription(row)
	if err != nil {
		return nil, fmt.Errorf("license: CreateSubscription: %w", err)
	}
	return result, nil
}

func (r *PostgresLicenseRepository) FindActiveSubscription(ctx context.Context, orgID uuid.UUID) (*OrgSubscription, error) {
	const q = `
		SELECT id, org_id, plan_id, status, starts_at, ends_at, trial_ends_at, stripe_sub_id, created_at, updated_at
		FROM org_subscriptions
		WHERE org_id = $1 AND status IN ('active', 'trial')`

	row := r.pool.QueryRow(ctx, q, orgID)
	result, err := scanSubscription(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("license: FindActiveSubscription: %w", err)
	}
	return result, nil
}

func (r *PostgresLicenseRepository) UpdateSubscription(ctx context.Context, s *OrgSubscription) (*OrgSubscription, error) {
	const q = `
		UPDATE org_subscriptions SET
			status        = $1,
			ends_at       = $2,
			trial_ends_at = $3,
			stripe_sub_id = $4,
			updated_at    = now()
		WHERE id = $5
		RETURNING id, org_id, plan_id, status, starts_at, ends_at, trial_ends_at, stripe_sub_id, created_at, updated_at`

	row := r.pool.QueryRow(ctx, q,
		s.Status, s.EndsAt, s.TrialEndsAt, s.StripeSubID, s.ID,
	)
	result, err := scanSubscription(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("license: UpdateSubscription: subscription %s not found", s.ID)
	}
	if err != nil {
		return nil, fmt.Errorf("license: UpdateSubscription: %w", err)
	}
	return result, nil
}

// ─── Overrides ────────────────────────────────────────────────────────────────

func (r *PostgresLicenseRepository) UpsertOverride(ctx context.Context, o *EntitlementOverride) (*EntitlementOverride, error) {
	const q = `
		INSERT INTO org_entitlement_overrides (org_id, feature_key, limit_value, reason, granted_by, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (org_id, feature_key) DO UPDATE SET
			limit_value = EXCLUDED.limit_value,
			reason      = EXCLUDED.reason,
			granted_by  = EXCLUDED.granted_by,
			expires_at  = EXCLUDED.expires_at
		RETURNING id, org_id, feature_key, limit_value, reason, granted_by, created_at, expires_at`

	row := r.pool.QueryRow(ctx, q,
		o.OrgID, o.FeatureKey, o.LimitValue, o.Reason, o.GrantedBy, o.ExpiresAt,
	)
	result, err := scanOverride(row)
	if err != nil {
		return nil, fmt.Errorf("license: UpsertOverride: %w", err)
	}
	return result, nil
}

func (r *PostgresLicenseRepository) ListOverridesByOrg(ctx context.Context, orgID uuid.UUID) ([]EntitlementOverride, error) {
	const q = `
		SELECT id, org_id, feature_key, limit_value, reason, granted_by, created_at, expires_at
		FROM org_entitlement_overrides WHERE org_id = $1 ORDER BY feature_key`

	rows, err := r.pool.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("license: ListOverridesByOrg: %w", err)
	}
	defer rows.Close()

	var overrides []EntitlementOverride
	for rows.Next() {
		o, err := scanOverrideRow(rows)
		if err != nil {
			return nil, fmt.Errorf("license: ListOverridesByOrg scan: %w", err)
		}
		overrides = append(overrides, *o)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("license: ListOverridesByOrg rows: %w", err)
	}
	return overrides, nil
}

// ─── Usage ────────────────────────────────────────────────────────────────────

func (r *PostgresLicenseRepository) RecordUsage(ctx context.Context, rec *UsageRecord) (*UsageRecord, error) {
	const q = `
		INSERT INTO usage_records (org_id, feature_key, quantity, period_start, period_end)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, org_id, feature_key, quantity, period_start, period_end, recorded_at`

	row := r.pool.QueryRow(ctx, q,
		rec.OrgID, rec.FeatureKey, rec.Quantity, rec.PeriodStart, rec.PeriodEnd,
	)
	var u UsageRecord
	if err := row.Scan(&u.ID, &u.OrgID, &u.FeatureKey, &u.Quantity, &u.PeriodStart, &u.PeriodEnd, &u.RecordedAt); err != nil {
		return nil, fmt.Errorf("license: RecordUsage: %w", err)
	}
	return &u, nil
}

func (r *PostgresLicenseRepository) GetUsageByOrg(ctx context.Context, orgID uuid.UUID, featureKey string, periodStart, periodEnd time.Time) (int64, error) {
	const q = `
		SELECT COALESCE(SUM(quantity), 0)
		FROM usage_records
		WHERE org_id = $1
		  AND feature_key = $2
		  AND period_start >= $3
		  AND period_end   <= $4`

	var total int64
	if err := r.pool.QueryRow(ctx, q, orgID, featureKey, periodStart, periodEnd).Scan(&total); err != nil {
		return 0, fmt.Errorf("license: GetUsageByOrg: %w", err)
	}
	return total, nil
}

// ─── Entitlement resolution ───────────────────────────────────────────────────

func (r *PostgresLicenseRepository) FindOverride(ctx context.Context, orgID uuid.UUID, featureKey string) (*EntitlementOverride, error) {
	const q = `
		SELECT id, org_id, feature_key, limit_value, reason, granted_by, created_at, expires_at
		FROM org_entitlement_overrides
		WHERE org_id = $1 AND feature_key = $2`

	row := r.pool.QueryRow(ctx, q, orgID, featureKey)
	result, err := scanOverride(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("license: FindOverride: %w", err)
	}
	return result, nil
}

func (r *PostgresLicenseRepository) FindEntitlementFromSubscription(ctx context.Context, orgID uuid.UUID, featureKey string) (*Entitlement, error) {
	const q = `
		SELECT e.id, e.plan_id, e.feature_key, e.limit_type, e.limit_value, e.created_at
		FROM entitlements e
		JOIN org_subscriptions s ON s.plan_id = e.plan_id
		WHERE s.org_id = $1
		  AND s.status IN ('active', 'trial')
		  AND e.feature_key = $2`

	row := r.pool.QueryRow(ctx, q, orgID, featureKey)
	var e Entitlement
	err := row.Scan(&e.ID, &e.PlanID, &e.FeatureKey, &e.LimitType, &e.LimitValue, &e.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("license: FindEntitlementFromSubscription: %w", err)
	}
	return &e, nil
}

func (r *PostgresLicenseRepository) FindEntitlementFromFirmBundle(ctx context.Context, hoaOrgID uuid.UUID, featureKey string) (*Entitlement, error) {
	const q = `
		SELECT e.id, e.plan_id, e.feature_key, e.limit_type, e.limit_value, e.created_at
		FROM entitlements e
		JOIN org_subscriptions s ON s.plan_id = e.plan_id
		JOIN plans p ON p.id = s.plan_id
		JOIN organizations_management om ON om.firm_org_id = s.org_id
		WHERE om.hoa_org_id = $1
		  AND om.ended_at IS NULL
		  AND s.status IN ('active', 'trial')
		  AND p.plan_type = 'firm_bundle'
		  AND e.feature_key = $2`

	row := r.pool.QueryRow(ctx, q, hoaOrgID, featureKey)
	var e Entitlement
	err := row.Scan(&e.ID, &e.PlanID, &e.FeatureKey, &e.LimitType, &e.LimitValue, &e.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("license: FindEntitlementFromFirmBundle: %w", err)
	}
	return &e, nil
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func marshalJSON(v map[string]any) ([]byte, error) {
	if v == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(v)
}

func scanPlan(row pgx.Row) (*Plan, error) {
	var p Plan
	var metaRaw []byte
	err := row.Scan(
		&p.ID, &p.Name, &p.Description, &p.PlanType,
		&p.PriceCents, &p.IsActive, &metaRaw,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if err := unmarshalMeta(metaRaw, &p.Metadata); err != nil {
		return nil, err
	}
	return &p, nil
}

// scanPlanRow scans a plan from pgx.Rows (used by ListPlans).
func scanPlanRow(rows pgx.Rows) (*Plan, error) {
	var p Plan
	var metaRaw []byte
	err := rows.Scan(
		&p.ID, &p.Name, &p.Description, &p.PlanType,
		&p.PriceCents, &p.IsActive, &metaRaw,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if err := unmarshalMeta(metaRaw, &p.Metadata); err != nil {
		return nil, err
	}
	return &p, nil
}

func scanSubscription(row pgx.Row) (*OrgSubscription, error) {
	var s OrgSubscription
	err := row.Scan(
		&s.ID, &s.OrgID, &s.PlanID, &s.Status,
		&s.StartsAt, &s.EndsAt, &s.TrialEndsAt, &s.StripeSubID,
		&s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func scanOverride(row pgx.Row) (*EntitlementOverride, error) {
	var o EntitlementOverride
	err := row.Scan(
		&o.ID, &o.OrgID, &o.FeatureKey,
		&o.LimitValue, &o.Reason, &o.GrantedBy,
		&o.CreatedAt, &o.ExpiresAt,
	)
	if err != nil {
		return nil, err
	}
	return &o, nil
}

// scanOverrideRow scans an override from pgx.Rows (used by ListOverridesByOrg).
func scanOverrideRow(rows pgx.Rows) (*EntitlementOverride, error) {
	var o EntitlementOverride
	err := rows.Scan(
		&o.ID, &o.OrgID, &o.FeatureKey,
		&o.LimitValue, &o.Reason, &o.GrantedBy,
		&o.CreatedAt, &o.ExpiresAt,
	)
	if err != nil {
		return nil, err
	}
	return &o, nil
}

func unmarshalMeta(raw []byte, dst *map[string]any) error {
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, dst); err != nil {
			return fmt.Errorf("unmarshal metadata: %w", err)
		}
	}
	if *dst == nil {
		*dst = map[string]any{}
	}
	return nil
}
