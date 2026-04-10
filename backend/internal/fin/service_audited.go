package fin

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/quorant/quorant/internal/audit"
	"github.com/quorant/quorant/internal/platform/middleware"
	"github.com/quorant/quorant/internal/platform/queue"
)

// AuditedFinService wraps a fin.Service, adding audit logging and domain event
// publishing after every successful mutation. Read-only methods pass through
// with no overhead.
type AuditedFinService struct {
	inner   Service
	auditor audit.Auditor
	pub     queue.Publisher
	logger  *slog.Logger
}

// compile-time interface assertion
var _ Service = (*AuditedFinService)(nil)

// NewAuditedFinService creates an AuditedFinService decorator.
func NewAuditedFinService(inner Service, auditor audit.Auditor, pub queue.Publisher, logger *slog.Logger) *AuditedFinService {
	return &AuditedFinService{
		inner:   inner,
		auditor: auditor,
		pub:     pub,
		logger:  logger,
	}
}

// ── Private helpers ──────────────────────────────────────────────────────────

// record creates and records an audit entry. Errors are logged but not returned.
func (s *AuditedFinService) record(ctx context.Context, action, resourceType string, resourceID, orgID uuid.UUID, before, after any) {
	var beforeJSON, afterJSON json.RawMessage
	if before != nil {
		if data, err := json.Marshal(before); err == nil {
			beforeJSON = data
		}
	}
	if after != nil {
		if data, err := json.Marshal(after); err == nil {
			afterJSON = data
		}
	}

	entry := audit.AuditEntry{
		OrgID:        orgID,
		ActorID:      middleware.UserIDFromContext(ctx),
		Action:       "fin." + action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Module:       "fin",
		BeforeState:  beforeJSON,
		AfterState:   afterJSON,
		OccurredAt:   time.Now().UTC(),
	}

	if err := s.auditor.Record(ctx, entry); err != nil {
		s.logger.Error("audit record failed", "action", action, "resource_id", resourceID, "error", err)
	}
}

// publish creates and publishes a domain event. Errors are logged but not returned.
func (s *AuditedFinService) publish(ctx context.Context, eventType, aggregateType string, aggregateID, orgID uuid.UUID, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		s.logger.ErrorContext(ctx, "event: failed to marshal payload", "error", err, "event_type", eventType)
		data = nil
	}

	evt := queue.NewBaseEvent(eventType, aggregateType, aggregateID, orgID, data)
	if err := s.pub.Publish(ctx, evt); err != nil {
		s.logger.ErrorContext(ctx, "event: failed to publish", "error", err, "event_type", eventType)
	}
}

// emit calls both record and publish.
// For create/update operations, pass nil for before and the result for after.
// For delete/deactivate operations, pass the fetched entity for before and nil for after.
func (s *AuditedFinService) emit(ctx context.Context, eventType, resourceType string, resourceID, orgID uuid.UUID, before, after any) {
	s.record(ctx, eventType, resourceType, resourceID, orgID, before, after)
	// publish uses whichever is non-nil as the event payload (after for creates/updates, before for deletes)
	payload := after
	if payload == nil {
		payload = before
	}
	s.publish(ctx, eventType, resourceType, resourceID, orgID, payload)
}

// ── Assessment Schedules (mutations) ─────────────────────────────────────────

func (s *AuditedFinService) CreateSchedule(ctx context.Context, orgID uuid.UUID, req CreateAssessmentScheduleRequest) (*AssessmentSchedule, error) {
	result, err := s.inner.CreateSchedule(ctx, orgID, req)
	if err != nil {
		return nil, err
	}
	s.emit(ctx, "assessment_schedule.created", "assessment_schedule", result.ID, result.OrgID, nil, result)
	return result, nil
}

func (s *AuditedFinService) UpdateSchedule(ctx context.Context, id uuid.UUID, req UpdateAssessmentScheduleRequest) (*AssessmentSchedule, error) {
	result, err := s.inner.UpdateSchedule(ctx, id, req)
	if err != nil {
		return nil, err
	}
	s.emit(ctx, "assessment_schedule.updated", "assessment_schedule", result.ID, result.OrgID, nil, result)
	return result, nil
}

func (s *AuditedFinService) DeactivateSchedule(ctx context.Context, id uuid.UUID) error {
	// Fetch before-state for audit payload.
	schedule, _ := s.inner.GetSchedule(ctx, id)

	if err := s.inner.DeactivateSchedule(ctx, id); err != nil {
		return err
	}

	orgID := middleware.OrgIDFromContext(ctx)
	var before any = map[string]any{"id": id}
	if schedule != nil {
		orgID = schedule.OrgID
		before = schedule
	}
	s.emit(ctx, "assessment_schedule.deactivated", "assessment_schedule", id, orgID, before, nil)
	return nil
}

// ── Assessments (mutations) ──────────────────────────────────────────────────

func (s *AuditedFinService) CreateAssessment(ctx context.Context, orgID uuid.UUID, req CreateAssessmentRequest) (*Assessment, error) {
	result, err := s.inner.CreateAssessment(ctx, orgID, req)
	if err != nil {
		return nil, err
	}
	s.emit(ctx, "assessment.created", "assessment", result.ID, result.OrgID, nil, result)
	return result, nil
}

func (s *AuditedFinService) UpdateAssessment(ctx context.Context, id uuid.UUID, a *Assessment) (*Assessment, error) {
	result, err := s.inner.UpdateAssessment(ctx, id, a)
	if err != nil {
		return nil, err
	}
	s.emit(ctx, "assessment.updated", "assessment", result.ID, result.OrgID, nil, result)
	return result, nil
}

func (s *AuditedFinService) DeleteAssessment(ctx context.Context, id uuid.UUID) error {
	// Fetch before-state for audit payload.
	assessment, _ := s.inner.GetAssessment(ctx, id)

	if err := s.inner.DeleteAssessment(ctx, id); err != nil {
		return err
	}

	orgID := middleware.OrgIDFromContext(ctx)
	var before any = map[string]any{"id": id}
	if assessment != nil {
		orgID = assessment.OrgID
		before = assessment
	}
	s.emit(ctx, "assessment.deleted", "assessment", id, orgID, before, nil)
	return nil
}

func (s *AuditedFinService) VoidAssessment(ctx context.Context, id uuid.UUID, voidedBy uuid.UUID) error {
	// Fetch before-state for audit payload.
	assessment, _ := s.inner.GetAssessment(ctx, id)

	if err := s.inner.VoidAssessment(ctx, id, voidedBy); err != nil {
		return err
	}

	orgID := middleware.OrgIDFromContext(ctx)
	var before any = map[string]any{"id": id}
	if assessment != nil {
		orgID = assessment.OrgID
		before = assessment
	}
	s.emit(ctx, "assessment.voided", "assessment", id, orgID, before, nil)
	return nil
}

// ── Payments (mutations) ─────────────────────────────────────────────────────

func (s *AuditedFinService) RecordPayment(ctx context.Context, orgID uuid.UUID, userID uuid.UUID, req CreatePaymentRequest) (*Payment, error) {
	result, err := s.inner.RecordPayment(ctx, orgID, userID, req)
	if err != nil {
		return nil, err
	}
	s.emit(ctx, "payment.received", "payment", result.ID, result.OrgID, nil, result)
	return result, nil
}

func (s *AuditedFinService) AddPaymentMethod(ctx context.Context, orgID uuid.UUID, m *PaymentMethod) (*PaymentMethod, error) {
	result, err := s.inner.AddPaymentMethod(ctx, orgID, m)
	if err != nil {
		return nil, err
	}
	s.emit(ctx, "payment_method.added", "payment_method", result.ID, result.OrgID, nil, result)
	return result, nil
}

func (s *AuditedFinService) RemovePaymentMethod(ctx context.Context, id uuid.UUID) error {
	// Fetch before-state for audit payload.
	// PaymentMethod is fetched indirectly since there is no GetPaymentMethod.
	// Fall back to context org ID.
	if err := s.inner.RemovePaymentMethod(ctx, id); err != nil {
		return err
	}

	orgID := middleware.OrgIDFromContext(ctx)
	before := map[string]any{"id": id}
	s.emit(ctx, "payment_method.removed", "payment_method", id, orgID, before, nil)
	return nil
}

func (s *AuditedFinService) VoidPayment(ctx context.Context, id uuid.UUID, voidedBy uuid.UUID) error {
	// Fetch before-state for audit payload.
	payment, _ := s.inner.GetPayment(ctx, id)

	if err := s.inner.VoidPayment(ctx, id, voidedBy); err != nil {
		return err
	}

	orgID := middleware.OrgIDFromContext(ctx)
	var before any = map[string]any{"id": id}
	if payment != nil {
		orgID = payment.OrgID
		before = payment
	}
	s.emit(ctx, "payment.voided", "payment", id, orgID, before, nil)
	return nil
}

// ── Budgets (mutations) ──────────────────────────────────────────────────────

func (s *AuditedFinService) CreateBudget(ctx context.Context, orgID uuid.UUID, createdBy uuid.UUID, req CreateBudgetRequest) (*Budget, error) {
	result, err := s.inner.CreateBudget(ctx, orgID, createdBy, req)
	if err != nil {
		return nil, err
	}
	s.emit(ctx, "budget.created", "budget", result.ID, result.OrgID, nil, result)
	return result, nil
}

func (s *AuditedFinService) UpdateBudget(ctx context.Context, id uuid.UUID, req UpdateBudgetRequest) (*Budget, error) {
	result, err := s.inner.UpdateBudget(ctx, id, req)
	if err != nil {
		return nil, err
	}
	s.emit(ctx, "budget.updated", "budget", result.ID, result.OrgID, nil, result)
	return result, nil
}

func (s *AuditedFinService) ProposeBudget(ctx context.Context, id uuid.UUID, proposedBy uuid.UUID) (*Budget, error) {
	result, err := s.inner.ProposeBudget(ctx, id, proposedBy)
	if err != nil {
		return nil, err
	}
	s.emit(ctx, "budget.proposed", "budget", result.ID, result.OrgID, nil, result)
	return result, nil
}

func (s *AuditedFinService) ApproveBudget(ctx context.Context, id uuid.UUID, approvedBy uuid.UUID) (*Budget, error) {
	result, err := s.inner.ApproveBudget(ctx, id, approvedBy)
	if err != nil {
		return nil, err
	}
	s.emit(ctx, "budget.approved", "budget", result.ID, result.OrgID, nil, result)
	return result, nil
}

func (s *AuditedFinService) UpdateCategory(ctx context.Context, id uuid.UUID, c *BudgetCategory) (*BudgetCategory, error) {
	result, err := s.inner.UpdateCategory(ctx, id, c)
	if err != nil {
		return nil, err
	}
	s.emit(ctx, "budget_category.updated", "budget_category", result.ID, result.OrgID, nil, result)
	return result, nil
}

func (s *AuditedFinService) CreateCategory(ctx context.Context, orgID uuid.UUID, c *BudgetCategory) (*BudgetCategory, error) {
	result, err := s.inner.CreateCategory(ctx, orgID, c)
	if err != nil {
		return nil, err
	}
	s.emit(ctx, "budget_category.created", "budget_category", result.ID, result.OrgID, nil, result)
	return result, nil
}

func (s *AuditedFinService) UpdateExpense(ctx context.Context, id uuid.UUID, e *Expense) (*Expense, error) {
	result, err := s.inner.UpdateExpense(ctx, id, e)
	if err != nil {
		return nil, err
	}
	s.emit(ctx, "expense.updated", "expense", result.ID, result.OrgID, nil, result)
	return result, nil
}

func (s *AuditedFinService) CreateLineItem(ctx context.Context, budgetID uuid.UUID, item *BudgetLineItem) (*BudgetLineItem, error) {
	result, err := s.inner.CreateLineItem(ctx, budgetID, item)
	if err != nil {
		return nil, err
	}
	orgID := middleware.OrgIDFromContext(ctx)
	s.emit(ctx, "budget_line_item.created", "budget_line_item", result.ID, orgID, nil, result)
	return result, nil
}

func (s *AuditedFinService) UpdateLineItem(ctx context.Context, id uuid.UUID, item *BudgetLineItem) (*BudgetLineItem, error) {
	result, err := s.inner.UpdateLineItem(ctx, id, item)
	if err != nil {
		return nil, err
	}
	orgID := middleware.OrgIDFromContext(ctx)
	s.emit(ctx, "budget_line_item.updated", "budget_line_item", result.ID, orgID, nil, result)
	return result, nil
}

func (s *AuditedFinService) DeleteLineItem(ctx context.Context, id uuid.UUID) error {
	if err := s.inner.DeleteLineItem(ctx, id); err != nil {
		return err
	}
	orgID := middleware.OrgIDFromContext(ctx)
	before := map[string]any{"id": id}
	s.emit(ctx, "budget_line_item.deleted", "budget_line_item", id, orgID, before, nil)
	return nil
}

// ── Expenses (mutations) ─────────────────────────────────────────────────────

func (s *AuditedFinService) CreateExpense(ctx context.Context, orgID uuid.UUID, submittedBy uuid.UUID, req CreateExpenseRequest) (*Expense, error) {
	result, err := s.inner.CreateExpense(ctx, orgID, submittedBy, req)
	if err != nil {
		return nil, err
	}
	s.emit(ctx, "expense.created", "expense", result.ID, result.OrgID, nil, result)
	return result, nil
}

func (s *AuditedFinService) ApproveExpense(ctx context.Context, id uuid.UUID, approvedBy uuid.UUID) (*Expense, error) {
	result, err := s.inner.ApproveExpense(ctx, id, approvedBy)
	if err != nil {
		return nil, err
	}
	s.emit(ctx, "expense.approved", "expense", result.ID, result.OrgID, nil, result)
	return result, nil
}

func (s *AuditedFinService) PayExpense(ctx context.Context, id uuid.UUID) (*Expense, error) {
	result, err := s.inner.PayExpense(ctx, id)
	if err != nil {
		return nil, err
	}
	s.emit(ctx, "expense.paid", "expense", result.ID, result.OrgID, nil, result)
	return result, nil
}

// ── Funds (mutations) ────────────────────────────────────────────────────────

func (s *AuditedFinService) CreateFund(ctx context.Context, orgID uuid.UUID, req CreateFundRequest) (*Fund, error) {
	result, err := s.inner.CreateFund(ctx, orgID, req)
	if err != nil {
		return nil, err
	}
	s.emit(ctx, "fund.created", "fund", result.ID, result.OrgID, nil, result)
	return result, nil
}

func (s *AuditedFinService) UpdateFund(ctx context.Context, id uuid.UUID, f *Fund) (*Fund, error) {
	result, err := s.inner.UpdateFund(ctx, id, f)
	if err != nil {
		return nil, err
	}
	s.emit(ctx, "fund.updated", "fund", result.ID, result.OrgID, nil, result)
	return result, nil
}

func (s *AuditedFinService) CreateFundTransfer(ctx context.Context, orgID uuid.UUID, req CreateFundTransferRequest) (*FundTransfer, error) {
	result, err := s.inner.CreateFundTransfer(ctx, orgID, req)
	if err != nil {
		return nil, err
	}
	s.emit(ctx, "fund_transfer.created", "fund_transfer", result.ID, result.OrgID, nil, result)
	return result, nil
}

// ── Collections (mutations) ──────────────────────────────────────────────────

func (s *AuditedFinService) UpdateCollection(ctx context.Context, id uuid.UUID, c *CollectionCase) (*CollectionCase, error) {
	result, err := s.inner.UpdateCollection(ctx, id, c)
	if err != nil {
		return nil, err
	}
	s.emit(ctx, "collection.updated", "collection", result.ID, result.OrgID, nil, result)
	return result, nil
}

func (s *AuditedFinService) AddCollectionAction(ctx context.Context, caseID uuid.UUID, req CreateCollectionActionRequest) (*CollectionAction, error) {
	result, err := s.inner.AddCollectionAction(ctx, caseID, req)
	if err != nil {
		return nil, err
	}
	// CollectionAction has no OrgID; use context.
	orgID := middleware.OrgIDFromContext(ctx)
	s.emit(ctx, "collection_action.added", "collection_action", result.ID, orgID, nil, result)
	return result, nil
}

func (s *AuditedFinService) CreatePaymentPlan(ctx context.Context, caseID uuid.UUID, orgID uuid.UUID, unitID uuid.UUID, req CreatePaymentPlanRequest) (*PaymentPlan, error) {
	result, err := s.inner.CreatePaymentPlan(ctx, caseID, orgID, unitID, req)
	if err != nil {
		return nil, err
	}
	s.emit(ctx, "payment_plan.created", "payment_plan", result.ID, result.OrgID, nil, result)
	return result, nil
}

func (s *AuditedFinService) UpdatePaymentPlan(ctx context.Context, id uuid.UUID, p *PaymentPlan) (*PaymentPlan, error) {
	result, err := s.inner.UpdatePaymentPlan(ctx, id, p)
	if err != nil {
		return nil, err
	}
	s.emit(ctx, "payment_plan.updated", "payment_plan", result.ID, result.OrgID, nil, result)
	return result, nil
}

// ── Read-only pass-through methods ───────────────────────────────────────────

func (s *AuditedFinService) ListSchedules(ctx context.Context, orgID uuid.UUID) ([]AssessmentSchedule, error) {
	return s.inner.ListSchedules(ctx, orgID)
}

func (s *AuditedFinService) GetSchedule(ctx context.Context, id uuid.UUID) (*AssessmentSchedule, error) {
	return s.inner.GetSchedule(ctx, id)
}

func (s *AuditedFinService) ListAssessments(ctx context.Context, orgID uuid.UUID, limit int, afterID *uuid.UUID) ([]Assessment, bool, error) {
	return s.inner.ListAssessments(ctx, orgID, limit, afterID)
}

func (s *AuditedFinService) GetAssessment(ctx context.Context, id uuid.UUID) (*Assessment, error) {
	return s.inner.GetAssessment(ctx, id)
}

func (s *AuditedFinService) GetUnitLedger(ctx context.Context, unitID uuid.UUID, limit int, afterID *uuid.UUID) ([]LedgerEntry, bool, error) {
	return s.inner.GetUnitLedger(ctx, unitID, limit, afterID)
}

func (s *AuditedFinService) GetOrgLedger(ctx context.Context, orgID uuid.UUID) ([]LedgerEntry, error) {
	return s.inner.GetOrgLedger(ctx, orgID)
}

func (s *AuditedFinService) ListPayments(ctx context.Context, orgID uuid.UUID, limit int, afterID *uuid.UUID) ([]Payment, bool, error) {
	return s.inner.ListPayments(ctx, orgID, limit, afterID)
}

func (s *AuditedFinService) GetPayment(ctx context.Context, id uuid.UUID) (*Payment, error) {
	return s.inner.GetPayment(ctx, id)
}

func (s *AuditedFinService) ListPaymentMethods(ctx context.Context, userID uuid.UUID) ([]PaymentMethod, error) {
	return s.inner.ListPaymentMethods(ctx, userID)
}

func (s *AuditedFinService) GetBudget(ctx context.Context, id uuid.UUID) (*Budget, error) {
	return s.inner.GetBudget(ctx, id)
}

func (s *AuditedFinService) ListBudgets(ctx context.Context, orgID uuid.UUID) ([]Budget, error) {
	return s.inner.ListBudgets(ctx, orgID)
}

func (s *AuditedFinService) GetBudgetReport(ctx context.Context, budgetID uuid.UUID) (*BudgetReport, error) {
	return s.inner.GetBudgetReport(ctx, budgetID)
}

func (s *AuditedFinService) ListCategories(ctx context.Context, orgID uuid.UUID) ([]BudgetCategory, error) {
	return s.inner.ListCategories(ctx, orgID)
}

func (s *AuditedFinService) GetExpense(ctx context.Context, id uuid.UUID) (*Expense, error) {
	return s.inner.GetExpense(ctx, id)
}

func (s *AuditedFinService) ListExpenses(ctx context.Context, orgID uuid.UUID) ([]Expense, error) {
	return s.inner.ListExpenses(ctx, orgID)
}

func (s *AuditedFinService) GetFund(ctx context.Context, id uuid.UUID) (*Fund, error) {
	return s.inner.GetFund(ctx, id)
}

func (s *AuditedFinService) ListFunds(ctx context.Context, orgID uuid.UUID) ([]Fund, error) {
	return s.inner.ListFunds(ctx, orgID)
}

func (s *AuditedFinService) GetFundTransactions(ctx context.Context, fundID uuid.UUID) ([]FundTransaction, error) {
	return s.inner.GetFundTransactions(ctx, fundID)
}

func (s *AuditedFinService) ListFundTransfers(ctx context.Context, orgID uuid.UUID) ([]FundTransfer, error) {
	return s.inner.ListFundTransfers(ctx, orgID)
}

func (s *AuditedFinService) ListCollections(ctx context.Context, orgID uuid.UUID) ([]CollectionCase, error) {
	return s.inner.ListCollections(ctx, orgID)
}

func (s *AuditedFinService) GetCollection(ctx context.Context, id uuid.UUID) (*CollectionCase, error) {
	return s.inner.GetCollection(ctx, id)
}

func (s *AuditedFinService) ListPaymentPlans(ctx context.Context, caseID uuid.UUID) ([]PaymentPlan, error) {
	return s.inner.ListPaymentPlans(ctx, caseID)
}

func (s *AuditedFinService) GetUnitCollectionStatus(ctx context.Context, unitID uuid.UUID) (*CollectionCase, error) {
	return s.inner.GetUnitCollectionStatus(ctx, unitID)
}

func (s *AuditedFinService) CheckReconciliation(ctx context.Context, orgID uuid.UUID) (*ReconciliationResult, error) {
	return s.inner.CheckReconciliation(ctx, orgID)
}
