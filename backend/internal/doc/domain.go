package doc

import (
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/platform/api"
)

// Document represents a file stored in the document management system.
type Document struct {
	ID            uuid.UUID      `json:"id"`
	OrgID         uuid.UUID      `json:"org_id"`
	CategoryID    *uuid.UUID     `json:"category_id,omitempty"`
	UploadedBy    uuid.UUID      `json:"uploaded_by"`
	Title         string         `json:"title"`
	FileName      string         `json:"file_name"`
	ContentType   string         `json:"content_type"`
	SizeBytes     int64          `json:"size_bytes"`
	StorageKey    string         `json:"storage_key"`
	Visibility    string         `json:"visibility"` // 'members', 'board_only', 'public'
	VersionNumber int            `json:"version_number"`
	ParentDocID   *uuid.UUID     `json:"parent_doc_id,omitempty"`
	IsCurrent     bool           `json:"is_current"`
	Metadata      map[string]any `json:"metadata"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     *time.Time     `json:"deleted_at,omitempty"`
}

// DocumentCategory represents a category for organizing documents.
type DocumentCategory struct {
	ID        uuid.UUID  `json:"id"`
	OrgID     uuid.UUID  `json:"org_id"`
	Name      string     `json:"name"`
	ParentID  *uuid.UUID `json:"parent_id,omitempty"`
	SortOrder int        `json:"sort_order"`
	CreatedAt time.Time  `json:"created_at"`
}

// UploadDocumentRequest is the request type for uploading a new document.
type UploadDocumentRequest struct {
	Title       string     `json:"title"`
	FileName    string     `json:"file_name"`
	ContentType string     `json:"content_type"`
	SizeBytes   int64      `json:"size_bytes"`
	CategoryID  *uuid.UUID `json:"category_id,omitempty"`
	Visibility  string     `json:"visibility,omitempty"`
}

// Validate checks that the UploadDocumentRequest has all required fields.
func (r UploadDocumentRequest) Validate() *api.ValidationError {
	if r.Title == "" {
		return api.NewValidationError("validation.required", "title", api.P("field", "title"))
	}
	if r.FileName == "" {
		return api.NewValidationError("validation.required", "file_name", api.P("field", "file_name"))
	}
	if r.ContentType == "" {
		return api.NewValidationError("validation.required", "content_type", api.P("field", "content_type"))
	}
	if r.SizeBytes <= 0 {
		return api.NewValidationError("validation.constraint", "size_bytes", api.P("field", "size_bytes"), api.P("constraint", "positive number"))
	}
	return nil
}

// UploadFromBytesRequest is the request type for server-side document upload
// where the caller provides raw bytes rather than using a pre-signed URL.
type UploadFromBytesRequest struct {
	Title       string     `json:"title"`
	FileName    string     `json:"file_name"`
	ContentType string     `json:"content_type"`
	CategoryID  *uuid.UUID `json:"category_id,omitempty"`
	Visibility  string     `json:"visibility,omitempty"`
}

// Validate checks that the UploadFromBytesRequest has all required fields.
func (r UploadFromBytesRequest) Validate() *api.ValidationError {
	if r.Title == "" {
		return api.NewValidationError("title is required", "title")
	}
	if r.FileName == "" {
		return api.NewValidationError("file_name is required", "file_name")
	}
	if r.ContentType == "" {
		return api.NewValidationError("content_type is required", "content_type")
	}
	return nil
}

// CreateCategoryRequest is the request type for creating a document category.
type CreateCategoryRequest struct {
	Name     string     `json:"name"`
	ParentID *uuid.UUID `json:"parent_id,omitempty"`
}

// Validate checks that the CreateCategoryRequest has all required fields.
func (r CreateCategoryRequest) Validate() *api.ValidationError {
	if r.Name == "" {
		return api.NewValidationError("validation.required", "name", api.P("field", "name"))
	}
	return nil
}
