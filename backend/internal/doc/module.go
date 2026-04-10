package doc

import (
	"net/http"

	"github.com/quorant/quorant/internal/platform/app"
	"github.com/quorant/quorant/internal/platform/storage"
)

type DocModule struct {
	storage storage.StorageClient
	bucket  string
}

func NewModule(stor storage.StorageClient, bucket string) *DocModule {
	return &DocModule{storage: stor, bucket: bucket}
}

func (m *DocModule) Name() string { return "doc" }

func (m *DocModule) Register(mux *http.ServeMux, deps app.Dependencies) {
	repo := NewPostgresDocRepository(deps.Pool)
	service := NewDocService(repo, m.storage, m.bucket, deps.Auditor, deps.Publisher, deps.Logger)
	handler := NewDocHandler(service, deps.Logger)
	RegisterRoutes(mux, handler, deps.TokenValidator, deps.PermChecker, deps.ResolveUserID)
}
