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

// ─── Mock amenity repository ──────────────────────────────────────────────────

type mockAmenityRepo struct {
	amenities    map[uuid.UUID]*org.Amenity
	reservations map[uuid.UUID]*org.AmenityReservation
	createErr    error
	findErr      error
	updateErr    error
}

func newMockAmenityRepo() *mockAmenityRepo {
	return &mockAmenityRepo{
		amenities:    make(map[uuid.UUID]*org.Amenity),
		reservations: make(map[uuid.UUID]*org.AmenityReservation),
	}
}

func (m *mockAmenityRepo) CreateAmenity(_ context.Context, a *org.Amenity) (*org.Amenity, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	a.CreatedAt = time.Now()
	a.UpdatedAt = time.Now()
	if a.ReservationRules == nil {
		a.ReservationRules = map[string]any{}
	}
	if a.Hours == nil {
		a.Hours = map[string]any{}
	}
	if a.Metadata == nil {
		a.Metadata = map[string]any{}
	}
	copy := *a
	m.amenities[a.ID] = &copy
	return &copy, nil
}

func (m *mockAmenityRepo) FindAmenityByID(_ context.Context, id uuid.UUID) (*org.Amenity, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	a, ok := m.amenities[id]
	if !ok {
		return nil, nil
	}
	copy := *a
	return &copy, nil
}

func (m *mockAmenityRepo) ListAmenitiesByOrg(_ context.Context, orgID uuid.UUID, limit int, _ *uuid.UUID) ([]org.Amenity, bool, error) {
	if m.findErr != nil {
		return nil, false, m.findErr
	}
	result := []org.Amenity{}
	for _, a := range m.amenities {
		if a.OrgID == orgID && a.DeletedAt == nil {
			result = append(result, *a)
		}
	}
	hasMore := limit > 0 && len(result) > limit
	if hasMore {
		result = result[:limit]
	}
	return result, hasMore, nil
}

func (m *mockAmenityRepo) UpdateAmenity(_ context.Context, a *org.Amenity) (*org.Amenity, error) {
	if m.updateErr != nil {
		return nil, m.updateErr
	}
	a.UpdatedAt = time.Now()
	copy := *a
	m.amenities[a.ID] = &copy
	return &copy, nil
}

func (m *mockAmenityRepo) SoftDeleteAmenity(_ context.Context, id uuid.UUID) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	delete(m.amenities, id)
	return nil
}

func (m *mockAmenityRepo) CreateReservation(_ context.Context, r *org.AmenityReservation) (*org.AmenityReservation, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	r.CreatedAt = time.Now()
	r.UpdatedAt = time.Now()
	if r.Status == "" {
		r.Status = "pending"
	}
	copy := *r
	m.reservations[r.ID] = &copy
	return &copy, nil
}

func (m *mockAmenityRepo) ListReservationsByAmenity(_ context.Context, amenityID uuid.UUID) ([]org.AmenityReservation, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	result := []org.AmenityReservation{}
	for _, r := range m.reservations {
		if r.AmenityID == amenityID {
			result = append(result, *r)
		}
	}
	return result, nil
}

func (m *mockAmenityRepo) ListReservationsByUser(_ context.Context, orgID, userID uuid.UUID) ([]org.AmenityReservation, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	result := []org.AmenityReservation{}
	for _, r := range m.reservations {
		if r.OrgID == orgID && r.UserID == userID {
			result = append(result, *r)
		}
	}
	return result, nil
}

func (m *mockAmenityRepo) FindReservationByID(_ context.Context, id uuid.UUID) (*org.AmenityReservation, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	r, ok := m.reservations[id]
	if !ok {
		return nil, nil
	}
	copy := *r
	return &copy, nil
}

func (m *mockAmenityRepo) UpdateReservation(_ context.Context, r *org.AmenityReservation) (*org.AmenityReservation, error) {
	if m.updateErr != nil {
		return nil, m.updateErr
	}
	r.UpdatedAt = time.Now()
	copy := *r
	m.reservations[r.ID] = &copy
	return &copy, nil
}

// ─── Test server setup ────────────────────────────────────────────────────────

type amenityTestServer struct {
	server          *httptest.Server
	mockAmenityRepo *mockAmenityRepo
	mockOrgRepo     *mockOrgRepo
}

func setupAmenityTestServer(t *testing.T) *amenityTestServer {
	t.Helper()

	mockOrgRepo := newMockOrgRepo()
	mockMembershipRepo := newMockMembershipRepo()
	mockUnitRepo := newMockUnitRepo()
	mockUserRepo := newMockUserRepo()
	mockAmenityRepo := newMockAmenityRepo()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	service := org.NewOrgService(mockOrgRepo, mockMembershipRepo, mockUnitRepo, mockUserRepo, audit.NewNoopAuditor(), queue.NewInMemoryPublisher(), logger).
		WithAmenityRepo(mockAmenityRepo)

	handler := org.NewAmenityHandler(service, logger)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/organizations/{org_id}/amenities", handler.CreateAmenity)
	mux.HandleFunc("GET /api/v1/organizations/{org_id}/amenities", handler.ListAmenities)
	mux.HandleFunc("GET /api/v1/organizations/{org_id}/amenities/{amenity_id}", handler.GetAmenity)
	mux.HandleFunc("PATCH /api/v1/organizations/{org_id}/amenities/{amenity_id}", handler.UpdateAmenity)
	mux.HandleFunc("DELETE /api/v1/organizations/{org_id}/amenities/{amenity_id}", handler.DeleteAmenity)
	mux.HandleFunc("POST /api/v1/organizations/{org_id}/amenities/{amenity_id}/reservations", handler.CreateReservation)
	mux.HandleFunc("GET /api/v1/organizations/{org_id}/amenities/{amenity_id}/reservations", handler.ListAmenityReservations)
	mux.HandleFunc("GET /api/v1/organizations/{org_id}/reservations", handler.ListOrgReservations)
	mux.HandleFunc("GET /api/v1/organizations/{org_id}/reservations/{reservation_id}", handler.GetReservation)
	mux.HandleFunc("PATCH /api/v1/organizations/{org_id}/reservations/{reservation_id}", handler.UpdateReservation)

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	return &amenityTestServer{
		server:          server,
		mockAmenityRepo: mockAmenityRepo,
		mockOrgRepo:     mockOrgRepo,
	}
}

func doAmenityRequest(t *testing.T, ts *amenityTestServer, method, path string, body any) *http.Response {
	t.Helper()
	proxy := &orgTestServer{server: ts.server, mockOrgRepo: ts.mockOrgRepo}
	return doRequest(t, proxy, method, path, body)
}

func seedAmenity(t *testing.T, repo *mockAmenityRepo, orgID uuid.UUID) *org.Amenity {
	t.Helper()
	a := &org.Amenity{
		ID:               uuid.New(),
		OrgID:            orgID,
		Name:             "Pool",
		AmenityType:      "pool",
		IsReservable:     true,
		ReservationRules: map[string]any{},
		Hours:            map[string]any{},
		Status:           "open",
		Metadata:         map[string]any{},
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
	repo.amenities[a.ID] = a
	return a
}

func seedReservation(t *testing.T, repo *mockAmenityRepo, amenityID, orgID, userID, unitID uuid.UUID) *org.AmenityReservation {
	t.Helper()
	r := &org.AmenityReservation{
		ID:        uuid.New(),
		AmenityID: amenityID,
		OrgID:     orgID,
		UserID:    userID,
		UnitID:    unitID,
		Status:    "pending",
		StartsAt:  time.Now().Add(time.Hour),
		EndsAt:    time.Now().Add(2 * time.Hour),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	repo.reservations[r.ID] = r
	return r
}

// ─── CreateAmenity tests ──────────────────────────────────────────────────────

func TestCreateAmenityHandler_Success(t *testing.T) {
	ts := setupAmenityTestServer(t)
	orgID := uuid.New()

	body := map[string]any{
		"name":         "Clubhouse",
		"amenity_type": "clubhouse",
	}
	path := "/api/v1/organizations/" + orgID.String() + "/amenities"
	resp := doAmenityRequest(t, ts, http.MethodPost, path, body)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var envelope struct {
		Data *org.Amenity `json:"data"`
	}
	decodeBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, "Clubhouse", envelope.Data.Name)
	assert.Equal(t, "clubhouse", envelope.Data.AmenityType)
	assert.NotEqual(t, uuid.Nil, envelope.Data.ID)
	assert.Equal(t, orgID, envelope.Data.OrgID)
}

func TestCreateAmenityHandler_MissingName(t *testing.T) {
	ts := setupAmenityTestServer(t)
	orgID := uuid.New()

	body := map[string]any{"amenity_type": "pool"}
	path := "/api/v1/organizations/" + orgID.String() + "/amenities"
	resp := doAmenityRequest(t, ts, http.MethodPost, path, body)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestCreateAmenityHandler_InvalidOrgID(t *testing.T) {
	ts := setupAmenityTestServer(t)

	body := map[string]any{"name": "Pool", "amenity_type": "pool"}
	resp := doAmenityRequest(t, ts, http.MethodPost, "/api/v1/organizations/bad-uuid/amenities", body)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// ─── ListAmenities tests ──────────────────────────────────────────────────────

func TestListAmenitiesHandler_Success(t *testing.T) {
	ts := setupAmenityTestServer(t)
	orgID := uuid.New()

	seedAmenity(t, ts.mockAmenityRepo, orgID)
	seedAmenity(t, ts.mockAmenityRepo, orgID)

	path := "/api/v1/organizations/" + orgID.String() + "/amenities"
	resp := doAmenityRequest(t, ts, http.MethodGet, path, nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data []org.Amenity `json:"data"`
	}
	decodeBody(t, resp, &envelope)
	assert.Len(t, envelope.Data, 2)
}

func TestListAmenitiesHandler_EmptyForUnknownOrg(t *testing.T) {
	ts := setupAmenityTestServer(t)

	path := "/api/v1/organizations/" + uuid.New().String() + "/amenities"
	resp := doAmenityRequest(t, ts, http.MethodGet, path, nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data []org.Amenity `json:"data"`
	}
	decodeBody(t, resp, &envelope)
	assert.Empty(t, envelope.Data)
}

// ─── GetAmenity tests ─────────────────────────────────────────────────────────

func TestGetAmenityHandler_Success(t *testing.T) {
	ts := setupAmenityTestServer(t)
	orgID := uuid.New()
	a := seedAmenity(t, ts.mockAmenityRepo, orgID)

	path := "/api/v1/organizations/" + orgID.String() + "/amenities/" + a.ID.String()
	resp := doAmenityRequest(t, ts, http.MethodGet, path, nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data *org.Amenity `json:"data"`
	}
	decodeBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, a.ID, envelope.Data.ID)
}

func TestGetAmenityHandler_NotFound(t *testing.T) {
	ts := setupAmenityTestServer(t)
	orgID := uuid.New()

	path := "/api/v1/organizations/" + orgID.String() + "/amenities/" + uuid.New().String()
	resp := doAmenityRequest(t, ts, http.MethodGet, path, nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// ─── UpdateAmenity tests ──────────────────────────────────────────────────────

func TestUpdateAmenityHandler_Success(t *testing.T) {
	ts := setupAmenityTestServer(t)
	orgID := uuid.New()
	a := seedAmenity(t, ts.mockAmenityRepo, orgID)

	newName := "Updated Pool"
	body := map[string]any{"name": newName}
	path := "/api/v1/organizations/" + orgID.String() + "/amenities/" + a.ID.String()
	resp := doAmenityRequest(t, ts, http.MethodPatch, path, body)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data *org.Amenity `json:"data"`
	}
	decodeBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, newName, envelope.Data.Name)
}

func TestUpdateAmenityHandler_NotFound(t *testing.T) {
	ts := setupAmenityTestServer(t)
	orgID := uuid.New()

	body := map[string]any{"name": "New Name"}
	path := "/api/v1/organizations/" + orgID.String() + "/amenities/" + uuid.New().String()
	resp := doAmenityRequest(t, ts, http.MethodPatch, path, body)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// ─── DeleteAmenity tests ──────────────────────────────────────────────────────

func TestDeleteAmenityHandler_Success(t *testing.T) {
	ts := setupAmenityTestServer(t)
	orgID := uuid.New()
	a := seedAmenity(t, ts.mockAmenityRepo, orgID)

	path := "/api/v1/organizations/" + orgID.String() + "/amenities/" + a.ID.String()
	resp := doAmenityRequest(t, ts, http.MethodDelete, path, nil)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	resp.Body.Close()

	assert.NotContains(t, ts.mockAmenityRepo.amenities, a.ID)
}

// ─── CreateReservation tests ──────────────────────────────────────────────────

func TestCreateReservationHandler_Success(t *testing.T) {
	ts := setupAmenityTestServer(t)
	orgID := uuid.New()
	a := seedAmenity(t, ts.mockAmenityRepo, orgID)

	body := map[string]any{
		"user_id":   uuid.New(),
		"unit_id":   uuid.New(),
		"starts_at": time.Now().Add(time.Hour).Format(time.RFC3339),
		"ends_at":   time.Now().Add(2 * time.Hour).Format(time.RFC3339),
	}
	path := "/api/v1/organizations/" + orgID.String() + "/amenities/" + a.ID.String() + "/reservations"
	resp := doAmenityRequest(t, ts, http.MethodPost, path, body)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var envelope struct {
		Data *org.AmenityReservation `json:"data"`
	}
	decodeBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, a.ID, envelope.Data.AmenityID)
	assert.Equal(t, orgID, envelope.Data.OrgID)
	assert.NotEqual(t, uuid.Nil, envelope.Data.ID)
}

func TestCreateReservationHandler_MissingFields(t *testing.T) {
	ts := setupAmenityTestServer(t)
	orgID := uuid.New()
	a := seedAmenity(t, ts.mockAmenityRepo, orgID)

	// Missing starts_at and ends_at
	body := map[string]any{
		"user_id": uuid.New(),
		"unit_id": uuid.New(),
	}
	path := "/api/v1/organizations/" + orgID.String() + "/amenities/" + a.ID.String() + "/reservations"
	resp := doAmenityRequest(t, ts, http.MethodPost, path, body)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// ─── ListAmenityReservations tests ───────────────────────────────────────────

func TestListAmenityReservationsHandler_Success(t *testing.T) {
	ts := setupAmenityTestServer(t)
	orgID := uuid.New()
	a := seedAmenity(t, ts.mockAmenityRepo, orgID)

	seedReservation(t, ts.mockAmenityRepo, a.ID, orgID, uuid.New(), uuid.New())
	seedReservation(t, ts.mockAmenityRepo, a.ID, orgID, uuid.New(), uuid.New())

	path := "/api/v1/organizations/" + orgID.String() + "/amenities/" + a.ID.String() + "/reservations"
	resp := doAmenityRequest(t, ts, http.MethodGet, path, nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data []org.AmenityReservation `json:"data"`
	}
	decodeBody(t, resp, &envelope)
	assert.Len(t, envelope.Data, 2)
}

// ─── GetReservation tests ─────────────────────────────────────────────────────

func TestGetReservationHandler_Success(t *testing.T) {
	ts := setupAmenityTestServer(t)
	orgID := uuid.New()
	a := seedAmenity(t, ts.mockAmenityRepo, orgID)
	res := seedReservation(t, ts.mockAmenityRepo, a.ID, orgID, uuid.New(), uuid.New())

	path := "/api/v1/organizations/" + orgID.String() + "/reservations/" + res.ID.String()
	resp := doAmenityRequest(t, ts, http.MethodGet, path, nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data *org.AmenityReservation `json:"data"`
	}
	decodeBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, res.ID, envelope.Data.ID)
}

func TestGetReservationHandler_NotFound(t *testing.T) {
	ts := setupAmenityTestServer(t)
	orgID := uuid.New()

	path := "/api/v1/organizations/" + orgID.String() + "/reservations/" + uuid.New().String()
	resp := doAmenityRequest(t, ts, http.MethodGet, path, nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// ─── UpdateReservation tests ──────────────────────────────────────────────────

func TestUpdateReservationHandler_Success(t *testing.T) {
	ts := setupAmenityTestServer(t)
	orgID := uuid.New()
	a := seedAmenity(t, ts.mockAmenityRepo, orgID)
	res := seedReservation(t, ts.mockAmenityRepo, a.ID, orgID, uuid.New(), uuid.New())

	newStatus := "confirmed"
	body := map[string]any{"status": newStatus}
	path := "/api/v1/organizations/" + orgID.String() + "/reservations/" + res.ID.String()
	resp := doAmenityRequest(t, ts, http.MethodPatch, path, body)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data *org.AmenityReservation `json:"data"`
	}
	decodeBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, newStatus, envelope.Data.Status)
}

func TestUpdateReservationHandler_NotFound(t *testing.T) {
	ts := setupAmenityTestServer(t)
	orgID := uuid.New()

	body := map[string]any{"status": "confirmed"}
	path := "/api/v1/organizations/" + orgID.String() + "/reservations/" + uuid.New().String()
	resp := doAmenityRequest(t, ts, http.MethodPatch, path, body)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}
