# Fin Module Audit & Event Publishing Decorator — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Wire audit logging and domain event publishing into every mutation in the fin module using the decorator pattern (approach C), so `FinService` retains pure business logic and cross-cutting concerns live in a separate `AuditedFinService` that implements the same `Service` interface.

**Architecture:** `AuditedFinService` wraps any `fin.Service` implementation. It delegates all calls to the inner service, and after each successful mutation, records an audit entry and publishes a domain event. Read-only methods pass through with no overhead. The decorator is wired in `main.go` between `NewFinService` and the handlers.

**Tech Stack:** Go 1.25, `audit.Auditor` interface, `queue.Publisher` interface, `queue.NewBaseEvent`, `middleware.UserIDFromContext`

**Resolves:** GitHub issue #65

---

## File Map

| Action | File | Responsibility |
|--------|------|---------------|
| Create | `backend/internal/platform/testutil/recording_auditor.go` | Reusable test auditor that captures entries |
| Create | `backend/internal/fin/service_audited.go` | Decorator: `AuditedFinService` struct + constructor + all `Service` methods |
| Create | `backend/internal/fin/service_audited_test.go` | Tests verifying audit + event for every mutation category |
| Modify | `backend/internal/fin/service.go:32-74` | Remove `auditor` and `publisher` fields from `FinService`; update constructor |
| Modify | `backend/internal/fin/service_test.go:713-721` | Update `newTestService` to drop auditor/publisher args |
| Modify | `backend/internal/fin/service_test.go:1076` | Update GL-aware test service constructor call |
| Modify | `backend/internal/fin/service_test.go:1132` | Update GL-aware test service constructor call |
| Modify | `backend/internal/fin/handler_assessment_test.go:41-52` | Update `fin.NewFinService` call |
| Modify | `backend/internal/fin/handler_payment_test.go:41-52` | Update `fin.NewFinService` call |
| Modify | `backend/internal/fin/handler_budget_test.go:40-52` | Update `fin.NewFinService` call |
| Modify | `backend/internal/fin/handler_fund_test.go:38-49` | Update `fin.NewFinService` call |
| Modify | `backend/internal/fin/handler_collection_test.go:38-49` | Update `fin.NewFinService` call |
| Modify | `backend/cmd/quorant-api/main.go:256-270` | Wire `AuditedFinService` decorator around `FinService` |

---

### Task 1: Create RecordingAuditor test utility

**Files:**
- Create: `backend/internal/platform/testutil/recording_auditor.go`

This is a reusable test double that captures audit entries for assertions. It will be used by the decorator tests and potentially by other modules.

- [ ] **Step 1: Create RecordingAuditor**

```go
package testutil

import (
	"context"
	"sync"

	"github.com/quorant/quorant/internal/audit"
)

// RecordingAuditor captures audit entries in memory for test assertions.
type RecordingAuditor struct {
	mu      sync.Mutex
	entries []audit.AuditEntry
}

// NewRecordingAuditor creates a new RecordingAuditor with an empty entry slice.
func NewRecordingAuditor() *RecordingAuditor {
	return &RecordingAuditor{entries: make([]audit.AuditEntry, 0)}
}

// Record appends the entry to the internal slice.
func (a *RecordingAuditor) Record(_ context.Context, entry audit.AuditEntry) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.entries = append(a.entries, entry)
	return nil
}

// Entries returns a copy of all recorded audit entries.
func (a *RecordingAuditor) Entries() []audit.AuditEntry {
	a.mu.Lock()
	defer a.mu.Unlock()
	cp := make([]audit.AuditEntry, len(a.entries))
	copy(cp, a.entries)
	return cp
}

// Reset clears all recorded entries.
func (a *RecordingAuditor) Reset() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.entries = a.entries[:0]
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd backend && go build ./internal/platform/testutil/...`
Expected: clean build, no errors

- [ ] **Step 3: Commit**

```bash
git add backend/internal/platform/testutil/recording_auditor.go
git commit -m "feat(testutil): add RecordingAuditor for capturing audit entries in tests"
```

---

### Task 2: Remove auditor and publisher from FinService

**Files:**
- Modify: `backend/internal/fin/service.go`

Strip the `auditor` and `publisher` fields from `FinService` and remove them from the constructor. These dependencies will live in the decorator instead.

- [ ] **Step 1: Update FinService struct and constructor**

In `backend/internal/fin/service.go`, change the struct definition to remove `auditor` and `publisher`:

```go
// FinService orchestrates all financial operations for the Finance module.
// It is the business logic layer between HTTP handlers and repositories.
type FinService struct {
	assessments AssessmentRepository
	payments    PaymentRepository
	budgets     BudgetRepository
	funds       FundRepository
	collections CollectionRepository
	gl          *GLService
	policy      ai.PolicyResolver
	compliance  ai.ComplianceResolver
	logger      *slog.Logger
}

// NewFinService creates a new FinService with the given repositories and logger.
func NewFinService(
	assessments AssessmentRepository,
	payments PaymentRepository,
	budgets BudgetRepository,
	funds FundRepository,
	collections CollectionRepository,
	gl *GLService,
	policy ai.PolicyResolver,
	compliance ai.ComplianceResolver,
	logger *slog.Logger,
) *FinService {
	return &FinService{
		assessments: assessments,
		payments:    payments,
		budgets:     budgets,
		funds:       funds,
		collections: collections,
		gl:          gl,
		policy:      policy,
		compliance:  compliance,
		logger:      logger,
	}
}
```

Also remove the now-unused imports from `service.go`: `"github.com/quorant/quorant/internal/audit"` and `"github.com/quorant/quorant/internal/platform/queue"`.

- [ ] **Step 2: Update all call sites in test files**

Every call to `fin.NewFinService` must drop the `auditor` and `publisher` arguments (positions 7 and 8 in the old signature). The following files need updating:

`backend/internal/fin/service_test.go` — `newTestService()` function (line 720):
```go
svc := fin.NewFinService(assessments, payments, budgets, funds, collections, nil, ai.NewNoopPolicyResolver(), ai.NewNoopComplianceResolver(), logger)
```

`backend/internal/fin/service_test.go` — GL-aware constructor at line 1076:
```go
svc := fin.NewFinService(assessments, payments, budgets, funds, collections, glService, ai.NewNoopPolicyResolver(), ai.NewNoopComplianceResolver(), logger)
```

`backend/internal/fin/service_test.go` — GL-aware constructor at line 1132:
```go
svc := fin.NewFinService(assessments, payments, budgets, funds, collections, glService, ai.NewNoopPolicyResolver(), ai.NewNoopComplianceResolver(), logger)
```

`backend/internal/fin/handler_assessment_test.go` — `setupAssessmentTestServer`:
```go
service := fin.NewFinService(
    mockAssessRepo,
    mockPaymentRepo,
    mockBudgetRepo,
    mockFundRepo,
    mockCollectionRepo,
    nil,
    ai.NewNoopPolicyResolver(),
    ai.NewNoopComplianceResolver(),
    logger,
)
```

`backend/internal/fin/handler_payment_test.go` — `setupPaymentTestServer`: same pattern as above.

`backend/internal/fin/handler_budget_test.go` — `setupBudgetTestServer`: same pattern as above.

`backend/internal/fin/handler_fund_test.go` — `setupFundTestServer`: same pattern as above.

`backend/internal/fin/handler_collection_test.go` — `setupCollectionTestServer`: same pattern as above.

Remove unused `audit` and `queue` imports from each test file where they are no longer referenced.

- [ ] **Step 3: Update main.go wiring**

In `backend/cmd/quorant-api/main.go`, line 263, drop `auditor` and `outboxPublisher` from the `fin.NewFinService` call:
```go
finService := fin.NewFinService(assessmentRepo, paymentRepo, budgetRepo, fundRepo, collectionRepo, glService, policyResolver, complianceService, logger)
```

(The decorator wiring happens in Task 4 after the decorator is created.)

- [ ] **Step 4: Verify everything compiles and tests pass**

Run: `cd backend && go build ./... && go test ./internal/fin/... -short -count=1`
Expected: clean build, all existing tests pass

- [ ] **Step 5: Commit**

```bash
git add backend/internal/fin/service.go backend/internal/fin/service_test.go \
  backend/internal/fin/handler_assessment_test.go backend/internal/fin/handler_payment_test.go \
  backend/internal/fin/handler_budget_test.go backend/internal/fin/handler_fund_test.go \
  backend/internal/fin/handler_collection_test.go backend/cmd/quorant-api/main.go
git commit -m "refactor(fin): remove auditor and publisher from FinService

These cross-cutting concerns will move to a decorator that wraps the
Service interface, keeping FinService focused on business logic only.

Resolves first half of #65"
```

---

### Task 3: Create AuditedFinService decorator

**Files:**
- Create: `backend/internal/fin/service_audited.go`

The decorator implements `Service` by delegating to an inner `Service`. After each successful mutation it records an audit entry and publishes a domain event. Read-only methods (all `List*`, `Get*`, `CheckReconciliation`) pass through directly.

- [ ] **Step 1: Write the decorator test file with tests for assessment mutations**

Create `backend/internal/fin/service_audited_test.go`. This file uses a `stubService` that implements `fin.Service` with minimal canned responses. The tests verify that for each mutation: (a) the inner service method is called, (b) an audit entry is recorded with the correct action/module/resource, (c) a domain event is published with the correct type.

```go
package fin_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/quorant/quorant/internal/fin"
	"github.com/quorant/quorant/internal/platform/middleware"
	"github.com/quorant/quorant/internal/platform/queue"
	"github.com/quorant/quorant/internal/platform/testutil"
)

// stubService is a minimal fin.Service that returns canned data.
// Only methods under test need real implementations; others can panic.
type stubService struct {
	fin.Service // embed to satisfy interface; unused methods will panic
	// Callbacks let individual tests control returns.
	createScheduleFn      func(ctx context.Context, orgID uuid.UUID, req fin.CreateAssessmentScheduleRequest) (*fin.AssessmentSchedule, error)
	updateScheduleFn      func(ctx context.Context, id uuid.UUID, req fin.UpdateAssessmentScheduleRequest) (*fin.AssessmentSchedule, error)
	deactivateScheduleFn  func(ctx context.Context, id uuid.UUID) error
	createAssessmentFn    func(ctx context.Context, orgID uuid.UUID, req fin.CreateAssessmentRequest) (*fin.Assessment, error)
	updateAssessmentFn    func(ctx context.Context, id uuid.UUID, a *fin.Assessment) (*fin.Assessment, error)
	deleteAssessmentFn    func(ctx context.Context, id uuid.UUID) error
	getAssessmentFn       func(ctx context.Context, id uuid.UUID) (*fin.Assessment, error)
	recordPaymentFn       func(ctx context.Context, orgID uuid.UUID, userID uuid.UUID, req fin.CreatePaymentRequest) (*fin.Payment, error)
	addPaymentMethodFn    func(ctx context.Context, orgID uuid.UUID, m *fin.PaymentMethod) (*fin.PaymentMethod, error)
	removePaymentMethodFn func(ctx context.Context, id uuid.UUID) error
	createBudgetFn        func(ctx context.Context, orgID uuid.UUID, createdBy uuid.UUID, req fin.CreateBudgetRequest) (*fin.Budget, error)
	updateBudgetFn        func(ctx context.Context, id uuid.UUID, req fin.UpdateBudgetRequest) (*fin.Budget, error)
	proposeBudgetFn       func(ctx context.Context, id uuid.UUID, proposedBy uuid.UUID) (*fin.Budget, error)
	approveBudgetFn       func(ctx context.Context, id uuid.UUID, approvedBy uuid.UUID) (*fin.Budget, error)
	createCategoryFn      func(ctx context.Context, orgID uuid.UUID, c *fin.BudgetCategory) (*fin.BudgetCategory, error)
	updateCategoryFn      func(ctx context.Context, id uuid.UUID, c *fin.BudgetCategory) (*fin.BudgetCategory, error)
	createLineItemFn      func(ctx context.Context, budgetID uuid.UUID, item *fin.BudgetLineItem) (*fin.BudgetLineItem, error)
	updateLineItemFn      func(ctx context.Context, id uuid.UUID, item *fin.BudgetLineItem) (*fin.BudgetLineItem, error)
	deleteLineItemFn      func(ctx context.Context, id uuid.UUID) error
	createExpenseFn       func(ctx context.Context, orgID uuid.UUID, submittedBy uuid.UUID, req fin.CreateExpenseRequest) (*fin.Expense, error)
	updateExpenseFn       func(ctx context.Context, id uuid.UUID, e *fin.Expense) (*fin.Expense, error)
	approveExpenseFn      func(ctx context.Context, id uuid.UUID, approvedBy uuid.UUID) (*fin.Expense, error)
	payExpenseFn          func(ctx context.Context, id uuid.UUID) (*fin.Expense, error)
	createFundFn          func(ctx context.Context, orgID uuid.UUID, req fin.CreateFundRequest) (*fin.Fund, error)
	updateFundFn          func(ctx context.Context, id uuid.UUID, f *fin.Fund) (*fin.Fund, error)
	createFundTransferFn  func(ctx context.Context, orgID uuid.UUID, req fin.CreateFundTransferRequest) (*fin.FundTransfer, error)
	updateCollectionFn    func(ctx context.Context, id uuid.UUID, c *fin.CollectionCase) (*fin.CollectionCase, error)
	addCollectionActionFn func(ctx context.Context, caseID uuid.UUID, req fin.CreateCollectionActionRequest) (*fin.CollectionAction, error)
	createPaymentPlanFn   func(ctx context.Context, caseID uuid.UUID, orgID uuid.UUID, unitID uuid.UUID, req fin.CreatePaymentPlanRequest) (*fin.PaymentPlan, error)
	updatePaymentPlanFn   func(ctx context.Context, id uuid.UUID, p *fin.PaymentPlan) (*fin.PaymentPlan, error)
}

func (s *stubService) CreateSchedule(ctx context.Context, orgID uuid.UUID, req fin.CreateAssessmentScheduleRequest) (*fin.AssessmentSchedule, error) {
	return s.createScheduleFn(ctx, orgID, req)
}
func (s *stubService) UpdateSchedule(ctx context.Context, id uuid.UUID, req fin.UpdateAssessmentScheduleRequest) (*fin.AssessmentSchedule, error) {
	return s.updateScheduleFn(ctx, id, req)
}
func (s *stubService) DeactivateSchedule(ctx context.Context, id uuid.UUID) error {
	return s.deactivateScheduleFn(ctx, id)
}
func (s *stubService) CreateAssessment(ctx context.Context, orgID uuid.UUID, req fin.CreateAssessmentRequest) (*fin.Assessment, error) {
	return s.createAssessmentFn(ctx, orgID, req)
}
func (s *stubService) GetAssessment(ctx context.Context, id uuid.UUID) (*fin.Assessment, error) {
	return s.getAssessmentFn(ctx, id)
}
func (s *stubService) UpdateAssessment(ctx context.Context, id uuid.UUID, a *fin.Assessment) (*fin.Assessment, error) {
	return s.updateAssessmentFn(ctx, id, a)
}
func (s *stubService) DeleteAssessment(ctx context.Context, id uuid.UUID) error {
	return s.deleteAssessmentFn(ctx, id)
}
func (s *stubService) RecordPayment(ctx context.Context, orgID uuid.UUID, userID uuid.UUID, req fin.CreatePaymentRequest) (*fin.Payment, error) {
	return s.recordPaymentFn(ctx, orgID, userID, req)
}
func (s *stubService) AddPaymentMethod(ctx context.Context, orgID uuid.UUID, m *fin.PaymentMethod) (*fin.PaymentMethod, error) {
	return s.addPaymentMethodFn(ctx, orgID, m)
}
func (s *stubService) RemovePaymentMethod(ctx context.Context, id uuid.UUID) error {
	return s.removePaymentMethodFn(ctx, id)
}
func (s *stubService) CreateBudget(ctx context.Context, orgID uuid.UUID, createdBy uuid.UUID, req fin.CreateBudgetRequest) (*fin.Budget, error) {
	return s.createBudgetFn(ctx, orgID, createdBy, req)
}
func (s *stubService) UpdateBudget(ctx context.Context, id uuid.UUID, req fin.UpdateBudgetRequest) (*fin.Budget, error) {
	return s.updateBudgetFn(ctx, id, req)
}
func (s *stubService) ProposeBudget(ctx context.Context, id uuid.UUID, proposedBy uuid.UUID) (*fin.Budget, error) {
	return s.proposeBudgetFn(ctx, id, proposedBy)
}
func (s *stubService) ApproveBudget(ctx context.Context, id uuid.UUID, approvedBy uuid.UUID) (*fin.Budget, error) {
	return s.approveBudgetFn(ctx, id, approvedBy)
}
func (s *stubService) CreateCategory(ctx context.Context, orgID uuid.UUID, c *fin.BudgetCategory) (*fin.BudgetCategory, error) {
	return s.createCategoryFn(ctx, orgID, c)
}
func (s *stubService) UpdateCategory(ctx context.Context, id uuid.UUID, c *fin.BudgetCategory) (*fin.BudgetCategory, error) {
	return s.updateCategoryFn(ctx, id, c)
}
func (s *stubService) CreateLineItem(ctx context.Context, budgetID uuid.UUID, item *fin.BudgetLineItem) (*fin.BudgetLineItem, error) {
	return s.createLineItemFn(ctx, budgetID, item)
}
func (s *stubService) UpdateLineItem(ctx context.Context, id uuid.UUID, item *fin.BudgetLineItem) (*fin.BudgetLineItem, error) {
	return s.updateLineItemFn(ctx, id, item)
}
func (s *stubService) DeleteLineItem(ctx context.Context, id uuid.UUID) error {
	return s.deleteLineItemFn(ctx, id)
}
func (s *stubService) CreateExpense(ctx context.Context, orgID uuid.UUID, submittedBy uuid.UUID, req fin.CreateExpenseRequest) (*fin.Expense, error) {
	return s.createExpenseFn(ctx, orgID, submittedBy, req)
}
func (s *stubService) UpdateExpense(ctx context.Context, id uuid.UUID, e *fin.Expense) (*fin.Expense, error) {
	return s.updateExpenseFn(ctx, id, e)
}
func (s *stubService) ApproveExpense(ctx context.Context, id uuid.UUID, approvedBy uuid.UUID) (*fin.Expense, error) {
	return s.approveExpenseFn(ctx, id, approvedBy)
}
func (s *stubService) PayExpense(ctx context.Context, id uuid.UUID) (*fin.Expense, error) {
	return s.payExpenseFn(ctx, id)
}
func (s *stubService) CreateFund(ctx context.Context, orgID uuid.UUID, req fin.CreateFundRequest) (*fin.Fund, error) {
	return s.createFundFn(ctx, orgID, req)
}
func (s *stubService) UpdateFund(ctx context.Context, id uuid.UUID, f *fin.Fund) (*fin.Fund, error) {
	return s.updateFundFn(ctx, id, f)
}
func (s *stubService) CreateFundTransfer(ctx context.Context, orgID uuid.UUID, req fin.CreateFundTransferRequest) (*fin.FundTransfer, error) {
	return s.createFundTransferFn(ctx, orgID, req)
}
func (s *stubService) UpdateCollection(ctx context.Context, id uuid.UUID, c *fin.CollectionCase) (*fin.CollectionCase, error) {
	return s.updateCollectionFn(ctx, id, c)
}
func (s *stubService) AddCollectionAction(ctx context.Context, caseID uuid.UUID, req fin.CreateCollectionActionRequest) (*fin.CollectionAction, error) {
	return s.addCollectionActionFn(ctx, caseID, req)
}
func (s *stubService) CreatePaymentPlan(ctx context.Context, caseID uuid.UUID, orgID uuid.UUID, unitID uuid.UUID, req fin.CreatePaymentPlanRequest) (*fin.PaymentPlan, error) {
	return s.createPaymentPlanFn(ctx, caseID, orgID, unitID, req)
}
func (s *stubService) UpdatePaymentPlan(ctx context.Context, id uuid.UUID, p *fin.PaymentPlan) (*fin.PaymentPlan, error) {
	return s.updatePaymentPlanFn(ctx, id, p)
}

// newAuditedTestService creates an AuditedFinService with a stubService inner,
// RecordingAuditor, and InMemoryPublisher for test assertions.
func newAuditedTestService(stub *stubService) (fin.Service, *testutil.RecordingAuditor, *queue.InMemoryPublisher) {
	aud := testutil.NewRecordingAuditor()
	pub := queue.NewInMemoryPublisher()
	logger := testutil.DiscardLogger()
	svc := fin.NewAuditedFinService(stub, aud, pub, logger)
	return svc, aud, pub
}

// ctxWithActor returns a context with a known user ID and org ID for audit assertions.
func ctxWithActor() (context.Context, uuid.UUID, uuid.UUID) {
	userID := testutil.TestUserID()
	orgID := testutil.TestOrgID()
	ctx := middleware.WithUserID(context.Background(), userID)
	ctx = middleware.WithOrgID(ctx, orgID)
	return ctx, userID, orgID
}

// ── Assessment Tests ─────────────────────────────────────────────────────────

func TestAudited_CreateAssessment(t *testing.T) {
	assessmentID := uuid.New()
	orgID := testutil.TestOrgID()

	stub := &stubService{
		createAssessmentFn: func(_ context.Context, _ uuid.UUID, _ fin.CreateAssessmentRequest) (*fin.Assessment, error) {
			return &fin.Assessment{ID: assessmentID, OrgID: orgID, Description: "Monthly HOA", AmountCents: 15000}, nil
		},
	}
	svc, aud, pub := newAuditedTestService(stub)
	ctx, userID, _ := ctxWithActor()

	req := fin.CreateAssessmentRequest{UnitID: uuid.New(), Description: "Monthly HOA", AmountCents: 15000, DueDate: time.Now()}
	result, err := svc.CreateAssessment(ctx, orgID, req)
	require.NoError(t, err)
	assert.Equal(t, assessmentID, result.ID)

	// Verify audit
	entries := aud.Entries()
	require.Len(t, entries, 1)
	assert.Equal(t, "fin.assessment.created", entries[0].Action)
	assert.Equal(t, "assessment", entries[0].ResourceType)
	assert.Equal(t, assessmentID, entries[0].ResourceID)
	assert.Equal(t, "fin", entries[0].Module)
	assert.Equal(t, orgID, entries[0].OrgID)
	assert.Equal(t, userID, entries[0].ActorID)

	// Verify event
	events := pub.Events()
	require.Len(t, events, 1)
	assert.Equal(t, "assessment.created", events[0].EventType())
	assert.Equal(t, assessmentID, events[0].AggregateID())
}

func TestAudited_RecordPayment(t *testing.T) {
	paymentID := uuid.New()
	orgID := testutil.TestOrgID()
	userID := testutil.TestUserID()

	stub := &stubService{
		recordPaymentFn: func(_ context.Context, _ uuid.UUID, _ uuid.UUID, _ fin.CreatePaymentRequest) (*fin.Payment, error) {
			return &fin.Payment{ID: paymentID, OrgID: orgID, UserID: userID, AmountCents: 15000}, nil
		},
	}
	svc, aud, pub := newAuditedTestService(stub)
	ctx, _, _ := ctxWithActor()

	req := fin.CreatePaymentRequest{UnitID: uuid.New(), AmountCents: 15000}
	result, err := svc.RecordPayment(ctx, orgID, userID, req)
	require.NoError(t, err)
	assert.Equal(t, paymentID, result.ID)

	entries := aud.Entries()
	require.Len(t, entries, 1)
	assert.Equal(t, "fin.payment.received", entries[0].Action)
	assert.Equal(t, "payment", entries[0].ResourceType)
	assert.Equal(t, paymentID, entries[0].ResourceID)

	events := pub.Events()
	require.Len(t, events, 1)
	assert.Equal(t, "payment.received", events[0].EventType())
}

func TestAudited_ApproveBudget(t *testing.T) {
	budgetID := uuid.New()
	orgID := testutil.TestOrgID()

	stub := &stubService{
		approveBudgetFn: func(_ context.Context, _ uuid.UUID, _ uuid.UUID) (*fin.Budget, error) {
			return &fin.Budget{ID: budgetID, OrgID: orgID, Status: "approved"}, nil
		},
	}
	svc, aud, pub := newAuditedTestService(stub)
	ctx, userID, _ := ctxWithActor()

	result, err := svc.ApproveBudget(ctx, budgetID, userID)
	require.NoError(t, err)
	assert.Equal(t, "approved", result.Status)

	entries := aud.Entries()
	require.Len(t, entries, 1)
	assert.Equal(t, "fin.budget.approved", entries[0].Action)
	assert.Equal(t, "budget", entries[0].ResourceType)
	assert.Equal(t, budgetID, entries[0].ResourceID)

	events := pub.Events()
	require.Len(t, events, 1)
	assert.Equal(t, "budget.approved", events[0].EventType())
}

func TestAudited_InnerError_NoAuditOrEvent(t *testing.T) {
	stub := &stubService{
		createAssessmentFn: func(_ context.Context, _ uuid.UUID, _ fin.CreateAssessmentRequest) (*fin.Assessment, error) {
			return nil, fin.ErrForTest
		},
	}
	svc, aud, pub := newAuditedTestService(stub)
	ctx, _, _ := ctxWithActor()

	_, err := svc.CreateAssessment(ctx, testutil.TestOrgID(), fin.CreateAssessmentRequest{})
	require.Error(t, err)

	assert.Empty(t, aud.Entries(), "audit should not record on error")
	assert.Empty(t, pub.Events(), "event should not publish on error")
}
```

Note: `fin.ErrForTest` is a sentinel error we'll export from the fin package for test usage. Add to `service_audited.go`:
```go
var ErrForTest = errors.New("test error")
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/fin/... -run TestAudited -short -count=1`
Expected: FAIL — `fin.NewAuditedFinService` and `fin.ErrForTest` not defined

- [ ] **Step 3: Implement the AuditedFinService decorator**

Create `backend/internal/fin/service_audited.go`:

```go
package fin

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/audit"
	"github.com/quorant/quorant/internal/platform/middleware"
	"github.com/quorant/quorant/internal/platform/queue"
)

// ErrForTest is a sentinel error exported for use in decorator tests.
var ErrForTest = errors.New("test error")

// AuditedFinService is a decorator that wraps a Service implementation with
// audit logging and domain event publishing. Read-only methods pass through
// directly; mutations record an audit entry and publish a domain event after
// successful delegation to the inner service.
type AuditedFinService struct {
	inner     Service
	auditor   audit.Auditor
	publisher queue.Publisher
	logger    *slog.Logger
}

// NewAuditedFinService creates a new AuditedFinService decorator.
func NewAuditedFinService(inner Service, auditor audit.Auditor, publisher queue.Publisher, logger *slog.Logger) *AuditedFinService {
	return &AuditedFinService{
		inner:     inner,
		auditor:   auditor,
		publisher: publisher,
		logger:    logger,
	}
}

// ── Internal helpers ─────────────────────────────────────────────────────────

func (s *AuditedFinService) record(ctx context.Context, action, resourceType string, resourceID, orgID uuid.UUID, after any) {
	var afterJSON json.RawMessage
	if after != nil {
		if data, err := json.Marshal(after); err == nil {
			afterJSON = data
		}
	}
	if err := s.auditor.Record(ctx, audit.AuditEntry{
		OrgID:        orgID,
		ActorID:      middleware.UserIDFromContext(ctx),
		Action:       "fin." + action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Module:       "fin",
		AfterState:   afterJSON,
		OccurredAt:   time.Now().UTC(),
	}); err != nil {
		s.logger.Error("audit record failed", "action", action, "resource_id", resourceID, "error", err)
	}
}

func (s *AuditedFinService) publish(ctx context.Context, eventType, aggregateType string, aggregateID, orgID uuid.UUID, payload any) {
	var data json.RawMessage
	if payload != nil {
		if b, err := json.Marshal(payload); err == nil {
			data = b
		}
	}
	event := queue.NewBaseEvent(eventType, aggregateType, aggregateID, orgID, data)
	if err := s.publisher.Publish(ctx, event); err != nil {
		s.logger.Error("event publish failed", "event_type", eventType, "aggregate_id", aggregateID, "error", err)
	}
}

func (s *AuditedFinService) emit(ctx context.Context, eventType, resourceType string, resourceID, orgID uuid.UUID, payload any) {
	s.record(ctx, eventType, resourceType, resourceID, orgID, payload)
	s.publish(ctx, eventType, resourceType, resourceID, orgID, payload)
}

// ── Read-only pass-throughs ──────────────────────────────────────────────────

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

// ── Assessment Schedule mutations ────────────────────────────────────────────

func (s *AuditedFinService) CreateSchedule(ctx context.Context, orgID uuid.UUID, req CreateAssessmentScheduleRequest) (*AssessmentSchedule, error) {
	result, err := s.inner.CreateSchedule(ctx, orgID, req)
	if err != nil {
		return nil, err
	}
	s.emit(ctx, "assessment_schedule.created", "assessment_schedule", result.ID, orgID, result)
	return result, nil
}

func (s *AuditedFinService) UpdateSchedule(ctx context.Context, id uuid.UUID, req UpdateAssessmentScheduleRequest) (*AssessmentSchedule, error) {
	result, err := s.inner.UpdateSchedule(ctx, id, req)
	if err != nil {
		return nil, err
	}
	s.emit(ctx, "assessment_schedule.updated", "assessment_schedule", result.ID, result.OrgID, result)
	return result, nil
}

func (s *AuditedFinService) DeactivateSchedule(ctx context.Context, id uuid.UUID) error {
	// Fetch before state for the audit record.
	schedule, _ := s.inner.GetSchedule(ctx, id)
	if err := s.inner.DeactivateSchedule(ctx, id); err != nil {
		return err
	}
	orgID := uuid.Nil
	if schedule != nil {
		orgID = schedule.OrgID
	}
	s.emit(ctx, "assessment_schedule.deactivated", "assessment_schedule", id, orgID, schedule)
	return nil
}

// ── Assessment mutations ─────────────────────────────────────────────────────

func (s *AuditedFinService) CreateAssessment(ctx context.Context, orgID uuid.UUID, req CreateAssessmentRequest) (*Assessment, error) {
	result, err := s.inner.CreateAssessment(ctx, orgID, req)
	if err != nil {
		return nil, err
	}
	s.emit(ctx, "assessment.created", "assessment", result.ID, orgID, result)
	return result, nil
}

func (s *AuditedFinService) UpdateAssessment(ctx context.Context, id uuid.UUID, a *Assessment) (*Assessment, error) {
	result, err := s.inner.UpdateAssessment(ctx, id, a)
	if err != nil {
		return nil, err
	}
	s.emit(ctx, "assessment.updated", "assessment", result.ID, result.OrgID, result)
	return result, nil
}

func (s *AuditedFinService) DeleteAssessment(ctx context.Context, id uuid.UUID) error {
	assessment, _ := s.inner.GetAssessment(ctx, id)
	if err := s.inner.DeleteAssessment(ctx, id); err != nil {
		return err
	}
	orgID := uuid.Nil
	if assessment != nil {
		orgID = assessment.OrgID
	}
	s.emit(ctx, "assessment.deleted", "assessment", id, orgID, assessment)
	return nil
}

// ── Payment mutations ────────────────────────────────────────────────────────

func (s *AuditedFinService) RecordPayment(ctx context.Context, orgID uuid.UUID, userID uuid.UUID, req CreatePaymentRequest) (*Payment, error) {
	result, err := s.inner.RecordPayment(ctx, orgID, userID, req)
	if err != nil {
		return nil, err
	}
	s.emit(ctx, "payment.received", "payment", result.ID, orgID, result)
	return result, nil
}

func (s *AuditedFinService) AddPaymentMethod(ctx context.Context, orgID uuid.UUID, m *PaymentMethod) (*PaymentMethod, error) {
	result, err := s.inner.AddPaymentMethod(ctx, orgID, m)
	if err != nil {
		return nil, err
	}
	s.emit(ctx, "payment_method.added", "payment_method", result.ID, orgID, result)
	return result, nil
}

func (s *AuditedFinService) RemovePaymentMethod(ctx context.Context, id uuid.UUID) error {
	if err := s.inner.RemovePaymentMethod(ctx, id); err != nil {
		return err
	}
	orgID := middleware.OrgIDFromContext(ctx)
	s.emit(ctx, "payment_method.removed", "payment_method", id, orgID, nil)
	return nil
}

// ── Budget mutations ─────────────────────────────────────────────────────────

func (s *AuditedFinService) CreateBudget(ctx context.Context, orgID uuid.UUID, createdBy uuid.UUID, req CreateBudgetRequest) (*Budget, error) {
	result, err := s.inner.CreateBudget(ctx, orgID, createdBy, req)
	if err != nil {
		return nil, err
	}
	s.emit(ctx, "budget.created", "budget", result.ID, orgID, result)
	return result, nil
}

func (s *AuditedFinService) UpdateBudget(ctx context.Context, id uuid.UUID, req UpdateBudgetRequest) (*Budget, error) {
	result, err := s.inner.UpdateBudget(ctx, id, req)
	if err != nil {
		return nil, err
	}
	s.emit(ctx, "budget.updated", "budget", result.ID, result.OrgID, result)
	return result, nil
}

func (s *AuditedFinService) ProposeBudget(ctx context.Context, id uuid.UUID, proposedBy uuid.UUID) (*Budget, error) {
	result, err := s.inner.ProposeBudget(ctx, id, proposedBy)
	if err != nil {
		return nil, err
	}
	s.emit(ctx, "budget.proposed", "budget", result.ID, result.OrgID, result)
	return result, nil
}

func (s *AuditedFinService) ApproveBudget(ctx context.Context, id uuid.UUID, approvedBy uuid.UUID) (*Budget, error) {
	result, err := s.inner.ApproveBudget(ctx, id, approvedBy)
	if err != nil {
		return nil, err
	}
	s.emit(ctx, "budget.approved", "budget", result.ID, result.OrgID, result)
	return result, nil
}

func (s *AuditedFinService) CreateCategory(ctx context.Context, orgID uuid.UUID, c *BudgetCategory) (*BudgetCategory, error) {
	result, err := s.inner.CreateCategory(ctx, orgID, c)
	if err != nil {
		return nil, err
	}
	s.emit(ctx, "budget_category.created", "budget_category", result.ID, orgID, result)
	return result, nil
}

func (s *AuditedFinService) UpdateCategory(ctx context.Context, id uuid.UUID, c *BudgetCategory) (*BudgetCategory, error) {
	result, err := s.inner.UpdateCategory(ctx, id, c)
	if err != nil {
		return nil, err
	}
	s.emit(ctx, "budget_category.updated", "budget_category", result.ID, result.OrgID, result)
	return result, nil
}

func (s *AuditedFinService) CreateLineItem(ctx context.Context, budgetID uuid.UUID, item *BudgetLineItem) (*BudgetLineItem, error) {
	result, err := s.inner.CreateLineItem(ctx, budgetID, item)
	if err != nil {
		return nil, err
	}
	orgID := middleware.OrgIDFromContext(ctx)
	s.emit(ctx, "budget_line_item.created", "budget_line_item", result.ID, orgID, result)
	return result, nil
}

func (s *AuditedFinService) UpdateLineItem(ctx context.Context, id uuid.UUID, item *BudgetLineItem) (*BudgetLineItem, error) {
	result, err := s.inner.UpdateLineItem(ctx, id, item)
	if err != nil {
		return nil, err
	}
	orgID := middleware.OrgIDFromContext(ctx)
	s.emit(ctx, "budget_line_item.updated", "budget_line_item", result.ID, orgID, result)
	return result, nil
}

func (s *AuditedFinService) DeleteLineItem(ctx context.Context, id uuid.UUID) error {
	if err := s.inner.DeleteLineItem(ctx, id); err != nil {
		return err
	}
	orgID := middleware.OrgIDFromContext(ctx)
	s.emit(ctx, "budget_line_item.deleted", "budget_line_item", id, orgID, nil)
	return nil
}

// ── Expense mutations ────────────────────────────────────────────────────────

func (s *AuditedFinService) CreateExpense(ctx context.Context, orgID uuid.UUID, submittedBy uuid.UUID, req CreateExpenseRequest) (*Expense, error) {
	result, err := s.inner.CreateExpense(ctx, orgID, submittedBy, req)
	if err != nil {
		return nil, err
	}
	s.emit(ctx, "expense.created", "expense", result.ID, orgID, result)
	return result, nil
}

func (s *AuditedFinService) UpdateExpense(ctx context.Context, id uuid.UUID, e *Expense) (*Expense, error) {
	result, err := s.inner.UpdateExpense(ctx, id, e)
	if err != nil {
		return nil, err
	}
	s.emit(ctx, "expense.updated", "expense", result.ID, result.OrgID, result)
	return result, nil
}

func (s *AuditedFinService) ApproveExpense(ctx context.Context, id uuid.UUID, approvedBy uuid.UUID) (*Expense, error) {
	result, err := s.inner.ApproveExpense(ctx, id, approvedBy)
	if err != nil {
		return nil, err
	}
	s.emit(ctx, "expense.approved", "expense", result.ID, result.OrgID, result)
	return result, nil
}

func (s *AuditedFinService) PayExpense(ctx context.Context, id uuid.UUID) (*Expense, error) {
	result, err := s.inner.PayExpense(ctx, id)
	if err != nil {
		return nil, err
	}
	s.emit(ctx, "expense.paid", "expense", result.ID, result.OrgID, result)
	return result, nil
}

// ── Fund mutations ───────────────────────────────────────────────────────────

func (s *AuditedFinService) CreateFund(ctx context.Context, orgID uuid.UUID, req CreateFundRequest) (*Fund, error) {
	result, err := s.inner.CreateFund(ctx, orgID, req)
	if err != nil {
		return nil, err
	}
	s.emit(ctx, "fund.created", "fund", result.ID, orgID, result)
	return result, nil
}

func (s *AuditedFinService) UpdateFund(ctx context.Context, id uuid.UUID, f *Fund) (*Fund, error) {
	result, err := s.inner.UpdateFund(ctx, id, f)
	if err != nil {
		return nil, err
	}
	s.emit(ctx, "fund.updated", "fund", result.ID, result.OrgID, result)
	return result, nil
}

func (s *AuditedFinService) CreateFundTransfer(ctx context.Context, orgID uuid.UUID, req CreateFundTransferRequest) (*FundTransfer, error) {
	result, err := s.inner.CreateFundTransfer(ctx, orgID, req)
	if err != nil {
		return nil, err
	}
	s.emit(ctx, "fund_transfer.created", "fund_transfer", result.ID, orgID, result)
	return result, nil
}

// ── Collection mutations ─────────────────────────────────────────────────────

func (s *AuditedFinService) UpdateCollection(ctx context.Context, id uuid.UUID, c *CollectionCase) (*CollectionCase, error) {
	result, err := s.inner.UpdateCollection(ctx, id, c)
	if err != nil {
		return nil, err
	}
	s.emit(ctx, "collection.updated", "collection_case", result.ID, result.OrgID, result)
	return result, nil
}

func (s *AuditedFinService) AddCollectionAction(ctx context.Context, caseID uuid.UUID, req CreateCollectionActionRequest) (*CollectionAction, error) {
	result, err := s.inner.AddCollectionAction(ctx, caseID, req)
	if err != nil {
		return nil, err
	}
	orgID := middleware.OrgIDFromContext(ctx)
	s.emit(ctx, "collection_action.added", "collection_action", result.ID, orgID, result)
	return result, nil
}

func (s *AuditedFinService) CreatePaymentPlan(ctx context.Context, caseID uuid.UUID, orgID uuid.UUID, unitID uuid.UUID, req CreatePaymentPlanRequest) (*PaymentPlan, error) {
	result, err := s.inner.CreatePaymentPlan(ctx, caseID, orgID, unitID, req)
	if err != nil {
		return nil, err
	}
	s.emit(ctx, "payment_plan.created", "payment_plan", result.ID, orgID, result)
	return result, nil
}

func (s *AuditedFinService) UpdatePaymentPlan(ctx context.Context, id uuid.UUID, p *PaymentPlan) (*PaymentPlan, error) {
	result, err := s.inner.UpdatePaymentPlan(ctx, id, p)
	if err != nil {
		return nil, err
	}
	s.emit(ctx, "payment_plan.updated", "payment_plan", result.ID, result.OrgID, result)
	return result, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd backend && go test ./internal/fin/... -run TestAudited -short -count=1 -v`
Expected: all 4 tests pass (CreateAssessment, RecordPayment, ApproveBudget, InnerError_NoAuditOrEvent)

- [ ] **Step 5: Commit**

```bash
git add backend/internal/fin/service_audited.go backend/internal/fin/service_audited_test.go
git commit -m "feat(fin): add AuditedFinService decorator for audit and event publishing

Implements the decorator pattern (approach C) — AuditedFinService wraps
any fin.Service, adding audit logging and domain event publishing after
every successful mutation. Read-only methods pass through with no overhead.

Resolves #65"
```

---

### Task 4: Wire decorator in main.go and run full test suite

**Files:**
- Modify: `backend/cmd/quorant-api/main.go:263-264`

- [ ] **Step 1: Wire the decorator**

In `backend/cmd/quorant-api/main.go`, after the `finService` line, wrap it with the decorator and pass the decorated version to handlers:

Change:
```go
finService := fin.NewFinService(assessmentRepo, paymentRepo, budgetRepo, fundRepo, collectionRepo, glService, policyResolver, complianceService, logger)
assessmentHandler := fin.NewAssessmentHandler(finService, logger)
```

To:
```go
finService := fin.NewFinService(assessmentRepo, paymentRepo, budgetRepo, fundRepo, collectionRepo, glService, policyResolver, complianceService, logger)
auditedFinService := fin.NewAuditedFinService(finService, auditor, outboxPublisher, logger)
assessmentHandler := fin.NewAssessmentHandler(auditedFinService, logger)
paymentHandler := fin.NewPaymentHandler(auditedFinService, logger)
budgetHandler := fin.NewBudgetHandler(auditedFinService, logger)
fundHandler := fin.NewFundHandler(auditedFinService, logger)
collectionHandler := fin.NewCollectionHandler(auditedFinService, logger)
```

Remove the existing lines that create handlers with `finService` directly (lines 264-268).

- [ ] **Step 2: Verify build**

Run: `cd backend && go build ./cmd/quorant-api && go build ./cmd/quorant-worker`
Expected: both binaries compile cleanly

- [ ] **Step 3: Run full test suite**

Run: `cd backend && go test ./... -short -count=1`
Expected: all tests pass

- [ ] **Step 4: Run linter**

Run: `make lint`
Expected: no lint errors

- [ ] **Step 5: Commit**

```bash
git add backend/cmd/quorant-api/main.go
git commit -m "feat(fin): wire AuditedFinService decorator in API server

All fin module handlers now receive the audited decorator instead of
the raw FinService, enabling audit logging and event publishing for
every financial mutation.

Closes #65"
```
