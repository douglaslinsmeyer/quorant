package task

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// TaskType represents a configurable task type definition.
// System-defined types have a nil OrgID.
type TaskType struct {
	ID                uuid.UUID       `json:"id"`
	OrgID             *uuid.UUID      `json:"org_id,omitempty"` // nil = system-defined
	Key               string          `json:"key"`
	Name              string          `json:"name"`
	Description       *string         `json:"description,omitempty"`
	DefaultPriority   string          `json:"default_priority"`
	SLAHours          *int            `json:"sla_hours,omitempty"`
	WorkflowStages    json.RawMessage `json:"workflow_stages"`
	ChecklistTemplate json.RawMessage `json:"checklist_template"`
	AutoAssignRole    *string         `json:"auto_assign_role,omitempty"`
	SourceModule      string          `json:"source_module"`
	IsActive          bool            `json:"is_active"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
}

// Task represents a unit of work within an organization.
type Task struct {
	ID              uuid.UUID       `json:"id"`
	OrgID           uuid.UUID       `json:"org_id"`
	TaskTypeID      uuid.UUID       `json:"task_type_id"`
	Title           string          `json:"title"`
	Description     *string         `json:"description,omitempty"`
	Status          string          `json:"status"`
	Priority        string          `json:"priority"`
	CurrentStage    *string         `json:"current_stage,omitempty"`
	ResourceType    string          `json:"resource_type"`
	ResourceID      uuid.UUID       `json:"resource_id"`
	UnitID          *uuid.UUID      `json:"unit_id,omitempty"`
	AssignedTo      *uuid.UUID      `json:"assigned_to,omitempty"`
	AssignedRole    *string         `json:"assigned_role,omitempty"`
	AssignedAt      *time.Time      `json:"assigned_at,omitempty"`
	AssignedBy      *uuid.UUID      `json:"assigned_by,omitempty"`
	DueAt           *time.Time      `json:"due_at,omitempty"`
	SLADeadline     *time.Time      `json:"sla_deadline,omitempty"`
	SLABreached     bool            `json:"sla_breached"`
	StartedAt       *time.Time      `json:"started_at,omitempty"`
	CompletedAt     *time.Time      `json:"completed_at,omitempty"`
	CancelledAt     *time.Time      `json:"cancelled_at,omitempty"`
	Checklist       json.RawMessage `json:"checklist"`
	ParentTaskID    *uuid.UUID      `json:"parent_task_id,omitempty"`
	BlockedByTaskID *uuid.UUID      `json:"blocked_by_task_id,omitempty"`
	CreatedBy       uuid.UUID       `json:"created_by"`
	Metadata        map[string]any  `json:"metadata"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

// TaskComment represents a comment on a task.
type TaskComment struct {
	ID            uuid.UUID   `json:"id"`
	TaskID        uuid.UUID   `json:"task_id"`
	AuthorID      uuid.UUID   `json:"author_id"`
	Body          string      `json:"body"`
	AttachmentIDs []uuid.UUID `json:"attachment_ids,omitempty"`
	IsInternal    bool        `json:"is_internal"`
	CreatedAt     time.Time   `json:"created_at"`
	UpdatedAt     time.Time   `json:"updated_at"`
	DeletedAt     *time.Time  `json:"deleted_at,omitempty"`
}

// TaskStatusHistory records an immutable audit trail of status transitions.
type TaskStatusHistory struct {
	ID         uuid.UUID `json:"id"`
	TaskID     uuid.UUID `json:"task_id"`
	FromStatus *string   `json:"from_status,omitempty"`
	ToStatus   string    `json:"to_status"`
	FromStage  *string   `json:"from_stage,omitempty"`
	ToStage    *string   `json:"to_stage,omitempty"`
	ChangedBy  uuid.UUID `json:"changed_by"`
	Reason     *string   `json:"reason,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}
