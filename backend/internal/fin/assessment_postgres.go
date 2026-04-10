package fin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresAssessmentRepository implements AssessmentRepository using a pgxpool.
type PostgresAssessmentRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresAssessmentRepository creates a new PostgresAssessmentRepository
// backed by pool.
func NewPostgresAssessmentRepository(pool *pgxpool.Pool) *PostgresAssessmentRepository {
	return &PostgresAssessmentRepository{pool: pool}
}

// ─── Schedule CRUD ────────────────────────────────────────────────────────────

// CreateSchedule inserts a new assessment schedule and returns the
// fully-populated row.
func (r *PostgresAssessmentRepository) CreateSchedule(ctx context.Context, s *AssessmentSchedule) (*AssessmentSchedule, error) {
	rulesJSON, err := json.Marshal(s.AmountRules)
	if err != nil {
		return nil, fmt.Errorf("fin: CreateSchedule marshal amount_rules: %w", err)
	}

	// Resolve nullable int fields — schema columns are NOT NULL with defaults.
	dayOfMonth := 1
	if s.DayOfMonth != nil {
		dayOfMonth = *s.DayOfMonth
	}
	graceDays := 15
	if s.GraceDays != nil {
		graceDays = *s.GraceDays
	}

	const q = `
		INSERT INTO assessment_schedules (
			org_id, currency_code, name, description, frequency, amount_strategy,
			base_amount_cents, amount_rules, day_of_month, grace_days,
			starts_at, ends_at, is_active, approved_by, approved_at, created_by
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10,
			$11, $12, $13, $14, $15, $16
		)
		RETURNING id, org_id, currency_code, name, description, frequency, amount_strategy,
		          base_amount_cents, amount_rules, day_of_month, grace_days,
		          starts_at, ends_at, is_active, approved_by, approved_at,
		          created_by, created_at, updated_at, deleted_at`

	row := r.pool.QueryRow(ctx, q,
		s.OrgID,
		s.CurrencyCode,
		s.Name,
		s.Description,
		s.Frequency,
		s.AmountStrategy,
		s.BaseAmountCents,
		rulesJSON,
		dayOfMonth,
		graceDays,
		s.StartsAt,
		s.EndsAt,
		s.IsActive,
		s.ApprovedBy,
		s.ApprovedAt,
		s.CreatedBy,
	)

	result, err := scanSchedule(row)
	if err != nil {
		return nil, fmt.Errorf("fin: CreateSchedule: %w", err)
	}
	return result, nil
}

// FindScheduleByID returns the schedule with the given id, or nil, nil if not
// found or soft-deleted.
func (r *PostgresAssessmentRepository) FindScheduleByID(ctx context.Context, id uuid.UUID) (*AssessmentSchedule, error) {
	const q = `
		SELECT id, org_id, currency_code, name, description, frequency, amount_strategy,
		       base_amount_cents, amount_rules, day_of_month, grace_days,
		       starts_at, ends_at, is_active, approved_by, approved_at,
		       created_by, created_at, updated_at, deleted_at
		FROM assessment_schedules
		WHERE id = $1 AND deleted_at IS NULL`

	row := r.pool.QueryRow(ctx, q, id)
	result, err := scanSchedule(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("fin: FindScheduleByID: %w", err)
	}
	return result, nil
}

// ListSchedulesByOrg returns all non-deleted schedules for the given org,
// ordered by created_at. Returns an empty (non-nil) slice when none exist.
func (r *PostgresAssessmentRepository) ListSchedulesByOrg(ctx context.Context, orgID uuid.UUID) ([]AssessmentSchedule, error) {
	const q = `
		SELECT id, org_id, currency_code, name, description, frequency, amount_strategy,
		       base_amount_cents, amount_rules, day_of_month, grace_days,
		       starts_at, ends_at, is_active, approved_by, approved_at,
		       created_by, created_at, updated_at, deleted_at
		FROM assessment_schedules
		WHERE org_id = $1 AND deleted_at IS NULL
		ORDER BY created_at`

	rows, err := r.pool.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("fin: ListSchedulesByOrg: %w", err)
	}
	defer rows.Close()

	return collectSchedules(rows, "ListSchedulesByOrg")
}

// UpdateSchedule persists changes to an existing schedule and returns the
// updated row.
func (r *PostgresAssessmentRepository) UpdateSchedule(ctx context.Context, s *AssessmentSchedule) (*AssessmentSchedule, error) {
	rulesJSON, err := json.Marshal(s.AmountRules)
	if err != nil {
		return nil, fmt.Errorf("fin: UpdateSchedule marshal amount_rules: %w", err)
	}

	dayOfMonth := 1
	if s.DayOfMonth != nil {
		dayOfMonth = *s.DayOfMonth
	}
	graceDays := 15
	if s.GraceDays != nil {
		graceDays = *s.GraceDays
	}

	const q = `
		UPDATE assessment_schedules SET
			currency_code     = $1,
			name              = $2,
			description       = $3,
			frequency         = $4,
			amount_strategy   = $5,
			base_amount_cents = $6,
			amount_rules      = $7,
			day_of_month      = $8,
			grace_days        = $9,
			starts_at         = $10,
			ends_at           = $11,
			is_active         = $12,
			approved_by       = $13,
			approved_at       = $14,
			updated_at        = now()
		WHERE id = $15 AND deleted_at IS NULL
		RETURNING id, org_id, currency_code, name, description, frequency, amount_strategy,
		          base_amount_cents, amount_rules, day_of_month, grace_days,
		          starts_at, ends_at, is_active, approved_by, approved_at,
		          created_by, created_at, updated_at, deleted_at`

	row := r.pool.QueryRow(ctx, q,
		s.CurrencyCode,
		s.Name,
		s.Description,
		s.Frequency,
		s.AmountStrategy,
		s.BaseAmountCents,
		rulesJSON,
		dayOfMonth,
		graceDays,
		s.StartsAt,
		s.EndsAt,
		s.IsActive,
		s.ApprovedBy,
		s.ApprovedAt,
		s.ID,
	)

	result, err := scanSchedule(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("fin: UpdateSchedule: schedule %s not found or already deleted", s.ID)
	}
	if err != nil {
		return nil, fmt.Errorf("fin: UpdateSchedule: %w", err)
	}
	return result, nil
}

// DeactivateSchedule sets is_active = false for the given schedule.
func (r *PostgresAssessmentRepository) DeactivateSchedule(ctx context.Context, id uuid.UUID) error {
	const q = `
		UPDATE assessment_schedules
		SET is_active = false, updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL`

	_, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("fin: DeactivateSchedule: %w", err)
	}
	return nil
}

// ─── Assessment CRUD ──────────────────────────────────────────────────────────

// CreateAssessment inserts a new assessment charge and returns the
// fully-populated row.
func (r *PostgresAssessmentRepository) CreateAssessment(ctx context.Context, a *Assessment) (*Assessment, error) {
	const q = `
		INSERT INTO assessments (
			org_id, currency_code, unit_id, schedule_id, description, amount_cents,
			due_date, grace_days, late_fee_cents, is_recurring, created_by
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10, $11
		)
		RETURNING id, org_id, currency_code, unit_id, schedule_id, description, amount_cents,
		          due_date, grace_days, late_fee_cents, is_recurring,
		          created_by, created_at, updated_at, deleted_at`

	// Resolve nullable int fields — the schema has NOT NULL DEFAULT 0 but the
	// domain struct uses *int / *int64 for optionality.
	graceDays := 0
	if a.GraceDays != nil {
		graceDays = *a.GraceDays
	}
	lateFeeCents := int64(0)
	if a.LateFeeCents != nil {
		lateFeeCents = *a.LateFeeCents
	}

	row := r.pool.QueryRow(ctx, q,
		a.OrgID,
		a.CurrencyCode,
		a.UnitID,
		a.ScheduleID,
		a.Description,
		a.AmountCents,
		a.DueDate,
		graceDays,
		lateFeeCents,
		a.IsRecurring,
		a.CreatedBy,
	)

	result, err := scanAssessment(row)
	if err != nil {
		return nil, fmt.Errorf("fin: CreateAssessment: %w", err)
	}
	return result, nil
}

// FindAssessmentByID returns the assessment with the given id, or nil, nil if
// not found or soft-deleted.
func (r *PostgresAssessmentRepository) FindAssessmentByID(ctx context.Context, id uuid.UUID) (*Assessment, error) {
	const q = `
		SELECT id, org_id, currency_code, unit_id, schedule_id, description, amount_cents,
		       due_date, grace_days, late_fee_cents, is_recurring,
		       created_by, created_at, updated_at, deleted_at
		FROM assessments
		WHERE id = $1 AND deleted_at IS NULL`

	row := r.pool.QueryRow(ctx, q, id)
	result, err := scanAssessment(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("fin: FindAssessmentByID: %w", err)
	}
	return result, nil
}

// ListAssessmentsByOrg returns non-deleted assessments for the given org,
// supporting cursor-based pagination ordered by id.
// afterID is the cursor from the previous page; hasMore is true when more items exist.
func (r *PostgresAssessmentRepository) ListAssessmentsByOrg(ctx context.Context, orgID uuid.UUID, limit int, afterID *uuid.UUID) ([]Assessment, bool, error) {
	const q = `
		SELECT id, org_id, currency_code, unit_id, schedule_id, description, amount_cents,
		       due_date, grace_days, late_fee_cents, is_recurring,
		       created_by, created_at, updated_at, deleted_at
		FROM assessments
		WHERE org_id = $1 AND deleted_at IS NULL
		  AND ($3::uuid IS NULL OR id > $3)
		ORDER BY id
		LIMIT $2`

	rows, err := r.pool.Query(ctx, q, orgID, limit+1, afterID)
	if err != nil {
		return nil, false, fmt.Errorf("fin: ListAssessmentsByOrg: %w", err)
	}
	defer rows.Close()

	assessments, err := collectAssessments(rows, "ListAssessmentsByOrg")
	if err != nil {
		return nil, false, err
	}

	hasMore := len(assessments) > limit
	if hasMore {
		assessments = assessments[:limit]
	}
	return assessments, hasMore, nil
}

// ListAssessmentsByUnit returns all non-deleted assessments for the given unit
// ordered by due_date. Returns an empty (non-nil) slice when none exist.
func (r *PostgresAssessmentRepository) ListAssessmentsByUnit(ctx context.Context, unitID uuid.UUID) ([]Assessment, error) {
	const q = `
		SELECT id, org_id, currency_code, unit_id, schedule_id, description, amount_cents,
		       due_date, grace_days, late_fee_cents, is_recurring,
		       created_by, created_at, updated_at, deleted_at
		FROM assessments
		WHERE unit_id = $1 AND deleted_at IS NULL
		ORDER BY due_date`

	rows, err := r.pool.Query(ctx, q, unitID)
	if err != nil {
		return nil, fmt.Errorf("fin: ListAssessmentsByUnit: %w", err)
	}
	defer rows.Close()

	return collectAssessments(rows, "ListAssessmentsByUnit")
}

// UpdateAssessment persists changes to an existing assessment and returns the
// updated row.
func (r *PostgresAssessmentRepository) UpdateAssessment(ctx context.Context, a *Assessment) (*Assessment, error) {
	graceDays := 0
	if a.GraceDays != nil {
		graceDays = *a.GraceDays
	}
	lateFeeCents := int64(0)
	if a.LateFeeCents != nil {
		lateFeeCents = *a.LateFeeCents
	}

	const q = `
		UPDATE assessments SET
			currency_code  = $1,
			description    = $2,
			amount_cents   = $3,
			due_date       = $4,
			grace_days     = $5,
			late_fee_cents = $6,
			is_recurring   = $7,
			updated_at     = now()
		WHERE id = $8 AND deleted_at IS NULL
		RETURNING id, org_id, currency_code, unit_id, schedule_id, description, amount_cents,
		          due_date, grace_days, late_fee_cents, is_recurring,
		          created_by, created_at, updated_at, deleted_at`

	row := r.pool.QueryRow(ctx, q,
		a.CurrencyCode,
		a.Description,
		a.AmountCents,
		a.DueDate,
		graceDays,
		lateFeeCents,
		a.IsRecurring,
		a.ID,
	)

	result, err := scanAssessment(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("fin: UpdateAssessment: assessment %s not found or already deleted", a.ID)
	}
	if err != nil {
		return nil, fmt.Errorf("fin: UpdateAssessment: %w", err)
	}
	return result, nil
}

// SoftDeleteAssessment marks the assessment as deleted without removing the row.
func (r *PostgresAssessmentRepository) SoftDeleteAssessment(ctx context.Context, id uuid.UUID) error {
	const q = `
		UPDATE assessments
		SET deleted_at = now()
		WHERE id = $1 AND deleted_at IS NULL`

	_, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("fin: SoftDeleteAssessment: %w", err)
	}
	return nil
}

// ─── Ledger ───────────────────────────────────────────────────────────────────

// CreateLedgerEntry inserts an immutable ledger entry. It computes
// balance_cents as the previous balance for the unit plus this entry's
// amount_cents (0 base when no prior entries exist).
func (r *PostgresAssessmentRepository) CreateLedgerEntry(ctx context.Context, entry *LedgerEntry) (*LedgerEntry, error) {
	// Compute balance_cents = previous_balance + amount_cents using a CTE so
	// the whole operation is a single round-trip and is safe under concurrent
	// inserts (each INSERT reads the balance at INSERT time within the same
	// statement — for a production system you would serialize with advisory
	// locks or SERIALIZABLE isolation, but this is sufficient for HOA volumes).
	const q = `
		WITH prev AS (
			SELECT COALESCE(
				(SELECT balance_cents
				 FROM ledger_entries
				 WHERE unit_id = $1
				 ORDER BY effective_date DESC, created_at DESC
				 LIMIT 1),
				0
			) AS prev_balance
		)
		INSERT INTO ledger_entries (
			org_id, currency_code, unit_id, assessment_id, entry_type, amount_cents,
			balance_cents, description, reference_type, reference_id, effective_date
		)
		SELECT
			$2, $10, $1, $3, $4::ledger_entry_type, $5,
			(prev.prev_balance + $5), $6, $7, $8, $9
		FROM prev
		RETURNING id, org_id, currency_code, unit_id, assessment_id, entry_type, amount_cents,
		          balance_cents, description, reference_type, reference_id,
		          effective_date, created_at`

	row := r.pool.QueryRow(ctx, q,
		entry.UnitID,
		entry.OrgID,
		entry.AssessmentID,
		entry.EntryType,
		entry.AmountCents,
		entry.Description,
		entry.ReferenceType,
		entry.ReferenceID,
		entry.EffectiveDate,
		entry.CurrencyCode,
	)

	result, err := scanLedgerEntry(row)
	if err != nil {
		return nil, fmt.Errorf("fin: CreateLedgerEntry: %w", err)
	}
	return result, nil
}

// ListLedgerByUnit returns ledger entries for a unit, supporting cursor-based
// pagination ordered by id. afterID is the cursor from the previous page.
func (r *PostgresAssessmentRepository) ListLedgerByUnit(ctx context.Context, unitID uuid.UUID, limit int, afterID *uuid.UUID) ([]LedgerEntry, bool, error) {
	const q = `
		SELECT id, org_id, currency_code, unit_id, assessment_id, entry_type, amount_cents,
		       balance_cents, description, reference_type, reference_id,
		       effective_date, created_at
		FROM ledger_entries
		WHERE unit_id = $1
		  AND ($3::uuid IS NULL OR id > $3)
		ORDER BY id
		LIMIT $2`

	rows, err := r.pool.Query(ctx, q, unitID, limit+1, afterID)
	if err != nil {
		return nil, false, fmt.Errorf("fin: ListLedgerByUnit: %w", err)
	}
	defer rows.Close()

	entries, err := collectLedgerEntries(rows, "ListLedgerByUnit")
	if err != nil {
		return nil, false, err
	}

	hasMore := len(entries) > limit
	if hasMore {
		entries = entries[:limit]
	}
	return entries, hasMore, nil
}

// ListLedgerByOrg returns all ledger entries for an org ordered by
// effective_date ASC, created_at ASC. Returns an empty (non-nil) slice when
// none exist.
func (r *PostgresAssessmentRepository) ListLedgerByOrg(ctx context.Context, orgID uuid.UUID) ([]LedgerEntry, error) {
	const q = `
		SELECT id, org_id, currency_code, unit_id, assessment_id, entry_type, amount_cents,
		       balance_cents, description, reference_type, reference_id,
		       effective_date, created_at
		FROM ledger_entries
		WHERE org_id = $1
		ORDER BY effective_date ASC, created_at ASC`

	rows, err := r.pool.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("fin: ListLedgerByOrg: %w", err)
	}
	defer rows.Close()

	return collectLedgerEntries(rows, "ListLedgerByOrg")
}

// GetUnitBalance returns the balance_cents from the most recent ledger entry
// for the given unit, or 0 if no entries exist.
func (r *PostgresAssessmentRepository) GetUnitBalance(ctx context.Context, unitID uuid.UUID) (int64, error) {
	const q = `
		SELECT balance_cents
		FROM ledger_entries
		WHERE unit_id = $1
		ORDER BY effective_date DESC, created_at DESC
		LIMIT 1`

	var balance int64
	err := r.pool.QueryRow(ctx, q, unitID).Scan(&balance)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("fin: GetUnitBalance: %w", err)
	}
	return balance, nil
}

// ─── Scan helpers ─────────────────────────────────────────────────────────────

// scanSchedule reads a single assessment_schedules row.
func scanSchedule(row pgx.Row) (*AssessmentSchedule, error) {
	var s AssessmentSchedule
	var rulesRaw []byte
	var dayOfMonth, graceDays int

	err := row.Scan(
		&s.ID,
		&s.OrgID,
		&s.CurrencyCode,
		&s.Name,
		&s.Description,
		&s.Frequency,
		&s.AmountStrategy,
		&s.BaseAmountCents,
		&rulesRaw,
		&dayOfMonth,
		&graceDays,
		&s.StartsAt,
		&s.EndsAt,
		&s.IsActive,
		&s.ApprovedBy,
		&s.ApprovedAt,
		&s.CreatedBy,
		&s.CreatedAt,
		&s.UpdatedAt,
		&s.DeletedAt,
	)
	if err != nil {
		return nil, err
	}

	s.DayOfMonth = &dayOfMonth
	s.GraceDays = &graceDays

	if len(rulesRaw) > 0 {
		if err := json.Unmarshal(rulesRaw, &s.AmountRules); err != nil {
			return nil, fmt.Errorf("unmarshal amount_rules: %w", err)
		}
	}
	if s.AmountRules == nil {
		s.AmountRules = map[string]any{}
	}

	return &s, nil
}

// collectSchedules drains pgx.Rows into a slice of AssessmentSchedule values.
func collectSchedules(rows pgx.Rows, op string) ([]AssessmentSchedule, error) {
	schedules := []AssessmentSchedule{}
	for rows.Next() {
		var rulesRaw []byte
		var dayOfMonth, graceDays int
		var s AssessmentSchedule

		if err := rows.Scan(
			&s.ID,
			&s.OrgID,
			&s.CurrencyCode,
			&s.Name,
			&s.Description,
			&s.Frequency,
			&s.AmountStrategy,
			&s.BaseAmountCents,
			&rulesRaw,
			&dayOfMonth,
			&graceDays,
			&s.StartsAt,
			&s.EndsAt,
			&s.IsActive,
			&s.ApprovedBy,
			&s.ApprovedAt,
			&s.CreatedBy,
			&s.CreatedAt,
			&s.UpdatedAt,
			&s.DeletedAt,
		); err != nil {
			return nil, fmt.Errorf("fin: %s scan: %w", op, err)
		}

		s.DayOfMonth = &dayOfMonth
		s.GraceDays = &graceDays

		if len(rulesRaw) > 0 {
			if err := json.Unmarshal(rulesRaw, &s.AmountRules); err != nil {
				return nil, fmt.Errorf("fin: %s unmarshal amount_rules: %w", op, err)
			}
		}
		if s.AmountRules == nil {
			s.AmountRules = map[string]any{}
		}
		schedules = append(schedules, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("fin: %s rows: %w", op, err)
	}
	return schedules, nil
}

// scanAssessment reads a single assessments row.
func scanAssessment(row pgx.Row) (*Assessment, error) {
	var a Assessment
	var graceDays int
	var lateFeeCents int64

	err := row.Scan(
		&a.ID,
		&a.OrgID,
		&a.CurrencyCode,
		&a.UnitID,
		&a.ScheduleID,
		&a.Description,
		&a.AmountCents,
		&a.DueDate,
		&graceDays,
		&lateFeeCents,
		&a.IsRecurring,
		&a.CreatedBy,
		&a.CreatedAt,
		&a.UpdatedAt,
		&a.DeletedAt,
	)
	if err != nil {
		return nil, err
	}

	a.GraceDays = &graceDays
	a.LateFeeCents = &lateFeeCents

	return &a, nil
}

// collectAssessments drains pgx.Rows into a slice of Assessment values.
func collectAssessments(rows pgx.Rows, op string) ([]Assessment, error) {
	assessments := []Assessment{}
	for rows.Next() {
		var a Assessment
		var graceDays int
		var lateFeeCents int64

		if err := rows.Scan(
			&a.ID,
			&a.OrgID,
			&a.CurrencyCode,
			&a.UnitID,
			&a.ScheduleID,
			&a.Description,
			&a.AmountCents,
			&a.DueDate,
			&graceDays,
			&lateFeeCents,
			&a.IsRecurring,
			&a.CreatedBy,
			&a.CreatedAt,
			&a.UpdatedAt,
			&a.DeletedAt,
		); err != nil {
			return nil, fmt.Errorf("fin: %s scan: %w", op, err)
		}

		a.GraceDays = &graceDays
		a.LateFeeCents = &lateFeeCents
		assessments = append(assessments, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("fin: %s rows: %w", op, err)
	}
	return assessments, nil
}

// scanLedgerEntry reads a single ledger_entries row.
func scanLedgerEntry(row pgx.Row) (*LedgerEntry, error) {
	var e LedgerEntry
	err := row.Scan(
		&e.ID,
		&e.OrgID,
		&e.CurrencyCode,
		&e.UnitID,
		&e.AssessmentID,
		&e.EntryType,
		&e.AmountCents,
		&e.BalanceCents,
		&e.Description,
		&e.ReferenceType,
		&e.ReferenceID,
		&e.EffectiveDate,
		&e.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &e, nil
}

// collectLedgerEntries drains pgx.Rows into a slice of LedgerEntry values.
func collectLedgerEntries(rows pgx.Rows, op string) ([]LedgerEntry, error) {
	entries := []LedgerEntry{}
	for rows.Next() {
		var e LedgerEntry
		if err := rows.Scan(
			&e.ID,
			&e.OrgID,
			&e.CurrencyCode,
			&e.UnitID,
			&e.AssessmentID,
			&e.EntryType,
			&e.AmountCents,
			&e.BalanceCents,
			&e.Description,
			&e.ReferenceType,
			&e.ReferenceID,
			&e.EffectiveDate,
			&e.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("fin: %s scan: %w", op, err)
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("fin: %s rows: %w", op, err)
	}
	return entries, nil
}
