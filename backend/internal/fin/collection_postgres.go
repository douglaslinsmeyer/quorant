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

// PostgresCollectionRepository implements CollectionRepository using a
// pgxpool.
type PostgresCollectionRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresCollectionRepository creates a new PostgresCollectionRepository
// backed by pool.
func NewPostgresCollectionRepository(pool *pgxpool.Pool) *PostgresCollectionRepository {
	return &PostgresCollectionRepository{pool: pool}
}

// ─── Collection Cases ─────────────────────────────────────────────────────────

// CreateCase inserts a new collection case and returns the fully-populated row.
func (r *PostgresCollectionRepository) CreateCase(ctx context.Context, c *CollectionCase) (*CollectionCase, error) {
	metaJSON, err := marshalMetadata(c.Metadata)
	if err != nil {
		return nil, fmt.Errorf("fin: CreateCase marshal metadata: %w", err)
	}

	const q = `
		INSERT INTO collection_cases (
			org_id, unit_id, status, total_owed_cents, current_owed_cents,
			escalation_paused, pause_reason, opened_at, closed_at, closed_reason,
			assigned_to, metadata
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9, $10,
			$11, $12
		)
		RETURNING id, org_id, unit_id, status, total_owed_cents, current_owed_cents,
		          escalation_paused, pause_reason, opened_at, closed_at, closed_reason,
		          assigned_to, metadata, created_at, updated_at`

	row := r.pool.QueryRow(ctx, q,
		c.OrgID,
		c.UnitID,
		c.Status,
		c.TotalOwedCents,
		c.CurrentOwedCents,
		c.EscalationPaused,
		c.PauseReason,
		c.OpenedAt,
		c.ClosedAt,
		c.ClosedReason,
		c.AssignedTo,
		metaJSON,
	)

	result, err := scanCollectionCase(row)
	if err != nil {
		return nil, fmt.Errorf("fin: CreateCase: %w", err)
	}
	return result, nil
}

// FindCaseByID returns the collection case with the given id, or nil, nil if
// not found.
func (r *PostgresCollectionRepository) FindCaseByID(ctx context.Context, id uuid.UUID) (*CollectionCase, error) {
	const q = `
		SELECT id, org_id, unit_id, status, total_owed_cents, current_owed_cents,
		       escalation_paused, pause_reason, opened_at, closed_at, closed_reason,
		       assigned_to, metadata, created_at, updated_at
		FROM collection_cases
		WHERE id = $1`

	row := r.pool.QueryRow(ctx, q, id)
	result, err := scanCollectionCase(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("fin: FindCaseByID: %w", err)
	}
	return result, nil
}

// ListCasesByOrg returns all collection cases for the given org ordered by
// created_at DESC. Returns an empty (non-nil) slice when none exist.
func (r *PostgresCollectionRepository) ListCasesByOrg(ctx context.Context, orgID uuid.UUID) ([]CollectionCase, error) {
	const q = `
		SELECT id, org_id, unit_id, status, total_owed_cents, current_owed_cents,
		       escalation_paused, pause_reason, opened_at, closed_at, closed_reason,
		       assigned_to, metadata, created_at, updated_at
		FROM collection_cases
		WHERE org_id = $1
		ORDER BY created_at DESC`

	rows, err := r.pool.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("fin: ListCasesByOrg: %w", err)
	}
	defer rows.Close()

	return collectCollectionCases(rows, "ListCasesByOrg")
}

// UpdateCase persists changes to an existing collection case and returns the
// updated row.
func (r *PostgresCollectionRepository) UpdateCase(ctx context.Context, c *CollectionCase) (*CollectionCase, error) {
	metaJSON, err := marshalMetadata(c.Metadata)
	if err != nil {
		return nil, fmt.Errorf("fin: UpdateCase marshal metadata: %w", err)
	}

	const q = `
		UPDATE collection_cases SET
			status              = $1,
			total_owed_cents    = $2,
			current_owed_cents  = $3,
			escalation_paused   = $4,
			pause_reason        = $5,
			closed_at           = $6,
			closed_reason       = $7,
			assigned_to         = $8,
			metadata            = $9,
			updated_at          = now()
		WHERE id = $10
		RETURNING id, org_id, unit_id, status, total_owed_cents, current_owed_cents,
		          escalation_paused, pause_reason, opened_at, closed_at, closed_reason,
		          assigned_to, metadata, created_at, updated_at`

	row := r.pool.QueryRow(ctx, q,
		c.Status,
		c.TotalOwedCents,
		c.CurrentOwedCents,
		c.EscalationPaused,
		c.PauseReason,
		c.ClosedAt,
		c.ClosedReason,
		c.AssignedTo,
		metaJSON,
		c.ID,
	)

	result, err := scanCollectionCase(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("fin: UpdateCase: case %s not found", c.ID)
	}
	if err != nil {
		return nil, fmt.Errorf("fin: UpdateCase: %w", err)
	}
	return result, nil
}

// ─── Collection Actions ───────────────────────────────────────────────────────

// CreateAction inserts a new collection action and returns the fully-populated
// row.
func (r *PostgresCollectionRepository) CreateAction(ctx context.Context, a *CollectionAction) (*CollectionAction, error) {
	metaJSON, err := marshalMetadata(a.Metadata)
	if err != nil {
		return nil, fmt.Errorf("fin: CreateAction marshal metadata: %w", err)
	}

	// triggered_by is NOT NULL in the DB; default to "system" when nil.
	triggeredBy := "system"
	if a.TriggeredBy != nil {
		triggeredBy = *a.TriggeredBy
	}

	const q = `
		INSERT INTO collection_actions (
			case_id, action_type, notes, document_id,
			triggered_by, performed_by, scheduled_for, completed_at, metadata
		) VALUES (
			$1, $2, $3, $4,
			$5, $6, $7, $8, $9
		)
		RETURNING id, case_id, action_type, notes, document_id,
		          triggered_by, performed_by, scheduled_for, completed_at,
		          metadata, created_at`

	row := r.pool.QueryRow(ctx, q,
		a.CaseID,
		a.ActionType,
		a.Notes,
		a.DocumentID,
		triggeredBy,
		a.PerformedBy,
		a.ScheduledFor,
		a.CompletedAt,
		metaJSON,
	)

	result, err := scanCollectionAction(row)
	if err != nil {
		return nil, fmt.Errorf("fin: CreateAction: %w", err)
	}
	return result, nil
}

// ListActionsByCase returns all actions for the given case ordered by
// created_at. Returns an empty (non-nil) slice when none exist.
func (r *PostgresCollectionRepository) ListActionsByCase(ctx context.Context, caseID uuid.UUID) ([]CollectionAction, error) {
	const q = `
		SELECT id, case_id, action_type, notes, document_id,
		       triggered_by, performed_by, scheduled_for, completed_at,
		       metadata, created_at
		FROM collection_actions
		WHERE case_id = $1
		ORDER BY created_at`

	rows, err := r.pool.Query(ctx, q, caseID)
	if err != nil {
		return nil, fmt.Errorf("fin: ListActionsByCase: %w", err)
	}
	defer rows.Close()

	return collectCollectionActions(rows, "ListActionsByCase")
}

// ─── Payment Plans ────────────────────────────────────────────────────────────

// CreatePaymentPlan inserts a new payment plan and returns the fully-populated
// row.
func (r *PostgresCollectionRepository) CreatePaymentPlan(ctx context.Context, p *PaymentPlan) (*PaymentPlan, error) {
	const q = `
		INSERT INTO payment_plans (
			case_id, org_id, unit_id, total_owed_cents, installment_cents,
			frequency, installments_total, installments_paid, next_due_date,
			status, approved_by, approved_at, defaulted_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9,
			$10, $11, $12, $13
		)
		RETURNING id, case_id, org_id, unit_id, total_owed_cents, installment_cents,
		          frequency, installments_total, installments_paid, next_due_date,
		          status, approved_by, approved_at, defaulted_at, created_at, updated_at`

	row := r.pool.QueryRow(ctx, q,
		p.CaseID,
		p.OrgID,
		p.UnitID,
		p.TotalOwedCents,
		p.InstallmentCents,
		p.Frequency,
		p.InstallmentsTotal,
		p.InstallmentsPaid,
		p.NextDueDate,
		p.Status,
		p.ApprovedBy,
		p.ApprovedAt,
		p.DefaultedAt,
	)

	result, err := scanPaymentPlan(row)
	if err != nil {
		return nil, fmt.Errorf("fin: CreatePaymentPlan: %w", err)
	}
	return result, nil
}

// ListPaymentPlansByCase returns all payment plans for the given case ordered
// by created_at. Returns an empty (non-nil) slice when none exist.
func (r *PostgresCollectionRepository) ListPaymentPlansByCase(ctx context.Context, caseID uuid.UUID) ([]PaymentPlan, error) {
	const q = `
		SELECT id, case_id, org_id, unit_id, total_owed_cents, installment_cents,
		       frequency, installments_total, installments_paid, next_due_date,
		       status, approved_by, approved_at, defaulted_at, created_at, updated_at
		FROM payment_plans
		WHERE case_id = $1
		ORDER BY created_at`

	rows, err := r.pool.Query(ctx, q, caseID)
	if err != nil {
		return nil, fmt.Errorf("fin: ListPaymentPlansByCase: %w", err)
	}
	defer rows.Close()

	return collectPaymentPlans(rows, "ListPaymentPlansByCase")
}

// UpdatePaymentPlan persists changes to an existing payment plan and returns
// the updated row.
func (r *PostgresCollectionRepository) UpdatePaymentPlan(ctx context.Context, p *PaymentPlan) (*PaymentPlan, error) {
	const q = `
		UPDATE payment_plans SET
			total_owed_cents   = $1,
			installment_cents  = $2,
			frequency          = $3,
			installments_total = $4,
			installments_paid  = $5,
			next_due_date      = $6,
			status             = $7,
			approved_by        = $8,
			approved_at        = $9,
			defaulted_at       = $10,
			updated_at         = now()
		WHERE id = $11
		RETURNING id, case_id, org_id, unit_id, total_owed_cents, installment_cents,
		          frequency, installments_total, installments_paid, next_due_date,
		          status, approved_by, approved_at, defaulted_at, created_at, updated_at`

	row := r.pool.QueryRow(ctx, q,
		p.TotalOwedCents,
		p.InstallmentCents,
		p.Frequency,
		p.InstallmentsTotal,
		p.InstallmentsPaid,
		p.NextDueDate,
		p.Status,
		p.ApprovedBy,
		p.ApprovedAt,
		p.DefaultedAt,
		p.ID,
	)

	result, err := scanPaymentPlan(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("fin: UpdatePaymentPlan: plan %s not found", p.ID)
	}
	if err != nil {
		return nil, fmt.Errorf("fin: UpdatePaymentPlan: %w", err)
	}
	return result, nil
}

// ─── Query Helpers ────────────────────────────────────────────────────────────

// GetCollectionStatusForUnit returns the open (not closed) collection case for
// the given unit, or nil, nil when no active case exists.
func (r *PostgresCollectionRepository) GetCollectionStatusForUnit(ctx context.Context, unitID uuid.UUID) (*CollectionCase, error) {
	const q = `
		SELECT id, org_id, unit_id, status, total_owed_cents, current_owed_cents,
		       escalation_paused, pause_reason, opened_at, closed_at, closed_reason,
		       assigned_to, metadata, created_at, updated_at
		FROM collection_cases
		WHERE unit_id = $1 AND closed_at IS NULL
		LIMIT 1`

	row := r.pool.QueryRow(ctx, q, unitID)
	result, err := scanCollectionCase(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("fin: GetCollectionStatusForUnit: %w", err)
	}
	return result, nil
}

// ─── Scan helpers ─────────────────────────────────────────────────────────────

// scanCollectionCase reads a single collection_cases row.
func scanCollectionCase(row pgx.Row) (*CollectionCase, error) {
	var c CollectionCase
	var metaRaw []byte

	err := row.Scan(
		&c.ID,
		&c.OrgID,
		&c.UnitID,
		&c.Status,
		&c.TotalOwedCents,
		&c.CurrentOwedCents,
		&c.EscalationPaused,
		&c.PauseReason,
		&c.OpenedAt,
		&c.ClosedAt,
		&c.ClosedReason,
		&c.AssignedTo,
		&metaRaw,
		&c.CreatedAt,
		&c.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if len(metaRaw) > 0 {
		if err := json.Unmarshal(metaRaw, &c.Metadata); err != nil {
			return nil, fmt.Errorf("unmarshal metadata: %w", err)
		}
	}
	if c.Metadata == nil {
		c.Metadata = map[string]any{}
	}

	return &c, nil
}

// collectCollectionCases drains pgx.Rows into a slice of CollectionCase
// values.
func collectCollectionCases(rows pgx.Rows, op string) ([]CollectionCase, error) {
	cases := []CollectionCase{}
	for rows.Next() {
		var c CollectionCase
		var metaRaw []byte

		if err := rows.Scan(
			&c.ID,
			&c.OrgID,
			&c.UnitID,
			&c.Status,
			&c.TotalOwedCents,
			&c.CurrentOwedCents,
			&c.EscalationPaused,
			&c.PauseReason,
			&c.OpenedAt,
			&c.ClosedAt,
			&c.ClosedReason,
			&c.AssignedTo,
			&metaRaw,
			&c.CreatedAt,
			&c.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("fin: %s scan: %w", op, err)
		}

		if len(metaRaw) > 0 {
			if err := json.Unmarshal(metaRaw, &c.Metadata); err != nil {
				return nil, fmt.Errorf("fin: %s unmarshal metadata: %w", op, err)
			}
		}
		if c.Metadata == nil {
			c.Metadata = map[string]any{}
		}
		cases = append(cases, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("fin: %s rows: %w", op, err)
	}
	return cases, nil
}

// scanCollectionAction reads a single collection_actions row.
func scanCollectionAction(row pgx.Row) (*CollectionAction, error) {
	var a CollectionAction
	var metaRaw []byte
	var triggeredBy string

	err := row.Scan(
		&a.ID,
		&a.CaseID,
		&a.ActionType,
		&a.Notes,
		&a.DocumentID,
		&triggeredBy,
		&a.PerformedBy,
		&a.ScheduledFor,
		&a.CompletedAt,
		&metaRaw,
		&a.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	a.TriggeredBy = &triggeredBy

	if len(metaRaw) > 0 {
		if err := json.Unmarshal(metaRaw, &a.Metadata); err != nil {
			return nil, fmt.Errorf("unmarshal metadata: %w", err)
		}
	}
	if a.Metadata == nil {
		a.Metadata = map[string]any{}
	}

	return &a, nil
}

// collectCollectionActions drains pgx.Rows into a slice of CollectionAction
// values.
func collectCollectionActions(rows pgx.Rows, op string) ([]CollectionAction, error) {
	actions := []CollectionAction{}
	for rows.Next() {
		var a CollectionAction
		var metaRaw []byte
		var triggeredBy string

		if err := rows.Scan(
			&a.ID,
			&a.CaseID,
			&a.ActionType,
			&a.Notes,
			&a.DocumentID,
			&triggeredBy,
			&a.PerformedBy,
			&a.ScheduledFor,
			&a.CompletedAt,
			&metaRaw,
			&a.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("fin: %s scan: %w", op, err)
		}

		a.TriggeredBy = &triggeredBy

		if len(metaRaw) > 0 {
			if err := json.Unmarshal(metaRaw, &a.Metadata); err != nil {
				return nil, fmt.Errorf("fin: %s unmarshal metadata: %w", op, err)
			}
		}
		if a.Metadata == nil {
			a.Metadata = map[string]any{}
		}
		actions = append(actions, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("fin: %s rows: %w", op, err)
	}
	return actions, nil
}

// scanPaymentPlan reads a single payment_plans row.
func scanPaymentPlan(row pgx.Row) (*PaymentPlan, error) {
	var p PaymentPlan
	err := row.Scan(
		&p.ID,
		&p.CaseID,
		&p.OrgID,
		&p.UnitID,
		&p.TotalOwedCents,
		&p.InstallmentCents,
		&p.Frequency,
		&p.InstallmentsTotal,
		&p.InstallmentsPaid,
		&p.NextDueDate,
		&p.Status,
		&p.ApprovedBy,
		&p.ApprovedAt,
		&p.DefaultedAt,
		&p.CreatedAt,
		&p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// collectPaymentPlans drains pgx.Rows into a slice of PaymentPlan values.
func collectPaymentPlans(rows pgx.Rows, op string) ([]PaymentPlan, error) {
	plans := []PaymentPlan{}
	for rows.Next() {
		var p PaymentPlan
		if err := rows.Scan(
			&p.ID,
			&p.CaseID,
			&p.OrgID,
			&p.UnitID,
			&p.TotalOwedCents,
			&p.InstallmentCents,
			&p.Frequency,
			&p.InstallmentsTotal,
			&p.InstallmentsPaid,
			&p.NextDueDate,
			&p.Status,
			&p.ApprovedBy,
			&p.ApprovedAt,
			&p.DefaultedAt,
			&p.CreatedAt,
			&p.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("fin: %s scan: %w", op, err)
		}
		plans = append(plans, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("fin: %s rows: %w", op, err)
	}
	return plans, nil
}
