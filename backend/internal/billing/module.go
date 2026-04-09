package billing

import (
	"net/http"

	"github.com/quorant/quorant/internal/platform/app"
)

type BillingModule struct{}

func NewModule() *BillingModule { return &BillingModule{} }

func (m *BillingModule) Name() string { return "billing" }

func (m *BillingModule) Register(mux *http.ServeMux, deps app.Dependencies) {
	repo := NewPostgresBillingRepository(deps.Pool)
	service := NewBillingService(repo, deps.Auditor, deps.Publisher, deps.Logger)
	handler := NewBillingHandler(service, deps.Logger)
	RegisterRoutes(mux, handler, deps.TokenValidator, deps.PermChecker, deps.ResolveUserID)
}
