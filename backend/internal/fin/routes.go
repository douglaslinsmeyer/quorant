package fin

import (
	"net/http"

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
	validator auth.TokenValidator,
) {
	orgMw := func(h http.HandlerFunc) http.Handler {
		return middleware.Auth(validator, middleware.TenantContext(http.HandlerFunc(h)))
	}

	// Assessment Schedules
	mux.Handle("POST /api/v1/organizations/{org_id}/assessment-schedules", orgMw(assessmentHandler.CreateSchedule))
	mux.Handle("GET /api/v1/organizations/{org_id}/assessment-schedules", orgMw(assessmentHandler.ListSchedules))
	mux.Handle("GET /api/v1/organizations/{org_id}/assessment-schedules/{schedule_id}", orgMw(assessmentHandler.GetSchedule))
	mux.Handle("PATCH /api/v1/organizations/{org_id}/assessment-schedules/{schedule_id}", orgMw(assessmentHandler.UpdateSchedule))
	mux.Handle("POST /api/v1/organizations/{org_id}/assessment-schedules/{schedule_id}/deactivate", orgMw(assessmentHandler.DeactivateSchedule))

	// Assessments
	mux.Handle("POST /api/v1/organizations/{org_id}/assessments", orgMw(assessmentHandler.CreateAssessment))
	mux.Handle("GET /api/v1/organizations/{org_id}/assessments", orgMw(assessmentHandler.ListAssessments))
	mux.Handle("GET /api/v1/organizations/{org_id}/assessments/{assessment_id}", orgMw(assessmentHandler.GetAssessment))

	// Ledger
	mux.Handle("GET /api/v1/organizations/{org_id}/units/{unit_id}/ledger", orgMw(assessmentHandler.GetUnitLedger))
	mux.Handle("GET /api/v1/organizations/{org_id}/ledger", orgMw(assessmentHandler.GetOrgLedger))

	// Payments
	mux.Handle("POST /api/v1/organizations/{org_id}/payments", orgMw(paymentHandler.RecordPayment))
	mux.Handle("GET /api/v1/organizations/{org_id}/payments", orgMw(paymentHandler.ListPayments))
	mux.Handle("GET /api/v1/organizations/{org_id}/payments/{payment_id}", orgMw(paymentHandler.GetPayment))

	// Payment Methods
	mux.Handle("POST /api/v1/organizations/{org_id}/payment-methods", orgMw(paymentHandler.AddPaymentMethod))
	mux.Handle("GET /api/v1/organizations/{org_id}/payment-methods", orgMw(paymentHandler.ListPaymentMethods))
	mux.Handle("DELETE /api/v1/organizations/{org_id}/payment-methods/{method_id}", orgMw(paymentHandler.RemovePaymentMethod))

	// Budgets
	mux.Handle("POST /api/v1/organizations/{org_id}/budgets", orgMw(budgetHandler.CreateBudget))
	mux.Handle("GET /api/v1/organizations/{org_id}/budgets", orgMw(budgetHandler.ListBudgets))
	mux.Handle("GET /api/v1/organizations/{org_id}/budgets/{budget_id}", orgMw(budgetHandler.GetBudget))
	mux.Handle("PATCH /api/v1/organizations/{org_id}/budgets/{budget_id}", orgMw(budgetHandler.UpdateBudget))
	mux.Handle("POST /api/v1/organizations/{org_id}/budgets/{budget_id}/propose", orgMw(budgetHandler.ProposeBudget))
	mux.Handle("POST /api/v1/organizations/{org_id}/budgets/{budget_id}/approve", orgMw(budgetHandler.ApproveBudget))
	mux.Handle("POST /api/v1/organizations/{org_id}/budgets/{budget_id}/line-items", orgMw(budgetHandler.CreateLineItem))
	mux.Handle("PATCH /api/v1/organizations/{org_id}/budgets/{budget_id}/line-items/{item_id}", orgMw(budgetHandler.UpdateLineItem))
	mux.Handle("DELETE /api/v1/organizations/{org_id}/budgets/{budget_id}/line-items/{item_id}", orgMw(budgetHandler.DeleteLineItem))

	// Budget Categories
	mux.Handle("POST /api/v1/organizations/{org_id}/budget-categories", orgMw(budgetHandler.CreateCategory))
	mux.Handle("GET /api/v1/organizations/{org_id}/budget-categories", orgMw(budgetHandler.ListCategories))

	// Expenses
	mux.Handle("POST /api/v1/organizations/{org_id}/expenses", orgMw(budgetHandler.CreateExpense))
	mux.Handle("GET /api/v1/organizations/{org_id}/expenses", orgMw(budgetHandler.ListExpenses))
	mux.Handle("GET /api/v1/organizations/{org_id}/expenses/{expense_id}", orgMw(budgetHandler.GetExpense))
	mux.Handle("POST /api/v1/organizations/{org_id}/expenses/{expense_id}/approve", orgMw(budgetHandler.ApproveExpense))
	mux.Handle("POST /api/v1/organizations/{org_id}/expenses/{expense_id}/pay", orgMw(budgetHandler.PayExpense))

	// Funds
	mux.Handle("POST /api/v1/organizations/{org_id}/funds", orgMw(fundHandler.CreateFund))
	mux.Handle("GET /api/v1/organizations/{org_id}/funds", orgMw(fundHandler.ListFunds))
	mux.Handle("GET /api/v1/organizations/{org_id}/funds/{fund_id}", orgMw(fundHandler.GetFund))
	mux.Handle("GET /api/v1/organizations/{org_id}/funds/{fund_id}/transactions", orgMw(fundHandler.GetFundTransactions))
	mux.Handle("POST /api/v1/organizations/{org_id}/fund-transfers", orgMw(fundHandler.CreateFundTransfer))
	mux.Handle("GET /api/v1/organizations/{org_id}/fund-transfers", orgMw(fundHandler.ListFundTransfers))

	// Collections
	mux.Handle("GET /api/v1/organizations/{org_id}/collections", orgMw(collectionHandler.ListCollections))
	mux.Handle("GET /api/v1/organizations/{org_id}/collections/{case_id}", orgMw(collectionHandler.GetCollection))
	mux.Handle("PATCH /api/v1/organizations/{org_id}/collections/{case_id}", orgMw(collectionHandler.UpdateCollection))
	mux.Handle("POST /api/v1/organizations/{org_id}/collections/{case_id}/actions", orgMw(collectionHandler.AddCollectionAction))
	mux.Handle("POST /api/v1/organizations/{org_id}/collections/{case_id}/payment-plans", orgMw(collectionHandler.CreatePaymentPlan))
	mux.Handle("GET /api/v1/organizations/{org_id}/collections/{case_id}/payment-plans", orgMw(collectionHandler.ListPaymentPlans))
	mux.Handle("GET /api/v1/organizations/{org_id}/units/{unit_id}/collection-status", orgMw(collectionHandler.GetUnitCollectionStatus))
}
