package task_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Test server setup
// ---------------------------------------------------------------------------

type taskTestServer struct {
	server *httptest.Server
	repo   *mockTaskRepo
}

func setupTaskTestServer(t *testing.T) *taskTestServer {
	t.Helper()

	repo := newMockTaskRepo()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := task.NewTaskService(repo, logger)
	handler := task.NewTaskHandler(svc, logger)

	mux := http.NewServeMux()

	// Cross-org routes (no org_id in path)
	mux.HandleFunc("GET /tasks", handler.ListMyTasks)
	mux.HandleFunc("GET /tasks/dashboard", handler.Dashboard)

	// Org-scoped routes
	mux.HandleFunc("POST /organizations/{org_id}/tasks", handler.Create)
	mux.HandleFunc("GET /organizations/{org_id}/tasks", handler.List)
	mux.HandleFunc("GET /organizations/{org_id}/tasks/{task_id}", handler.Get)
	mux.HandleFunc("PATCH /organizations/{org_id}/tasks/{task_id}", handler.Update)
	mux.HandleFunc("POST /organizations/{org_id}/tasks/{task_id}/assign", handler.Assign)
	mux.HandleFunc("POST /organizations/{org_id}/tasks/{task_id}/transition", handler.Transition)
	mux.HandleFunc("POST /organizations/{org_id}/tasks/{task_id}/comments", handler.AddComment)
	mux.HandleFunc("PATCH /organizations/{org_id}/tasks/{task_id}/checklist/{item_id}", handler.ToggleChecklist)
	mux.HandleFunc("GET /organizations/{org_id}/task-types", handler.ListTypes)
	mux.HandleFunc("POST /organizations/{org_id}/task-types", handler.CreateType)
	mux.HandleFunc("PATCH /organizations/{org_id}/task-types/{type_id}", handler.UpdateType)

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	return &taskTestServer{server: server, repo: repo}
}

// ---------------------------------------------------------------------------
// HTTP helpers
// ---------------------------------------------------------------------------

func doTaskRequest(t *testing.T, serverURL, method, path string, body any) *http.Response {
	t.Helper()

	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		require.NoError(t, err)
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, serverURL+path, bodyReader)
	require.NoError(t, err)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

func decodeTaskBody(t *testing.T, resp *http.Response, dst any) {
	t.Helper()
	defer resp.Body.Close()
	require.NoError(t, json.NewDecoder(resp.Body).Decode(dst))
}

func seedHandlerTask(t *testing.T, repo *mockTaskRepo, orgID uuid.UUID) *task.Task {
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
// TestListMyTasks_Handler
// ---------------------------------------------------------------------------

func TestListMyTasks_Handler(t *testing.T) {
	ts := setupTaskTestServer(t)
	orgID := uuid.New()
	userID := uuid.New()

	// Seed a task assigned to userID.
	tsk := seedHandlerTask(t, ts.repo, orgID)
	tsk.AssignedTo = &userID
	ts.repo.tasks[tsk.ID] = tsk

	resp := doTaskRequest(t, ts.server.URL, http.MethodGet, "/tasks", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Decode raw envelope — data may be null/empty array (no auth in test, callerID = uuid.Nil).
	var envelope map[string]json.RawMessage
	decodeTaskBody(t, resp, &envelope)
	// Verify we got a valid JSON response envelope.
	assert.Contains(t, envelope, "data", "response should contain a data key")
}

// ---------------------------------------------------------------------------
// TestDashboard_Handler
// ---------------------------------------------------------------------------

func TestDashboard_Handler(t *testing.T) {
	ts := setupTaskTestServer(t)

	resp := doTaskRequest(t, ts.server.URL, http.MethodGet, "/tasks/dashboard", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data map[string]string `json:"data"`
	}
	decodeTaskBody(t, resp, &envelope)
	assert.Equal(t, "dashboard coming soon", envelope.Data["message"])
}

// ---------------------------------------------------------------------------
// TestCreateTask_Handler
// ---------------------------------------------------------------------------

func TestCreateTask_Handler(t *testing.T) {
	ts := setupTaskTestServer(t)
	orgID := uuid.New()
	ttID := uuid.New()

	body := map[string]any{
		"task_type_id":  ttID,
		"title":         "Inspect HVAC unit",
		"resource_type": "unit",
		"resource_id":   uuid.New(),
	}

	resp := doTaskRequest(t, ts.server.URL, http.MethodPost,
		fmt.Sprintf("/organizations/%s/tasks", orgID), body)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var envelope struct {
		Data *task.Task `json:"data"`
	}
	decodeTaskBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, orgID, envelope.Data.OrgID)
	assert.Equal(t, "Inspect HVAC unit", envelope.Data.Title)
	assert.Equal(t, "open", envelope.Data.Status)
	assert.NotEqual(t, uuid.Nil, envelope.Data.ID)
}

// ---------------------------------------------------------------------------
// TestCreateTask_Handler_ValidationError
// ---------------------------------------------------------------------------

func TestCreateTask_Handler_ValidationError(t *testing.T) {
	ts := setupTaskTestServer(t)
	orgID := uuid.New()

	// Missing required task_type_id.
	body := map[string]any{
		"title":         "Bad task",
		"resource_type": "unit",
		"resource_id":   uuid.New(),
	}

	resp := doTaskRequest(t, ts.server.URL, http.MethodPost,
		fmt.Sprintf("/organizations/%s/tasks", orgID), body)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// ---------------------------------------------------------------------------
// TestListTasks_Handler
// ---------------------------------------------------------------------------

func TestListTasks_Handler(t *testing.T) {
	ts := setupTaskTestServer(t)
	orgID := uuid.New()
	seedHandlerTask(t, ts.repo, orgID)
	seedHandlerTask(t, ts.repo, orgID)

	resp := doTaskRequest(t, ts.server.URL, http.MethodGet,
		fmt.Sprintf("/organizations/%s/tasks", orgID), nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data []task.Task `json:"data"`
	}
	decodeTaskBody(t, resp, &envelope)
	assert.Len(t, envelope.Data, 2)
}

// ---------------------------------------------------------------------------
// TestGetTask_Handler_Success
// ---------------------------------------------------------------------------

func TestGetTask_Handler_Success(t *testing.T) {
	ts := setupTaskTestServer(t)
	orgID := uuid.New()
	tsk := seedHandlerTask(t, ts.repo, orgID)

	resp := doTaskRequest(t, ts.server.URL, http.MethodGet,
		fmt.Sprintf("/organizations/%s/tasks/%s", orgID, tsk.ID), nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data *task.Task `json:"data"`
	}
	decodeTaskBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, tsk.ID, envelope.Data.ID)
}

// ---------------------------------------------------------------------------
// TestGetTask_Handler_NotFound
// ---------------------------------------------------------------------------

func TestGetTask_Handler_NotFound(t *testing.T) {
	ts := setupTaskTestServer(t)
	orgID := uuid.New()

	resp := doTaskRequest(t, ts.server.URL, http.MethodGet,
		fmt.Sprintf("/organizations/%s/tasks/%s", orgID, uuid.New()), nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// ---------------------------------------------------------------------------
// TestAssignTask_Handler
// ---------------------------------------------------------------------------

func TestAssignTask_Handler(t *testing.T) {
	ts := setupTaskTestServer(t)
	orgID := uuid.New()
	tsk := seedHandlerTask(t, ts.repo, orgID)
	assignedTo := uuid.New()

	body := map[string]any{
		"assigned_to": assignedTo,
	}

	resp := doTaskRequest(t, ts.server.URL, http.MethodPost,
		fmt.Sprintf("/organizations/%s/tasks/%s/assign", orgID, tsk.ID), body)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data *task.Task `json:"data"`
	}
	decodeTaskBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	require.NotNil(t, envelope.Data.AssignedTo)
	assert.Equal(t, assignedTo, *envelope.Data.AssignedTo)
}

// ---------------------------------------------------------------------------
// TestTransitionTask_Handler
// ---------------------------------------------------------------------------

func TestTransitionTask_Handler(t *testing.T) {
	ts := setupTaskTestServer(t)
	orgID := uuid.New()
	tsk := seedHandlerTask(t, ts.repo, orgID)

	body := map[string]any{
		"status": "in_progress",
	}

	resp := doTaskRequest(t, ts.server.URL, http.MethodPost,
		fmt.Sprintf("/organizations/%s/tasks/%s/transition", orgID, tsk.ID), body)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data *task.Task `json:"data"`
	}
	decodeTaskBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, "in_progress", envelope.Data.Status)
	assert.NotNil(t, envelope.Data.StartedAt)
}

// ---------------------------------------------------------------------------
// TestTransitionTask_Handler_InvalidTransition
// ---------------------------------------------------------------------------

func TestTransitionTask_Handler_InvalidTransition(t *testing.T) {
	ts := setupTaskTestServer(t)
	orgID := uuid.New()
	tsk := seedHandlerTask(t, ts.repo, orgID)
	tsk.Status = "completed"
	ts.repo.tasks[tsk.ID] = tsk

	body := map[string]any{
		"status": "open",
	}

	resp := doTaskRequest(t, ts.server.URL, http.MethodPost,
		fmt.Sprintf("/organizations/%s/tasks/%s/transition", orgID, tsk.ID), body)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// ---------------------------------------------------------------------------
// TestAddComment_Handler
// ---------------------------------------------------------------------------

func TestAddComment_Handler(t *testing.T) {
	ts := setupTaskTestServer(t)
	orgID := uuid.New()
	tsk := seedHandlerTask(t, ts.repo, orgID)

	body := map[string]any{
		"body":        "Technician arriving at 2pm",
		"is_internal": true,
	}

	resp := doTaskRequest(t, ts.server.URL, http.MethodPost,
		fmt.Sprintf("/organizations/%s/tasks/%s/comments", orgID, tsk.ID), body)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var envelope struct {
		Data *task.TaskComment `json:"data"`
	}
	decodeTaskBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, tsk.ID, envelope.Data.TaskID)
	assert.Equal(t, "Technician arriving at 2pm", envelope.Data.Body)
	assert.True(t, envelope.Data.IsInternal)
}

// ---------------------------------------------------------------------------
// TestAddComment_Handler_MissingBody
// ---------------------------------------------------------------------------

func TestAddComment_Handler_MissingBody(t *testing.T) {
	ts := setupTaskTestServer(t)
	orgID := uuid.New()
	tsk := seedHandlerTask(t, ts.repo, orgID)

	body := map[string]any{
		"is_internal": false,
	}

	resp := doTaskRequest(t, ts.server.URL, http.MethodPost,
		fmt.Sprintf("/organizations/%s/tasks/%s/comments", orgID, tsk.ID), body)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// ---------------------------------------------------------------------------
// TestListTaskTypes_Handler
// ---------------------------------------------------------------------------

func TestListTaskTypes_Handler(t *testing.T) {
	ts := setupTaskTestServer(t)
	orgID := uuid.New()

	// Seed a system-level task type (nil OrgID).
	tt := &task.TaskType{
		ID:                uuid.New(),
		OrgID:             nil,
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
	ts.repo.taskTypes[tt.ID] = tt

	resp := doTaskRequest(t, ts.server.URL, http.MethodGet,
		fmt.Sprintf("/organizations/%s/task-types", orgID), nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data []task.TaskType `json:"data"`
	}
	decodeTaskBody(t, resp, &envelope)
	assert.Len(t, envelope.Data, 1)
}

// ---------------------------------------------------------------------------
// TestCreateTaskType_Handler
// ---------------------------------------------------------------------------

func TestCreateTaskType_Handler(t *testing.T) {
	ts := setupTaskTestServer(t)
	orgID := uuid.New()

	body := map[string]any{
		"key":           "custom_maintenance",
		"name":          "Custom Maintenance",
		"source_module": "maintenance",
	}

	resp := doTaskRequest(t, ts.server.URL, http.MethodPost,
		fmt.Sprintf("/organizations/%s/task-types", orgID), body)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var envelope struct {
		Data *task.TaskType `json:"data"`
	}
	decodeTaskBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, "custom_maintenance", envelope.Data.Key)
	assert.Equal(t, "Custom Maintenance", envelope.Data.Name)
	assert.NotEqual(t, uuid.Nil, envelope.Data.ID)
}
