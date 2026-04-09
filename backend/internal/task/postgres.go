package task

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresTaskRepository implements TaskRepository using a pgxpool.
type PostgresTaskRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresTaskRepository creates a new PostgresTaskRepository backed by pool.
func NewPostgresTaskRepository(pool *pgxpool.Pool) *PostgresTaskRepository {
	return &PostgresTaskRepository{pool: pool}
}

// ─── Task Types ───────────────────────────────────────────────────────────────

// CreateTaskType inserts a new task type and returns the persisted record.
func (r *PostgresTaskRepository) CreateTaskType(ctx context.Context, tt *TaskType) (*TaskType, error) {
	const q = `
		INSERT INTO task_types (
			org_id, key, name, description, default_priority, sla_hours,
			workflow_stages, checklist_template, auto_assign_role, source_module, is_active
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, org_id, key, name, description, default_priority, sla_hours,
		          workflow_stages, checklist_template, auto_assign_role, source_module,
		          is_active, created_at, updated_at`

	row := r.pool.QueryRow(ctx, q,
		tt.OrgID,
		tt.Key,
		tt.Name,
		tt.Description,
		tt.DefaultPriority,
		tt.SLAHours,
		tt.WorkflowStages,
		tt.ChecklistTemplate,
		tt.AutoAssignRole,
		tt.SourceModule,
		tt.IsActive,
	)
	result, err := scanTaskType(row)
	if err != nil {
		return nil, fmt.Errorf("task: CreateTaskType: %w", err)
	}
	return result, nil
}

// ListTaskTypesByOrg returns system-defined types plus org-specific types.
func (r *PostgresTaskRepository) ListTaskTypesByOrg(ctx context.Context, orgID uuid.UUID) ([]TaskType, error) {
	const q = `
		SELECT id, org_id, key, name, description, default_priority, sla_hours,
		       workflow_stages, checklist_template, auto_assign_role, source_module,
		       is_active, created_at, updated_at
		FROM task_types
		WHERE (org_id IS NULL OR org_id = $1)
		  AND is_active = TRUE
		ORDER BY source_module, key`

	rows, err := r.pool.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("task: ListTaskTypesByOrg: %w", err)
	}
	defer rows.Close()

	return collectTaskTypes(rows, "ListTaskTypesByOrg")
}

// UpdateTaskType updates a task type and returns the updated record.
func (r *PostgresTaskRepository) UpdateTaskType(ctx context.Context, tt *TaskType) (*TaskType, error) {
	const q = `
		UPDATE task_types
		SET name               = $2,
		    description        = $3,
		    default_priority   = $4,
		    sla_hours          = $5,
		    workflow_stages    = $6,
		    checklist_template = $7,
		    auto_assign_role   = $8,
		    is_active          = $9,
		    updated_at         = now()
		WHERE id = $1
		RETURNING id, org_id, key, name, description, default_priority, sla_hours,
		          workflow_stages, checklist_template, auto_assign_role, source_module,
		          is_active, created_at, updated_at`

	row := r.pool.QueryRow(ctx, q,
		tt.ID,
		tt.Name,
		tt.Description,
		tt.DefaultPriority,
		tt.SLAHours,
		tt.WorkflowStages,
		tt.ChecklistTemplate,
		tt.AutoAssignRole,
		tt.IsActive,
	)
	result, err := scanTaskType(row)
	if err != nil {
		return nil, fmt.Errorf("task: UpdateTaskType: %w", err)
	}
	return result, nil
}

// ─── Tasks ────────────────────────────────────────────────────────────────────

// CreateTask inserts a new task and returns the persisted record.
func (r *PostgresTaskRepository) CreateTask(ctx context.Context, t *Task) (*Task, error) {
	metadataJSON, err := json.Marshal(t.Metadata)
	if err != nil {
		return nil, fmt.Errorf("task: CreateTask: marshal metadata: %w", err)
	}

	const q = `
		INSERT INTO tasks (
			org_id, task_type_id, title, description, status, priority,
			current_stage, resource_type, resource_id, unit_id,
			assigned_to, assigned_role, assigned_at, assigned_by,
			due_at, sla_deadline, sla_breached,
			started_at, completed_at, cancelled_at,
			checklist, parent_task_id, blocked_by_task_id,
			created_by, metadata
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10,
			$11, $12, $13, $14,
			$15, $16, $17,
			$18, $19, $20,
			$21, $22, $23,
			$24, $25
		)
		RETURNING id, org_id, task_type_id, title, description, status, priority,
		          current_stage, resource_type, resource_id, unit_id,
		          assigned_to, assigned_role, assigned_at, assigned_by,
		          due_at, sla_deadline, sla_breached,
		          started_at, completed_at, cancelled_at,
		          checklist, parent_task_id, blocked_by_task_id,
		          created_by, metadata, created_at, updated_at`

	row := r.pool.QueryRow(ctx, q,
		t.OrgID, t.TaskTypeID, t.Title, t.Description, t.Status, t.Priority,
		t.CurrentStage, t.ResourceType, t.ResourceID, t.UnitID,
		t.AssignedTo, t.AssignedRole, t.AssignedAt, t.AssignedBy,
		t.DueAt, t.SLADeadline, t.SLABreached,
		t.StartedAt, t.CompletedAt, t.CancelledAt,
		t.Checklist, t.ParentTaskID, t.BlockedByTaskID,
		t.CreatedBy, metadataJSON,
	)
	result, err := scanTask(row)
	if err != nil {
		return nil, fmt.Errorf("task: CreateTask: %w", err)
	}
	return result, nil
}

// FindTaskByID returns the task with the given ID, or nil if not found.
func (r *PostgresTaskRepository) FindTaskByID(ctx context.Context, id uuid.UUID) (*Task, error) {
	const q = `
		SELECT id, org_id, task_type_id, title, description, status, priority,
		       current_stage, resource_type, resource_id, unit_id,
		       assigned_to, assigned_role, assigned_at, assigned_by,
		       due_at, sla_deadline, sla_breached,
		       started_at, completed_at, cancelled_at,
		       checklist, parent_task_id, blocked_by_task_id,
		       created_by, metadata, created_at, updated_at
		FROM tasks
		WHERE id = $1`

	row := r.pool.QueryRow(ctx, q, id)
	result, err := scanTask(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("task: FindTaskByID: %w", err)
	}
	return result, nil
}

// ListTasksByOrg returns tasks belonging to an organization, supporting
// cursor-based pagination ordered by id DESC.
// afterID is the cursor from the previous page; hasMore is true when more items exist.
func (r *PostgresTaskRepository) ListTasksByOrg(ctx context.Context, orgID uuid.UUID, limit int, afterID *uuid.UUID) ([]Task, bool, error) {
	const q = `
		SELECT id, org_id, task_type_id, title, description, status, priority,
		       current_stage, resource_type, resource_id, unit_id,
		       assigned_to, assigned_role, assigned_at, assigned_by,
		       due_at, sla_deadline, sla_breached,
		       started_at, completed_at, cancelled_at,
		       checklist, parent_task_id, blocked_by_task_id,
		       created_by, metadata, created_at, updated_at
		FROM tasks
		WHERE org_id = $1
		  AND ($3::uuid IS NULL OR id < $3)
		ORDER BY id DESC
		LIMIT $2`

	rows, err := r.pool.Query(ctx, q, orgID, limit+1, afterID)
	if err != nil {
		return nil, false, fmt.Errorf("task: ListTasksByOrg: %w", err)
	}
	defer rows.Close()

	results, err := collectTasks(rows, "ListTasksByOrg")
	if err != nil {
		return nil, false, err
	}

	hasMore := len(results) > limit
	if hasMore {
		results = results[:limit]
	}
	return results, hasMore, nil
}

// ListTasksByAssignee returns all tasks assigned to a user across orgs.
func (r *PostgresTaskRepository) ListTasksByAssignee(ctx context.Context, userID uuid.UUID) ([]Task, error) {
	const q = `
		SELECT id, org_id, task_type_id, title, description, status, priority,
		       current_stage, resource_type, resource_id, unit_id,
		       assigned_to, assigned_role, assigned_at, assigned_by,
		       due_at, sla_deadline, sla_breached,
		       started_at, completed_at, cancelled_at,
		       checklist, parent_task_id, blocked_by_task_id,
		       created_by, metadata, created_at, updated_at
		FROM tasks
		WHERE assigned_to = $1
		ORDER BY priority DESC, created_at DESC`

	rows, err := r.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("task: ListTasksByAssignee: %w", err)
	}
	defer rows.Close()

	return collectTasks(rows, "ListTasksByAssignee")
}

// ListTasksByResource returns all tasks linked to a specific resource.
func (r *PostgresTaskRepository) ListTasksByResource(ctx context.Context, resourceType string, resourceID uuid.UUID) ([]Task, error) {
	const q = `
		SELECT id, org_id, task_type_id, title, description, status, priority,
		       current_stage, resource_type, resource_id, unit_id,
		       assigned_to, assigned_role, assigned_at, assigned_by,
		       due_at, sla_deadline, sla_breached,
		       started_at, completed_at, cancelled_at,
		       checklist, parent_task_id, blocked_by_task_id,
		       created_by, metadata, created_at, updated_at
		FROM tasks
		WHERE resource_type = $1
		  AND resource_id = $2
		ORDER BY created_at DESC`

	rows, err := r.pool.Query(ctx, q, resourceType, resourceID)
	if err != nil {
		return nil, fmt.Errorf("task: ListTasksByResource: %w", err)
	}
	defer rows.Close()

	return collectTasks(rows, "ListTasksByResource")
}

// UpdateTask updates a task record and returns the updated record.
func (r *PostgresTaskRepository) UpdateTask(ctx context.Context, t *Task) (*Task, error) {
	metadataJSON, err := json.Marshal(t.Metadata)
	if err != nil {
		return nil, fmt.Errorf("task: UpdateTask: marshal metadata: %w", err)
	}

	const q = `
		UPDATE tasks
		SET title              = $2,
		    description        = $3,
		    status             = $4,
		    priority           = $5,
		    current_stage      = $6,
		    assigned_to        = $7,
		    assigned_role      = $8,
		    assigned_at        = $9,
		    assigned_by        = $10,
		    due_at             = $11,
		    sla_deadline       = $12,
		    sla_breached       = $13,
		    started_at         = $14,
		    completed_at       = $15,
		    cancelled_at       = $16,
		    checklist          = $17,
		    parent_task_id     = $18,
		    blocked_by_task_id = $19,
		    metadata           = $20,
		    updated_at         = now()
		WHERE id = $1
		RETURNING id, org_id, task_type_id, title, description, status, priority,
		          current_stage, resource_type, resource_id, unit_id,
		          assigned_to, assigned_role, assigned_at, assigned_by,
		          due_at, sla_deadline, sla_breached,
		          started_at, completed_at, cancelled_at,
		          checklist, parent_task_id, blocked_by_task_id,
		          created_by, metadata, created_at, updated_at`

	row := r.pool.QueryRow(ctx, q,
		t.ID, t.Title, t.Description, t.Status, t.Priority,
		t.CurrentStage, t.AssignedTo, t.AssignedRole, t.AssignedAt, t.AssignedBy,
		t.DueAt, t.SLADeadline, t.SLABreached,
		t.StartedAt, t.CompletedAt, t.CancelledAt,
		t.Checklist, t.ParentTaskID, t.BlockedByTaskID,
		metadataJSON,
	)
	result, err := scanTask(row)
	if err != nil {
		return nil, fmt.Errorf("task: UpdateTask: %w", err)
	}
	return result, nil
}

// ─── Comments ─────────────────────────────────────────────────────────────────

// CreateComment inserts a comment and returns the persisted record.
func (r *PostgresTaskRepository) CreateComment(ctx context.Context, c *TaskComment) (*TaskComment, error) {
	const q = `
		INSERT INTO task_comments (task_id, author_id, body, attachment_ids, is_internal)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, task_id, author_id, body, attachment_ids, is_internal,
		          created_at, updated_at, deleted_at`

	attachmentIDs := c.AttachmentIDs
	if attachmentIDs == nil {
		attachmentIDs = []uuid.UUID{}
	}

	row := r.pool.QueryRow(ctx, q,
		c.TaskID, c.AuthorID, c.Body, attachmentIDs, c.IsInternal,
	)
	result, err := scanComment(row)
	if err != nil {
		return nil, fmt.Errorf("task: CreateComment: %w", err)
	}
	return result, nil
}

// ListCommentsByTask returns all non-deleted comments for a task ordered by created_at.
func (r *PostgresTaskRepository) ListCommentsByTask(ctx context.Context, taskID uuid.UUID) ([]TaskComment, error) {
	const q = `
		SELECT id, task_id, author_id, body, attachment_ids, is_internal,
		       created_at, updated_at, deleted_at
		FROM task_comments
		WHERE task_id = $1
		  AND deleted_at IS NULL
		ORDER BY created_at ASC`

	rows, err := r.pool.Query(ctx, q, taskID)
	if err != nil {
		return nil, fmt.Errorf("task: ListCommentsByTask: %w", err)
	}
	defer rows.Close()

	var comments []TaskComment
	for rows.Next() {
		c, err := scanCommentRow(rows)
		if err != nil {
			return nil, fmt.Errorf("task: ListCommentsByTask scan: %w", err)
		}
		comments = append(comments, *c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("task: ListCommentsByTask rows: %w", err)
	}
	return comments, nil
}

// ─── Status History ───────────────────────────────────────────────────────────

// CreateStatusHistory inserts an immutable status transition record.
func (r *PostgresTaskRepository) CreateStatusHistory(ctx context.Context, h *TaskStatusHistory) (*TaskStatusHistory, error) {
	const q = `
		INSERT INTO task_status_history (task_id, from_status, to_status, from_stage, to_stage, changed_by, reason)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, task_id, from_status, to_status, from_stage, to_stage, changed_by, reason, created_at`

	row := r.pool.QueryRow(ctx, q,
		h.TaskID, h.FromStatus, h.ToStatus, h.FromStage, h.ToStage, h.ChangedBy, h.Reason,
	)
	result, err := scanStatusHistory(row)
	if err != nil {
		return nil, fmt.Errorf("task: CreateStatusHistory: %w", err)
	}
	return result, nil
}

// ListStatusHistoryByTask returns status history for a task ordered by created_at.
func (r *PostgresTaskRepository) ListStatusHistoryByTask(ctx context.Context, taskID uuid.UUID) ([]TaskStatusHistory, error) {
	const q = `
		SELECT id, task_id, from_status, to_status, from_stage, to_stage, changed_by, reason, created_at
		FROM task_status_history
		WHERE task_id = $1
		ORDER BY created_at ASC`

	rows, err := r.pool.Query(ctx, q, taskID)
	if err != nil {
		return nil, fmt.Errorf("task: ListStatusHistoryByTask: %w", err)
	}
	defer rows.Close()

	var history []TaskStatusHistory
	for rows.Next() {
		var h TaskStatusHistory
		if err := rows.Scan(
			&h.ID, &h.TaskID, &h.FromStatus, &h.ToStatus,
			&h.FromStage, &h.ToStage, &h.ChangedBy, &h.Reason, &h.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("task: ListStatusHistoryByTask scan: %w", err)
		}
		history = append(history, h)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("task: ListStatusHistoryByTask rows: %w", err)
	}
	return history, nil
}

// ─── Scan helpers ─────────────────────────────────────────────────────────────

func scanTaskType(row pgx.Row) (*TaskType, error) {
	var tt TaskType
	err := row.Scan(
		&tt.ID, &tt.OrgID, &tt.Key, &tt.Name, &tt.Description,
		&tt.DefaultPriority, &tt.SLAHours,
		&tt.WorkflowStages, &tt.ChecklistTemplate,
		&tt.AutoAssignRole, &tt.SourceModule,
		&tt.IsActive, &tt.CreatedAt, &tt.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &tt, nil
}

func collectTaskTypes(rows pgx.Rows, caller string) ([]TaskType, error) {
	var types []TaskType
	for rows.Next() {
		var tt TaskType
		if err := rows.Scan(
			&tt.ID, &tt.OrgID, &tt.Key, &tt.Name, &tt.Description,
			&tt.DefaultPriority, &tt.SLAHours,
			&tt.WorkflowStages, &tt.ChecklistTemplate,
			&tt.AutoAssignRole, &tt.SourceModule,
			&tt.IsActive, &tt.CreatedAt, &tt.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("task: %s scan: %w", caller, err)
		}
		types = append(types, tt)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("task: %s rows: %w", caller, err)
	}
	return types, nil
}

func scanTask(row pgx.Row) (*Task, error) {
	var t Task
	var metadataJSON []byte
	err := row.Scan(
		&t.ID, &t.OrgID, &t.TaskTypeID, &t.Title, &t.Description,
		&t.Status, &t.Priority, &t.CurrentStage,
		&t.ResourceType, &t.ResourceID, &t.UnitID,
		&t.AssignedTo, &t.AssignedRole, &t.AssignedAt, &t.AssignedBy,
		&t.DueAt, &t.SLADeadline, &t.SLABreached,
		&t.StartedAt, &t.CompletedAt, &t.CancelledAt,
		&t.Checklist, &t.ParentTaskID, &t.BlockedByTaskID,
		&t.CreatedBy, &metadataJSON,
		&t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(metadataJSON, &t.Metadata); err != nil {
		return nil, fmt.Errorf("unmarshal metadata: %w", err)
	}
	return &t, nil
}

func collectTasks(rows pgx.Rows, caller string) ([]Task, error) {
	var tasks []Task
	for rows.Next() {
		var t Task
		var metadataJSON []byte
		if err := rows.Scan(
			&t.ID, &t.OrgID, &t.TaskTypeID, &t.Title, &t.Description,
			&t.Status, &t.Priority, &t.CurrentStage,
			&t.ResourceType, &t.ResourceID, &t.UnitID,
			&t.AssignedTo, &t.AssignedRole, &t.AssignedAt, &t.AssignedBy,
			&t.DueAt, &t.SLADeadline, &t.SLABreached,
			&t.StartedAt, &t.CompletedAt, &t.CancelledAt,
			&t.Checklist, &t.ParentTaskID, &t.BlockedByTaskID,
			&t.CreatedBy, &metadataJSON,
			&t.CreatedAt, &t.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("task: %s scan: %w", caller, err)
		}
		if err := json.Unmarshal(metadataJSON, &t.Metadata); err != nil {
			return nil, fmt.Errorf("task: %s unmarshal metadata: %w", caller, err)
		}
		tasks = append(tasks, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("task: %s rows: %w", caller, err)
	}
	return tasks, nil
}

func scanComment(row pgx.Row) (*TaskComment, error) {
	var c TaskComment
	err := row.Scan(
		&c.ID, &c.TaskID, &c.AuthorID, &c.Body,
		&c.AttachmentIDs, &c.IsInternal,
		&c.CreatedAt, &c.UpdatedAt, &c.DeletedAt,
	)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func scanCommentRow(rows pgx.Rows) (*TaskComment, error) {
	var c TaskComment
	err := rows.Scan(
		&c.ID, &c.TaskID, &c.AuthorID, &c.Body,
		&c.AttachmentIDs, &c.IsInternal,
		&c.CreatedAt, &c.UpdatedAt, &c.DeletedAt,
	)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func scanStatusHistory(row pgx.Row) (*TaskStatusHistory, error) {
	var h TaskStatusHistory
	err := row.Scan(
		&h.ID, &h.TaskID, &h.FromStatus, &h.ToStatus,
		&h.FromStage, &h.ToStage, &h.ChangedBy, &h.Reason, &h.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &h, nil
}
