package admin

import (
	"net/http"

	"github.com/quorant/quorant/internal/platform/app"
)

// AdminModule implements app.Module for the admin domain.
type AdminModule struct{}

func NewModule() *AdminModule { return &AdminModule{} }

func (m *AdminModule) Name() string { return "admin" }

func (m *AdminModule) Register(mux *http.ServeMux, deps app.Dependencies) {
	repo := NewPostgresAdminRepository(deps.Pool)
	service := NewAdminService(repo, deps.Auditor, deps.Publisher, deps.Logger)
	handler := NewAdminHandler(service, deps.Logger)
	RegisterRoutes(mux, handler, deps.TokenValidator, deps.PermChecker, deps.ResolveUserID)
}
