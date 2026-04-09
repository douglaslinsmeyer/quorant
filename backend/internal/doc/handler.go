package doc

import (
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/platform/api"
	"github.com/quorant/quorant/internal/platform/middleware"
)

// DocHandler handles HTTP requests for the document management module.
type DocHandler struct {
	service *DocService
	logger  *slog.Logger
}

// NewDocHandler constructs a DocHandler backed by the given service.
func NewDocHandler(service *DocService, logger *slog.Logger) *DocHandler {
	return &DocHandler{service: service, logger: logger}
}

// ─── Documents ────────────────────────────────────────────────────────────────

// UploadDocument handles POST /organizations/{org_id}/documents.
// Returns 201 with the created document metadata. File upload is client-side
// via a pre-signed PUT URL obtained from the storage layer.
func (h *DocHandler) UploadDocument(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseDocOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var req UploadDocumentRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}

	created, err := h.service.UploadDocument(r.Context(), orgID, req, middleware.UserIDFromContext(r.Context()))
	if err != nil {
		h.logger.Error("UploadDocument failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusCreated, created)
}

// ListDocuments handles GET /organizations/{org_id}/documents.
func (h *DocHandler) ListDocuments(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseDocOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	page := api.ParsePageRequest(r)
	afterID, err := parseDocCursorID(page.Cursor)
	if err != nil {
		api.WriteError(w, api.NewValidationError("invalid cursor", "cursor"))
		return
	}

	docs, hasMore, err := h.service.ListDocuments(r.Context(), orgID, page.Limit, afterID)
	if err != nil {
		h.logger.Error("ListDocuments failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	var meta *api.Meta
	if hasMore && len(docs) > 0 {
		meta = &api.Meta{
			Cursor:  api.EncodeCursor(map[string]string{"id": docs[len(docs)-1].ID.String()}),
			HasMore: true,
		}
	}

	api.WriteJSONWithMeta(w, http.StatusOK, docs, meta)
}

// GetDocument handles GET /organizations/{org_id}/documents/{document_id}.
func (h *DocHandler) GetDocument(w http.ResponseWriter, r *http.Request) {
	_, err := parseDocOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	docID, err := parseDocPathUUID(r, "document_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	d, err := h.service.GetDocument(r.Context(), docID)
	if err != nil {
		h.logger.Error("GetDocument failed", "document_id", docID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, d)
}

// GetDownloadURL handles GET /organizations/{org_id}/documents/{document_id}/download.
// Returns a JSON object with a pre-signed URL: {"data": {"url": "https://..."}}
func (h *DocHandler) GetDownloadURL(w http.ResponseWriter, r *http.Request) {
	_, err := parseDocOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	docID, err := parseDocPathUUID(r, "document_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	url, err := h.service.GetDownloadURL(r.Context(), docID)
	if err != nil {
		h.logger.Error("GetDownloadURL failed", "document_id", docID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, map[string]string{"url": url})
}

// UpdateDocument handles PATCH /organizations/{org_id}/documents/{document_id}.
func (h *DocHandler) UpdateDocument(w http.ResponseWriter, r *http.Request) {
	_, err := parseDocOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	docID, err := parseDocPathUUID(r, "document_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var d Document
	if err := api.ReadJSON(r, &d); err != nil {
		api.WriteError(w, err)
		return
	}

	updated, err := h.service.UpdateDocument(r.Context(), docID, &d)
	if err != nil {
		h.logger.Error("UpdateDocument failed", "document_id", docID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, updated)
}

// DeleteDocument handles DELETE /organizations/{org_id}/documents/{document_id}.
func (h *DocHandler) DeleteDocument(w http.ResponseWriter, r *http.Request) {
	_, err := parseDocOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	docID, err := parseDocPathUUID(r, "document_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	if err := h.service.DeleteDocument(r.Context(), docID); err != nil {
		h.logger.Error("DeleteDocument failed", "document_id", docID, "error", err)
		api.WriteError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ─── Versioning ───────────────────────────────────────────────────────────────

// UploadVersion handles POST /organizations/{org_id}/documents/{document_id}/versions.
func (h *DocHandler) UploadVersion(w http.ResponseWriter, r *http.Request) {
	_, err := parseDocOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	docID, err := parseDocPathUUID(r, "document_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var req UploadDocumentRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}

	newVersion, err := h.service.UploadNewVersion(r.Context(), docID, req, middleware.UserIDFromContext(r.Context()))
	if err != nil {
		h.logger.Error("UploadVersion failed", "document_id", docID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusCreated, newVersion)
}

// ListVersions handles GET /organizations/{org_id}/documents/{document_id}/versions.
func (h *DocHandler) ListVersions(w http.ResponseWriter, r *http.Request) {
	_, err := parseDocOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	docID, err := parseDocPathUUID(r, "document_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	versions, err := h.service.ListVersions(r.Context(), docID)
	if err != nil {
		h.logger.Error("ListVersions failed", "document_id", docID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, versions)
}

// ─── Categories ───────────────────────────────────────────────────────────────

// CreateCategory handles POST /organizations/{org_id}/document-categories.
func (h *DocHandler) CreateCategory(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseDocOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var req CreateCategoryRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}

	cat, err := h.service.CreateCategory(r.Context(), orgID, req)
	if err != nil {
		h.logger.Error("CreateCategory failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusCreated, cat)
}

// ListCategories handles GET /organizations/{org_id}/document-categories.
func (h *DocHandler) ListCategories(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseDocOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	cats, err := h.service.ListCategories(r.Context(), orgID)
	if err != nil {
		h.logger.Error("ListCategories failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, cats)
}

// UpdateCategory handles PATCH /organizations/{org_id}/document-categories/{category_id}.
func (h *DocHandler) UpdateCategory(w http.ResponseWriter, r *http.Request) {
	_, err := parseDocOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	catID, err := parseDocPathUUID(r, "category_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var cat DocumentCategory
	if err := api.ReadJSON(r, &cat); err != nil {
		api.WriteError(w, err)
		return
	}

	updated, err := h.service.UpdateCategory(r.Context(), catID, &cat)
	if err != nil {
		h.logger.Error("UpdateCategory failed", "category_id", catID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, updated)
}

// DeleteCategory handles DELETE /organizations/{org_id}/document-categories/{category_id}.
func (h *DocHandler) DeleteCategory(w http.ResponseWriter, r *http.Request) {
	_, err := parseDocOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	catID, err := parseDocPathUUID(r, "category_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	if err := h.service.DeleteCategory(r.Context(), catID); err != nil {
		h.logger.Error("DeleteCategory failed", "category_id", catID, "error", err)
		api.WriteError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

// parseDocCursorID decodes a pagination cursor and returns the ID it encodes.
// Returns nil, nil when cursor is empty (first page).
func parseDocCursorID(cursor string) (*uuid.UUID, error) {
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

// parseDocOrgID extracts and parses the {org_id} path value.
func parseDocOrgID(r *http.Request) (uuid.UUID, error) {
	return parseDocPathUUID(r, "org_id")
}

// parseDocPathUUID extracts and parses a UUID path value by the given key.
func parseDocPathUUID(r *http.Request, key string) (uuid.UUID, error) {
	raw := r.PathValue(key)
	if raw == "" {
		return uuid.Nil, api.NewValidationError(key+" is required", key)
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, api.NewValidationError(key+" must be a valid UUID", key)
	}
	return id, nil
}
