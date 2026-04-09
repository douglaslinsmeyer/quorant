package task

import (
	"net/http"

	"github.com/quorant/quorant/internal/platform/auth"
	"github.com/quorant/quorant/internal/platform/middleware"
)

// RegisterRoutes mounts all task endpoints on mux, protected by appropriate middleware.
func RegisterRoutes(mux *http.ServeMux, handler *TaskHandler, validator auth.TokenValidator) {
	orgMw := func(h http.HandlerFunc) http.Handler {
		return middleware.Auth(validator, middleware.TenantContext(http.HandlerFunc(h)))
	}
	authMw := func(h http.HandlerFunc) http.Handler {
		return middleware.Auth(validator, http.HandlerFunc(h))
	}

	// Cross-org task views (user-scoped)
	mux.Handle("GET /api/v1/tasks", authMw(handler.ListMyTasks))
	mux.Handle("GET /api/v1/tasks/dashboard", authMw(handler.Dashboard))

	// Org-scoped task operations
	mux.Handle("POST /api/v1/organizations/{org_id}/tasks", orgMw(handler.Create))
	mux.Handle("GET /api/v1/organizations/{org_id}/tasks", orgMw(handler.List))
	mux.Handle("GET /api/v1/organizations/{org_id}/tasks/{task_id}", orgMw(handler.Get))
	mux.Handle("PATCH /api/v1/organizations/{org_id}/tasks/{task_id}", orgMw(handler.Update))
	mux.Handle("POST /api/v1/organizations/{org_id}/tasks/{task_id}/assign", orgMw(handler.Assign))
	mux.Handle("POST /api/v1/organizations/{org_id}/tasks/{task_id}/transition", orgMw(handler.Transition))
	mux.Handle("POST /api/v1/organizations/{org_id}/tasks/{task_id}/comments", orgMw(handler.AddComment))
	mux.Handle("PATCH /api/v1/organizations/{org_id}/tasks/{task_id}/checklist/{item_id}", orgMw(handler.ToggleChecklist))
	mux.Handle("GET /api/v1/organizations/{org_id}/task-types", orgMw(handler.ListTypes))
	mux.Handle("POST /api/v1/organizations/{org_id}/task-types", orgMw(handler.CreateType))
	mux.Handle("PATCH /api/v1/organizations/{org_id}/task-types/{type_id}", orgMw(handler.UpdateType))
}
