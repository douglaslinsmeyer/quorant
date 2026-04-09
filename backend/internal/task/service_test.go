package task_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/audit"
	"github.com/quorant/quorant/internal/platform/api"
	"github.com/quorant/quorant/internal/platform/queue"
	"github.com/quorant/quorant/internal/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// In-memory mock repository
// ---------------------------------------------------------------------------

type mockTaskRepo struct {
	taskTypes   map[uuid.UUID]*task.TaskType
	tasks       map[uuid.UUID]*task.Task
	comments    map[uuid.UUID][]*task.TaskComment
	history     map[uuid.UUID][]*task.TaskStatusHistory
}

func newMockTaskRepo() *mockTaskRepo {
	return &mockTaskRepo{
		taskTypes: make(map[uuid.UUID]*task.TaskType),
		tasks:     make(map[uuid.UUID]*task.Task),
		comments:  make(map[uuid.UUID][]*task.TaskComment),
		history:   make(map[uuid.UUID][]*task.TaskStatusHistory),
	}
}

func (r *mockTaskRepo) CreateTaskType(_ context.Context, tt *task.TaskType) (*task.TaskType, error) {
	r.taskTypes[tt.ID] = tt
	return tt, nil
}

func (r *mockTaskRepo) ListTaskTypesByOrg(_ context.Context, orgID uuid.UUID) ([]task.TaskType, error) {
	var out []task.TaskType
	for _, tt := range r.taskTypes {
		if tt.OrgID == nil || *tt.OrgID == orgID {
			out = append(out, *tt)
		}
	}
	return out, nil
}

func (r *mockTaskRepo) UpdateTaskType(_ context.Context, tt *task.TaskType) (*task.TaskType, error) {
	r.taskTypes[tt.ID] = tt
	return tt, nil
}

func (r *mockTaskRepo) CreateTask(_ context.Context, t *task.Task) (*task.Task, error) {
	r.tasks[t.ID] = t
	return t, nil
}

func (r *mockTaskRepo) FindTaskByID(_ context.Context, id uuid.UUID) (*task.Task, error) {
	t, ok := r.tasks[id]
	if !ok {
		return nil, nil
	}
	return t, nil
}

func (r *mockTaskRepo) ListTasksByOrg(_ context.Context, orgID uuid.UUID) ([]task.Task, error) {
	var out []task.Task
	for _, t := range r.tasks {
		if t.OrgID == orgID {
			out = append(out, *t)
		}
	}
	return out, nil
}

func (r *mockTaskRepo) ListTasksByAssignee(_ context.Context, userID uuid.UUID) ([]task.Task, error) {
	var out []task.Task
	for _, t := range r.tasks {
		if t.AssignedTo != nil && *t.AssignedTo == userID {
			out = append(out, *t)
		}
	}
	return out, nil
}

func (r *mockTaskRepo) ListTasksByResource(_ context.Context, resourceType string, resourceID uuid.UUID) ([]task.Task, error) {
	var out []task.Task
	for _, t := range r.tasks {
		if t.ResourceType == resourceType && t.ResourceID == resourceID {
			out = append(out, *t)
		}
	}
	return out, nil
}

func (r *mockTaskRepo) UpdateTask(_ context.Context, t *task.Task) (*task.Task, error) {
	r.tasks[t.ID] = t
	return t, nil
}

func (r *mockTaskRepo) CreateComment(_ context.Context, c *task.TaskComment) (*task.TaskComment, error) {
	r.comments[c.TaskID] = append(r.comments[c.TaskID], c)
	return c, nil
}

func (r *mockTaskRepo) ListCommentsByTask(_ context.Context, taskID uuid.UUID) ([]task.TaskComment, error) {
	var out []task.TaskComment
	for _, c := range r.comments[taskID] {
		if c.DeletedAt == nil {
			out = append(out, *c)
		}
	}
	return out, nil
}

func (r *mockTaskRepo) CreateStatusHistory(_ context.Context, h *task.TaskStatusHistory) (*task.TaskStatusHistory, error) {
	r.history[h.TaskID] = append(r.history[h.TaskID], h)
	return h, nil
}

func (r *mockTaskRepo) ListStatusHistoryByTask(_ context.Context, taskID uuid.UUID) ([]task.TaskStatusHistory, error) {
	var out []task.TaskStatusHistory
	for _, h := range r.history[taskID] {
		out = append(out, *h)
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func newTestTaskService() (*task.TaskService, *mockTaskRepo) {
	repo := newMockTaskRepo()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	svc := task.NewTaskService(repo, audit.NewNoopAuditor(), queue.NewInMemoryPublisher(), logger)
	return svc, repo
}

func seedTask(t *testing.T, repo *mockTaskRepo, orgID uuid.UUID) *task.Task {
	t.Helper()
	tsk := &task.Task{
		ID:           uuid.New(),
		OrgID:        orgID,
		TaskTypeID:   uuid.New(),
		Title:        "Fix the leak",
		Status:       "open",
		Priority:     "normal",
		ResourceType: "unit",
		ResourceID:   uuid.New(),
		CreatedBy:    uuid.New(),
		Metadata:     map[string]any{},
		Checklist:    json.RawMessage(`[]`),
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	repo.tasks[tsk.ID] = tsk
	return tsk
}

// ---------------------------------------------------------------------------
// TestCreateTask_ComputesSLADeadline
// ---------------------------------------------------------------------------

func TestCreateTask_ComputesSLADeadline(t *testing.T) {
	svc, repo := newTestTaskService()
	ctx := context.Background()

	orgID := uuid.New()
	slaHours := 48
	ttID := uuid.New()
	tt := &task.TaskType{
		ID:                ttID,
		OrgID:             &orgID,
		Key:               "maintenance_request",
		Name:              "Maintenance Request",
		DefaultPriority:   "normal",
		SLAHours:          &slaHours,
		WorkflowStages:    json.RawMessage(`[]`),
		ChecklistTemplate: json.RawMessage(`[]`),
		SourceModule:      "maintenance",
		IsActive:          true,
		CreatedAt:         time.Now().UTC(),
		UpdatedAt:         time.Now().UTC(),
	}
	repo.taskTypes[ttID] = tt

	before := time.Now().UTC()
	req := task.CreateTaskRequest{
		TaskTypeID:   ttID,
		Title:        "Fix the roof",
		ResourceType: "unit",
		ResourceID:   uuid.New(),
	}

	created, err := svc.CreateTask(ctx, orgID, req, uuid.New())
	require.NoError(t, err)
	require.NotNil(t, created)
	require.NotNil(t, created.SLADeadline, "SLA deadline should be computed from task type's sla_hours")

	expectedDeadline := before.Add(48 * time.Hour)
	assert.WithinDuration(t, expectedDeadline, *created.SLADeadline, 5*time.Second)
}

// ---------------------------------------------------------------------------
// TestCreateTask_NoSLAWhenTypeHasNoSLAHours
// ---------------------------------------------------------------------------

func TestCreateTask_NoSLAWhenTypeHasNoSLAHours(t *testing.T) {
	svc, repo := newTestTaskService()
	ctx := context.Background()

	orgID := uuid.New()
	ttID := uuid.New()
	tt := &task.TaskType{
		ID:                ttID,
		OrgID:             &orgID,
		Key:               "general",
		Name:              "General",
		DefaultPriority:   "normal",
		SLAHours:          nil, // no SLA
		WorkflowStages:    json.RawMessage(`[]`),
		ChecklistTemplate: json.RawMessage(`[]`),
		SourceModule:      "general",
		IsActive:          true,
		CreatedAt:         time.Now().UTC(),
		UpdatedAt:         time.Now().UTC(),
	}
	repo.taskTypes[ttID] = tt

	req := task.CreateTaskRequest{
		TaskTypeID:   ttID,
		Title:        "General task",
		ResourceType: "unit",
		ResourceID:   uuid.New(),
	}

	created, err := svc.CreateTask(ctx, orgID, req, uuid.New())
	require.NoError(t, err)
	require.NotNil(t, created)
	assert.Nil(t, created.SLADeadline, "no SLA deadline when task type has no sla_hours")
}

// ---------------------------------------------------------------------------
// TestTransitionTask_SetsStartedAt
// ---------------------------------------------------------------------------

func TestTransitionTask_SetsStartedAt(t *testing.T) {
	svc, repo := newTestTaskService()
	ctx := context.Background()

	orgID := uuid.New()
	tsk := seedTask(t, repo, orgID)
	changedBy := uuid.New()

	req := task.TransitionTaskRequest{Status: "in_progress"}
	updated, err := svc.TransitionTask(ctx, tsk.ID, req, changedBy)
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, "in_progress", updated.Status)
	require.NotNil(t, updated.StartedAt, "started_at should be set on transition to in_progress")
}

// ---------------------------------------------------------------------------
// TestTransitionTask_DoesNotOverwriteStartedAt
// ---------------------------------------------------------------------------

func TestTransitionTask_DoesNotOverwriteStartedAt(t *testing.T) {
	svc, repo := newTestTaskService()
	ctx := context.Background()

	orgID := uuid.New()
	originalStart := time.Now().UTC().Add(-1 * time.Hour)
	tsk := seedTask(t, repo, orgID)
	tsk.Status = "in_progress"
	tsk.StartedAt = &originalStart
	repo.tasks[tsk.ID] = tsk

	// Transition from in_progress -> blocked -> in_progress again.
	// First move to blocked.
	_, err := svc.TransitionTask(ctx, tsk.ID, task.TransitionTaskRequest{Status: "blocked"}, uuid.New())
	require.NoError(t, err)

	// Now move back to in_progress.
	updated, err := svc.TransitionTask(ctx, tsk.ID, task.TransitionTaskRequest{Status: "in_progress"}, uuid.New())
	require.NoError(t, err)
	require.NotNil(t, updated.StartedAt)
	assert.WithinDuration(t, originalStart, *updated.StartedAt, time.Second,
		"started_at should not be overwritten on re-entry to in_progress")
}

// ---------------------------------------------------------------------------
// TestTransitionTask_SetsCompletedAt
// ---------------------------------------------------------------------------

func TestTransitionTask_SetsCompletedAt(t *testing.T) {
	svc, repo := newTestTaskService()
	ctx := context.Background()

	orgID := uuid.New()
	tsk := seedTask(t, repo, orgID)
	tsk.Status = "in_progress"
	repo.tasks[tsk.ID] = tsk

	before := time.Now().UTC()
	req := task.TransitionTaskRequest{Status: "completed"}
	updated, err := svc.TransitionTask(ctx, tsk.ID, req, uuid.New())
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, "completed", updated.Status)
	require.NotNil(t, updated.CompletedAt, "completed_at should be set on transition to completed")
	assert.True(t, updated.CompletedAt.After(before) || updated.CompletedAt.Equal(before))
}

// ---------------------------------------------------------------------------
// TestTransitionTask_RecordsHistory
// ---------------------------------------------------------------------------

func TestTransitionTask_RecordsHistory(t *testing.T) {
	svc, repo := newTestTaskService()
	ctx := context.Background()

	orgID := uuid.New()
	tsk := seedTask(t, repo, orgID)
	changedBy := uuid.New()

	req := task.TransitionTaskRequest{Status: "in_progress"}
	_, err := svc.TransitionTask(ctx, tsk.ID, req, changedBy)
	require.NoError(t, err)

	history := repo.history[tsk.ID]
	require.NotEmpty(t, history, "status history should be recorded on transition")

	// Find the transition record.
	var found *task.TaskStatusHistory
	for _, h := range history {
		if h.ToStatus == "in_progress" {
			found = h
			break
		}
	}
	require.NotNil(t, found)
	require.NotNil(t, found.FromStatus)
	assert.Equal(t, "open", *found.FromStatus)
	assert.Equal(t, "in_progress", found.ToStatus)
	assert.Equal(t, changedBy, found.ChangedBy)
}

// ---------------------------------------------------------------------------
// TestTransitionTask_InvalidTransition
// ---------------------------------------------------------------------------

func TestTransitionTask_InvalidTransition(t *testing.T) {
	svc, repo := newTestTaskService()
	ctx := context.Background()

	orgID := uuid.New()
	tsk := seedTask(t, repo, orgID)
	tsk.Status = "completed"
	repo.tasks[tsk.ID] = tsk

	req := task.TransitionTaskRequest{Status: "open"}
	_, err := svc.TransitionTask(ctx, tsk.ID, req, uuid.New())
	require.Error(t, err, "transition from completed to open should be invalid")
	var ve *api.ValidationError
	require.ErrorAs(t, err, &ve)
}

// ---------------------------------------------------------------------------
// TestAssignTask_SetsFields
// ---------------------------------------------------------------------------

func TestAssignTask_SetsFields(t *testing.T) {
	svc, repo := newTestTaskService()
	ctx := context.Background()

	orgID := uuid.New()
	tsk := seedTask(t, repo, orgID)
	assignedBy := uuid.New()
	assignedTo := uuid.New()

	req := task.AssignTaskRequest{AssignedTo: &assignedTo}
	updated, err := svc.AssignTask(ctx, tsk.ID, req, assignedBy)
	require.NoError(t, err)
	require.NotNil(t, updated)

	require.NotNil(t, updated.AssignedTo)
	assert.Equal(t, assignedTo, *updated.AssignedTo)
	require.NotNil(t, updated.AssignedBy)
	assert.Equal(t, assignedBy, *updated.AssignedBy)
	require.NotNil(t, updated.AssignedAt)
	assert.Equal(t, "assigned", updated.Status)
}

// ---------------------------------------------------------------------------
// TestAssignTask_SetsRole
// ---------------------------------------------------------------------------

func TestAssignTask_SetsRole(t *testing.T) {
	svc, repo := newTestTaskService()
	ctx := context.Background()

	orgID := uuid.New()
	tsk := seedTask(t, repo, orgID)
	role := "maintenance_staff"

	req := task.AssignTaskRequest{AssignedRole: &role}
	updated, err := svc.AssignTask(ctx, tsk.ID, req, uuid.New())
	require.NoError(t, err)
	require.NotNil(t, updated.AssignedRole)
	assert.Equal(t, role, *updated.AssignedRole)
}

// ---------------------------------------------------------------------------
// TestGetTask_NotFound
// ---------------------------------------------------------------------------

func TestGetTask_NotFound(t *testing.T) {
	svc, _ := newTestTaskService()
	ctx := context.Background()

	_, err := svc.GetTask(ctx, uuid.New())
	require.Error(t, err)
	var nfe *api.NotFoundError
	require.ErrorAs(t, err, &nfe)
}

// ---------------------------------------------------------------------------
// TestAddComment_Success
// ---------------------------------------------------------------------------

func TestAddComment_Success(t *testing.T) {
	svc, repo := newTestTaskService()
	ctx := context.Background()

	orgID := uuid.New()
	tsk := seedTask(t, repo, orgID)
	authorID := uuid.New()

	req := task.AddCommentRequest{Body: "Technician dispatched", IsInternal: true}
	comment, err := svc.AddComment(ctx, tsk.ID, req, authorID)
	require.NoError(t, err)
	require.NotNil(t, comment)

	assert.Equal(t, tsk.ID, comment.TaskID)
	assert.Equal(t, authorID, comment.AuthorID)
	assert.Equal(t, "Technician dispatched", comment.Body)
	assert.True(t, comment.IsInternal)
}

// ---------------------------------------------------------------------------
// TestAddComment_ValidationError
// ---------------------------------------------------------------------------

func TestAddComment_ValidationError(t *testing.T) {
	svc, _ := newTestTaskService()
	ctx := context.Background()

	req := task.AddCommentRequest{Body: ""}
	_, err := svc.AddComment(ctx, uuid.New(), req, uuid.New())
	require.Error(t, err)
	var ve *api.ValidationError
	require.ErrorAs(t, err, &ve)
}

// ---------------------------------------------------------------------------
// TestToggleChecklistItem_TogglesCompleted
// ---------------------------------------------------------------------------

func TestToggleChecklistItem_TogglesCompleted(t *testing.T) {
	svc, repo := newTestTaskService()
	ctx := context.Background()

	orgID := uuid.New()
	tsk := seedTask(t, repo, orgID)
	tsk.Checklist = json.RawMessage(`[{"id":"item-1","done":false,"label":"Check water pressure"},{"id":"item-2","done":true,"label":"Inspect pipes"}]`)
	repo.tasks[tsk.ID] = tsk

	updated, err := svc.ToggleChecklistItem(ctx, tsk.ID, "item-1")
	require.NoError(t, err)
	require.NotNil(t, updated)

	var checklist []map[string]any
	require.NoError(t, json.Unmarshal(updated.Checklist, &checklist))
	require.Len(t, checklist, 2)
	assert.Equal(t, true, checklist[0]["done"], "item-1 should be toggled to done=true")
	assert.Equal(t, true, checklist[1]["done"], "item-2 should remain done=true")
}

// ---------------------------------------------------------------------------
// TestToggleChecklistItem_NotFound
// ---------------------------------------------------------------------------

func TestToggleChecklistItem_NotFound(t *testing.T) {
	svc, repo := newTestTaskService()
	ctx := context.Background()

	orgID := uuid.New()
	tsk := seedTask(t, repo, orgID)
	tsk.Checklist = json.RawMessage(`[{"id":"item-1","done":false,"label":"Check water pressure"}]`)
	repo.tasks[tsk.ID] = tsk

	_, err := svc.ToggleChecklistItem(ctx, tsk.ID, "nonexistent-id")
	require.Error(t, err)
	var nfe *api.NotFoundError
	require.ErrorAs(t, err, &nfe)
}

// ---------------------------------------------------------------------------
// TestListTasks_ReturnsOrgTasks
// ---------------------------------------------------------------------------

func TestListTasks_ReturnsOrgTasks(t *testing.T) {
	svc, repo := newTestTaskService()
	ctx := context.Background()

	orgID := uuid.New()
	otherOrgID := uuid.New()
	seedTask(t, repo, orgID)
	seedTask(t, repo, orgID)
	seedTask(t, repo, otherOrgID) // different org — should not appear

	tasks, err := svc.ListTasks(ctx, orgID)
	require.NoError(t, err)
	assert.Len(t, tasks, 2)
}

// ---------------------------------------------------------------------------
// TestListMyTasks_ReturnsAssignedTasks
// ---------------------------------------------------------------------------

func TestListMyTasks_ReturnsAssignedTasks(t *testing.T) {
	svc, repo := newTestTaskService()
	ctx := context.Background()

	userID := uuid.New()
	orgID := uuid.New()
	tsk1 := seedTask(t, repo, orgID)
	tsk1.AssignedTo = &userID
	repo.tasks[tsk1.ID] = tsk1

	seedTask(t, repo, orgID) // not assigned to userID

	tasks, err := svc.ListMyTasks(ctx, userID)
	require.NoError(t, err)
	assert.Len(t, tasks, 1)
	assert.Equal(t, tsk1.ID, tasks[0].ID)
}
