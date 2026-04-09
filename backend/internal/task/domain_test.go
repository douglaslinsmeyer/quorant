package task_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── TaskType serialization ───────────────────────────────────────────────────

func TestTaskType_JSONSerialization_SystemType(t *testing.T) {
	tt := task.TaskType{
		ID:                uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		Key:               "maintenance_request",
		Name:              "Maintenance Request",
		DefaultPriority:   "normal",
		WorkflowStages:    json.RawMessage(`[]`),
		ChecklistTemplate: json.RawMessage(`[]`),
		SourceModule:      "maintenance",
		IsActive:          true,
		CreatedAt:         time.Now().UTC(),
		UpdatedAt:         time.Now().UTC(),
	}

	data, err := json.Marshal(tt)
	require.NoError(t, err)

	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &result))

	// Required keys must be present
	for _, key := range []string{"id", "key", "name", "default_priority", "workflow_stages", "checklist_template", "source_module", "is_active", "created_at", "updated_at"} {
		assert.Contains(t, result, key, "expected JSON key %q to be present", key)
	}

	// org_id omitted when nil (system-defined)
	assert.NotContains(t, result, "org_id", "org_id should be omitted for system-defined types")
	assert.NotContains(t, result, "description", "description should be omitted when nil")
	assert.NotContains(t, result, "sla_hours", "sla_hours should be omitted when nil")
	assert.NotContains(t, result, "auto_assign_role", "auto_assign_role should be omitted when nil")
}

func TestTaskType_JSONSerialization_OrgType(t *testing.T) {
	orgID := uuid.New()
	desc := "Custom maintenance workflow"
	slaHours := 48
	role := "maintenance_staff"

	tt := task.TaskType{
		ID:                uuid.New(),
		OrgID:             &orgID,
		Key:               "custom_maintenance",
		Name:              "Custom Maintenance",
		Description:       &desc,
		DefaultPriority:   "high",
		SLAHours:          &slaHours,
		WorkflowStages:    json.RawMessage(`[{"name":"open"},{"name":"in_progress"}]`),
		ChecklistTemplate: json.RawMessage(`[{"item":"Inspect area"}]`),
		AutoAssignRole:    &role,
		SourceModule:      "maintenance",
		IsActive:          true,
		CreatedAt:         time.Now().UTC(),
		UpdatedAt:         time.Now().UTC(),
	}

	data, err := json.Marshal(tt)
	require.NoError(t, err)

	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &result))

	assert.Contains(t, result, "org_id")
	assert.Contains(t, result, "description")
	assert.Contains(t, result, "sla_hours")
	assert.Contains(t, result, "auto_assign_role")
}

// ─── Task serialization ───────────────────────────────────────────────────────

func TestTask_JSONSerialization_RequiredFields(t *testing.T) {
	now := time.Now().UTC()
	createdBy := uuid.New()

	tsk := task.Task{
		ID:           uuid.New(),
		OrgID:        uuid.New(),
		TaskTypeID:   uuid.New(),
		Title:        "Fix the leak",
		Status:       "open",
		Priority:     "normal",
		ResourceType: "unit",
		ResourceID:   uuid.New(),
		CreatedBy:    createdBy,
		Metadata:     map[string]any{},
		Checklist:    json.RawMessage(`[]`),
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	data, err := json.Marshal(tsk)
	require.NoError(t, err)

	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &result))

	for _, key := range []string{"id", "org_id", "task_type_id", "title", "status", "priority", "resource_type", "resource_id", "created_by", "metadata", "checklist", "created_at", "updated_at"} {
		assert.Contains(t, result, key, "expected JSON key %q", key)
	}

	// Optional nil fields should be omitted
	for _, key := range []string{"description", "current_stage", "unit_id", "assigned_to", "assigned_role", "assigned_at", "assigned_by", "due_at", "sla_deadline", "started_at", "completed_at", "cancelled_at", "parent_task_id", "blocked_by_task_id"} {
		assert.NotContains(t, result, key, "nil optional field %q should be omitted", key)
	}
}

func TestTask_JSONSerialization_OptionalFields(t *testing.T) {
	now := time.Now().UTC()
	desc := "Water stain on ceiling"
	stage := "in_progress"
	unitID := uuid.New()
	assignedTo := uuid.New()
	assignedRole := "maintenance_staff"
	assignedBy := uuid.New()
	dueAt := now.Add(24 * time.Hour)
	slaDeadline := now.Add(48 * time.Hour)
	parentID := uuid.New()

	tsk := task.Task{
		ID:              uuid.New(),
		OrgID:           uuid.New(),
		TaskTypeID:      uuid.New(),
		Title:           "Fix the leak",
		Description:     &desc,
		Status:          "in_progress",
		Priority:        "high",
		CurrentStage:    &stage,
		ResourceType:    "unit",
		ResourceID:      uuid.New(),
		UnitID:          &unitID,
		AssignedTo:      &assignedTo,
		AssignedRole:    &assignedRole,
		AssignedAt:      &now,
		AssignedBy:      &assignedBy,
		DueAt:           &dueAt,
		SLADeadline:     &slaDeadline,
		SLABreached:     false,
		StartedAt:       &now,
		ParentTaskID:    &parentID,
		CreatedBy:       uuid.New(),
		Metadata:        map[string]any{"ticket": "TKT-001"},
		Checklist:       json.RawMessage(`[{"done":false,"item":"Turn off water"}]`),
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	data, err := json.Marshal(tsk)
	require.NoError(t, err)

	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &result))

	for _, key := range []string{"description", "current_stage", "unit_id", "assigned_to", "assigned_role", "assigned_at", "assigned_by", "due_at", "sla_deadline", "started_at", "parent_task_id"} {
		assert.Contains(t, result, key, "expected optional field %q to be present when set", key)
	}
}

// ─── TaskComment serialization ────────────────────────────────────────────────

func TestTaskComment_JSONSerialization(t *testing.T) {
	now := time.Now().UTC()

	c := task.TaskComment{
		ID:         uuid.New(),
		TaskID:     uuid.New(),
		AuthorID:   uuid.New(),
		Body:       "Technician will arrive tomorrow",
		IsInternal: true,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	data, err := json.Marshal(c)
	require.NoError(t, err)

	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &result))

	for _, key := range []string{"id", "task_id", "author_id", "body", "is_internal", "created_at", "updated_at"} {
		assert.Contains(t, result, key)
	}

	// deleted_at and attachment_ids omitted when nil/empty
	assert.NotContains(t, result, "deleted_at")
	assert.NotContains(t, result, "attachment_ids")
}

func TestTaskComment_JSONSerialization_WithAttachments(t *testing.T) {
	now := time.Now().UTC()

	c := task.TaskComment{
		ID:            uuid.New(),
		TaskID:        uuid.New(),
		AuthorID:      uuid.New(),
		Body:          "See attached photo",
		AttachmentIDs: []uuid.UUID{uuid.New(), uuid.New()},
		IsInternal:    false,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	data, err := json.Marshal(c)
	require.NoError(t, err)

	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &result))

	attachments, ok := result["attachment_ids"]
	require.True(t, ok, "attachment_ids should be present when non-empty")
	arr, ok := attachments.([]interface{})
	require.True(t, ok)
	assert.Len(t, arr, 2)
}

// ─── TaskStatusHistory serialization ─────────────────────────────────────────

func TestTaskStatusHistory_JSONSerialization(t *testing.T) {
	now := time.Now().UTC()
	from := "open"

	h := task.TaskStatusHistory{
		ID:         uuid.New(),
		TaskID:     uuid.New(),
		FromStatus: &from,
		ToStatus:   "in_progress",
		ChangedBy:  uuid.New(),
		CreatedAt:  now,
	}

	data, err := json.Marshal(h)
	require.NoError(t, err)

	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &result))

	for _, key := range []string{"id", "task_id", "from_status", "to_status", "changed_by", "created_at"} {
		assert.Contains(t, result, key)
	}

	assert.NotContains(t, result, "from_stage")
	assert.NotContains(t, result, "to_stage")
	assert.NotContains(t, result, "reason")
}

// ─── Request validation ───────────────────────────────────────────────────────

func TestCreateTaskRequest_Validate_MissingTaskTypeID(t *testing.T) {
	req := task.CreateTaskRequest{
		Title:        "Fix leak",
		ResourceType: "unit",
		ResourceID:   uuid.New(),
	}
	err := req.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "task_type_id")
}

func TestCreateTaskRequest_Validate_MissingTitle(t *testing.T) {
	req := task.CreateTaskRequest{
		TaskTypeID:   uuid.New(),
		ResourceType: "unit",
		ResourceID:   uuid.New(),
	}
	err := req.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "title")
}

func TestCreateTaskRequest_Validate_MissingResourceType(t *testing.T) {
	req := task.CreateTaskRequest{
		TaskTypeID: uuid.New(),
		Title:      "Fix leak",
		ResourceID: uuid.New(),
	}
	err := req.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "resource_type")
}

func TestCreateTaskRequest_Validate_MissingResourceID(t *testing.T) {
	req := task.CreateTaskRequest{
		TaskTypeID:   uuid.New(),
		Title:        "Fix leak",
		ResourceType: "unit",
	}
	err := req.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "resource_id")
}

func TestCreateTaskRequest_Validate_InvalidPriority(t *testing.T) {
	badPriority := "critical"
	req := task.CreateTaskRequest{
		TaskTypeID:   uuid.New(),
		Title:        "Fix leak",
		ResourceType: "unit",
		ResourceID:   uuid.New(),
		Priority:     &badPriority,
	}
	err := req.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "priority")
}

func TestCreateTaskRequest_Validate_Valid(t *testing.T) {
	req := task.CreateTaskRequest{
		TaskTypeID:   uuid.New(),
		Title:        "Fix leak",
		ResourceType: "unit",
		ResourceID:   uuid.New(),
	}
	assert.NoError(t, req.Validate())
}

func TestCreateTaskTypeRequest_Validate_MissingKey(t *testing.T) {
	req := task.CreateTaskTypeRequest{
		Name:         "Maintenance",
		SourceModule: "maintenance",
	}
	err := req.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "key")
}

func TestCreateTaskTypeRequest_Validate_MissingName(t *testing.T) {
	req := task.CreateTaskTypeRequest{
		Key:          "maintenance",
		SourceModule: "maintenance",
	}
	err := req.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name")
}

func TestCreateTaskTypeRequest_Validate_MissingSourceModule(t *testing.T) {
	req := task.CreateTaskTypeRequest{
		Key:  "maintenance",
		Name: "Maintenance",
	}
	err := req.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "source_module")
}

func TestCreateTaskTypeRequest_Validate_InvalidDefaultPriority(t *testing.T) {
	bad := "extreme"
	req := task.CreateTaskTypeRequest{
		Key:             "maintenance",
		Name:            "Maintenance",
		SourceModule:    "maintenance",
		DefaultPriority: &bad,
	}
	err := req.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "default_priority")
}

func TestCreateTaskTypeRequest_Validate_Valid(t *testing.T) {
	req := task.CreateTaskTypeRequest{
		Key:          "maintenance_request",
		Name:         "Maintenance Request",
		SourceModule: "maintenance",
	}
	assert.NoError(t, req.Validate())
}

func TestAssignTaskRequest_Validate_BothNil(t *testing.T) {
	req := task.AssignTaskRequest{}
	err := req.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "assigned_to")
}

func TestAssignTaskRequest_Validate_AssignedToSet(t *testing.T) {
	id := uuid.New()
	req := task.AssignTaskRequest{AssignedTo: &id}
	assert.NoError(t, req.Validate())
}

func TestAssignTaskRequest_Validate_AssignedRoleSet(t *testing.T) {
	role := "maintenance_staff"
	req := task.AssignTaskRequest{AssignedRole: &role}
	assert.NoError(t, req.Validate())
}

func TestTransitionTaskRequest_Validate_MissingStatus(t *testing.T) {
	req := task.TransitionTaskRequest{}
	err := req.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status")
}

func TestTransitionTaskRequest_Validate_InvalidStatus(t *testing.T) {
	req := task.TransitionTaskRequest{Status: "pending"}
	err := req.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status")
}

func TestTransitionTaskRequest_Validate_ValidStatuses(t *testing.T) {
	validStatuses := []string{"open", "assigned", "in_progress", "blocked", "review", "completed", "cancelled"}
	for _, s := range validStatuses {
		req := task.TransitionTaskRequest{Status: s}
		assert.NoError(t, req.Validate(), "status %q should be valid", s)
	}
}

func TestAddCommentRequest_Validate_MissingBody(t *testing.T) {
	req := task.AddCommentRequest{}
	err := req.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "body")
}

func TestAddCommentRequest_Validate_Valid(t *testing.T) {
	req := task.AddCommentRequest{Body: "Technician dispatched", IsInternal: true}
	assert.NoError(t, req.Validate())
}
