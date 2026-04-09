package task

import (
	"net/http"

	"github.com/quorant/quorant/internal/platform/app"
)

type TaskModule struct{}

func NewModule() *TaskModule { return &TaskModule{} }

func (m *TaskModule) Name() string { return "task" }

func (m *TaskModule) Register(mux *http.ServeMux, deps app.Dependencies) {
	repo := NewPostgresTaskRepository(deps.Pool)
	service := NewTaskService(repo, deps.Auditor, deps.Publisher, deps.Logger)
	handler := NewTaskHandler(service, deps.Logger)
	RegisterRoutes(mux, handler, deps.TokenValidator, deps.PermChecker, deps.ResolveUserID)
}
