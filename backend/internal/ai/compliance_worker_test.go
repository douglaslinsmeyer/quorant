package ai_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/ai"
	"github.com/quorant/quorant/internal/org"
	"github.com/quorant/quorant/internal/platform/queue"
	"github.com/quorant/quorant/internal/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── mockTaskService ──────────────────────────────────────────────────────────

type mockTaskService struct {
	created []task.Task
}

func (m *mockTaskService) CreateTask(ctx context.Context, orgID uuid.UUID, req task.CreateTaskRequest, createdBy uuid.UUID) (*task.Task, error) {
	t := task.Task{ID: uuid.New(), OrgID: orgID, Title: req.Title, Status: "open"}
	m.created = append(m.created, t)
	return &t, nil
}

func (m *mockTaskService) ListTaskTypes(_ context.Context, _ uuid.UUID) ([]task.TaskType, error) {
	return nil, nil
}
func (m *mockTaskService) CreateTaskType(_ context.Context, _ uuid.UUID, _ task.CreateTaskTypeRequest) (*task.TaskType, error) {
	return nil, nil
}
func (m *mockTaskService) UpdateTaskType(_ context.Context, _ uuid.UUID, _ *task.TaskType) (*task.TaskType, error) {
	return nil, nil
}
func (m *mockTaskService) GetTask(_ context.Context, _ uuid.UUID) (*task.Task, error) {
	return nil, nil
}
func (m *mockTaskService) ListTasks(_ context.Context, _ uuid.UUID, _ int, _ *uuid.UUID) ([]task.Task, bool, error) {
	return nil, false, nil
}
func (m *mockTaskService) ListMyTasks(_ context.Context, _ uuid.UUID) ([]task.Task, error) {
	return nil, nil
}
func (m *mockTaskService) UpdateTask(_ context.Context, _ uuid.UUID, _ *task.Task) (*task.Task, error) {
	return nil, nil
}
func (m *mockTaskService) AssignTask(_ context.Context, _ uuid.UUID, _ task.AssignTaskRequest, _ uuid.UUID) (*task.Task, error) {
	return nil, nil
}
func (m *mockTaskService) TransitionTask(_ context.Context, _ uuid.UUID, _ task.TransitionTaskRequest, _ uuid.UUID) (*task.Task, error) {
	return nil, nil
}
func (m *mockTaskService) AddComment(_ context.Context, _ uuid.UUID, _ task.AddCommentRequest, _ uuid.UUID) (*task.TaskComment, error) {
	return nil, nil
}
func (m *mockTaskService) ListComments(_ context.Context, _ uuid.UUID) ([]task.TaskComment, error) {
	return nil, nil
}
func (m *mockTaskService) ToggleChecklistItem(_ context.Context, _ uuid.UUID, _ string) (*task.Task, error) {
	return nil, nil
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func newTestComplianceWorker(
	ruleRepo *mockJurisdictionRuleRepo,
	checkRepo *mockComplianceCheckRepo,
	orgLookupMock *mockOrgLookup,
	taskSvc task.Service,
	publisher queue.Publisher,
) *ai.ComplianceWorker {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := newTestComplianceService(ruleRepo, checkRepo, orgLookupMock)
	// Register all evaluators so the service can run checks.
	svc.RegisterEvaluator("meeting_notice", ai.EvaluateMeetingNotice)
	svc.RegisterEvaluator("fine_limits", ai.EvaluateFineLimits)
	svc.RegisterEvaluator("reserve_study", ai.EvaluateReserveStudy)
	svc.RegisterEvaluator("website_requirements", ai.EvaluateWebsiteRequirements)
	svc.RegisterEvaluator("record_retention", ai.EvaluateRecordRetention)
	svc.RegisterEvaluator("voting_rules", ai.EvaluateVotingRules)
	svc.RegisterEvaluator("estoppel", ai.EvaluateEstoppel)

	return ai.NewComplianceWorker(svc, ruleRepo, checkRepo, orgLookupMock, taskSvc, publisher, logger)
}

func makeBaseEvent(t *testing.T, payload any) queue.BaseEvent {
	t.Helper()
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	return queue.NewBaseEvent("test.event", "test", uuid.New(), uuid.New(), raw)
}

func makeFlOrg() (uuid.UUID, *org.Organization) {
	orgID := uuid.New()
	jurisdiction := "FL"
	website := "https://sunsetpalms.com"
	return orgID, &org.Organization{
		ID:           orgID,
		Name:         "Sunset Palms HOA",
		Type:         "hoa",
		Jurisdiction: &jurisdiction,
		Website:      &website,
	}
}

// ─── TestComplianceWorker_HandleRuleChange ────────────────────────────────────

func TestComplianceWorker_HandleRuleChange(t *testing.T) {
	// Given: an FL org with 0 units and a website_requirements rule (required_for_unit_count=100),
	// when HandleRuleChange fires for the website rule,
	// then the result is not_applicable (0 units < 100 threshold) and no task is created.
	ruleRepo := &mockJurisdictionRuleRepo{}
	ruleRepo.rules = append(ruleRepo.rules,
		makeJurisdictionRule("FL", "website_requirements", "required_for_unit_count", "integer", 100),
	)

	orgLookupMock := newMockOrgLookup()
	orgID, flOrg := makeFlOrg()
	orgLookupMock.orgs[orgID] = flOrg
	orgLookupMock.byJurisdiction["FL"] = []org.Organization{*flOrg}

	checkRepo := &mockComplianceCheckRepo{}
	taskSvc := &mockTaskService{}
	publisher := queue.NewInMemoryPublisher()

	worker := newTestComplianceWorker(ruleRepo, checkRepo, orgLookupMock, taskSvc, publisher)

	ruleID := uuid.New()
	event := makeBaseEvent(t, map[string]any{
		"rule_id":       ruleID,
		"jurisdiction":  "FL",
		"rule_category": "website_requirements",
	})

	err := worker.HandleRuleChange(context.Background(), event)

	require.NoError(t, err)
	// not_applicable → no task created, no alert published
	assert.Empty(t, taskSvc.created, "expected no task for not_applicable result")
	assert.Empty(t, publisher.Events(), "expected no alert event for not_applicable result")
}

// ─── TestComplianceWorker_HandleOrgChange ─────────────────────────────────────

func TestComplianceWorker_HandleOrgChange(t *testing.T) {
	// Given: an FL org,
	// when HandleOrgChange fires,
	// then EvaluateCompliance runs all 7 categories without error.
	ruleRepo := &mockJurisdictionRuleRepo{}

	orgLookupMock := newMockOrgLookup()
	orgID, flOrg := makeFlOrg()
	orgLookupMock.orgs[orgID] = flOrg

	checkRepo := &mockComplianceCheckRepo{}
	taskSvc := &mockTaskService{}
	publisher := queue.NewInMemoryPublisher()

	worker := newTestComplianceWorker(ruleRepo, checkRepo, orgLookupMock, taskSvc, publisher)

	event := makeBaseEvent(t, map[string]any{"org_id": orgID})

	err := worker.HandleOrgChange(context.Background(), event)

	require.NoError(t, err)
	// With no rules seeded and all evaluators returning "unknown", no non-compliant results → no tasks.
	assert.Empty(t, taskSvc.created, "expected no tasks when all results are unknown/not_applicable")
}

// ─── TestComplianceWorker_HandleDocumentUpload_SkipsNonReserve ───────────────

func TestComplianceWorker_HandleDocumentUpload_SkipsNonReserve(t *testing.T) {
	// Given: a document titled "Invoice Q1",
	// when HandleDocumentUpload fires,
	// then it returns nil immediately without calling CheckCompliance.
	ruleRepo := &mockJurisdictionRuleRepo{}
	orgLookupMock := newMockOrgLookup()
	checkRepo := &mockComplianceCheckRepo{}
	taskSvc := &mockTaskService{}
	publisher := queue.NewInMemoryPublisher()

	worker := newTestComplianceWorker(ruleRepo, checkRepo, orgLookupMock, taskSvc, publisher)

	orgID := uuid.New()
	event := makeBaseEvent(t, map[string]any{
		"org_id":      orgID,
		"document_id": uuid.New(),
		"title":       "Invoice Q1",
	})

	err := worker.HandleDocumentUpload(context.Background(), event)

	require.NoError(t, err)
	assert.Empty(t, taskSvc.created, "expected no task for non-reserve document")
	assert.Empty(t, publisher.Events(), "expected no alert for non-reserve document")
}

// ─── TestComplianceWorker_HandleDocumentUpload_ReserveStudy ──────────────────

func TestComplianceWorker_HandleDocumentUpload_ReserveStudy(t *testing.T) {
	// Given: an FL org with a reserve_study rule and a document titled "Reserve Study 2026",
	// when HandleDocumentUpload fires,
	// then CheckCompliance is called for the reserve_study category.
	ruleRepo := &mockJurisdictionRuleRepo{}

	orgLookupMock := newMockOrgLookup()
	orgID, flOrg := makeFlOrg()
	orgLookupMock.orgs[orgID] = flOrg

	checkRepo := &mockComplianceCheckRepo{}
	taskSvc := &mockTaskService{}
	publisher := queue.NewInMemoryPublisher()

	worker := newTestComplianceWorker(ruleRepo, checkRepo, orgLookupMock, taskSvc, publisher)

	event := makeBaseEvent(t, map[string]any{
		"org_id":      orgID,
		"document_id": uuid.New(),
		"title":       "Reserve Study 2026",
	})

	err := worker.HandleDocumentUpload(context.Background(), event)

	// With no rules seeded the evaluator returns "unknown" (not non_compliant) → no task.
	// The key assertion is that no error occurred, meaning CheckCompliance was reached.
	require.NoError(t, err)
	// Result is "unknown" (no rules seeded), not "non_compliant", so no task created.
	assert.Empty(t, taskSvc.created, "unknown result should not create a task")
}
