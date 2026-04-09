package org_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/audit"
	"github.com/quorant/quorant/internal/iam"
	"github.com/quorant/quorant/internal/org"
	"github.com/quorant/quorant/internal/platform/auth"
	"github.com/quorant/quorant/internal/platform/queue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Test server setup ────────────────────────────────────────────────────────

type orgTestServer struct {
	server      *httptest.Server
	mockOrgRepo *mockOrgRepo
	mockUserRepo *mockUserRepo
}

func setupOrgTestServer(t *testing.T) *orgTestServer {
	t.Helper()

	mockOrgRepo := newMockOrgRepo()
	mockMembershipRepo := newMockMembershipRepo()
	mockUnitRepo := newMockUnitRepo()
	mockUserRepo := newMockUserRepo()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	service := org.NewOrgService(mockOrgRepo, mockMembershipRepo, mockUnitRepo, mockUserRepo, audit.NewNoopAuditor(), queue.NewInMemoryPublisher(), logger)
	handler := org.NewOrgHandler(service, logger)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/organizations", handler.CreateOrg)
	mux.HandleFunc("GET /api/v1/organizations", handler.ListOrgs)
	mux.HandleFunc("GET /api/v1/organizations/{org_id}", handler.GetOrg)
	mux.HandleFunc("PATCH /api/v1/organizations/{org_id}", handler.UpdateOrg)
	mux.HandleFunc("DELETE /api/v1/organizations/{org_id}", handler.DeleteOrg)
	mux.HandleFunc("GET /api/v1/organizations/{org_id}/children", handler.ListChildren)
	mux.HandleFunc("POST /api/v1/organizations/{org_id}/management", handler.ConnectManagement)
	mux.HandleFunc("DELETE /api/v1/organizations/{org_id}/management", handler.DisconnectManagement)
	mux.HandleFunc("GET /api/v1/organizations/{org_id}/management/history", handler.GetManagementHistory)

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	return &orgTestServer{
		server:      server,
		mockOrgRepo: mockOrgRepo,
		mockUserRepo: mockUserRepo,
	}
}

// doRequest sends an HTTP request to the test server. An optional body is
// JSON-encoded and set as the request body. The response body is decoded into
// dst when dst is non-nil.
func doRequest(t *testing.T, ts *orgTestServer, method, path string, body any) *http.Response {
	t.Helper()

	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		require.NoError(t, err)
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, ts.server.URL+path, bodyReader)
	require.NoError(t, err)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

// doRequestWithClaims is like doRequest but injects JWT claims into the request
// context via a wrapping handler. Because httptest.Server is already running we
// instead embed the claims in the test using a custom transport that injects a
// context. For simplicity we seed the mock user repo and add an auth header
// that a thin middleware would normally set — here we use a helper server that
// wraps the real mux and injects the context.
func setupOrgTestServerWithUser(t *testing.T) (*orgTestServer, *iam.User) {
	t.Helper()

	mockOrgRepo := newMockOrgRepo()
	mockMembershipRepo := newMockMembershipRepo()
	mockUnitRepo := newMockUnitRepo()
	mockUserRepo := newMockUserRepo()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Pre-seed a user in the mock repo.
	testUser := &iam.User{
		ID:          uuid.New(),
		IDPUserID:   "test-subject",
		Email:       "test@example.com",
		DisplayName: "Test User",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	mockUserRepo.users["test-subject"] = testUser

	service := org.NewOrgService(mockOrgRepo, mockMembershipRepo, mockUnitRepo, mockUserRepo, audit.NewNoopAuditor(), queue.NewInMemoryPublisher(), logger)
	handler := org.NewOrgHandler(service, logger)

	// Wrap the real mux with a middleware that injects auth claims.
	inner := http.NewServeMux()
	inner.HandleFunc("POST /api/v1/organizations", handler.CreateOrg)
	inner.HandleFunc("GET /api/v1/organizations", handler.ListOrgs)
	inner.HandleFunc("GET /api/v1/organizations/{org_id}", handler.GetOrg)
	inner.HandleFunc("PATCH /api/v1/organizations/{org_id}", handler.UpdateOrg)
	inner.HandleFunc("DELETE /api/v1/organizations/{org_id}", handler.DeleteOrg)
	inner.HandleFunc("GET /api/v1/organizations/{org_id}/children", handler.ListChildren)
	inner.HandleFunc("POST /api/v1/organizations/{org_id}/management", handler.ConnectManagement)
	inner.HandleFunc("DELETE /api/v1/organizations/{org_id}/management", handler.DisconnectManagement)
	inner.HandleFunc("GET /api/v1/organizations/{org_id}/management/history", handler.GetManagementHistory)

	// Middleware that injects claims when the X-Test-Auth header is present.
	wrapped := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Test-Auth") == "true" {
			ctx := auth.WithClaims(r.Context(), &auth.Claims{
				Subject: "test-subject",
				Email:   "test@example.com",
				Name:    "Test User",
			})
			r = r.WithContext(ctx)
		}
		inner.ServeHTTP(w, r)
	})

	server := httptest.NewServer(wrapped)
	t.Cleanup(server.Close)

	ts := &orgTestServer{
		server:      server,
		mockOrgRepo: mockOrgRepo,
		mockUserRepo: mockUserRepo,
	}
	return ts, testUser
}

// doAuthRequest sends an authenticated request with claims injected via middleware.
func doAuthRequest(t *testing.T, ts *orgTestServer, method, path string, body any) *http.Response {
	t.Helper()

	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		require.NoError(t, err)
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, ts.server.URL+path, bodyReader)
	require.NoError(t, err)
	req.Header.Set("X-Test-Auth", "true")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

// decodeBody JSON-decodes the response body into dst.
func decodeBody(t *testing.T, resp *http.Response, dst any) {
	t.Helper()
	defer resp.Body.Close()
	require.NoError(t, json.NewDecoder(resp.Body).Decode(dst))
}

// seedOrg pre-populates the mock org repo with an organization and returns it.
func seedOrg(t *testing.T, mockRepo *mockOrgRepo, orgType string) *org.Organization {
	t.Helper()
	o := &org.Organization{
		ID:        uuid.New(),
		Type:      orgType,
		Name:      fmt.Sprintf("Test %s", orgType),
		Settings:  map[string]any{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	mockRepo.orgs[o.ID] = o
	return o
}

// ─── CreateOrg tests ──────────────────────────────────────────────────────────

func TestCreateOrg_Success(t *testing.T) {
	ts := setupOrgTestServer(t)

	body := map[string]any{
		"type": "hoa",
		"name": "Sunset Ridge HOA",
	}
	resp := doRequest(t, ts, http.MethodPost, "/api/v1/organizations", body)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var envelope struct {
		Data *org.Organization `json:"data"`
	}
	decodeBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, "Sunset Ridge HOA", envelope.Data.Name)
	assert.Equal(t, "hoa", envelope.Data.Type)
	assert.NotEqual(t, uuid.Nil, envelope.Data.ID)
}

func TestCreateOrg_InvalidBody_MissingName(t *testing.T) {
	ts := setupOrgTestServer(t)

	body := map[string]any{"type": "hoa"}
	resp := doRequest(t, ts, http.MethodPost, "/api/v1/organizations", body)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestCreateOrg_InvalidBody_MalformedJSON(t *testing.T) {
	ts := setupOrgTestServer(t)

	req, err := http.NewRequest(http.MethodPost, ts.server.URL+"/api/v1/organizations", bytes.NewBufferString("{bad json"))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// ─── ListOrgs tests ───────────────────────────────────────────────────────────

func TestListOrgs_Success(t *testing.T) {
	ts, _ := setupOrgTestServerWithUser(t)
	seedOrg(t, ts.mockOrgRepo, "hoa")
	seedOrg(t, ts.mockOrgRepo, "firm")

	resp := doAuthRequest(t, ts, http.MethodGet, "/api/v1/organizations", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data []org.Organization `json:"data"`
	}
	decodeBody(t, resp, &envelope)
	assert.Len(t, envelope.Data, 2)
}

func TestListOrgs_Unauthenticated(t *testing.T) {
	ts, _ := setupOrgTestServerWithUser(t)

	// No X-Test-Auth header — no claims in context.
	resp := doRequest(t, ts, http.MethodGet, "/api/v1/organizations", nil)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

// ─── GetOrg tests ─────────────────────────────────────────────────────────────

func TestGetOrg_Success(t *testing.T) {
	ts := setupOrgTestServer(t)
	o := seedOrg(t, ts.mockOrgRepo, "hoa")

	resp := doRequest(t, ts, http.MethodGet, "/api/v1/organizations/"+o.ID.String(), nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data *org.Organization `json:"data"`
	}
	decodeBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, o.ID, envelope.Data.ID)
}

func TestGetOrg_InvalidUUID(t *testing.T) {
	ts := setupOrgTestServer(t)

	resp := doRequest(t, ts, http.MethodGet, "/api/v1/organizations/bad-uuid", nil)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestGetOrg_NotFound(t *testing.T) {
	ts := setupOrgTestServer(t)

	resp := doRequest(t, ts, http.MethodGet, "/api/v1/organizations/"+uuid.New().String(), nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// ─── UpdateOrg tests ──────────────────────────────────────────────────────────

func TestUpdateOrg_Success(t *testing.T) {
	ts := setupOrgTestServer(t)
	o := seedOrg(t, ts.mockOrgRepo, "hoa")

	newName := "Updated HOA Name"
	body := map[string]any{"name": newName}
	resp := doRequest(t, ts, http.MethodPatch, "/api/v1/organizations/"+o.ID.String(), body)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data *org.Organization `json:"data"`
	}
	decodeBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, newName, envelope.Data.Name)
}

func TestUpdateOrg_InvalidUUID(t *testing.T) {
	ts := setupOrgTestServer(t)

	body := map[string]any{"name": "New Name"}
	resp := doRequest(t, ts, http.MethodPatch, "/api/v1/organizations/bad-uuid", body)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestUpdateOrg_NotFound(t *testing.T) {
	ts := setupOrgTestServer(t)

	body := map[string]any{"name": "New Name"}
	resp := doRequest(t, ts, http.MethodPatch, "/api/v1/organizations/"+uuid.New().String(), body)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// ─── DeleteOrg tests ──────────────────────────────────────────────────────────

func TestDeleteOrg_Success(t *testing.T) {
	ts := setupOrgTestServer(t)
	o := seedOrg(t, ts.mockOrgRepo, "hoa")

	resp := doRequest(t, ts, http.MethodDelete, "/api/v1/organizations/"+o.ID.String(), nil)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	resp.Body.Close()

	// Verify removed from mock.
	assert.Empty(t, ts.mockOrgRepo.orgs)
}

func TestDeleteOrg_InvalidUUID(t *testing.T) {
	ts := setupOrgTestServer(t)

	resp := doRequest(t, ts, http.MethodDelete, "/api/v1/organizations/bad-uuid", nil)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// ─── ListChildren tests ───────────────────────────────────────────────────────

func TestListChildren_Success(t *testing.T) {
	ts := setupOrgTestServer(t)
	parent := seedOrg(t, ts.mockOrgRepo, "firm")

	// Add two child HOAs that reference the parent.
	child1 := seedOrg(t, ts.mockOrgRepo, "hoa")
	child1.ParentID = &parent.ID
	ts.mockOrgRepo.orgs[child1.ID] = child1

	child2 := seedOrg(t, ts.mockOrgRepo, "hoa")
	child2.ParentID = &parent.ID
	ts.mockOrgRepo.orgs[child2.ID] = child2

	resp := doRequest(t, ts, http.MethodGet, "/api/v1/organizations/"+parent.ID.String()+"/children", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data []org.Organization `json:"data"`
	}
	decodeBody(t, resp, &envelope)
	assert.Len(t, envelope.Data, 2)
}

func TestListChildren_InvalidUUID(t *testing.T) {
	ts := setupOrgTestServer(t)

	resp := doRequest(t, ts, http.MethodGet, "/api/v1/organizations/bad-uuid/children", nil)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// ─── ConnectManagement tests ──────────────────────────────────────────────────

func TestConnectManagementHandler_Success(t *testing.T) {
	ts := setupOrgTestServer(t)
	hoa := seedOrg(t, ts.mockOrgRepo, "hoa")
	firm := seedOrg(t, ts.mockOrgRepo, "firm")

	body := map[string]any{"firm_org_id": firm.ID}
	resp := doRequest(t, ts, http.MethodPost, "/api/v1/organizations/"+hoa.ID.String()+"/management", body)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var envelope struct {
		Data *org.OrgManagement `json:"data"`
	}
	decodeBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, hoa.ID, envelope.Data.HOAOrgID)
	assert.Equal(t, firm.ID, envelope.Data.FirmOrgID)
}

func TestConnectManagement_InvalidUUID(t *testing.T) {
	ts := setupOrgTestServer(t)

	body := map[string]any{"firm_org_id": uuid.New()}
	resp := doRequest(t, ts, http.MethodPost, "/api/v1/organizations/bad-uuid/management", body)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestConnectManagement_HOANotFound(t *testing.T) {
	ts := setupOrgTestServer(t)

	body := map[string]any{"firm_org_id": uuid.New()}
	resp := doRequest(t, ts, http.MethodPost, "/api/v1/organizations/"+uuid.New().String()+"/management", body)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// ─── DisconnectManagement tests ───────────────────────────────────────────────

func TestDisconnectManagementHandler_Success(t *testing.T) {
	ts := setupOrgTestServer(t)
	hoa := seedOrg(t, ts.mockOrgRepo, "hoa")
	firm := seedOrg(t, ts.mockOrgRepo, "firm")

	// First connect so there's something to disconnect.
	ts.mockOrgRepo.management = append(ts.mockOrgRepo.management, &org.OrgManagement{
		ID:        uuid.New(),
		FirmOrgID: firm.ID,
		HOAOrgID:  hoa.ID,
		StartedAt: time.Now(),
		CreatedAt: time.Now(),
	})

	resp := doRequest(t, ts, http.MethodDelete, "/api/v1/organizations/"+hoa.ID.String()+"/management", nil)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	resp.Body.Close()
}

func TestDisconnectManagement_InvalidUUID(t *testing.T) {
	ts := setupOrgTestServer(t)

	resp := doRequest(t, ts, http.MethodDelete, "/api/v1/organizations/bad-uuid/management", nil)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// ─── GetManagementHistory tests ───────────────────────────────────────────────

func TestGetManagementHistory_Success(t *testing.T) {
	ts := setupOrgTestServer(t)
	hoa := seedOrg(t, ts.mockOrgRepo, "hoa")
	firm := seedOrg(t, ts.mockOrgRepo, "firm")

	now := time.Now()
	ended := now.Add(-24 * time.Hour)
	ts.mockOrgRepo.management = append(ts.mockOrgRepo.management,
		&org.OrgManagement{
			ID:        uuid.New(),
			FirmOrgID: firm.ID,
			HOAOrgID:  hoa.ID,
			StartedAt: now.Add(-48 * time.Hour),
			EndedAt:   &ended,
			CreatedAt: now.Add(-48 * time.Hour),
		},
		&org.OrgManagement{
			ID:        uuid.New(),
			FirmOrgID: firm.ID,
			HOAOrgID:  hoa.ID,
			StartedAt: now,
			CreatedAt: now,
		},
	)

	resp := doRequest(t, ts, http.MethodGet, "/api/v1/organizations/"+hoa.ID.String()+"/management/history", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data []org.OrgManagement `json:"data"`
	}
	decodeBody(t, resp, &envelope)
	assert.Len(t, envelope.Data, 2)
}

func TestGetManagementHistory_InvalidUUID(t *testing.T) {
	ts := setupOrgTestServer(t)

	resp := doRequest(t, ts, http.MethodGet, "/api/v1/organizations/bad-uuid/management/history", nil)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestGetManagementHistory_EmptyForUnknownOrg(t *testing.T) {
	ts := setupOrgTestServer(t)

	resp := doRequest(t, ts, http.MethodGet, "/api/v1/organizations/"+uuid.New().String()+"/management/history", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data []org.OrgManagement `json:"data"`
	}
	decodeBody(t, resp, &envelope)
	assert.Empty(t, envelope.Data)
}
