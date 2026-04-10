package fin

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/platform/auth"
	"github.com/quorant/quorant/internal/platform/middleware"
)

// RegisterRoutes registers all finance module routes on the mux.
// All routes require authentication and tenant context middleware.
func RegisterRoutes(
	mux *http.ServeMux,
	assessmentHandler *AssessmentHandler,
	paymentHandler *PaymentHandler,
	budgetHandler *BudgetHandler,
	fundHandler *FundHandler,
	collectionHandler *CollectionHandler,
	glHandler *GLHandler,
	validator auth.TokenValidator,
	checker middleware.PermissionChecker,
	resolveUserID func(context.Context) (uuid.UUID, error),
) {
	permMw := func(perm string, h http.HandlerFunc) http.Handler {
		return middleware.Auth(validator,
			middleware.TenantContext(
				middleware.RequirePermission(checker, perm, resolveUserID)(
					http.HandlerFunc(h))))
	}

	// Assessment Schedules
	mux.Handle("POST /api/v1/organizations/{org_id}/assessment-schedules", permMw("fin.schedule.manage", assessmentHandler.CreateSchedule))
	mux.Handle("GET /api/v1/organizations/{org_id}/assessment-schedules", permMw("fin.schedule.read", assessmentHandler.ListSchedules))
	mux.Handle("GET /api/v1/organizations/{org_id}/assessment-schedules/{schedule_id}", permMw("fin.schedule.read", assessmentHandler.GetSchedule))
	mux.Handle("PATCH /api/v1/organizations/{org_id}/assessment-schedules/{schedule_id}", permMw("fin.schedule.manage", assessmentHandler.UpdateSchedule))
	mux.Handle("POST /api/v1/organizations/{org_id}/assessment-schedules/{schedule_id}/deactivate", permMw("fin.schedule.manage", assessmentHandler.DeactivateSchedule))

	// Assessments
	mux.Handle("POST /api/v1/organizations/{org_id}/assessments", permMw("fin.assessment.create", assessmentHandler.CreateAssessment))
	mux.Handle("GET /api/v1/organizations/{org_id}/assessments", permMw("fin.assessment.read", assessmentHandler.ListAssessments))
	mux.Handle("GET /api/v1/organizations/{org_id}/assessments/{assessment_id}", permMw("fin.assessment.read", assessmentHandler.GetAssessment))
	mux.Handle("PATCH /api/v1/organizations/{org_id}/assessments/{assessment_id}", permMw("fin.assessment.create", assessmentHandler.UpdateAssessment))
	mux.Handle("DELETE /api/v1/organizations/{org_id}/assessments/{assessment_id}", permMw("fin.assessment.create", assessmentHandler.DeleteAssessment))

	// Ledger
	mux.Handle("GET /api/v1/organizations/{org_id}/units/{unit_id}/ledger", permMw("fin.ledger.read", assessmentHandler.GetUnitLedger))
	mux.Handle("GET /api/v1/organizations/{org_id}/ledger", permMw("fin.ledger.read", assessmentHandler.GetOrgLedger))

	// Payments
	mux.Handle("POST /api/v1/organizations/{org_id}/payments", permMw("fin.payment.create", paymentHandler.RecordPayment))
	mux.Handle("GET /api/v1/organizations/{org_id}/payments", permMw("fin.payment.read", paymentHandler.ListPayments))
	mux.Handle("GET /api/v1/organizations/{org_id}/payments/{payment_id}", permMw("fin.payment.read", paymentHandler.GetPayment))
	mux.Handle("POST /api/v1/organizations/{org_id}/payments/{payment_id}/void", permMw("fin.payment.void", paymentHandler.VoidPayment))

	// Payment Methods
	mux.Handle("POST /api/v1/organizations/{org_id}/payment-methods", permMw("fin.payment_method.manage", paymentHandler.AddPaymentMethod))
	mux.Handle("GET /api/v1/organizations/{org_id}/payment-methods", permMw("fin.payment_method.manage", paymentHandler.ListPaymentMethods))
	mux.Handle("DELETE /api/v1/organizations/{org_id}/payment-methods/{method_id}", permMw("fin.payment_method.manage", paymentHandler.RemovePaymentMethod))

	// Budgets
	mux.Handle("POST /api/v1/organizations/{org_id}/budgets", permMw("fin.budget.create", budgetHandler.CreateBudget))
	mux.Handle("GET /api/v1/organizations/{org_id}/budgets", permMw("fin.budget.read", budgetHandler.ListBudgets))
	mux.Handle("GET /api/v1/organizations/{org_id}/budgets/{budget_id}", permMw("fin.budget.read", budgetHandler.GetBudget))
	mux.Handle("PATCH /api/v1/organizations/{org_id}/budgets/{budget_id}", permMw("fin.budget.create", budgetHandler.UpdateBudget))
	mux.Handle("GET /api/v1/organizations/{org_id}/budgets/{budget_id}/report", permMw("fin.budget.read", budgetHandler.GetBudgetReport))
	mux.Handle("POST /api/v1/organizations/{org_id}/budgets/{budget_id}/propose", permMw("fin.budget.approve", budgetHandler.ProposeBudget))
	mux.Handle("POST /api/v1/organizations/{org_id}/budgets/{budget_id}/approve", permMw("fin.budget.approve", budgetHandler.ApproveBudget))
	mux.Handle("POST /api/v1/organizations/{org_id}/budgets/{budget_id}/line-items", permMw("fin.budget.create", budgetHandler.CreateLineItem))
	mux.Handle("PATCH /api/v1/organizations/{org_id}/budgets/{budget_id}/line-items/{item_id}", permMw("fin.budget.create", budgetHandler.UpdateLineItem))
	mux.Handle("DELETE /api/v1/organizations/{org_id}/budgets/{budget_id}/line-items/{item_id}", permMw("fin.budget.create", budgetHandler.DeleteLineItem))

	// Budget Categories
	mux.Handle("POST /api/v1/organizations/{org_id}/budget-categories", permMw("fin.budget.create", budgetHandler.CreateCategory))
	mux.Handle("GET /api/v1/organizations/{org_id}/budget-categories", permMw("fin.budget.read", budgetHandler.ListCategories))
	mux.Handle("PATCH /api/v1/organizations/{org_id}/budget-categories/{category_id}", permMw("fin.budget.create", budgetHandler.UpdateCategory))

	// Expenses
	mux.Handle("POST /api/v1/organizations/{org_id}/expenses", permMw("fin.expense.submit", budgetHandler.CreateExpense))
	mux.Handle("GET /api/v1/organizations/{org_id}/expenses", permMw("fin.expense.read", budgetHandler.ListExpenses))
	mux.Handle("GET /api/v1/organizations/{org_id}/expenses/{expense_id}", permMw("fin.expense.read", budgetHandler.GetExpense))
	mux.Handle("PATCH /api/v1/organizations/{org_id}/expenses/{expense_id}", permMw("fin.expense.submit", budgetHandler.UpdateExpense))
	mux.Handle("POST /api/v1/organizations/{org_id}/expenses/{expense_id}/approve", permMw("fin.expense.approve", budgetHandler.ApproveExpense))
	mux.Handle("POST /api/v1/organizations/{org_id}/expenses/{expense_id}/pay", permMw("fin.expense.approve", budgetHandler.PayExpense))

	// Funds
	mux.Handle("POST /api/v1/organizations/{org_id}/funds", permMw("fin.fund.manage", fundHandler.CreateFund))
	mux.Handle("GET /api/v1/organizations/{org_id}/funds", permMw("fin.fund.read", fundHandler.ListFunds))
	mux.Handle("GET /api/v1/organizations/{org_id}/funds/{fund_id}", permMw("fin.fund.read", fundHandler.GetFund))
	mux.Handle("PATCH /api/v1/organizations/{org_id}/funds/{fund_id}", permMw("fin.fund.manage", fundHandler.UpdateFund))
	mux.Handle("GET /api/v1/organizations/{org_id}/funds/{fund_id}/transactions", permMw("fin.fund.read", fundHandler.GetFundTransactions))
	mux.Handle("POST /api/v1/organizations/{org_id}/fund-transfers", permMw("fin.fund.transfer", fundHandler.CreateFundTransfer))
	mux.Handle("GET /api/v1/organizations/{org_id}/fund-transfers", permMw("fin.fund.read", fundHandler.ListFundTransfers))

	// Collections
	mux.Handle("GET /api/v1/organizations/{org_id}/collections", permMw("fin.collection.read", collectionHandler.ListCollections))
	mux.Handle("GET /api/v1/organizations/{org_id}/collections/{case_id}", permMw("fin.collection.read", collectionHandler.GetCollection))
	mux.Handle("PATCH /api/v1/organizations/{org_id}/collections/{case_id}", permMw("fin.collection.manage", collectionHandler.UpdateCollection))
	mux.Handle("POST /api/v1/organizations/{org_id}/collections/{case_id}/actions", permMw("fin.collection.manage", collectionHandler.AddCollectionAction))
	mux.Handle("POST /api/v1/organizations/{org_id}/collections/{case_id}/payment-plans", permMw("fin.payment_plan.manage", collectionHandler.CreatePaymentPlan))
	mux.Handle("GET /api/v1/organizations/{org_id}/collections/{case_id}/payment-plans", permMw("fin.payment_plan.manage", collectionHandler.ListPaymentPlans))
	mux.Handle("GET /api/v1/organizations/{org_id}/units/{unit_id}/collection-status", permMw("fin.collection.read", collectionHandler.GetUnitCollectionStatus))
	mux.Handle("PATCH /api/v1/organizations/{org_id}/payment-plans/{plan_id}", permMw("fin.payment_plan.manage", collectionHandler.UpdatePaymentPlan))

	// GL: Chart of Accounts
	mux.Handle("POST /api/v1/organizations/{org_id}/gl/accounts", permMw("fin.gl.account.manage", glHandler.CreateAccount))
	mux.Handle("GET /api/v1/organizations/{org_id}/gl/accounts", permMw("fin.gl.account.read", glHandler.ListAccounts))
	mux.Handle("GET /api/v1/organizations/{org_id}/gl/accounts/{account_id}", permMw("fin.gl.account.read", glHandler.GetAccount))
	mux.Handle("PATCH /api/v1/organizations/{org_id}/gl/accounts/{account_id}", permMw("fin.gl.account.manage", glHandler.UpdateAccount))
	mux.Handle("DELETE /api/v1/organizations/{org_id}/gl/accounts/{account_id}", permMw("fin.gl.account.manage", glHandler.DeleteAccount))

	// GL: Journal Entries
	mux.Handle("POST /api/v1/organizations/{org_id}/gl/journal-entries", permMw("fin.gl.journal.create", glHandler.CreateJournalEntry))
	mux.Handle("GET /api/v1/organizations/{org_id}/gl/journal-entries", permMw("fin.gl.journal.read", glHandler.ListJournalEntries))
	mux.Handle("GET /api/v1/organizations/{org_id}/gl/journal-entries/{entry_id}", permMw("fin.gl.journal.read", glHandler.GetJournalEntry))

	// GL: Reports
	mux.Handle("GET /api/v1/organizations/{org_id}/gl/trial-balance", permMw("fin.gl.report.read", glHandler.GetTrialBalance))
	mux.Handle("GET /api/v1/organizations/{org_id}/gl/account-balances", permMw("fin.gl.report.read", glHandler.GetAccountBalances))
}
