package task

import (
	"context"

	"github.com/google/uuid"
)

// Service defines the business operations for the task module.
// Handlers depend on this interface rather than the concrete TaskService struct.
type Service interface {
	// Task Types
	ListTaskTypes(ctx context.Context, orgID uuid.UUID) ([]TaskType, error)
	CreateTaskType(ctx context.Context, orgID uuid.UUID, req CreateTaskTypeRequest) (*TaskType, error)
	UpdateTaskType(ctx context.Context, id uuid.UUID, tt *TaskType) (*TaskType, error)

	// Tasks
	CreateTask(ctx context.Context, orgID uuid.UUID, req CreateTaskRequest, createdBy uuid.UUID) (*Task, error)
	GetTask(ctx context.Context, id uuid.UUID) (*Task, error)
	ListTasks(ctx context.Context, orgID uuid.UUID, limit int, afterID *uuid.UUID) ([]Task, bool, error)
	ListMyTasks(ctx context.Context, userID uuid.UUID) ([]Task, error)
	UpdateTask(ctx context.Context, id uuid.UUID, t *Task) (*Task, error)
	AssignTask(ctx context.Context, taskID uuid.UUID, req AssignTaskRequest, assignedBy uuid.UUID) (*Task, error)
	TransitionTask(ctx context.Context, taskID uuid.UUID, req TransitionTaskRequest, changedBy uuid.UUID) (*Task, error)

	// Comments and Checklist
	AddComment(ctx context.Context, taskID uuid.UUID, req AddCommentRequest, authorID uuid.UUID) (*TaskComment, error)
	ListComments(ctx context.Context, taskID uuid.UUID) ([]TaskComment, error)
	ToggleChecklistItem(ctx context.Context, taskID uuid.UUID, itemID string) (*Task, error)
}
