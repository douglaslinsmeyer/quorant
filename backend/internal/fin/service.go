package fin

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/platform/api"
)

// UpdateAssessmentScheduleRequest holds the fields that can be updated on an
// existing assessment schedule.
type UpdateAssessmentScheduleRequest struct {
	Name            *string        `json:"name,omitempty"`
	Description     *string        `json:"description,omitempty"`
	Frequency       *string        `json:"frequency,omitempty"`
	AmountStrategy  *string        `json:"amount_strategy,omitempty"`
	BaseAmountCents *int64         `json:"base_amount_cents,omitempty"`
	AmountRules     map[string]any `json:"amount_rules,omitempty"`
	DayOfMonth      *int           `json:"day_of_month,omitempty"`
	GraceDays       *int           `json:"grace_days,omitempty"`
	EndsAt          *time.Time     `json:"ends_at,omitempty"`
}

// FinService orchestrates all financial operations for the Finance module.
// It is the business logic layer between HTTP handlers and repositories.
type FinService struct {
	assessments AssessmentRepository
	payments    PaymentRepository
	budgets     BudgetRepository
	funds       FundRepository
	collections CollectionRepository
	logger      *slog.Logger
}

// NewFinService creates a new FinService with the given repositories and logger.
func NewFinService(
	assessments AssessmentRepository,
	payments PaymentRepository,
	budgets BudgetRepository,
	funds FundRepository,
	collections CollectionRepository,
	logger *slog.Logger,
) *FinService {
	return &FinService{
		assessments: assessments,
		payments:    payments,
		budgets:     budgets,
		funds:       funds,
		collections: collections,
		logger:      logger,
	}
}

// ── Assessment Schedules ──────────────────────────────────────────────────────

// CreateSchedule validates the request and persists a new assessment schedule.
func (s *FinService) CreateSchedule(ctx context.Context, orgID uuid.UUID, req CreateAssessmentScheduleRequest) (*AssessmentSchedule, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	schedule := &AssessmentSchedule{
		OrgID:           orgID,
		Name:            req.Name,
		Description:     req.Description,
		Frequency:       req.Frequency,
		AmountStrategy:  req.AmountStrategy,
		BaseAmountCents: req.BaseAmountCents,
		AmountRules:     req.AmountRules,
		DayOfMonth:      req.DayOfMonth,
		GraceDays:       req.GraceDays,
		StartsAt:        req.StartsAt,
		EndsAt:          req.EndsAt,
		IsActive:        true,
	}
	return s.assessments.CreateSchedule(ctx, schedule)
}

// ListSchedules returns all non-deleted assessment schedules for the given org.
func (s *FinService) ListSchedules(ctx context.Context, orgID uuid.UUID) ([]AssessmentSchedule, error) {
	return s.assessments.ListSchedulesByOrg(ctx, orgID)
}

// GetSchedule returns the schedule with the given id, or a 404 error if not found.
func (s *FinService) GetSchedule(ctx context.Context, id uuid.UUID) (*AssessmentSchedule, error) {
	schedule, err := s.assessments.FindScheduleByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if schedule == nil {
		return nil, api.NewNotFoundError(fmt.Sprintf("assessment schedule %s not found", id))
	}
	return schedule, nil
}

// UpdateSchedule applies partial updates to an existing schedule and returns the
// updated row.
func (s *FinService) UpdateSchedule(ctx context.Context, id uuid.UUID, req UpdateAssessmentScheduleRequest) (*AssessmentSchedule, error) {
	existing, err := s.GetSchedule(ctx, id)
	if err != nil {
		return nil, err
	}
	if req.Name != nil {
		existing.Name = *req.Name
	}
	if req.Description != nil {
		existing.Description = req.Description
	}
	if req.Frequency != nil {
		existing.Frequency = *req.Frequency
	}
	if req.AmountStrategy != nil {
		existing.AmountStrategy = *req.AmountStrategy
	}
	if req.BaseAmountCents != nil {
		existing.BaseAmountCents = *req.BaseAmountCents
	}
	if req.AmountRules != nil {
		existing.AmountRules = req.AmountRules
	}
	if req.DayOfMonth != nil {
		existing.DayOfMonth = req.DayOfMonth
	}
	if req.GraceDays != nil {
		existing.GraceDays = req.GraceDays
	}
	if req.EndsAt != nil {
		existing.EndsAt = req.EndsAt
	}
	return s.assessments.UpdateSchedule(ctx, existing)
}

// DeactivateSchedule sets is_active = false for the given schedule.
func (s *FinService) DeactivateSchedule(ctx context.Context, id uuid.UUID) error {
	// Verify it exists first so we return a proper 404 if needed.
	if _, err := s.GetSchedule(ctx, id); err != nil {
		return err
	}
	return s.assessments.DeactivateSchedule(ctx, id)
}

// ── Assessments ───────────────────────────────────────────────────────────────

// CreateAssessment validates the request, creates the assessment record, and
// also creates a corresponding "charge" ledger entry against the unit.
func (s *FinService) CreateAssessment(ctx context.Context, orgID uuid.UUID, req CreateAssessmentRequest) (*Assessment, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	a := &Assessment{
		OrgID:       orgID,
		UnitID:      req.UnitID,
		Description: req.Description,
		AmountCents: req.AmountCents,
		DueDate:     req.DueDate,
		GraceDays:   req.GraceDays,
	}
	created, err := s.assessments.CreateAssessment(ctx, a)
	if err != nil {
		return nil, err
	}

	// Create a matching charge ledger entry.
	desc := created.Description
	entry := &LedgerEntry{
		OrgID:         orgID,
		UnitID:        req.UnitID,
		AssessmentID:  &created.ID,
		EntryType:     "charge",
		AmountCents:   created.AmountCents,
		Description:   &desc,
		EffectiveDate: created.DueDate,
	}
	if _, err := s.assessments.CreateLedgerEntry(ctx, entry); err != nil {
		s.logger.Error("failed to create ledger entry for assessment", "assessment_id", created.ID, "error", err)
		return nil, err
	}

	return created, nil
}

// ListAssessments returns all non-deleted assessments for the given org.
func (s *FinService) ListAssessments(ctx context.Context, orgID uuid.UUID) ([]Assessment, error) {
	return s.assessments.ListAssessmentsByOrg(ctx, orgID)
}

// GetAssessment returns the assessment with the given id, or a 404 if not found.
func (s *FinService) GetAssessment(ctx context.Context, id uuid.UUID) (*Assessment, error) {
	a, err := s.assessments.FindAssessmentByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if a == nil {
		return nil, api.NewNotFoundError(fmt.Sprintf("assessment %s not found", id))
	}
	return a, nil
}

// ── Ledger ────────────────────────────────────────────────────────────────────

// GetUnitLedger returns all ledger entries for the given unit.
func (s *FinService) GetUnitLedger(ctx context.Context, unitID uuid.UUID) ([]LedgerEntry, error) {
	return s.assessments.ListLedgerByUnit(ctx, unitID)
}

// GetOrgLedger returns all ledger entries for the given org.
func (s *FinService) GetOrgLedger(ctx context.Context, orgID uuid.UUID) ([]LedgerEntry, error) {
	return s.assessments.ListLedgerByOrg(ctx, orgID)
}

// ── Payments ──────────────────────────────────────────────────────────────────

// RecordPayment validates the request, records the payment, and creates a
// "payment" credit ledger entry for the unit.
func (s *FinService) RecordPayment(ctx context.Context, orgID uuid.UUID, userID uuid.UUID, req CreatePaymentRequest) (*Payment, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	now := time.Now()
	p := &Payment{
		OrgID:           orgID,
		UnitID:          req.UnitID,
		UserID:          userID,
		PaymentMethodID: req.PaymentMethodID,
		AmountCents:     req.AmountCents,
		Status:          "completed",
		Description:     req.Description,
		PaidAt:          &now,
	}
	created, err := s.payments.CreatePayment(ctx, p)
	if err != nil {
		return nil, err
	}

	// Create a credit ledger entry (negative amount reduces the balance).
	refType := "payment"
	entry := &LedgerEntry{
		OrgID:         orgID,
		UnitID:        req.UnitID,
		EntryType:     "payment",
		AmountCents:   -created.AmountCents,
		Description:   req.Description,
		ReferenceType: &refType,
		ReferenceID:   &created.ID,
		EffectiveDate: now,
	}
	if _, err := s.assessments.CreateLedgerEntry(ctx, entry); err != nil {
		s.logger.Error("failed to create ledger entry for payment", "payment_id", created.ID, "error", err)
		return nil, err
	}

	return created, nil
}

// ListPayments returns all payments for the given org.
func (s *FinService) ListPayments(ctx context.Context, orgID uuid.UUID) ([]Payment, error) {
	return s.payments.ListPaymentsByOrg(ctx, orgID)
}

// GetPayment returns the payment with the given id, or a 404 if not found.
func (s *FinService) GetPayment(ctx context.Context, id uuid.UUID) (*Payment, error) {
	p, err := s.payments.FindPaymentByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, api.NewNotFoundError(fmt.Sprintf("payment %s not found", id))
	}
	return p, nil
}

// AddPaymentMethod persists a new payment method for a user.
func (s *FinService) AddPaymentMethod(ctx context.Context, orgID uuid.UUID, m *PaymentMethod) (*PaymentMethod, error) {
	m.OrgID = orgID
	return s.payments.CreatePaymentMethod(ctx, m)
}

// ListPaymentMethods returns all non-deleted payment methods for the given user.
func (s *FinService) ListPaymentMethods(ctx context.Context, userID uuid.UUID) ([]PaymentMethod, error) {
	return s.payments.ListPaymentMethodsByUser(ctx, userID)
}

// RemovePaymentMethod soft-deletes a payment method.
func (s *FinService) RemovePaymentMethod(ctx context.Context, id uuid.UUID) error {
	return s.payments.SoftDeletePaymentMethod(ctx, id)
}

// ── Budgets ───────────────────────────────────────────────────────────────────

// CreateBudget validates the request and creates a new budget in "draft" status.
func (s *FinService) CreateBudget(ctx context.Context, orgID uuid.UUID, createdBy uuid.UUID, req CreateBudgetRequest) (*Budget, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	b := &Budget{
		OrgID:      orgID,
		FiscalYear: req.FiscalYear,
		Name:       req.Name,
		Status:     "draft",
		Notes:      req.Notes,
		CreatedBy:  createdBy,
	}
	return s.budgets.CreateBudget(ctx, b)
}

// GetBudget returns the budget with the given id, or a 404 if not found.
func (s *FinService) GetBudget(ctx context.Context, id uuid.UUID) (*Budget, error) {
	b, err := s.budgets.FindBudgetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if b == nil {
		return nil, api.NewNotFoundError(fmt.Sprintf("budget %s not found", id))
	}
	return b, nil
}

// ListBudgets returns all non-deleted budgets for the given org.
func (s *FinService) ListBudgets(ctx context.Context, orgID uuid.UUID) ([]Budget, error) {
	return s.budgets.ListBudgetsByOrg(ctx, orgID)
}

// ProposeBudget transitions a budget from "draft" to "proposed".
func (s *FinService) ProposeBudget(ctx context.Context, id uuid.UUID, proposedBy uuid.UUID) (*Budget, error) {
	b, err := s.GetBudget(ctx, id)
	if err != nil {
		return nil, err
	}
	if b.Status != "draft" {
		return nil, api.NewValidationError(fmt.Sprintf("budget must be in 'draft' status to propose, current status: %s", b.Status), "status")
	}
	now := time.Now()
	b.Status = "proposed"
	b.ProposedAt = &now
	b.ProposedBy = &proposedBy
	return s.budgets.UpdateBudget(ctx, b)
}

// ApproveBudget transitions a budget from "proposed" to "approved".
func (s *FinService) ApproveBudget(ctx context.Context, id uuid.UUID, approvedBy uuid.UUID) (*Budget, error) {
	b, err := s.GetBudget(ctx, id)
	if err != nil {
		return nil, err
	}
	if b.Status != "proposed" {
		return nil, api.NewValidationError(fmt.Sprintf("budget must be in 'proposed' status to approve, current status: %s", b.Status), "status")
	}
	now := time.Now()
	b.Status = "approved"
	b.ApprovedAt = &now
	b.ApprovedBy = &approvedBy
	return s.budgets.UpdateBudget(ctx, b)
}

// CreateLineItem creates a budget line item.
func (s *FinService) CreateLineItem(ctx context.Context, budgetID uuid.UUID, item *BudgetLineItem) (*BudgetLineItem, error) {
	item.BudgetID = budgetID
	return s.budgets.CreateLineItem(ctx, item)
}

// UpdateLineItem persists changes to an existing budget line item.
func (s *FinService) UpdateLineItem(ctx context.Context, id uuid.UUID, item *BudgetLineItem) (*BudgetLineItem, error) {
	item.ID = id
	return s.budgets.UpdateLineItem(ctx, item)
}

// DeleteLineItem hard-deletes a budget line item.
func (s *FinService) DeleteLineItem(ctx context.Context, id uuid.UUID) error {
	return s.budgets.DeleteLineItem(ctx, id)
}

// ListCategories returns all budget categories for the given org.
func (s *FinService) ListCategories(ctx context.Context, orgID uuid.UUID) ([]BudgetCategory, error) {
	return s.budgets.ListCategoriesByOrg(ctx, orgID)
}

// CreateCategory creates a new budget category for the given org.
func (s *FinService) CreateCategory(ctx context.Context, orgID uuid.UUID, c *BudgetCategory) (*BudgetCategory, error) {
	c.OrgID = orgID
	return s.budgets.CreateCategory(ctx, c)
}

// ── Expenses ──────────────────────────────────────────────────────────────────

// CreateExpense validates the request and creates a new expense in "pending" status.
func (s *FinService) CreateExpense(ctx context.Context, orgID uuid.UUID, submittedBy uuid.UUID, req CreateExpenseRequest) (*Expense, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	e := &Expense{
		OrgID:       orgID,
		Description: req.Description,
		AmountCents: req.AmountCents,
		TaxCents:    0,
		TotalCents:  req.AmountCents,
		Status:      "pending",
		ExpenseDate: req.ExpenseDate,
		DueDate:     req.DueDate,
		FundType:    req.FundType,
		VendorID:    req.VendorID,
		CategoryID:  req.CategoryID,
		BudgetID:    req.BudgetID,
		SubmittedBy: submittedBy,
		Metadata:    map[string]any{},
	}
	return s.budgets.CreateExpense(ctx, e)
}

// GetExpense returns the expense with the given id, or a 404 if not found.
func (s *FinService) GetExpense(ctx context.Context, id uuid.UUID) (*Expense, error) {
	e, err := s.budgets.FindExpenseByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if e == nil {
		return nil, api.NewNotFoundError(fmt.Sprintf("expense %s not found", id))
	}
	return e, nil
}

// ListExpenses returns all non-deleted expenses for the given org.
func (s *FinService) ListExpenses(ctx context.Context, orgID uuid.UUID) ([]Expense, error) {
	return s.budgets.ListExpensesByOrg(ctx, orgID)
}

// ApproveExpense transitions an expense to "approved" status.
func (s *FinService) ApproveExpense(ctx context.Context, id uuid.UUID, approvedBy uuid.UUID) (*Expense, error) {
	e, err := s.GetExpense(ctx, id)
	if err != nil {
		return nil, err
	}
	if e.Status != "pending" {
		return nil, api.NewValidationError(fmt.Sprintf("expense must be in 'pending' status to approve, current status: %s", e.Status), "status")
	}
	now := time.Now()
	e.Status = "approved"
	e.ApprovedBy = &approvedBy
	e.ApprovedAt = &now
	return s.budgets.UpdateExpense(ctx, e)
}

// PayExpense transitions an expense to "paid" status and sets the paid_date.
func (s *FinService) PayExpense(ctx context.Context, id uuid.UUID) (*Expense, error) {
	e, err := s.GetExpense(ctx, id)
	if err != nil {
		return nil, err
	}
	if e.Status != "approved" {
		return nil, api.NewValidationError(fmt.Sprintf("expense must be in 'approved' status to pay, current status: %s", e.Status), "status")
	}
	now := time.Now()
	e.Status = "paid"
	e.PaidDate = &now
	return s.budgets.UpdateExpense(ctx, e)
}

// ── Funds ─────────────────────────────────────────────────────────────────────

// CreateFund validates the request and creates a new fund.
func (s *FinService) CreateFund(ctx context.Context, orgID uuid.UUID, req CreateFundRequest) (*Fund, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	f := &Fund{
		OrgID:              orgID,
		Name:               req.Name,
		FundType:           req.FundType,
		BalanceCents:       0,
		TargetBalanceCents: req.TargetBalanceCents,
	}
	return s.funds.CreateFund(ctx, f)
}

// GetFund returns the fund with the given id, or a 404 if not found.
func (s *FinService) GetFund(ctx context.Context, id uuid.UUID) (*Fund, error) {
	f, err := s.funds.FindFundByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if f == nil {
		return nil, api.NewNotFoundError(fmt.Sprintf("fund %s not found", id))
	}
	return f, nil
}

// ListFunds returns all non-deleted funds for the given org.
func (s *FinService) ListFunds(ctx context.Context, orgID uuid.UUID) ([]Fund, error) {
	return s.funds.ListFundsByOrg(ctx, orgID)
}

// GetFundTransactions returns all transactions for the given fund.
func (s *FinService) GetFundTransactions(ctx context.Context, fundID uuid.UUID) ([]FundTransaction, error) {
	return s.funds.ListTransactionsByFund(ctx, fundID)
}

// CreateFundTransfer validates the request and creates a fund transfer record.
func (s *FinService) CreateFundTransfer(ctx context.Context, orgID uuid.UUID, req CreateFundTransferRequest) (*FundTransfer, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	now := time.Now()
	t := &FundTransfer{
		OrgID:         orgID,
		FromFundID:    req.FromFundID,
		ToFundID:      req.ToFundID,
		AmountCents:   req.AmountCents,
		Description:   req.Description,
		EffectiveDate: now,
	}
	return s.funds.CreateTransfer(ctx, t)
}

// ListFundTransfers returns all fund transfers for the given org.
func (s *FinService) ListFundTransfers(ctx context.Context, orgID uuid.UUID) ([]FundTransfer, error) {
	return s.funds.ListTransfersByOrg(ctx, orgID)
}

// ── Collections ───────────────────────────────────────────────────────────────

// ListCollections returns all collection cases for the given org.
func (s *FinService) ListCollections(ctx context.Context, orgID uuid.UUID) ([]CollectionCase, error) {
	return s.collections.ListCasesByOrg(ctx, orgID)
}

// GetCollection returns the collection case with the given id, or a 404 if not found.
func (s *FinService) GetCollection(ctx context.Context, id uuid.UUID) (*CollectionCase, error) {
	c, err := s.collections.FindCaseByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if c == nil {
		return nil, api.NewNotFoundError(fmt.Sprintf("collection case %s not found", id))
	}
	return c, nil
}

// UpdateCollection persists changes to an existing collection case.
func (s *FinService) UpdateCollection(ctx context.Context, id uuid.UUID, c *CollectionCase) (*CollectionCase, error) {
	c.ID = id
	return s.collections.UpdateCase(ctx, c)
}

// AddCollectionAction validates the request and adds a new action to a collection case.
func (s *FinService) AddCollectionAction(ctx context.Context, caseID uuid.UUID, req CreateCollectionActionRequest) (*CollectionAction, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	a := &CollectionAction{
		CaseID:       caseID,
		ActionType:   req.ActionType,
		Notes:        req.Notes,
		DocumentID:   req.DocumentID,
		ScheduledFor: req.ScheduledFor,
		Metadata:     map[string]any{},
	}
	return s.collections.CreateAction(ctx, a)
}

// CreatePaymentPlan validates the request and creates a new payment plan for a
// collection case.
func (s *FinService) CreatePaymentPlan(ctx context.Context, caseID uuid.UUID, orgID uuid.UUID, unitID uuid.UUID, req CreatePaymentPlanRequest) (*PaymentPlan, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	p := &PaymentPlan{
		CaseID:            caseID,
		OrgID:             orgID,
		UnitID:            unitID,
		TotalOwedCents:    req.TotalOwedCents,
		InstallmentCents:  req.InstallmentCents,
		InstallmentsTotal: req.InstallmentsTotal,
		InstallmentsPaid:  0,
		NextDueDate:       req.NextDueDate,
		Frequency:         req.Frequency,
		Status:            "active",
	}
	return s.collections.CreatePaymentPlan(ctx, p)
}

// ListPaymentPlans returns all payment plans for the given collection case.
func (s *FinService) ListPaymentPlans(ctx context.Context, caseID uuid.UUID) ([]PaymentPlan, error) {
	return s.collections.ListPaymentPlansByCase(ctx, caseID)
}

// GetUnitCollectionStatus returns the active collection case for the given unit,
// or a 404 error if no active case exists.
func (s *FinService) GetUnitCollectionStatus(ctx context.Context, unitID uuid.UUID) (*CollectionCase, error) {
	c, err := s.collections.GetCollectionStatusForUnit(ctx, unitID)
	if err != nil {
		return nil, err
	}
	if c == nil {
		return nil, api.NewNotFoundError(fmt.Sprintf("no active collection case for unit %s", unitID))
	}
	return c, nil
}
