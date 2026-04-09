package doc_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/audit"
	"github.com/quorant/quorant/internal/doc"
	"github.com/quorant/quorant/internal/platform/api"
	"github.com/quorant/quorant/internal/platform/queue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Mock repository ──────────────────────────────────────────────────────────

type mockDocRepo struct {
	docs       map[uuid.UUID]*doc.Document
	categories map[uuid.UUID]*doc.DocumentCategory
}

func newMockDocRepo() *mockDocRepo {
	return &mockDocRepo{
		docs:       make(map[uuid.UUID]*doc.Document),
		categories: make(map[uuid.UUID]*doc.DocumentCategory),
	}
}

func (r *mockDocRepo) Create(_ context.Context, d *doc.Document) (*doc.Document, error) {
	d.ID = uuid.New()
	d.VersionNumber = 1
	d.IsCurrent = true
	d.StorageKey = fmt.Sprintf("%s/%s/%s", d.OrgID, d.ID, d.FileName)
	d.CreatedAt = time.Now()
	d.UpdatedAt = time.Now()
	if d.Metadata == nil {
		d.Metadata = map[string]any{}
	}
	copy := *d
	r.docs[d.ID] = &copy
	return &copy, nil
}

func (r *mockDocRepo) FindByID(_ context.Context, id uuid.UUID) (*doc.Document, error) {
	d, ok := r.docs[id]
	if !ok {
		return nil, nil
	}
	copy := *d
	return &copy, nil
}

func (r *mockDocRepo) ListByOrg(_ context.Context, orgID uuid.UUID) ([]doc.Document, error) {
	var out []doc.Document
	for _, d := range r.docs {
		if d.OrgID == orgID && d.IsCurrent && d.DeletedAt == nil {
			out = append(out, *d)
		}
	}
	return out, nil
}

func (r *mockDocRepo) Update(_ context.Context, d *doc.Document) (*doc.Document, error) {
	existing, ok := r.docs[d.ID]
	if !ok {
		return nil, fmt.Errorf("doc not found: %s", d.ID)
	}
	existing.Title = d.Title
	existing.CategoryID = d.CategoryID
	existing.Visibility = d.Visibility
	existing.Metadata = d.Metadata
	existing.UpdatedAt = time.Now()
	copy := *existing
	return &copy, nil
}

func (r *mockDocRepo) SoftDelete(_ context.Context, id uuid.UUID) error {
	d, ok := r.docs[id]
	if !ok {
		return nil
	}
	now := time.Now()
	d.DeletedAt = &now
	return nil
}

func (r *mockDocRepo) CreateVersion(_ context.Context, parentDocID uuid.UUID, d *doc.Document) (*doc.Document, error) {
	parent, ok := r.docs[parentDocID]
	if !ok {
		return nil, fmt.Errorf("parent doc not found: %s", parentDocID)
	}
	parent.IsCurrent = false

	d.ID = uuid.New()
	d.ParentDocID = &parentDocID
	d.VersionNumber = parent.VersionNumber + 1
	d.IsCurrent = true
	d.StorageKey = fmt.Sprintf("%s/%s/%s", d.OrgID, d.ID, d.FileName)
	d.CreatedAt = time.Now()
	d.UpdatedAt = time.Now()
	if d.Metadata == nil {
		d.Metadata = map[string]any{}
	}
	copy := *d
	r.docs[d.ID] = &copy
	return &copy, nil
}

func (r *mockDocRepo) ListVersions(_ context.Context, docID uuid.UUID) ([]doc.Document, error) {
	d, ok := r.docs[docID]
	if !ok {
		return nil, nil
	}
	// Find root: if this doc has a parent, chase up.
	rootID := docID
	if d.ParentDocID != nil {
		rootID = *d.ParentDocID
	}
	var out []doc.Document
	for _, dd := range r.docs {
		if dd.ID == rootID || (dd.ParentDocID != nil && *dd.ParentDocID == rootID) {
			out = append(out, *dd)
		}
	}
	return out, nil
}

func (r *mockDocRepo) CreateCategory(_ context.Context, cat *doc.DocumentCategory) (*doc.DocumentCategory, error) {
	cat.ID = uuid.New()
	cat.CreatedAt = time.Now()
	copy := *cat
	r.categories[cat.ID] = &copy
	return &copy, nil
}

func (r *mockDocRepo) ListCategoriesByOrg(_ context.Context, orgID uuid.UUID) ([]doc.DocumentCategory, error) {
	var out []doc.DocumentCategory
	for _, c := range r.categories {
		if c.OrgID == orgID {
			out = append(out, *c)
		}
	}
	return out, nil
}

func (r *mockDocRepo) UpdateCategory(_ context.Context, cat *doc.DocumentCategory) (*doc.DocumentCategory, error) {
	existing, ok := r.categories[cat.ID]
	if !ok {
		return nil, fmt.Errorf("category not found: %s", cat.ID)
	}
	existing.Name = cat.Name
	existing.ParentID = cat.ParentID
	existing.SortOrder = cat.SortOrder
	copy := *existing
	return &copy, nil
}

func (r *mockDocRepo) DeleteCategory(_ context.Context, id uuid.UUID) error {
	delete(r.categories, id)
	return nil
}

// ─── Mock storage client ──────────────────────────────────────────────────────

type mockStorageClient struct {
	uploaded map[string][]byte
}

func newMockStorageClient() *mockStorageClient {
	return &mockStorageClient{uploaded: make(map[string][]byte)}
}

func (m *mockStorageClient) Upload(_ context.Context, bucket, key, _ string, reader io.Reader, _ int64) error {
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	m.uploaded[bucket+"/"+key] = data
	return nil
}

func (m *mockStorageClient) PresignedGetURL(_ context.Context, bucket, key string, _ time.Duration) (string, error) {
	return "https://mock.example.com/" + bucket + "/" + key + "?sig=abc", nil
}

func (m *mockStorageClient) PresignedPutURL(_ context.Context, bucket, key, _ string, _ time.Duration) (string, error) {
	return "https://mock.example.com/" + bucket + "/" + key + "?sig=put", nil
}

func (m *mockStorageClient) Delete(_ context.Context, bucket, key string) error {
	delete(m.uploaded, bucket+"/"+key)
	return nil
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func newTestService(t *testing.T) (*doc.DocService, *mockDocRepo, *mockStorageClient) {
	t.Helper()
	repo := newMockDocRepo()
	stor := newMockStorageClient()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := doc.NewDocService(repo, stor, "test-bucket", audit.NewNoopAuditor(), queue.NewInMemoryPublisher(), logger)
	return svc, repo, stor
}

// ─── Service tests ────────────────────────────────────────────────────────────

func TestUploadDocument_Success(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()

	orgID := uuid.New()
	uploadedBy := uuid.New()
	req := doc.UploadDocumentRequest{
		Title:       "CC&Rs 2024",
		FileName:    "ccrs-2024.pdf",
		ContentType: "application/pdf",
		SizeBytes:   204800,
	}

	result, err := svc.UploadDocument(ctx, orgID, req, uploadedBy)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.NotEqual(t, uuid.Nil, result.ID)
	assert.Equal(t, orgID, result.OrgID)
	assert.Equal(t, uploadedBy, result.UploadedBy)
	assert.Equal(t, "CC&Rs 2024", result.Title)
	assert.Equal(t, "ccrs-2024.pdf", result.FileName)
	assert.Equal(t, "application/pdf", result.ContentType)
	assert.Equal(t, int64(204800), result.SizeBytes)
	assert.True(t, result.IsCurrent)
	assert.Equal(t, 1, result.VersionNumber)

	// Storage key must contain org_id, doc_id, and file_name.
	assert.Contains(t, result.StorageKey, orgID.String())
	assert.Contains(t, result.StorageKey, result.ID.String())
	assert.Contains(t, result.StorageKey, "ccrs-2024.pdf")
}

func TestUploadDocument_ValidationError_MissingTitle(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()

	req := doc.UploadDocumentRequest{
		FileName:    "file.pdf",
		ContentType: "application/pdf",
		SizeBytes:   1024,
	}

	_, err := svc.UploadDocument(ctx, uuid.New(), req, uuid.New())
	require.Error(t, err)

	var valErr *api.ValidationError
	assert.ErrorAs(t, err, &valErr)
	assert.Equal(t, "title", valErr.Field)
}

func TestGetDocument_NotFound(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()

	_, err := svc.GetDocument(ctx, uuid.New())
	require.Error(t, err)

	var notFoundErr *api.NotFoundError
	assert.ErrorAs(t, err, &notFoundErr)
}

func TestGetDocument_Success(t *testing.T) {
	svc, repo, _ := newTestService(t)
	ctx := context.Background()

	orgID := uuid.New()
	// Seed via repo directly.
	d, err := repo.Create(ctx, &doc.Document{
		OrgID:       orgID,
		UploadedBy:  uuid.New(),
		Title:       "Rules",
		FileName:    "rules.pdf",
		ContentType: "application/pdf",
		SizeBytes:   512,
		Visibility:  "members",
		Metadata:    map[string]any{},
	})
	require.NoError(t, err)

	result, err := svc.GetDocument(ctx, d.ID)
	require.NoError(t, err)
	assert.Equal(t, d.ID, result.ID)
	assert.Equal(t, "Rules", result.Title)
}

func TestGetDownloadURL_ReturnsPresignedURL(t *testing.T) {
	svc, repo, _ := newTestService(t)
	ctx := context.Background()

	d, err := repo.Create(ctx, &doc.Document{
		OrgID:       uuid.New(),
		UploadedBy:  uuid.New(),
		Title:       "Budget",
		FileName:    "budget.pdf",
		ContentType: "application/pdf",
		SizeBytes:   1024,
		Visibility:  "members",
		Metadata:    map[string]any{},
	})
	require.NoError(t, err)

	url, err := svc.GetDownloadURL(ctx, d.ID)
	require.NoError(t, err)
	assert.NotEmpty(t, url)
	assert.True(t, strings.HasPrefix(url, "https://"), "expected HTTPS URL, got: %s", url)
}

func TestGetDownloadURL_NotFound(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()

	_, err := svc.GetDownloadURL(ctx, uuid.New())
	require.Error(t, err)

	var notFoundErr *api.NotFoundError
	assert.ErrorAs(t, err, &notFoundErr)
}

func TestUploadNewVersion_Success(t *testing.T) {
	svc, repo, _ := newTestService(t)
	ctx := context.Background()

	orgID := uuid.New()
	uploadedBy := uuid.New()

	// Create the original document.
	parent, err := repo.Create(ctx, &doc.Document{
		OrgID:       orgID,
		UploadedBy:  uploadedBy,
		Title:       "Bylaws v1",
		FileName:    "bylaws-v1.pdf",
		ContentType: "application/pdf",
		SizeBytes:   2048,
		Visibility:  "members",
		Metadata:    map[string]any{},
	})
	require.NoError(t, err)

	// Upload a new version.
	req := doc.UploadDocumentRequest{
		Title:       "Bylaws v2",
		FileName:    "bylaws-v2.pdf",
		ContentType: "application/pdf",
		SizeBytes:   2100,
	}

	v2, err := svc.UploadNewVersion(ctx, parent.ID, req, uploadedBy)
	require.NoError(t, err)
	require.NotNil(t, v2)

	assert.NotEqual(t, parent.ID, v2.ID)
	assert.Equal(t, orgID, v2.OrgID)
	assert.Equal(t, 2, v2.VersionNumber)
	assert.True(t, v2.IsCurrent)
	assert.NotNil(t, v2.ParentDocID)
	assert.Equal(t, parent.ID, *v2.ParentDocID)

	// Storage key must embed the new doc ID.
	assert.Contains(t, v2.StorageKey, v2.ID.String())
	assert.Contains(t, v2.StorageKey, "bylaws-v2.pdf")
}

func TestUploadNewVersion_ParentNotFound(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()

	req := doc.UploadDocumentRequest{
		Title:       "V2",
		FileName:    "v2.pdf",
		ContentType: "application/pdf",
		SizeBytes:   512,
	}

	_, err := svc.UploadNewVersion(ctx, uuid.New(), req, uuid.New())
	require.Error(t, err)
}

func TestListDocuments_ReturnsCurrentDocuments(t *testing.T) {
	svc, repo, _ := newTestService(t)
	ctx := context.Background()

	orgID := uuid.New()

	_, err := repo.Create(ctx, &doc.Document{
		OrgID: orgID, UploadedBy: uuid.New(), Title: "Doc A",
		FileName: "a.pdf", ContentType: "application/pdf", SizeBytes: 100,
		Visibility: "members", Metadata: map[string]any{},
	})
	require.NoError(t, err)

	_, err = repo.Create(ctx, &doc.Document{
		OrgID: orgID, UploadedBy: uuid.New(), Title: "Doc B",
		FileName: "b.pdf", ContentType: "application/pdf", SizeBytes: 200,
		Visibility: "members", Metadata: map[string]any{},
	})
	require.NoError(t, err)

	docs, err := svc.ListDocuments(ctx, orgID)
	require.NoError(t, err)
	assert.Len(t, docs, 2)
}

func TestCreateCategory_Success(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()

	orgID := uuid.New()
	req := doc.CreateCategoryRequest{Name: "Financial Records"}

	cat, err := svc.CreateCategory(ctx, orgID, req)
	require.NoError(t, err)
	require.NotNil(t, cat)

	assert.NotEqual(t, uuid.Nil, cat.ID)
	assert.Equal(t, orgID, cat.OrgID)
	assert.Equal(t, "Financial Records", cat.Name)
}

func TestCreateCategory_ValidationError_MissingName(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()

	req := doc.CreateCategoryRequest{}
	_, err := svc.CreateCategory(ctx, uuid.New(), req)
	require.Error(t, err)

	var valErr *api.ValidationError
	assert.ErrorAs(t, err, &valErr)
	assert.Equal(t, "name", valErr.Field)
}

func TestListCategories_ReturnsOrgCategories(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()

	orgID := uuid.New()

	_, err := svc.CreateCategory(ctx, orgID, doc.CreateCategoryRequest{Name: "Cat A"})
	require.NoError(t, err)
	_, err = svc.CreateCategory(ctx, orgID, doc.CreateCategoryRequest{Name: "Cat B"})
	require.NoError(t, err)

	cats, err := svc.ListCategories(ctx, orgID)
	require.NoError(t, err)
	assert.Len(t, cats, 2)
}

func TestDeleteCategory_RemovesCategory(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()

	orgID := uuid.New()
	cat, err := svc.CreateCategory(ctx, orgID, doc.CreateCategoryRequest{Name: "To Delete"})
	require.NoError(t, err)

	err = svc.DeleteCategory(ctx, cat.ID)
	require.NoError(t, err)

	cats, err := svc.ListCategories(ctx, orgID)
	require.NoError(t, err)
	assert.Empty(t, cats)
}
