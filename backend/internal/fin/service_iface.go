package fin

import (
	"context"

	"github.com/google/uuid"
)

// Service defines the business operations for the finance module.
// Handlers depend on this interface rather than the concrete FinService struct.
type Service interface {
	// Assessment Schedules
	CreateSchedule(ctx context.Context, orgID uuid.UUID, req CreateAssessmentScheduleRequest) (*AssessmentSchedule, error)
	ListSchedules(ctx context.Context, orgID uuid.UUID) ([]AssessmentSchedule, error)
	GetSchedule(ctx context.Context, id uuid.UUID) (*AssessmentSchedule, error)
	UpdateSchedule(ctx context.Context, id uuid.UUID, req UpdateAssessmentScheduleRequest) (*AssessmentSchedule, error)
	DeactivateSchedule(ctx context.Context, id uuid.UUID) error

	// Assessments
	CreateAssessment(ctx context.Context, orgID uuid.UUID, req CreateAssessmentRequest) (*Assessment, error)
	ListAssessments(ctx context.Context, orgID uuid.UUID, limit int, afterID *uuid.UUID) ([]Assessment, bool, error)
	GetAssessment(ctx context.Context, id uuid.UUID) (*Assessment, error)
	UpdateAssessment(ctx context.Context, id uuid.UUID, a *Assessment) (*Assessment, error)
	DeleteAssessment(ctx context.Context, id uuid.UUID) error
	VoidAssessment(ctx context.Context, id uuid.UUID, voidedBy uuid.UUID) error

	// Ledger
	GetUnitLedger(ctx context.Context, unitID uuid.UUID, limit int, afterID *uuid.UUID) ([]LedgerEntry, bool, error)
	GetOrgLedger(ctx context.Context, orgID uuid.UUID) ([]LedgerEntry, error)

	// Payments
	RecordPayment(ctx context.Context, orgID uuid.UUID, userID uuid.UUID, req CreatePaymentRequest) (*Payment, error)
	ListPayments(ctx context.Context, orgID uuid.UUID, limit int, afterID *uuid.UUID) ([]Payment, bool, error)
	GetPayment(ctx context.Context, id uuid.UUID) (*Payment, error)
	AddPaymentMethod(ctx context.Context, orgID uuid.UUID, m *PaymentMethod) (*PaymentMethod, error)
	ListPaymentMethods(ctx context.Context, userID uuid.UUID) ([]PaymentMethod, error)
	RemovePaymentMethod(ctx context.Context, id uuid.UUID) error

	// Budgets
	CreateBudget(ctx context.Context, orgID uuid.UUID, createdBy uuid.UUID, req CreateBudgetRequest) (*Budget, error)
	GetBudget(ctx context.Context, id uuid.UUID) (*Budget, error)
	ListBudgets(ctx context.Context, orgID uuid.UUID) ([]Budget, error)
	UpdateBudget(ctx context.Context, id uuid.UUID, req UpdateBudgetRequest) (*Budget, error)
	ProposeBudget(ctx context.Context, id uuid.UUID, proposedBy uuid.UUID) (*Budget, error)
	ApproveBudget(ctx context.Context, id uuid.UUID, approvedBy uuid.UUID) (*Budget, error)
	GetBudgetReport(ctx context.Context, budgetID uuid.UUID) (*BudgetReport, error)
	UpdateCategory(ctx context.Context, id uuid.UUID, c *BudgetCategory) (*BudgetCategory, error)
	UpdateExpense(ctx context.Context, id uuid.UUID, e *Expense) (*Expense, error)
	CreateLineItem(ctx context.Context, budgetID uuid.UUID, item *BudgetLineItem) (*BudgetLineItem, error)
	UpdateLineItem(ctx context.Context, id uuid.UUID, item *BudgetLineItem) (*BudgetLineItem, error)
	DeleteLineItem(ctx context.Context, id uuid.UUID) error
	ListCategories(ctx context.Context, orgID uuid.UUID) ([]BudgetCategory, error)
	CreateCategory(ctx context.Context, orgID uuid.UUID, c *BudgetCategory) (*BudgetCategory, error)
	CreateExpense(ctx context.Context, orgID uuid.UUID, submittedBy uuid.UUID, req CreateExpenseRequest) (*Expense, error)
	GetExpense(ctx context.Context, id uuid.UUID) (*Expense, error)
	ListExpenses(ctx context.Context, orgID uuid.UUID) ([]Expense, error)
	ApproveExpense(ctx context.Context, id uuid.UUID, approvedBy uuid.UUID) (*Expense, error)
	PayExpense(ctx context.Context, id uuid.UUID) (*Expense, error)

	// Funds
	CreateFund(ctx context.Context, orgID uuid.UUID, req CreateFundRequest) (*Fund, error)
	GetFund(ctx context.Context, id uuid.UUID) (*Fund, error)
	ListFunds(ctx context.Context, orgID uuid.UUID) ([]Fund, error)
	UpdateFund(ctx context.Context, id uuid.UUID, f *Fund) (*Fund, error)
	GetFundTransactions(ctx context.Context, fundID uuid.UUID) ([]FundTransaction, error)
	CreateFundTransfer(ctx context.Context, orgID uuid.UUID, req CreateFundTransferRequest) (*FundTransfer, error)
	ListFundTransfers(ctx context.Context, orgID uuid.UUID) ([]FundTransfer, error)

	// Collections
	ListCollections(ctx context.Context, orgID uuid.UUID) ([]CollectionCase, error)
	GetCollection(ctx context.Context, id uuid.UUID) (*CollectionCase, error)
	UpdateCollection(ctx context.Context, id uuid.UUID, c *CollectionCase) (*CollectionCase, error)
	AddCollectionAction(ctx context.Context, caseID uuid.UUID, req CreateCollectionActionRequest) (*CollectionAction, error)
	CreatePaymentPlan(ctx context.Context, caseID uuid.UUID, orgID uuid.UUID, unitID uuid.UUID, req CreatePaymentPlanRequest) (*PaymentPlan, error)
	UpdatePaymentPlan(ctx context.Context, id uuid.UUID, p *PaymentPlan) (*PaymentPlan, error)
	ListPaymentPlans(ctx context.Context, caseID uuid.UUID) ([]PaymentPlan, error)
	GetUnitCollectionStatus(ctx context.Context, unitID uuid.UUID) (*CollectionCase, error)
	CheckReconciliation(ctx context.Context, orgID uuid.UUID) (*ReconciliationResult, error)
}
