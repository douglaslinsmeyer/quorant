package org_test

import (
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
	"github.com/quorant/quorant/internal/platform/queue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Test server setup ────────────────────────────────────────────────────────

type membershipTestServer struct {
	server             *httptest.Server
	mockMembershipRepo *mockMembershipRepo
	mockOrgRepo        *mockOrgRepo
}

func setupMembershipTestServer(t *testing.T) *membershipTestServer {
	t.Helper()

	mockOrgRepo := newMockOrgRepo()
	mockMembershipRepo := newMockMembershipRepo()
	mockUnitRepo := newMockUnitRepo()
	mockUserRepo := newMockUserRepo()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	service := org.NewOrgService(mockOrgRepo, mockMembershipRepo, mockUnitRepo, mockUserRepo, audit.NewNoopAuditor(), queue.NewInMemoryPublisher(), logger)
	handler := org.NewMembershipHandler(service, logger)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/organizations/{org_id}/memberships", handler.CreateMembership)
	mux.HandleFunc("GET /api/v1/organizations/{org_id}/memberships", handler.ListMemberships)
	mux.HandleFunc("GET /api/v1/organizations/{org_id}/memberships/{membership_id}", handler.GetMembership)
	mux.HandleFunc("PATCH /api/v1/organizations/{org_id}/memberships/{membership_id}", handler.UpdateMembership)
	mux.HandleFunc("DELETE /api/v1/organizations/{org_id}/memberships/{membership_id}", handler.DeleteMembership)

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	return &membershipTestServer{
		server:             server,
		mockMembershipRepo: mockMembershipRepo,
		mockOrgRepo:        mockOrgRepo,
	}
}

// doMembershipRequest sends an HTTP request to the membership test server.
func doMembershipRequest(t *testing.T, ts *membershipTestServer, method, path string, body any) *http.Response {
	t.Helper()

	// Reuse the orgTestServer-compatible doRequest by wrapping ts in a temporary orgTestServer.
	proxy := &orgTestServer{server: ts.server, mockOrgRepo: ts.mockOrgRepo}
	return doRequest(t, proxy, method, path, body)
}

// seedMembership pre-populates the mock membership repo and returns the record.
func seedMembership(t *testing.T, repo *mockMembershipRepo, orgID uuid.UUID) *iam.Membership {
	t.Helper()
	m := &iam.Membership{
		ID:        uuid.New(),
		OrgID:     orgID,
		UserID:    uuid.New(),
		RoleID:    uuid.New(),
		Status:    "invited",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	repo.memberships[m.ID] = m
	return m
}

// ─── CreateMembership tests ───────────────────────────────────────────────────

func TestCreateMembershipHandler_Success(t *testing.T) {
	ts := setupMembershipTestServer(t)
	orgID := uuid.New()

	body := map[string]any{
		"user_id": uuid.New(),
		"role_id": uuid.New(),
	}
	path := "/api/v1/organizations/" + orgID.String() + "/memberships"
	resp := doMembershipRequest(t, ts, http.MethodPost, path, body)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var envelope struct {
		Data *iam.Membership `json:"data"`
	}
	decodeBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, orgID, envelope.Data.OrgID)
	assert.NotEqual(t, uuid.Nil, envelope.Data.ID)
	assert.Equal(t, "invited", envelope.Data.Status)
}

func TestCreateMembershipHandler_InvalidBody(t *testing.T) {
	ts := setupMembershipTestServer(t)
	orgID := uuid.New()

	// Missing user_id — zero UUID should fail validation.
	body := map[string]any{
		"role_id": uuid.New(),
	}
	path := "/api/v1/organizations/" + orgID.String() + "/memberships"
	resp := doMembershipRequest(t, ts, http.MethodPost, path, body)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// ─── ListMemberships tests ────────────────────────────────────────────────────

func TestListMembershipsHandler_Success(t *testing.T) {
	ts := setupMembershipTestServer(t)
	orgID := uuid.New()

	seedMembership(t, ts.mockMembershipRepo, orgID)
	seedMembership(t, ts.mockMembershipRepo, orgID)

	path := "/api/v1/organizations/" + orgID.String() + "/memberships"
	resp := doMembershipRequest(t, ts, http.MethodGet, path, nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data []iam.Membership `json:"data"`
	}
	decodeBody(t, resp, &envelope)
	assert.Len(t, envelope.Data, 2)
}

// ─── GetMembership tests ──────────────────────────────────────────────────────

func TestGetMembershipHandler_NotFound(t *testing.T) {
	ts := setupMembershipTestServer(t)
	orgID := uuid.New()

	path := "/api/v1/organizations/" + orgID.String() + "/memberships/" + uuid.New().String()
	resp := doMembershipRequest(t, ts, http.MethodGet, path, nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// ─── UpdateMembership tests ───────────────────────────────────────────────────

func TestUpdateMembershipHandler_Success(t *testing.T) {
	ts := setupMembershipTestServer(t)
	orgID := uuid.New()
	m := seedMembership(t, ts.mockMembershipRepo, orgID)

	newStatus := "active"
	body := map[string]any{
		"status": newStatus,
	}
	path := "/api/v1/organizations/" + orgID.String() + "/memberships/" + m.ID.String()
	resp := doMembershipRequest(t, ts, http.MethodPatch, path, body)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data *iam.Membership `json:"data"`
	}
	decodeBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, newStatus, envelope.Data.Status)
}

// ─── DeleteMembership tests ───────────────────────────────────────────────────

func TestDeleteMembershipHandler_Success(t *testing.T) {
	ts := setupMembershipTestServer(t)
	orgID := uuid.New()
	m := seedMembership(t, ts.mockMembershipRepo, orgID)

	path := "/api/v1/organizations/" + orgID.String() + "/memberships/" + m.ID.String()
	resp := doMembershipRequest(t, ts, http.MethodDelete, path, nil)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	resp.Body.Close()

	// Verify removed from mock.
	assert.NotContains(t, ts.mockMembershipRepo.memberships, m.ID)
}
