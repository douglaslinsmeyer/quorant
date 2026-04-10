package fin_test

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/ai"
	"github.com/quorant/quorant/internal/audit"
	"github.com/quorant/quorant/internal/fin"
	"github.com/quorant/quorant/internal/platform/middleware"
	"github.com/quorant/quorant/internal/platform/queue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── Test server setup ─────────────────────────────────────────────────────────

type budgetTestServer struct {
	server         *httptest.Server
	mockBudgetRepo *mockBudgetRepo
	userID         uuid.UUID
}

func setupBudgetTestServer(t *testing.T) *budgetTestServer {
	t.Helper()

	mockAssessRepo := &mockAssessmentRepo{}
	mockPaymentRepo := &mockPaymentRepo{}
	mockBudgetRepo := &mockBudgetRepo{}
	mockFundRepo := &mockFundRepo{}
	mockCollectionRepo := &mockCollectionRepo{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	service := fin.NewFinService(
		mockAssessRepo,
		mockPaymentRepo,
		mockBudgetRepo,
		mockFundRepo,
		mockCollectionRepo,
		nil,
		audit.NewNoopAuditor(),
		queue.NewInMemoryPublisher(),
		ai.NewNoopPolicyResolver(),
		ai.NewNoopComplianceResolver(),
		logger,
	)
	budgetHandler := fin.NewBudgetHandler(service, logger)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /organizations/{org_id}/budgets", budgetHandler.CreateBudget)
	mux.HandleFunc("GET /organizations/{org_id}/budgets", budgetHandler.ListBudgets)
	mux.HandleFunc("GET /organizations/{org_id}/budgets/{budget_id}", budgetHandler.GetBudget)
	mux.HandleFunc("PATCH /organizations/{org_id}/budgets/{budget_id}", budgetHandler.UpdateBudget)
	mux.HandleFunc("POST /organizations/{org_id}/budgets/{budget_id}/propose", budgetHandler.ProposeBudget)
	mux.HandleFunc("POST /organizations/{org_id}/budgets/{budget_id}/approve", budgetHandler.ApproveBudget)
	mux.HandleFunc("POST /organizations/{org_id}/budgets/{budget_id}/line-items", budgetHandler.CreateLineItem)
	mux.HandleFunc("PATCH /organizations/{org_id}/budgets/{budget_id}/line-items/{item_id}", budgetHandler.UpdateLineItem)
	mux.HandleFunc("DELETE /organizations/{org_id}/budgets/{budget_id}/line-items/{item_id}", budgetHandler.DeleteLineItem)
	mux.HandleFunc("GET /organizations/{org_id}/budgets/{budget_id}/report", budgetHandler.GetBudgetReport)
	mux.HandleFunc("POST /organizations/{org_id}/budget-categories", budgetHandler.CreateCategory)
	mux.HandleFunc("GET /organizations/{org_id}/budget-categories", budgetHandler.ListCategories)
	mux.HandleFunc("PATCH /organizations/{org_id}/budget-categories/{category_id}", budgetHandler.UpdateCategory)
	mux.HandleFunc("POST /organizations/{org_id}/expenses", budgetHandler.CreateExpense)
	mux.HandleFunc("GET /organizations/{org_id}/expenses", budgetHandler.ListExpenses)
	mux.HandleFunc("GET /organizations/{org_id}/expenses/{expense_id}", budgetHandler.GetExpense)
	mux.HandleFunc("PATCH /organizations/{org_id}/expenses/{expense_id}", budgetHandler.UpdateExpense)
	mux.HandleFunc("POST /organizations/{org_id}/expenses/{expense_id}/approve", budgetHandler.ApproveExpense)
	mux.HandleFunc("POST /organizations/{org_id}/expenses/{expense_id}/pay", budgetHandler.PayExpense)

	testUserID := uuid.New()
	handlerWithUserID := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := middleware.WithUserID(r.Context(), testUserID)
		mux.ServeHTTP(w, r.WithContext(ctx))
	})
	server := httptest.NewServer(handlerWithUserID)
	t.Cleanup(server.Close)

	return &budgetTestServer{
		server:         server,
		mockBudgetRepo: mockBudgetRepo,
		userID:         testUserID,
	}
}

// ── Seed helpers ──────────────────────────────────────────────────────────────

func seedBudget(t *testing.T, repo *mockBudgetRepo, orgID uuid.UUID) *fin.Budget {
	t.Helper()
	b := fin.Budget{
		ID:         uuid.New(),
		OrgID:      orgID,
		FiscalYear: 2026,
		Name:       "Annual HOA Budget",
		Status:     fin.BudgetStatusDraft,
		CreatedBy:  uuid.New(),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	repo.budgets = append(repo.budgets, b)
	return &repo.budgets[len(repo.budgets)-1]
}

func seedBudgetLineItem(t *testing.T, repo *mockBudgetRepo, budgetID, categoryID uuid.UUID) *fin.BudgetLineItem {
	t.Helper()
	item := fin.BudgetLineItem{
		ID:           uuid.New(),
		BudgetID:     budgetID,
		CategoryID:   categoryID,
		PlannedCents: 50000,
		ActualCents:  0,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	repo.lineItems = append(repo.lineItems, item)
	return &repo.lineItems[len(repo.lineItems)-1]
}

func seedExpense(t *testing.T, repo *mockBudgetRepo, orgID uuid.UUID, status fin.ExpenseStatus) *fin.Expense {
	t.Helper()
	e := fin.Expense{
		ID:          uuid.New(),
		OrgID:       orgID,
		Description: "Landscaping Q1",
		AmountCents: 75000,
		TaxCents:    0,
		TotalCents:  75000,
		Status:      status,
		ExpenseDate: time.Now(),
		SubmittedBy: uuid.Nil,
		Metadata:    map[string]any{},
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	repo.expenses = append(repo.expenses, e)
	return &repo.expenses[len(repo.expenses)-1]
}

// ── CreateBudget tests ────────────────────────────────────────────────────────

func TestCreateBudget_Success(t *testing.T) {
	ts := setupBudgetTestServer(t)
	orgID := uuid.New()

	body := map[string]any{
		"fiscal_year": 2026,
		"name":        "Annual HOA Budget 2026",
	}
	resp := doFinRequest(t, ts.server.URL, http.MethodPost,
		fmt.Sprintf("/organizations/%s/budgets", orgID), body)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var envelope struct {
		Data *fin.Budget `json:"data"`
	}
	decodeFinBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, orgID, envelope.Data.OrgID)
	assert.Equal(t, 2026, envelope.Data.FiscalYear)
	assert.Equal(t, "Annual HOA Budget 2026", envelope.Data.Name)
	assert.Equal(t, fin.BudgetStatusDraft, envelope.Data.Status)
	assert.NotEqual(t, uuid.Nil, envelope.Data.ID)
}

func TestCreateBudget_InvalidBody(t *testing.T) {
	ts := setupBudgetTestServer(t)
	orgID := uuid.New()

	// Missing required fiscal_year
	body := map[string]any{"name": "Incomplete Budget"}
	resp := doFinRequest(t, ts.server.URL, http.MethodPost,
		fmt.Sprintf("/organizations/%s/budgets", orgID), body)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestCreateBudget_InvalidOrgID(t *testing.T) {
	ts := setupBudgetTestServer(t)

	body := map[string]any{"fiscal_year": 2026, "name": "Test Budget"}
	resp := doFinRequest(t, ts.server.URL, http.MethodPost,
		"/organizations/not-a-uuid/budgets", body)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// ── ListBudgets tests ─────────────────────────────────────────────────────────

func TestListBudgets_Success(t *testing.T) {
	ts := setupBudgetTestServer(t)
	orgID := uuid.New()
	seedBudget(t, ts.mockBudgetRepo, orgID)
	seedBudget(t, ts.mockBudgetRepo, orgID)

	resp := doFinRequest(t, ts.server.URL, http.MethodGet,
		fmt.Sprintf("/organizations/%s/budgets", orgID), nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data []fin.Budget `json:"data"`
	}
	decodeFinBody(t, resp, &envelope)
	assert.Len(t, envelope.Data, 2)
}

// ── GetBudget tests ───────────────────────────────────────────────────────────

func TestGetBudget_Success(t *testing.T) {
	ts := setupBudgetTestServer(t)
	orgID := uuid.New()
	budget := seedBudget(t, ts.mockBudgetRepo, orgID)

	resp := doFinRequest(t, ts.server.URL, http.MethodGet,
		fmt.Sprintf("/organizations/%s/budgets/%s", orgID, budget.ID), nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data *fin.Budget `json:"data"`
	}
	decodeFinBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, budget.ID, envelope.Data.ID)
}

func TestGetBudget_NotFound(t *testing.T) {
	ts := setupBudgetTestServer(t)
	orgID := uuid.New()

	resp := doFinRequest(t, ts.server.URL, http.MethodGet,
		fmt.Sprintf("/organizations/%s/budgets/%s", orgID, uuid.New()), nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// ── ProposeBudget tests ───────────────────────────────────────────────────────

func TestProposeBudget_Success(t *testing.T) {
	ts := setupBudgetTestServer(t)
	orgID := uuid.New()
	budget := seedBudget(t, ts.mockBudgetRepo, orgID)

	resp := doFinRequest(t, ts.server.URL, http.MethodPost,
		fmt.Sprintf("/organizations/%s/budgets/%s/propose", orgID, budget.ID), nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data *fin.Budget `json:"data"`
	}
	decodeFinBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, fin.BudgetStatusProposed, envelope.Data.Status)
	assert.NotNil(t, envelope.Data.ProposedAt)
}

func TestProposeBudget_NotFound(t *testing.T) {
	ts := setupBudgetTestServer(t)
	orgID := uuid.New()

	resp := doFinRequest(t, ts.server.URL, http.MethodPost,
		fmt.Sprintf("/organizations/%s/budgets/%s/propose", orgID, uuid.New()), nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// ── ApproveBudget tests ───────────────────────────────────────────────────────

func TestApproveBudget_Success(t *testing.T) {
	ts := setupBudgetTestServer(t)
	orgID := uuid.New()
	budget := seedBudget(t, ts.mockBudgetRepo, orgID)

	// Must propose first.
	budget.Status = fin.BudgetStatusProposed
	ts.mockBudgetRepo.budgets[0] = *budget

	resp := doFinRequest(t, ts.server.URL, http.MethodPost,
		fmt.Sprintf("/organizations/%s/budgets/%s/approve", orgID, budget.ID), nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data *fin.Budget `json:"data"`
	}
	decodeFinBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, fin.BudgetStatusApproved, envelope.Data.Status)
	assert.NotNil(t, envelope.Data.ApprovedAt)
}

func TestApproveBudget_WrongStatus(t *testing.T) {
	ts := setupBudgetTestServer(t)
	orgID := uuid.New()
	budget := seedBudget(t, ts.mockBudgetRepo, orgID) // status = "draft"

	resp := doFinRequest(t, ts.server.URL, http.MethodPost,
		fmt.Sprintf("/organizations/%s/budgets/%s/approve", orgID, budget.ID), nil)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// ── CreateLineItem tests ──────────────────────────────────────────────────────

func TestCreateLineItem_Success(t *testing.T) {
	ts := setupBudgetTestServer(t)
	orgID := uuid.New()
	budget := seedBudget(t, ts.mockBudgetRepo, orgID)
	categoryID := uuid.New()

	body := map[string]any{
		"category_id":   categoryID,
		"planned_cents": 50000,
	}
	resp := doFinRequest(t, ts.server.URL, http.MethodPost,
		fmt.Sprintf("/organizations/%s/budgets/%s/line-items", orgID, budget.ID), body)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var envelope struct {
		Data *fin.BudgetLineItem `json:"data"`
	}
	decodeFinBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, budget.ID, envelope.Data.BudgetID)
	assert.Equal(t, categoryID, envelope.Data.CategoryID)
	assert.Equal(t, int64(50000), envelope.Data.PlannedCents)
	assert.NotEqual(t, uuid.Nil, envelope.Data.ID)
}

// ── DeleteLineItem tests ──────────────────────────────────────────────────────

func TestDeleteLineItem_Success(t *testing.T) {
	ts := setupBudgetTestServer(t)
	orgID := uuid.New()
	budget := seedBudget(t, ts.mockBudgetRepo, orgID)
	categoryID := uuid.New()
	item := seedBudgetLineItem(t, ts.mockBudgetRepo, budget.ID, categoryID)

	resp := doFinRequest(t, ts.server.URL, http.MethodDelete,
		fmt.Sprintf("/organizations/%s/budgets/%s/line-items/%s", orgID, budget.ID, item.ID), nil)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	resp.Body.Close()

	// Verify it was deleted.
	assert.Empty(t, ts.mockBudgetRepo.lineItems)
}

func TestDeleteLineItem_InvalidItemID(t *testing.T) {
	ts := setupBudgetTestServer(t)
	orgID := uuid.New()
	budget := seedBudget(t, ts.mockBudgetRepo, orgID)

	resp := doFinRequest(t, ts.server.URL, http.MethodDelete,
		fmt.Sprintf("/organizations/%s/budgets/%s/line-items/bad-uuid", orgID, budget.ID), nil)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// ── CreateExpense tests ───────────────────────────────────────────────────────

func TestCreateExpense_Success(t *testing.T) {
	ts := setupBudgetTestServer(t)
	orgID := uuid.New()

	body := map[string]any{
		"description":  "Landscaping Q1",
		"amount_cents": 75000,
		"expense_date": time.Now().Format(time.RFC3339),
	}
	resp := doFinRequest(t, ts.server.URL, http.MethodPost,
		fmt.Sprintf("/organizations/%s/expenses", orgID), body)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var envelope struct {
		Data *fin.Expense `json:"data"`
	}
	decodeFinBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, orgID, envelope.Data.OrgID)
	assert.Equal(t, "Landscaping Q1", envelope.Data.Description)
	assert.Equal(t, int64(75000), envelope.Data.AmountCents)
	assert.Equal(t, fin.ExpenseStatusPending, envelope.Data.Status)
	assert.NotEqual(t, uuid.Nil, envelope.Data.ID)
}

func TestCreateExpense_InvalidBody(t *testing.T) {
	ts := setupBudgetTestServer(t)
	orgID := uuid.New()

	// Missing required fields
	body := map[string]any{"description": "Partial expense"}
	resp := doFinRequest(t, ts.server.URL, http.MethodPost,
		fmt.Sprintf("/organizations/%s/expenses", orgID), body)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// ── ApproveExpense tests ──────────────────────────────────────────────────────

func TestApproveExpense_Success(t *testing.T) {
	ts := setupBudgetTestServer(t)
	orgID := uuid.New()
	expense := seedExpense(t, ts.mockBudgetRepo, orgID, fin.ExpenseStatusPending)

	resp := doFinRequest(t, ts.server.URL, http.MethodPost,
		fmt.Sprintf("/organizations/%s/expenses/%s/approve", orgID, expense.ID), nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data *fin.Expense `json:"data"`
	}
	decodeFinBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, fin.ExpenseStatusApproved, envelope.Data.Status)
	assert.NotNil(t, envelope.Data.ApprovedAt)
}

func TestApproveExpense_NotFound(t *testing.T) {
	ts := setupBudgetTestServer(t)
	orgID := uuid.New()

	resp := doFinRequest(t, ts.server.URL, http.MethodPost,
		fmt.Sprintf("/organizations/%s/expenses/%s/approve", orgID, uuid.New()), nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// ── PayExpense tests ──────────────────────────────────────────────────────────

func TestPayExpense_Success(t *testing.T) {
	ts := setupBudgetTestServer(t)
	orgID := uuid.New()
	expense := seedExpense(t, ts.mockBudgetRepo, orgID, fin.ExpenseStatusApproved)

	resp := doFinRequest(t, ts.server.URL, http.MethodPost,
		fmt.Sprintf("/organizations/%s/expenses/%s/pay", orgID, expense.ID), nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data *fin.Expense `json:"data"`
	}
	decodeFinBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, fin.ExpenseStatusPaid, envelope.Data.Status)
	assert.NotNil(t, envelope.Data.PaidDate)
}

func TestPayExpense_WrongStatus(t *testing.T) {
	ts := setupBudgetTestServer(t)
	orgID := uuid.New()
	expense := seedExpense(t, ts.mockBudgetRepo, orgID, fin.ExpenseStatusPending)

	resp := doFinRequest(t, ts.server.URL, http.MethodPost,
		fmt.Sprintf("/organizations/%s/expenses/%s/pay", orgID, expense.ID), nil)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// ── GetBudgetReport tests ─────────────────────────────────────────────────────

func TestGetBudgetReport_Success(t *testing.T) {
	ts := setupBudgetTestServer(t)
	orgID := uuid.New()
	budget := seedBudget(t, ts.mockBudgetRepo, orgID)
	// Add a line item to the budget.
	ts.mockBudgetRepo.lineItems = append(ts.mockBudgetRepo.lineItems, fin.BudgetLineItem{
		ID:           uuid.New(),
		BudgetID:     budget.ID,
		CategoryID:   uuid.New(),
		PlannedCents: 100000,
		ActualCents:  80000,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	})

	resp := doFinRequest(t, ts.server.URL, http.MethodGet,
		fmt.Sprintf("/organizations/%s/budgets/%s/report", orgID, budget.ID), nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data *fin.BudgetReport `json:"data"`
	}
	decodeFinBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	require.NotNil(t, envelope.Data.Budget)
	assert.Equal(t, budget.ID, envelope.Data.Budget.ID)
	assert.Len(t, envelope.Data.LineItems, 1)
}

func TestGetBudgetReport_NotFound(t *testing.T) {
	ts := setupBudgetTestServer(t)
	orgID := uuid.New()

	resp := doFinRequest(t, ts.server.URL, http.MethodGet,
		fmt.Sprintf("/organizations/%s/budgets/%s/report", orgID, uuid.New()), nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()
}

// ── UpdateCategory tests ──────────────────────────────────────────────────────

func TestUpdateCategory_Success(t *testing.T) {
	ts := setupBudgetTestServer(t)
	orgID := uuid.New()
	cat := fin.BudgetCategory{
		ID:           uuid.New(),
		OrgID:        orgID,
		Name:         "Landscaping",
		CategoryType: fin.BudgetCategoryTypeExpense,
		CreatedAt:    time.Now(),
	}
	ts.mockBudgetRepo.categories = append(ts.mockBudgetRepo.categories, cat)

	body := map[string]any{"name": "Landscaping & Grounds", "category_type": "expense"}
	resp := doFinRequest(t, ts.server.URL, http.MethodPatch,
		fmt.Sprintf("/organizations/%s/budget-categories/%s", orgID, cat.ID), body)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data *fin.BudgetCategory `json:"data"`
	}
	decodeFinBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, "Landscaping & Grounds", envelope.Data.Name)
}

// ── UpdateExpense tests ───────────────────────────────────────────────────────

func TestUpdateExpense_Success(t *testing.T) {
	ts := setupBudgetTestServer(t)
	orgID := uuid.New()
	expense := seedExpense(t, ts.mockBudgetRepo, orgID, fin.ExpenseStatusPending)

	body := map[string]any{
		"description":  "Updated vendor invoice",
		"amount_cents": 75000,
		"expense_date": time.Now().Format(time.RFC3339),
		"status":       "pending",
	}
	resp := doFinRequest(t, ts.server.URL, http.MethodPatch,
		fmt.Sprintf("/organizations/%s/expenses/%s", orgID, expense.ID), body)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data *fin.Expense `json:"data"`
	}
	decodeFinBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, "Updated vendor invoice", envelope.Data.Description)
}

func TestUpdateExpense_NotFound(t *testing.T) {
	ts := setupBudgetTestServer(t)
	orgID := uuid.New()

	body := map[string]any{
		"description":  "Ghost",
		"amount_cents": 1000,
		"expense_date": time.Now().Format(time.RFC3339),
		"status":       "pending",
	}
	resp := doFinRequest(t, ts.server.URL, http.MethodPatch,
		fmt.Sprintf("/organizations/%s/expenses/%s", orgID, uuid.New()), body)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()
}
