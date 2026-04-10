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

// ─── Mock vendor repository ───────────────────────────────────────────────────

type mockVendorRepo struct {
	vendors     map[uuid.UUID]*org.Vendor
	assignments map[uuid.UUID]*org.VendorAssignment
	createErr   error
	findErr     error
	updateErr   error
}

func newMockVendorRepo() *mockVendorRepo {
	return &mockVendorRepo{
		vendors:     make(map[uuid.UUID]*org.Vendor),
		assignments: make(map[uuid.UUID]*org.VendorAssignment),
	}
}

func (m *mockVendorRepo) CreateVendor(_ context.Context, v *org.Vendor) (*org.Vendor, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	if v.ID == uuid.Nil {
		v.ID = uuid.New()
	}
	v.CreatedAt = time.Now()
	v.UpdatedAt = time.Now()
	if v.ServiceTypes == nil {
		v.ServiceTypes = []string{}
	}
	if v.Metadata == nil {
		v.Metadata = map[string]any{}
	}
	cp := *v
	m.vendors[v.ID] = &cp
	return &cp, nil
}

func (m *mockVendorRepo) FindVendorByID(_ context.Context, id uuid.UUID) (*org.Vendor, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	v, ok := m.vendors[id]
	if !ok {
		return nil, nil
	}
	cp := *v
	return &cp, nil
}

func (m *mockVendorRepo) ListVendors(_ context.Context, limit int, _ *uuid.UUID) ([]org.Vendor, bool, error) {
	if m.findErr != nil {
		return nil, false, m.findErr
	}
	result := []org.Vendor{}
	for _, v := range m.vendors {
		if v.DeletedAt == nil {
			result = append(result, *v)
		}
	}
	hasMore := limit > 0 && len(result) > limit
	if hasMore {
		result = result[:limit]
	}
	return result, hasMore, nil
}

func (m *mockVendorRepo) UpdateVendor(_ context.Context, v *org.Vendor) (*org.Vendor, error) {
	if m.updateErr != nil {
		return nil, m.updateErr
	}
	v.UpdatedAt = time.Now()
	cp := *v
	m.vendors[v.ID] = &cp
	return &cp, nil
}

func (m *mockVendorRepo) SoftDeleteVendor(_ context.Context, id uuid.UUID) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	delete(m.vendors, id)
	return nil
}

func (m *mockVendorRepo) CreateAssignment(_ context.Context, a *org.VendorAssignment) (*org.VendorAssignment, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	a.CreatedAt = time.Now()
	if a.StartedAt.IsZero() {
		a.StartedAt = time.Now()
	}
	cp := *a
	m.assignments[a.ID] = &cp
	return &cp, nil
}

func (m *mockVendorRepo) ListAssignmentsByOrg(_ context.Context, orgID uuid.UUID) ([]org.VendorAssignment, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	result := []org.VendorAssignment{}
	for _, a := range m.assignments {
		if a.OrgID == orgID && a.EndedAt == nil {
			result = append(result, *a)
		}
	}
	return result, nil
}

func (m *mockVendorRepo) DeleteAssignment(_ context.Context, id uuid.UUID) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	if a, ok := m.assignments[id]; ok {
		now := time.Now()
		a.EndedAt = &now
	}
	return nil
}

// ─── Test server setup ────────────────────────────────────────────────────────

type vendorTestServer struct {
	server         *httptest.Server
	mockVendorRepo *mockVendorRepo
	mockOrgRepo    *mockOrgRepo
}

func setupVendorTestServer(t *testing.T) *vendorTestServer {
	t.Helper()

	mockOrgRepo := newMockOrgRepo()
	mockMembershipRepo := newMockMembershipRepo()
	mockUnitRepo := newMockUnitRepo()
	mockUserRepo := newMockUserRepo()
	mockVendorRepo := newMockVendorRepo()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	service := org.NewOrgService(mockOrgRepo, mockMembershipRepo, mockUnitRepo, mockUserRepo, audit.NewNoopAuditor(), queue.NewInMemoryPublisher(), logger).
		WithVendorRepo(mockVendorRepo)

	handler := org.NewVendorHandler(service, logger)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/vendors", handler.CreateVendor)
	mux.HandleFunc("GET /api/v1/vendors", handler.ListVendors)
	mux.HandleFunc("GET /api/v1/vendors/{vendor_id}", handler.GetVendor)
	mux.HandleFunc("PATCH /api/v1/vendors/{vendor_id}", handler.UpdateVendor)
	mux.HandleFunc("DELETE /api/v1/vendors/{vendor_id}", handler.DeleteVendor)
	mux.HandleFunc("POST /api/v1/organizations/{org_id}/vendor-assignments", handler.CreateVendorAssignment)
	mux.HandleFunc("GET /api/v1/organizations/{org_id}/vendor-assignments", handler.ListVendorAssignments)
	mux.HandleFunc("DELETE /api/v1/organizations/{org_id}/vendor-assignments/{id}", handler.DeleteVendorAssignment)

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	return &vendorTestServer{
		server:         server,
		mockVendorRepo: mockVendorRepo,
		mockOrgRepo:    mockOrgRepo,
	}
}

func doVendorRequest(t *testing.T, ts *vendorTestServer, method, path string, body any) *http.Response {
	t.Helper()
	proxy := &orgTestServer{server: ts.server, mockOrgRepo: ts.mockOrgRepo}
	return doRequest(t, proxy, method, path, body)
}

func seedVendor(t *testing.T, repo *mockVendorRepo) *org.Vendor {
	t.Helper()
	v := &org.Vendor{
		ID:           uuid.New(),
		Name:         "Acme Landscaping",
		ServiceTypes: []string{"landscaping"},
		Metadata:     map[string]any{},
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	repo.vendors[v.ID] = v
	return v
}

func seedVendorAssignment(t *testing.T, repo *mockVendorRepo, vendorID, orgID uuid.UUID) *org.VendorAssignment {
	t.Helper()
	a := &org.VendorAssignment{
		ID:           uuid.New(),
		VendorID:     vendorID,
		OrgID:        orgID,
		ServiceScope: "general",
		StartedAt:    time.Now(),
		CreatedAt:    time.Now(),
	}
	repo.assignments[a.ID] = a
	return a
}

// ─── CreateVendor tests ───────────────────────────────────────────────────────

func TestCreateVendorHandler_Success(t *testing.T) {
	ts := setupVendorTestServer(t)

	body := map[string]any{
		"name": "Green Thumb Landscaping",
	}
	resp := doVendorRequest(t, ts, http.MethodPost, "/api/v1/vendors", body)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var envelope struct {
		Data *org.Vendor `json:"data"`
	}
	decodeBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, "Green Thumb Landscaping", envelope.Data.Name)
	assert.NotEqual(t, uuid.Nil, envelope.Data.ID)
}

func TestCreateVendorHandler_MissingName(t *testing.T) {
	ts := setupVendorTestServer(t)

	body := map[string]any{}
	resp := doVendorRequest(t, ts, http.MethodPost, "/api/v1/vendors", body)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// ─── ListVendors tests ────────────────────────────────────────────────────────

func TestListVendorsHandler_Success(t *testing.T) {
	ts := setupVendorTestServer(t)

	seedVendor(t, ts.mockVendorRepo)
	seedVendor(t, ts.mockVendorRepo)

	resp := doVendorRequest(t, ts, http.MethodGet, "/api/v1/vendors", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data []org.Vendor `json:"data"`
	}
	decodeBody(t, resp, &envelope)
	assert.Len(t, envelope.Data, 2)
}

func TestListVendorsHandler_Empty(t *testing.T) {
	ts := setupVendorTestServer(t)

	resp := doVendorRequest(t, ts, http.MethodGet, "/api/v1/vendors", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data []org.Vendor `json:"data"`
	}
	decodeBody(t, resp, &envelope)
	assert.Empty(t, envelope.Data)
}

// ─── GetVendor tests ──────────────────────────────────────────────────────────

func TestGetVendorHandler_Success(t *testing.T) {
	ts := setupVendorTestServer(t)
	v := seedVendor(t, ts.mockVendorRepo)

	resp := doVendorRequest(t, ts, http.MethodGet, "/api/v1/vendors/"+v.ID.String(), nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data *org.Vendor `json:"data"`
	}
	decodeBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, v.ID, envelope.Data.ID)
}

func TestGetVendorHandler_NotFound(t *testing.T) {
	ts := setupVendorTestServer(t)

	resp := doVendorRequest(t, ts, http.MethodGet, "/api/v1/vendors/"+uuid.New().String(), nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestGetVendorHandler_InvalidUUID(t *testing.T) {
	ts := setupVendorTestServer(t)

	resp := doVendorRequest(t, ts, http.MethodGet, "/api/v1/vendors/bad-uuid", nil)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// ─── UpdateVendor tests ───────────────────────────────────────────────────────

func TestUpdateVendorHandler_Success(t *testing.T) {
	ts := setupVendorTestServer(t)
	v := seedVendor(t, ts.mockVendorRepo)

	newName := "Updated Landscaping Co"
	body := map[string]any{"name": newName}
	resp := doVendorRequest(t, ts, http.MethodPatch, "/api/v1/vendors/"+v.ID.String(), body)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data *org.Vendor `json:"data"`
	}
	decodeBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, newName, envelope.Data.Name)
}

func TestUpdateVendorHandler_NotFound(t *testing.T) {
	ts := setupVendorTestServer(t)

	body := map[string]any{"name": "New Name"}
	resp := doVendorRequest(t, ts, http.MethodPatch, "/api/v1/vendors/"+uuid.New().String(), body)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// ─── DeleteVendor tests ───────────────────────────────────────────────────────

func TestDeleteVendorHandler_Success(t *testing.T) {
	ts := setupVendorTestServer(t)
	v := seedVendor(t, ts.mockVendorRepo)

	resp := doVendorRequest(t, ts, http.MethodDelete, "/api/v1/vendors/"+v.ID.String(), nil)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	resp.Body.Close()

	assert.NotContains(t, ts.mockVendorRepo.vendors, v.ID)
}

// ─── CreateVendorAssignment tests ─────────────────────────────────────────────

func TestCreateVendorAssignmentHandler_Success(t *testing.T) {
	ts := setupVendorTestServer(t)
	orgID := uuid.New()
	v := seedVendor(t, ts.mockVendorRepo)

	body := map[string]any{
		"vendor_id":     v.ID,
		"service_scope": "lawn care",
	}
	path := "/api/v1/organizations/" + orgID.String() + "/vendor-assignments"
	resp := doVendorRequest(t, ts, http.MethodPost, path, body)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var envelope struct {
		Data *org.VendorAssignment `json:"data"`
	}
	decodeBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, v.ID, envelope.Data.VendorID)
	assert.Equal(t, orgID, envelope.Data.OrgID)
	assert.Equal(t, "lawn care", envelope.Data.ServiceScope)
}

func TestCreateVendorAssignmentHandler_MissingVendorID(t *testing.T) {
	ts := setupVendorTestServer(t)
	orgID := uuid.New()

	body := map[string]any{"service_scope": "lawn care"}
	path := "/api/v1/organizations/" + orgID.String() + "/vendor-assignments"
	resp := doVendorRequest(t, ts, http.MethodPost, path, body)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// ─── ListVendorAssignments tests ──────────────────────────────────────────────

func TestListVendorAssignmentsHandler_Success(t *testing.T) {
	ts := setupVendorTestServer(t)
	orgID := uuid.New()
	v1 := seedVendor(t, ts.mockVendorRepo)
	v2 := seedVendor(t, ts.mockVendorRepo)

	seedVendorAssignment(t, ts.mockVendorRepo, v1.ID, orgID)
	seedVendorAssignment(t, ts.mockVendorRepo, v2.ID, orgID)

	path := "/api/v1/organizations/" + orgID.String() + "/vendor-assignments"
	resp := doVendorRequest(t, ts, http.MethodGet, path, nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data []org.VendorAssignment `json:"data"`
	}
	decodeBody(t, resp, &envelope)
	assert.Len(t, envelope.Data, 2)
}

// ─── DeleteVendorAssignment tests ─────────────────────────────────────────────

func TestDeleteVendorAssignmentHandler_Success(t *testing.T) {
	ts := setupVendorTestServer(t)
	orgID := uuid.New()
	v := seedVendor(t, ts.mockVendorRepo)
	a := seedVendorAssignment(t, ts.mockVendorRepo, v.ID, orgID)

	path := "/api/v1/organizations/" + orgID.String() + "/vendor-assignments/" + a.ID.String()
	resp := doVendorRequest(t, ts, http.MethodDelete, path, nil)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	resp.Body.Close()

	stored := ts.mockVendorRepo.assignments[a.ID]
	require.NotNil(t, stored)
	assert.NotNil(t, stored.EndedAt)
}
