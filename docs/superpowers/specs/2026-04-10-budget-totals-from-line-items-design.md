# Budget Totals Computed from Line Items

**Issue:** [#73 — P1: Budget totals not computed from line items](https://github.com/douglaslinsmeyer/quorant/issues/73)
**Date:** 2026-04-10

## Problem

`Budget.TotalIncomeCents`, `TotalExpenseCents`, and `NetCents` are stored in Postgres but initialized to 0 and never recalculated when line items change. The values are always stale.

## Design

### 1. Add `BudgetCategoryTypeIncome` Enum

Add `"income"` to `BudgetCategoryType` in `enums.go` alongside the existing `"expense"` value. Update `IsValid()` to accept it. The DB column is `TEXT`, so no migration is needed.

### 2. Repository: `RecalculateBudgetTotals`

Add a new method to `BudgetRepository`:

```go
RecalculateBudgetTotals(ctx context.Context, budgetID uuid.UUID) error
```

Postgres implementation runs a single SQL UPDATE that joins `budget_line_items` to `budget_categories`, sums `planned_cents` grouped by `category_type`, and writes the results into the budget row:

```sql
UPDATE budgets SET
    total_income_cents = COALESCE((
        SELECT SUM(li.planned_cents)
        FROM budget_line_items li
        JOIN budget_categories bc ON bc.id = li.category_id
        WHERE li.budget_id = $1 AND bc.category_type = 'income'
    ), 0),
    total_expense_cents = COALESCE((
        SELECT SUM(li.planned_cents)
        FROM budget_line_items li
        JOIN budget_categories bc ON bc.id = li.category_id
        WHERE li.budget_id = $1 AND bc.category_type = 'expense'
    ), 0),
    net_cents = COALESCE((
        SELECT SUM(li.planned_cents)
        FROM budget_line_items li
        JOIN budget_categories bc ON bc.id = li.category_id
        WHERE li.budget_id = $1 AND bc.category_type = 'income'
    ), 0) - COALESCE((
        SELECT SUM(li.planned_cents)
        FROM budget_line_items li
        JOIN budget_categories bc ON bc.id = li.category_id
        WHERE li.budget_id = $1 AND bc.category_type = 'expense'
    ), 0),
    updated_at = now()
WHERE id = $1
```

All sums default to 0 via `COALESCE` when no matching line items exist.

### 3. Repository: `FindLineItemByID`

Add a new method to `BudgetRepository`:

```go
FindLineItemByID(ctx context.Context, id uuid.UUID) (*BudgetLineItem, error)
```

Needed by `DeleteLineItem` to resolve the `BudgetID` before deletion so that totals can be recalculated afterward.

### 4. Service Layer Changes

Call `RecalculateBudgetTotals` after each line item mutation:

- **`CreateLineItem`** — after insert, call `RecalculateBudgetTotals(ctx, item.BudgetID)`
- **`UpdateLineItem`** — after update, call `RecalculateBudgetTotals(ctx, item.BudgetID)`
- **`DeleteLineItem`** — fetch line item via `FindLineItemByID` to get `BudgetID`, delete, then `RecalculateBudgetTotals(ctx, budgetID)`

If recalculation fails, the error propagates to the caller. Totals may be briefly stale in this case but the next successful mutation corrects them.

### 5. Testing

**Unit tests (service):**
- `CreateLineItem` calls `RecalculateBudgetTotals` after insert
- `UpdateLineItem` calls `RecalculateBudgetTotals` after update
- `DeleteLineItem` fetches item, deletes, then recalculates
- Recalculation failure propagates error

**Unit tests (repository):**
- `FindLineItemByID` returns item / not-found error
- `RecalculateBudgetTotals` with mixed income/expense line items computes correct sums
- `RecalculateBudgetTotals` with no line items zeros out all totals

**Integration tests:**
- Create budget with income + expense line items, verify totals
- Update a line item's `planned_cents`, verify totals update
- Delete a line item, verify totals decrease
- Delete all line items, verify totals are 0

**Enum tests:**
- `BudgetCategoryTypeIncome.IsValid()` returns true
