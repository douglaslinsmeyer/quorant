package task

import (
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/platform/api"
)

// validTaskStatuses lists all valid values for the task_status enum.
var validTaskStatuses = map[string]bool{
	"open":        true,
	"assigned":    true,
	"in_progress": true,
	"blocked":     true,
	"review":      true,
	"completed":   true,
	"cancelled":   true,
}

// validTaskPriorities lists all valid values for the task_priority enum.
var validTaskPriorities = map[string]bool{
	"low":    true,
	"normal": true,
	"high":   true,
	"urgent": true,
}

// CreateTaskRequest is the request body for creating a new task.
type CreateTaskRequest struct {
	TaskTypeID   uuid.UUID  `json:"task_type_id"`  // required
	Title        string     `json:"title"`          // required
	ResourceType string     `json:"resource_type"`  // required
	ResourceID   uuid.UUID  `json:"resource_id"`    // required
	Description  *string    `json:"description,omitempty"`
	Priority     *string    `json:"priority,omitempty"`
	UnitID       *uuid.UUID `json:"unit_id,omitempty"`
	DueAt        *time.Time `json:"due_at,omitempty"`
}

// Validate checks required fields and any optional enum values.
func (r CreateTaskRequest) Validate() error {
	if r.TaskTypeID == uuid.Nil {
		return api.NewValidationError("task_type_id is required", "task_type_id")
	}
	if r.Title == "" {
		return api.NewValidationError("title is required", "title")
	}
	if r.ResourceType == "" {
		return api.NewValidationError("resource_type is required", "resource_type")
	}
	if r.ResourceID == uuid.Nil {
		return api.NewValidationError("resource_id is required", "resource_id")
	}
	if r.Priority != nil && !validTaskPriorities[*r.Priority] {
		return api.NewValidationError("priority must be one of: low, normal, high, urgent", "priority")
	}
	return nil
}

// CreateTaskTypeRequest is the request body for creating a task type.
type CreateTaskTypeRequest struct {
	Key               string          `json:"key"`           // required
	Name              string          `json:"name"`          // required
	SourceModule      string          `json:"source_module"` // required
	DefaultPriority   *string         `json:"default_priority,omitempty"`
	SLAHours          *int            `json:"sla_hours,omitempty"`
	WorkflowStages    interface{}     `json:"workflow_stages,omitempty"`
	ChecklistTemplate interface{}     `json:"checklist_template,omitempty"`
	AutoAssignRole    *string         `json:"auto_assign_role,omitempty"`
}

// Validate checks that required fields are present.
func (r CreateTaskTypeRequest) Validate() error {
	if r.Key == "" {
		return api.NewValidationError("key is required", "key")
	}
	if r.Name == "" {
		return api.NewValidationError("name is required", "name")
	}
	if r.SourceModule == "" {
		return api.NewValidationError("source_module is required", "source_module")
	}
	if r.DefaultPriority != nil && !validTaskPriorities[*r.DefaultPriority] {
		return api.NewValidationError("default_priority must be one of: low, normal, high, urgent", "default_priority")
	}
	return nil
}

// AssignTaskRequest is the request body for assigning a task.
// At least one of AssignedTo or AssignedRole must be provided.
type AssignTaskRequest struct {
	AssignedTo   *uuid.UUID `json:"assigned_to,omitempty"`
	AssignedRole *string    `json:"assigned_role,omitempty"`
}

// Validate checks that at least one of assigned_to or assigned_role is set.
func (r AssignTaskRequest) Validate() error {
	if r.AssignedTo == nil && r.AssignedRole == nil {
		return api.NewValidationError("at least one of assigned_to or assigned_role is required", "")
	}
	return nil
}

// TransitionTaskRequest is the request body for transitioning a task status.
type TransitionTaskRequest struct {
	Status string  `json:"status"` // required, valid task_status
	Stage  *string `json:"stage,omitempty"`
	Reason *string `json:"reason,omitempty"`
}

// Validate checks that the status is present and valid.
func (r TransitionTaskRequest) Validate() error {
	if r.Status == "" {
		return api.NewValidationError("status is required", "status")
	}
	if !validTaskStatuses[r.Status] {
		return api.NewValidationError("status must be one of: open, assigned, in_progress, blocked, review, completed, cancelled", "status")
	}
	return nil
}

// AddCommentRequest is the request body for adding a comment to a task.
type AddCommentRequest struct {
	Body       string `json:"body"`        // required
	IsInternal bool   `json:"is_internal"` // default true
}

// Validate checks that the body is present.
func (r AddCommentRequest) Validate() error {
	if r.Body == "" {
		return api.NewValidationError("body is required", "body")
	}
	return nil
}
