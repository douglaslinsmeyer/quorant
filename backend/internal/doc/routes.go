package doc

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/platform/auth"
	"github.com/quorant/quorant/internal/platform/middleware"
)

// RegisterRoutes registers all doc module routes on mux, protected by auth and
// tenant-context middleware.
func RegisterRoutes(
	mux *http.ServeMux,
	handler *DocHandler,
	validator auth.TokenValidator,
	checker middleware.PermissionChecker,
	resolveUserID func(context.Context) (uuid.UUID, error),
) {
	permMw := func(perm string, h http.HandlerFunc) http.Handler {
		return middleware.Auth(validator,
			middleware.TenantContext(
				middleware.RequirePermission(checker, perm, resolveUserID)(
					http.HandlerFunc(h))))
	}

	// Documents (8 routes)
	mux.Handle("POST /api/v1/organizations/{org_id}/documents", permMw("doc.document.upload", handler.UploadDocument))
	mux.Handle("GET /api/v1/organizations/{org_id}/documents", permMw("doc.document.read", handler.ListDocuments))
	mux.Handle("GET /api/v1/organizations/{org_id}/documents/{document_id}", permMw("doc.document.read", handler.GetDocument))
	mux.Handle("GET /api/v1/organizations/{org_id}/documents/{document_id}/download", permMw("doc.document.read", handler.GetDownloadURL))
	mux.Handle("PATCH /api/v1/organizations/{org_id}/documents/{document_id}", permMw("doc.document.manage", handler.UpdateDocument))
	mux.Handle("DELETE /api/v1/organizations/{org_id}/documents/{document_id}", permMw("doc.document.manage", handler.DeleteDocument))
	mux.Handle("POST /api/v1/organizations/{org_id}/documents/{document_id}/versions", permMw("doc.version.create", handler.UploadVersion))
	mux.Handle("GET /api/v1/organizations/{org_id}/documents/{document_id}/versions", permMw("doc.document.read", handler.ListVersions))

	// Document Categories (4 routes)
	mux.Handle("POST /api/v1/organizations/{org_id}/document-categories", permMw("doc.document.manage", handler.CreateCategory))
	mux.Handle("GET /api/v1/organizations/{org_id}/document-categories", permMw("doc.document.read", handler.ListCategories))
	mux.Handle("PATCH /api/v1/organizations/{org_id}/document-categories/{category_id}", permMw("doc.document.manage", handler.UpdateCategory))
	mux.Handle("DELETE /api/v1/organizations/{org_id}/document-categories/{category_id}", permMw("doc.document.manage", handler.DeleteCategory))
}
