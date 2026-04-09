package doc

import (
	"net/http"

	"github.com/quorant/quorant/internal/platform/auth"
	"github.com/quorant/quorant/internal/platform/middleware"
)

// RegisterRoutes registers all doc module routes on mux, protected by auth and
// tenant-context middleware.
func RegisterRoutes(mux *http.ServeMux, handler *DocHandler, validator auth.TokenValidator) {
	orgMw := func(h http.HandlerFunc) http.Handler {
		return middleware.Auth(validator, middleware.TenantContext(http.HandlerFunc(h)))
	}

	// Documents (8 routes)
	mux.Handle("POST /api/v1/organizations/{org_id}/documents", orgMw(handler.UploadDocument))
	mux.Handle("GET /api/v1/organizations/{org_id}/documents", orgMw(handler.ListDocuments))
	mux.Handle("GET /api/v1/organizations/{org_id}/documents/{document_id}", orgMw(handler.GetDocument))
	mux.Handle("GET /api/v1/organizations/{org_id}/documents/{document_id}/download", orgMw(handler.GetDownloadURL))
	mux.Handle("PATCH /api/v1/organizations/{org_id}/documents/{document_id}", orgMw(handler.UpdateDocument))
	mux.Handle("DELETE /api/v1/organizations/{org_id}/documents/{document_id}", orgMw(handler.DeleteDocument))
	mux.Handle("POST /api/v1/organizations/{org_id}/documents/{document_id}/versions", orgMw(handler.UploadVersion))
	mux.Handle("GET /api/v1/organizations/{org_id}/documents/{document_id}/versions", orgMw(handler.ListVersions))

	// Document Categories (4 routes)
	mux.Handle("POST /api/v1/organizations/{org_id}/document-categories", orgMw(handler.CreateCategory))
	mux.Handle("GET /api/v1/organizations/{org_id}/document-categories", orgMw(handler.ListCategories))
	mux.Handle("PATCH /api/v1/organizations/{org_id}/document-categories/{category_id}", orgMw(handler.UpdateCategory))
	mux.Handle("DELETE /api/v1/organizations/{org_id}/document-categories/{category_id}", orgMw(handler.DeleteCategory))
}
