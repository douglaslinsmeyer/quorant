# Budget Totals Computed from Line Items — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Recompute `Budget.TotalIncomeCents`, `TotalExpenseCents`, and `NetCents` from line items whenever a line item is created, updated, or deleted.

**Architecture:** Add `BudgetCategoryTypeIncome` enum. Add `FindLineItemByID` and `RecalculateBudgetTotals` to `BudgetRepository`. Wire recalculation into the three service-layer line item mutation methods.

**Tech Stack:** Go, PostgreSQL, testify

---

### Task 1: Add `BudgetCategoryTypeIncome` Enum

**Files:**
- Modify: `backend/internal/fin/enums.go:290-301`
- Modify: `backend/internal/fin/enums_test.go:242-255`

- [ ] **Step 1: Write the failing test**

In `backend/internal/fin/enums_test.go`, update `TestBudgetCategoryType_IsValid` to include the new `income` value:

```go
func TestBudgetCategoryType_IsValid(t *testing.T) {
	valid := []BudgetCategoryType{BudgetCategoryTypeExpense, BudgetCategoryTypeIncome}
	for _, v := range valid {
		if !v.IsValid() {
			t.Errorf("expected %q to be valid", v)
		}
	}
	invalid := []BudgetCategoryType{"", "unknown", "Expense", "Income"}
	for _, v := range invalid {
		if v.IsValid() {
			t.Errorf("expected %q to be invalid", v)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/fin/... -run TestBudgetCategoryType_IsValid -short -count=1`
Expected: Compilation error — `BudgetCategoryTypeIncome` is undefined.

- [ ] **Step 3: Write minimal implementation**

In `backend/internal/fin/enums.go`, add the new constant and update `IsValid`:

```go
const (
	BudgetCategoryTypeExpense BudgetCategoryType = "expense"
	BudgetCategoryTypeIncome  BudgetCategoryType = "income"
)

func (s BudgetCategoryType) IsValid() bool {
	switch s {
	case BudgetCategoryTypeExpense, BudgetCategoryTypeIncome:
		return true
	}
	return false
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd backend && go test ./internal/fin/... -run TestBudgetCategoryType_IsValid -short -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/internal/fin/enums.go backend/internal/fin/enums_test.go
git commit -m "feat(fin): add BudgetCategoryTypeIncome enum value (#73)"
```

---

### Task 2: Add `FindLineItemByID` to Repository

**Files:**
- Modify: `backend/internal/fin/budget_repository.go:48-64`
- Modify: `backend/internal/fin/budget_postgres.go` (after line 291, before `ListLineItemsByBudget`)
- Modify: `backend/internal/fin/budget_postgres_test.go` (add new test)
- Modify: `backend/internal/fin/service_test.go:314-319,408-449` (update mock)

- [ ] **Step 1: Write the failing integration test**

In `backend/internal/fin/budget_postgres_test.go`, add after `TestDeleteLineItem`:

```go
// ─── TestFindLineItemByID ───────────────────────────────────────────────────

func TestFindLineItemByID(t *testing.T) {
	f := setupBudgetFixture(t)
	ctx := context.Background()

	cat, err := f.repo.CreateCategory(ctx, minimalCategory(f.orgID))
	require.NoError(t, err)

	budget, err := f.repo.CreateBudget(ctx, minimalBudget(f.orgID, f.userID))
	require.NoError(t, err)

	created, err := f.repo.CreateLineItem(ctx, minimalLineItem(budget.ID, cat.ID))
	require.NoError(t, err)

	found, err := f.repo.FindLineItemByID(ctx, created.ID)

	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, created.ID, found.ID)
	assert.Equal(t, created.BudgetID, found.BudgetID)
	assert.Equal(t, created.PlannedCents, found.PlannedCents)
}

func TestFindLineItemByID_ReturnsNilWhenNotFound(t *testing.T) {
	f := setupBudgetFixture(t)
	ctx := context.Background()

	found, err := f.repo.FindLineItemByID(ctx, uuid.New())

	require.NoError(t, err)
	assert.Nil(t, found, "should return nil for non-existent line item")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/fin/... -run TestFindLineItemByID -short -count=1`
Expected: Compilation error — `FindLineItemByID` is not defined on the interface.

- [ ] **Step 3: Add interface method**

In `backend/internal/fin/budget_repository.go`, add after the `CreateLineItem` method (after line 52):

```go
	// FindLineItemByID returns the line item with the given id, or nil, nil
	// if no matching row exists.
	FindLineItemByID(ctx context.Context, id uuid.UUID) (*BudgetLineItem, error)
```

- [ ] **Step 4: Add Postgres implementation**

In `backend/internal/fin/budget_postgres.go`, add after `CreateLineItem` (after line 291):

```go
// FindLineItemByID returns the line item with the given id, or nil, nil if
// no matching row exists.
func (r *PostgresBudgetRepository) FindLineItemByID(ctx context.Context, id uuid.UUID) (*BudgetLineItem, error) {
	const q = `
		SELECT id, budget_id, category_id, description, planned_cents, actual_cents,
		       notes, created_at, updated_at
		FROM budget_line_items
		WHERE id = $1`

	row := r.db.QueryRow(ctx, q, id)

	result, err := scanLineItem(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("fin: FindLineItemByID: %w", err)
	}
	return result, nil
}
```

- [ ] **Step 5: Add mock implementation**

In `backend/internal/fin/service_test.go`, add after `DeleteLineItem` mock (after line 450):

```go
func (m *mockBudgetRepo) FindLineItemByID(_ context.Context, id uuid.UUID) (*fin.BudgetLineItem, error) {
	for i := range m.lineItems {
		if m.lineItems[i].ID == id {
			out := m.lineItems[i]
			return &out, nil
		}
	}
	return nil, nil
}
```

- [ ] **Step 6: Run test to verify it passes**

Run: `cd backend && go test ./internal/fin/... -run TestFindLineItemByID -short -count=1`
Expected: PASS (unit tests compile and pass; integration tests skip in short mode)

- [ ] **Step 7: Commit**

```bash
git add backend/internal/fin/budget_repository.go backend/internal/fin/budget_postgres.go backend/internal/fin/budget_postgres_test.go backend/internal/fin/service_test.go
git commit -m "feat(fin): add FindLineItemByID to BudgetRepository (#73)"
```

---

### Task 3: Add `RecalculateBudgetTotals` to Repository

**Files:**
- Modify: `backend/internal/fin/budget_repository.go` (add method to interface)
- Modify: `backend/internal/fin/budget_postgres.go` (add implementation)
- Modify: `backend/internal/fin/budget_postgres_test.go` (add integration tests)
- Modify: `backend/internal/fin/service_test.go` (update mock)

- [ ] **Step 1: Write the failing integration test — mixed income/expense**

In `backend/internal/fin/budget_postgres_test.go`, add:

```go
// ─── TestRecalculateBudgetTotals ────────────────────────────────────────────

func TestRecalculateBudgetTotals_MixedIncomeExpense(t *testing.T) {
	f := setupBudgetFixture(t)
	ctx := context.Background()

	expenseCat, err := f.repo.CreateCategory(ctx, minimalCategory(f.orgID))
	require.NoError(t, err)

	incomeCat := &fin.BudgetCategory{
		OrgID:        f.orgID,
		Name:         "Assessment Income",
		CategoryType: fin.BudgetCategoryTypeIncome,
		SortOrder:    0,
	}
	incomeCat, err = f.repo.CreateCategory(ctx, incomeCat)
	require.NoError(t, err)

	budget, err := f.repo.CreateBudget(ctx, minimalBudget(f.orgID, f.userID))
	require.NoError(t, err)

	// Two expense line items: 500_00 + 300_00 = 800_00
	item1 := minimalLineItem(budget.ID, expenseCat.ID)
	item1.PlannedCents = 50000
	_, err = f.repo.CreateLineItem(ctx, item1)
	require.NoError(t, err)

	item2 := minimalLineItem(budget.ID, expenseCat.ID)
	item2.PlannedCents = 30000
	_, err = f.repo.CreateLineItem(ctx, item2)
	require.NoError(t, err)

	// One income line item: 1200_00
	item3 := minimalLineItem(budget.ID, incomeCat.ID)
	item3.PlannedCents = 120000
	_, err = f.repo.CreateLineItem(ctx, item3)
	require.NoError(t, err)

	err = f.repo.RecalculateBudgetTotals(ctx, budget.ID)
	require.NoError(t, err)

	updated, err := f.repo.FindBudgetByID(ctx, budget.ID)
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, int64(120000), updated.TotalIncomeCents, "income should be 1200.00")
	assert.Equal(t, int64(80000), updated.TotalExpenseCents, "expenses should be 800.00")
	assert.Equal(t, int64(40000), updated.NetCents, "net should be income - expense = 400.00")
}

func TestRecalculateBudgetTotals_NoLineItems(t *testing.T) {
	f := setupBudgetFixture(t)
	ctx := context.Background()

	budget, err := f.repo.CreateBudget(ctx, minimalBudget(f.orgID, f.userID))
	require.NoError(t, err)

	err = f.repo.RecalculateBudgetTotals(ctx, budget.ID)
	require.NoError(t, err)

	updated, err := f.repo.FindBudgetByID(ctx, budget.ID)
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, int64(0), updated.TotalIncomeCents)
	assert.Equal(t, int64(0), updated.TotalExpenseCents)
	assert.Equal(t, int64(0), updated.NetCents)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/fin/... -run TestRecalculateBudgetTotals -short -count=1`
Expected: Compilation error — `RecalculateBudgetTotals` is not defined on the interface.

- [ ] **Step 3: Add interface method**

In `backend/internal/fin/budget_repository.go`, add after `DeleteLineItem` (before the Expenses section):

```go
	// RecalculateBudgetTotals recomputes TotalIncomeCents, TotalExpenseCents,
	// and NetCents from the budget's line items joined to their categories.
	RecalculateBudgetTotals(ctx context.Context, budgetID uuid.UUID) error
```

- [ ] **Step 4: Add Postgres implementation**

In `backend/internal/fin/budget_postgres.go`, add after `DeleteLineItem`:

```go
// RecalculateBudgetTotals recomputes the budget's total_income_cents,
// total_expense_cents, and net_cents from its line items.
func (r *PostgresBudgetRepository) RecalculateBudgetTotals(ctx context.Context, budgetID uuid.UUID) error {
	const q = `
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
		WHERE id = $1`

	_, err := r.db.Exec(ctx, q, budgetID)
	if err != nil {
		return fmt.Errorf("fin: RecalculateBudgetTotals: %w", err)
	}
	return nil
}
```

- [ ] **Step 5: Add mock implementation**

In `backend/internal/fin/service_test.go`, add after the `FindLineItemByID` mock (added in Task 2):

```go
func (m *mockBudgetRepo) RecalculateBudgetTotals(_ context.Context, budgetID uuid.UUID) error {
	var totalIncome, totalExpense int64
	for _, item := range m.lineItems {
		if item.BudgetID != budgetID {
			continue
		}
		// Look up the category to determine type.
		for _, cat := range m.categories {
			if cat.ID == item.CategoryID {
				switch cat.CategoryType {
				case fin.BudgetCategoryTypeIncome:
					totalIncome += item.PlannedCents
				case fin.BudgetCategoryTypeExpense:
					totalExpense += item.PlannedCents
				}
				break
			}
		}
	}
	for i := range m.budgets {
		if m.budgets[i].ID == budgetID {
			m.budgets[i].TotalIncomeCents = totalIncome
			m.budgets[i].TotalExpenseCents = totalExpense
			m.budgets[i].NetCents = totalIncome - totalExpense
			return nil
		}
	}
	return nil
}
```

- [ ] **Step 6: Run test to verify it passes**

Run: `cd backend && go test ./internal/fin/... -run TestRecalculateBudgetTotals -short -count=1`
Expected: PASS (unit tests compile; integration tests skip in short mode)

- [ ] **Step 7: Commit**

```bash
git add backend/internal/fin/budget_repository.go backend/internal/fin/budget_postgres.go backend/internal/fin/budget_postgres_test.go backend/internal/fin/service_test.go
git commit -m "feat(fin): add RecalculateBudgetTotals to BudgetRepository (#73)"
```

---

### Task 4: Wire Recalculation into Service Layer

**Files:**
- Modify: `backend/internal/fin/service.go:547-562`
- Modify: `backend/internal/fin/service_test.go` (add new tests)

- [ ] **Step 1: Write the failing test — CreateLineItem recalculates totals**

In `backend/internal/fin/service_test.go`, add:

```go
func TestCreateLineItem_RecalculatesTotals(t *testing.T) {
	budgetRepo := &mockBudgetRepo{}
	svc := fin.NewFinService(nil, nil, budgetRepo, nil, nil, nil, nil, nil, testutil.DiscardLogger(), nil)

	ctx := context.Background()
	orgID := testutil.TestOrgID()
	userID := testutil.TestUserID()

	// Create an income category.
	incomeCat, err := budgetRepo.CreateCategory(ctx, &fin.BudgetCategory{
		OrgID:        orgID,
		Name:         "Assessment Income",
		CategoryType: fin.BudgetCategoryTypeIncome,
	})
	require.NoError(t, err)

	// Create a budget.
	budget, err := svc.CreateBudget(ctx, orgID, userID, &fin.CreateBudgetRequest{
		FiscalYear: 2026,
		Name:       "FY2026 Budget",
	})
	require.NoError(t, err)
	assert.Equal(t, int64(0), budget.TotalIncomeCents)

	// Create a line item — totals should recalculate.
	_, err = svc.CreateLineItem(ctx, budget.ID, &fin.BudgetLineItem{
		CategoryID:   incomeCat.ID,
		PlannedCents: 100000,
	})
	require.NoError(t, err)

	// Fetch the budget and verify totals were updated.
	updated, err := svc.GetBudget(ctx, budget.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(100000), updated.TotalIncomeCents)
	assert.Equal(t, int64(0), updated.TotalExpenseCents)
	assert.Equal(t, int64(100000), updated.NetCents)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/fin/... -run TestCreateLineItem_RecalculatesTotals -short -count=1`
Expected: FAIL — `TotalIncomeCents` is still 0 because the service doesn't call recalculate yet.

- [ ] **Step 3: Write the failing test — UpdateLineItem recalculates totals**

In `backend/internal/fin/service_test.go`, add:

```go
func TestUpdateLineItem_RecalculatesTotals(t *testing.T) {
	budgetRepo := &mockBudgetRepo{}
	svc := fin.NewFinService(nil, nil, budgetRepo, nil, nil, nil, nil, nil, testutil.DiscardLogger(), nil)

	ctx := context.Background()
	orgID := testutil.TestOrgID()
	userID := testutil.TestUserID()

	expenseCat, err := budgetRepo.CreateCategory(ctx, &fin.BudgetCategory{
		OrgID:        orgID,
		Name:         "Maintenance",
		CategoryType: fin.BudgetCategoryTypeExpense,
	})
	require.NoError(t, err)

	budget, err := svc.CreateBudget(ctx, orgID, userID, &fin.CreateBudgetRequest{
		FiscalYear: 2026,
		Name:       "FY2026 Budget",
	})
	require.NoError(t, err)

	item, err := svc.CreateLineItem(ctx, budget.ID, &fin.BudgetLineItem{
		CategoryID:   expenseCat.ID,
		PlannedCents: 50000,
	})
	require.NoError(t, err)

	// Update planned amount from 500.00 to 75000.
	_, err = svc.UpdateLineItem(ctx, item.ID, &fin.BudgetLineItem{
		CategoryID:   expenseCat.ID,
		PlannedCents: 75000,
	})
	require.NoError(t, err)

	updated, err := svc.GetBudget(ctx, budget.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(75000), updated.TotalExpenseCents)
	assert.Equal(t, int64(-75000), updated.NetCents)
}
```

- [ ] **Step 4: Write the failing test — DeleteLineItem recalculates totals**

In `backend/internal/fin/service_test.go`, add:

```go
func TestDeleteLineItem_RecalculatesTotals(t *testing.T) {
	budgetRepo := &mockBudgetRepo{}
	svc := fin.NewFinService(nil, nil, budgetRepo, nil, nil, nil, nil, nil, testutil.DiscardLogger(), nil)

	ctx := context.Background()
	orgID := testutil.TestOrgID()
	userID := testutil.TestUserID()

	expenseCat, err := budgetRepo.CreateCategory(ctx, &fin.BudgetCategory{
		OrgID:        orgID,
		Name:         "Maintenance",
		CategoryType: fin.BudgetCategoryTypeExpense,
	})
	require.NoError(t, err)

	budget, err := svc.CreateBudget(ctx, orgID, userID, &fin.CreateBudgetRequest{
		FiscalYear: 2026,
		Name:       "FY2026 Budget",
	})
	require.NoError(t, err)

	item, err := svc.CreateLineItem(ctx, budget.ID, &fin.BudgetLineItem{
		CategoryID:   expenseCat.ID,
		PlannedCents: 50000,
	})
	require.NoError(t, err)

	// Verify the total was set after create.
	b, err := svc.GetBudget(ctx, budget.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(50000), b.TotalExpenseCents)

	// Delete the line item — totals should zero out.
	err = svc.DeleteLineItem(ctx, item.ID)
	require.NoError(t, err)

	updated, err := svc.GetBudget(ctx, budget.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), updated.TotalExpenseCents)
	assert.Equal(t, int64(0), updated.TotalIncomeCents)
	assert.Equal(t, int64(0), updated.NetCents)
}
```

- [ ] **Step 5: Run tests to verify they fail**

Run: `cd backend && go test ./internal/fin/... -run "TestCreateLineItem_Recalculates|TestUpdateLineItem_Recalculates|TestDeleteLineItem_Recalculates" -short -count=1`
Expected: FAIL — totals not recalculated.

- [ ] **Step 6: Implement service changes**

In `backend/internal/fin/service.go`, replace the three methods (lines 547-562):

```go
// CreateLineItem creates a budget line item and recalculates budget totals.
func (s *FinService) CreateLineItem(ctx context.Context, budgetID uuid.UUID, item *BudgetLineItem) (*BudgetLineItem, error) {
	item.BudgetID = budgetID
	created, err := s.budgets.CreateLineItem(ctx, item)
	if err != nil {
		return nil, err
	}
	if err := s.budgets.RecalculateBudgetTotals(ctx, budgetID); err != nil {
		return nil, err
	}
	return created, nil
}

// UpdateLineItem persists changes to an existing budget line item and
// recalculates budget totals.
func (s *FinService) UpdateLineItem(ctx context.Context, id uuid.UUID, item *BudgetLineItem) (*BudgetLineItem, error) {
	item.ID = id
	updated, err := s.budgets.UpdateLineItem(ctx, item)
	if err != nil {
		return nil, err
	}
	if err := s.budgets.RecalculateBudgetTotals(ctx, updated.BudgetID); err != nil {
		return nil, err
	}
	return updated, nil
}

// DeleteLineItem hard-deletes a budget line item and recalculates budget totals.
func (s *FinService) DeleteLineItem(ctx context.Context, id uuid.UUID) error {
	item, err := s.budgets.FindLineItemByID(ctx, id)
	if err != nil {
		return err
	}
	if item == nil {
		return api.NewNotFoundError("budget.line_item.not_found")
	}
	if err := s.budgets.DeleteLineItem(ctx, id); err != nil {
		return err
	}
	return s.budgets.RecalculateBudgetTotals(ctx, item.BudgetID)
}
```

Note: `DeleteLineItem` now returns a not-found error if the line item doesn't exist, which is better behavior than silently succeeding.

- [ ] **Step 7: Run tests to verify they pass**

Run: `cd backend && go test ./internal/fin/... -run "TestCreateLineItem_Recalculates|TestUpdateLineItem_Recalculates|TestDeleteLineItem_Recalculates" -short -count=1`
Expected: PASS

- [ ] **Step 8: Run the full test suite to catch regressions**

Run: `cd backend && go test ./internal/fin/... -short -count=1`
Expected: All tests PASS

- [ ] **Step 9: Commit**

```bash
git add backend/internal/fin/service.go backend/internal/fin/service_test.go
git commit -m "feat(fin): recalculate budget totals on line item mutations (#73)"
```

---

### Task 5: Run Lint and Final Verification

**Files:** None (verification only)

- [ ] **Step 1: Run linter**

Run: `make lint`
Expected: PASS — no new warnings.

- [ ] **Step 2: Run full unit test suite**

Run: `make test`
Expected: All tests PASS.

- [ ] **Step 3: Close the issue (if authorized)**

```bash
gh issue close 73 --comment "Fixed: budget totals now recomputed from line items on every create/update/delete mutation."
```
