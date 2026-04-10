package estoppel_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/audit"
	"github.com/quorant/quorant/internal/estoppel"
	"github.com/quorant/quorant/internal/platform/middleware"
	"github.com/quorant/quorant/internal/platform/queue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Handler test setup
// ---------------------------------------------------------------------------

func newTestHandler() (*estoppel.Handler, *mockRepo) {
	repo := newMockRepo()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	svc := estoppel.NewEstoppelService(
		repo,
		&mockFinancialProvider{snapshot: &estoppel.FinancialSnapshot{}},
		&mockComplianceProvider{snapshot: &estoppel.ComplianceSnapshot{}},
		&mockPropertyProvider{snapshot: &estoppel.PropertySnapshot{UnitNumber: "1A", OrgState: "FL"}},
		flJurisdictionRulesRepo(),
		estoppel.NewNoopNarrativeGenerator(),
		&mockCertificateGenerator{},
		nil, // docUploader
		nil, // docDownloader
		audit.NewNoopAuditor(),
		queue.NewInMemoryPublisher(),
		logger,
	)
	return estoppel.NewHandler(svc, logger), repo
}

// withTestUserID is a test middleware that injects a fixed user UUID into the
// request context so that handlers requiring authentication succeed.
func withTestUserID(userID uuid.UUID, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := middleware.WithUserID(r.Context(), userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// newHandlerMux builds a minimal ServeMux wiring the handler routes with a
// stub auth middleware that injects a test user ID into the request context.
func newHandlerMux(h *estoppel.Handler) *http.ServeMux {
	return newHandlerMuxWithUserID(h, uuid.New())
}

// newHandlerMuxWithUserID builds a ServeMux with a fixed user ID injected into
// context. Useful when tests need to know the actor ID in advance.
func newHandlerMuxWithUserID(h *estoppel.Handler, testUserID uuid.UUID) *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("POST /organizations/{org_id}/estoppel/requests",
		withTestUserID(testUserID, http.HandlerFunc(h.CreateRequest)))
	mux.HandleFunc("GET /organizations/{org_id}/estoppel/requests", h.ListRequests)
	mux.HandleFunc("GET /organizations/{org_id}/estoppel/requests/{id}", h.GetRequest)
	mux.Handle("POST /organizations/{org_id}/estoppel/requests/{id}/approve",
		withTestUserID(testUserID, http.HandlerFunc(h.ApproveRequest)))
	mux.Handle("POST /organizations/{org_id}/estoppel/requests/{id}/reject",
		withTestUserID(testUserID, http.HandlerFunc(h.RejectRequest)))
	mux.Handle("PATCH /organizations/{org_id}/estoppel/requests/{id}/narratives",
		withTestUserID(testUserID, http.HandlerFunc(h.UpdateNarratives)))
	mux.HandleFunc("GET /organizations/{org_id}/estoppel/requests/{id}/preview", h.PreviewCertificate)
	mux.Handle("GET /organizations/{org_id}/estoppel/certificates/{id}/download",
		withTestUserID(testUserID, http.HandlerFunc(h.DownloadCertificate)))
	mux.Handle("POST /organizations/{org_id}/estoppel/certificates/{id}/amend",
		withTestUserID(testUserID, http.HandlerFunc(h.AmendCertificate)))
	return mux
}

// newTestHandlerWithDownloader creates a Handler backed by a service that has
// a mock DocumentDownloader wired in. Useful for DownloadCertificate tests.
func newTestHandlerWithDownloader(downloader estoppel.DocumentDownloader) (*estoppel.Handler, *mockRepo) {
	repo := newMockRepo()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	svc := estoppel.NewEstoppelService(
		repo,
		&mockFinancialProvider{snapshot: &estoppel.FinancialSnapshot{}},
		&mockComplianceProvider{snapshot: &estoppel.ComplianceSnapshot{}},
		&mockPropertyProvider{snapshot: &estoppel.PropertySnapshot{UnitNumber: "1A", OrgState: "FL"}},
		flJurisdictionRulesRepo(),
		estoppel.NewNoopNarrativeGenerator(),
		&mockCertificateGenerator{},
		nil, // docUploader
		downloader,
		audit.NewNoopAuditor(),
		queue.NewInMemoryPublisher(),
		logger,
	)
	return estoppel.NewHandler(svc, logger), repo
}

// ---------------------------------------------------------------------------
// Helper: decode response envelope
// ---------------------------------------------------------------------------

type apiResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"errors"`
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestHandler_CreateRequest_Success(t *testing.T) {
	h, _ := newTestHandler()
	mux := newHandlerMux(h)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	orgID := uuid.New()
	body := estoppel.CreateEstoppelRequestDTO{
		UnitID:          uuid.New(),
		RequestType:     "estoppel_certificate",
		RequestorType:   "title_company",
		RequestorName:   "Jane Doe",
		RequestorEmail:  "jane@title.com",
		RequestorPhone:  "555-0200",
		RequestorCompany: "Best Title",
		PropertyAddress: "456 Elm St",
		OwnerName:       "Sam Green",
	}

	b, err := json.Marshal(body)
	require.NoError(t, err)

	url := srv.URL + "/organizations/" + orgID.String() + "/estoppel/requests"
	resp, err := http.Post(url, "application/json", bytes.NewReader(b)) //nolint:noctx
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var env apiResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&env))
	assert.Nil(t, env.Errors)
	assert.NotEmpty(t, env.Data)
}

func TestHandler_CreateRequest_InvalidBody(t *testing.T) {
	h, _ := newTestHandler()
	mux := newHandlerMux(h)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	orgID := uuid.New()
	// Send empty JSON object — all required fields missing.
	url := srv.URL + "/organizations/" + orgID.String() + "/estoppel/requests"
	resp, err := http.Post(url, "application/json", bytes.NewReader([]byte(`{}`))) //nolint:noctx
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var env apiResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&env))
	assert.NotEmpty(t, env.Errors)
}

func TestHandler_ListRequests_Empty(t *testing.T) {
	h, _ := newTestHandler()
	mux := newHandlerMux(h)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	orgID := uuid.New()
	url := srv.URL + "/organizations/" + orgID.String() + "/estoppel/requests"
	resp, err := http.Get(url) //nolint:noctx
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var env apiResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&env))
	assert.Empty(t, env.Errors)
}

func TestHandler_GetRequest_NotFound(t *testing.T) {
	h, _ := newTestHandler()
	mux := newHandlerMux(h)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	orgID := uuid.New()
	unknownID := uuid.New()
	url := srv.URL + "/organizations/" + orgID.String() + "/estoppel/requests/" + unknownID.String()

	req, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestHandler_CreateRequest_MissingOrgID(t *testing.T) {
	h, _ := newTestHandler()
	mux := newHandlerMux(h)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// Route without {org_id} won't match the registered pattern, so we set up
	// a minimal check using httptest directly to verify the handler responds
	// correctly to a bad UUID in the path.
	orgID := "not-a-uuid"
	body := estoppel.CreateEstoppelRequestDTO{
		UnitID:        uuid.New(),
		RequestType:   "estoppel_certificate",
		RequestorType: "homeowner",
	}
	b, _ := json.Marshal(body)

	url := srv.URL + "/organizations/" + orgID + "/estoppel/requests"
	resp, err := http.Post(url, "application/json", bytes.NewReader(b)) //nolint:noctx
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestHandler_CreateRequest_Unauthenticated(t *testing.T) {
	h, _ := newTestHandler()
	// Build a mux WITHOUT the auth middleware so no user ID is injected.
	mux := http.NewServeMux()
	mux.HandleFunc("POST /organizations/{org_id}/estoppel/requests", h.CreateRequest)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	orgID := uuid.New()
	body := estoppel.CreateEstoppelRequestDTO{
		UnitID:          uuid.New(),
		RequestType:     "estoppel_certificate",
		RequestorType:   "title_company",
		RequestorName:   "Jane Doe",
		RequestorEmail:  "jane@title.com",
		RequestorPhone:  "555-0200",
		RequestorCompany: "Best Title",
		PropertyAddress: "456 Elm St",
		OwnerName:       "Sam Green",
	}
	b, err := json.Marshal(body)
	require.NoError(t, err)

	url := srv.URL + "/organizations/" + orgID.String() + "/estoppel/requests"
	resp, err := http.Post(url, "application/json", bytes.NewReader(b)) //nolint:noctx
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	var env apiResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&env))
	require.Len(t, env.Errors, 1)
	assert.Equal(t, "UNAUTHENTICATED", env.Errors[0].Code)
}

// ---------------------------------------------------------------------------
// Mock DocumentDownloader
// ---------------------------------------------------------------------------

type mockDocDownloader struct {
	url string
	err error
}

func (m *mockDocDownloader) GetDownloadURL(_ context.Context, _ uuid.UUID) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.url, nil
}

// ---------------------------------------------------------------------------
// Task 6: UpdateNarratives handler tests
// ---------------------------------------------------------------------------

// seedRequestAtStatus creates a request in the mock repo and walks it to the
// given status by directly updating via the repo helper.
func seedRequestAtStatus(t *testing.T, repo *mockRepo, orgID uuid.UUID, status string) *estoppel.EstoppelRequest {
	t.Helper()
	req := &estoppel.EstoppelRequest{
		ID:              uuid.New(),
		OrgID:           orgID,
		UnitID:          uuid.New(),
		RequestType:     "estoppel_certificate",
		RequestorType:   "title_company",
		RequestorName:   "Test User",
		RequestorEmail:  "test@example.com",
		PropertyAddress: "123 Test St",
		OwnerName:       "Owner Name",
		Status:          status,
		DeadlineAt:      time.Now().AddDate(0, 0, 10),
		CreatedBy:       uuid.New(),
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
		Metadata:        map[string]any{},
	}
	repo.mu.Lock()
	repo.requests[req.ID] = req
	repo.mu.Unlock()
	return req
}

func TestHandler_UpdateNarratives_Success(t *testing.T) {
	h, repo := newTestHandler()
	orgID := uuid.New()

	// Seed a request in manager_review status.
	req := seedRequestAtStatus(t, repo, orgID, "manager_review")

	mux := newHandlerMux(h)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	body := estoppel.UpdateNarrativesDTO{
		Narratives: estoppel.NarrativeSections{
			AssessmentSummary: []estoppel.NarrativeField{
				{Label: "Summary", Value: "All dues are current."},
			},
		},
	}
	b, err := json.Marshal(body)
	require.NoError(t, err)

	url := srv.URL + "/organizations/" + orgID.String() + "/estoppel/requests/" + req.ID.String() + "/narratives"
	httpReq, err := http.NewRequest(http.MethodPatch, url, bytes.NewReader(b)) //nolint:noctx
	require.NoError(t, err)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var env apiResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&env))
	assert.Empty(t, env.Errors)
	assert.NotEmpty(t, env.Data)
}

func TestHandler_UpdateNarratives_WrongStatus(t *testing.T) {
	h, repo := newTestHandler()
	orgID := uuid.New()

	// Seed a request in submitted status (not manager_review).
	req := seedRequestAtStatus(t, repo, orgID, "submitted")

	mux := newHandlerMux(h)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	body := estoppel.UpdateNarrativesDTO{
		Narratives: estoppel.NarrativeSections{},
	}
	b, err := json.Marshal(body)
	require.NoError(t, err)

	url := srv.URL + "/organizations/" + orgID.String() + "/estoppel/requests/" + req.ID.String() + "/narratives"
	httpReq, err := http.NewRequest(http.MethodPatch, url, bytes.NewReader(b)) //nolint:noctx
	require.NoError(t, err)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var env apiResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&env))
	assert.NotEmpty(t, env.Errors)
}

func TestHandler_UpdateNarratives_NotFound(t *testing.T) {
	h, _ := newTestHandler()
	mux := newHandlerMux(h)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	orgID := uuid.New()
	unknownID := uuid.New()

	body := estoppel.UpdateNarrativesDTO{}
	b, err := json.Marshal(body)
	require.NoError(t, err)

	url := srv.URL + "/organizations/" + orgID.String() + "/estoppel/requests/" + unknownID.String() + "/narratives"
	httpReq, err := http.NewRequest(http.MethodPatch, url, bytes.NewReader(b)) //nolint:noctx
	require.NoError(t, err)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestHandler_UpdateNarratives_Unauthenticated(t *testing.T) {
	h, repo := newTestHandler()
	orgID := uuid.New()
	req := seedRequestAtStatus(t, repo, orgID, "manager_review")

	// Wire handler directly without the auth middleware.
	mux := http.NewServeMux()
	mux.HandleFunc("PATCH /organizations/{org_id}/estoppel/requests/{id}/narratives", h.UpdateNarratives)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	body := estoppel.UpdateNarrativesDTO{}
	b, err := json.Marshal(body)
	require.NoError(t, err)

	url := srv.URL + "/organizations/" + orgID.String() + "/estoppel/requests/" + req.ID.String() + "/narratives"
	httpReq, err := http.NewRequest(http.MethodPatch, url, bytes.NewReader(b)) //nolint:noctx
	require.NoError(t, err)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

// ---------------------------------------------------------------------------
// Task 7: PreviewCertificate handler tests
// ---------------------------------------------------------------------------

func TestHandler_PreviewCertificate_Success(t *testing.T) {
	h, repo := newTestHandler()
	orgID := uuid.New()

	// Seed a request in any status — preview doesn't check status.
	req := seedRequestAtStatus(t, repo, orgID, "manager_review")

	mux := newHandlerMux(h)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	url := srv.URL + "/organizations/" + orgID.String() + "/estoppel/requests/" + req.ID.String() + "/preview"
	resp, err := http.Get(url) //nolint:noctx
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/pdf", resp.Header.Get("Content-Type"))
	assert.Contains(t, resp.Header.Get("Content-Disposition"), "preview-"+req.ID.String())

	// Body should contain the mock PDF bytes.
	var buf bytes.Buffer
	_, err = buf.ReadFrom(resp.Body)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "%PDF")
}

func TestHandler_PreviewCertificate_NotFound(t *testing.T) {
	h, _ := newTestHandler()
	mux := newHandlerMux(h)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	orgID := uuid.New()
	unknownID := uuid.New()
	url := srv.URL + "/organizations/" + orgID.String() + "/estoppel/requests/" + unknownID.String() + "/preview"

	resp, err := http.Get(url) //nolint:noctx
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// ---------------------------------------------------------------------------
// Task 8: DownloadCertificate handler tests
// ---------------------------------------------------------------------------

func TestHandler_DownloadCertificate_Success(t *testing.T) {
	expectedURL := "https://storage.example.com/signed/cert.pdf?token=abc"
	downloader := &mockDocDownloader{url: expectedURL}

	h, repo := newTestHandlerWithDownloader(downloader)
	orgID := uuid.New()

	// Seed a certificate with a DocumentID.
	docID := uuid.New()
	certID := uuid.New()
	cert := &estoppel.EstoppelCertificate{
		ID:           certID,
		RequestID:    uuid.New(),
		OrgID:        orgID,
		UnitID:       uuid.New(),
		DocumentID:   &docID,
		EffectiveDate: time.Now(),
		SignedBy:     uuid.New(),
		SignedAt:     time.Now(),
		SignerTitle:  "Manager",
		TemplateVersion: "1.0",
		CreatedAt:    time.Now(),
	}
	repo.mu.Lock()
	repo.certificates[certID] = cert
	repo.mu.Unlock()

	mux := newHandlerMux(h)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	url := srv.URL + "/organizations/" + orgID.String() + "/estoppel/certificates/" + certID.String() + "/download"
	resp, err := http.Get(url) //nolint:noctx
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var env apiResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&env))
	assert.Empty(t, env.Errors)

	// The data should contain the URL.
	assert.Contains(t, string(env.Data), expectedURL)
}

func TestHandler_DownloadCertificate_NotFound(t *testing.T) {
	h, _ := newTestHandlerWithDownloader(&mockDocDownloader{url: "https://example.com"})
	mux := newHandlerMux(h)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	orgID := uuid.New()
	unknownID := uuid.New()
	url := srv.URL + "/organizations/" + orgID.String() + "/estoppel/certificates/" + unknownID.String() + "/download"

	resp, err := http.Get(url) //nolint:noctx
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestHandler_DownloadCertificate_NoDocumentID(t *testing.T) {
	downloader := &mockDocDownloader{url: "https://example.com"}
	h, repo := newTestHandlerWithDownloader(downloader)
	orgID := uuid.New()

	// Seed a certificate WITHOUT a DocumentID.
	certID := uuid.New()
	cert := &estoppel.EstoppelCertificate{
		ID:           certID,
		RequestID:    uuid.New(),
		OrgID:        orgID,
		UnitID:       uuid.New(),
		DocumentID:   nil, // no document
		EffectiveDate: time.Now(),
		SignedBy:     uuid.New(),
		SignedAt:     time.Now(),
		SignerTitle:  "Manager",
		TemplateVersion: "1.0",
		CreatedAt:    time.Now(),
	}
	repo.mu.Lock()
	repo.certificates[certID] = cert
	repo.mu.Unlock()

	mux := newHandlerMux(h)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	url := srv.URL + "/organizations/" + orgID.String() + "/estoppel/certificates/" + certID.String() + "/download"
	resp, err := http.Get(url) //nolint:noctx
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
}

func TestHandler_DownloadCertificate_Unauthenticated(t *testing.T) {
	h, _ := newTestHandlerWithDownloader(&mockDocDownloader{url: "https://example.com"})

	mux := http.NewServeMux()
	mux.HandleFunc("GET /organizations/{org_id}/estoppel/certificates/{id}/download", h.DownloadCertificate)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	orgID := uuid.New()
	url := srv.URL + "/organizations/" + orgID.String() + "/estoppel/certificates/" + uuid.New().String() + "/download"

	resp, err := http.Get(url) //nolint:noctx
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

// ---------------------------------------------------------------------------
// Task 9: AmendCertificate handler tests
// ---------------------------------------------------------------------------

// seedCertWithRequest creates an EstoppelRequest and a linked certificate in
// the mock repo. Returns both.
func seedCertWithRequest(t *testing.T, repo *mockRepo, orgID uuid.UUID) (*estoppel.EstoppelRequest, *estoppel.EstoppelCertificate) {
	t.Helper()
	req := seedRequestAtStatus(t, repo, orgID, "delivered")

	certID := uuid.New()
	cert := &estoppel.EstoppelCertificate{
		ID:           certID,
		RequestID:    req.ID,
		OrgID:        orgID,
		UnitID:       req.UnitID,
		EffectiveDate: time.Now(),
		SignedBy:     uuid.New(),
		SignedAt:     time.Now(),
		SignerTitle:  "Manager",
		TemplateVersion: "1.0",
		CreatedAt:    time.Now(),
	}
	repo.mu.Lock()
	repo.certificates[certID] = cert
	repo.mu.Unlock()

	return req, cert
}

func TestHandler_AmendCertificate_Success(t *testing.T) {
	h, repo := newTestHandler()
	orgID := uuid.New()

	_, cert := seedCertWithRequest(t, repo, orgID)

	mux := newHandlerMux(h)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	body := estoppel.AmendCertificateDTO{Reason: "incorrect balance reported"}
	b, err := json.Marshal(body)
	require.NoError(t, err)

	url := srv.URL + "/organizations/" + orgID.String() + "/estoppel/certificates/" + cert.ID.String() + "/amend"
	resp, err := http.Post(url, "application/json", bytes.NewReader(b)) //nolint:noctx
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var env apiResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&env))
	assert.Empty(t, env.Errors)
	assert.NotEmpty(t, env.Data)

	// Verify the returned request has amendment_of set.
	var newReq estoppel.EstoppelRequest
	require.NoError(t, json.Unmarshal(env.Data, &newReq))
	require.NotNil(t, newReq.AmendmentOf)
	assert.Equal(t, cert.ID, *newReq.AmendmentOf)
	assert.Equal(t, "submitted", newReq.Status)
}

func TestHandler_AmendCertificate_NoBody(t *testing.T) {
	h, repo := newTestHandler()
	orgID := uuid.New()

	_, cert := seedCertWithRequest(t, repo, orgID)

	mux := newHandlerMux(h)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// POST with no body (empty) should succeed since the DTO is optional.
	url := srv.URL + "/organizations/" + orgID.String() + "/estoppel/certificates/" + cert.ID.String() + "/amend"
	resp, err := http.Post(url, "application/json", strings.NewReader("")) //nolint:noctx
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}

func TestHandler_AmendCertificate_NotFound(t *testing.T) {
	h, _ := newTestHandler()
	mux := newHandlerMux(h)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	orgID := uuid.New()
	unknownID := uuid.New()

	url := srv.URL + "/organizations/" + orgID.String() + "/estoppel/certificates/" + unknownID.String() + "/amend"
	resp, err := http.Post(url, "application/json", strings.NewReader("")) //nolint:noctx
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestHandler_AmendCertificate_Unauthenticated(t *testing.T) {
	h, repo := newTestHandler()
	orgID := uuid.New()
	_, cert := seedCertWithRequest(t, repo, orgID)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /organizations/{org_id}/estoppel/certificates/{id}/amend", h.AmendCertificate)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	url := srv.URL + "/organizations/" + orgID.String() + "/estoppel/certificates/" + cert.ID.String() + "/amend"
	resp, err := http.Post(url, "application/json", strings.NewReader("")) //nolint:noctx
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
