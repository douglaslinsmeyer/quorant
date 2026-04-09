package fin

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// PaymentRepository persists and retrieves payment transactions and saved
// payment methods for the Finance module.
type PaymentRepository interface {
	// ── Payments ──────────────────────────────────────────────────────────────

	// CreatePayment inserts a new payment record and returns the
	// fully-populated row (including generated id and timestamps).
	CreatePayment(ctx context.Context, p *Payment) (*Payment, error)

	// FindPaymentByID returns the payment with the given id, or nil, nil if
	// no matching row exists.
	FindPaymentByID(ctx context.Context, id uuid.UUID) (*Payment, error)

	// ListPaymentsByOrg returns all payments for the given org ordered by
	// created_at DESC. Returns an empty (non-nil) slice when none exist.
	ListPaymentsByOrg(ctx context.Context, orgID uuid.UUID) ([]Payment, error)

	// ListPaymentsByUnit returns all payments for the given unit ordered by
	// created_at DESC. Returns an empty (non-nil) slice when none exist.
	ListPaymentsByUnit(ctx context.Context, unitID uuid.UUID) ([]Payment, error)

	// UpdatePaymentStatus updates the status and optionally paid_at for the
	// given payment, and sets updated_at to now().
	UpdatePaymentStatus(ctx context.Context, id uuid.UUID, status string, paidAt *time.Time) error

	// ── Payment Methods ───────────────────────────────────────────────────────

	// CreatePaymentMethod inserts a new payment method and returns the
	// fully-populated row.
	CreatePaymentMethod(ctx context.Context, m *PaymentMethod) (*PaymentMethod, error)

	// ListPaymentMethodsByUser returns all non-deleted payment methods for the
	// given user ordered by created_at. Returns an empty (non-nil) slice when
	// none exist.
	ListPaymentMethodsByUser(ctx context.Context, userID uuid.UUID) ([]PaymentMethod, error)

	// SoftDeletePaymentMethod marks the payment method as deleted without
	// removing the row.
	SoftDeletePaymentMethod(ctx context.Context, id uuid.UUID) error
}
