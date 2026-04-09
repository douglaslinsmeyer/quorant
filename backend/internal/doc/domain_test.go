package doc_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/doc"
)

// ─── Document JSON Serialization ──────────────────────────────────────────────

func TestDocument_JSONSerialization(t *testing.T) {
	catID := uuid.New()
	parentID := uuid.New()
	now := time.Now().UTC().Truncate(time.Second)

	d := doc.Document{
		ID:            uuid.New(),
		OrgID:         uuid.New(),
		CategoryID:    &catID,
		UploadedBy:    uuid.New(),
		Title:         "CC&Rs 2024",
		FileName:      "ccrs-2024.pdf",
		ContentType:   "application/pdf",
		SizeBytes:     204800,
		StorageKey:    "org-id/doc-id/ccrs-2024.pdf",
		Visibility:    "members",
		VersionNumber: 1,
		ParentDocID:   &parentID,
		IsCurrent:     true,
		Metadata:      map[string]any{"tags": []string{"legal"}},
		CreatedAt:     now,
		UpdatedAt:     now,
		DeletedAt:     nil,
	}

	data, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("json.Marshal(Document) error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	requiredKeys := []string{
		"id", "org_id", "category_id", "uploaded_by",
		"title", "file_name", "content_type", "size_bytes",
		"storage_key", "visibility", "version_number", "parent_doc_id",
		"is_current", "metadata", "created_at", "updated_at",
	}
	for _, key := range requiredKeys {
		if _, ok := result[key]; !ok {
			t.Errorf("expected JSON key %q to be present", key)
		}
	}

	if _, ok := result["deleted_at"]; ok {
		t.Errorf("expected JSON key %q to be omitted when nil", "deleted_at")
	}

	if result["title"] != "CC&Rs 2024" {
		t.Errorf("title: got %v, want %v", result["title"], "CC&Rs 2024")
	}
	if result["visibility"] != "members" {
		t.Errorf("visibility: got %v, want %v", result["visibility"], "members")
	}
	if result["is_current"] != true {
		t.Errorf("is_current: got %v, want true", result["is_current"])
	}
}

func TestDocument_OmitsOptionalFieldsWhenNil(t *testing.T) {
	d := doc.Document{
		ID:            uuid.New(),
		OrgID:         uuid.New(),
		UploadedBy:    uuid.New(),
		Title:         "Budget Report",
		FileName:      "budget.pdf",
		ContentType:   "application/pdf",
		SizeBytes:     1024,
		StorageKey:    "org/doc/budget.pdf",
		Visibility:    "board_only",
		VersionNumber: 1,
		IsCurrent:     true,
		Metadata:      map[string]any{},
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	data, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("json.Marshal(Document) error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	omittedKeys := []string{"category_id", "parent_doc_id", "deleted_at"}
	for _, key := range omittedKeys {
		if _, ok := result[key]; ok {
			t.Errorf("expected JSON key %q to be omitted when nil", key)
		}
	}
}

// ─── DocumentCategory JSON Serialization ──────────────────────────────────────

func TestDocumentCategory_JSONSerialization(t *testing.T) {
	parentID := uuid.New()
	now := time.Now().UTC().Truncate(time.Second)

	cat := doc.DocumentCategory{
		ID:        uuid.New(),
		OrgID:     uuid.New(),
		Name:      "Legal Documents",
		ParentID:  &parentID,
		SortOrder: 2,
		CreatedAt: now,
	}

	data, err := json.Marshal(cat)
	if err != nil {
		t.Fatalf("json.Marshal(DocumentCategory) error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	requiredKeys := []string{"id", "org_id", "name", "parent_id", "sort_order", "created_at"}
	for _, key := range requiredKeys {
		if _, ok := result[key]; !ok {
			t.Errorf("expected JSON key %q to be present", key)
		}
	}

	if result["name"] != "Legal Documents" {
		t.Errorf("name: got %v, want %v", result["name"], "Legal Documents")
	}
}

func TestDocumentCategory_OmitsParentIDWhenNil(t *testing.T) {
	cat := doc.DocumentCategory{
		ID:        uuid.New(),
		OrgID:     uuid.New(),
		Name:      "Root Category",
		SortOrder: 0,
		CreatedAt: time.Now(),
	}

	data, err := json.Marshal(cat)
	if err != nil {
		t.Fatalf("json.Marshal(DocumentCategory) error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	if _, ok := result["parent_id"]; ok {
		t.Errorf("expected JSON key %q to be omitted when nil", "parent_id")
	}
}

// ─── UploadDocumentRequest Validate ──────────────────────────────────────────

func TestUploadDocumentRequest_Validate_AllRequiredPresent(t *testing.T) {
	req := doc.UploadDocumentRequest{
		Title:       "CC&Rs",
		FileName:    "ccrs.pdf",
		ContentType: "application/pdf",
		SizeBytes:   1024,
	}

	if err := req.Validate(); err != nil {
		t.Errorf("expected nil error for valid request, got: %v", err)
	}
}

func TestUploadDocumentRequest_Validate_MissingTitle(t *testing.T) {
	req := doc.UploadDocumentRequest{
		FileName:    "ccrs.pdf",
		ContentType: "application/pdf",
		SizeBytes:   1024,
	}

	err := req.Validate()
	if err == nil {
		t.Error("expected error when title is missing, got nil")
	}
	if err.Field != "title" {
		t.Errorf("expected field %q, got %q", "title", err.Field)
	}
}

func TestUploadDocumentRequest_Validate_MissingFileName(t *testing.T) {
	req := doc.UploadDocumentRequest{
		Title:       "CC&Rs",
		ContentType: "application/pdf",
		SizeBytes:   1024,
	}

	err := req.Validate()
	if err == nil {
		t.Error("expected error when file_name is missing, got nil")
	}
	if err.Field != "file_name" {
		t.Errorf("expected field %q, got %q", "file_name", err.Field)
	}
}

func TestUploadDocumentRequest_Validate_MissingContentType(t *testing.T) {
	req := doc.UploadDocumentRequest{
		Title:     "CC&Rs",
		FileName:  "ccrs.pdf",
		SizeBytes: 1024,
	}

	err := req.Validate()
	if err == nil {
		t.Error("expected error when content_type is missing, got nil")
	}
	if err.Field != "content_type" {
		t.Errorf("expected field %q, got %q", "content_type", err.Field)
	}
}

func TestUploadDocumentRequest_Validate_ZeroSizeBytes(t *testing.T) {
	req := doc.UploadDocumentRequest{
		Title:       "CC&Rs",
		FileName:    "ccrs.pdf",
		ContentType: "application/pdf",
		SizeBytes:   0,
	}

	err := req.Validate()
	if err == nil {
		t.Error("expected error when size_bytes is zero, got nil")
	}
	if err.Field != "size_bytes" {
		t.Errorf("expected field %q, got %q", "size_bytes", err.Field)
	}
}

func TestUploadDocumentRequest_Validate_NegativeSizeBytes(t *testing.T) {
	req := doc.UploadDocumentRequest{
		Title:       "CC&Rs",
		FileName:    "ccrs.pdf",
		ContentType: "application/pdf",
		SizeBytes:   -100,
	}

	err := req.Validate()
	if err == nil {
		t.Error("expected error when size_bytes is negative, got nil")
	}
	if err.Field != "size_bytes" {
		t.Errorf("expected field %q, got %q", "size_bytes", err.Field)
	}
}

func TestUploadDocumentRequest_Validate_OptionalFieldsAreOptional(t *testing.T) {
	catID := uuid.New()
	req := doc.UploadDocumentRequest{
		Title:       "Meeting Minutes",
		FileName:    "minutes.pdf",
		ContentType: "application/pdf",
		SizeBytes:   512,
		CategoryID:  &catID,
		Visibility:  "board_only",
	}

	if err := req.Validate(); err != nil {
		t.Errorf("expected nil error for valid request with optional fields, got: %v", err)
	}
}

// ─── CreateCategoryRequest Validate ──────────────────────────────────────────

func TestCreateCategoryRequest_Validate_ValidName(t *testing.T) {
	req := doc.CreateCategoryRequest{
		Name: "Financial Records",
	}

	if err := req.Validate(); err != nil {
		t.Errorf("expected nil error for valid request, got: %v", err)
	}
}

func TestCreateCategoryRequest_Validate_MissingName(t *testing.T) {
	req := doc.CreateCategoryRequest{}

	err := req.Validate()
	if err == nil {
		t.Error("expected error when name is missing, got nil")
	}
	if err.Field != "name" {
		t.Errorf("expected field %q, got %q", "name", err.Field)
	}
}

func TestCreateCategoryRequest_Validate_WithParentID(t *testing.T) {
	parentID := uuid.New()
	req := doc.CreateCategoryRequest{
		Name:     "Subcategory",
		ParentID: &parentID,
	}

	if err := req.Validate(); err != nil {
		t.Errorf("expected nil error for valid request with parent_id, got: %v", err)
	}
}

func TestCreateCategoryRequest_Validate_ErrorHasMessage(t *testing.T) {
	req := doc.CreateCategoryRequest{}

	err := req.Validate()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() == "" {
		t.Error("expected non-empty error message")
	}
}
