package doc

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/audit"
	"github.com/quorant/quorant/internal/platform/api"
	"github.com/quorant/quorant/internal/platform/queue"
	"github.com/quorant/quorant/internal/platform/storage"
)

// DocService orchestrates document management operations.
type DocService struct {
	repo      DocRepository
	storage   storage.StorageClient
	bucket    string
	auditor   audit.Auditor
	publisher queue.Publisher
	logger    *slog.Logger
}

// NewDocService constructs a DocService with all required dependencies.
func NewDocService(repo DocRepository, storage storage.StorageClient, bucket string, auditor audit.Auditor, publisher queue.Publisher, logger *slog.Logger) *DocService {
	return &DocService{
		repo:      repo,
		storage:   storage,
		bucket:    bucket,
		auditor:   auditor,
		publisher: publisher,
		logger:    logger,
	}
}

// ─── Documents ────────────────────────────────────────────────────────────────

// UploadDocument validates the request, then creates a document record.
// Actual file upload is done client-side via a pre-signed PUT URL obtained
// separately; this method only persists metadata and returns the new document.
func (s *DocService) UploadDocument(ctx context.Context, orgID uuid.UUID, req UploadDocumentRequest, uploadedBy uuid.UUID) (*Document, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	visibility := req.Visibility
	if visibility == "" {
		visibility = "members"
	}

	doc := &Document{
		OrgID:       orgID,
		CategoryID:  req.CategoryID,
		UploadedBy:  uploadedBy,
		Title:       req.Title,
		FileName:    req.FileName,
		ContentType: req.ContentType,
		SizeBytes:   req.SizeBytes,
		Visibility:  visibility,
		Metadata:    map[string]any{},
	}

	return s.repo.Create(ctx, doc)
}

// GetDocument returns a single document by ID or a 404 error if not found.
func (s *DocService) GetDocument(ctx context.Context, id uuid.UUID) (*Document, error) {
	doc, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, api.NewNotFoundError("document not found")
	}
	return doc, nil
}

// ListDocuments returns current documents for the given organization, supporting cursor-based pagination.
// limit controls the page size; afterID is the cursor from the previous page.
func (s *DocService) ListDocuments(ctx context.Context, orgID uuid.UUID, limit int, afterID *uuid.UUID) ([]Document, bool, error) {
	return s.repo.ListByOrg(ctx, orgID, limit, afterID)
}

// UpdateDocument persists changes to an existing document and returns it.
func (s *DocService) UpdateDocument(ctx context.Context, id uuid.UUID, doc *Document) (*Document, error) {
	doc.ID = id
	return s.repo.Update(ctx, doc)
}

// DeleteDocument soft-deletes a document by ID.
func (s *DocService) DeleteDocument(ctx context.Context, id uuid.UUID) error {
	return s.repo.SoftDelete(ctx, id)
}

// GetDownloadURL retrieves the document and returns a pre-signed GET URL
// (valid for 15 minutes) that the client can use to download the file directly.
func (s *DocService) GetDownloadURL(ctx context.Context, id uuid.UUID) (string, error) {
	doc, err := s.GetDocument(ctx, id)
	if err != nil {
		return "", err
	}

	url, err := s.storage.PresignedGetURL(ctx, s.bucket, doc.StorageKey, 15*time.Minute)
	if err != nil {
		return "", err
	}
	return url, nil
}

// ─── Versioning ───────────────────────────────────────────────────────────────

// UploadNewVersion validates the request and creates a new version linked to
// the given parent document. The parent document's is_current flag is set to
// false by the repository layer within a transaction.
func (s *DocService) UploadNewVersion(ctx context.Context, parentDocID uuid.UUID, req UploadDocumentRequest, uploadedBy uuid.UUID) (*Document, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	// Look up the parent to inherit org_id and category.
	parent, err := s.GetDocument(ctx, parentDocID)
	if err != nil {
		return nil, err
	}

	visibility := req.Visibility
	if visibility == "" {
		visibility = parent.Visibility
	}

	categoryID := req.CategoryID
	if categoryID == nil {
		categoryID = parent.CategoryID
	}

	newDoc := &Document{
		OrgID:       parent.OrgID,
		CategoryID:  categoryID,
		UploadedBy:  uploadedBy,
		Title:       req.Title,
		FileName:    req.FileName,
		ContentType: req.ContentType,
		SizeBytes:   req.SizeBytes,
		Visibility:  visibility,
		Metadata:    map[string]any{},
	}

	return s.repo.CreateVersion(ctx, parentDocID, newDoc)
}

// ListVersions returns all versions in the document chain, sorted by
// version_number DESC.
func (s *DocService) ListVersions(ctx context.Context, docID uuid.UUID) ([]Document, error) {
	return s.repo.ListVersions(ctx, docID)
}

// ─── Categories ───────────────────────────────────────────────────────────────

// CreateCategory validates the request and persists a new document category.
func (s *DocService) CreateCategory(ctx context.Context, orgID uuid.UUID, req CreateCategoryRequest) (*DocumentCategory, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	cat := &DocumentCategory{
		OrgID:    orgID,
		Name:     req.Name,
		ParentID: req.ParentID,
	}

	return s.repo.CreateCategory(ctx, cat)
}

// ListCategories returns all categories for the given organization.
func (s *DocService) ListCategories(ctx context.Context, orgID uuid.UUID) ([]DocumentCategory, error) {
	return s.repo.ListCategoriesByOrg(ctx, orgID)
}

// UpdateCategory persists changes to an existing category and returns it.
func (s *DocService) UpdateCategory(ctx context.Context, id uuid.UUID, cat *DocumentCategory) (*DocumentCategory, error) {
	cat.ID = id
	return s.repo.UpdateCategory(ctx, cat)
}

// DeleteCategory hard-deletes a category by ID.
func (s *DocService) DeleteCategory(ctx context.Context, id uuid.UUID) error {
	return s.repo.DeleteCategory(ctx, id)
}
