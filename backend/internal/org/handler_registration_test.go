package org_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/audit"
	"github.com/quorant/quorant/internal/org"
	"github.com/quorant/quorant/internal/platform/queue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Mock registration repository ────────────────────────────────────────────

type mockRegistrationRepo struct {
	types         map[uuid.UUID]*org.RegistrationType
	registrations map[uuid.UUID]*org.Registration
	createErr     error
	findErr       error
	updateErr     error
}

func newMockRegistrationRepo() *mockRegistrationRepo {
	return &mockRegistrationRepo{
		types:         make(map[uuid.UUID]*org.RegistrationType),
		registrations: make(map[uuid.UUID]*org.Registration),
	}
}

func (m *mockRegistrationRepo) CreateRegistrationType(_ context.Context, rt *org.RegistrationType) (*org.RegistrationType, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	if rt.ID == uuid.Nil {
		rt.ID = uuid.New()
	}
	rt.CreatedAt = time.Now()
	rt.UpdatedAt = time.Now()
	if rt.Schema == nil {
		rt.Schema = map[string]any{}
	}
	copy := *rt
	m.types[rt.ID] = &copy
	return &copy, nil
}

func (m *mockRegistrationRepo) FindRegistrationTypeByID(_ context.Context, id uuid.UUID) (*org.RegistrationType, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	rt, ok := m.types[id]
	if !ok {
		return nil, nil
	}
	copy := *rt
	return &copy, nil
}

func (m *mockRegistrationRepo) ListRegistrationTypesByOrg(_ context.Context, orgID uuid.UUID) ([]org.RegistrationType, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	result := []org.RegistrationType{}
	for _, rt := range m.types {
		if rt.OrgID == orgID {
			result = append(result, *rt)
		}
	}
	return result, nil
}

func (m *mockRegistrationRepo) UpdateRegistrationType(_ context.Context, rt *org.RegistrationType) (*org.RegistrationType, error) {
	if m.updateErr != nil {
		return nil, m.updateErr
	}
	rt.UpdatedAt = time.Now()
	copy := *rt
	m.types[rt.ID] = &copy
	return &copy, nil
}

func (m *mockRegistrationRepo) CreateRegistration(_ context.Context, reg *org.Registration) (*org.Registration, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	if reg.ID == uuid.Nil {
		reg.ID = uuid.New()
	}
	reg.CreatedAt = time.Now()
	reg.UpdatedAt = time.Now()
	if reg.Data == nil {
		reg.Data = map[string]any{}
	}
	if reg.Status == "" {
		reg.Status = "active"
	}
	copy := *reg
	m.registrations[reg.ID] = &copy
	return &copy, nil
}

func (m *mockRegistrationRepo) FindRegistrationByID(_ context.Context, id uuid.UUID) (*org.Registration, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	reg, ok := m.registrations[id]
	if !ok {
		return nil, nil
	}
	if reg.DeletedAt != nil {
		return nil, nil
	}
	copy := *reg
	return &copy, nil
}

func (m *mockRegistrationRepo) ListRegistrationsByUnit(_ context.Context, unitID uuid.UUID) ([]org.Registration, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	result := []org.Registration{}
	for _, reg := range m.registrations {
		if reg.UnitID == unitID && reg.DeletedAt == nil {
			result = append(result, *reg)
		}
	}
	return result, nil
}

func (m *mockRegistrationRepo) UpdateRegistration(_ context.Context, reg *org.Registration) (*org.Registration, error) {
	if m.updateErr != nil {
		return nil, m.updateErr
	}
	reg.UpdatedAt = time.Now()
	copy := *reg
	m.registrations[reg.ID] = &copy
	return &copy, nil
}

// ─── Test server setup ────────────────────────────────────────────────────────

type registrationTestServer struct {
	server               *httptest.Server
	mockRegistrationRepo *mockRegistrationRepo
	mockUnitRepo         *mockUnitRepo
	mockOrgRepo          *mockOrgRepo
}

func setupRegistrationTestServer(t *testing.T) *registrationTestServer {
	t.Helper()

	mockOrgRepo := newMockOrgRepo()
	mockMembershipRepo := newMockMembershipRepo()
	mockUnitRepo := newMockUnitRepo()
	mockUserRepo := newMockUserRepo()
	mockRegRepo := newMockRegistrationRepo()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	service := org.NewOrgService(mockOrgRepo, mockMembershipRepo, mockUnitRepo, mockUserRepo, audit.NewNoopAuditor(), queue.NewInMemoryPublisher(), logger).
		WithRegistrationRepo(mockRegRepo)

	handler := org.NewRegistrationHandler(service, logger)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/organizations/{org_id}/registration-types", handler.CreateRegistrationType)
	mux.HandleFunc("GET /api/v1/organizations/{org_id}/registration-types", handler.ListRegistrationTypes)
	mux.HandleFunc("PATCH /api/v1/organizations/{org_id}/registration-types/{id}", handler.UpdateRegistrationType)
	mux.HandleFunc("POST /api/v1/organizations/{org_id}/units/{unit_id}/registrations", handler.CreateRegistration)
	mux.HandleFunc("GET /api/v1/organizations/{org_id}/units/{unit_id}/registrations", handler.ListRegistrations)
	mux.HandleFunc("PATCH /api/v1/organizations/{org_id}/registrations/{id}", handler.UpdateRegistration)
	mux.HandleFunc("POST /api/v1/organizations/{org_id}/registrations/{id}/approve", handler.ApproveRegistration)
	mux.HandleFunc("POST /api/v1/organizations/{org_id}/registrations/{id}/revoke", handler.RevokeRegistration)

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	return &registrationTestServer{
		server:               server,
		mockRegistrationRepo: mockRegRepo,
		mockUnitRepo:         mockUnitRepo,
		mockOrgRepo:          mockOrgRepo,
	}
}

func doRegistrationRequest(t *testing.T, ts *registrationTestServer, method, path string, body any) *http.Response {
	t.Helper()
	proxy := &orgTestServer{server: ts.server, mockOrgRepo: ts.mockOrgRepo}
	return doRequest(t, proxy, method, path, body)
}

func seedRegistrationType(t *testing.T, repo *mockRegistrationRepo, orgID uuid.UUID) *org.RegistrationType {
	t.Helper()
	rt := &org.RegistrationType{
		ID:               uuid.New(),
		OrgID:            orgID,
		Name:             "Pet Registration",
		Slug:             "pet",
		Schema:           map[string]any{},
		RequiresApproval: false,
		IsActive:         true,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
	repo.types[rt.ID] = rt
	return rt
}

func seedRegistration(t *testing.T, repo *mockRegistrationRepo, orgID, unitID, typeID uuid.UUID) *org.Registration {
	t.Helper()
	reg := &org.Registration{
		ID:                 uuid.New(),
		OrgID:              orgID,
		UnitID:             unitID,
		UserID:             uuid.New(),
		RegistrationTypeID: typeID,
		Data:               map[string]any{"breed": "labrador"},
		Status:             "active",
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	repo.registrations[reg.ID] = reg
	return reg
}

// ─── CreateRegistrationType tests ────────────────────────────────────────────

func TestCreateRegistrationTypeHandler_Success(t *testing.T) {
	ts := setupRegistrationTestServer(t)
	orgID := uuid.New()

	body := map[string]any{
		"name": "Vehicle",
		"slug": "vehicle",
	}
	path := "/api/v1/organizations/" + orgID.String() + "/registration-types"
	resp := doRegistrationRequest(t, ts, http.MethodPost, path, body)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var envelope struct {
		Data *org.RegistrationType `json:"data"`
	}
	decodeBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, "Vehicle", envelope.Data.Name)
	assert.Equal(t, "vehicle", envelope.Data.Slug)
	assert.NotEqual(t, uuid.Nil, envelope.Data.ID)
	assert.Equal(t, orgID, envelope.Data.OrgID)
	assert.True(t, envelope.Data.IsActive)
}

func TestCreateRegistrationTypeHandler_MissingName(t *testing.T) {
	ts := setupRegistrationTestServer(t)
	orgID := uuid.New()

	body := map[string]any{"slug": "vehicle"}
	path := "/api/v1/organizations/" + orgID.String() + "/registration-types"
	resp := doRegistrationRequest(t, ts, http.MethodPost, path, body)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestCreateRegistrationTypeHandler_MissingSlug(t *testing.T) {
	ts := setupRegistrationTestServer(t)
	orgID := uuid.New()

	body := map[string]any{"name": "Vehicle"}
	path := "/api/v1/organizations/" + orgID.String() + "/registration-types"
	resp := doRegistrationRequest(t, ts, http.MethodPost, path, body)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// ─── ListRegistrationTypes tests ─────────────────────────────────────────────

func TestListRegistrationTypesHandler_Success(t *testing.T) {
	ts := setupRegistrationTestServer(t)
	orgID := uuid.New()

	seedRegistrationType(t, ts.mockRegistrationRepo, orgID)
	seedRegistrationType(t, ts.mockRegistrationRepo, orgID)

	path := "/api/v1/organizations/" + orgID.String() + "/registration-types"
	resp := doRegistrationRequest(t, ts, http.MethodGet, path, nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data []org.RegistrationType `json:"data"`
	}
	decodeBody(t, resp, &envelope)
	assert.Len(t, envelope.Data, 2)
}

// ─── UpdateRegistrationType tests ────────────────────────────────────────────

func TestUpdateRegistrationTypeHandler_Success(t *testing.T) {
	ts := setupRegistrationTestServer(t)
	orgID := uuid.New()
	rt := seedRegistrationType(t, ts.mockRegistrationRepo, orgID)

	newName := "Updated Pet Registration"
	body := map[string]any{"name": newName}
	path := "/api/v1/organizations/" + orgID.String() + "/registration-types/" + rt.ID.String()
	resp := doRegistrationRequest(t, ts, http.MethodPatch, path, body)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data *org.RegistrationType `json:"data"`
	}
	decodeBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, newName, envelope.Data.Name)
}

func TestUpdateRegistrationTypeHandler_NotFound(t *testing.T) {
	ts := setupRegistrationTestServer(t)
	orgID := uuid.New()

	body := map[string]any{"name": "New Name"}
	path := "/api/v1/organizations/" + orgID.String() + "/registration-types/" + uuid.New().String()
	resp := doRegistrationRequest(t, ts, http.MethodPatch, path, body)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// ─── CreateRegistration tests ─────────────────────────────────────────────────

func TestCreateRegistrationHandler_Success(t *testing.T) {
	ts := setupRegistrationTestServer(t)
	orgID := uuid.New()
	unitID := uuid.New()
	typeID := uuid.New()

	body := map[string]any{
		"user_id":              uuid.New(),
		"registration_type_id": typeID,
		"data":                 map[string]any{"breed": "labrador"},
	}
	path := "/api/v1/organizations/" + orgID.String() + "/units/" + unitID.String() + "/registrations"
	resp := doRegistrationRequest(t, ts, http.MethodPost, path, body)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var envelope struct {
		Data *org.Registration `json:"data"`
	}
	decodeBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, orgID, envelope.Data.OrgID)
	assert.Equal(t, unitID, envelope.Data.UnitID)
	assert.Equal(t, typeID, envelope.Data.RegistrationTypeID)
	assert.NotEqual(t, uuid.Nil, envelope.Data.ID)
}

func TestCreateRegistrationHandler_MissingUserID(t *testing.T) {
	ts := setupRegistrationTestServer(t)
	orgID := uuid.New()
	unitID := uuid.New()

	body := map[string]any{
		"registration_type_id": uuid.New(),
	}
	path := "/api/v1/organizations/" + orgID.String() + "/units/" + unitID.String() + "/registrations"
	resp := doRegistrationRequest(t, ts, http.MethodPost, path, body)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// ─── ListRegistrations tests ──────────────────────────────────────────────────

func TestListRegistrationsHandler_Success(t *testing.T) {
	ts := setupRegistrationTestServer(t)
	orgID := uuid.New()
	unitID := uuid.New()
	typeID := uuid.New()

	seedRegistration(t, ts.mockRegistrationRepo, orgID, unitID, typeID)
	seedRegistration(t, ts.mockRegistrationRepo, orgID, unitID, typeID)

	path := "/api/v1/organizations/" + orgID.String() + "/units/" + unitID.String() + "/registrations"
	resp := doRegistrationRequest(t, ts, http.MethodGet, path, nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data []org.Registration `json:"data"`
	}
	decodeBody(t, resp, &envelope)
	assert.Len(t, envelope.Data, 2)
}

// ─── UpdateRegistration tests ─────────────────────────────────────────────────

func TestUpdateRegistrationHandler_Success(t *testing.T) {
	ts := setupRegistrationTestServer(t)
	orgID := uuid.New()
	unitID := uuid.New()
	typeID := uuid.New()
	reg := seedRegistration(t, ts.mockRegistrationRepo, orgID, unitID, typeID)

	newStatus := "pending"
	body := map[string]any{"status": newStatus}
	path := "/api/v1/organizations/" + orgID.String() + "/registrations/" + reg.ID.String()
	resp := doRegistrationRequest(t, ts, http.MethodPatch, path, body)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data *org.Registration `json:"data"`
	}
	decodeBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, newStatus, envelope.Data.Status)
}

func TestUpdateRegistrationHandler_NotFound(t *testing.T) {
	ts := setupRegistrationTestServer(t)
	orgID := uuid.New()

	body := map[string]any{"status": "pending"}
	path := "/api/v1/organizations/" + orgID.String() + "/registrations/" + uuid.New().String()
	resp := doRegistrationRequest(t, ts, http.MethodPatch, path, body)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// ─── ApproveRegistration tests ────────────────────────────────────────────────

func TestApproveRegistrationHandler_Success(t *testing.T) {
	ts := setupRegistrationTestServer(t)
	orgID := uuid.New()
	unitID := uuid.New()
	typeID := uuid.New()
	reg := seedRegistration(t, ts.mockRegistrationRepo, orgID, unitID, typeID)

	path := "/api/v1/organizations/" + orgID.String() + "/registrations/" + reg.ID.String() + "/approve"
	resp := doRegistrationRequest(t, ts, http.MethodPost, path, nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data *org.Registration `json:"data"`
	}
	decodeBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, "active", envelope.Data.Status)
}

func TestApproveRegistrationHandler_NotFound(t *testing.T) {
	ts := setupRegistrationTestServer(t)
	orgID := uuid.New()

	path := "/api/v1/organizations/" + orgID.String() + "/registrations/" + uuid.New().String() + "/approve"
	resp := doRegistrationRequest(t, ts, http.MethodPost, path, nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// ─── RevokeRegistration tests ─────────────────────────────────────────────────

func TestRevokeRegistrationHandler_Success(t *testing.T) {
	ts := setupRegistrationTestServer(t)
	orgID := uuid.New()
	unitID := uuid.New()
	typeID := uuid.New()
	reg := seedRegistration(t, ts.mockRegistrationRepo, orgID, unitID, typeID)

	path := "/api/v1/organizations/" + orgID.String() + "/registrations/" + reg.ID.String() + "/revoke"
	resp := doRegistrationRequest(t, ts, http.MethodPost, path, nil)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	resp.Body.Close()

	stored := ts.mockRegistrationRepo.registrations[reg.ID]
	require.NotNil(t, stored)
	assert.Equal(t, "revoked", stored.Status)
}

func TestRevokeRegistrationHandler_NotFound(t *testing.T) {
	ts := setupRegistrationTestServer(t)
	orgID := uuid.New()

	path := "/api/v1/organizations/" + orgID.String() + "/registrations/" + uuid.New().String() + "/revoke"
	resp := doRegistrationRequest(t, ts, http.MethodPost, path, nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}
