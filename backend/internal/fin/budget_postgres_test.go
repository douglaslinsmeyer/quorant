//go:build integration

package fin_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/fin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Fixture ─────────────────────────────────────────────────────────────────

// budgetTestFixture holds all resources needed by budget repository tests.
type budgetTestFixture struct {
	repo   fin.BudgetRepository
	orgID  uuid.UUID
	userID uuid.UUID
}

// setupBudgetFixture creates a pool (reusing setupFinDB for DB connection and
// cleanup), a test org, and a test user.
func setupBudgetFixture(t *testing.T) budgetTestFixture {
	t.Helper()
	ctx := context.Background()
	pool := setupFinDB(t)

	var orgID uuid.UUID
	err := pool.QueryRow(ctx,
		`INSERT INTO organizations (type, name, slug, path, settings)
		 VALUES ('hoa', 'Budget Test HOA', $1, $2, '{}')
		 RETURNING id`,
		"budget-test-hoa-"+uuid.New().String(),
		"budget_test_hoa_"+uuid.New().String()[:8],
	).Scan(&orgID)
	require.NoError(t, err, "create test org")

	var userID uuid.UUID
	err = pool.QueryRow(ctx,
		`INSERT INTO users (idp_user_id, email, display_name)
		 VALUES ($1, $2, 'Budget Test User')
		 RETURNING id`,
		"budget-test-idp-"+uuid.New().String(),
		"budget-test-"+uuid.New().String()+"@example.com",
	).Scan(&userID)
	require.NoError(t, err, "create test user")

	repo := fin.NewPostgresBudgetRepository(pool)

	return budgetTestFixture{
		repo:   repo,
		orgID:  orgID,
		userID: userID,
	}
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func minimalCategory(orgID uuid.UUID) *fin.BudgetCategory {
	return &fin.BudgetCategory{
		OrgID:        orgID,
		Name:         "Maintenance",
		CategoryType: fin.BudgetCategoryTypeExpense,
		SortOrder:    0,
		IsReserve:    false,
	}
}

func minimalBudget(orgID, userID uuid.UUID) *fin.Budget {
	return &fin.Budget{
		OrgID:      orgID,
		FiscalYear: 2026,
		Name:       "FY2026 Operating Budget",
		Status:     fin.BudgetStatusDraft,
		CreatedBy:  userID,
	}
}

func minimalLineItem(budgetID, categoryID uuid.UUID) *fin.BudgetLineItem {
	return &fin.BudgetLineItem{
		BudgetID:     budgetID,
		CategoryID:   categoryID,
		PlannedCents: 500000,
		ActualCents:  0,
	}
}

func minimalExpense(orgID, userID uuid.UUID) *fin.Expense {
	expDate := time.Now().UTC().Truncate(24 * time.Hour)
	return &fin.Expense{
		OrgID:       orgID,
		Description: "Pool maintenance",
		AmountCents: 75000,
		TaxCents:    0,
		TotalCents:  75000,
		Status:      fin.ExpenseStatusPending,
		ExpenseDate: expDate,
		SubmittedBy: userID,
		Metadata:    map[string]any{},
	}
}

// ─── TestCreateCategory + ListCategoriesByOrg ────────────────────────────────

func TestCreateCategory(t *testing.T) {
	f := setupBudgetFixture(t)
	ctx := context.Background()

	input := minimalCategory(f.orgID)
	got, err := f.repo.CreateCategory(ctx, input)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.NotEqual(t, uuid.Nil, got.ID, "should have a generated UUID")
	assert.Equal(t, f.orgID, got.OrgID)
	assert.Equal(t, "Maintenance", got.Name)
	assert.Equal(t, fin.BudgetCategoryTypeExpense, got.CategoryType)
	assert.False(t, got.IsReserve)
	assert.False(t, got.CreatedAt.IsZero())
}

func TestListCategoriesByOrg(t *testing.T) {
	f := setupBudgetFixture(t)
	ctx := context.Background()

	c1 := minimalCategory(f.orgID)
	c1.Name = "Landscaping"
	_, err := f.repo.CreateCategory(ctx, c1)
	require.NoError(t, err)

	c2 := minimalCategory(f.orgID)
	c2.Name = "Insurance"
	c2.CategoryType = fin.BudgetCategoryTypeExpense
	_, err = f.repo.CreateCategory(ctx, c2)
	require.NoError(t, err)

	list, err := f.repo.ListCategoriesByOrg(ctx, f.orgID)

	require.NoError(t, err)
	require.Len(t, list, 2, "should return both categories")

	names := []string{list[0].Name, list[1].Name}
	assert.Contains(t, names, "Landscaping")
	assert.Contains(t, names, "Insurance")
}

func TestListCategoriesByOrg_EmptySliceWhenNone(t *testing.T) {
	f := setupBudgetFixture(t)
	ctx := context.Background()

	list, err := f.repo.ListCategoriesByOrg(ctx, f.orgID)

	require.NoError(t, err)
	assert.NotNil(t, list, "should return empty slice, not nil")
	assert.Empty(t, list)
}

// ─── TestCreateBudget + FindBudgetByID ───────────────────────────────────────

func TestCreateBudget(t *testing.T) {
	f := setupBudgetFixture(t)
	ctx := context.Background()

	input := minimalBudget(f.orgID, f.userID)
	got, err := f.repo.CreateBudget(ctx, input)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.NotEqual(t, uuid.Nil, got.ID)
	assert.Equal(t, f.orgID, got.OrgID)
	assert.Equal(t, 2026, got.FiscalYear)
	assert.Equal(t, "FY2026 Operating Budget", got.Name)
	assert.Equal(t, fin.BudgetStatusDraft, got.Status)
	assert.Equal(t, f.userID, got.CreatedBy)
	assert.Nil(t, got.DeletedAt)
	assert.False(t, got.CreatedAt.IsZero())
	assert.False(t, got.UpdatedAt.IsZero())
}

func TestFindBudgetByID(t *testing.T) {
	f := setupBudgetFixture(t)
	ctx := context.Background()

	created, err := f.repo.CreateBudget(ctx, minimalBudget(f.orgID, f.userID))
	require.NoError(t, err)

	found, err := f.repo.FindBudgetByID(ctx, created.ID)

	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, created.ID, found.ID)
	assert.Equal(t, created.Name, found.Name)
	assert.Equal(t, created.FiscalYear, found.FiscalYear)
}

func TestFindBudgetByID_ReturnsNilWhenNotFound(t *testing.T) {
	f := setupBudgetFixture(t)
	ctx := context.Background()

	found, err := f.repo.FindBudgetByID(ctx, uuid.New())

	require.NoError(t, err)
	assert.Nil(t, found, "should return nil for non-existent budget")
}

// ─── TestListBudgetsByOrg ────────────────────────────────────────────────────

func TestListBudgetsByOrg(t *testing.T) {
	f := setupBudgetFixture(t)
	ctx := context.Background()

	b1 := minimalBudget(f.orgID, f.userID)
	b1.FiscalYear = 2025
	b1.Name = "FY2025 Budget"
	_, err := f.repo.CreateBudget(ctx, b1)
	require.NoError(t, err)

	b2 := minimalBudget(f.orgID, f.userID)
	b2.FiscalYear = 2026
	b2.Name = "FY2026 Budget"
	_, err = f.repo.CreateBudget(ctx, b2)
	require.NoError(t, err)

	list, err := f.repo.ListBudgetsByOrg(ctx, f.orgID)

	require.NoError(t, err)
	require.Len(t, list, 2, "should return both budgets")
	// ordered fiscal_year DESC: 2026 first
	assert.Equal(t, 2026, list[0].FiscalYear, "most recent fiscal year should be first")
	assert.Equal(t, 2025, list[1].FiscalYear)
}

func TestListBudgetsByOrg_EmptySliceWhenNone(t *testing.T) {
	f := setupBudgetFixture(t)
	ctx := context.Background()

	list, err := f.repo.ListBudgetsByOrg(ctx, f.orgID)

	require.NoError(t, err)
	assert.NotNil(t, list, "should return empty slice, not nil")
	assert.Empty(t, list)
}

// ─── TestCreateLineItem + ListLineItemsByBudget ──────────────────────────────

func TestCreateLineItem(t *testing.T) {
	f := setupBudgetFixture(t)
	ctx := context.Background()

	cat, err := f.repo.CreateCategory(ctx, minimalCategory(f.orgID))
	require.NoError(t, err)

	budget, err := f.repo.CreateBudget(ctx, minimalBudget(f.orgID, f.userID))
	require.NoError(t, err)

	item := minimalLineItem(budget.ID, cat.ID)
	got, err := f.repo.CreateLineItem(ctx, item)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.NotEqual(t, uuid.Nil, got.ID)
	assert.Equal(t, budget.ID, got.BudgetID)
	assert.Equal(t, cat.ID, got.CategoryID)
	assert.Equal(t, int64(500000), got.PlannedCents)
	assert.Equal(t, int64(0), got.ActualCents)
	assert.False(t, got.CreatedAt.IsZero())
}

func TestListLineItemsByBudget(t *testing.T) {
	f := setupBudgetFixture(t)
	ctx := context.Background()

	cat, err := f.repo.CreateCategory(ctx, minimalCategory(f.orgID))
	require.NoError(t, err)

	budget, err := f.repo.CreateBudget(ctx, minimalBudget(f.orgID, f.userID))
	require.NoError(t, err)

	desc1 := "Lawn care"
	item1 := minimalLineItem(budget.ID, cat.ID)
	item1.Description = &desc1
	item1.PlannedCents = 100000
	_, err = f.repo.CreateLineItem(ctx, item1)
	require.NoError(t, err)

	desc2 := "Tree trimming"
	item2 := minimalLineItem(budget.ID, cat.ID)
	item2.Description = &desc2
	item2.PlannedCents = 50000
	_, err = f.repo.CreateLineItem(ctx, item2)
	require.NoError(t, err)

	list, err := f.repo.ListLineItemsByBudget(ctx, budget.ID)

	require.NoError(t, err)
	require.Len(t, list, 2, "should return both line items")

	descs := []string{}
	for _, li := range list {
		if li.Description != nil {
			descs = append(descs, *li.Description)
		}
	}
	assert.Contains(t, descs, "Lawn care")
	assert.Contains(t, descs, "Tree trimming")
}

func TestListLineItemsByBudget_EmptySliceWhenNone(t *testing.T) {
	f := setupBudgetFixture(t)
	ctx := context.Background()

	budget, err := f.repo.CreateBudget(ctx, minimalBudget(f.orgID, f.userID))
	require.NoError(t, err)

	list, err := f.repo.ListLineItemsByBudget(ctx, budget.ID)

	require.NoError(t, err)
	assert.NotNil(t, list, "should return empty slice, not nil")
	assert.Empty(t, list)
}

// ─── TestDeleteLineItem ──────────────────────────────────────────────────────

func TestDeleteLineItem(t *testing.T) {
	f := setupBudgetFixture(t)
	ctx := context.Background()

	cat, err := f.repo.CreateCategory(ctx, minimalCategory(f.orgID))
	require.NoError(t, err)

	budget, err := f.repo.CreateBudget(ctx, minimalBudget(f.orgID, f.userID))
	require.NoError(t, err)

	item, err := f.repo.CreateLineItem(ctx, minimalLineItem(budget.ID, cat.ID))
	require.NoError(t, err)

	err = f.repo.DeleteLineItem(ctx, item.ID)
	require.NoError(t, err)

	list, err := f.repo.ListLineItemsByBudget(ctx, budget.ID)
	require.NoError(t, err)
	assert.Empty(t, list, "line item should be removed after deletion")
}

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

// ─── TestCreateExpense + FindExpenseByID ─────────────────────────────────────

func TestCreateExpense(t *testing.T) {
	f := setupBudgetFixture(t)
	ctx := context.Background()

	input := minimalExpense(f.orgID, f.userID)
	got, err := f.repo.CreateExpense(ctx, input)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.NotEqual(t, uuid.Nil, got.ID)
	assert.Equal(t, f.orgID, got.OrgID)
	assert.Equal(t, "Pool maintenance", got.Description)
	assert.Equal(t, int64(75000), got.AmountCents)
	assert.Equal(t, int64(75000), got.TotalCents)
	assert.Equal(t, fin.ExpenseStatusPending, got.Status)
	assert.Equal(t, f.userID, got.SubmittedBy)
	assert.NotNil(t, got.Metadata, "metadata should not be nil")
	assert.Nil(t, got.DeletedAt)
	assert.False(t, got.CreatedAt.IsZero())
	assert.False(t, got.UpdatedAt.IsZero())
}

func TestFindExpenseByID(t *testing.T) {
	f := setupBudgetFixture(t)
	ctx := context.Background()

	created, err := f.repo.CreateExpense(ctx, minimalExpense(f.orgID, f.userID))
	require.NoError(t, err)

	found, err := f.repo.FindExpenseByID(ctx, created.ID)

	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, created.ID, found.ID)
	assert.Equal(t, created.Description, found.Description)
	assert.Equal(t, created.AmountCents, found.AmountCents)
}

func TestFindExpenseByID_ReturnsNilWhenNotFound(t *testing.T) {
	f := setupBudgetFixture(t)
	ctx := context.Background()

	found, err := f.repo.FindExpenseByID(ctx, uuid.New())

	require.NoError(t, err)
	assert.Nil(t, found, "should return nil for non-existent expense")
}

// ─── TestListExpensesByOrg ────────────────────────────────────────────────────

func TestListExpensesByOrg(t *testing.T) {
	f := setupBudgetFixture(t)
	ctx := context.Background()

	e1 := minimalExpense(f.orgID, f.userID)
	e1.Description = "Plumbing repair"
	_, err := f.repo.CreateExpense(ctx, e1)
	require.NoError(t, err)

	e2 := minimalExpense(f.orgID, f.userID)
	e2.Description = "Electrical work"
	_, err = f.repo.CreateExpense(ctx, e2)
	require.NoError(t, err)

	list, err := f.repo.ListExpensesByOrg(ctx, f.orgID)

	require.NoError(t, err)
	require.Len(t, list, 2, "should return both expenses")

	descs := []string{list[0].Description, list[1].Description}
	assert.Contains(t, descs, "Plumbing repair")
	assert.Contains(t, descs, "Electrical work")
}

func TestListExpensesByOrg_EmptySliceWhenNone(t *testing.T) {
	f := setupBudgetFixture(t)
	ctx := context.Background()

	list, err := f.repo.ListExpensesByOrg(ctx, f.orgID)

	require.NoError(t, err)
	assert.NotNil(t, list, "should return empty slice, not nil")
	assert.Empty(t, list)
}

func TestListExpensesByOrg_ExcludesSoftDeleted(t *testing.T) {
	f := setupBudgetFixture(t)
	ctx := context.Background()

	e1 := minimalExpense(f.orgID, f.userID)
	e1.Description = "Active expense"
	_, err := f.repo.CreateExpense(ctx, e1)
	require.NoError(t, err)

	e2 := minimalExpense(f.orgID, f.userID)
	e2.Description = "Deleted expense"
	created2, err := f.repo.CreateExpense(ctx, e2)
	require.NoError(t, err)

	err = f.repo.SoftDeleteExpense(ctx, created2.ID)
	require.NoError(t, err)

	list, err := f.repo.ListExpensesByOrg(ctx, f.orgID)

	require.NoError(t, err)
	require.Len(t, list, 1, "soft-deleted expense should be excluded")
	assert.Equal(t, "Active expense", list[0].Description)
}

// ─── TestSoftDeleteExpense ────────────────────────────────────────────────────

func TestSoftDeleteExpense(t *testing.T) {
	f := setupBudgetFixture(t)
	ctx := context.Background()

	created, err := f.repo.CreateExpense(ctx, minimalExpense(f.orgID, f.userID))
	require.NoError(t, err)
	require.Nil(t, created.DeletedAt, "newly created expense should not be deleted")

	err = f.repo.SoftDeleteExpense(ctx, created.ID)
	require.NoError(t, err)

	// FindExpenseByID should return nil after soft delete (WHERE deleted_at IS NULL).
	found, err := f.repo.FindExpenseByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Nil(t, found, "soft-deleted expense should not be returned by FindExpenseByID")
}
