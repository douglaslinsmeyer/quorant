package fin_test

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/audit"
	"github.com/quorant/quorant/internal/fin"
	"github.com/quorant/quorant/internal/platform/api"
	"github.com/quorant/quorant/internal/platform/queue"
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

func (m *mockAssessmentRepo) ListAssessmentsByOrg(_ context.Context, orgID uuid.UUID) ([]fin.Assessment, error) {
	var result []fin.Assessment
	for _, a := range m.assessments {
		if a.OrgID == orgID {
			result = append(result, a)
		}
	}
	if result == nil {
		return []fin.Assessment{}, nil
	}
	return result, nil
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

func (m *mockAssessmentRepo) ListLedgerByUnit(_ context.Context, unitID uuid.UUID) ([]fin.LedgerEntry, error) {
	var result []fin.LedgerEntry
	for _, e := range m.ledger {
		if e.UnitID == unitID {
			result = append(result, e)
		}
	}
	if result == nil {
		return []fin.LedgerEntry{}, nil
	}
	return result, nil
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

func (m *mockPaymentRepo) ListPaymentsByOrg(_ context.Context, orgID uuid.UUID) ([]fin.Payment, error) {
	var result []fin.Payment
	for _, p := range m.payments {
		if p.OrgID == orgID {
			result = append(result, p)
		}
	}
	if result == nil {
		return []fin.Payment{}, nil
	}
	return result, nil
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

func (m *mockPaymentRepo) UpdatePaymentStatus(_ context.Context, id uuid.UUID, status string, paidAt *time.Time) error {
	for i := range m.payments {
		if m.payments[i].ID == id {
			m.payments[i].Status = status
			m.payments[i].PaidAt = paidAt
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
	m.transactions = append(m.transactions, *t)
	out := m.transactions[len(m.transactions)-1]
	return &out, nil
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

// ── Helper ────────────────────────────────────────────────────────────────────

func newTestService() (*fin.FinService, *mockAssessmentRepo, *mockPaymentRepo, *mockBudgetRepo, *mockFundRepo, *mockCollectionRepo) {
	assessments := &mockAssessmentRepo{}
	payments := &mockPaymentRepo{}
	budgets := &mockBudgetRepo{}
	funds := &mockFundRepo{}
	collections := &mockCollectionRepo{}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	svc := fin.NewFinService(assessments, payments, budgets, funds, collections, audit.NewNoopAuditor(), queue.NewInMemoryPublisher(), logger)
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
	assert.Equal(t, "charge", entry.EntryType)
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
	assert.Equal(t, "completed", payment.Status)

	// Verify a ledger entry was created with negative amount (credit).
	require.Len(t, assessmentRepo.ledger, 1)
	entry := assessmentRepo.ledger[0]
	assert.Equal(t, "payment", entry.EntryType)
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
	assert.Equal(t, "draft", created.Status)

	proposed, err := svc.ProposeBudget(ctx, created.ID, userID)
	require.NoError(t, err)
	assert.Equal(t, "proposed", proposed.Status)
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
	assert.Equal(t, "approved", approved.Status)
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
	assert.Equal(t, "draft", created.Status)

	_, err = svc.ApproveBudget(ctx, created.ID, userID)
	require.Error(t, err)

	var valErr *api.ValidationError
	require.ErrorAs(t, err, &valErr)
}

// TestApproveExpense_SetsStatus verifies that ApproveExpense transitions from
// "pending" to "approved".
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
	assert.Equal(t, "pending", created.Status)

	approved, err := svc.ApproveExpense(ctx, created.ID, userID)
	require.NoError(t, err)
	assert.Equal(t, "approved", approved.Status)
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
	assert.Equal(t, "approved", approved.Status)

	paid, err := svc.PayExpense(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "paid", paid.Status)
	assert.NotNil(t, paid.PaidDate)
}

// TestPayExpense_RejectsWhenNotApproved ensures that a pending expense cannot be
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
	assert.Equal(t, "operating", fund.FundType)
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
