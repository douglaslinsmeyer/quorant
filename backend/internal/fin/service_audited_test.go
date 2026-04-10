package fin_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/quorant/quorant/internal/audit"
	"github.com/quorant/quorant/internal/fin"
	"github.com/quorant/quorant/internal/platform/middleware"
	"github.com/quorant/quorant/internal/platform/queue"
	"github.com/quorant/quorant/internal/platform/testutil"
)

// errForTest is a sentinel error for use in tests within this package.
var errForTest = errors.New("test error")

// stubService implements fin.Service with function callbacks for each mutation
// under test. Embedding fin.Service means any uncalled method will panic,
// which is acceptable for focused unit tests.
type stubService struct {
	fin.Service // embedded to satisfy the interface; panics if uncalled methods are invoked

	createAssessmentFn func(ctx context.Context, orgID uuid.UUID, req fin.CreateAssessmentRequest) (*fin.Assessment, error)
	getAssessmentFn    func(ctx context.Context, id uuid.UUID) (*fin.Assessment, error)
	deleteAssessmentFn func(ctx context.Context, id uuid.UUID) error
	recordPaymentFn    func(ctx context.Context, orgID uuid.UUID, userID uuid.UUID, req fin.CreatePaymentRequest) (*fin.Payment, error)
	approveBudgetFn    func(ctx context.Context, id uuid.UUID, approvedBy uuid.UUID) (*fin.Budget, error)
	deleteLineItemFn   func(ctx context.Context, id uuid.UUID) error
}

func (s *stubService) CreateAssessment(ctx context.Context, orgID uuid.UUID, req fin.CreateAssessmentRequest) (*fin.Assessment, error) {
	return s.createAssessmentFn(ctx, orgID, req)
}

func (s *stubService) GetAssessment(ctx context.Context, id uuid.UUID) (*fin.Assessment, error) {
	return s.getAssessmentFn(ctx, id)
}

func (s *stubService) DeleteAssessment(ctx context.Context, id uuid.UUID) error {
	return s.deleteAssessmentFn(ctx, id)
}

func (s *stubService) RecordPayment(ctx context.Context, orgID uuid.UUID, userID uuid.UUID, req fin.CreatePaymentRequest) (*fin.Payment, error) {
	return s.recordPaymentFn(ctx, orgID, userID, req)
}

func (s *stubService) ApproveBudget(ctx context.Context, id uuid.UUID, approvedBy uuid.UUID) (*fin.Budget, error) {
	return s.approveBudgetFn(ctx, id, approvedBy)
}

func (s *stubService) DeleteLineItem(ctx context.Context, id uuid.UUID) error {
	return s.deleteLineItemFn(ctx, id)
}

// newAuditedTestService constructs an AuditedFinService with recording test doubles.
func newAuditedTestService(inner fin.Service) (*fin.AuditedFinService, *testutil.RecordingAuditor, *queue.InMemoryPublisher) {
	auditor := testutil.NewRecordingAuditor()
	publisher := queue.NewInMemoryPublisher()
	svc := fin.NewAuditedFinService(inner, auditor, publisher, testutil.DiscardLogger())
	return svc, auditor, publisher
}

// ctxWithActor returns a context with unique user and org IDs set, plus those IDs
// for use in assertions.
func ctxWithActor() (context.Context, uuid.UUID, uuid.UUID) {
	userID := uuid.New()
	orgID := uuid.New()
	ctx := context.Background()
	ctx = middleware.WithUserID(ctx, userID)
	ctx = middleware.WithOrgID(ctx, orgID)
	return ctx, userID, orgID
}

// testContext returns a context with test user and org IDs.
func testContext() context.Context {
	ctx := context.Background()
	ctx = middleware.WithUserID(ctx, testutil.TestUserID())
	ctx = middleware.WithOrgID(ctx, testutil.TestOrgID())
	return ctx
}

func TestAudited_CreateAssessment(t *testing.T) {
	assessmentID := uuid.New()
	orgID := testutil.TestOrgID()
	now := time.Now()

	stub := &stubService{
		createAssessmentFn: func(_ context.Context, _ uuid.UUID, _ fin.CreateAssessmentRequest) (*fin.Assessment, error) {
			return &fin.Assessment{
				ID:          assessmentID,
				OrgID:       orgID,
				UnitID:      uuid.New(),
				Description: "Q1 dues",
				AmountCents: 50000,
				DueDate:     now,
				CreatedAt:   now,
				UpdatedAt:   now,
			}, nil
		},
	}

	auditor := testutil.NewRecordingAuditor()
	publisher := queue.NewInMemoryPublisher()
	svc := fin.NewAuditedFinService(stub, auditor, publisher, testutil.DiscardLogger())

	ctx := testContext()
	result, err := svc.CreateAssessment(ctx, orgID, fin.CreateAssessmentRequest{})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, assessmentID, result.ID)

	// Verify audit entry
	entries := auditor.Entries()
	require.Len(t, entries, 1)
	entry := entries[0]
	assert.Equal(t, "fin.assessment.created", entry.Action)
	assert.Equal(t, "assessment", entry.ResourceType)
	assert.Equal(t, assessmentID, entry.ResourceID)
	assert.Equal(t, "fin", entry.Module)
	assert.Equal(t, orgID, entry.OrgID)
	assert.Equal(t, testutil.TestUserID(), entry.ActorID)
	assert.NotEmpty(t, entry.AfterState)

	// Verify published event
	events := publisher.Events()
	require.Len(t, events, 1)
	evt := events[0]
	assert.Equal(t, "assessment.created", evt.EventType())
	assert.Equal(t, assessmentID, evt.AggregateID())
	assert.NotEmpty(t, evt.Payload())
}

func TestAudited_RecordPayment(t *testing.T) {
	paymentID := uuid.New()
	orgID := testutil.TestOrgID()
	userID := testutil.TestUserID()
	now := time.Now()

	stub := &stubService{
		recordPaymentFn: func(_ context.Context, _ uuid.UUID, _ uuid.UUID, _ fin.CreatePaymentRequest) (*fin.Payment, error) {
			return &fin.Payment{
				ID:          paymentID,
				OrgID:       orgID,
				UserID:      userID,
				UnitID:      uuid.New(),
				AmountCents: 25000,
				Status:      "completed",
				CreatedAt:   now,
				UpdatedAt:   now,
			}, nil
		},
	}

	auditor := testutil.NewRecordingAuditor()
	publisher := queue.NewInMemoryPublisher()
	svc := fin.NewAuditedFinService(stub, auditor, publisher, testutil.DiscardLogger())

	ctx := testContext()
	result, err := svc.RecordPayment(ctx, orgID, userID, fin.CreatePaymentRequest{})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, paymentID, result.ID)

	// Verify audit entry
	entries := auditor.Entries()
	require.Len(t, entries, 1)
	entry := entries[0]
	assert.Equal(t, "fin.payment.received", entry.Action)
	assert.Equal(t, "payment", entry.ResourceType)
	assert.Equal(t, paymentID, entry.ResourceID)
	assert.Equal(t, "fin", entry.Module)
	assert.Equal(t, orgID, entry.OrgID)
	assert.Equal(t, userID, entry.ActorID)

	// Verify published event
	events := publisher.Events()
	require.Len(t, events, 1)
	evt := events[0]
	assert.Equal(t, "payment.received", evt.EventType())
	assert.Equal(t, paymentID, evt.AggregateID())
}

func TestAudited_ApproveBudget(t *testing.T) {
	budgetID := uuid.New()
	orgID := testutil.TestOrgID()
	approvedBy := testutil.TestUserID()
	now := time.Now()

	stub := &stubService{
		approveBudgetFn: func(_ context.Context, _ uuid.UUID, _ uuid.UUID) (*fin.Budget, error) {
			return &fin.Budget{
				ID:         budgetID,
				OrgID:      orgID,
				FiscalYear: 2026,
				Name:       "Annual Budget 2026",
				Status:     "approved",
				ApprovedBy: &approvedBy,
				ApprovedAt: &now,
				CreatedBy:  uuid.New(),
				CreatedAt:  now,
				UpdatedAt:  now,
			}, nil
		},
	}

	auditor := testutil.NewRecordingAuditor()
	publisher := queue.NewInMemoryPublisher()
	svc := fin.NewAuditedFinService(stub, auditor, publisher, testutil.DiscardLogger())

	ctx := testContext()
	result, err := svc.ApproveBudget(ctx, budgetID, approvedBy)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, budgetID, result.ID)

	// Verify audit entry
	entries := auditor.Entries()
	require.Len(t, entries, 1)
	entry := entries[0]
	assert.Equal(t, "fin.budget.approved", entry.Action)
	assert.Equal(t, "budget", entry.ResourceType)
	assert.Equal(t, budgetID, entry.ResourceID)
	assert.Equal(t, "fin", entry.Module)
	assert.Equal(t, orgID, entry.OrgID)
	assert.Equal(t, approvedBy, entry.ActorID)

	// Verify published event
	events := publisher.Events()
	require.Len(t, events, 1)
	evt := events[0]
	assert.Equal(t, "budget.approved", evt.EventType())
	assert.Equal(t, budgetID, evt.AggregateID())
}

func TestAudited_InnerError_NoAuditOrEvent(t *testing.T) {
	stub := &stubService{
		createAssessmentFn: func(_ context.Context, _ uuid.UUID, _ fin.CreateAssessmentRequest) (*fin.Assessment, error) {
			return nil, errForTest
		},
	}

	auditor := testutil.NewRecordingAuditor()
	publisher := queue.NewInMemoryPublisher()
	svc := fin.NewAuditedFinService(stub, auditor, publisher, testutil.DiscardLogger())

	ctx := testContext()
	result, err := svc.CreateAssessment(ctx, testutil.TestOrgID(), fin.CreateAssessmentRequest{})
	require.Error(t, err)
	assert.ErrorIs(t, err, errForTest)
	assert.Nil(t, result)

	// No audit entry should be recorded
	assert.Empty(t, auditor.Entries())

	// No event should be published
	assert.Empty(t, publisher.Events())
}

func TestAudited_DeleteAssessment(t *testing.T) {
	assessmentID := uuid.New()
	orgID := testutil.TestOrgID()

	stub := &stubService{
		getAssessmentFn: func(_ context.Context, id uuid.UUID) (*fin.Assessment, error) {
			return &fin.Assessment{ID: id, OrgID: orgID, Description: "Test", AmountCents: 10000}, nil
		},
		deleteAssessmentFn: func(_ context.Context, _ uuid.UUID) error {
			return nil
		},
	}
	svc, aud, pub := newAuditedTestService(stub)
	ctx, userID, _ := ctxWithActor()

	err := svc.DeleteAssessment(ctx, assessmentID)
	require.NoError(t, err)

	entries := aud.Entries()
	require.Len(t, entries, 1)
	assert.Equal(t, "fin.assessment.deleted", entries[0].Action)
	assert.Equal(t, assessmentID, entries[0].ResourceID)
	assert.Equal(t, orgID, entries[0].OrgID)
	assert.Equal(t, userID, entries[0].ActorID)
	assert.NotNil(t, entries[0].BeforeState, "delete should record before state")
	assert.Nil(t, entries[0].AfterState, "delete should have nil after state")

	events := pub.Events()
	require.Len(t, events, 1)
	assert.Equal(t, "assessment.deleted", events[0].EventType())
}

func TestAudited_DeleteLineItem_UsesContextOrgID(t *testing.T) {
	lineItemID := uuid.New()

	stub := &stubService{
		deleteLineItemFn: func(_ context.Context, _ uuid.UUID) error {
			return nil
		},
	}
	svc, aud, pub := newAuditedTestService(stub)
	ctx, _, orgID := ctxWithActor()

	err := svc.DeleteLineItem(ctx, lineItemID)
	require.NoError(t, err)

	entries := aud.Entries()
	require.Len(t, entries, 1)
	assert.Equal(t, "fin.budget_line_item.deleted", entries[0].Action)
	assert.Equal(t, orgID, entries[0].OrgID, "should use org ID from context")

	events := pub.Events()
	require.Len(t, events, 1)
	assert.Equal(t, "budget_line_item.deleted", events[0].EventType())
}

// Ensure RecordingAuditor is the concrete type returned by NewRecordingAuditor.
var _ audit.Auditor = (*testutil.RecordingAuditor)(nil)
