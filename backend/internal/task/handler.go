package task

import (
	"log/slog"
	"time"
	"net/http"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/platform/api"
	"github.com/quorant/quorant/internal/platform/middleware"
)

// TaskHandler handles HTTP requests for the task module.
type TaskHandler struct {
	service Service
	logger  *slog.Logger
}

// NewTaskHandler constructs a TaskHandler backed by the given service.
func NewTaskHandler(service Service, logger *slog.Logger) *TaskHandler {
	return &TaskHandler{service: service, logger: logger}
}

// ---------------------------------------------------------------------------
// Cross-org / user-scoped endpoints
// ---------------------------------------------------------------------------

// ListMyTasks handles GET /api/v1/tasks — returns tasks assigned to the caller.
func (h *TaskHandler) ListMyTasks(w http.ResponseWriter, r *http.Request) {
	userID := callerID(r)

	tasks, err := h.service.ListMyTasks(r.Context(), userID)
	if err != nil {
		h.logger.Error("ListMyTasks failed", "user_id", userID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, tasks)
}

// Dashboard handles GET /api/v1/tasks/dashboard — aggregated task metrics.
func (h *TaskHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	userID := callerID(r)

	tasks, err := h.service.ListMyTasks(r.Context(), userID)
	if err != nil {
		api.WriteError(w, api.NewInternalError(err))
		return
	}

	// Aggregate counts by status
	counts := map[string]int{
		"open": 0, "assigned": 0, "in_progress": 0, "blocked": 0,
		"review": 0, "completed": 0, "cancelled": 0,
	}
	overdue := 0
	slaBreach := 0
	for _, t := range tasks {
		counts[t.Status]++
		if t.DueAt != nil && t.DueAt.Before(time.Now()) && t.Status != "completed" && t.Status != "cancelled" {
			overdue++
		}
		if t.SLABreached {
			slaBreach++
		}
	}

	api.WriteJSON(w, http.StatusOK, map[string]any{
		"total":         len(tasks),
		"by_status":     counts,
		"overdue":       overdue,
		"sla_breached":  slaBreach,
	})
}

// ---------------------------------------------------------------------------
// Org-scoped task endpoints
// ---------------------------------------------------------------------------

// Create handles POST /api/v1/organizations/{org_id}/tasks.
func (h *TaskHandler) Create(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseTaskPathUUID(r, "org_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var req CreateTaskRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}

	created, err := h.service.CreateTask(r.Context(), orgID, req, callerID(r))
	if err != nil {
		h.logger.Error("CreateTask failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusCreated, created)
}

// List handles GET /api/v1/organizations/{org_id}/tasks.
func (h *TaskHandler) List(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseTaskPathUUID(r, "org_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	page := api.ParsePageRequest(r)
	afterID, err := parseTaskCursorID(page.Cursor)
	if err != nil {
		api.WriteError(w, api.NewValidationError("validation.invalid_cursor", "cursor"))
		return
	}

	tasks, hasMore, err := h.service.ListTasks(r.Context(), orgID, page.Limit, afterID)
	if err != nil {
		h.logger.Error("ListTasks failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	var meta *api.Meta
	if hasMore && len(tasks) > 0 {
		meta = &api.Meta{
			Cursor:  api.EncodeCursor(map[string]string{"id": tasks[len(tasks)-1].ID.String()}),
			HasMore: true,
		}
	}

	api.WriteJSONWithMeta(w, http.StatusOK, tasks, meta)
}

// Get handles GET /api/v1/organizations/{org_id}/tasks/{task_id}.
func (h *TaskHandler) Get(w http.ResponseWriter, r *http.Request) {
	taskID, err := parseTaskPathUUID(r, "task_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	t, err := h.service.GetTask(r.Context(), taskID)
	if err != nil {
		h.logger.Error("GetTask failed", "task_id", taskID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, t)
}

// Update handles PATCH /api/v1/organizations/{org_id}/tasks/{task_id}.
func (h *TaskHandler) Update(w http.ResponseWriter, r *http.Request) {
	taskID, err := parseTaskPathUUID(r, "task_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var t Task
	if err := api.ReadJSON(r, &t); err != nil {
		api.WriteError(w, err)
		return
	}

	updated, err := h.service.UpdateTask(r.Context(), taskID, &t)
	if err != nil {
		h.logger.Error("UpdateTask failed", "task_id", taskID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, updated)
}

// Assign handles POST /api/v1/organizations/{org_id}/tasks/{task_id}/assign.
func (h *TaskHandler) Assign(w http.ResponseWriter, r *http.Request) {
	taskID, err := parseTaskPathUUID(r, "task_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var req AssignTaskRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}

	updated, err := h.service.AssignTask(r.Context(), taskID, req, callerID(r))
	if err != nil {
		h.logger.Error("AssignTask failed", "task_id", taskID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, updated)
}

// Transition handles POST /api/v1/organizations/{org_id}/tasks/{task_id}/transition.
func (h *TaskHandler) Transition(w http.ResponseWriter, r *http.Request) {
	taskID, err := parseTaskPathUUID(r, "task_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var req TransitionTaskRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}

	updated, err := h.service.TransitionTask(r.Context(), taskID, req, callerID(r))
	if err != nil {
		h.logger.Error("TransitionTask failed", "task_id", taskID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, updated)
}

// AddComment handles POST /api/v1/organizations/{org_id}/tasks/{task_id}/comments.
func (h *TaskHandler) AddComment(w http.ResponseWriter, r *http.Request) {
	taskID, err := parseTaskPathUUID(r, "task_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var req AddCommentRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}

	comment, err := h.service.AddComment(r.Context(), taskID, req, callerID(r))
	if err != nil {
		h.logger.Error("AddComment failed", "task_id", taskID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusCreated, comment)
}

// ToggleChecklist handles PATCH /api/v1/organizations/{org_id}/tasks/{task_id}/checklist/{item_id}.
func (h *TaskHandler) ToggleChecklist(w http.ResponseWriter, r *http.Request) {
	taskID, err := parseTaskPathUUID(r, "task_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	itemID := r.PathValue("item_id")
	if itemID == "" {
		api.WriteError(w, api.NewValidationError("validation.required", "item_id", api.P("field", "item_id")))
		return
	}

	updated, err := h.service.ToggleChecklistItem(r.Context(), taskID, itemID)
	if err != nil {
		h.logger.Error("ToggleChecklistItem failed", "task_id", taskID, "item_id", itemID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, updated)
}

// ---------------------------------------------------------------------------
// Task Type endpoints
// ---------------------------------------------------------------------------

// ListTypes handles GET /api/v1/organizations/{org_id}/task-types.
func (h *TaskHandler) ListTypes(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseTaskPathUUID(r, "org_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	types, err := h.service.ListTaskTypes(r.Context(), orgID)
	if err != nil {
		h.logger.Error("ListTaskTypes failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, types)
}

// CreateType handles POST /api/v1/organizations/{org_id}/task-types.
func (h *TaskHandler) CreateType(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseTaskPathUUID(r, "org_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var req CreateTaskTypeRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}

	tt, err := h.service.CreateTaskType(r.Context(), orgID, req)
	if err != nil {
		h.logger.Error("CreateTaskType failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusCreated, tt)
}

// UpdateType handles PATCH /api/v1/organizations/{org_id}/task-types/{type_id}.
func (h *TaskHandler) UpdateType(w http.ResponseWriter, r *http.Request) {
	typeID, err := parseTaskPathUUID(r, "type_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var tt TaskType
	if err := api.ReadJSON(r, &tt); err != nil {
		api.WriteError(w, err)
		return
	}

	updated, err := h.service.UpdateTaskType(r.Context(), typeID, &tt)
	if err != nil {
		h.logger.Error("UpdateTaskType failed", "type_id", typeID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, updated)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// parseTaskCursorID decodes a pagination cursor and returns the ID it encodes.
// Returns nil, nil when cursor is empty (first page).
func parseTaskCursorID(cursor string) (*uuid.UUID, error) {
	if cursor == "" {
		return nil, nil
	}
	vals, err := api.DecodeCursor(cursor)
	if err != nil {
		return nil, err
	}
	id, err := uuid.Parse(vals["id"])
	if err != nil {
		return nil, err
	}
	return &id, nil
}

// parseTaskPathUUID extracts and parses a UUID path value by the given key.
func parseTaskPathUUID(r *http.Request, key string) (uuid.UUID, error) {
	raw := r.PathValue(key)
	if raw == "" {
		return uuid.Nil, api.NewValidationError("validation.required", key, api.P("field", key))
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, api.NewValidationError("validation.invalid_uuid", key, api.P("field", key))
	}
	return id, nil
}

// callerID extracts the authenticated user's internal UUID from context.
// Requires either RBAC middleware or ResolveUserID middleware to have run first.
func callerID(r *http.Request) uuid.UUID {
	return middleware.UserIDFromContext(r.Context())
}

// orgIDFromContext reads the org UUID stored by TenantContext middleware.
func orgIDFromContext(r *http.Request) uuid.UUID {
	return middleware.OrgIDFromContext(r.Context())
}
