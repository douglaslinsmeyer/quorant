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

// PostgresPaymentRepository implements PaymentRepository using a DBTX.
type PostgresPaymentRepository struct {
	db dbpkg.DBTX
}

// NewPostgresPaymentRepository creates a new PostgresPaymentRepository backed
// by pool.
func NewPostgresPaymentRepository(pool *pgxpool.Pool) *PostgresPaymentRepository {
	return &PostgresPaymentRepository{db: pool}
}

// WithTx returns a new PostgresPaymentRepository scoped to the given
// transaction, enabling participation in a caller-managed transaction.
func (r *PostgresPaymentRepository) WithTx(tx pgx.Tx) PaymentRepository {
	return &PostgresPaymentRepository{db: tx}
}

// ─── Payments ─────────────────────────────────────────────────────────────────

// CreatePayment inserts a new payment record and returns the fully-populated
// row.
func (r *PostgresPaymentRepository) CreatePayment(ctx context.Context, p *Payment) (*Payment, error) {
	const q = `
		INSERT INTO payments (
			org_id, currency_code, unit_id, user_id, payment_method_id, amount_cents,
			status, provider_ref, description, paid_at
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10
		)
		RETURNING id, org_id, currency_code, unit_id, user_id, payment_method_id, amount_cents,
		          status, provider_ref, description, paid_at,
		          voided_by, voided_at, created_at, updated_at`

	row := r.db.QueryRow(ctx, q,
		p.OrgID,
		p.CurrencyCode,
		p.UnitID,
		p.UserID,
		p.PaymentMethodID,
		p.AmountCents,
		p.Status,
		p.ProviderRef,
		p.Description,
		p.PaidAt,
	)

	result, err := scanPayment(row)
	if err != nil {
		return nil, fmt.Errorf("fin: CreatePayment: %w", err)
	}
	return result, nil
}

// FindPaymentByID returns the payment with the given id, or nil, nil if not
// found.
func (r *PostgresPaymentRepository) FindPaymentByID(ctx context.Context, id uuid.UUID) (*Payment, error) {
	const q = `
		SELECT id, org_id, currency_code, unit_id, user_id, payment_method_id, amount_cents,
		       status, provider_ref, description, paid_at,
		       voided_by, voided_at, created_at, updated_at
		FROM payments
		WHERE id = $1`

	row := r.db.QueryRow(ctx, q, id)
	result, err := scanPayment(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("fin: FindPaymentByID: %w", err)
	}
	return result, nil
}

// ListPaymentsByOrg returns payments for the given org, supporting cursor-based
// pagination ordered by id DESC. afterID is the cursor from the previous page.
func (r *PostgresPaymentRepository) ListPaymentsByOrg(ctx context.Context, orgID uuid.UUID, limit int, afterID *uuid.UUID) ([]Payment, bool, error) {
	const q = `
		SELECT id, org_id, currency_code, unit_id, user_id, payment_method_id, amount_cents,
		       status, provider_ref, description, paid_at,
		       voided_by, voided_at, created_at, updated_at
		FROM payments
		WHERE org_id = $1
		  AND ($3::uuid IS NULL OR id < $3)
		ORDER BY id DESC
		LIMIT $2`

	rows, err := r.db.Query(ctx, q, orgID, limit+1, afterID)
	if err != nil {
		return nil, false, fmt.Errorf("fin: ListPaymentsByOrg: %w", err)
	}
	defer rows.Close()

	payments, err := collectPayments(rows, "ListPaymentsByOrg")
	if err != nil {
		return nil, false, err
	}

	hasMore := len(payments) > limit
	if hasMore {
		payments = payments[:limit]
	}
	return payments, hasMore, nil
}

// ListPaymentsByUnit returns all payments for the given unit ordered by
// created_at DESC. Returns an empty (non-nil) slice when none exist.
func (r *PostgresPaymentRepository) ListPaymentsByUnit(ctx context.Context, unitID uuid.UUID) ([]Payment, error) {
	const q = `
		SELECT id, org_id, currency_code, unit_id, user_id, payment_method_id, amount_cents,
		       status, provider_ref, description, paid_at,
		       voided_by, voided_at, created_at, updated_at
		FROM payments
		WHERE unit_id = $1
		ORDER BY created_at DESC`

	rows, err := r.db.Query(ctx, q, unitID)
	if err != nil {
		return nil, fmt.Errorf("fin: ListPaymentsByUnit: %w", err)
	}
	defer rows.Close()

	return collectPayments(rows, "ListPaymentsByUnit")
}

// UpdatePaymentStatus updates the status and optionally paid_at for the given
// payment, and bumps updated_at to now().
func (r *PostgresPaymentRepository) UpdatePaymentStatus(ctx context.Context, id uuid.UUID, status PaymentStatus, paidAt *time.Time) error {
	const q = `
		UPDATE payments
		SET status     = $1,
		    paid_at    = $2,
		    updated_at = now()
		WHERE id = $3`

	_, err := r.db.Exec(ctx, q, status, paidAt, id)
	if err != nil {
		return fmt.Errorf("fin: UpdatePaymentStatus: %w", err)
	}
	return nil
}

// UpdatePaymentVoid marks the payment as voided, recording who voided it and when.
// voidedBy may be nil when the void is triggered by a system operation.
func (r *PostgresPaymentRepository) UpdatePaymentVoid(ctx context.Context, id uuid.UUID, voidedBy *uuid.UUID, voidedAt *time.Time) error {
	const q = `
		UPDATE payments
		SET status     = 'void',
		    voided_by  = $1,
		    voided_at  = $2,
		    updated_at = now()
		WHERE id = $3`

	_, err := r.db.Exec(ctx, q, voidedBy, voidedAt, id)
	if err != nil {
		return fmt.Errorf("fin: UpdatePaymentVoid: %w", err)
	}
	return nil
}

// ─── Payment Methods ──────────────────────────────────────────────────────────

// CreatePaymentMethod inserts a new payment method and returns the
// fully-populated row.
func (r *PostgresPaymentRepository) CreatePaymentMethod(ctx context.Context, m *PaymentMethod) (*PaymentMethod, error) {
	const q = `
		INSERT INTO payment_methods (
			org_id, user_id, method_type, provider_ref, last_four, is_default
		) VALUES (
			$1, $2, $3, $4, $5, $6
		)
		RETURNING id, org_id, user_id, method_type, provider_ref, last_four,
		          is_default, created_at, deleted_at`

	row := r.db.QueryRow(ctx, q,
		m.OrgID,
		m.UserID,
		m.MethodType,
		m.ProviderRef,
		m.LastFour,
		m.IsDefault,
	)

	result, err := scanPaymentMethod(row)
	if err != nil {
		return nil, fmt.Errorf("fin: CreatePaymentMethod: %w", err)
	}
	return result, nil
}

// ListPaymentMethodsByUser returns all non-deleted payment methods for the
// given user ordered by created_at. Returns an empty (non-nil) slice when none
// exist.
func (r *PostgresPaymentRepository) ListPaymentMethodsByUser(ctx context.Context, userID uuid.UUID) ([]PaymentMethod, error) {
	const q = `
		SELECT id, org_id, user_id, method_type, provider_ref, last_four,
		       is_default, created_at, deleted_at
		FROM payment_methods
		WHERE user_id = $1 AND deleted_at IS NULL
		ORDER BY created_at`

	rows, err := r.db.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("fin: ListPaymentMethodsByUser: %w", err)
	}
	defer rows.Close()

	return collectPaymentMethods(rows, "ListPaymentMethodsByUser")
}

// SoftDeletePaymentMethod marks the payment method as deleted without removing
// the row.
func (r *PostgresPaymentRepository) SoftDeletePaymentMethod(ctx context.Context, id uuid.UUID) error {
	const q = `
		UPDATE payment_methods
		SET deleted_at = now()
		WHERE id = $1 AND deleted_at IS NULL`

	_, err := r.db.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("fin: SoftDeletePaymentMethod: %w", err)
	}
	return nil
}

// ─── Payment Allocations ──────────────────────────────────────────────────────

func (r *PostgresPaymentRepository) CreatePaymentAllocation(ctx context.Context, a *PaymentAllocation) (*PaymentAllocation, error) {
	const q = `
		INSERT INTO payment_allocations (
			payment_id, charge_type, charge_id, allocated_cents, resolution_id,
			estoppel_id, reversed_at, reversed_by_id
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, payment_id, charge_type, charge_id, allocated_cents, resolution_id,
		          estoppel_id, reversed_at, reversed_by_id, created_at`

	var out PaymentAllocation
	err := r.db.QueryRow(ctx, q,
		a.PaymentID, a.ChargeType, a.ChargeID, a.AllocatedCents,
		a.ResolutionID, a.EstoppelID, a.ReversedAt, a.ReversedByID,
	).Scan(
		&out.ID, &out.PaymentID, &out.ChargeType, &out.ChargeID,
		&out.AllocatedCents, &out.ResolutionID, &out.EstoppelID,
		&out.ReversedAt, &out.ReversedByID, &out.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("fin: CreatePaymentAllocation: %w", err)
	}
	return &out, nil
}

func (r *PostgresPaymentRepository) ListAllocationsByPayment(ctx context.Context, paymentID uuid.UUID) ([]PaymentAllocation, error) {
	const q = `
		SELECT id, payment_id, charge_type, charge_id, allocated_cents, resolution_id,
		       estoppel_id, reversed_at, reversed_by_id, created_at
		FROM payment_allocations
		WHERE payment_id = $1
		ORDER BY created_at`

	rows, err := r.db.Query(ctx, q, paymentID)
	if err != nil {
		return nil, fmt.Errorf("fin: ListAllocationsByPayment: %w", err)
	}
	defer rows.Close()

	var results []PaymentAllocation
	for rows.Next() {
		var a PaymentAllocation
		if err := rows.Scan(
			&a.ID, &a.PaymentID, &a.ChargeType, &a.ChargeID,
			&a.AllocatedCents, &a.ResolutionID, &a.EstoppelID,
			&a.ReversedAt, &a.ReversedByID, &a.CreatedAt,
		); err != nil {
			return nil, err
		}
		results = append(results, a)
	}
	if results == nil {
		return []PaymentAllocation{}, nil
	}
	return results, nil
}

// ─── Scan helpers ─────────────────────────────────────────────────────────────

// scanPayment reads a single payments row.
func scanPayment(row pgx.Row) (*Payment, error) {
	var p Payment
	err := row.Scan(
		&p.ID,
		&p.OrgID,
		&p.CurrencyCode,
		&p.UnitID,
		&p.UserID,
		&p.PaymentMethodID,
		&p.AmountCents,
		&p.Status,
		&p.ProviderRef,
		&p.Description,
		&p.PaidAt,
		&p.VoidedBy,
		&p.VoidedAt,
		&p.CreatedAt,
		&p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// collectPayments drains pgx.Rows into a slice of Payment values.
func collectPayments(rows pgx.Rows, op string) ([]Payment, error) {
	payments := []Payment{}
	for rows.Next() {
		var p Payment
		if err := rows.Scan(
			&p.ID,
			&p.OrgID,
			&p.CurrencyCode,
			&p.UnitID,
			&p.UserID,
			&p.PaymentMethodID,
			&p.AmountCents,
			&p.Status,
			&p.ProviderRef,
			&p.Description,
			&p.PaidAt,
			&p.VoidedBy,
			&p.VoidedAt,
			&p.CreatedAt,
			&p.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("fin: %s scan: %w", op, err)
		}
		payments = append(payments, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("fin: %s rows: %w", op, err)
	}
	return payments, nil
}

// scanPaymentMethod reads a single payment_methods row.
func scanPaymentMethod(row pgx.Row) (*PaymentMethod, error) {
	var m PaymentMethod
	err := row.Scan(
		&m.ID,
		&m.OrgID,
		&m.UserID,
		&m.MethodType,
		&m.ProviderRef,
		&m.LastFour,
		&m.IsDefault,
		&m.CreatedAt,
		&m.DeletedAt,
	)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// collectPaymentMethods drains pgx.Rows into a slice of PaymentMethod values.
func collectPaymentMethods(rows pgx.Rows, op string) ([]PaymentMethod, error) {
	methods := []PaymentMethod{}
	for rows.Next() {
		var m PaymentMethod
		if err := rows.Scan(
			&m.ID,
			&m.OrgID,
			&m.UserID,
			&m.MethodType,
			&m.ProviderRef,
			&m.LastFour,
			&m.IsDefault,
			&m.CreatedAt,
			&m.DeletedAt,
		); err != nil {
			return nil, fmt.Errorf("fin: %s scan: %w", op, err)
		}
		methods = append(methods, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("fin: %s rows: %w", op, err)
	}
	return methods, nil
}
