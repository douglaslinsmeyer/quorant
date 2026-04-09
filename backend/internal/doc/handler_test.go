package doc_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/doc"
	"github.com/quorant/quorant/internal/platform/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Test server ──────────────────────────────────────────────────────────────

type docTestServer struct {
	server *httptest.Server
	repo   *mockDocRepo
}

func setupDocTestServer(t *testing.T) *docTestServer {
	t.Helper()

	repo := newMockDocRepo()
	stor := newMockStorageClient()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := doc.NewDocService(repo, stor, "test-bucket", logger)
	handler := doc.NewDocHandler(svc, logger)

	mux := http.NewServeMux()

	// Documents
	mux.HandleFunc("POST /organizations/{org_id}/documents", handler.UploadDocument)
	mux.HandleFunc("GET /organizations/{org_id}/documents", handler.ListDocuments)
	mux.HandleFunc("GET /organizations/{org_id}/documents/{document_id}", handler.GetDocument)
	mux.HandleFunc("GET /organizations/{org_id}/documents/{document_id}/download", handler.GetDownloadURL)
	mux.HandleFunc("PATCH /organizations/{org_id}/documents/{document_id}", handler.UpdateDocument)
	mux.HandleFunc("DELETE /organizations/{org_id}/documents/{document_id}", handler.DeleteDocument)
	mux.HandleFunc("POST /organizations/{org_id}/documents/{document_id}/versions", handler.UploadVersion)
	mux.HandleFunc("GET /organizations/{org_id}/documents/{document_id}/versions", handler.ListVersions)

	// Categories
	mux.HandleFunc("POST /organizations/{org_id}/document-categories", handler.CreateCategory)
	mux.HandleFunc("GET /organizations/{org_id}/document-categories", handler.ListCategories)
	mux.HandleFunc("PATCH /organizations/{org_id}/document-categories/{category_id}", handler.UpdateCategory)
	mux.HandleFunc("DELETE /organizations/{org_id}/document-categories/{category_id}", handler.DeleteCategory)

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	return &docTestServer{server: srv, repo: repo}
}

// ─── Helper functions ─────────────────────────────────────────────────────────

func doDocRequest(t *testing.T, serverURL, method, path string, body any) *http.Response {
	t.Helper()

	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		require.NoError(t, err)
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, serverURL+path, bodyReader)
	require.NoError(t, err)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

func decodeDocResponse(t *testing.T, resp *http.Response, dst any) {
	t.Helper()
	defer resp.Body.Close()
	require.NoError(t, json.NewDecoder(resp.Body).Decode(dst))
}

func seedDoc(t *testing.T, repo *mockDocRepo, orgID uuid.UUID) *doc.Document {
	t.Helper()
	import_ctx := t.Context()
	d, err := repo.Create(import_ctx, &doc.Document{
		OrgID:       orgID,
		UploadedBy:  uuid.New(),
		Title:       "Seeded Document",
		FileName:    "seeded.pdf",
		ContentType: "application/pdf",
		SizeBytes:   1024,
		Visibility:  "members",
		Metadata:    map[string]any{},
	})
	require.NoError(t, err)
	return d
}

// ─── Handler tests ────────────────────────────────────────────────────────────

func TestUploadDocument_Handler(t *testing.T) {
	ts := setupDocTestServer(t)
	orgID := uuid.New()

	body := map[string]any{
		"title":        "Bylaws",
		"file_name":    "bylaws.pdf",
		"content_type": "application/pdf",
		"size_bytes":   2048,
	}

	resp := doDocRequest(t, ts.server.URL, http.MethodPost,
		fmt.Sprintf("/organizations/%s/documents", orgID), body)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var envelope struct {
		Data doc.Document `json:"data"`
	}
	decodeDocResponse(t, resp, &envelope)

	assert.NotEqual(t, uuid.Nil, envelope.Data.ID)
	assert.Equal(t, "Bylaws", envelope.Data.Title)
	assert.Equal(t, orgID, envelope.Data.OrgID)
}

func TestUploadDocument_Handler_ValidationError(t *testing.T) {
	ts := setupDocTestServer(t)
	orgID := uuid.New()

	// Missing required fields.
	body := map[string]any{"title": "Incomplete"}

	resp := doDocRequest(t, ts.server.URL, http.MethodPost,
		fmt.Sprintf("/organizations/%s/documents", orgID), body)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestListDocuments_Handler(t *testing.T) {
	ts := setupDocTestServer(t)
	orgID := uuid.New()

	seedDoc(t, ts.repo, orgID)
	seedDoc(t, ts.repo, orgID)

	resp := doDocRequest(t, ts.server.URL, http.MethodGet,
		fmt.Sprintf("/organizations/%s/documents", orgID), nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data []doc.Document `json:"data"`
	}
	decodeDocResponse(t, resp, &envelope)
	assert.Len(t, envelope.Data, 2)
}

func TestGetDocument_Handler_NotFound(t *testing.T) {
	ts := setupDocTestServer(t)
	orgID := uuid.New()
	nonExistentID := uuid.New()

	resp := doDocRequest(t, ts.server.URL, http.MethodGet,
		fmt.Sprintf("/organizations/%s/documents/%s", orgID, nonExistentID), nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	var envelope struct {
		Errors []api.Error `json:"errors"`
	}
	decodeDocResponse(t, resp, &envelope)
	require.Len(t, envelope.Errors, 1)
	assert.Equal(t, "NOT_FOUND", envelope.Errors[0].Code)
}

func TestGetDocument_Handler_Success(t *testing.T) {
	ts := setupDocTestServer(t)
	orgID := uuid.New()
	d := seedDoc(t, ts.repo, orgID)

	resp := doDocRequest(t, ts.server.URL, http.MethodGet,
		fmt.Sprintf("/organizations/%s/documents/%s", orgID, d.ID), nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data doc.Document `json:"data"`
	}
	decodeDocResponse(t, resp, &envelope)
	assert.Equal(t, d.ID, envelope.Data.ID)
}

func TestDownload_Handler(t *testing.T) {
	ts := setupDocTestServer(t)
	orgID := uuid.New()
	d := seedDoc(t, ts.repo, orgID)

	resp := doDocRequest(t, ts.server.URL, http.MethodGet,
		fmt.Sprintf("/organizations/%s/documents/%s/download", orgID, d.ID), nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data struct {
			URL string `json:"url"`
		} `json:"data"`
	}
	decodeDocResponse(t, resp, &envelope)
	assert.NotEmpty(t, envelope.Data.URL)
}

func TestUpdateDocument_Handler(t *testing.T) {
	ts := setupDocTestServer(t)
	orgID := uuid.New()
	d := seedDoc(t, ts.repo, orgID)

	body := map[string]any{
		"title":      "Updated Title",
		"visibility": "board_only",
	}

	resp := doDocRequest(t, ts.server.URL, http.MethodPatch,
		fmt.Sprintf("/organizations/%s/documents/%s", orgID, d.ID), body)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data doc.Document `json:"data"`
	}
	decodeDocResponse(t, resp, &envelope)
	assert.Equal(t, "Updated Title", envelope.Data.Title)
}

func TestDeleteDocument_Handler(t *testing.T) {
	ts := setupDocTestServer(t)
	orgID := uuid.New()
	d := seedDoc(t, ts.repo, orgID)

	resp := doDocRequest(t, ts.server.URL, http.MethodDelete,
		fmt.Sprintf("/organizations/%s/documents/%s", orgID, d.ID), nil)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func TestUploadVersion_Handler(t *testing.T) {
	ts := setupDocTestServer(t)
	orgID := uuid.New()
	parent := seedDoc(t, ts.repo, orgID)

	body := map[string]any{
		"title":        "Bylaws v2",
		"file_name":    "bylaws-v2.pdf",
		"content_type": "application/pdf",
		"size_bytes":   3000,
	}

	resp := doDocRequest(t, ts.server.URL, http.MethodPost,
		fmt.Sprintf("/organizations/%s/documents/%s/versions", orgID, parent.ID), body)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var envelope struct {
		Data doc.Document `json:"data"`
	}
	decodeDocResponse(t, resp, &envelope)
	assert.Equal(t, 2, envelope.Data.VersionNumber)
	assert.NotNil(t, envelope.Data.ParentDocID)
}

func TestListVersions_Handler(t *testing.T) {
	ts := setupDocTestServer(t)
	orgID := uuid.New()
	parent := seedDoc(t, ts.repo, orgID)

	// Create a version via the repo directly.
	v2 := &doc.Document{
		OrgID:       orgID,
		UploadedBy:  uuid.New(),
		Title:       "v2",
		FileName:    "v2.pdf",
		ContentType: "application/pdf",
		SizeBytes:   500,
		Visibility:  "members",
		Metadata:    map[string]any{},
	}
	import_ctx := t.Context()
	_, err := ts.repo.CreateVersion(import_ctx, parent.ID, v2)
	require.NoError(t, err)

	resp := doDocRequest(t, ts.server.URL, http.MethodGet,
		fmt.Sprintf("/organizations/%s/documents/%s/versions", orgID, parent.ID), nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data []doc.Document `json:"data"`
	}
	decodeDocResponse(t, resp, &envelope)
	assert.GreaterOrEqual(t, len(envelope.Data), 2)
}

func TestCreateCategory_Handler(t *testing.T) {
	ts := setupDocTestServer(t)
	orgID := uuid.New()

	body := map[string]any{"name": "Legal Documents"}

	resp := doDocRequest(t, ts.server.URL, http.MethodPost,
		fmt.Sprintf("/organizations/%s/document-categories", orgID), body)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var envelope struct {
		Data doc.DocumentCategory `json:"data"`
	}
	decodeDocResponse(t, resp, &envelope)
	assert.NotEqual(t, uuid.Nil, envelope.Data.ID)
	assert.Equal(t, "Legal Documents", envelope.Data.Name)
}

func TestListCategories_Handler(t *testing.T) {
	ts := setupDocTestServer(t)
	orgID := uuid.New()

	// Seed categories via service.
	stor := newMockStorageClient()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := doc.NewDocService(ts.repo, stor, "test-bucket", logger)

	_, err := svc.CreateCategory(t.Context(), orgID, doc.CreateCategoryRequest{Name: "Cat 1"})
	require.NoError(t, err)

	resp := doDocRequest(t, ts.server.URL, http.MethodGet,
		fmt.Sprintf("/organizations/%s/document-categories", orgID), nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data []doc.DocumentCategory `json:"data"`
	}
	decodeDocResponse(t, resp, &envelope)
	assert.Len(t, envelope.Data, 1)
}

func TestDeleteCategory_Handler(t *testing.T) {
	ts := setupDocTestServer(t)
	orgID := uuid.New()

	stor := newMockStorageClient()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := doc.NewDocService(ts.repo, stor, "test-bucket", logger)

	cat, err := svc.CreateCategory(t.Context(), orgID, doc.CreateCategoryRequest{Name: "Temp Cat"})
	require.NoError(t, err)

	resp := doDocRequest(t, ts.server.URL, http.MethodDelete,
		fmt.Sprintf("/organizations/%s/document-categories/%s", orgID, cat.ID), nil)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}
