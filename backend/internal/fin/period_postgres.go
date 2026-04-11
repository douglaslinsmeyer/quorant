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

// PostgresAccountingPeriodRepository implements AccountingPeriodRepository using a DBTX.
type PostgresAccountingPeriodRepository struct {
	db dbpkg.DBTX
}

// NewPostgresAccountingPeriodRepository creates a new PostgresAccountingPeriodRepository
// backed by pool.
func NewPostgresAccountingPeriodRepository(pool *pgxpool.Pool) *PostgresAccountingPeriodRepository {
	return &PostgresAccountingPeriodRepository{db: pool}
}

// WithTx returns a new PostgresAccountingPeriodRepository scoped to the given
// transaction, enabling participation in a caller-managed transaction.
func (r *PostgresAccountingPeriodRepository) WithTx(tx pgx.Tx) AccountingPeriodRepository {
	return &PostgresAccountingPeriodRepository{db: tx}
}

// CreatePeriod inserts a new accounting period and returns the fully-populated row.
func (r *PostgresAccountingPeriodRepository) CreatePeriod(ctx context.Context, p *AccountingPeriod) (*AccountingPeriod, error) {
	const q = `
		INSERT INTO accounting_periods (
			org_id, fiscal_year, period_number, start_date, end_date, status,
			closed_by, closed_at
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8
		)
		RETURNING id, org_id, fiscal_year, period_number, start_date, end_date,
		          status, closed_by, closed_at, created_at`

	row := r.db.QueryRow(ctx, q,
		p.OrgID,
		p.FiscalYear,
		p.PeriodNumber,
		p.StartDate,
		p.EndDate,
		p.Status,
		p.ClosedBy,
		p.ClosedAt,
	)

	result, err := scanAccountingPeriod(row)
	if err != nil {
		return nil, fmt.Errorf("fin: CreatePeriod: %w", err)
	}
	return result, nil
}

// GetPeriodForDate returns the accounting period that contains the given date,
// or nil, nil if no period covers that date.
func (r *PostgresAccountingPeriodRepository) GetPeriodForDate(ctx context.Context, orgID uuid.UUID, date time.Time) (*AccountingPeriod, error) {
	const q = `
		SELECT id, org_id, fiscal_year, period_number, start_date, end_date,
		       status, closed_by, closed_at, created_at
		FROM accounting_periods
		WHERE org_id = $1 AND start_date <= $2 AND end_date >= $2`

	row := r.db.QueryRow(ctx, q, orgID, date)
	result, err := scanAccountingPeriod(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("fin: GetPeriodForDate: %w", err)
	}
	return result, nil
}

// ListPeriodsByFiscalYear returns all accounting periods for the given org and
// fiscal year ordered by period_number. Returns an empty (non-nil) slice when
// none exist.
func (r *PostgresAccountingPeriodRepository) ListPeriodsByFiscalYear(ctx context.Context, orgID uuid.UUID, fiscalYear int) ([]AccountingPeriod, error) {
	const q = `
		SELECT id, org_id, fiscal_year, period_number, start_date, end_date,
		       status, closed_by, closed_at, created_at
		FROM accounting_periods
		WHERE org_id = $1 AND fiscal_year = $2
		ORDER BY period_number`

	rows, err := r.db.Query(ctx, q, orgID, fiscalYear)
	if err != nil {
		return nil, fmt.Errorf("fin: ListPeriodsByFiscalYear: %w", err)
	}
	defer rows.Close()

	return collectAccountingPeriods(rows, "ListPeriodsByFiscalYear")
}

// UpdatePeriodStatus updates the status of an accounting period. If closedBy
// is non-nil, closed_by and closed_at are also set.
func (r *PostgresAccountingPeriodRepository) UpdatePeriodStatus(ctx context.Context, id uuid.UUID, status PeriodStatus, closedBy *uuid.UUID) error {
	var closedAt *time.Time
	if closedBy != nil {
		now := time.Now()
		closedAt = &now
	}

	const q = `
		UPDATE accounting_periods
		SET status = $1, closed_by = $2, closed_at = $3
		WHERE id = $4`

	tag, err := r.db.Exec(ctx, q, status, closedBy, closedAt, id)
	if err != nil {
		return fmt.Errorf("fin: UpdatePeriodStatus: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("fin: UpdatePeriodStatus: period %s not found", id)
	}
	return nil
}

// AllPeriodsClosedForYear returns true if every period in the given fiscal year
// has status 'closed'.
func (r *PostgresAccountingPeriodRepository) AllPeriodsClosedForYear(ctx context.Context, orgID uuid.UUID, fiscalYear int) (bool, error) {
	const q = `
		SELECT NOT EXISTS (
			SELECT 1 FROM accounting_periods
			WHERE org_id = $1 AND fiscal_year = $2 AND status != 'closed'
		) AND EXISTS (
			SELECT 1 FROM accounting_periods
			WHERE org_id = $1 AND fiscal_year = $2
		)`

	var allClosed bool
	err := r.db.QueryRow(ctx, q, orgID, fiscalYear).Scan(&allClosed)
	if err != nil {
		return false, fmt.Errorf("fin: AllPeriodsClosedForYear: %w", err)
	}
	return allClosed, nil
}

// ─── Scan helpers ─────────────────────────────────────────────────────────────

// scanAccountingPeriod reads a single accounting_periods row.
func scanAccountingPeriod(row pgx.Row) (*AccountingPeriod, error) {
	var p AccountingPeriod
	err := row.Scan(
		&p.ID,
		&p.OrgID,
		&p.FiscalYear,
		&p.PeriodNumber,
		&p.StartDate,
		&p.EndDate,
		&p.Status,
		&p.ClosedBy,
		&p.ClosedAt,
		&p.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// collectAccountingPeriods drains pgx.Rows into a slice of AccountingPeriod values.
func collectAccountingPeriods(rows pgx.Rows, op string) ([]AccountingPeriod, error) {
	periods := []AccountingPeriod{}
	for rows.Next() {
		var p AccountingPeriod
		if err := rows.Scan(
			&p.ID,
			&p.OrgID,
			&p.FiscalYear,
			&p.PeriodNumber,
			&p.StartDate,
			&p.EndDate,
			&p.Status,
			&p.ClosedBy,
			&p.ClosedAt,
			&p.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("fin: %s scan: %w", op, err)
		}
		periods = append(periods, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("fin: %s rows: %w", op, err)
	}
	return periods, nil
}
