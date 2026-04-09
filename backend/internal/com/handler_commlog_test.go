package com_test

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/com"
	"github.com/quorant/quorant/internal/platform/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Test server setup ─────────────────────────────────────────────────────────

type commLogTestServer struct {
	server          *httptest.Server
	mockCommLogRepo *mockCommLogRepo
	userID          uuid.UUID
}

func setupCommLogTestServer(t *testing.T) *commLogTestServer {
	t.Helper()

	mockCLRepo := newMockCommLogRepo()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	svc := newTestService(
		newMockAnnouncementRepo(),
		newMockThreadRepo(),
		newMockNotificationRepo(),
		newMockCalendarRepo(),
		newMockTemplateRepo(),
		newMockDirectoryRepo(),
		mockCLRepo,
	)
	handler := com.NewCommLogHandler(svc, logger)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/organizations/{org_id}/communications", handler.Log)
	mux.HandleFunc("GET /api/v1/organizations/{org_id}/communications", handler.List)
	mux.HandleFunc("GET /api/v1/organizations/{org_id}/communications/{comm_id}", handler.Get)
	mux.HandleFunc("PATCH /api/v1/organizations/{org_id}/communications/{comm_id}", handler.Update)
	mux.HandleFunc("GET /api/v1/organizations/{org_id}/units/{unit_id}/communications", handler.ListByUnit)

	testUserID := uuid.New()
	handlerWithUserID := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := middleware.WithUserID(r.Context(), testUserID)
		mux.ServeHTTP(w, r.WithContext(ctx))
	})
	server := httptest.NewServer(handlerWithUserID)
	t.Cleanup(server.Close)

	return &commLogTestServer{
		server:          server,
		mockCommLogRepo: mockCLRepo,
		userID:          testUserID,
	}
}

// doCommLogRequest sends an HTTP request to the comm log test server.
func doCommLogRequest(t *testing.T, ts *commLogTestServer, method, path string, body any) *http.Response {
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

// decodeCommLogBody JSON-decodes the response body into dst.
func decodeCommLogBody(t *testing.T, resp *http.Response, dst any) {
	t.Helper()
	defer resp.Body.Close()
	require.NoError(t, json.NewDecoder(resp.Body).Decode(dst))
}

// seedCommLog pre-populates the mock comm log repo with a communication entry.
func seedCommLog(t *testing.T, repo *mockCommLogRepo, orgID uuid.UUID) *com.CommunicationLog {
	t.Helper()
	now := time.Now()
	contactName := "John Doe"
	entry := &com.CommunicationLog{
		ID:          uuid.New(),
		OrgID:       orgID,
		Direction:   "outbound",
		Channel:     "email",
		ContactName: &contactName,
		Status:      "sent",
		Source:      "manual",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	repo.items[entry.ID] = entry
	return entry
}

// ─── Log tests ─────────────────────────────────────────────────────────────────

func TestLogCommunicationHandler_Success(t *testing.T) {
	// Given: a valid communication log request
	ts := setupCommLogTestServer(t)
	orgID := uuid.New()

	contactName := "Jane Smith"
	body := map[string]any{
		"direction":    "outbound",
		"channel":      "email",
		"contact_name": contactName,
	}
	path := "/api/v1/organizations/" + orgID.String() + "/communications"

	// When: POST /communications is called
	resp := doCommLogRequest(t, ts, http.MethodPost, path, body)

	// Then: 201 Created with the communication log entry
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var envelope struct {
		Data *com.CommunicationLog `json:"data"`
	}
	decodeCommLogBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.NotEqual(t, uuid.Nil, envelope.Data.ID)
	assert.Equal(t, orgID, envelope.Data.OrgID)
	assert.Equal(t, "outbound", envelope.Data.Direction)
}

func TestLogCommunication_MissingContactIdentifier(t *testing.T) {
	// Given: a request body with neither contact_name nor contact_user_id
	ts := setupCommLogTestServer(t)
	orgID := uuid.New()

	body := map[string]any{
		"direction": "outbound",
		"channel":   "email",
	}
	path := "/api/v1/organizations/" + orgID.String() + "/communications"

	// When: POST /communications without a contact identifier
	resp := doCommLogRequest(t, ts, http.MethodPost, path, body)

	// Then: 400 Bad Request
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()
}

func TestLogCommunication_InvalidOrgID(t *testing.T) {
	// Given: an invalid org UUID in the path
	ts := setupCommLogTestServer(t)

	body := map[string]any{
		"direction":    "outbound",
		"channel":      "email",
		"contact_name": "Test",
	}
	path := "/api/v1/organizations/bad-uuid/communications"

	// When: POST with bad org_id
	resp := doCommLogRequest(t, ts, http.MethodPost, path, body)

	// Then: 400 Bad Request
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()
}

// ─── List tests ────────────────────────────────────────────────────────────────

func TestListCommunications_Success(t *testing.T) {
	// Given: two communication entries seeded for an org
	ts := setupCommLogTestServer(t)
	orgID := uuid.New()
	seedCommLog(t, ts.mockCommLogRepo, orgID)
	seedCommLog(t, ts.mockCommLogRepo, orgID)

	path := "/api/v1/organizations/" + orgID.String() + "/communications"

	// When: GET /communications is called
	resp := doCommLogRequest(t, ts, http.MethodGet, path, nil)

	// Then: 200 OK with two entries
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data []com.CommunicationLog `json:"data"`
	}
	decodeCommLogBody(t, resp, &envelope)
	assert.Len(t, envelope.Data, 2)
}

func TestListCommunications_EmptyResult(t *testing.T) {
	// Given: no communication entries exist
	ts := setupCommLogTestServer(t)
	orgID := uuid.New()

	path := "/api/v1/organizations/" + orgID.String() + "/communications"

	// When: GET /communications is called
	resp := doCommLogRequest(t, ts, http.MethodGet, path, nil)

	// Then: 200 OK with empty list
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data []com.CommunicationLog `json:"data"`
	}
	decodeCommLogBody(t, resp, &envelope)
	assert.Empty(t, envelope.Data)
}

// ─── Get tests ─────────────────────────────────────────────────────────────────

func TestGetCommunication_Success(t *testing.T) {
	// Given: an existing communication entry
	ts := setupCommLogTestServer(t)
	orgID := uuid.New()
	entry := seedCommLog(t, ts.mockCommLogRepo, orgID)

	path := "/api/v1/organizations/" + orgID.String() + "/communications/" + entry.ID.String()

	// When: GET /communications/{comm_id} is called
	resp := doCommLogRequest(t, ts, http.MethodGet, path, nil)

	// Then: 200 OK with the entry
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data *com.CommunicationLog `json:"data"`
	}
	decodeCommLogBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, entry.ID, envelope.Data.ID)
}

func TestGetCommunication_NotFound(t *testing.T) {
	// Given: no matching communication entry
	ts := setupCommLogTestServer(t)
	orgID := uuid.New()

	path := "/api/v1/organizations/" + orgID.String() + "/communications/" + uuid.New().String()

	// When: GET /communications/{unknown_id} is called
	resp := doCommLogRequest(t, ts, http.MethodGet, path, nil)

	// Then: 404 Not Found
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()
}

func TestGetCommunication_InvalidUUID(t *testing.T) {
	// Given: an invalid comm UUID
	ts := setupCommLogTestServer(t)
	orgID := uuid.New()

	path := "/api/v1/organizations/" + orgID.String() + "/communications/bad-uuid"

	// When: GET with bad comm_id
	resp := doCommLogRequest(t, ts, http.MethodGet, path, nil)

	// Then: 400 Bad Request
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()
}

// ─── Update tests ──────────────────────────────────────────────────────────────

func TestUpdateCommunication_Success(t *testing.T) {
	// Given: an existing communication entry
	ts := setupCommLogTestServer(t)
	orgID := uuid.New()
	entry := seedCommLog(t, ts.mockCommLogRepo, orgID)

	body := map[string]any{
		"direction": "outbound",
		"channel":   "phone",
		"status":    "delivered",
	}
	path := "/api/v1/organizations/" + orgID.String() + "/communications/" + entry.ID.String()

	// When: PATCH /communications/{comm_id} is called
	resp := doCommLogRequest(t, ts, http.MethodPatch, path, body)

	// Then: 200 OK with updated entry
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data *com.CommunicationLog `json:"data"`
	}
	decodeCommLogBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, entry.ID, envelope.Data.ID)
}

// ─── ListByUnit tests ──────────────────────────────────────────────────────────

func TestListCommunicationsByUnit_Success(t *testing.T) {
	// Given: two communication entries for a unit
	ts := setupCommLogTestServer(t)
	orgID := uuid.New()
	unitID := uuid.New()

	entry1 := seedCommLog(t, ts.mockCommLogRepo, orgID)
	entry1.UnitID = &unitID
	ts.mockCommLogRepo.items[entry1.ID] = entry1

	entry2 := seedCommLog(t, ts.mockCommLogRepo, orgID)
	entry2.UnitID = &unitID
	ts.mockCommLogRepo.items[entry2.ID] = entry2

	path := "/api/v1/organizations/" + orgID.String() + "/units/" + unitID.String() + "/communications"

	// When: GET /units/{unit_id}/communications is called
	resp := doCommLogRequest(t, ts, http.MethodGet, path, nil)

	// Then: 200 OK with two entries
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data []com.CommunicationLog `json:"data"`
	}
	decodeCommLogBody(t, resp, &envelope)
	assert.Len(t, envelope.Data, 2)
}

func TestListCommunicationsByUnit_InvalidUnitID(t *testing.T) {
	// Given: an invalid unit UUID
	ts := setupCommLogTestServer(t)
	orgID := uuid.New()

	path := "/api/v1/organizations/" + orgID.String() + "/units/bad-uuid/communications"

	// When: GET with bad unit_id
	resp := doCommLogRequest(t, ts, http.MethodGet, path, nil)

	// Then: 400 Bad Request
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()
}
