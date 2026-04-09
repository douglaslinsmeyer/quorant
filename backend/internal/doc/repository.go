package doc

import (
	"context"

	"github.com/google/uuid"
)

// DocRepository persists and retrieves documents and document categories.
type DocRepository interface {
	// ─── Documents ────────────────────────────────────────────────────────────

	// Create inserts a new document, generating a storage_key of the form
	// {org_id}/{uuid}/{file_name} and setting is_current=true, version_number=1.
	Create(ctx context.Context, doc *Document) (*Document, error)

	// FindByID returns the document with the given ID, or nil if not found or soft-deleted.
	FindByID(ctx context.Context, id uuid.UUID) (*Document, error)

	// ListByOrg returns current, non-deleted documents for the given org, supporting
	// cursor-based pagination ordered by id. afterID is the cursor from the previous page;
	// hasMore is true when more items exist.
	ListByOrg(ctx context.Context, orgID uuid.UUID, limit int, afterID *uuid.UUID) ([]Document, bool, error)

	// Update persists changes to an existing document and returns the updated row.
	Update(ctx context.Context, doc *Document) (*Document, error)

	// SoftDelete marks a document as deleted without removing the row.
	SoftDelete(ctx context.Context, id uuid.UUID) error

	// ─── Versioning ───────────────────────────────────────────────────────────

	// CreateVersion creates a new version of an existing document in a transaction:
	// sets the parent document's is_current=false, then inserts a new document
	// with parent_doc_id set, version_number incremented by 1, and is_current=true.
	CreateVersion(ctx context.Context, parentDocID uuid.UUID, doc *Document) (*Document, error)

	// ListVersions returns all versions of a document chain sorted by version_number DESC.
	// Given any doc in the chain, it finds the root (COALESCE(parent_doc_id, id))
	// and returns all documents with that root.
	ListVersions(ctx context.Context, docID uuid.UUID) ([]Document, error)

	// ─── Categories ───────────────────────────────────────────────────────────

	// CreateCategory inserts a new document category and returns the persisted record.
	CreateCategory(ctx context.Context, cat *DocumentCategory) (*DocumentCategory, error)

	// ListCategoriesByOrg returns all categories for the given org, ordered by sort_order, name.
	ListCategoriesByOrg(ctx context.Context, orgID uuid.UUID) ([]DocumentCategory, error)

	// UpdateCategory persists changes to an existing category and returns the updated row.
	UpdateCategory(ctx context.Context, cat *DocumentCategory) (*DocumentCategory, error)

	// DeleteCategory hard-deletes a category by ID.
	DeleteCategory(ctx context.Context, id uuid.UUID) error
}
