//go:build integration

package task_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/quorant/quorant/internal/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testDSN = "postgres://quorant:quorant@localhost:5432/quorant_dev?sslmode=disable"

// setupIntegrationDB connects to the local Docker postgres and registers a
// cleanup function that removes test data in the correct FK order.
func setupIntegrationDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, testDSN)
	require.NoError(t, err, "connecting to test database")

	t.Cleanup(func() {
		cleanCtx := context.Background()
		pool.Exec(cleanCtx, "DELETE FROM task_status_history")
		pool.Exec(cleanCtx, "DELETE FROM task_comments")
		pool.Exec(cleanCtx, "DELETE FROM tasks")
		pool.Exec(cleanCtx, "DELETE FROM task_types WHERE org_id IS NOT NULL")
		pool.Exec(cleanCtx, "DELETE FROM task_types WHERE source_module = 'test'")
		pool.Exec(cleanCtx, "DELETE FROM memberships")
		pool.Exec(cleanCtx, "DELETE FROM users WHERE email LIKE '%@task-test.example.com'")
		pool.Exec(cleanCtx, "DELETE FROM organizations WHERE name LIKE 'Task Test%'")
		pool.Close()
	})

	return pool
}

// testFixtures holds the IDs of resources created during test setup.
type testFixtures struct {
	OrgID      uuid.UUID
	UserID     uuid.UUID
	TaskTypeID uuid.UUID
}

// createTestFixtures creates a test org, user, and system-level task type.
func createTestFixtures(t *testing.T, pool *pgxpool.Pool) testFixtures {
	t.Helper()
	ctx := context.Background()

	// Create test org
	orgID := uuid.New()
	_, err := pool.Exec(ctx,
		`INSERT INTO organizations (id, type, name, slug, path)
		 VALUES ($1, 'hoa', 'Task Test HOA', $2, $3)`,
		orgID,
		"task-test-hoa-"+orgID.String()[:8],
		"task_test_hoa_"+orgID.String()[:8],
	)
	require.NoError(t, err, "inserting test organization")

	// Create test user
	userID := uuid.New()
	_, err = pool.Exec(ctx,
		`INSERT INTO users (id, idp_user_id, email, display_name, is_active)
		 VALUES ($1, $2, $3, 'Task Test User', true)`,
		userID,
		"idp-task-test-"+userID.String()[:8],
		"task-test-"+userID.String()[:8]+"@task-test.example.com",
	)
	require.NoError(t, err, "inserting test user")

	// Create a system-level task type directly via pool (for reuse across tests)
	repo := task.NewPostgresTaskRepository(pool)
	tt, err := repo.CreateTaskType(ctx, &task.TaskType{
		Key:               "test_maintenance",
		Name:              "Test Maintenance",
		DefaultPriority:   "normal",
		WorkflowStages:    json.RawMessage(`[]`),
		ChecklistTemplate: json.RawMessage(`[]`),
		SourceModule:      "test",
		IsActive:          true,
	})
	require.NoError(t, err, "creating system task type")

	return testFixtures{
		OrgID:      orgID,
		UserID:     userID,
		TaskTypeID: tt.ID,
	}
}

// newTestTask returns a minimal Task struct populated for testing.
func newTestTask(fix testFixtures) *task.Task {
	return &task.Task{
		OrgID:        fix.OrgID,
		TaskTypeID:   fix.TaskTypeID,
		Title:        "Fix the leaky faucet",
		Status:       "open",
		Priority:     "normal",
		ResourceType: "unit",
		ResourceID:   uuid.New(),
		CreatedBy:    fix.UserID,
		Metadata:     map[string]any{},
		Checklist:    json.RawMessage(`[]`),
	}
}

// ─── TaskType ─────────────────────────────────────────────────────────────────

func TestCreateTaskType_AndListByOrg(t *testing.T) {
	pool := setupIntegrationDB(t)
	repo := task.NewPostgresTaskRepository(pool)
	ctx := context.Background()
	fix := createTestFixtures(t, pool)

	// Create an org-specific task type
	desc := "An org-specific task type"
	sla := 24
	role := "maintenance_staff"
	orgTT, err := repo.CreateTaskType(ctx, &task.TaskType{
		OrgID:             &fix.OrgID,
		Key:               "org_maintenance",
		Name:              "Org Maintenance",
		Description:       &desc,
		DefaultPriority:   "high",
		SLAHours:          &sla,
		WorkflowStages:    json.RawMessage(`[{"name":"open"}]`),
		ChecklistTemplate: json.RawMessage(`[{"item":"Check area"}]`),
		AutoAssignRole:    &role,
		SourceModule:      "test",
		IsActive:          true,
	})
	require.NoError(t, err)
	require.NotNil(t, orgTT)
	assert.NotEqual(t, uuid.Nil, orgTT.ID)
	assert.Equal(t, &fix.OrgID, orgTT.OrgID)
	assert.Equal(t, "org_maintenance", orgTT.Key)
	assert.Equal(t, "high", orgTT.DefaultPriority)
	assert.Equal(t, &sla, orgTT.SLAHours)
	assert.Equal(t, &role, orgTT.AutoAssignRole)
	assert.False(t, orgTT.CreatedAt.IsZero())

	// ListByOrg should include both system type (from fixture) and org-specific type
	types, err := repo.ListTaskTypesByOrg(ctx, fix.OrgID)
	require.NoError(t, err)

	var keys []string
	for _, tt := range types {
		keys = append(keys, tt.Key)
	}
	assert.Contains(t, keys, "test_maintenance", "system type should be in list")
	assert.Contains(t, keys, "org_maintenance", "org-specific type should be in list")
}

func TestUpdateTaskType(t *testing.T) {
	pool := setupIntegrationDB(t)
	repo := task.NewPostgresTaskRepository(pool)
	ctx := context.Background()
	fix := createTestFixtures(t, pool)

	// Create a task type to update
	tt, err := repo.CreateTaskType(ctx, &task.TaskType{
		OrgID:             &fix.OrgID,
		Key:               "update_test",
		Name:              "Update Test",
		DefaultPriority:   "normal",
		WorkflowStages:    json.RawMessage(`[]`),
		ChecklistTemplate: json.RawMessage(`[]`),
		SourceModule:      "test",
		IsActive:          true,
	})
	require.NoError(t, err)

	tt.Name = "Updated Name"
	tt.IsActive = false

	updated, err := repo.UpdateTaskType(ctx, tt)
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, "Updated Name", updated.Name)
	assert.False(t, updated.IsActive)
}

// ─── Tasks ────────────────────────────────────────────────────────────────────

func TestCreateTask_AndFindByID(t *testing.T) {
	pool := setupIntegrationDB(t)
	repo := task.NewPostgresTaskRepository(pool)
	ctx := context.Background()
	fix := createTestFixtures(t, pool)

	input := newTestTask(fix)
	created, err := repo.CreateTask(ctx, input)

	require.NoError(t, err)
	require.NotNil(t, created)
	assert.NotEqual(t, uuid.Nil, created.ID)
	assert.Equal(t, fix.OrgID, created.OrgID)
	assert.Equal(t, fix.TaskTypeID, created.TaskTypeID)
	assert.Equal(t, "Fix the leaky faucet", created.Title)
	assert.Equal(t, "open", created.Status)
	assert.Equal(t, "normal", created.Priority)
	assert.False(t, created.SLABreached)
	assert.False(t, created.CreatedAt.IsZero())

	// FindByID
	found, err := repo.FindTaskByID(ctx, created.ID)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, created.ID, found.ID)
	assert.Equal(t, "Fix the leaky faucet", found.Title)
}

func TestFindTaskByID_NotFound(t *testing.T) {
	pool := setupIntegrationDB(t)
	repo := task.NewPostgresTaskRepository(pool)
	ctx := context.Background()

	found, err := repo.FindTaskByID(ctx, uuid.New())

	require.NoError(t, err)
	assert.Nil(t, found, "should return nil for unknown task ID")
}

func TestListTasksByOrg(t *testing.T) {
	pool := setupIntegrationDB(t)
	repo := task.NewPostgresTaskRepository(pool)
	ctx := context.Background()
	fix := createTestFixtures(t, pool)

	// Create two tasks for the test org
	_, err := repo.CreateTask(ctx, newTestTask(fix))
	require.NoError(t, err)
	_, err = repo.CreateTask(ctx, newTestTask(fix))
	require.NoError(t, err)

	tasks, err := repo.ListTasksByOrg(ctx, fix.OrgID)

	require.NoError(t, err)
	assert.Len(t, tasks, 2)
	for _, tsk := range tasks {
		assert.Equal(t, fix.OrgID, tsk.OrgID)
	}
}

func TestListTasksByAssignee(t *testing.T) {
	pool := setupIntegrationDB(t)
	repo := task.NewPostgresTaskRepository(pool)
	ctx := context.Background()
	fix := createTestFixtures(t, pool)

	// Create a task and assign it
	created, err := repo.CreateTask(ctx, newTestTask(fix))
	require.NoError(t, err)

	created.AssignedTo = &fix.UserID
	created.Status = "assigned"
	updated, err := repo.UpdateTask(ctx, created)
	require.NoError(t, err)
	require.NotNil(t, updated)

	tasks, err := repo.ListTasksByAssignee(ctx, fix.UserID)

	require.NoError(t, err)
	require.Len(t, tasks, 1)
	assert.Equal(t, created.ID, tasks[0].ID)
	assert.Equal(t, &fix.UserID, tasks[0].AssignedTo)
}

func TestListTasksByResource(t *testing.T) {
	pool := setupIntegrationDB(t)
	repo := task.NewPostgresTaskRepository(pool)
	ctx := context.Background()
	fix := createTestFixtures(t, pool)

	resourceID := uuid.New()

	// Create two tasks for the same resource
	t1 := newTestTask(fix)
	t1.ResourceID = resourceID
	_, err := repo.CreateTask(ctx, t1)
	require.NoError(t, err)

	t2 := newTestTask(fix)
	t2.ResourceID = resourceID
	t2.Title = "Second task for resource"
	_, err = repo.CreateTask(ctx, t2)
	require.NoError(t, err)

	// Create a task for a different resource (should not appear)
	t3 := newTestTask(fix)
	t3.ResourceID = uuid.New()
	_, err = repo.CreateTask(ctx, t3)
	require.NoError(t, err)

	tasks, err := repo.ListTasksByResource(ctx, "unit", resourceID)

	require.NoError(t, err)
	assert.Len(t, tasks, 2)
	for _, tsk := range tasks {
		assert.Equal(t, resourceID, tsk.ResourceID)
	}
}

func TestUpdateTask_ChangeStatusAndPriority(t *testing.T) {
	pool := setupIntegrationDB(t)
	repo := task.NewPostgresTaskRepository(pool)
	ctx := context.Background()
	fix := createTestFixtures(t, pool)

	created, err := repo.CreateTask(ctx, newTestTask(fix))
	require.NoError(t, err)
	assert.Equal(t, "open", created.Status)
	assert.Equal(t, "normal", created.Priority)

	now := time.Now().UTC()
	created.Status = "in_progress"
	created.Priority = "high"
	created.StartedAt = &now

	updated, err := repo.UpdateTask(ctx, created)

	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, created.ID, updated.ID)
	assert.Equal(t, "in_progress", updated.Status)
	assert.Equal(t, "high", updated.Priority)
	require.NotNil(t, updated.StartedAt)
}

// ─── Comments ─────────────────────────────────────────────────────────────────

func TestCreateComment_AndListByTask(t *testing.T) {
	pool := setupIntegrationDB(t)
	repo := task.NewPostgresTaskRepository(pool)
	ctx := context.Background()
	fix := createTestFixtures(t, pool)

	tsk, err := repo.CreateTask(ctx, newTestTask(fix))
	require.NoError(t, err)

	// Create two comments
	c1, err := repo.CreateComment(ctx, &task.TaskComment{
		TaskID:     tsk.ID,
		AuthorID:   fix.UserID,
		Body:       "Investigating the issue",
		IsInternal: true,
	})
	require.NoError(t, err)
	require.NotNil(t, c1)
	assert.NotEqual(t, uuid.Nil, c1.ID)
	assert.Equal(t, tsk.ID, c1.TaskID)
	assert.Equal(t, "Investigating the issue", c1.Body)
	assert.True(t, c1.IsInternal)
	assert.Nil(t, c1.DeletedAt)

	c2, err := repo.CreateComment(ctx, &task.TaskComment{
		TaskID:        tsk.ID,
		AuthorID:      fix.UserID,
		Body:          "Technician dispatched",
		AttachmentIDs: []uuid.UUID{uuid.New()},
		IsInternal:    false,
	})
	require.NoError(t, err)
	require.NotNil(t, c2)

	// List comments for the task
	comments, err := repo.ListCommentsByTask(ctx, tsk.ID)

	require.NoError(t, err)
	require.Len(t, comments, 2)
	assert.Equal(t, c1.ID, comments[0].ID, "comments should be ordered by created_at ASC")
	assert.Equal(t, c2.ID, comments[1].ID)
	assert.Len(t, comments[1].AttachmentIDs, 1)
}

func TestListCommentsByTask_ExcludesDeletedComments(t *testing.T) {
	pool := setupIntegrationDB(t)
	repo := task.NewPostgresTaskRepository(pool)
	ctx := context.Background()
	fix := createTestFixtures(t, pool)

	tsk, err := repo.CreateTask(ctx, newTestTask(fix))
	require.NoError(t, err)

	c, err := repo.CreateComment(ctx, &task.TaskComment{
		TaskID:   tsk.ID,
		AuthorID: fix.UserID,
		Body:     "This comment will be deleted",
	})
	require.NoError(t, err)

	// Soft-delete the comment directly
	_, err = pool.Exec(ctx, "UPDATE task_comments SET deleted_at = now() WHERE id = $1", c.ID)
	require.NoError(t, err)

	comments, err := repo.ListCommentsByTask(ctx, tsk.ID)
	require.NoError(t, err)
	assert.Empty(t, comments, "deleted comments should be excluded")
}

// ─── Status History ───────────────────────────────────────────────────────────

func TestCreateStatusHistory_AndListByTask(t *testing.T) {
	pool := setupIntegrationDB(t)
	repo := task.NewPostgresTaskRepository(pool)
	ctx := context.Background()
	fix := createTestFixtures(t, pool)

	tsk, err := repo.CreateTask(ctx, newTestTask(fix))
	require.NoError(t, err)

	// Record two transitions
	fromOpen := "open"
	h1, err := repo.CreateStatusHistory(ctx, &task.TaskStatusHistory{
		TaskID:    tsk.ID,
		ToStatus:  "open",
		ChangedBy: fix.UserID,
	})
	require.NoError(t, err)
	require.NotNil(t, h1)
	assert.NotEqual(t, uuid.Nil, h1.ID)
	assert.Equal(t, tsk.ID, h1.TaskID)
	assert.Nil(t, h1.FromStatus, "first transition has no from_status")
	assert.Equal(t, "open", h1.ToStatus)

	reason := "Starting work"
	h2, err := repo.CreateStatusHistory(ctx, &task.TaskStatusHistory{
		TaskID:     tsk.ID,
		FromStatus: &fromOpen,
		ToStatus:   "in_progress",
		ChangedBy:  fix.UserID,
		Reason:     &reason,
	})
	require.NoError(t, err)
	require.NotNil(t, h2)

	// List history — should be ordered by created_at ASC
	history, err := repo.ListStatusHistoryByTask(ctx, tsk.ID)

	require.NoError(t, err)
	require.Len(t, history, 2)
	assert.Equal(t, h1.ID, history[0].ID, "history should be ordered by created_at ASC")
	assert.Equal(t, h2.ID, history[1].ID)
	assert.Equal(t, "in_progress", history[1].ToStatus)
	require.NotNil(t, history[1].FromStatus)
	assert.Equal(t, "open", *history[1].FromStatus)
	require.NotNil(t, history[1].Reason)
	assert.Equal(t, "Starting work", *history[1].Reason)
}

func TestListStatusHistoryByTask_Empty(t *testing.T) {
	pool := setupIntegrationDB(t)
	repo := task.NewPostgresTaskRepository(pool)
	ctx := context.Background()
	fix := createTestFixtures(t, pool)

	tsk, err := repo.CreateTask(ctx, newTestTask(fix))
	require.NoError(t, err)

	history, err := repo.ListStatusHistoryByTask(ctx, tsk.ID)
	require.NoError(t, err)
	assert.Empty(t, history)
}
