package fin

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	dbpkg "github.com/quorant/quorant/internal/platform/db"
)

// PostgresOrgAccountingConfigRepository implements OrgAccountingConfigRepository using a DBTX.
type PostgresOrgAccountingConfigRepository struct {
	db dbpkg.DBTX
}

// NewPostgresOrgAccountingConfigRepository creates a new
// PostgresOrgAccountingConfigRepository backed by pool.
func NewPostgresOrgAccountingConfigRepository(pool *pgxpool.Pool) *PostgresOrgAccountingConfigRepository {
	return &PostgresOrgAccountingConfigRepository{db: pool}
}

// WithTx returns a new PostgresOrgAccountingConfigRepository scoped to the
// given transaction, enabling participation in a caller-managed transaction.
func (r *PostgresOrgAccountingConfigRepository) WithTx(tx pgx.Tx) OrgAccountingConfigRepository {
	return &PostgresOrgAccountingConfigRepository{db: tx}
}

// CreateConfig inserts a new org accounting config and returns the fully-populated row.
func (r *PostgresOrgAccountingConfigRepository) CreateConfig(ctx context.Context, cfg *OrgAccountingConfig) (*OrgAccountingConfig, error) {
	const q = `
		INSERT INTO org_accounting_configs (
			org_id, standard, recognition_basis, fiscal_year_start,
			availability_period_days, effective_date, created_by
		) VALUES (
			$1, $2, $3, $4,
			$5, $6, $7
		)
		RETURNING id, org_id, standard, recognition_basis, fiscal_year_start,
		          availability_period_days, effective_date, created_at, created_by`

	row := r.db.QueryRow(ctx, q,
		cfg.OrgID,
		cfg.Standard,
		cfg.RecognitionBasis,
		int(cfg.FiscalYearStart),
		cfg.AvailabilityPeriodDays,
		cfg.EffectiveDate,
		cfg.CreatedBy,
	)

	result, err := scanOrgAccountingConfig(row)
	if err != nil {
		return nil, fmt.Errorf("fin: CreateConfig: %w", err)
	}
	return result, nil
}

// GetEffectiveConfig returns the most recent config that is effective on or
// before asOfDate for the given org. Returns an error if no config is found.
func (r *PostgresOrgAccountingConfigRepository) GetEffectiveConfig(ctx context.Context, orgID uuid.UUID, asOfDate time.Time) (*OrgAccountingConfig, error) {
	const q = `
		SELECT id, org_id, standard, recognition_basis, fiscal_year_start,
		       availability_period_days, effective_date, created_at, created_by
		FROM org_accounting_configs
		WHERE org_id = $1 AND effective_date <= $2
		ORDER BY effective_date DESC
		LIMIT 1`

	row := r.db.QueryRow(ctx, q, orgID, asOfDate)
	result, err := scanOrgAccountingConfig(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("fin: GetEffectiveConfig: no config for org %s at %s", orgID, asOfDate.Format("2006-01-02"))
	}
	if err != nil {
		return nil, fmt.Errorf("fin: GetEffectiveConfig: %w", err)
	}
	return result, nil
}

// ListConfigsByOrg returns all configs for the given org ordered by
// effective_date DESC. Returns an empty (non-nil) slice when none exist.
func (r *PostgresOrgAccountingConfigRepository) ListConfigsByOrg(ctx context.Context, orgID uuid.UUID) ([]OrgAccountingConfig, error) {
	const q = `
		SELECT id, org_id, standard, recognition_basis, fiscal_year_start,
		       availability_period_days, effective_date, created_at, created_by
		FROM org_accounting_configs
		WHERE org_id = $1
		ORDER BY effective_date DESC`

	rows, err := r.db.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("fin: ListConfigsByOrg: %w", err)
	}
	defer rows.Close()

	return collectOrgAccountingConfigs(rows, "ListConfigsByOrg")
}

// ─── Scan helpers ─────────────────────────────────────────────────────────────

// scanOrgAccountingConfig reads a single org_accounting_configs row.
// FiscalYearStart is stored as INTEGER in Postgres but as time.Month in Go.
func scanOrgAccountingConfig(row pgx.Row) (*OrgAccountingConfig, error) {
	var c OrgAccountingConfig
	var fiscalYearStart int
	err := row.Scan(
		&c.ID,
		&c.OrgID,
		&c.Standard,
		&c.RecognitionBasis,
		&fiscalYearStart,
		&c.AvailabilityPeriodDays,
		&c.EffectiveDate,
		&c.CreatedAt,
		&c.CreatedBy,
	)
	if err != nil {
		return nil, err
	}
	c.FiscalYearStart = time.Month(fiscalYearStart)
	return &c, nil
}

// collectOrgAccountingConfigs drains pgx.Rows into a slice of OrgAccountingConfig values.
func collectOrgAccountingConfigs(rows pgx.Rows, op string) ([]OrgAccountingConfig, error) {
	configs := []OrgAccountingConfig{}
	for rows.Next() {
		var c OrgAccountingConfig
		var fiscalYearStart int
		if err := rows.Scan(
			&c.ID,
			&c.OrgID,
			&c.Standard,
			&c.RecognitionBasis,
			&fiscalYearStart,
			&c.AvailabilityPeriodDays,
			&c.EffectiveDate,
			&c.CreatedAt,
			&c.CreatedBy,
		); err != nil {
			return nil, fmt.Errorf("fin: %s scan: %w", op, err)
		}
		c.FiscalYearStart = time.Month(fiscalYearStart)
		configs = append(configs, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("fin: %s rows: %w", op, err)
	}
	return configs, nil
}
