package task

import (
	"context"

	"github.com/google/uuid"
)

// TaskRepository persists and retrieves task-related entities.
type TaskRepository interface {
	// Task Types

	// CreateTaskType inserts a new task type and returns the persisted record.
	CreateTaskType(ctx context.Context, tt *TaskType) (*TaskType, error)

	// ListTaskTypesByOrg returns all active task types visible to an org:
	// system-defined types (org_id IS NULL) plus org-specific types.
	ListTaskTypesByOrg(ctx context.Context, orgID uuid.UUID) ([]TaskType, error)

	// UpdateTaskType updates a task type and returns the updated record.
	UpdateTaskType(ctx context.Context, tt *TaskType) (*TaskType, error)

	// Tasks

	// CreateTask inserts a new task and returns the persisted record.
	CreateTask(ctx context.Context, t *Task) (*Task, error)

	// FindTaskByID returns the task with the given ID, or nil if not found.
	FindTaskByID(ctx context.Context, id uuid.UUID) (*Task, error)

	// ListTasksByOrg returns tasks belonging to an organization, supporting cursor-based pagination.
	// afterID is the cursor from the previous page; hasMore is true when more items exist.
	ListTasksByOrg(ctx context.Context, orgID uuid.UUID, limit int, afterID *uuid.UUID) ([]Task, bool, error)

	// ListTasksByAssignee returns all tasks assigned to a user across orgs.
	ListTasksByAssignee(ctx context.Context, userID uuid.UUID) ([]Task, error)

	// ListTasksByResource returns all tasks linked to a specific resource.
	ListTasksByResource(ctx context.Context, resourceType string, resourceID uuid.UUID) ([]Task, error)

	// UpdateTask updates a task and returns the updated record.
	UpdateTask(ctx context.Context, t *Task) (*Task, error)

	// Comments

	// CreateComment inserts a comment on a task and returns the persisted record.
	CreateComment(ctx context.Context, c *TaskComment) (*TaskComment, error)

	// ListCommentsByTask returns all non-deleted comments for a task ordered by created_at.
	ListCommentsByTask(ctx context.Context, taskID uuid.UUID) ([]TaskComment, error)

	// Status History

	// CreateStatusHistory inserts an immutable status transition record.
	CreateStatusHistory(ctx context.Context, h *TaskStatusHistory) (*TaskStatusHistory, error)

	// ListStatusHistoryByTask returns status history for a task ordered by created_at.
	ListStatusHistoryByTask(ctx context.Context, taskID uuid.UUID) ([]TaskStatusHistory, error)
}
