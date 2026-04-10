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
		return api.NewValidationError("validation.required", "task_type_id", api.P("field", "task_type_id"))
	}
	if r.Title == "" {
		return api.NewValidationError("validation.required", "title", api.P("field", "title"))
	}
	if r.ResourceType == "" {
		return api.NewValidationError("validation.required", "resource_type", api.P("field", "resource_type"))
	}
	if r.ResourceID == uuid.Nil {
		return api.NewValidationError("validation.required", "resource_id", api.P("field", "resource_id"))
	}
	if r.Priority != nil && !validTaskPriorities[*r.Priority] {
		return api.NewValidationError("validation.one_of", "priority", api.P("field", "priority"), api.P("values", "low, normal, high, urgent"))
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
		return api.NewValidationError("validation.required", "key", api.P("field", "key"))
	}
	if r.Name == "" {
		return api.NewValidationError("validation.required", "name", api.P("field", "name"))
	}
	if r.SourceModule == "" {
		return api.NewValidationError("validation.required", "source_module", api.P("field", "source_module"))
	}
	if r.DefaultPriority != nil && !validTaskPriorities[*r.DefaultPriority] {
		return api.NewValidationError("validation.one_of", "default_priority", api.P("field", "default_priority"), api.P("values", "low, normal, high, urgent"))
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
		return api.NewValidationError("validation.at_least_one", "")
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
		return api.NewValidationError("validation.required", "status", api.P("field", "status"))
	}
	if !validTaskStatuses[r.Status] {
		return api.NewValidationError("validation.one_of", "status", api.P("field", "status"), api.P("values", "open, assigned, in_progress, blocked, review, completed, cancelled"))
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
		return api.NewValidationError("validation.required", "body", api.P("field", "body"))
	}
	return nil
}
