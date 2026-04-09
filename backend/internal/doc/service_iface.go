package doc

import (
	"context"

	"github.com/google/uuid"
)

// Service defines the business operations for the document module.
// Handlers depend on this interface rather than the concrete DocService struct.
type Service interface {
	// Documents
	UploadDocument(ctx context.Context, orgID uuid.UUID, req UploadDocumentRequest, uploadedBy uuid.UUID) (*Document, error)
	GetDocument(ctx context.Context, id uuid.UUID) (*Document, error)
	ListDocuments(ctx context.Context, orgID uuid.UUID, limit int, afterID *uuid.UUID) ([]Document, bool, error)
	UpdateDocument(ctx context.Context, id uuid.UUID, doc *Document) (*Document, error)
	DeleteDocument(ctx context.Context, id uuid.UUID) error
	GetDownloadURL(ctx context.Context, id uuid.UUID) (string, error)
	UploadNewVersion(ctx context.Context, parentDocID uuid.UUID, req UploadDocumentRequest, uploadedBy uuid.UUID) (*Document, error)
	ListVersions(ctx context.Context, docID uuid.UUID) ([]Document, error)

	// Categories
	CreateCategory(ctx context.Context, orgID uuid.UUID, req CreateCategoryRequest) (*DocumentCategory, error)
	ListCategories(ctx context.Context, orgID uuid.UUID) ([]DocumentCategory, error)
	UpdateCategory(ctx context.Context, id uuid.UUID, cat *DocumentCategory) (*DocumentCategory, error)
	DeleteCategory(ctx context.Context, id uuid.UUID) error
}
