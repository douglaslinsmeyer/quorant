package fin

import (
	"log/slog"
	"net/http"

	"github.com/quorant/quorant/internal/platform/api"
	"github.com/quorant/quorant/internal/platform/middleware"
)

// BudgetHandler handles HTTP requests for budgets, line items, categories,
// and expenses.
type BudgetHandler struct {
	service *FinService
	logger  *slog.Logger
}

// NewBudgetHandler constructs a BudgetHandler backed by the given service.
func NewBudgetHandler(service *FinService, logger *slog.Logger) *BudgetHandler {
	return &BudgetHandler{service: service, logger: logger}
}

// ── Budgets ───────────────────────────────────────────────────────────────────

// CreateBudget handles POST /organizations/{org_id}/budgets.
func (h *BudgetHandler) CreateBudget(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseFinOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var req CreateBudgetRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}

	created, err := h.service.CreateBudget(r.Context(), orgID, middleware.UserIDFromContext(r.Context()), req)
	if err != nil {
		h.logger.Error("CreateBudget failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusCreated, created)
}

// ListBudgets handles GET /organizations/{org_id}/budgets.
func (h *BudgetHandler) ListBudgets(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseFinOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	budgets, err := h.service.ListBudgets(r.Context(), orgID)
	if err != nil {
		h.logger.Error("ListBudgets failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, budgets)
}

// GetBudget handles GET /organizations/{org_id}/budgets/{budget_id}.
func (h *BudgetHandler) GetBudget(w http.ResponseWriter, r *http.Request) {
	_, err := parseFinOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	budgetID, err := parsePathUUID(r, "budget_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	budget, err := h.service.GetBudget(r.Context(), budgetID)
	if err != nil {
		h.logger.Error("GetBudget failed", "budget_id", budgetID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, budget)
}

// UpdateBudget handles PATCH /organizations/{org_id}/budgets/{budget_id}.
func (h *BudgetHandler) UpdateBudget(w http.ResponseWriter, r *http.Request) {
	_, err := parseFinOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	budgetID, err := parsePathUUID(r, "budget_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var req UpdateBudgetRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}

	updated, err := h.service.UpdateBudget(r.Context(), budgetID, req)
	if err != nil {
		h.logger.Error("UpdateBudget failed", "budget_id", budgetID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, updated)
}

// ProposeBudget handles POST /organizations/{org_id}/budgets/{budget_id}/propose.
func (h *BudgetHandler) ProposeBudget(w http.ResponseWriter, r *http.Request) {
	_, err := parseFinOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	budgetID, err := parsePathUUID(r, "budget_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	updated, err := h.service.ProposeBudget(r.Context(), budgetID, middleware.UserIDFromContext(r.Context()))
	if err != nil {
		h.logger.Error("ProposeBudget failed", "budget_id", budgetID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, updated)
}

// ApproveBudget handles POST /organizations/{org_id}/budgets/{budget_id}/approve.
func (h *BudgetHandler) ApproveBudget(w http.ResponseWriter, r *http.Request) {
	_, err := parseFinOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	budgetID, err := parsePathUUID(r, "budget_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	updated, err := h.service.ApproveBudget(r.Context(), budgetID, middleware.UserIDFromContext(r.Context()))
	if err != nil {
		h.logger.Error("ApproveBudget failed", "budget_id", budgetID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, updated)
}

// ── Line Items ────────────────────────────────────────────────────────────────

// CreateLineItem handles POST /organizations/{org_id}/budgets/{budget_id}/line-items.
func (h *BudgetHandler) CreateLineItem(w http.ResponseWriter, r *http.Request) {
	_, err := parseFinOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	budgetID, err := parsePathUUID(r, "budget_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var item BudgetLineItem
	if err := api.ReadJSON(r, &item); err != nil {
		api.WriteError(w, err)
		return
	}

	created, err := h.service.CreateLineItem(r.Context(), budgetID, &item)
	if err != nil {
		h.logger.Error("CreateLineItem failed", "budget_id", budgetID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusCreated, created)
}

// UpdateLineItem handles PATCH /organizations/{org_id}/budgets/{budget_id}/line-items/{item_id}.
func (h *BudgetHandler) UpdateLineItem(w http.ResponseWriter, r *http.Request) {
	_, err := parseFinOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	_, err = parsePathUUID(r, "budget_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	itemID, err := parsePathUUID(r, "item_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var item BudgetLineItem
	if err := api.ReadJSON(r, &item); err != nil {
		api.WriteError(w, err)
		return
	}

	updated, err := h.service.UpdateLineItem(r.Context(), itemID, &item)
	if err != nil {
		h.logger.Error("UpdateLineItem failed", "item_id", itemID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, updated)
}

// DeleteLineItem handles DELETE /organizations/{org_id}/budgets/{budget_id}/line-items/{item_id}.
func (h *BudgetHandler) DeleteLineItem(w http.ResponseWriter, r *http.Request) {
	_, err := parseFinOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	_, err = parsePathUUID(r, "budget_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	itemID, err := parsePathUUID(r, "item_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	if err := h.service.DeleteLineItem(r.Context(), itemID); err != nil {
		h.logger.Error("DeleteLineItem failed", "item_id", itemID, "error", err)
		api.WriteError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ── Categories ────────────────────────────────────────────────────────────────

// CreateCategory handles POST /organizations/{org_id}/budget-categories.
func (h *BudgetHandler) CreateCategory(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseFinOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var c BudgetCategory
	if err := api.ReadJSON(r, &c); err != nil {
		api.WriteError(w, err)
		return
	}

	created, err := h.service.CreateCategory(r.Context(), orgID, &c)
	if err != nil {
		h.logger.Error("CreateCategory failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusCreated, created)
}

// ListCategories handles GET /organizations/{org_id}/budget-categories.
func (h *BudgetHandler) ListCategories(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseFinOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	categories, err := h.service.ListCategories(r.Context(), orgID)
	if err != nil {
		h.logger.Error("ListCategories failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, categories)
}

// ── Expenses ──────────────────────────────────────────────────────────────────

// CreateExpense handles POST /organizations/{org_id}/expenses.
func (h *BudgetHandler) CreateExpense(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseFinOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var req CreateExpenseRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}

	created, err := h.service.CreateExpense(r.Context(), orgID, middleware.UserIDFromContext(r.Context()), req)
	if err != nil {
		h.logger.Error("CreateExpense failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusCreated, created)
}

// ListExpenses handles GET /organizations/{org_id}/expenses.
func (h *BudgetHandler) ListExpenses(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseFinOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	expenses, err := h.service.ListExpenses(r.Context(), orgID)
	if err != nil {
		h.logger.Error("ListExpenses failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, expenses)
}

// GetExpense handles GET /organizations/{org_id}/expenses/{expense_id}.
func (h *BudgetHandler) GetExpense(w http.ResponseWriter, r *http.Request) {
	_, err := parseFinOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	expenseID, err := parsePathUUID(r, "expense_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	expense, err := h.service.GetExpense(r.Context(), expenseID)
	if err != nil {
		h.logger.Error("GetExpense failed", "expense_id", expenseID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, expense)
}

// ApproveExpense handles POST /organizations/{org_id}/expenses/{expense_id}/approve.
func (h *BudgetHandler) ApproveExpense(w http.ResponseWriter, r *http.Request) {
	_, err := parseFinOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	expenseID, err := parsePathUUID(r, "expense_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	updated, err := h.service.ApproveExpense(r.Context(), expenseID, middleware.UserIDFromContext(r.Context()))
	if err != nil {
		h.logger.Error("ApproveExpense failed", "expense_id", expenseID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, updated)
}

// PayExpense handles POST /organizations/{org_id}/expenses/{expense_id}/pay.
func (h *BudgetHandler) PayExpense(w http.ResponseWriter, r *http.Request) {
	_, err := parseFinOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	expenseID, err := parsePathUUID(r, "expense_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	updated, err := h.service.PayExpense(r.Context(), expenseID)
	if err != nil {
		h.logger.Error("PayExpense failed", "expense_id", expenseID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, updated)
}
