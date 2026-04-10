package fin

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/ai"
	"github.com/quorant/quorant/internal/platform/api"
	"github.com/quorant/quorant/internal/platform/db"
	"github.com/quorant/quorant/internal/platform/policy"
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
	gl          *GLService
	engine      AccountingEngine
	policy      ai.PolicyResolver
	compliance  ai.ComplianceResolver
	registry    *policy.Registry
	logger      *slog.Logger
	uowFactory  *db.UnitOfWorkFactory
}

// NewFinService creates a new FinService with the given repositories and logger.
func NewFinService(
	assessments AssessmentRepository,
	payments PaymentRepository,
	budgets BudgetRepository,
	funds FundRepository,
	collections CollectionRepository,
	gl *GLService,
	engine AccountingEngine,
	policy ai.PolicyResolver,
	compliance ai.ComplianceResolver,
	registry *policy.Registry,
	logger *slog.Logger,
	uowFactory *db.UnitOfWorkFactory,
) *FinService {
	return &FinService{
		assessments: assessments,
		payments:    payments,
		budgets:     budgets,
		funds:       funds,
		collections: collections,
		gl:          gl,
		engine:      engine,
		policy:      policy,
		compliance:  compliance,
		registry:    registry,
		logger:      logger,
		uowFactory:  uowFactory,
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
		CurrencyCode:    "USD",
		Name:            req.Name,
		Description:     req.Description,
		Frequency:       AssessmentFrequency(req.Frequency),
		AmountStrategy:  AmountStrategy(req.AmountStrategy),
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
		return nil, api.NewNotFoundError("resource.not_found", api.P("resource", "assessment_schedule"), api.P("id", id.String()))
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
		existing.Frequency = AssessmentFrequency(*req.Frequency)
	}
	if req.AmountStrategy != nil {
		existing.AmountStrategy = AmountStrategy(*req.AmountStrategy)
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
// When a UnitOfWorkFactory is configured, all writes (assessment, ledger entry,
// GL journal entry) execute within a single database transaction.
func (s *FinService) CreateAssessment(ctx context.Context, orgID uuid.UUID, req CreateAssessmentRequest) (*Assessment, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	a := &Assessment{
		OrgID:        orgID,
		CurrencyCode: "USD",
		UnitID:       req.UnitID,
		Description:  req.Description,
		AmountCents:  req.AmountCents,
		DueDate:      req.DueDate,
		GraceDays:    req.GraceDays,
		Status:       AssessmentStatusPosted,
	}

	// Optional: look up late fee policy to set late_fee_cents if not provided.
	if s.policy != nil && a.LateFeeCents == nil {
		result, err := s.policy.GetPolicy(ctx, orgID, "late_fee_schedule")
		if err == nil && result != nil {
			var cfg struct {
				LateFeeCents int64 `json:"late_fee_cents"`
			}
			if jsonErr := json.Unmarshal(result.Config, &cfg); jsonErr == nil && cfg.LateFeeCents > 0 {
				a.LateFeeCents = &cfg.LateFeeCents
			}
		}
	}

	// When UoW factory available, wrap all writes in a single transaction.
	// When nil (unit tests), operations run against the repos directly.
	var uow *db.UnitOfWork
	assessments := s.assessments
	gl := s.gl

	if s.uowFactory != nil {
		var err error
		uow, err = s.uowFactory.Begin(ctx)
		if err != nil {
			return nil, fmt.Errorf("fin: CreateAssessment begin tx: %w", err)
		}
		defer uow.Rollback(ctx) //nolint:errcheck
		assessments = s.assessments.WithTx(uow.Tx())
		if s.gl != nil {
			gl = s.gl.WithTx(uow.Tx())
		}
	}

	created, err := assessments.CreateAssessment(ctx, a)
	if err != nil {
		return nil, err
	}

	// Create a matching charge ledger entry.
	desc := created.Description
	entry := &LedgerEntry{
		OrgID:         orgID,
		CurrencyCode:  "USD",
		UnitID:        req.UnitID,
		AssessmentID:  &created.ID,
		EntryType:     LedgerEntryTypeCharge,
		AmountCents:   created.AmountCents,
		Description:   &desc,
		EffectiveDate: created.DueDate,
	}
	if _, err := assessments.CreateLedgerEntry(ctx, entry); err != nil {
		return nil, fmt.Errorf("fin: CreateAssessment ledger entry: %w", err)
	}

	// Post GL journal entry via accounting engine.
	if gl != nil && s.engine != nil {
		ftx := FinancialTransaction{
			Type:          TxTypeAssessment,
			OrgID:         orgID,
			AmountCents:   created.AmountCents,
			EffectiveDate: created.DueDate,
			SourceID:      created.ID,
			UnitID:        &req.UnitID,
			Memo:          fmt.Sprintf("Assessment: %s", created.Description),
		}
		lines, glErr := s.engine.JournalLines(ctx, gl, ftx)
		if glErr != nil {
			return nil, fmt.Errorf("fin: CreateAssessment GL entry: %w", glErr)
		}
		sourceType := GLSourceTypeAssessment
		if _, glErr := gl.PostSystemJournalEntry(ctx, orgID, uuid.Nil, created.DueDate, ftx.Memo, &sourceType, &created.ID, &req.UnitID, lines); glErr != nil {
			return nil, fmt.Errorf("fin: CreateAssessment GL entry: %w", glErr)
		}
	}

	if uow != nil {
		if err := uow.Commit(ctx); err != nil {
			return nil, fmt.Errorf("fin: CreateAssessment commit: %w", err)
		}
	}
	return created, nil
}

// ListAssessments returns non-deleted assessments for the given org, supporting pagination.
// limit controls the page size; afterID is the cursor from the previous page.
func (s *FinService) ListAssessments(ctx context.Context, orgID uuid.UUID, limit int, afterID *uuid.UUID) ([]Assessment, bool, error) {
	return s.assessments.ListAssessmentsByOrg(ctx, orgID, limit, afterID)
}

// GetAssessment returns the assessment with the given id, or a 404 if not found.
func (s *FinService) GetAssessment(ctx context.Context, id uuid.UUID) (*Assessment, error) {
	a, err := s.assessments.FindAssessmentByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if a == nil {
		return nil, api.NewNotFoundError("resource.not_found", api.P("resource", "assessment"), api.P("id", id.String()))
	}
	return a, nil
}

// UpdateAssessment persists changes to an existing assessment and returns the updated row.
func (s *FinService) UpdateAssessment(ctx context.Context, id uuid.UUID, a *Assessment) (*Assessment, error) {
	existing, err := s.GetAssessment(ctx, id)
	if err != nil {
		return nil, err
	}
	a.ID = existing.ID
	a.OrgID = existing.OrgID
	a.UnitID = existing.UnitID
	return s.assessments.UpdateAssessment(ctx, a)
}

// DeleteAssessment delegates to VoidAssessment with a nil voided-by user.
func (s *FinService) DeleteAssessment(ctx context.Context, id uuid.UUID) error {
	return s.VoidAssessment(ctx, id, uuid.Nil)
}

// VoidAssessment reverses the charge ledger entry for the assessment, reverses
// any associated GL journal entries, and marks the assessment as void.
func (s *FinService) VoidAssessment(ctx context.Context, id uuid.UUID, voidedBy uuid.UUID) error {
	assessment, err := s.GetAssessment(ctx, id)
	if err != nil {
		return err
	}

	if assessment.Status == AssessmentStatusVoid {
		return api.NewValidationError("fin.assessment.already_void", "status", api.P("assessment_id", id.String()))
	}

	// Check if any payment-type ledger entries are linked to this assessment.
	entries, err := s.assessments.FindLedgerEntriesByAssessment(ctx, id)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.EntryType == LedgerEntryTypePayment {
			return api.NewValidationError("fin.assessment.has_payments", "id", api.P("assessment_id", id.String()))
		}
	}

	// Find the unreversed charge entry and reverse it.
	for _, e := range entries {
		if e.EntryType == LedgerEntryTypeCharge && e.ReversedByEntryID == nil {
			if _, err := s.ReverseLedgerEntry(ctx, e.ID, voidedBy); err != nil {
				return err
			}
			break
		}
	}

	// Reverse associated GL journal entries.
	if s.gl != nil {
		glEntries, glErr := s.gl.FindJournalEntriesBySource(ctx, GLSourceTypeAssessment, id)
		if glErr == nil {
			for _, ge := range glEntries {
				if ge.ReversedBy == nil && !ge.IsReversal {
					if _, rErr := s.gl.ReverseJournalEntry(ctx, ge.ID, voidedBy); rErr != nil {
						s.logger.Error("GL: failed to reverse assessment journal entry", "journal_entry_id", ge.ID, "error", rErr)
					}
				}
			}
		}
	}

	// Mark the assessment as void.
	now := time.Now()
	return s.assessments.UpdateAssessmentStatus(ctx, id, AssessmentStatusVoid, &voidedBy, &now)
}

// ── Ledger ────────────────────────────────────────────────────────────────────

// GetUnitLedger returns ledger entries for the given unit, supporting pagination.
// limit controls the page size; afterID is the cursor from the previous page.
func (s *FinService) GetUnitLedger(ctx context.Context, unitID uuid.UUID, limit int, afterID *uuid.UUID) ([]LedgerEntry, bool, error) {
	return s.assessments.ListLedgerByUnit(ctx, unitID, limit, afterID)
}

// GetOrgLedger returns all ledger entries for the given org.
func (s *FinService) GetOrgLedger(ctx context.Context, orgID uuid.UUID) ([]LedgerEntry, error) {
	return s.assessments.ListLedgerByOrg(ctx, orgID)
}

// ReverseLedgerEntry creates a reversal entry that negates the original entry's
// amount and links the two entries together. The original entry is marked as
// reversed by the new entry.
func (s *FinService) ReverseLedgerEntry(ctx context.Context, entryID, reversedBy uuid.UUID) (*LedgerEntry, error) {
	original, err := s.assessments.FindLedgerEntryByID(ctx, entryID)
	if err != nil {
		return nil, err
	}
	if original == nil {
		return nil, api.NewNotFoundError("resource.not_found", api.P("resource", "ledger_entry"), api.P("id", entryID.String()))
	}

	if original.ReversedByEntryID != nil {
		return nil, fmt.Errorf("fin: ledger entry %s already reversed", entryID)
	}

	// Build description from original.
	desc := "Reversal"
	if original.Description != nil {
		desc = "Reversal: " + *original.Description
	}

	refType := LedgerRefTypeReversal
	reversal := &LedgerEntry{
		OrgID:         original.OrgID,
		CurrencyCode:  original.CurrencyCode,
		UnitID:        original.UnitID,
		AssessmentID:  original.AssessmentID,
		EntryType:     LedgerEntryTypeReversal,
		AmountCents:   -original.AmountCents,
		Description:   &desc,
		ReferenceType: &refType,
		ReferenceID:   &entryID,
		EffectiveDate: time.Now(),
	}

	created, err := s.assessments.CreateLedgerEntry(ctx, reversal)
	if err != nil {
		return nil, err
	}

	if err := s.assessments.UpdateLedgerEntryReversedBy(ctx, entryID, created.ID); err != nil {
		return nil, err
	}

	return created, nil
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
		CurrencyCode:    "USD",
		UnitID:          req.UnitID,
		UserID:          userID,
		PaymentMethodID: req.PaymentMethodID,
		AmountCents:     req.AmountCents,
		Status:          PaymentStatusCompleted,
		Description:     req.Description,
		PaidAt:          &now,
	}

	var uow *db.UnitOfWork
	payments := s.payments
	assessments := s.assessments
	gl := s.gl

	if s.uowFactory != nil {
		var err error
		uow, err = s.uowFactory.Begin(ctx)
		if err != nil {
			return nil, fmt.Errorf("fin: RecordPayment begin tx: %w", err)
		}
		defer uow.Rollback(ctx) //nolint:errcheck
		payments = s.payments.WithTx(uow.Tx())
		assessments = s.assessments.WithTx(uow.Tx())
		if s.gl != nil {
			gl = s.gl.WithTx(uow.Tx())
		}
	}

	created, err := payments.CreatePayment(ctx, p)
	if err != nil {
		return nil, err
	}

	// Create a credit ledger entry (negative amount reduces the balance).
	refType := LedgerRefTypePayment
	entry := &LedgerEntry{
		OrgID:         orgID,
		CurrencyCode:  "USD",
		UnitID:        req.UnitID,
		EntryType:     LedgerEntryTypePayment,
		AmountCents:   -created.AmountCents,
		Description:   req.Description,
		ReferenceType: &refType,
		ReferenceID:   &created.ID,
		EffectiveDate: now,
	}
	if _, err := assessments.CreateLedgerEntry(ctx, entry); err != nil {
		return nil, fmt.Errorf("fin: RecordPayment ledger entry: %w", err)
	}

	// Post GL journal entry via accounting engine.
	if gl != nil && s.engine != nil {
		memo := "Payment received"
		if req.Description != nil {
			memo = *req.Description
		}
		ftx := FinancialTransaction{
			Type:          TxTypePayment,
			OrgID:         orgID,
			AmountCents:   created.AmountCents,
			EffectiveDate: now,
			SourceID:      created.ID,
			UnitID:        &req.UnitID,
			Memo:          memo,
		}
		lines, glErr := s.engine.JournalLines(ctx, gl, ftx)
		if glErr != nil {
			return nil, fmt.Errorf("fin: RecordPayment GL entry: %w", glErr)
		}
		sourceType := GLSourceTypePayment
		if _, glErr := gl.PostSystemJournalEntry(ctx, orgID, userID, now, ftx.Memo, &sourceType, &created.ID, &req.UnitID, lines); glErr != nil {
			return nil, fmt.Errorf("fin: RecordPayment GL entry: %w", glErr)
		}
	}

	if uow != nil {
		if err := uow.Commit(ctx); err != nil {
			return nil, fmt.Errorf("fin: RecordPayment commit: %w", err)
		}
	}
	return created, nil
}

// ListPayments returns payments for the given org, supporting cursor-based pagination.
// limit controls the page size; afterID is the cursor from the previous page.
func (s *FinService) ListPayments(ctx context.Context, orgID uuid.UUID, limit int, afterID *uuid.UUID) ([]Payment, bool, error) {
	return s.payments.ListPaymentsByOrg(ctx, orgID, limit, afterID)
}

// GetPayment returns the payment with the given id, or a 404 if not found.
func (s *FinService) GetPayment(ctx context.Context, id uuid.UUID) (*Payment, error) {
	p, err := s.payments.FindPaymentByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, api.NewNotFoundError("resource.not_found", api.P("resource", "payment"), api.P("id", id.String()))
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
		Status:     BudgetStatusDraft,
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
		return nil, api.NewNotFoundError("resource.not_found", api.P("resource", "budget"), api.P("id", id.String()))
	}
	return b, nil
}

// ListBudgets returns all non-deleted budgets for the given org.
func (s *FinService) ListBudgets(ctx context.Context, orgID uuid.UUID) ([]Budget, error) {
	return s.budgets.ListBudgetsByOrg(ctx, orgID)
}

// UpdateBudget applies partial updates (name, notes) to an existing budget.
func (s *FinService) UpdateBudget(ctx context.Context, id uuid.UUID, req UpdateBudgetRequest) (*Budget, error) {
	b, err := s.GetBudget(ctx, id)
	if err != nil {
		return nil, err
	}
	if req.Name != nil {
		b.Name = *req.Name
	}
	if req.Notes != nil {
		b.Notes = req.Notes
	}
	return s.budgets.UpdateBudget(ctx, b)
}

// ProposeBudget transitions a budget from "draft" to "proposed".
func (s *FinService) ProposeBudget(ctx context.Context, id uuid.UUID, proposedBy uuid.UUID) (*Budget, error) {
	b, err := s.GetBudget(ctx, id)
	if err != nil {
		return nil, err
	}
	if b.Status != BudgetStatusDraft {
		return nil, api.NewValidationError("budget.invalid_status_transition", "status", api.P("expected", string(BudgetStatusDraft)), api.P("action", "propose"), api.P("current", string(b.Status)))
	}
	now := time.Now()
	b.Status = BudgetStatusProposed
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
	if b.Status != BudgetStatusProposed {
		return nil, api.NewValidationError("budget.invalid_status_transition", "status", api.P("expected", string(BudgetStatusProposed)), api.P("action", "approve"), api.P("current", string(b.Status)))
	}
	now := time.Now()
	b.Status = BudgetStatusApproved
	b.ApprovedAt = &now
	b.ApprovedBy = &approvedBy
	return s.budgets.UpdateBudget(ctx, b)
}

// GetBudgetReport returns a budget with its line items.
func (s *FinService) GetBudgetReport(ctx context.Context, budgetID uuid.UUID) (*BudgetReport, error) {
	b, err := s.GetBudget(ctx, budgetID)
	if err != nil {
		return nil, err
	}
	items, err := s.budgets.ListLineItemsByBudget(ctx, budgetID)
	if err != nil {
		return nil, err
	}
	return &BudgetReport{Budget: b, LineItems: items}, nil
}

// UpdateCategory persists changes to an existing budget category.
func (s *FinService) UpdateCategory(ctx context.Context, id uuid.UUID, c *BudgetCategory) (*BudgetCategory, error) {
	c.ID = id
	return s.budgets.UpdateCategory(ctx, c)
}

// UpdateExpense applies partial updates to an expense and returns the updated row.
func (s *FinService) UpdateExpense(ctx context.Context, id uuid.UUID, e *Expense) (*Expense, error) {
	existing, err := s.GetExpense(ctx, id)
	if err != nil {
		return nil, err
	}
	e.ID = existing.ID
	e.OrgID = existing.OrgID
	return s.budgets.UpdateExpense(ctx, e)
}

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
	if updated == nil {
		return nil, api.NewNotFoundError("resource.not_found", api.P("resource", "line_item"), api.P("id", id.String()))
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
		return api.NewNotFoundError("resource.not_found", api.P("resource", "line_item"), api.P("id", id.String()))
	}
	if err := s.budgets.DeleteLineItem(ctx, id); err != nil {
		return err
	}
	return s.budgets.RecalculateBudgetTotals(ctx, item.BudgetID)
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

// CreateExpense validates the request and creates a new expense in "submitted" status.
func (s *FinService) CreateExpense(ctx context.Context, orgID uuid.UUID, submittedBy uuid.UUID, req CreateExpenseRequest) (*Expense, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	var expenseFundType *FundType
	if req.FundType != nil {
		ft := FundType(*req.FundType)
		expenseFundType = &ft
	}
	e := &Expense{
		OrgID:        orgID,
		CurrencyCode: "USD",
		Description:  req.Description,
		AmountCents:  req.AmountCents,
		TaxCents:     0,
		TotalCents:   req.AmountCents,
		Status:       ExpenseStatusSubmitted,
		ExpenseDate:  req.ExpenseDate,
		DueDate:      req.DueDate,
		FundType:     expenseFundType,
		VendorID:     req.VendorID,
		CategoryID:   req.CategoryID,
		BudgetID:     req.BudgetID,
		SubmittedBy:  submittedBy,
		Metadata:     map[string]any{},
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
		return nil, api.NewNotFoundError("resource.not_found", api.P("resource", "expense"), api.P("id", id.String()))
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
	if e.Status != ExpenseStatusSubmitted {
		return nil, api.NewValidationError("budget.invalid_status_transition", "status", api.P("expected", string(ExpenseStatusSubmitted)), api.P("action", "approve"), api.P("current", string(e.Status)))
	}
	now := time.Now()
	e.Status = ExpenseStatusApproved
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
	if e.Status != ExpenseStatusApproved {
		return nil, api.NewValidationError("budget.invalid_status_transition", "status", api.P("expected", string(ExpenseStatusApproved)), api.P("action", "pay"), api.P("current", string(e.Status)))
	}
	now := time.Now()
	e.Status = ExpenseStatusPaid
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
		CurrencyCode:       "USD",
		Name:               req.Name,
		FundType:           FundType(req.FundType),
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
		return nil, api.NewNotFoundError("resource.not_found", api.P("resource", "fund"), api.P("id", id.String()))
	}
	return f, nil
}

// ListFunds returns all non-deleted funds for the given org.
func (s *FinService) ListFunds(ctx context.Context, orgID uuid.UUID) ([]Fund, error) {
	return s.funds.ListFundsByOrg(ctx, orgID)
}

// UpdateFund persists changes to an existing fund and returns the updated row.
func (s *FinService) UpdateFund(ctx context.Context, id uuid.UUID, f *Fund) (*Fund, error) {
	existing, err := s.GetFund(ctx, id)
	if err != nil {
		return nil, err
	}
	f.ID = existing.ID
	f.OrgID = existing.OrgID
	return s.funds.UpdateFund(ctx, f)
}

// GetFundTransactions returns all transactions for the given fund.
func (s *FinService) GetFundTransactions(ctx context.Context, fundID uuid.UUID) ([]FundTransaction, error) {
	return s.funds.ListTransactionsByFund(ctx, fundID)
}

// CreateFundTransfer validates the request, creates a fund transfer record,
// and atomically debits the source fund and credits the destination fund
// by creating two FundTransaction records.
func (s *FinService) CreateFundTransfer(ctx context.Context, orgID uuid.UUID, req CreateFundTransferRequest) (*FundTransfer, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	now := time.Now()
	t := &FundTransfer{
		OrgID:         orgID,
		CurrencyCode:  "USD",
		FromFundID:    req.FromFundID,
		ToFundID:      req.ToFundID,
		AmountCents:   req.AmountCents,
		Description:   req.Description,
		EffectiveDate: now,
	}

	var uow *db.UnitOfWork
	funds := s.funds
	gl := s.gl

	if s.uowFactory != nil {
		var err error
		uow, err = s.uowFactory.Begin(ctx)
		if err != nil {
			return nil, fmt.Errorf("fin: CreateFundTransfer begin tx: %w", err)
		}
		defer uow.Rollback(ctx) //nolint:errcheck
		funds = s.funds.WithTx(uow.Tx())
		if s.gl != nil {
			gl = s.gl.WithTx(uow.Tx())
		}
	}

	created, err := funds.CreateTransfer(ctx, t)
	if err != nil {
		return nil, err
	}

	// Debit the source fund.
	refType := FundTxRefTypeTransfer
	debitDesc := "Transfer out"
	if _, err := s.funds.CreateTransaction(ctx, &FundTransaction{
		FundID:          req.FromFundID,
		OrgID:           orgID,
		CurrencyCode:    "USD",
		TransactionType: FundTxTypeTransferOut,
		AmountCents:     -req.AmountCents,
		Description:     &debitDesc,
		ReferenceType:   &refType,
		ReferenceID:     &created.ID,
		EffectiveDate:   now,
	}); err != nil {
		return nil, fmt.Errorf("fin: CreateFundTransfer debit source fund: %w", err)
	}

	// Credit the destination fund.
	creditDesc := "Transfer in"
	if _, err := s.funds.CreateTransaction(ctx, &FundTransaction{
		FundID:          req.ToFundID,
		OrgID:           orgID,
		CurrencyCode:    "USD",
		TransactionType: FundTxTypeTransferIn,
		AmountCents:     req.AmountCents,
		Description:     &creditDesc,
		ReferenceType:   &refType,
		ReferenceID:     &created.ID,
		EffectiveDate:   now,
	}); err != nil {
		return nil, fmt.Errorf("fin: CreateFundTransfer credit destination fund: %w", err)
	}

	// Post GL journal entry via accounting engine.
	if gl != nil && s.engine != nil {
		fromFund, _ := funds.FindFundByID(ctx, req.FromFundID)
		toFund, _ := funds.FindFundByID(ctx, req.ToFundID)
		if fromFund != nil && toFund != nil {
			ftx := FinancialTransaction{
				Type:          TxTypeFundTransfer,
				OrgID:         orgID,
				AmountCents:   req.AmountCents,
				EffectiveDate: now,
				SourceID:      created.ID,
				Memo:          fmt.Sprintf("Transfer: %s to %s", fromFund.Name, toFund.Name),
				Metadata: map[string]any{
					"from_fund_type": string(fromFund.FundType),
					"to_fund_type":   string(toFund.FundType),
					"from_fund_name": fromFund.Name,
					"to_fund_name":   toFund.Name,
				},
			}
			lines, glErr := s.engine.JournalLines(ctx, gl, ftx)
			if glErr != nil {
				return nil, fmt.Errorf("fin: CreateFundTransfer GL entry: %w", glErr)
			}
			sourceType := GLSourceTypeTransfer
			if _, glErr := gl.PostSystemJournalEntry(ctx, orgID, uuid.Nil, now, ftx.Memo, &sourceType, &created.ID, nil, lines); glErr != nil {
				return nil, fmt.Errorf("fin: CreateFundTransfer GL entry: %w", glErr)
			}
		}
	}

	if uow != nil {
		if err := uow.Commit(ctx); err != nil {
			return nil, fmt.Errorf("fin: CreateFundTransfer commit: %w", err)
		}
	}
	return created, nil
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
		return nil, api.NewNotFoundError("resource.not_found", api.P("resource", "collection_case"), api.P("id", id.String()))
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
		ActionType:   CollectionActionType(req.ActionType),
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
		Frequency:         PaymentPlanFrequency(req.Frequency),
		Status:            PaymentPlanStatusActive,
	}
	return s.collections.CreatePaymentPlan(ctx, p)
}

// UpdatePaymentPlan persists changes to an existing payment plan.
func (s *FinService) UpdatePaymentPlan(ctx context.Context, id uuid.UUID, p *PaymentPlan) (*PaymentPlan, error) {
	p.ID = id
	return s.collections.UpdatePaymentPlan(ctx, p)
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
		return nil, api.NewNotFoundError("resource.not_found", api.P("resource", "collection_case"), api.P("id", unitID.String()))
	}
	return c, nil
}

// ReconciliationResult holds the outcome of a ledger reconciliation check.
type ReconciliationResult struct {
	OrgID              uuid.UUID `json:"org_id"`
	UnitLedgerTotal    int64     `json:"unit_ledger_total_cents"`
	FundTransTotal     int64     `json:"fund_transaction_total_cents"`
	Discrepancy        int64     `json:"discrepancy_cents"`
	IsReconciled       bool      `json:"is_reconciled"`
}

// CheckReconciliation compares unit-level ledger totals with org-level fund transaction totals.
// Returns the discrepancy if any. This is a read-only diagnostic — it does not modify data.
func (s *FinService) CheckReconciliation(ctx context.Context, orgID uuid.UUID) (*ReconciliationResult, error) {
	// Get total from unit ledger (sum of all charges and payments)
	ledger, err := s.assessments.ListLedgerByOrg(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("listing org ledger: %w", err)
	}

	var unitTotal int64
	for _, entry := range ledger {
		unitTotal += entry.AmountCents
	}

	// Get total from fund transactions
	funds, err := s.funds.ListFundsByOrg(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("listing funds: %w", err)
	}

	var fundTotal int64
	for _, fund := range funds {
		fundTotal += fund.BalanceCents
	}

	return &ReconciliationResult{
		OrgID:           orgID,
		UnitLedgerTotal: unitTotal,
		FundTransTotal:  fundTotal,
		Discrepancy:     unitTotal - fundTotal,
		IsReconciled:    unitTotal == fundTotal,
	}, nil
}
