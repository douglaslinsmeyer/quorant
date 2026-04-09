package task

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/platform/auth"
	"github.com/quorant/quorant/internal/platform/middleware"
)

// RegisterRoutes mounts all task endpoints on mux, protected by appropriate middleware.
func RegisterRoutes(
	mux *http.ServeMux,
	handler *TaskHandler,
	validator auth.TokenValidator,
	checker middleware.PermissionChecker,
	resolveUserID func(context.Context) (uuid.UUID, error),
) {
	// auth only — user-scoped, no org context
	authMw := func(h http.HandlerFunc) http.Handler {
		return middleware.Auth(validator, http.HandlerFunc(h))
	}

	// auth + tenant context + permission check
	permMw := func(perm string, h http.HandlerFunc) http.Handler {
		return middleware.Auth(validator,
			middleware.TenantContext(
				middleware.RequirePermission(checker, perm, resolveUserID)(
					http.HandlerFunc(h))))
	}

	// Cross-org task views (user-scoped, auth only)
	mux.Handle("GET /api/v1/tasks", authMw(handler.ListMyTasks))
	mux.Handle("GET /api/v1/tasks/dashboard", authMw(handler.Dashboard))

	// Org-scoped task operations
	mux.Handle("POST /api/v1/organizations/{org_id}/tasks", permMw("task.create", handler.Create))
	mux.Handle("GET /api/v1/organizations/{org_id}/tasks", permMw("task.read", handler.List))
	mux.Handle("GET /api/v1/organizations/{org_id}/tasks/{task_id}", permMw("task.read", handler.Get))
	mux.Handle("PATCH /api/v1/organizations/{org_id}/tasks/{task_id}", permMw("task.manage", handler.Update))
	mux.Handle("POST /api/v1/organizations/{org_id}/tasks/{task_id}/assign", permMw("task.assign", handler.Assign))
	mux.Handle("POST /api/v1/organizations/{org_id}/tasks/{task_id}/transition", permMw("task.transition", handler.Transition))
	mux.Handle("POST /api/v1/organizations/{org_id}/tasks/{task_id}/comments", permMw("task.comment", handler.AddComment))
	mux.Handle("PATCH /api/v1/organizations/{org_id}/tasks/{task_id}/checklist/{item_id}", permMw("task.manage", handler.ToggleChecklist))
	mux.Handle("GET /api/v1/organizations/{org_id}/task-types", permMw("task.read", handler.ListTypes))
	mux.Handle("POST /api/v1/organizations/{org_id}/task-types", permMw("task.manage", handler.CreateType))
	mux.Handle("PATCH /api/v1/organizations/{org_id}/task-types/{type_id}", permMw("task.manage", handler.UpdateType))
}
