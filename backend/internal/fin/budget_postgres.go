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

// PostgresBudgetRepository implements BudgetRepository using a pgxpool.
type PostgresBudgetRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresBudgetRepository creates a new PostgresBudgetRepository backed by
// pool.
func NewPostgresBudgetRepository(pool *pgxpool.Pool) *PostgresBudgetRepository {
	return &PostgresBudgetRepository{pool: pool}
}

// ─── Categories ───────────────────────────────────────────────────────────────

// CreateCategory inserts a new budget category and returns the fully-populated
// row.
func (r *PostgresBudgetRepository) CreateCategory(ctx context.Context, c *BudgetCategory) (*BudgetCategory, error) {
	const q = `
		INSERT INTO budget_categories (
			org_id, name, category_type, parent_id, sort_order, is_reserve
		) VALUES (
			$1, $2, $3, $4, $5, $6
		)
		RETURNING id, org_id, name, category_type, parent_id, sort_order, is_reserve, created_at`

	row := r.pool.QueryRow(ctx, q,
		c.OrgID,
		c.Name,
		c.CategoryType,
		c.ParentID,
		c.SortOrder,
		c.IsReserve,
	)

	result, err := scanCategory(row)
	if err != nil {
		return nil, fmt.Errorf("fin: CreateCategory: %w", err)
	}
	return result, nil
}

// ListCategoriesByOrg returns all categories for the given org ordered by
// sort_order, name. Returns an empty (non-nil) slice when none exist.
func (r *PostgresBudgetRepository) ListCategoriesByOrg(ctx context.Context, orgID uuid.UUID) ([]BudgetCategory, error) {
	const q = `
		SELECT id, org_id, name, category_type, parent_id, sort_order, is_reserve, created_at
		FROM budget_categories
		WHERE org_id = $1
		ORDER BY sort_order, name`

	rows, err := r.pool.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("fin: ListCategoriesByOrg: %w", err)
	}
	defer rows.Close()

	return collectCategories(rows, "ListCategoriesByOrg")
}

// UpdateCategory persists changes to an existing category and returns the
// updated row.
func (r *PostgresBudgetRepository) UpdateCategory(ctx context.Context, c *BudgetCategory) (*BudgetCategory, error) {
	const q = `
		UPDATE budget_categories SET
			name          = $1,
			category_type = $2,
			parent_id     = $3,
			sort_order    = $4,
			is_reserve    = $5
		WHERE id = $6
		RETURNING id, org_id, name, category_type, parent_id, sort_order, is_reserve, created_at`

	row := r.pool.QueryRow(ctx, q,
		c.Name,
		c.CategoryType,
		c.ParentID,
		c.SortOrder,
		c.IsReserve,
		c.ID,
	)

	result, err := scanCategory(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("fin: UpdateCategory: category %s not found", c.ID)
	}
	if err != nil {
		return nil, fmt.Errorf("fin: UpdateCategory: %w", err)
	}
	return result, nil
}

// ─── Budgets ──────────────────────────────────────────────────────────────────

// CreateBudget inserts a new budget and returns the fully-populated row.
func (r *PostgresBudgetRepository) CreateBudget(ctx context.Context, b *Budget) (*Budget, error) {
	const q = `
		INSERT INTO budgets (
			org_id, fiscal_year, name, status,
			total_income_cents, total_expense_cents, net_cents,
			notes, proposed_at, proposed_by, approved_at, approved_by,
			document_id, created_by
		) VALUES (
			$1, $2, $3, $4,
			$5, $6, $7,
			$8, $9, $10, $11, $12,
			$13, $14
		)
		RETURNING id, org_id, fiscal_year, name, status,
		          total_income_cents, total_expense_cents, net_cents,
		          notes, proposed_at, proposed_by, approved_at, approved_by,
		          document_id, created_by, created_at, updated_at, deleted_at`

	row := r.pool.QueryRow(ctx, q,
		b.OrgID,
		b.FiscalYear,
		b.Name,
		b.Status,
		b.TotalIncomeCents,
		b.TotalExpenseCents,
		b.NetCents,
		b.Notes,
		b.ProposedAt,
		b.ProposedBy,
		b.ApprovedAt,
		b.ApprovedBy,
		b.DocumentID,
		b.CreatedBy,
	)

	result, err := scanBudget(row)
	if err != nil {
		return nil, fmt.Errorf("fin: CreateBudget: %w", err)
	}
	return result, nil
}

// FindBudgetByID returns the budget with the given id, or nil, nil if not
// found or soft-deleted.
func (r *PostgresBudgetRepository) FindBudgetByID(ctx context.Context, id uuid.UUID) (*Budget, error) {
	const q = `
		SELECT id, org_id, fiscal_year, name, status,
		       total_income_cents, total_expense_cents, net_cents,
		       notes, proposed_at, proposed_by, approved_at, approved_by,
		       document_id, created_by, created_at, updated_at, deleted_at
		FROM budgets
		WHERE id = $1 AND deleted_at IS NULL`

	row := r.pool.QueryRow(ctx, q, id)
	result, err := scanBudget(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("fin: FindBudgetByID: %w", err)
	}
	return result, nil
}

// ListBudgetsByOrg returns all non-deleted budgets for the given org ordered
// by fiscal_year DESC. Returns an empty (non-nil) slice when none exist.
func (r *PostgresBudgetRepository) ListBudgetsByOrg(ctx context.Context, orgID uuid.UUID) ([]Budget, error) {
	const q = `
		SELECT id, org_id, fiscal_year, name, status,
		       total_income_cents, total_expense_cents, net_cents,
		       notes, proposed_at, proposed_by, approved_at, approved_by,
		       document_id, created_by, created_at, updated_at, deleted_at
		FROM budgets
		WHERE org_id = $1 AND deleted_at IS NULL
		ORDER BY fiscal_year DESC`

	rows, err := r.pool.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("fin: ListBudgetsByOrg: %w", err)
	}
	defer rows.Close()

	return collectBudgets(rows, "ListBudgetsByOrg")
}

// UpdateBudget persists changes to an existing budget and returns the updated
// row.
func (r *PostgresBudgetRepository) UpdateBudget(ctx context.Context, b *Budget) (*Budget, error) {
	const q = `
		UPDATE budgets SET
			fiscal_year         = $1,
			name                = $2,
			status              = $3,
			total_income_cents  = $4,
			total_expense_cents = $5,
			net_cents           = $6,
			notes               = $7,
			proposed_at         = $8,
			proposed_by         = $9,
			approved_at         = $10,
			approved_by         = $11,
			document_id         = $12,
			updated_at          = now()
		WHERE id = $13 AND deleted_at IS NULL
		RETURNING id, org_id, fiscal_year, name, status,
		          total_income_cents, total_expense_cents, net_cents,
		          notes, proposed_at, proposed_by, approved_at, approved_by,
		          document_id, created_by, created_at, updated_at, deleted_at`

	row := r.pool.QueryRow(ctx, q,
		b.FiscalYear,
		b.Name,
		b.Status,
		b.TotalIncomeCents,
		b.TotalExpenseCents,
		b.NetCents,
		b.Notes,
		b.ProposedAt,
		b.ProposedBy,
		b.ApprovedAt,
		b.ApprovedBy,
		b.DocumentID,
		b.ID,
	)

	result, err := scanBudget(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("fin: UpdateBudget: budget %s not found or already deleted", b.ID)
	}
	if err != nil {
		return nil, fmt.Errorf("fin: UpdateBudget: %w", err)
	}
	return result, nil
}

// SoftDeleteBudget marks the budget as deleted without removing the row.
func (r *PostgresBudgetRepository) SoftDeleteBudget(ctx context.Context, id uuid.UUID) error {
	const q = `
		UPDATE budgets
		SET deleted_at = now()
		WHERE id = $1 AND deleted_at IS NULL`

	_, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("fin: SoftDeleteBudget: %w", err)
	}
	return nil
}

// ─── Line Items ───────────────────────────────────────────────────────────────

// CreateLineItem inserts a new budget line item and returns the
// fully-populated row.
func (r *PostgresBudgetRepository) CreateLineItem(ctx context.Context, item *BudgetLineItem) (*BudgetLineItem, error) {
	const q = `
		INSERT INTO budget_line_items (
			budget_id, category_id, description, planned_cents, actual_cents, notes
		) VALUES (
			$1, $2, $3, $4, $5, $6
		)
		RETURNING id, budget_id, category_id, description, planned_cents, actual_cents,
		          notes, created_at, updated_at`

	row := r.pool.QueryRow(ctx, q,
		item.BudgetID,
		item.CategoryID,
		item.Description,
		item.PlannedCents,
		item.ActualCents,
		item.Notes,
	)

	result, err := scanLineItem(row)
	if err != nil {
		return nil, fmt.Errorf("fin: CreateLineItem: %w", err)
	}
	return result, nil
}

// ListLineItemsByBudget returns all line items for the given budget ordered by
// created_at. Returns an empty (non-nil) slice when none exist.
func (r *PostgresBudgetRepository) ListLineItemsByBudget(ctx context.Context, budgetID uuid.UUID) ([]BudgetLineItem, error) {
	const q = `
		SELECT id, budget_id, category_id, description, planned_cents, actual_cents,
		       notes, created_at, updated_at
		FROM budget_line_items
		WHERE budget_id = $1
		ORDER BY created_at`

	rows, err := r.pool.Query(ctx, q, budgetID)
	if err != nil {
		return nil, fmt.Errorf("fin: ListLineItemsByBudget: %w", err)
	}
	defer rows.Close()

	return collectLineItems(rows, "ListLineItemsByBudget")
}

// UpdateLineItem persists changes to an existing line item and returns the
// updated row.
func (r *PostgresBudgetRepository) UpdateLineItem(ctx context.Context, item *BudgetLineItem) (*BudgetLineItem, error) {
	const q = `
		UPDATE budget_line_items SET
			category_id   = $1,
			description   = $2,
			planned_cents = $3,
			actual_cents  = $4,
			notes         = $5,
			updated_at    = now()
		WHERE id = $6
		RETURNING id, budget_id, category_id, description, planned_cents, actual_cents,
		          notes, created_at, updated_at`

	row := r.pool.QueryRow(ctx, q,
		item.CategoryID,
		item.Description,
		item.PlannedCents,
		item.ActualCents,
		item.Notes,
		item.ID,
	)

	result, err := scanLineItem(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("fin: UpdateLineItem: line item %s not found", item.ID)
	}
	if err != nil {
		return nil, fmt.Errorf("fin: UpdateLineItem: %w", err)
	}
	return result, nil
}

// DeleteLineItem hard-deletes the line item. Budget line items CASCADE from
// budgets, so this is a real DELETE rather than a soft delete.
func (r *PostgresBudgetRepository) DeleteLineItem(ctx context.Context, id uuid.UUID) error {
	const q = `DELETE FROM budget_line_items WHERE id = $1`

	_, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("fin: DeleteLineItem: %w", err)
	}
	return nil
}

// ─── Expenses ─────────────────────────────────────────────────────────────────

// CreateExpense inserts a new expense record and returns the fully-populated
// row.
func (r *PostgresBudgetRepository) CreateExpense(ctx context.Context, e *Expense) (*Expense, error) {
	metaJSON, err := marshalMetadata(e.Metadata)
	if err != nil {
		return nil, fmt.Errorf("fin: CreateExpense marshal metadata: %w", err)
	}

	// fund_type is NOT NULL in the schema; default to "operating" when nil.
	fundType := FundTypeOperating
	if e.FundType != nil {
		fundType = *e.FundType
	}

	const q = `
		INSERT INTO expenses (
			org_id, currency_code, vendor_id, category_id, budget_id, fund_type,
			description, amount_cents, tax_cents, total_cents,
			status, expense_date, due_date, paid_date, payment_ref,
			receipt_doc_id, submitted_by, approved_by, approved_at, metadata
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10,
			$11, $12, $13, $14, $15,
			$16, $17, $18, $19, $20
		)
		RETURNING id, org_id, currency_code, vendor_id, category_id, budget_id, fund_type,
		          description, amount_cents, tax_cents, total_cents,
		          status, expense_date, due_date, paid_date, payment_ref,
		          receipt_doc_id, submitted_by, approved_by, approved_at, metadata,
		          created_at, updated_at, deleted_at`

	row := r.pool.QueryRow(ctx, q,
		e.OrgID,
		e.CurrencyCode,
		e.VendorID,
		e.CategoryID,
		e.BudgetID,
		fundType,
		e.Description,
		e.AmountCents,
		e.TaxCents,
		e.TotalCents,
		e.Status,
		e.ExpenseDate,
		e.DueDate,
		e.PaidDate,
		e.PaymentRef,
		e.ReceiptDocID,
		e.SubmittedBy,
		e.ApprovedBy,
		e.ApprovedAt,
		metaJSON,
	)

	result, err := scanExpense(row)
	if err != nil {
		return nil, fmt.Errorf("fin: CreateExpense: %w", err)
	}
	return result, nil
}

// FindExpenseByID returns the expense with the given id, or nil, nil if not
// found or soft-deleted.
func (r *PostgresBudgetRepository) FindExpenseByID(ctx context.Context, id uuid.UUID) (*Expense, error) {
	const q = `
		SELECT id, org_id, currency_code, vendor_id, category_id, budget_id, fund_type,
		       description, amount_cents, tax_cents, total_cents,
		       status, expense_date, due_date, paid_date, payment_ref,
		       receipt_doc_id, submitted_by, approved_by, approved_at, metadata,
		       created_at, updated_at, deleted_at
		FROM expenses
		WHERE id = $1 AND deleted_at IS NULL`

	row := r.pool.QueryRow(ctx, q, id)
	result, err := scanExpense(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("fin: FindExpenseByID: %w", err)
	}
	return result, nil
}

// ListExpensesByOrg returns all non-deleted expenses for the given org ordered
// by expense_date DESC. Returns an empty (non-nil) slice when none exist.
func (r *PostgresBudgetRepository) ListExpensesByOrg(ctx context.Context, orgID uuid.UUID) ([]Expense, error) {
	const q = `
		SELECT id, org_id, currency_code, vendor_id, category_id, budget_id, fund_type,
		       description, amount_cents, tax_cents, total_cents,
		       status, expense_date, due_date, paid_date, payment_ref,
		       receipt_doc_id, submitted_by, approved_by, approved_at, metadata,
		       created_at, updated_at, deleted_at
		FROM expenses
		WHERE org_id = $1 AND deleted_at IS NULL
		ORDER BY expense_date DESC`

	rows, err := r.pool.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("fin: ListExpensesByOrg: %w", err)
	}
	defer rows.Close()

	return collectExpenses(rows, "ListExpensesByOrg")
}

// UpdateExpense persists changes to an existing expense and returns the
// updated row.
func (r *PostgresBudgetRepository) UpdateExpense(ctx context.Context, e *Expense) (*Expense, error) {
	metaJSON, err := marshalMetadata(e.Metadata)
	if err != nil {
		return nil, fmt.Errorf("fin: UpdateExpense marshal metadata: %w", err)
	}

	fundType := FundTypeOperating
	if e.FundType != nil {
		fundType = *e.FundType
	}

	const q = `
		UPDATE expenses SET
			currency_code  = $1,
			vendor_id     = $2,
			category_id   = $3,
			budget_id     = $4,
			fund_type     = $5,
			description   = $6,
			amount_cents  = $7,
			tax_cents     = $8,
			total_cents   = $9,
			status        = $10,
			expense_date  = $11,
			due_date      = $12,
			paid_date     = $13,
			payment_ref   = $14,
			receipt_doc_id = $15,
			approved_by   = $16,
			approved_at   = $17,
			metadata      = $18,
			updated_at    = now()
		WHERE id = $19 AND deleted_at IS NULL
		RETURNING id, org_id, currency_code, vendor_id, category_id, budget_id, fund_type,
		          description, amount_cents, tax_cents, total_cents,
		          status, expense_date, due_date, paid_date, payment_ref,
		          receipt_doc_id, submitted_by, approved_by, approved_at, metadata,
		          created_at, updated_at, deleted_at`

	row := r.pool.QueryRow(ctx, q,
		e.CurrencyCode,
		e.VendorID,
		e.CategoryID,
		e.BudgetID,
		fundType,
		e.Description,
		e.AmountCents,
		e.TaxCents,
		e.TotalCents,
		e.Status,
		e.ExpenseDate,
		e.DueDate,
		e.PaidDate,
		e.PaymentRef,
		e.ReceiptDocID,
		e.ApprovedBy,
		e.ApprovedAt,
		metaJSON,
		e.ID,
	)

	result, err := scanExpense(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("fin: UpdateExpense: expense %s not found or already deleted", e.ID)
	}
	if err != nil {
		return nil, fmt.Errorf("fin: UpdateExpense: %w", err)
	}
	return result, nil
}

// SoftDeleteExpense marks the expense as deleted without removing the row.
func (r *PostgresBudgetRepository) SoftDeleteExpense(ctx context.Context, id uuid.UUID) error {
	const q = `
		UPDATE expenses
		SET deleted_at = now()
		WHERE id = $1 AND deleted_at IS NULL`

	_, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("fin: SoftDeleteExpense: %w", err)
	}
	return nil
}

// ─── Scan helpers ─────────────────────────────────────────────────────────────

// scanCategory reads a single budget_categories row.
func scanCategory(row pgx.Row) (*BudgetCategory, error) {
	var c BudgetCategory
	err := row.Scan(
		&c.ID,
		&c.OrgID,
		&c.Name,
		&c.CategoryType,
		&c.ParentID,
		&c.SortOrder,
		&c.IsReserve,
		&c.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// collectCategories drains pgx.Rows into a slice of BudgetCategory values.
func collectCategories(rows pgx.Rows, op string) ([]BudgetCategory, error) {
	categories := []BudgetCategory{}
	for rows.Next() {
		var c BudgetCategory
		if err := rows.Scan(
			&c.ID,
			&c.OrgID,
			&c.Name,
			&c.CategoryType,
			&c.ParentID,
			&c.SortOrder,
			&c.IsReserve,
			&c.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("fin: %s scan: %w", op, err)
		}
		categories = append(categories, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("fin: %s rows: %w", op, err)
	}
	return categories, nil
}

// scanBudget reads a single budgets row.
func scanBudget(row pgx.Row) (*Budget, error) {
	var b Budget
	err := row.Scan(
		&b.ID,
		&b.OrgID,
		&b.FiscalYear,
		&b.Name,
		&b.Status,
		&b.TotalIncomeCents,
		&b.TotalExpenseCents,
		&b.NetCents,
		&b.Notes,
		&b.ProposedAt,
		&b.ProposedBy,
		&b.ApprovedAt,
		&b.ApprovedBy,
		&b.DocumentID,
		&b.CreatedBy,
		&b.CreatedAt,
		&b.UpdatedAt,
		&b.DeletedAt,
	)
	if err != nil {
		return nil, err
	}
	return &b, nil
}

// collectBudgets drains pgx.Rows into a slice of Budget values.
func collectBudgets(rows pgx.Rows, op string) ([]Budget, error) {
	budgets := []Budget{}
	for rows.Next() {
		var b Budget
		if err := rows.Scan(
			&b.ID,
			&b.OrgID,
			&b.FiscalYear,
			&b.Name,
			&b.Status,
			&b.TotalIncomeCents,
			&b.TotalExpenseCents,
			&b.NetCents,
			&b.Notes,
			&b.ProposedAt,
			&b.ProposedBy,
			&b.ApprovedAt,
			&b.ApprovedBy,
			&b.DocumentID,
			&b.CreatedBy,
			&b.CreatedAt,
			&b.UpdatedAt,
			&b.DeletedAt,
		); err != nil {
			return nil, fmt.Errorf("fin: %s scan: %w", op, err)
		}
		budgets = append(budgets, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("fin: %s rows: %w", op, err)
	}
	return budgets, nil
}

// scanLineItem reads a single budget_line_items row.
func scanLineItem(row pgx.Row) (*BudgetLineItem, error) {
	var item BudgetLineItem
	err := row.Scan(
		&item.ID,
		&item.BudgetID,
		&item.CategoryID,
		&item.Description,
		&item.PlannedCents,
		&item.ActualCents,
		&item.Notes,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// collectLineItems drains pgx.Rows into a slice of BudgetLineItem values.
func collectLineItems(rows pgx.Rows, op string) ([]BudgetLineItem, error) {
	items := []BudgetLineItem{}
	for rows.Next() {
		var item BudgetLineItem
		if err := rows.Scan(
			&item.ID,
			&item.BudgetID,
			&item.CategoryID,
			&item.Description,
			&item.PlannedCents,
			&item.ActualCents,
			&item.Notes,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("fin: %s scan: %w", op, err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("fin: %s rows: %w", op, err)
	}
	return items, nil
}

// scanExpense reads a single expenses row.
func scanExpense(row pgx.Row) (*Expense, error) {
	var e Expense
	var metaRaw []byte
	var fundType FundType

	err := row.Scan(
		&e.ID,
		&e.OrgID,
		&e.CurrencyCode,
		&e.VendorID,
		&e.CategoryID,
		&e.BudgetID,
		&fundType,
		&e.Description,
		&e.AmountCents,
		&e.TaxCents,
		&e.TotalCents,
		&e.Status,
		&e.ExpenseDate,
		&e.DueDate,
		&e.PaidDate,
		&e.PaymentRef,
		&e.ReceiptDocID,
		&e.SubmittedBy,
		&e.ApprovedBy,
		&e.ApprovedAt,
		&metaRaw,
		&e.CreatedAt,
		&e.UpdatedAt,
		&e.DeletedAt,
	)
	if err != nil {
		return nil, err
	}

	e.FundType = &fundType

	if len(metaRaw) > 0 {
		if err := json.Unmarshal(metaRaw, &e.Metadata); err != nil {
			return nil, fmt.Errorf("unmarshal metadata: %w", err)
		}
	}
	if e.Metadata == nil {
		e.Metadata = map[string]any{}
	}

	return &e, nil
}

// collectExpenses drains pgx.Rows into a slice of Expense values.
func collectExpenses(rows pgx.Rows, op string) ([]Expense, error) {
	expenses := []Expense{}
	for rows.Next() {
		var e Expense
		var metaRaw []byte
		var fundType FundType

		if err := rows.Scan(
			&e.ID,
			&e.OrgID,
			&e.CurrencyCode,
			&e.VendorID,
			&e.CategoryID,
			&e.BudgetID,
			&fundType,
			&e.Description,
			&e.AmountCents,
			&e.TaxCents,
			&e.TotalCents,
			&e.Status,
			&e.ExpenseDate,
			&e.DueDate,
			&e.PaidDate,
			&e.PaymentRef,
			&e.ReceiptDocID,
			&e.SubmittedBy,
			&e.ApprovedBy,
			&e.ApprovedAt,
			&metaRaw,
			&e.CreatedAt,
			&e.UpdatedAt,
			&e.DeletedAt,
		); err != nil {
			return nil, fmt.Errorf("fin: %s scan: %w", op, err)
		}

		e.FundType = &fundType

		if len(metaRaw) > 0 {
			if err := json.Unmarshal(metaRaw, &e.Metadata); err != nil {
				return nil, fmt.Errorf("fin: %s unmarshal metadata: %w", op, err)
			}
		}
		if e.Metadata == nil {
			e.Metadata = map[string]any{}
		}
		expenses = append(expenses, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("fin: %s rows: %w", op, err)
	}
	return expenses, nil
}

// marshalMetadata marshals a metadata map to JSON, returning "{}" for nil
// maps.
func marshalMetadata(m map[string]any) ([]byte, error) {
	if m == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(m)
}
