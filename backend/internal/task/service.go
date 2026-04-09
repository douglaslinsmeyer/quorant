package task

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/audit"
	"github.com/quorant/quorant/internal/platform/api"
	"github.com/quorant/quorant/internal/platform/queue"
)

// validTransitions defines the allowed status transitions for tasks.
// A nil entry means "any status can transition here" is not valid;
// we use an explicit allow-list per source status.
var validTransitions = map[string]map[string]bool{
	"open": {
		"assigned":    true,
		"in_progress": true,
		"cancelled":   true,
	},
	"assigned": {
		"in_progress": true,
		"open":        true,
		"cancelled":   true,
	},
	"in_progress": {
		"blocked": true,
		"review":  true,
		"completed": true,
		"cancelled": true,
	},
	"blocked": {
		"in_progress": true,
		"cancelled":   true,
	},
	"review": {
		"in_progress": true,
		"completed":   true,
		"cancelled":   true,
	},
	"completed": {},
	"cancelled": {},
}

// TaskService orchestrates task business logic.
type TaskService struct {
	repo      TaskRepository
	auditor   audit.Auditor
	publisher queue.Publisher
	logger    *slog.Logger
}

// NewTaskService constructs a TaskService backed by the given repository.
func NewTaskService(repo TaskRepository, auditor audit.Auditor, publisher queue.Publisher, logger *slog.Logger) *TaskService {
	return &TaskService{repo: repo, auditor: auditor, publisher: publisher, logger: logger}
}

// ---------------------------------------------------------------------------
// Task Types
// ---------------------------------------------------------------------------

// ListTaskTypes returns all task types visible to an org (system + org-specific).
func (s *TaskService) ListTaskTypes(ctx context.Context, orgID uuid.UUID) ([]TaskType, error) {
	return s.repo.ListTaskTypesByOrg(ctx, orgID)
}

// CreateTaskType validates and persists a new org-specific task type.
func (s *TaskService) CreateTaskType(ctx context.Context, orgID uuid.UUID, req CreateTaskTypeRequest) (*TaskType, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	priority := "normal"
	if req.DefaultPriority != nil {
		priority = *req.DefaultPriority
	}

	workflowStages := json.RawMessage(`[]`)
	if req.WorkflowStages != nil {
		b, err := json.Marshal(req.WorkflowStages)
		if err != nil {
			return nil, api.NewValidationError("invalid workflow_stages", "workflow_stages")
		}
		workflowStages = json.RawMessage(b)
	}

	checklistTemplate := json.RawMessage(`[]`)
	if req.ChecklistTemplate != nil {
		b, err := json.Marshal(req.ChecklistTemplate)
		if err != nil {
			return nil, api.NewValidationError("invalid checklist_template", "checklist_template")
		}
		checklistTemplate = json.RawMessage(b)
	}

	now := time.Now().UTC()
	tt := &TaskType{
		ID:                uuid.New(),
		OrgID:             &orgID,
		Key:               req.Key,
		Name:              req.Name,
		DefaultPriority:   priority,
		SLAHours:          req.SLAHours,
		WorkflowStages:    workflowStages,
		ChecklistTemplate: checklistTemplate,
		AutoAssignRole:    req.AutoAssignRole,
		SourceModule:      req.SourceModule,
		IsActive:          true,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	return s.repo.CreateTaskType(ctx, tt)
}

// UpdateTaskType persists changes to a task type.
func (s *TaskService) UpdateTaskType(ctx context.Context, id uuid.UUID, tt *TaskType) (*TaskType, error) {
	tt.ID = id
	tt.UpdatedAt = time.Now().UTC()
	return s.repo.UpdateTaskType(ctx, tt)
}

// ---------------------------------------------------------------------------
// Tasks
// ---------------------------------------------------------------------------

// CreateTask validates, optionally computes the SLA deadline, and persists a new task.
func (s *TaskService) CreateTask(ctx context.Context, orgID uuid.UUID, req CreateTaskRequest, createdBy uuid.UUID) (*Task, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	priority := "normal"
	if req.Priority != nil {
		priority = *req.Priority
	}

	now := time.Now().UTC()

	t := &Task{
		ID:           uuid.New(),
		OrgID:        orgID,
		TaskTypeID:   req.TaskTypeID,
		Title:        req.Title,
		Description:  req.Description,
		Status:       "open",
		Priority:     priority,
		ResourceType: req.ResourceType,
		ResourceID:   req.ResourceID,
		UnitID:       req.UnitID,
		DueAt:        req.DueAt,
		CreatedBy:    createdBy,
		Metadata:     map[string]any{},
		Checklist:    json.RawMessage(`[]`),
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	// Fetch task type to compute SLA deadline.
	types, err := s.repo.ListTaskTypesByOrg(ctx, orgID)
	if err != nil {
		s.logger.Warn("failed to list task types for SLA computation", "org_id", orgID, "error", err)
	} else {
		for _, tt := range types {
			if tt.ID == req.TaskTypeID && tt.SLAHours != nil {
				deadline := now.Add(time.Duration(*tt.SLAHours) * time.Hour)
				t.SLADeadline = &deadline
				// Copy the checklist template as the initial checklist.
				if len(tt.ChecklistTemplate) > 0 {
					t.Checklist = tt.ChecklistTemplate
				}
				break
			}
		}
	}

	created, err := s.repo.CreateTask(ctx, t)
	if err != nil {
		return nil, err
	}

	// Record initial status history entry.
	h := &TaskStatusHistory{
		ID:        uuid.New(),
		TaskID:    created.ID,
		ToStatus:  "open",
		ChangedBy: createdBy,
		CreatedAt: now,
	}
	if _, err := s.repo.CreateStatusHistory(ctx, h); err != nil {
		s.logger.Warn("failed to record initial status history", "task_id", created.ID, "error", err)
	}

	return created, nil
}

// GetTask returns a single task by ID or a 404 error if not found.
func (s *TaskService) GetTask(ctx context.Context, id uuid.UUID) (*Task, error) {
	t, err := s.repo.FindTaskByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if t == nil {
		return nil, api.NewNotFoundError("task not found")
	}
	return t, nil
}

// ListTasks returns all tasks for an org.
func (s *TaskService) ListTasks(ctx context.Context, orgID uuid.UUID) ([]Task, error) {
	return s.repo.ListTasksByOrg(ctx, orgID)
}

// ListMyTasks returns all tasks assigned to a user across all orgs.
func (s *TaskService) ListMyTasks(ctx context.Context, userID uuid.UUID) ([]Task, error) {
	return s.repo.ListTasksByAssignee(ctx, userID)
}

// UpdateTask persists changes to an existing task.
func (s *TaskService) UpdateTask(ctx context.Context, id uuid.UUID, t *Task) (*Task, error) {
	t.ID = id
	t.UpdatedAt = time.Now().UTC()
	return s.repo.UpdateTask(ctx, t)
}

// AssignTask sets the assigned_to/role, assigned_at, and assigned_by fields.
func (s *TaskService) AssignTask(ctx context.Context, taskID uuid.UUID, req AssignTaskRequest, assignedBy uuid.UUID) (*Task, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	t, err := s.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	t.AssignedTo = req.AssignedTo
	t.AssignedRole = req.AssignedRole
	t.AssignedAt = &now
	t.AssignedBy = &assignedBy
	if t.Status == "open" {
		t.Status = "assigned"
	}
	t.UpdatedAt = now

	return s.repo.UpdateTask(ctx, t)
}

// TransitionTask validates a status transition and updates the task accordingly.
func (s *TaskService) TransitionTask(ctx context.Context, taskID uuid.UUID, req TransitionTaskRequest, changedBy uuid.UUID) (*Task, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	t, err := s.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}

	// Validate transition.
	allowed, ok := validTransitions[t.Status]
	if !ok || !allowed[req.Status] {
		return nil, api.NewValidationError(
			"invalid transition from "+t.Status+" to "+req.Status, "status",
		)
	}

	now := time.Now().UTC()
	fromStatus := t.Status
	fromStage := t.CurrentStage

	t.Status = req.Status
	if req.Stage != nil {
		t.CurrentStage = req.Stage
	}
	t.UpdatedAt = now

	// Set timestamps based on target status.
	switch req.Status {
	case "in_progress":
		if t.StartedAt == nil {
			t.StartedAt = &now
		}
	case "completed":
		t.CompletedAt = &now
	case "cancelled":
		t.CancelledAt = &now
	}

	updated, err := s.repo.UpdateTask(ctx, t)
	if err != nil {
		return nil, err
	}

	// Record status history.
	h := &TaskStatusHistory{
		ID:         uuid.New(),
		TaskID:     taskID,
		FromStatus: &fromStatus,
		ToStatus:   req.Status,
		FromStage:  fromStage,
		ToStage:    req.Stage,
		ChangedBy:  changedBy,
		Reason:     req.Reason,
		CreatedAt:  now,
	}
	if _, err := s.repo.CreateStatusHistory(ctx, h); err != nil {
		s.logger.Warn("failed to record status history", "task_id", taskID, "error", err)
	}

	return updated, nil
}

// ---------------------------------------------------------------------------
// Comments
// ---------------------------------------------------------------------------

// AddComment validates and persists a new comment on a task.
func (s *TaskService) AddComment(ctx context.Context, taskID uuid.UUID, req AddCommentRequest, authorID uuid.UUID) (*TaskComment, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	c := &TaskComment{
		ID:         uuid.New(),
		TaskID:     taskID,
		AuthorID:   authorID,
		Body:       req.Body,
		IsInternal: req.IsInternal,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	return s.repo.CreateComment(ctx, c)
}

// ListComments returns all non-deleted comments for a task.
func (s *TaskService) ListComments(ctx context.Context, taskID uuid.UUID) ([]TaskComment, error) {
	return s.repo.ListCommentsByTask(ctx, taskID)
}

// ---------------------------------------------------------------------------
// Checklist
// ---------------------------------------------------------------------------

// ToggleChecklistItem toggles a checklist item's completed state in the JSONB checklist.
func (s *TaskService) ToggleChecklistItem(ctx context.Context, taskID uuid.UUID, itemID string) (*Task, error) {
	t, err := s.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}

	// Parse checklist as array of items.
	var checklist []map[string]any
	if len(t.Checklist) > 0 {
		if err := json.Unmarshal(t.Checklist, &checklist); err != nil {
			return nil, api.NewValidationError("invalid checklist format", "checklist")
		}
	}

	found := false
	for i, item := range checklist {
		id, _ := item["id"].(string)
		if id == itemID {
			done, _ := item["done"].(bool)
			checklist[i]["done"] = !done
			found = true
			break
		}
	}

	if !found {
		return nil, api.NewNotFoundError("checklist item not found")
	}

	b, err := json.Marshal(checklist)
	if err != nil {
		return nil, api.NewValidationError("failed to serialize checklist", "checklist")
	}
	t.Checklist = json.RawMessage(b)
	t.UpdatedAt = time.Now().UTC()

	return s.repo.UpdateTask(ctx, t)
}
