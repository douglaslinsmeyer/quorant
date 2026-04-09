package org_test

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/org"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Test server setup ────────────────────────────────────────────────────────

type unitTestServer struct {
	server      *httptest.Server
	mockUnitRepo *mockUnitRepo
	mockOrgRepo  *mockOrgRepo
}

func setupUnitTestServer(t *testing.T) *unitTestServer {
	t.Helper()

	mockOrgRepo := newMockOrgRepo()
	mockMembershipRepo := newMockMembershipRepo()
	mockUnitRepo := newMockUnitRepo()
	mockUserRepo := newMockUserRepo()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	service := org.NewOrgService(mockOrgRepo, mockMembershipRepo, mockUnitRepo, mockUserRepo, logger)
	handler := org.NewUnitHandler(service, logger)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/organizations/{org_id}/units", handler.CreateUnit)
	mux.HandleFunc("GET /api/v1/organizations/{org_id}/units", handler.ListUnits)
	mux.HandleFunc("GET /api/v1/organizations/{org_id}/units/{unit_id}", handler.GetUnit)
	mux.HandleFunc("PATCH /api/v1/organizations/{org_id}/units/{unit_id}", handler.UpdateUnit)
	mux.HandleFunc("DELETE /api/v1/organizations/{org_id}/units/{unit_id}", handler.DeleteUnit)
	mux.HandleFunc("GET /api/v1/organizations/{org_id}/units/{unit_id}/property", handler.GetProperty)
	mux.HandleFunc("PUT /api/v1/organizations/{org_id}/units/{unit_id}/property", handler.SetProperty)
	mux.HandleFunc("POST /api/v1/organizations/{org_id}/units/{unit_id}/memberships", handler.CreateUnitMembership)
	mux.HandleFunc("GET /api/v1/organizations/{org_id}/units/{unit_id}/memberships", handler.ListUnitMemberships)
	mux.HandleFunc("PATCH /api/v1/organizations/{org_id}/units/{unit_id}/memberships/{id}", handler.UpdateUnitMembership)
	mux.HandleFunc("DELETE /api/v1/organizations/{org_id}/units/{unit_id}/memberships/{id}", handler.EndUnitMembership)

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	return &unitTestServer{
		server:       server,
		mockUnitRepo: mockUnitRepo,
		mockOrgRepo:  mockOrgRepo,
	}
}

// doUnitRequest sends an HTTP request to the unit test server.
func doUnitRequest(t *testing.T, ts *unitTestServer, method, path string, body any) *http.Response {
	t.Helper()
	proxy := &orgTestServer{server: ts.server, mockOrgRepo: ts.mockOrgRepo}
	return doRequest(t, proxy, method, path, body)
}

// seedUnit pre-populates the mock unit repo and returns the record.
func seedUnit(t *testing.T, repo *mockUnitRepo, orgID uuid.UUID) *org.Unit {
	t.Helper()
	u := &org.Unit{
		ID:           uuid.New(),
		OrgID:        orgID,
		Label:        "Unit 101",
		Status:       "active",
		VotingWeight: 1.0,
		Metadata:     map[string]any{},
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	repo.units[u.ID] = u
	return u
}

// seedProperty pre-populates the mock unit repo with a property for the given unit.
func seedProperty(t *testing.T, repo *mockUnitRepo, unitID uuid.UUID) *org.Property {
	t.Helper()
	sqft := 1200
	p := &org.Property{
		ID:         uuid.New(),
		UnitID:     unitID,
		SquareFeet: &sqft,
		Metadata:   map[string]any{},
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	repo.properties[unitID] = p
	return p
}

// seedUnitMembership pre-populates the mock unit repo with a unit membership.
func seedUnitMembership(t *testing.T, repo *mockUnitRepo, unitID uuid.UUID) *org.UnitMembership {
	t.Helper()
	um := &org.UnitMembership{
		ID:           uuid.New(),
		UnitID:       unitID,
		UserID:       uuid.New(),
		Relationship: "owner",
		IsVoter:      true,
		StartedAt:    time.Now(),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	repo.unitMemberships[um.ID] = um
	return um
}

// ─── CreateUnit tests ─────────────────────────────────────────────────────────

func TestCreateUnitHandler_Success(t *testing.T) {
	ts := setupUnitTestServer(t)
	orgID := uuid.New()

	body := map[string]any{
		"label": "Unit 42",
	}
	path := "/api/v1/organizations/" + orgID.String() + "/units"
	resp := doUnitRequest(t, ts, http.MethodPost, path, body)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var envelope struct {
		Data *org.Unit `json:"data"`
	}
	decodeBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, "Unit 42", envelope.Data.Label)
	assert.NotEqual(t, uuid.Nil, envelope.Data.ID)
	assert.Equal(t, orgID, envelope.Data.OrgID)
	assert.Equal(t, "active", envelope.Data.Status)
}

func TestCreateUnitHandler_InvalidLabel(t *testing.T) {
	ts := setupUnitTestServer(t)
	orgID := uuid.New()

	// Missing label field — should fail validation.
	body := map[string]any{}
	path := "/api/v1/organizations/" + orgID.String() + "/units"
	resp := doUnitRequest(t, ts, http.MethodPost, path, body)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestCreateUnitHandler_InvalidOrgUUID(t *testing.T) {
	ts := setupUnitTestServer(t)

	body := map[string]any{"label": "Unit A"}
	resp := doUnitRequest(t, ts, http.MethodPost, "/api/v1/organizations/bad-uuid/units", body)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// ─── ListUnits tests ──────────────────────────────────────────────────────────

func TestListUnitsHandler_Success(t *testing.T) {
	ts := setupUnitTestServer(t)
	orgID := uuid.New()

	seedUnit(t, ts.mockUnitRepo, orgID)
	seedUnit(t, ts.mockUnitRepo, orgID)

	path := "/api/v1/organizations/" + orgID.String() + "/units"
	resp := doUnitRequest(t, ts, http.MethodGet, path, nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data []org.Unit `json:"data"`
	}
	decodeBody(t, resp, &envelope)
	assert.Len(t, envelope.Data, 2)
}

func TestListUnitsHandler_EmptyForUnknownOrg(t *testing.T) {
	ts := setupUnitTestServer(t)

	path := "/api/v1/organizations/" + uuid.New().String() + "/units"
	resp := doUnitRequest(t, ts, http.MethodGet, path, nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data []org.Unit `json:"data"`
	}
	decodeBody(t, resp, &envelope)
	assert.Empty(t, envelope.Data)
}

// ─── GetUnit tests ────────────────────────────────────────────────────────────

func TestGetUnitHandler_Success(t *testing.T) {
	ts := setupUnitTestServer(t)
	orgID := uuid.New()
	u := seedUnit(t, ts.mockUnitRepo, orgID)

	path := "/api/v1/organizations/" + orgID.String() + "/units/" + u.ID.String()
	resp := doUnitRequest(t, ts, http.MethodGet, path, nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data *org.Unit `json:"data"`
	}
	decodeBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, u.ID, envelope.Data.ID)
}

func TestGetUnitHandler_NotFound(t *testing.T) {
	ts := setupUnitTestServer(t)
	orgID := uuid.New()

	path := "/api/v1/organizations/" + orgID.String() + "/units/" + uuid.New().String()
	resp := doUnitRequest(t, ts, http.MethodGet, path, nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestGetUnitHandler_InvalidUUID(t *testing.T) {
	ts := setupUnitTestServer(t)
	orgID := uuid.New()

	path := "/api/v1/organizations/" + orgID.String() + "/units/not-a-uuid"
	resp := doUnitRequest(t, ts, http.MethodGet, path, nil)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// ─── UpdateUnit tests ─────────────────────────────────────────────────────────

func TestUpdateUnitHandler_Success(t *testing.T) {
	ts := setupUnitTestServer(t)
	orgID := uuid.New()
	u := seedUnit(t, ts.mockUnitRepo, orgID)

	newLabel := "Updated Label"
	body := map[string]any{"label": newLabel}
	path := "/api/v1/organizations/" + orgID.String() + "/units/" + u.ID.String()
	resp := doUnitRequest(t, ts, http.MethodPatch, path, body)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data *org.Unit `json:"data"`
	}
	decodeBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, newLabel, envelope.Data.Label)
}

func TestUpdateUnitHandler_NotFound(t *testing.T) {
	ts := setupUnitTestServer(t)
	orgID := uuid.New()

	body := map[string]any{"label": "New Label"}
	path := "/api/v1/organizations/" + orgID.String() + "/units/" + uuid.New().String()
	resp := doUnitRequest(t, ts, http.MethodPatch, path, body)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// ─── DeleteUnit tests ─────────────────────────────────────────────────────────

func TestDeleteUnitHandler_Success(t *testing.T) {
	ts := setupUnitTestServer(t)
	orgID := uuid.New()
	u := seedUnit(t, ts.mockUnitRepo, orgID)

	path := "/api/v1/organizations/" + orgID.String() + "/units/" + u.ID.String()
	resp := doUnitRequest(t, ts, http.MethodDelete, path, nil)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	resp.Body.Close()

	assert.NotContains(t, ts.mockUnitRepo.units, u.ID)
}

// ─── GetProperty tests ────────────────────────────────────────────────────────

func TestGetPropertyHandler_Success(t *testing.T) {
	ts := setupUnitTestServer(t)
	orgID := uuid.New()
	u := seedUnit(t, ts.mockUnitRepo, orgID)
	prop := seedProperty(t, ts.mockUnitRepo, u.ID)

	path := "/api/v1/organizations/" + orgID.String() + "/units/" + u.ID.String() + "/property"
	resp := doUnitRequest(t, ts, http.MethodGet, path, nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data *org.Property `json:"data"`
	}
	decodeBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, prop.ID, envelope.Data.ID)
	assert.Equal(t, u.ID, envelope.Data.UnitID)
}

func TestGetPropertyHandler_NotFound(t *testing.T) {
	ts := setupUnitTestServer(t)
	orgID := uuid.New()
	u := seedUnit(t, ts.mockUnitRepo, orgID)

	path := "/api/v1/organizations/" + orgID.String() + "/units/" + u.ID.String() + "/property"
	resp := doUnitRequest(t, ts, http.MethodGet, path, nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// ─── SetProperty tests ────────────────────────────────────────────────────────

func TestSetPropertyHandler_Success(t *testing.T) {
	ts := setupUnitTestServer(t)
	orgID := uuid.New()
	u := seedUnit(t, ts.mockUnitRepo, orgID)

	sqft := 1500
	bedrooms := 3
	body := map[string]any{
		"square_feet": sqft,
		"bedrooms":    bedrooms,
	}
	path := "/api/v1/organizations/" + orgID.String() + "/units/" + u.ID.String() + "/property"
	resp := doUnitRequest(t, ts, http.MethodPut, path, body)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data *org.Property `json:"data"`
	}
	decodeBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, u.ID, envelope.Data.UnitID)
	require.NotNil(t, envelope.Data.SquareFeet)
	assert.Equal(t, sqft, *envelope.Data.SquareFeet)
	require.NotNil(t, envelope.Data.Bedrooms)
	assert.Equal(t, bedrooms, *envelope.Data.Bedrooms)
}

// ─── CreateUnitMembership tests ───────────────────────────────────────────────

func TestCreateUnitMembership_Success(t *testing.T) {
	ts := setupUnitTestServer(t)
	orgID := uuid.New()
	u := seedUnit(t, ts.mockUnitRepo, orgID)

	body := map[string]any{
		"user_id":      uuid.New(),
		"relationship": "owner",
		"is_voter":     true,
	}
	path := "/api/v1/organizations/" + orgID.String() + "/units/" + u.ID.String() + "/memberships"
	resp := doUnitRequest(t, ts, http.MethodPost, path, body)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var envelope struct {
		Data *org.UnitMembership `json:"data"`
	}
	decodeBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, u.ID, envelope.Data.UnitID)
	assert.Equal(t, "owner", envelope.Data.Relationship)
	assert.True(t, envelope.Data.IsVoter)
}

func TestCreateUnitMembership_InvalidRelationship(t *testing.T) {
	ts := setupUnitTestServer(t)
	orgID := uuid.New()
	u := seedUnit(t, ts.mockUnitRepo, orgID)

	body := map[string]any{
		"user_id":      uuid.New(),
		"relationship": "invalid_type",
	}
	path := "/api/v1/organizations/" + orgID.String() + "/units/" + u.ID.String() + "/memberships"
	resp := doUnitRequest(t, ts, http.MethodPost, path, body)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestCreateUnitMembership_MissingUserID(t *testing.T) {
	ts := setupUnitTestServer(t)
	orgID := uuid.New()
	u := seedUnit(t, ts.mockUnitRepo, orgID)

	body := map[string]any{
		"relationship": "owner",
	}
	path := "/api/v1/organizations/" + orgID.String() + "/units/" + u.ID.String() + "/memberships"
	resp := doUnitRequest(t, ts, http.MethodPost, path, body)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// ─── ListUnitMemberships tests ────────────────────────────────────────────────

func TestListUnitMemberships_Success(t *testing.T) {
	ts := setupUnitTestServer(t)
	orgID := uuid.New()
	u := seedUnit(t, ts.mockUnitRepo, orgID)

	seedUnitMembership(t, ts.mockUnitRepo, u.ID)
	seedUnitMembership(t, ts.mockUnitRepo, u.ID)

	path := "/api/v1/organizations/" + orgID.String() + "/units/" + u.ID.String() + "/memberships"
	resp := doUnitRequest(t, ts, http.MethodGet, path, nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data []org.UnitMembership `json:"data"`
	}
	decodeBody(t, resp, &envelope)
	assert.Len(t, envelope.Data, 2)
}

// ─── UpdateUnitMembership tests ───────────────────────────────────────────────

func TestUpdateUnitMembership_Success(t *testing.T) {
	ts := setupUnitTestServer(t)
	orgID := uuid.New()
	u := seedUnit(t, ts.mockUnitRepo, orgID)
	um := seedUnitMembership(t, ts.mockUnitRepo, u.ID)

	newRel := "tenant"
	body := map[string]any{"relationship": newRel}
	path := "/api/v1/organizations/" + orgID.String() + "/units/" + u.ID.String() + "/memberships/" + um.ID.String()
	resp := doUnitRequest(t, ts, http.MethodPatch, path, body)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data *org.UnitMembership `json:"data"`
	}
	decodeBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, newRel, envelope.Data.Relationship)
}

func TestUpdateUnitMembership_NotFound(t *testing.T) {
	ts := setupUnitTestServer(t)
	orgID := uuid.New()
	u := seedUnit(t, ts.mockUnitRepo, orgID)

	body := map[string]any{"relationship": "tenant"}
	path := "/api/v1/organizations/" + orgID.String() + "/units/" + u.ID.String() + "/memberships/" + uuid.New().String()
	resp := doUnitRequest(t, ts, http.MethodPatch, path, body)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// ─── EndUnitMembership tests ──────────────────────────────────────────────────

func TestEndUnitMembership_Success(t *testing.T) {
	ts := setupUnitTestServer(t)
	orgID := uuid.New()
	u := seedUnit(t, ts.mockUnitRepo, orgID)
	um := seedUnitMembership(t, ts.mockUnitRepo, u.ID)

	path := "/api/v1/organizations/" + orgID.String() + "/units/" + u.ID.String() + "/memberships/" + um.ID.String()
	resp := doUnitRequest(t, ts, http.MethodDelete, path, nil)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	resp.Body.Close()

	// Verify ended_at was set.
	stored := ts.mockUnitRepo.unitMemberships[um.ID]
	require.NotNil(t, stored)
	assert.NotNil(t, stored.EndedAt)
}

func TestEndUnitMembership_InvalidUUID(t *testing.T) {
	ts := setupUnitTestServer(t)
	orgID := uuid.New()
	u := seedUnit(t, ts.mockUnitRepo, orgID)

	path := "/api/v1/organizations/" + orgID.String() + "/units/" + u.ID.String() + "/memberships/not-a-uuid"
	resp := doUnitRequest(t, ts, http.MethodDelete, path, nil)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}
