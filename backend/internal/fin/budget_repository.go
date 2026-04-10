package fin

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// BudgetRepository persists and retrieves budget categories, budgets, budget
// line items, and expenses for the Finance module.
type BudgetRepository interface {
	// ── Categories ────────────────────────────────────────────────────────────

	// CreateCategory inserts a new budget category and returns the
	// fully-populated row (including generated id and timestamp).
	CreateCategory(ctx context.Context, c *BudgetCategory) (*BudgetCategory, error)

	// ListCategoriesByOrg returns all categories for the given org, ordered by
	// sort_order, name. Returns an empty (non-nil) slice when none exist.
	ListCategoriesByOrg(ctx context.Context, orgID uuid.UUID) ([]BudgetCategory, error)

	// UpdateCategory persists changes to an existing category and returns the
	// updated row.
	UpdateCategory(ctx context.Context, c *BudgetCategory) (*BudgetCategory, error)

	// ── Budgets ───────────────────────────────────────────────────────────────

	// CreateBudget inserts a new budget and returns the fully-populated row.
	CreateBudget(ctx context.Context, b *Budget) (*Budget, error)

	// FindBudgetByID returns the budget with the given id, or nil, nil if no
	// matching (non-deleted) row exists.
	FindBudgetByID(ctx context.Context, id uuid.UUID) (*Budget, error)

	// ListBudgetsByOrg returns all non-deleted budgets for the given org,
	// ordered by fiscal_year DESC. Returns an empty (non-nil) slice when none
	// exist.
	ListBudgetsByOrg(ctx context.Context, orgID uuid.UUID) ([]Budget, error)

	// UpdateBudget persists changes to an existing budget and returns the
	// updated row.
	UpdateBudget(ctx context.Context, b *Budget) (*Budget, error)

	// SoftDeleteBudget marks the budget as deleted without removing the row.
	SoftDeleteBudget(ctx context.Context, id uuid.UUID) error

	// ── Line Items ────────────────────────────────────────────────────────────

	// CreateLineItem inserts a new budget line item and returns the
	// fully-populated row.
	CreateLineItem(ctx context.Context, item *BudgetLineItem) (*BudgetLineItem, error)

	// FindLineItemByID returns the line item with the given id, or nil, nil
	// if no matching row exists.
	FindLineItemByID(ctx context.Context, id uuid.UUID) (*BudgetLineItem, error)

	// ListLineItemsByBudget returns all line items for the given budget, ordered
	// by created_at. Returns an empty (non-nil) slice when none exist.
	ListLineItemsByBudget(ctx context.Context, budgetID uuid.UUID) ([]BudgetLineItem, error)

	// UpdateLineItem persists changes to an existing line item and returns the
	// updated row.
	UpdateLineItem(ctx context.Context, item *BudgetLineItem) (*BudgetLineItem, error)

	// DeleteLineItem hard-deletes the line item. Since budget_line_items
	// CASCADE from budgets, this is a real DELETE rather than a soft delete.
	DeleteLineItem(ctx context.Context, id uuid.UUID) error

	// ── Expenses ──────────────────────────────────────────────────────────────

	// CreateExpense inserts a new expense record and returns the
	// fully-populated row.
	CreateExpense(ctx context.Context, e *Expense) (*Expense, error)

	// FindExpenseByID returns the expense with the given id, or nil, nil if no
	// matching (non-deleted) row exists.
	FindExpenseByID(ctx context.Context, id uuid.UUID) (*Expense, error)

	// ListExpensesByOrg returns all non-deleted expenses for the given org,
	// ordered by expense_date DESC. Returns an empty (non-nil) slice when none
	// exist.
	ListExpensesByOrg(ctx context.Context, orgID uuid.UUID) ([]Expense, error)

	// UpdateExpense persists changes to an existing expense and returns the
	// updated row.
	UpdateExpense(ctx context.Context, e *Expense) (*Expense, error)

	// SoftDeleteExpense marks the expense as deleted without removing the row.
	SoftDeleteExpense(ctx context.Context, id uuid.UUID) error

	// WithTx returns a copy of the repository that runs queries against the
	// given transaction instead of the pool.
	WithTx(tx pgx.Tx) BudgetRepository
}
