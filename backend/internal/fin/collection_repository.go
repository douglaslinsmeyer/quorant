package fin

import (
	"context"

	"github.com/google/uuid"
)

// CollectionRepository persists and retrieves collection cases, collection
// actions, and payment plans for the Finance module.
type CollectionRepository interface {
	// ── Collection Cases ──────────────────────────────────────────────────────

	// CreateCase inserts a new collection case and returns the fully-populated
	// row (including generated id and timestamps).
	CreateCase(ctx context.Context, c *CollectionCase) (*CollectionCase, error)

	// FindCaseByID returns the collection case with the given id, or nil, nil
	// if no matching row exists.
	FindCaseByID(ctx context.Context, id uuid.UUID) (*CollectionCase, error)

	// ListCasesByOrg returns all collection cases for the given org, ordered by
	// created_at DESC. Returns an empty (non-nil) slice when none exist.
	ListCasesByOrg(ctx context.Context, orgID uuid.UUID) ([]CollectionCase, error)

	// UpdateCase persists changes to an existing collection case and returns the
	// updated row.
	UpdateCase(ctx context.Context, c *CollectionCase) (*CollectionCase, error)

	// ── Collection Actions ────────────────────────────────────────────────────

	// CreateAction inserts a new collection action and returns the
	// fully-populated row.
	CreateAction(ctx context.Context, a *CollectionAction) (*CollectionAction, error)

	// ListActionsByCase returns all actions for the given case, ordered by
	// created_at. Returns an empty (non-nil) slice when none exist.
	ListActionsByCase(ctx context.Context, caseID uuid.UUID) ([]CollectionAction, error)

	// ── Payment Plans ─────────────────────────────────────────────────────────

	// CreatePaymentPlan inserts a new payment plan and returns the
	// fully-populated row.
	CreatePaymentPlan(ctx context.Context, p *PaymentPlan) (*PaymentPlan, error)

	// ListPaymentPlansByCase returns all payment plans for the given case,
	// ordered by created_at. Returns an empty (non-nil) slice when none exist.
	ListPaymentPlansByCase(ctx context.Context, caseID uuid.UUID) ([]PaymentPlan, error)

	// UpdatePaymentPlan persists changes to an existing payment plan and
	// returns the updated row.
	UpdatePaymentPlan(ctx context.Context, p *PaymentPlan) (*PaymentPlan, error)

	// ── Query Helpers ─────────────────────────────────────────────────────────

	// GetCollectionStatusForUnit returns the open (not closed) collection case
	// for the given unit, or nil, nil when no active case exists. The DB unique
	// index (idx_collection_cases_active) ensures at most one open case per
	// unit.
	GetCollectionStatusForUnit(ctx context.Context, unitID uuid.UUID) (*CollectionCase, error)
}
