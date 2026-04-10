package fin_test

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/quorant/quorant/internal/ai"
	"github.com/quorant/quorant/internal/audit"
	"github.com/quorant/quorant/internal/fin"
	"github.com/quorant/quorant/internal/platform/api"
	"github.com/quorant/quorant/internal/platform/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── Mock repositories ─────────────────────────────────────────────────────────

// mockAssessmentRepo is an in-memory implementation of AssessmentRepository.
type mockAssessmentRepo struct {
	schedules []fin.AssessmentSchedule
	assessments []fin.Assessment
	ledger      []fin.LedgerEntry
}

func (m *mockAssessmentRepo) CreateSchedule(_ context.Context, s *fin.AssessmentSchedule) (*fin.AssessmentSchedule, error) {
	s.ID = uuid.New()
	s.CreatedAt = time.Now()
	s.UpdatedAt = time.Now()
	m.schedules = append(m.schedules, *s)
	out := m.schedules[len(m.schedules)-1]
	return &out, nil
}

func (m *mockAssessmentRepo) FindScheduleByID(_ context.Context, id uuid.UUID) (*fin.AssessmentSchedule, error) {
	for i := range m.schedules {
		if m.schedules[i].ID == id {
			out := m.schedules[i]
			return &out, nil
		}
	}
	return nil, nil
}

func (m *mockAssessmentRepo) ListSchedulesByOrg(_ context.Context, orgID uuid.UUID) ([]fin.AssessmentSchedule, error) {
	var result []fin.AssessmentSchedule
	for _, s := range m.schedules {
		if s.OrgID == orgID {
			result = append(result, s)
		}
	}
	if result == nil {
		return []fin.AssessmentSchedule{}, nil
	}
	return result, nil
}

func (m *mockAssessmentRepo) UpdateSchedule(_ context.Context, s *fin.AssessmentSchedule) (*fin.AssessmentSchedule, error) {
	for i := range m.schedules {
		if m.schedules[i].ID == s.ID {
			s.UpdatedAt = time.Now()
			m.schedules[i] = *s
			out := m.schedules[i]
			return &out, nil
		}
	}
	return nil, nil
}

func (m *mockAssessmentRepo) DeactivateSchedule(_ context.Context, id uuid.UUID) error {
	for i := range m.schedules {
		if m.schedules[i].ID == id {
			m.schedules[i].IsActive = false
			return nil
		}
	}
	return nil
}

func (m *mockAssessmentRepo) CreateAssessment(_ context.Context, a *fin.Assessment) (*fin.Assessment, error) {
	a.ID = uuid.New()
	a.CreatedAt = time.Now()
	a.UpdatedAt = time.Now()
	m.assessments = append(m.assessments, *a)
	out := m.assessments[len(m.assessments)-1]
	return &out, nil
}

func (m *mockAssessmentRepo) FindAssessmentByID(_ context.Context, id uuid.UUID) (*fin.Assessment, error) {
	for i := range m.assessments {
		if m.assessments[i].ID == id {
			out := m.assessments[i]
			return &out, nil
		}
	}
	return nil, nil
}

func (m *mockAssessmentRepo) ListAssessmentsByOrg(_ context.Context, orgID uuid.UUID, limit int, afterID *uuid.UUID) ([]fin.Assessment, bool, error) {
	var result []fin.Assessment
	for _, a := range m.assessments {
		if a.OrgID == orgID {
			result = append(result, a)
		}
	}
	if result == nil {
		return []fin.Assessment{}, false, nil
	}
	hasMore := limit > 0 && len(result) > limit
	if hasMore {
		result = result[:limit]
	}
	return result, hasMore, nil
}

func (m *mockAssessmentRepo) ListAssessmentsByUnit(_ context.Context, unitID uuid.UUID) ([]fin.Assessment, error) {
	var result []fin.Assessment
	for _, a := range m.assessments {
		if a.UnitID == unitID {
			result = append(result, a)
		}
	}
	if result == nil {
		return []fin.Assessment{}, nil
	}
	return result, nil
}

func (m *mockAssessmentRepo) UpdateAssessment(_ context.Context, a *fin.Assessment) (*fin.Assessment, error) {
	for i := range m.assessments {
		if m.assessments[i].ID == a.ID {
			m.assessments[i] = *a
			out := m.assessments[i]
			return &out, nil
		}
	}
	return nil, nil
}

func (m *mockAssessmentRepo) SoftDeleteAssessment(_ context.Context, id uuid.UUID) error {
	now := time.Now()
	for i := range m.assessments {
		if m.assessments[i].ID == id {
			m.assessments[i].DeletedAt = &now
			return nil
		}
	}
	return nil
}

func (m *mockAssessmentRepo) CreateLedgerEntry(_ context.Context, entry *fin.LedgerEntry) (*fin.LedgerEntry, error) {
	entry.ID = uuid.New()
	entry.CreatedAt = time.Now()
	// Compute running balance for the unit.
	var prev int64
	for _, e := range m.ledger {
		if e.UnitID == entry.UnitID {
			prev = e.BalanceCents
		}
	}
	entry.BalanceCents = prev + entry.AmountCents
	m.ledger = append(m.ledger, *entry)
	out := m.ledger[len(m.ledger)-1]
	return &out, nil
}

func (m *mockAssessmentRepo) ListLedgerByUnit(_ context.Context, unitID uuid.UUID, limit int, afterID *uuid.UUID) ([]fin.LedgerEntry, bool, error) {
	var result []fin.LedgerEntry
	for _, e := range m.ledger {
		if e.UnitID == unitID {
			result = append(result, e)
		}
	}
	if result == nil {
		return []fin.LedgerEntry{}, false, nil
	}
	hasMore := limit > 0 && len(result) > limit
	if hasMore {
		result = result[:limit]
	}
	return result, hasMore, nil
}

func (m *mockAssessmentRepo) ListLedgerByOrg(_ context.Context, orgID uuid.UUID) ([]fin.LedgerEntry, error) {
	var result []fin.LedgerEntry
	for _, e := range m.ledger {
		if e.OrgID == orgID {
			result = append(result, e)
		}
	}
	if result == nil {
		return []fin.LedgerEntry{}, nil
	}
	return result, nil
}

func (m *mockAssessmentRepo) GetUnitBalance(_ context.Context, unitID uuid.UUID) (int64, error) {
	var balance int64
	for _, e := range m.ledger {
		if e.UnitID == unitID {
			balance = e.BalanceCents
		}
	}
	return balance, nil
}

func (m *mockAssessmentRepo) FindLedgerEntryByID(_ context.Context, id uuid.UUID) (*fin.LedgerEntry, error) {
	for i := range m.ledger {
		if m.ledger[i].ID == id {
			out := m.ledger[i]
			return &out, nil
		}
	}
	return nil, nil
}

func (m *mockAssessmentRepo) FindLedgerEntriesByAssessment(_ context.Context, assessmentID uuid.UUID) ([]fin.LedgerEntry, error) {
	var result []fin.LedgerEntry
	for _, e := range m.ledger {
		if e.AssessmentID != nil && *e.AssessmentID == assessmentID {
			result = append(result, e)
		}
	}
	if result == nil {
		return []fin.LedgerEntry{}, nil
	}
	return result, nil
}

func (m *mockAssessmentRepo) FindLedgerEntryByPaymentRef(_ context.Context, paymentID uuid.UUID) (*fin.LedgerEntry, error) {
	for i := range m.ledger {
		e := &m.ledger[i]
		if e.ReferenceType != nil && *e.ReferenceType == fin.LedgerRefTypePayment &&
			e.ReferenceID != nil && *e.ReferenceID == paymentID {
			out := *e
			return &out, nil
		}
	}
	return nil, nil
}

func (m *mockAssessmentRepo) UpdateLedgerEntryReversedBy(_ context.Context, entryID, reversalEntryID uuid.UUID) error {
	for i := range m.ledger {
		if m.ledger[i].ID == entryID {
			m.ledger[i].ReversedByEntryID = &reversalEntryID
			return nil
		}
	}
	return nil
}

func (m *mockAssessmentRepo) UpdateAssessmentStatus(_ context.Context, id uuid.UUID, status fin.AssessmentStatus, voidedBy *uuid.UUID, voidedAt *time.Time) error {
	for i := range m.assessments {
		if m.assessments[i].ID == id {
			m.assessments[i].Status = status
			m.assessments[i].VoidedBy = voidedBy
			m.assessments[i].VoidedAt = voidedAt
			return nil
		}
	}
	return nil
}

func (m *mockAssessmentRepo) WithTx(_ pgx.Tx) fin.AssessmentRepository { return m }

// mockPaymentRepo is an in-memory implementation of PaymentRepository.
type mockPaymentRepo struct {
	payments []fin.Payment
	methods  []fin.PaymentMethod
}

func (m *mockPaymentRepo) CreatePayment(_ context.Context, p *fin.Payment) (*fin.Payment, error) {
	p.ID = uuid.New()
	p.CreatedAt = time.Now()
	p.UpdatedAt = time.Now()
	m.payments = append(m.payments, *p)
	out := m.payments[len(m.payments)-1]
	return &out, nil
}

func (m *mockPaymentRepo) FindPaymentByID(_ context.Context, id uuid.UUID) (*fin.Payment, error) {
	for i := range m.payments {
		if m.payments[i].ID == id {
			out := m.payments[i]
			return &out, nil
		}
	}
	return nil, nil
}

func (m *mockPaymentRepo) ListPaymentsByOrg(_ context.Context, orgID uuid.UUID, limit int, afterID *uuid.UUID) ([]fin.Payment, bool, error) {
	var result []fin.Payment
	for _, p := range m.payments {
		if p.OrgID == orgID {
			result = append(result, p)
		}
	}
	if result == nil {
		return []fin.Payment{}, false, nil
	}
	hasMore := limit > 0 && len(result) > limit
	if hasMore {
		result = result[:limit]
	}
	return result, hasMore, nil
}

func (m *mockPaymentRepo) ListPaymentsByUnit(_ context.Context, unitID uuid.UUID) ([]fin.Payment, error) {
	var result []fin.Payment
	for _, p := range m.payments {
		if p.UnitID == unitID {
			result = append(result, p)
		}
	}
	if result == nil {
		return []fin.Payment{}, nil
	}
	return result, nil
}

func (m *mockPaymentRepo) UpdatePaymentStatus(_ context.Context, id uuid.UUID, status fin.PaymentStatus, paidAt *time.Time) error {
	for i := range m.payments {
		if m.payments[i].ID == id {
			m.payments[i].Status = status
			m.payments[i].PaidAt = paidAt
			return nil
		}
	}
	return nil
}

func (m *mockPaymentRepo) UpdatePaymentVoid(_ context.Context, id uuid.UUID, voidedBy uuid.UUID, voidedAt time.Time) error {
	for i := range m.payments {
		if m.payments[i].ID == id {
			m.payments[i].Status = fin.PaymentStatusVoid
			m.payments[i].VoidedBy = &voidedBy
			m.payments[i].VoidedAt = &voidedAt
			return nil
		}
	}
	return nil
}

func (m *mockPaymentRepo) CreatePaymentMethod(_ context.Context, pm *fin.PaymentMethod) (*fin.PaymentMethod, error) {
	pm.ID = uuid.New()
	pm.CreatedAt = time.Now()
	m.methods = append(m.methods, *pm)
	out := m.methods[len(m.methods)-1]
	return &out, nil
}

func (m *mockPaymentRepo) ListPaymentMethodsByUser(_ context.Context, userID uuid.UUID) ([]fin.PaymentMethod, error) {
	var result []fin.PaymentMethod
	for _, pm := range m.methods {
		if pm.UserID == userID && pm.DeletedAt == nil {
			result = append(result, pm)
		}
	}
	if result == nil {
		return []fin.PaymentMethod{}, nil
	}
	return result, nil
}

func (m *mockPaymentRepo) SoftDeletePaymentMethod(_ context.Context, id uuid.UUID) error {
	now := time.Now()
	for i := range m.methods {
		if m.methods[i].ID == id {
			m.methods[i].DeletedAt = &now
			return nil
		}
	}
	return nil
}

func (m *mockPaymentRepo) CreatePaymentAllocation(_ context.Context, a *fin.PaymentAllocation) (*fin.PaymentAllocation, error) {
	a.ID = uuid.New()
	a.CreatedAt = time.Now()
	return a, nil
}

func (m *mockPaymentRepo) ListAllocationsByPayment(_ context.Context, _ uuid.UUID) ([]fin.PaymentAllocation, error) {
	return []fin.PaymentAllocation{}, nil
}

func (m *mockPaymentRepo) WithTx(_ pgx.Tx) fin.PaymentRepository { return m }

// mockBudgetRepo is an in-memory implementation of BudgetRepository.
type mockBudgetRepo struct {
	categories []fin.BudgetCategory
	budgets    []fin.Budget
	lineItems  []fin.BudgetLineItem
	expenses   []fin.Expense
}

func (m *mockBudgetRepo) CreateCategory(_ context.Context, c *fin.BudgetCategory) (*fin.BudgetCategory, error) {
	c.ID = uuid.New()
	c.CreatedAt = time.Now()
	m.categories = append(m.categories, *c)
	out := m.categories[len(m.categories)-1]
	return &out, nil
}

func (m *mockBudgetRepo) ListCategoriesByOrg(_ context.Context, orgID uuid.UUID) ([]fin.BudgetCategory, error) {
	var result []fin.BudgetCategory
	for _, c := range m.categories {
		if c.OrgID == orgID {
			result = append(result, c)
		}
	}
	if result == nil {
		return []fin.BudgetCategory{}, nil
	}
	return result, nil
}

func (m *mockBudgetRepo) UpdateCategory(_ context.Context, c *fin.BudgetCategory) (*fin.BudgetCategory, error) {
	for i := range m.categories {
		if m.categories[i].ID == c.ID {
			m.categories[i] = *c
			out := m.categories[i]
			return &out, nil
		}
	}
	return nil, nil
}

func (m *mockBudgetRepo) CreateBudget(_ context.Context, b *fin.Budget) (*fin.Budget, error) {
	b.ID = uuid.New()
	b.CreatedAt = time.Now()
	b.UpdatedAt = time.Now()
	m.budgets = append(m.budgets, *b)
	out := m.budgets[len(m.budgets)-1]
	return &out, nil
}

func (m *mockBudgetRepo) FindBudgetByID(_ context.Context, id uuid.UUID) (*fin.Budget, error) {
	for i := range m.budgets {
		if m.budgets[i].ID == id && m.budgets[i].DeletedAt == nil {
			out := m.budgets[i]
			return &out, nil
		}
	}
	return nil, nil
}

func (m *mockBudgetRepo) ListBudgetsByOrg(_ context.Context, orgID uuid.UUID) ([]fin.Budget, error) {
	var result []fin.Budget
	for _, b := range m.budgets {
		if b.OrgID == orgID && b.DeletedAt == nil {
			result = append(result, b)
		}
	}
	if result == nil {
		return []fin.Budget{}, nil
	}
	return result, nil
}

func (m *mockBudgetRepo) UpdateBudget(_ context.Context, b *fin.Budget) (*fin.Budget, error) {
	for i := range m.budgets {
		if m.budgets[i].ID == b.ID {
			b.UpdatedAt = time.Now()
			m.budgets[i] = *b
			out := m.budgets[i]
			return &out, nil
		}
	}
	return nil, nil
}

func (m *mockBudgetRepo) SoftDeleteBudget(_ context.Context, id uuid.UUID) error {
	now := time.Now()
	for i := range m.budgets {
		if m.budgets[i].ID == id {
			m.budgets[i].DeletedAt = &now
			return nil
		}
	}
	return nil
}

func (m *mockBudgetRepo) CreateLineItem(_ context.Context, item *fin.BudgetLineItem) (*fin.BudgetLineItem, error) {
	item.ID = uuid.New()
	item.CreatedAt = time.Now()
	item.UpdatedAt = time.Now()
	m.lineItems = append(m.lineItems, *item)
	out := m.lineItems[len(m.lineItems)-1]
	return &out, nil
}

func (m *mockBudgetRepo) ListLineItemsByBudget(_ context.Context, budgetID uuid.UUID) ([]fin.BudgetLineItem, error) {
	var result []fin.BudgetLineItem
	for _, item := range m.lineItems {
		if item.BudgetID == budgetID {
			result = append(result, item)
		}
	}
	if result == nil {
		return []fin.BudgetLineItem{}, nil
	}
	return result, nil
}

func (m *mockBudgetRepo) UpdateLineItem(_ context.Context, item *fin.BudgetLineItem) (*fin.BudgetLineItem, error) {
	for i := range m.lineItems {
		if m.lineItems[i].ID == item.ID {
			item.BudgetID = m.lineItems[i].BudgetID
			item.UpdatedAt = time.Now()
			m.lineItems[i] = *item
			out := m.lineItems[i]
			return &out, nil
		}
	}
	return nil, nil
}

func (m *mockBudgetRepo) DeleteLineItem(_ context.Context, id uuid.UUID) error {
	for i := range m.lineItems {
		if m.lineItems[i].ID == id {
			m.lineItems = append(m.lineItems[:i], m.lineItems[i+1:]...)
			return nil
		}
	}
	return nil
}

func (m *mockBudgetRepo) FindLineItemByID(_ context.Context, id uuid.UUID) (*fin.BudgetLineItem, error) {
	for i := range m.lineItems {
		if m.lineItems[i].ID == id {
			out := m.lineItems[i]
			return &out, nil
		}
	}
	return nil, nil
}

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
	return fmt.Errorf("fin: RecalculateBudgetTotals: budget %s not found", budgetID)
}

func (m *mockBudgetRepo) CreateExpense(_ context.Context, e *fin.Expense) (*fin.Expense, error) {
	e.ID = uuid.New()
	e.CreatedAt = time.Now()
	e.UpdatedAt = time.Now()
	m.expenses = append(m.expenses, *e)
	out := m.expenses[len(m.expenses)-1]
	return &out, nil
}

func (m *mockBudgetRepo) FindExpenseByID(_ context.Context, id uuid.UUID) (*fin.Expense, error) {
	for i := range m.expenses {
		if m.expenses[i].ID == id && m.expenses[i].DeletedAt == nil {
			out := m.expenses[i]
			return &out, nil
		}
	}
	return nil, nil
}

func (m *mockBudgetRepo) ListExpensesByOrg(_ context.Context, orgID uuid.UUID) ([]fin.Expense, error) {
	var result []fin.Expense
	for _, e := range m.expenses {
		if e.OrgID == orgID && e.DeletedAt == nil {
			result = append(result, e)
		}
	}
	if result == nil {
		return []fin.Expense{}, nil
	}
	return result, nil
}

func (m *mockBudgetRepo) UpdateExpense(_ context.Context, e *fin.Expense) (*fin.Expense, error) {
	for i := range m.expenses {
		if m.expenses[i].ID == e.ID {
			e.UpdatedAt = time.Now()
			m.expenses[i] = *e
			out := m.expenses[i]
			return &out, nil
		}
	}
	return nil, nil
}

func (m *mockBudgetRepo) SoftDeleteExpense(_ context.Context, id uuid.UUID) error {
	now := time.Now()
	for i := range m.expenses {
		if m.expenses[i].ID == id {
			m.expenses[i].DeletedAt = &now
			return nil
		}
	}
	return nil
}

func (m *mockBudgetRepo) WithTx(_ pgx.Tx) fin.BudgetRepository { return m }

// mockFundRepo is an in-memory implementation of FundRepository.
type mockFundRepo struct {
	funds        []fin.Fund
	transactions []fin.FundTransaction
	transfers    []fin.FundTransfer
}

func (m *mockFundRepo) CreateFund(_ context.Context, f *fin.Fund) (*fin.Fund, error) {
	f.ID = uuid.New()
	f.CreatedAt = time.Now()
	f.UpdatedAt = time.Now()
	m.funds = append(m.funds, *f)
	out := m.funds[len(m.funds)-1]
	return &out, nil
}

func (m *mockFundRepo) FindFundByID(_ context.Context, id uuid.UUID) (*fin.Fund, error) {
	for i := range m.funds {
		if m.funds[i].ID == id && m.funds[i].DeletedAt == nil {
			out := m.funds[i]
			return &out, nil
		}
	}
	return nil, nil
}

func (m *mockFundRepo) ListFundsByOrg(_ context.Context, orgID uuid.UUID) ([]fin.Fund, error) {
	var result []fin.Fund
	for _, f := range m.funds {
		if f.OrgID == orgID && f.DeletedAt == nil {
			result = append(result, f)
		}
	}
	if result == nil {
		return []fin.Fund{}, nil
	}
	return result, nil
}

func (m *mockFundRepo) UpdateFund(_ context.Context, f *fin.Fund) (*fin.Fund, error) {
	for i := range m.funds {
		if m.funds[i].ID == f.ID {
			f.UpdatedAt = time.Now()
			m.funds[i] = *f
			out := m.funds[i]
			return &out, nil
		}
	}
	return nil, nil
}

func (m *mockFundRepo) CreateTransaction(_ context.Context, t *fin.FundTransaction) (*fin.FundTransaction, error) {
	t.ID = uuid.New()
	t.CreatedAt = time.Now()
	// Mirror the real repo: update the parent fund's balance.
	for i := range m.funds {
		if m.funds[i].ID == t.FundID && m.funds[i].DeletedAt == nil {
			m.funds[i].BalanceCents += t.AmountCents
			t.BalanceAfterCents = m.funds[i].BalanceCents
			m.transactions = append(m.transactions, *t)
			out := m.transactions[len(m.transactions)-1]
			return &out, nil
		}
	}
	return nil, fmt.Errorf("fin: CreateTransaction: fund %s not found or deleted", t.FundID)
}

func (m *mockFundRepo) ListTransactionsByFund(_ context.Context, fundID uuid.UUID) ([]fin.FundTransaction, error) {
	var result []fin.FundTransaction
	for _, t := range m.transactions {
		if t.FundID == fundID {
			result = append(result, t)
		}
	}
	if result == nil {
		return []fin.FundTransaction{}, nil
	}
	return result, nil
}

func (m *mockFundRepo) CreateTransfer(_ context.Context, t *fin.FundTransfer) (*fin.FundTransfer, error) {
	t.ID = uuid.New()
	t.CreatedAt = time.Now()
	m.transfers = append(m.transfers, *t)
	out := m.transfers[len(m.transfers)-1]
	return &out, nil
}

func (m *mockFundRepo) ListTransfersByOrg(_ context.Context, orgID uuid.UUID) ([]fin.FundTransfer, error) {
	var result []fin.FundTransfer
	for _, t := range m.transfers {
		if t.OrgID == orgID {
			result = append(result, t)
		}
	}
	if result == nil {
		return []fin.FundTransfer{}, nil
	}
	return result, nil
}

func (m *mockFundRepo) WithTx(_ pgx.Tx) fin.FundRepository { return m }

// mockCollectionRepo is an in-memory implementation of CollectionRepository.
type mockCollectionRepo struct {
	cases        []fin.CollectionCase
	actions      []fin.CollectionAction
	paymentPlans []fin.PaymentPlan
}

func (m *mockCollectionRepo) CreateCase(_ context.Context, c *fin.CollectionCase) (*fin.CollectionCase, error) {
	c.ID = uuid.New()
	c.CreatedAt = time.Now()
	c.UpdatedAt = time.Now()
	m.cases = append(m.cases, *c)
	out := m.cases[len(m.cases)-1]
	return &out, nil
}

func (m *mockCollectionRepo) FindCaseByID(_ context.Context, id uuid.UUID) (*fin.CollectionCase, error) {
	for i := range m.cases {
		if m.cases[i].ID == id {
			out := m.cases[i]
			return &out, nil
		}
	}
	return nil, nil
}

func (m *mockCollectionRepo) ListCasesByOrg(_ context.Context, orgID uuid.UUID) ([]fin.CollectionCase, error) {
	var result []fin.CollectionCase
	for _, c := range m.cases {
		if c.OrgID == orgID {
			result = append(result, c)
		}
	}
	if result == nil {
		return []fin.CollectionCase{}, nil
	}
	return result, nil
}

func (m *mockCollectionRepo) UpdateCase(_ context.Context, c *fin.CollectionCase) (*fin.CollectionCase, error) {
	for i := range m.cases {
		if m.cases[i].ID == c.ID {
			c.UpdatedAt = time.Now()
			m.cases[i] = *c
			out := m.cases[i]
			return &out, nil
		}
	}
	return nil, nil
}

func (m *mockCollectionRepo) CreateAction(_ context.Context, a *fin.CollectionAction) (*fin.CollectionAction, error) {
	a.ID = uuid.New()
	a.CreatedAt = time.Now()
	m.actions = append(m.actions, *a)
	out := m.actions[len(m.actions)-1]
	return &out, nil
}

func (m *mockCollectionRepo) ListActionsByCase(_ context.Context, caseID uuid.UUID) ([]fin.CollectionAction, error) {
	var result []fin.CollectionAction
	for _, a := range m.actions {
		if a.CaseID == caseID {
			result = append(result, a)
		}
	}
	if result == nil {
		return []fin.CollectionAction{}, nil
	}
	return result, nil
}

func (m *mockCollectionRepo) CreatePaymentPlan(_ context.Context, p *fin.PaymentPlan) (*fin.PaymentPlan, error) {
	p.ID = uuid.New()
	p.CreatedAt = time.Now()
	p.UpdatedAt = time.Now()
	m.paymentPlans = append(m.paymentPlans, *p)
	out := m.paymentPlans[len(m.paymentPlans)-1]
	return &out, nil
}

func (m *mockCollectionRepo) ListPaymentPlansByCase(_ context.Context, caseID uuid.UUID) ([]fin.PaymentPlan, error) {
	var result []fin.PaymentPlan
	for _, p := range m.paymentPlans {
		if p.CaseID == caseID {
			result = append(result, p)
		}
	}
	if result == nil {
		return []fin.PaymentPlan{}, nil
	}
	return result, nil
}

func (m *mockCollectionRepo) UpdatePaymentPlan(_ context.Context, p *fin.PaymentPlan) (*fin.PaymentPlan, error) {
	for i := range m.paymentPlans {
		if m.paymentPlans[i].ID == p.ID {
			p.UpdatedAt = time.Now()
			m.paymentPlans[i] = *p
			out := m.paymentPlans[i]
			return &out, nil
		}
	}
	return nil, nil
}

func (m *mockCollectionRepo) GetCollectionStatusForUnit(_ context.Context, unitID uuid.UUID) (*fin.CollectionCase, error) {
	for i := range m.cases {
		if m.cases[i].UnitID == unitID && m.cases[i].ClosedAt == nil {
			out := m.cases[i]
			return &out, nil
		}
	}
	return nil, nil
}

func (m *mockCollectionRepo) WithTx(_ pgx.Tx) fin.CollectionRepository { return m }

// ── Helper ────────────────────────────────────────────────────────────────────

func newTestService() (*fin.FinService, *mockAssessmentRepo, *mockPaymentRepo, *mockBudgetRepo, *mockFundRepo, *mockCollectionRepo) {
	assessments := &mockAssessmentRepo{}
	payments := &mockPaymentRepo{}
	budgets := &mockBudgetRepo{}
	funds := &mockFundRepo{}
	collections := &mockCollectionRepo{}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	svc := fin.NewFinService(assessments, payments, budgets, funds, collections, nil, nil, ai.NewNoopPolicyResolver(), ai.NewNoopComplianceResolver(), nil, logger, nil)
	return svc, assessments, payments, budgets, funds, collections
}

// ── Tests ─────────────────────────────────────────────────────────────────────

// TestCreateAssessment_CreatesLedgerEntry verifies that creating an assessment
// also produces a "charge" ledger entry for the unit.
func TestCreateAssessment_CreatesLedgerEntry(t *testing.T) {
	svc, assessmentRepo, _, _, _, _ := newTestService()
	ctx := context.Background()
	orgID := uuid.New()
	unitID := uuid.New()

	req := fin.CreateAssessmentRequest{
		UnitID:      unitID,
		Description: "Monthly HOA fee",
		AmountCents: 15000,
		DueDate:     time.Now().AddDate(0, 1, 0),
	}

	assessment, err := svc.CreateAssessment(ctx, orgID, req)
	require.NoError(t, err)
	assert.NotNil(t, assessment)
	assert.Equal(t, int64(15000), assessment.AmountCents)

	// Verify a ledger entry was created.
	require.Len(t, assessmentRepo.ledger, 1)
	entry := assessmentRepo.ledger[0]
	assert.Equal(t, fin.LedgerEntryTypeCharge, entry.EntryType)
	assert.Equal(t, int64(15000), entry.AmountCents)
	assert.Equal(t, unitID, entry.UnitID)
	assert.Equal(t, orgID, entry.OrgID)
	require.NotNil(t, entry.AssessmentID)
	assert.Equal(t, assessment.ID, *entry.AssessmentID)
}

// TestRecordPayment_CreatesLedgerEntry verifies that recording a payment creates
// a "payment" credit ledger entry (negative amount).
func TestRecordPayment_CreatesLedgerEntry(t *testing.T) {
	svc, assessmentRepo, _, _, _, _ := newTestService()
	ctx := context.Background()
	orgID := uuid.New()
	unitID := uuid.New()
	userID := uuid.New()

	req := fin.CreatePaymentRequest{
		UnitID:      unitID,
		AmountCents: 15000,
	}

	payment, err := svc.RecordPayment(ctx, orgID, userID, req)
	require.NoError(t, err)
	assert.NotNil(t, payment)
	assert.Equal(t, int64(15000), payment.AmountCents)
	assert.Equal(t, fin.PaymentStatusCompleted, payment.Status)

	// Verify a ledger entry was created with negative amount (credit).
	require.Len(t, assessmentRepo.ledger, 1)
	entry := assessmentRepo.ledger[0]
	assert.Equal(t, fin.LedgerEntryTypePayment, entry.EntryType)
	assert.Equal(t, int64(-15000), entry.AmountCents)
	assert.Equal(t, unitID, entry.UnitID)
	assert.Equal(t, orgID, entry.OrgID)
}

// TestRecordPayment_Validation verifies that a negative amount is rejected.
func TestRecordPayment_Validation(t *testing.T) {
	svc, _, _, _, _, _ := newTestService()
	ctx := context.Background()
	orgID := uuid.New()
	userID := uuid.New()

	req := fin.CreatePaymentRequest{
		UnitID:      uuid.New(),
		AmountCents: -500,
	}

	_, err := svc.RecordPayment(ctx, orgID, userID, req)
	require.Error(t, err)

	var valErr *api.ValidationError
	require.ErrorAs(t, err, &valErr)
	assert.Equal(t, "amount_cents", valErr.Field)
}

// TestProposeBudget_SetsStatus verifies that ProposeBudget changes status from
// "draft" to "proposed".
func TestProposeBudget_SetsStatus(t *testing.T) {
	svc, _, _, _, _, _ := newTestService()
	ctx := context.Background()
	orgID := uuid.New()
	userID := uuid.New()

	created, err := svc.CreateBudget(ctx, orgID, userID, fin.CreateBudgetRequest{
		FiscalYear: 2025,
		Name:       "FY2025 Budget",
	})
	require.NoError(t, err)
	assert.Equal(t, fin.BudgetStatusDraft, created.Status)

	proposed, err := svc.ProposeBudget(ctx, created.ID, userID)
	require.NoError(t, err)
	assert.Equal(t, fin.BudgetStatusProposed, proposed.Status)
	assert.NotNil(t, proposed.ProposedAt)
	assert.NotNil(t, proposed.ProposedBy)
	assert.Equal(t, userID, *proposed.ProposedBy)
}

// TestApproveBudget_SetsStatus verifies that ApproveBudget changes status from
// "proposed" to "approved".
func TestApproveBudget_SetsStatus(t *testing.T) {
	svc, _, _, _, _, _ := newTestService()
	ctx := context.Background()
	orgID := uuid.New()
	userID := uuid.New()

	created, err := svc.CreateBudget(ctx, orgID, userID, fin.CreateBudgetRequest{
		FiscalYear: 2025,
		Name:       "FY2025 Budget",
	})
	require.NoError(t, err)

	_, err = svc.ProposeBudget(ctx, created.ID, userID)
	require.NoError(t, err)

	approved, err := svc.ApproveBudget(ctx, created.ID, userID)
	require.NoError(t, err)
	assert.Equal(t, fin.BudgetStatusApproved, approved.Status)
	assert.NotNil(t, approved.ApprovedAt)
	assert.NotNil(t, approved.ApprovedBy)
	assert.Equal(t, userID, *approved.ApprovedBy)
}

// TestApproveBudget_RejectsWhenNotProposed ensures that a draft budget cannot be
// approved directly.
func TestApproveBudget_RejectsWhenNotProposed(t *testing.T) {
	svc, _, _, _, _, _ := newTestService()
	ctx := context.Background()
	orgID := uuid.New()
	userID := uuid.New()

	created, err := svc.CreateBudget(ctx, orgID, userID, fin.CreateBudgetRequest{
		FiscalYear: 2025,
		Name:       "FY2025 Budget",
	})
	require.NoError(t, err)
	assert.Equal(t, fin.BudgetStatusDraft, created.Status)

	_, err = svc.ApproveBudget(ctx, created.ID, userID)
	require.Error(t, err)

	var valErr *api.ValidationError
	require.ErrorAs(t, err, &valErr)
}

// TestApproveExpense_SetsStatus verifies that ApproveExpense transitions from
// "submitted" to "approved".
func TestApproveExpense_SetsStatus(t *testing.T) {
	svc, _, _, _, _, _ := newTestService()
	ctx := context.Background()
	orgID := uuid.New()
	userID := uuid.New()

	created, err := svc.CreateExpense(ctx, orgID, userID, fin.CreateExpenseRequest{
		Description: "Landscaping",
		AmountCents: 50000,
		ExpenseDate: time.Now(),
	})
	require.NoError(t, err)
	assert.Equal(t, fin.ExpenseStatusSubmitted, created.Status)

	approved, err := svc.ApproveExpense(ctx, created.ID, userID)
	require.NoError(t, err)
	assert.Equal(t, fin.ExpenseStatusApproved, approved.Status)
	assert.NotNil(t, approved.ApprovedAt)
	assert.NotNil(t, approved.ApprovedBy)
	assert.Equal(t, userID, *approved.ApprovedBy)
}

// TestPayExpense_SetsPaidDate verifies that PayExpense transitions from "approved"
// to "paid" and sets paid_date.
func TestPayExpense_SetsPaidDate(t *testing.T) {
	svc, _, _, _, _, _ := newTestService()
	ctx := context.Background()
	orgID := uuid.New()
	userID := uuid.New()

	created, err := svc.CreateExpense(ctx, orgID, userID, fin.CreateExpenseRequest{
		Description: "Landscaping",
		AmountCents: 50000,
		ExpenseDate: time.Now(),
	})
	require.NoError(t, err)

	approved, err := svc.ApproveExpense(ctx, created.ID, userID)
	require.NoError(t, err)
	assert.Equal(t, fin.ExpenseStatusApproved, approved.Status)

	paid, err := svc.PayExpense(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, fin.ExpenseStatusPaid, paid.Status)
	assert.NotNil(t, paid.PaidDate)
}

// TestPayExpense_RejectsWhenNotApproved ensures that a submitted expense cannot be
// paid directly.
func TestPayExpense_RejectsWhenNotApproved(t *testing.T) {
	svc, _, _, _, _, _ := newTestService()
	ctx := context.Background()
	orgID := uuid.New()
	userID := uuid.New()

	created, err := svc.CreateExpense(ctx, orgID, userID, fin.CreateExpenseRequest{
		Description: "Landscaping",
		AmountCents: 50000,
		ExpenseDate: time.Now(),
	})
	require.NoError(t, err)

	_, err = svc.PayExpense(ctx, created.ID)
	require.Error(t, err)

	var valErr *api.ValidationError
	require.ErrorAs(t, err, &valErr)
}

// TestGetSchedule_NotFound verifies that a 404 error is returned when the
// schedule does not exist.
func TestGetSchedule_NotFound(t *testing.T) {
	svc, _, _, _, _, _ := newTestService()
	ctx := context.Background()

	_, err := svc.GetSchedule(ctx, uuid.New())
	require.Error(t, err)

	var notFound *api.NotFoundError
	require.ErrorAs(t, err, &notFound)
}

// TestCreateFund_Validation verifies that invalid fund_type is rejected.
func TestCreateFund_Validation(t *testing.T) {
	svc, _, _, _, _, _ := newTestService()
	ctx := context.Background()
	orgID := uuid.New()

	_, err := svc.CreateFund(ctx, orgID, fin.CreateFundRequest{
		Name:     "Bad Fund",
		FundType: "invalid_type",
	})
	require.Error(t, err)

	var valErr *api.ValidationError
	require.ErrorAs(t, err, &valErr)
	assert.Equal(t, "fund_type", valErr.Field)
}

// TestCreateFund_Success verifies that a valid fund request persists correctly.
func TestCreateFund_Success(t *testing.T) {
	svc, _, _, _, fundRepo, _ := newTestService()
	ctx := context.Background()
	orgID := uuid.New()

	fund, err := svc.CreateFund(ctx, orgID, fin.CreateFundRequest{
		Name:     "Operating Fund",
		FundType: "operating",
	})
	require.NoError(t, err)
	assert.Equal(t, "Operating Fund", fund.Name)
	assert.Equal(t, fin.FundTypeOperating, fund.FundType)
	assert.Equal(t, int64(0), fund.BalanceCents)
	assert.Len(t, fundRepo.funds, 1)
}

// TestGetAssessment_NotFound verifies that a 404 error is returned when the
// assessment does not exist.
func TestGetAssessment_NotFound(t *testing.T) {
	svc, _, _, _, _, _ := newTestService()
	ctx := context.Background()

	_, err := svc.GetAssessment(ctx, uuid.New())
	require.Error(t, err)

	var notFound *api.NotFoundError
	require.ErrorAs(t, err, &notFound)
}

// TestCreateAssessment_Validation verifies that invalid requests are rejected.
func TestCreateAssessment_Validation(t *testing.T) {
	svc, _, _, _, _, _ := newTestService()
	ctx := context.Background()
	orgID := uuid.New()

	// Missing unit_id.
	_, err := svc.CreateAssessment(ctx, orgID, fin.CreateAssessmentRequest{
		Description: "Fee",
		AmountCents: 1000,
		DueDate:     time.Now().AddDate(0, 1, 0),
	})
	require.Error(t, err)

	var valErr *api.ValidationError
	require.ErrorAs(t, err, &valErr)
	assert.Equal(t, "unit_id", valErr.Field)
}

// TestCreateSchedule_SetsOrgID verifies that the org_id is set on the schedule.
func TestCreateSchedule_SetsOrgID(t *testing.T) {
	svc, repo, _, _, _, _ := newTestService()
	ctx := context.Background()
	orgID := uuid.New()

	req := fin.CreateAssessmentScheduleRequest{
		Name:            "Monthly HOA",
		Frequency:       "monthly",
		AmountStrategy:  "flat",
		BaseAmountCents: 15000,
		StartsAt:        time.Now(),
	}

	schedule, err := svc.CreateSchedule(ctx, orgID, req)
	require.NoError(t, err)
	assert.Equal(t, orgID, schedule.OrgID)
	assert.True(t, schedule.IsActive)
	assert.Len(t, repo.schedules, 1)
}

// ── GL Integration Tests ────────────────────────────────────────────────────

// TestCreateAssessment_PostsJournalEntry verifies that creating an assessment
// posts a GL journal entry debiting AR (1100) and crediting Revenue (4010).
func TestCreateAssessment_PostsJournalEntry(t *testing.T) {
	assessments := &mockAssessmentRepo{}
	payments := &mockPaymentRepo{}
	budgets := &mockBudgetRepo{}
	funds := &mockFundRepo{}
	collections := &mockCollectionRepo{}
	glRepo := newMockGLRepo()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	glService := fin.NewGLService(glRepo, audit.NewNoopAuditor(), logger)

	orgID := uuid.New()
	ctx := context.Background()

	// Seed the GL accounts the FinService will look up.
	arAccount := &fin.GLAccount{
		ID: uuid.New(), OrgID: orgID, AccountNumber: 1100, Name: "AR-Assessments", AccountType: fin.GLAccountTypeAsset,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	revenueAccount := &fin.GLAccount{
		ID: uuid.New(), OrgID: orgID, AccountNumber: 4010, Name: "Assessment Revenue-Operating", AccountType: fin.GLAccountTypeRevenue,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	glRepo.accounts[arAccount.ID] = arAccount
	glRepo.accounts[revenueAccount.ID] = revenueAccount

	svc := fin.NewFinService(assessments, payments, budgets, funds, collections, glService, fin.NewGaapEngine(), ai.NewNoopPolicyResolver(), ai.NewNoopComplianceResolver(), nil, logger, nil)

	unitID := uuid.New()
	req := fin.CreateAssessmentRequest{
		UnitID:      unitID,
		Description: "Monthly HOA fee",
		AmountCents: 15000,
		DueDate:     time.Now().AddDate(0, 1, 0),
	}

	assessment, err := svc.CreateAssessment(ctx, orgID, req)
	require.NoError(t, err)
	require.NotNil(t, assessment)

	// Verify exactly 1 GL journal entry was posted.
	require.Len(t, glRepo.entries, 1)
	for _, entry := range glRepo.entries {
		require.Len(t, entry.Lines, 2)
		// Line 0: Debit AR
		assert.Equal(t, arAccount.ID, entry.Lines[0].AccountID)
		assert.Equal(t, int64(15000), entry.Lines[0].DebitCents)
		assert.Equal(t, int64(0), entry.Lines[0].CreditCents)
		// Line 1: Credit Revenue
		assert.Equal(t, revenueAccount.ID, entry.Lines[1].AccountID)
		assert.Equal(t, int64(0), entry.Lines[1].DebitCents)
		assert.Equal(t, int64(15000), entry.Lines[1].CreditCents)
	}
}

// TestCreateAssessment_GLFailureReturnsError verifies that a GL posting error
// now propagates to the caller instead of being silently swallowed.
func TestCreateAssessment_GLFailureReturnsError(t *testing.T) {
	assessments := &mockAssessmentRepo{}
	payments := &mockPaymentRepo{}
	budgets := &mockBudgetRepo{}
	funds := &mockFundRepo{}
	collections := &mockCollectionRepo{}

	glRepo := newMockGLRepo()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	glService := fin.NewGLService(glRepo, audit.NewNoopAuditor(), logger)

	orgID := uuid.New()

	arAccount := &fin.GLAccount{
		ID: uuid.New(), OrgID: orgID, AccountNumber: 1100, Name: "AR",
		AccountType: fin.GLAccountTypeAsset, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	revenueAccount := &fin.GLAccount{
		ID: uuid.New(), OrgID: orgID, AccountNumber: 4010, Name: "Revenue",
		AccountType: fin.GLAccountTypeRevenue, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	glRepo.SetAccounts(arAccount, revenueAccount)
	glRepo.SetPostError(fmt.Errorf("simulated GL failure"))

	svc := fin.NewFinService(assessments, payments, budgets, funds, collections, glService, fin.NewGaapEngine(), ai.NewNoopPolicyResolver(), ai.NewNoopComplianceResolver(), nil, logger, nil)

	_, err := svc.CreateAssessment(context.Background(), orgID, fin.CreateAssessmentRequest{
		UnitID:      uuid.New(),
		Description: "Test assessment",
		AmountCents: 10000,
		DueDate:     time.Now().Add(30 * 24 * time.Hour),
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "simulated GL failure")
}

// TestRecordPayment_PostsJournalEntry verifies that recording a payment
// posts a GL journal entry debiting Cash (1010) and crediting AR (1100).
func TestRecordPayment_PostsJournalEntry(t *testing.T) {
	assessments := &mockAssessmentRepo{}
	payments := &mockPaymentRepo{}
	budgets := &mockBudgetRepo{}
	funds := &mockFundRepo{}
	collections := &mockCollectionRepo{}
	glRepo := newMockGLRepo()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	glService := fin.NewGLService(glRepo, audit.NewNoopAuditor(), logger)

	orgID := uuid.New()
	ctx := context.Background()

	// Seed the GL accounts the FinService will look up.
	cashAccount := &fin.GLAccount{
		ID: uuid.New(), OrgID: orgID, AccountNumber: 1010, Name: "Cash-Operating", AccountType: fin.GLAccountTypeAsset,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	arAccount := &fin.GLAccount{
		ID: uuid.New(), OrgID: orgID, AccountNumber: 1100, Name: "AR-Assessments", AccountType: fin.GLAccountTypeAsset,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	glRepo.accounts[cashAccount.ID] = cashAccount
	glRepo.accounts[arAccount.ID] = arAccount

	svc := fin.NewFinService(assessments, payments, budgets, funds, collections, glService, fin.NewGaapEngine(), ai.NewNoopPolicyResolver(), ai.NewNoopComplianceResolver(), nil, logger, nil)

	unitID := uuid.New()
	userID := uuid.New()
	req := fin.CreatePaymentRequest{
		UnitID:      unitID,
		AmountCents: 15000,
	}

	payment, err := svc.RecordPayment(ctx, orgID, userID, req)
	require.NoError(t, err)
	require.NotNil(t, payment)

	// Verify exactly 1 GL journal entry was posted.
	require.Len(t, glRepo.entries, 1)
	for _, entry := range glRepo.entries {
		require.Len(t, entry.Lines, 2)
		// Line 0: Debit Cash
		assert.Equal(t, cashAccount.ID, entry.Lines[0].AccountID)
		assert.Equal(t, int64(15000), entry.Lines[0].DebitCents)
		assert.Equal(t, int64(0), entry.Lines[0].CreditCents)
		// Line 1: Credit AR
		assert.Equal(t, arAccount.ID, entry.Lines[1].AccountID)
		assert.Equal(t, int64(0), entry.Lines[1].DebitCents)
		assert.Equal(t, int64(15000), entry.Lines[1].CreditCents)
	}
}

// TestCreateAssessment_NilGLService_StillWorks verifies that when GLService is
// nil, CreateAssessment still succeeds (backward compatibility).
func TestCreateAssessment_NilGLService_StillWorks(t *testing.T) {
	svc, _, _, _, _, _ := newTestService()
	ctx := context.Background()
	orgID := uuid.New()
	unitID := uuid.New()

	req := fin.CreateAssessmentRequest{
		UnitID:      unitID,
		Description: "Monthly HOA fee",
		AmountCents: 15000,
		DueDate:     time.Now().AddDate(0, 1, 0),
	}

	assessment, err := svc.CreateAssessment(ctx, orgID, req)
	require.NoError(t, err)
	require.NotNil(t, assessment)
	assert.Equal(t, int64(15000), assessment.AmountCents)
	assert.Equal(t, unitID, assessment.UnitID)
}

// ── CurrencyCode Tests ──────────────────────────────────────────────────────

// TestRecordPayment_GLFailureReturnsError verifies that a GL posting error
// propagates to the caller instead of being silently swallowed.
func TestRecordPayment_GLFailureReturnsError(t *testing.T) {
	assessments := &mockAssessmentRepo{}
	payments := &mockPaymentRepo{}
	budgets := &mockBudgetRepo{}
	funds := &mockFundRepo{}
	collections := &mockCollectionRepo{}

	glRepo := newMockGLRepo()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	glService := fin.NewGLService(glRepo, audit.NewNoopAuditor(), logger)

	orgID := uuid.New()
	userID := uuid.New()

	cashAccount := &fin.GLAccount{
		ID: uuid.New(), OrgID: orgID, AccountNumber: 1010, Name: "Cash",
		AccountType: fin.GLAccountTypeAsset, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	arAccount := &fin.GLAccount{
		ID: uuid.New(), OrgID: orgID, AccountNumber: 1100, Name: "AR",
		AccountType: fin.GLAccountTypeAsset, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	glRepo.SetAccounts(cashAccount, arAccount)
	glRepo.SetPostError(fmt.Errorf("simulated GL failure"))

	svc := fin.NewFinService(assessments, payments, budgets, funds, collections, glService, fin.NewGaapEngine(), ai.NewNoopPolicyResolver(), ai.NewNoopComplianceResolver(), nil, logger, nil)

	desc := "Test payment"
	_, err := svc.RecordPayment(context.Background(), orgID, userID, fin.CreatePaymentRequest{
		UnitID:      uuid.New(),
		AmountCents: 5000,
		Description: &desc,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "simulated GL failure")
}

// TestCreateFundTransfer_GLFailureReturnsError verifies that a GL posting error
// propagates to the caller instead of being silently swallowed.
func TestCreateFundTransfer_GLFailureReturnsError(t *testing.T) {
	assessments := &mockAssessmentRepo{}
	payments := &mockPaymentRepo{}
	budgets := &mockBudgetRepo{}
	funds := &mockFundRepo{}
	collections := &mockCollectionRepo{}

	glRepo := newMockGLRepo()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	glService := fin.NewGLService(glRepo, audit.NewNoopAuditor(), logger)

	orgID := uuid.New()

	fromFund := &fin.Fund{OrgID: orgID, CurrencyCode: "USD", Name: "Operating", FundType: "operating", BalanceCents: 100000}
	toFund := &fin.Fund{OrgID: orgID, CurrencyCode: "USD", Name: "Reserve", FundType: "reserve", BalanceCents: 0}
	fromFund, _ = funds.CreateFund(context.Background(), fromFund)
	toFund, _ = funds.CreateFund(context.Background(), toFund)

	fromCash := &fin.GLAccount{ID: uuid.New(), OrgID: orgID, AccountNumber: 1010, Name: "Cash-Operating", AccountType: fin.GLAccountTypeAsset, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	toCash := &fin.GLAccount{ID: uuid.New(), OrgID: orgID, AccountNumber: 1020, Name: "Cash-Reserve", AccountType: fin.GLAccountTypeAsset, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	transferOut := &fin.GLAccount{ID: uuid.New(), OrgID: orgID, AccountNumber: 3100, Name: "Transfer Out", AccountType: fin.GLAccountTypeEquity, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	transferIn := &fin.GLAccount{ID: uuid.New(), OrgID: orgID, AccountNumber: 3110, Name: "Transfer In", AccountType: fin.GLAccountTypeEquity, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	glRepo.SetAccounts(fromCash, toCash, transferOut, transferIn)
	glRepo.SetPostError(fmt.Errorf("simulated GL failure"))

	svc := fin.NewFinService(assessments, payments, budgets, funds, collections, glService, fin.NewGaapEngine(), ai.NewNoopPolicyResolver(), ai.NewNoopComplianceResolver(), nil, logger, nil)

	desc := "Test transfer"
	_, err := svc.CreateFundTransfer(context.Background(), orgID, fin.CreateFundTransferRequest{
		FromFundID:  fromFund.ID,
		ToFundID:    toFund.ID,
		AmountCents: 5000,
		Description: &desc,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "simulated GL failure")
}

// TestCreateSchedule_SetsCurrencyCode verifies that CurrencyCode is explicitly
// set to "USD" when creating a schedule.
func TestCreateSchedule_SetsCurrencyCode(t *testing.T) {
	svc, _, _, _, _, _ := newTestService()
	ctx := context.Background()
	orgID := uuid.New()

	req := fin.CreateAssessmentScheduleRequest{
		Name:            "Monthly HOA",
		Frequency:       "monthly",
		AmountStrategy:  "flat",
		BaseAmountCents: 15000,
		StartsAt:        time.Now(),
	}

	schedule, err := svc.CreateSchedule(ctx, orgID, req)
	require.NoError(t, err)
	assert.Equal(t, "USD", schedule.CurrencyCode)
}

// TestCreateAssessment_SetsCurrencyCode verifies that CurrencyCode is explicitly
// set to "USD" on both the assessment and the charge ledger entry.
func TestCreateAssessment_SetsCurrencyCode(t *testing.T) {
	svc, assessmentRepo, _, _, _, _ := newTestService()
	ctx := context.Background()
	orgID := uuid.New()
	unitID := uuid.New()

	req := fin.CreateAssessmentRequest{
		UnitID:      unitID,
		Description: "Monthly HOA fee",
		AmountCents: 15000,
		DueDate:     time.Now().AddDate(0, 1, 0),
	}

	assessment, err := svc.CreateAssessment(ctx, orgID, req)
	require.NoError(t, err)
	assert.Equal(t, "USD", assessment.CurrencyCode)

	// Verify ledger entry also has CurrencyCode set.
	require.Len(t, assessmentRepo.ledger, 1)
	assert.Equal(t, "USD", assessmentRepo.ledger[0].CurrencyCode)
}

// TestRecordPayment_SetsCurrencyCode verifies that CurrencyCode is explicitly
// set to "USD" on both the payment and the payment ledger entry.
func TestRecordPayment_SetsCurrencyCode(t *testing.T) {
	svc, assessmentRepo, _, _, _, _ := newTestService()
	ctx := context.Background()
	orgID := uuid.New()
	unitID := uuid.New()
	userID := uuid.New()

	req := fin.CreatePaymentRequest{
		UnitID:      unitID,
		AmountCents: 15000,
	}

	payment, err := svc.RecordPayment(ctx, orgID, userID, req)
	require.NoError(t, err)
	assert.Equal(t, "USD", payment.CurrencyCode)

	// Verify ledger entry also has CurrencyCode set.
	require.Len(t, assessmentRepo.ledger, 1)
	assert.Equal(t, "USD", assessmentRepo.ledger[0].CurrencyCode)
}

// TestCreateExpense_SetsCurrencyCode verifies that CurrencyCode is explicitly
// set to "USD" when creating an expense.
func TestCreateExpense_SetsCurrencyCode(t *testing.T) {
	svc, _, _, _, _, _ := newTestService()
	ctx := context.Background()
	orgID := uuid.New()
	userID := uuid.New()

	req := fin.CreateExpenseRequest{
		Description: "Landscaping",
		AmountCents: 50000,
		ExpenseDate: time.Now(),
	}

	expense, err := svc.CreateExpense(ctx, orgID, userID, req)
	require.NoError(t, err)
	assert.Equal(t, "USD", expense.CurrencyCode)
}

// TestCreateFund_SetsCurrencyCode verifies that CurrencyCode is explicitly
// set to "USD" when creating a fund.
func TestCreateFund_SetsCurrencyCode(t *testing.T) {
	svc, _, _, _, _, _ := newTestService()
	ctx := context.Background()
	orgID := uuid.New()

	req := fin.CreateFundRequest{
		Name:     "Operating Fund",
		FundType: "operating",
	}

	fund, err := svc.CreateFund(ctx, orgID, req)
	require.NoError(t, err)
	assert.Equal(t, "USD", fund.CurrencyCode)
}

// TestCreateFundTransfer_SetsCurrencyCode verifies that CurrencyCode is explicitly
// set to "USD" when creating a fund transfer.
func TestCreateFundTransfer_SetsCurrencyCode(t *testing.T) {
	svc, _, _, _, fundRepo, _ := newTestService()
	ctx := context.Background()
	orgID := uuid.New()

	// Create two funds to transfer between.
	from, err := svc.CreateFund(ctx, orgID, fin.CreateFundRequest{Name: "From Fund", FundType: "operating"})
	require.NoError(t, err)
	to, err := svc.CreateFund(ctx, orgID, fin.CreateFundRequest{Name: "To Fund", FundType: "reserve"})
	require.NoError(t, err)

	req := fin.CreateFundTransferRequest{
		FromFundID:  from.ID,
		ToFundID:    to.ID,
		AmountCents: 5000,
	}

	transfer, err := svc.CreateFundTransfer(ctx, orgID, req)
	require.NoError(t, err)
	assert.Equal(t, "USD", transfer.CurrencyCode)
	assert.Len(t, fundRepo.transfers, 1)
}

// TestCreateFundTransfer_UpdatesFundBalances verifies that a transfer debits the
// source fund and credits the destination fund.
func TestCreateFundTransfer_UpdatesFundBalances(t *testing.T) {
	svc, _, _, _, fundRepo, _ := newTestService()
	ctx := context.Background()
	orgID := uuid.New()

	from, err := svc.CreateFund(ctx, orgID, fin.CreateFundRequest{Name: "Operating", FundType: "operating"})
	require.NoError(t, err)
	to, err := svc.CreateFund(ctx, orgID, fin.CreateFundRequest{Name: "Reserve", FundType: "reserve"})
	require.NoError(t, err)

	// Seed the source fund with a starting balance.
	for i := range fundRepo.funds {
		if fundRepo.funds[i].ID == from.ID {
			fundRepo.funds[i].BalanceCents = 100_000
		}
	}

	_, err = svc.CreateFundTransfer(ctx, orgID, fin.CreateFundTransferRequest{
		FromFundID:  from.ID,
		ToFundID:    to.ID,
		AmountCents: 25_000,
	})
	require.NoError(t, err)

	// Verify source fund was debited.
	updatedFrom, err := svc.GetFund(ctx, from.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(75_000), updatedFrom.BalanceCents, "source fund should be debited")

	// Verify destination fund was credited.
	updatedTo, err := svc.GetFund(ctx, to.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(25_000), updatedTo.BalanceCents, "destination fund should be credited")
}

// TestCreateFundTransfer_CreatesFundTransactions verifies that a transfer creates
// a debit transaction on the source fund and a credit transaction on the destination.
func TestCreateFundTransfer_CreatesFundTransactions(t *testing.T) {
	svc, _, _, _, fundRepo, _ := newTestService()
	ctx := context.Background()
	orgID := uuid.New()

	from, err := svc.CreateFund(ctx, orgID, fin.CreateFundRequest{Name: "Operating", FundType: "operating"})
	require.NoError(t, err)
	to, err := svc.CreateFund(ctx, orgID, fin.CreateFundRequest{Name: "Reserve", FundType: "reserve"})
	require.NoError(t, err)

	// Seed starting balance so debit succeeds.
	for i := range fundRepo.funds {
		if fundRepo.funds[i].ID == from.ID {
			fundRepo.funds[i].BalanceCents = 50_000
		}
	}

	transfer, err := svc.CreateFundTransfer(ctx, orgID, fin.CreateFundTransferRequest{
		FromFundID:  from.ID,
		ToFundID:    to.ID,
		AmountCents: 20_000,
	})
	require.NoError(t, err)

	require.Len(t, fundRepo.transactions, 2, "should create exactly 2 fund transactions")

	debit := fundRepo.transactions[0]
	assert.Equal(t, from.ID, debit.FundID)
	assert.Equal(t, int64(-20_000), debit.AmountCents, "debit should be negative")
	assert.Equal(t, fin.FundTxTypeTransferOut, debit.TransactionType)
	refType := fin.FundTxRefTypeTransfer
	assert.Equal(t, &refType, debit.ReferenceType)
	assert.Equal(t, &transfer.ID, debit.ReferenceID)

	credit := fundRepo.transactions[1]
	assert.Equal(t, to.ID, credit.FundID)
	assert.Equal(t, int64(20_000), credit.AmountCents, "credit should be positive")
	assert.Equal(t, fin.FundTxTypeTransferIn, credit.TransactionType)
	assert.Equal(t, &refType, credit.ReferenceType)
	assert.Equal(t, &transfer.ID, credit.ReferenceID)
}

// ── Budget Line Item Recalculation Tests ─────────────────────────────────────

func TestCreateLineItem_RecalculatesTotals(t *testing.T) {
	budgetRepo := &mockBudgetRepo{}
	svc := fin.NewFinService(nil, nil, budgetRepo, nil, nil, nil, nil, nil, nil, nil, testutil.DiscardLogger(), nil)

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
	budget, err := svc.CreateBudget(ctx, orgID, userID, fin.CreateBudgetRequest{
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

func TestUpdateLineItem_RecalculatesTotals(t *testing.T) {
	budgetRepo := &mockBudgetRepo{}
	svc := fin.NewFinService(nil, nil, budgetRepo, nil, nil, nil, nil, nil, nil, nil, testutil.DiscardLogger(), nil)

	ctx := context.Background()
	orgID := testutil.TestOrgID()
	userID := testutil.TestUserID()

	expenseCat, err := budgetRepo.CreateCategory(ctx, &fin.BudgetCategory{
		OrgID:        orgID,
		Name:         "Maintenance",
		CategoryType: fin.BudgetCategoryTypeExpense,
	})
	require.NoError(t, err)

	budget, err := svc.CreateBudget(ctx, orgID, userID, fin.CreateBudgetRequest{
		FiscalYear: 2026,
		Name:       "FY2026 Budget",
	})
	require.NoError(t, err)

	item, err := svc.CreateLineItem(ctx, budget.ID, &fin.BudgetLineItem{
		CategoryID:   expenseCat.ID,
		PlannedCents: 50000,
	})
	require.NoError(t, err)

	// Update planned amount from 500.00 to 750.00
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

func TestDeleteLineItem_RecalculatesTotals(t *testing.T) {
	budgetRepo := &mockBudgetRepo{}
	svc := fin.NewFinService(nil, nil, budgetRepo, nil, nil, nil, nil, nil, nil, nil, testutil.DiscardLogger(), nil)

	ctx := context.Background()
	orgID := testutil.TestOrgID()
	userID := testutil.TestUserID()

	expenseCat, err := budgetRepo.CreateCategory(ctx, &fin.BudgetCategory{
		OrgID:        orgID,
		Name:         "Maintenance",
		CategoryType: fin.BudgetCategoryTypeExpense,
	})
	require.NoError(t, err)

	budget, err := svc.CreateBudget(ctx, orgID, userID, fin.CreateBudgetRequest{
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

func TestDeleteLineItem_NotFound(t *testing.T) {
	budgetRepo := &mockBudgetRepo{}
	svc := fin.NewFinService(nil, nil, budgetRepo, nil, nil, nil, nil, nil, nil, nil, testutil.DiscardLogger(), nil)

	err := svc.DeleteLineItem(context.Background(), uuid.New())
	require.Error(t, err)

	var notFound *api.NotFoundError
	require.ErrorAs(t, err, &notFound)
}

func TestUpdateLineItem_NotFound(t *testing.T) {
	budgetRepo := &mockBudgetRepo{}
	svc := fin.NewFinService(nil, nil, budgetRepo, nil, nil, nil, nil, nil, nil, nil, testutil.DiscardLogger(), nil)

	_, err := svc.UpdateLineItem(context.Background(), uuid.New(), &fin.BudgetLineItem{
		PlannedCents: 10000,
	})
	require.Error(t, err)
}

// ── ReverseLedgerEntry Tests ────────────────────────────────────────────────

// TestReverseLedgerEntry verifies the happy-path reversal of a charge ledger
// entry: the reversal has the correct type, negated amount, reference back to
// the original, and the unit balance returns to zero.
func TestReverseLedgerEntry(t *testing.T) {
	svc, assessmentRepo, _, _, _, _ := newTestService()
	ctx := context.Background()
	orgID := uuid.New()
	unitID := uuid.New()
	reversedBy := uuid.New()

	// Create an assessment which also creates a charge ledger entry.
	req := fin.CreateAssessmentRequest{
		UnitID:      unitID,
		Description: "Monthly HOA fee",
		AmountCents: 15000,
		DueDate:     time.Now().AddDate(0, 1, 0),
	}
	_, err := svc.CreateAssessment(ctx, orgID, req)
	require.NoError(t, err)
	require.Len(t, assessmentRepo.ledger, 1)

	originalEntry := assessmentRepo.ledger[0]
	assert.Equal(t, int64(15000), originalEntry.BalanceCents)

	// Reverse the charge entry.
	reversal, err := svc.ReverseLedgerEntry(ctx, originalEntry.ID, reversedBy)
	require.NoError(t, err)
	require.NotNil(t, reversal)

	// Verify reversal fields.
	assert.Equal(t, fin.LedgerEntryTypeReversal, reversal.EntryType)
	assert.Equal(t, int64(-15000), reversal.AmountCents)
	assert.Equal(t, unitID, reversal.UnitID)
	assert.Equal(t, orgID, reversal.OrgID)

	// Verify reference points back to the original entry.
	require.NotNil(t, reversal.ReferenceType)
	assert.Equal(t, fin.LedgerRefTypeReversal, *reversal.ReferenceType)
	require.NotNil(t, reversal.ReferenceID)
	assert.Equal(t, originalEntry.ID, *reversal.ReferenceID)

	// Verify the original entry's ReversedByEntryID points to the reversal.
	updated := assessmentRepo.ledger[0]
	require.NotNil(t, updated.ReversedByEntryID)
	assert.Equal(t, reversal.ID, *updated.ReversedByEntryID)

	// Verify balance is back to zero.
	assert.Equal(t, int64(0), reversal.BalanceCents)
}

// TestReverseLedgerEntry_AlreadyReversed verifies that attempting to reverse a
// ledger entry that has already been reversed returns an error.
func TestReverseLedgerEntry_AlreadyReversed(t *testing.T) {
	svc, assessmentRepo, _, _, _, _ := newTestService()
	ctx := context.Background()
	orgID := uuid.New()
	unitID := uuid.New()
	reversedBy := uuid.New()

	// Create an assessment which also creates a charge ledger entry.
	req := fin.CreateAssessmentRequest{
		UnitID:      unitID,
		Description: "Monthly HOA fee",
		AmountCents: 15000,
		DueDate:     time.Now().AddDate(0, 1, 0),
	}
	_, err := svc.CreateAssessment(ctx, orgID, req)
	require.NoError(t, err)
	require.Len(t, assessmentRepo.ledger, 1)

	originalEntry := assessmentRepo.ledger[0]

	// First reversal should succeed.
	_, err = svc.ReverseLedgerEntry(ctx, originalEntry.ID, reversedBy)
	require.NoError(t, err)

	// Second reversal should fail.
	_, err = svc.ReverseLedgerEntry(ctx, originalEntry.ID, reversedBy)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already reversed")
}

// ── VoidAssessment Tests ────────────────────────────────────────────────────

// TestVoidAssessment verifies the happy-path void of a posted assessment:
// the assessment status becomes void, VoidedBy/VoidedAt are set, and the
// ledger contains both the original charge and a reversal entry.
func TestVoidAssessment(t *testing.T) {
	svc, assessmentRepo, _, _, _, _ := newTestService()
	ctx := context.Background()
	orgID := uuid.New()
	unitID := uuid.New()
	voidedBy := uuid.New()

	// Create an assessment (status=posted, charge ledger entry created).
	req := fin.CreateAssessmentRequest{
		UnitID:      unitID,
		Description: "Monthly HOA fee",
		AmountCents: 15000,
		DueDate:     time.Now().AddDate(0, 1, 0),
	}
	assessment, err := svc.CreateAssessment(ctx, orgID, req)
	require.NoError(t, err)
	assert.Equal(t, fin.AssessmentStatusPosted, assessment.Status)
	require.Len(t, assessmentRepo.ledger, 1)

	// Void the assessment.
	err = svc.VoidAssessment(ctx, assessment.ID, voidedBy)
	require.NoError(t, err)

	// Verify the assessment is now void with metadata set.
	voided, err := svc.GetAssessment(ctx, assessment.ID)
	require.NoError(t, err)
	assert.Equal(t, fin.AssessmentStatusVoid, voided.Status)
	require.NotNil(t, voided.VoidedBy)
	assert.Equal(t, voidedBy, *voided.VoidedBy)
	require.NotNil(t, voided.VoidedAt)

	// Verify ledger has 2 entries: original charge + reversal.
	require.Len(t, assessmentRepo.ledger, 2)
	reversal := assessmentRepo.ledger[1]
	assert.Equal(t, fin.LedgerEntryTypeReversal, reversal.EntryType)
	assert.Equal(t, int64(-15000), reversal.AmountCents)
	assert.Equal(t, int64(0), reversal.BalanceCents)
}

// TestVoidAssessment_BlockedByPayments verifies that voiding an assessment
// with linked payment-type ledger entries returns a validation error.
func TestVoidAssessment_BlockedByPayments(t *testing.T) {
	svc, assessmentRepo, _, _, _, _ := newTestService()
	ctx := context.Background()
	orgID := uuid.New()
	unitID := uuid.New()
	voidedBy := uuid.New()

	// Create an assessment (charge entry created).
	req := fin.CreateAssessmentRequest{
		UnitID:      unitID,
		Description: "Monthly HOA fee",
		AmountCents: 15000,
		DueDate:     time.Now().AddDate(0, 1, 0),
	}
	assessment, err := svc.CreateAssessment(ctx, orgID, req)
	require.NoError(t, err)

	// Manually add a payment-type ledger entry linked to this assessment.
	paymentEntry := fin.LedgerEntry{
		OrgID:        orgID,
		CurrencyCode: "USD",
		UnitID:       unitID,
		AssessmentID: &assessment.ID,
		EntryType:    fin.LedgerEntryTypePayment,
		AmountCents:  -15000,
	}
	_, err = assessmentRepo.CreateLedgerEntry(ctx, &paymentEntry)
	require.NoError(t, err)

	// Attempt to void should fail because of linked payment entries.
	err = svc.VoidAssessment(ctx, assessment.ID, voidedBy)
	require.Error(t, err)

	var valErr *api.ValidationError
	require.ErrorAs(t, err, &valErr)
	assert.Contains(t, valErr.MsgKey(), "has_payments")
}

// TestVoidAssessment_AlreadyVoid verifies that voiding an already-void
// assessment returns a validation error.
func TestVoidAssessment_AlreadyVoid(t *testing.T) {
	svc, assessmentRepo, _, _, _, _ := newTestService()
	ctx := context.Background()
	orgID := uuid.New()
	unitID := uuid.New()
	voidedBy := uuid.New()

	// Create an assessment and void it.
	req := fin.CreateAssessmentRequest{
		UnitID:      unitID,
		Description: "Monthly HOA fee",
		AmountCents: 15000,
		DueDate:     time.Now().AddDate(0, 1, 0),
	}
	assessment, err := svc.CreateAssessment(ctx, orgID, req)
	require.NoError(t, err)

	// Manually set the assessment status to void.
	for i := range assessmentRepo.assessments {
		if assessmentRepo.assessments[i].ID == assessment.ID {
			assessmentRepo.assessments[i].Status = fin.AssessmentStatusVoid
		}
	}

	// Attempt to void again should fail.
	err = svc.VoidAssessment(ctx, assessment.ID, voidedBy)
	require.Error(t, err)

	var valErr *api.ValidationError
	require.ErrorAs(t, err, &valErr)
	assert.Contains(t, valErr.MsgKey(), "already_void")
}
