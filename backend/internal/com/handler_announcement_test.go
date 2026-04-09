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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Test server setup ─────────────────────────────────────────────────────────

type announcementTestServer struct {
	server           *httptest.Server
	mockAnnRepo      *mockAnnouncementRepo
}

func setupAnnouncementTestServer(t *testing.T) *announcementTestServer {
	t.Helper()

	mockAnnRepo := newMockAnnouncementRepo()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	svc := newTestService(
		mockAnnRepo,
		newMockThreadRepo(),
		newMockNotificationRepo(),
		newMockCalendarRepo(),
		newMockTemplateRepo(),
		newMockDirectoryRepo(),
		newMockCommLogRepo(),
	)
	handler := com.NewAnnouncementHandler(svc, logger)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/organizations/{org_id}/announcements", handler.Create)
	mux.HandleFunc("GET /api/v1/organizations/{org_id}/announcements", handler.List)
	mux.HandleFunc("GET /api/v1/organizations/{org_id}/announcements/{announcement_id}", handler.Get)
	mux.HandleFunc("PATCH /api/v1/organizations/{org_id}/announcements/{announcement_id}", handler.Update)
	mux.HandleFunc("DELETE /api/v1/organizations/{org_id}/announcements/{announcement_id}", handler.Delete)

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	return &announcementTestServer{
		server:      server,
		mockAnnRepo: mockAnnRepo,
	}
}

// doAnnRequest sends an HTTP request to the announcement test server.
func doAnnRequest(t *testing.T, ts *announcementTestServer, method, path string, body any) *http.Response {
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

// decodeAnnBody JSON-decodes the response body into dst.
func decodeAnnBody(t *testing.T, resp *http.Response, dst any) {
	t.Helper()
	defer resp.Body.Close()
	require.NoError(t, json.NewDecoder(resp.Body).Decode(dst))
}

// seedAnnouncement pre-populates the mock announcement repo with an announcement.
func seedAnnouncement(t *testing.T, repo *mockAnnouncementRepo, orgID uuid.UUID) *com.Announcement {
	t.Helper()
	now := time.Now()
	a := &com.Announcement{
		ID:        uuid.New(),
		OrgID:     orgID,
		AuthorID:  uuid.New(),
		Title:     "Test Announcement",
		Body:      "Body content here.",
		CreatedAt: now,
		UpdatedAt: now,
	}
	repo.items[a.ID] = a
	return a
}

// ─── Create tests ──────────────────────────────────────────────────────────────

func TestCreateAnnouncement_Success(t *testing.T) {
	// Given: a valid create request
	ts := setupAnnouncementTestServer(t)
	orgID := uuid.New()

	body := map[string]any{
		"title": "Welcome to the Community",
		"body":  "We are excited to announce the new portal.",
	}
	path := "/api/v1/organizations/" + orgID.String() + "/announcements"

	// When: POST /announcements is called
	resp := doAnnRequest(t, ts, http.MethodPost, path, body)

	// Then: 201 Created with the announcement in the response
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var envelope struct {
		Data *com.Announcement `json:"data"`
	}
	decodeAnnBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.NotEqual(t, uuid.Nil, envelope.Data.ID)
	assert.Equal(t, orgID, envelope.Data.OrgID)
	assert.Equal(t, "Welcome to the Community", envelope.Data.Title)
}

func TestCreateAnnouncement_InvalidBody_MissingTitle(t *testing.T) {
	// Given: a request body missing required title
	ts := setupAnnouncementTestServer(t)
	orgID := uuid.New()

	body := map[string]any{"body": "Some content"}
	path := "/api/v1/organizations/" + orgID.String() + "/announcements"

	// When: POST /announcements is called
	resp := doAnnRequest(t, ts, http.MethodPost, path, body)

	// Then: 400 Bad Request
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()
}

func TestCreateAnnouncement_InvalidOrgID(t *testing.T) {
	// Given: an invalid org UUID in the path
	ts := setupAnnouncementTestServer(t)

	body := map[string]any{"title": "Title", "body": "Body"}
	path := "/api/v1/organizations/bad-uuid/announcements"

	// When: POST with bad org_id
	resp := doAnnRequest(t, ts, http.MethodPost, path, body)

	// Then: 400 Bad Request
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()
}

// ─── List tests ────────────────────────────────────────────────────────────────

func TestListAnnouncements_Success(t *testing.T) {
	// Given: two announcements seeded for an org
	ts := setupAnnouncementTestServer(t)
	orgID := uuid.New()
	seedAnnouncement(t, ts.mockAnnRepo, orgID)
	seedAnnouncement(t, ts.mockAnnRepo, orgID)

	path := "/api/v1/organizations/" + orgID.String() + "/announcements"

	// When: GET /announcements is called
	resp := doAnnRequest(t, ts, http.MethodGet, path, nil)

	// Then: 200 OK with two announcements
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data []com.Announcement `json:"data"`
	}
	decodeAnnBody(t, resp, &envelope)
	assert.Len(t, envelope.Data, 2)
}

func TestListAnnouncements_EmptyForUnknownOrg(t *testing.T) {
	// Given: no announcements exist
	ts := setupAnnouncementTestServer(t)
	orgID := uuid.New()

	path := "/api/v1/organizations/" + orgID.String() + "/announcements"

	// When: GET /announcements is called
	resp := doAnnRequest(t, ts, http.MethodGet, path, nil)

	// Then: 200 OK with empty list
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data []com.Announcement `json:"data"`
	}
	decodeAnnBody(t, resp, &envelope)
	assert.Empty(t, envelope.Data)
}

// ─── Get tests ─────────────────────────────────────────────────────────────────

func TestGetAnnouncement_Success(t *testing.T) {
	// Given: an existing announcement
	ts := setupAnnouncementTestServer(t)
	orgID := uuid.New()
	a := seedAnnouncement(t, ts.mockAnnRepo, orgID)

	path := "/api/v1/organizations/" + orgID.String() + "/announcements/" + a.ID.String()

	// When: GET /announcements/{id} is called
	resp := doAnnRequest(t, ts, http.MethodGet, path, nil)

	// Then: 200 OK with the announcement
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data *com.Announcement `json:"data"`
	}
	decodeAnnBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, a.ID, envelope.Data.ID)
}

func TestGetAnnouncementHandler_NotFound(t *testing.T) {
	// Given: no matching announcement
	ts := setupAnnouncementTestServer(t)
	orgID := uuid.New()

	path := "/api/v1/organizations/" + orgID.String() + "/announcements/" + uuid.New().String()

	// When: GET /announcements/{unknown_id} is called
	resp := doAnnRequest(t, ts, http.MethodGet, path, nil)

	// Then: 404 Not Found
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()
}

func TestGetAnnouncement_InvalidUUID(t *testing.T) {
	// Given: an invalid announcement UUID
	ts := setupAnnouncementTestServer(t)
	orgID := uuid.New()

	path := "/api/v1/organizations/" + orgID.String() + "/announcements/bad-uuid"

	// When: GET with bad announcement_id
	resp := doAnnRequest(t, ts, http.MethodGet, path, nil)

	// Then: 400 Bad Request
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()
}

// ─── Update tests ──────────────────────────────────────────────────────────────

func TestUpdateAnnouncement_Success(t *testing.T) {
	// Given: an existing announcement
	ts := setupAnnouncementTestServer(t)
	orgID := uuid.New()
	a := seedAnnouncement(t, ts.mockAnnRepo, orgID)

	path := "/api/v1/organizations/" + orgID.String() + "/announcements/" + a.ID.String()
	body := map[string]any{"title": "Updated Title", "body": "Updated body."}

	// When: PATCH /announcements/{id} is called
	resp := doAnnRequest(t, ts, http.MethodPatch, path, body)

	// Then: 200 OK with updated data
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data *com.Announcement `json:"data"`
	}
	decodeAnnBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, a.ID, envelope.Data.ID)
}

func TestUpdateAnnouncement_InvalidUUID(t *testing.T) {
	// Given: an invalid announcement UUID in the path
	ts := setupAnnouncementTestServer(t)
	orgID := uuid.New()

	path := "/api/v1/organizations/" + orgID.String() + "/announcements/bad-uuid"
	body := map[string]any{"title": "Title", "body": "Body"}

	// When: PATCH with bad announcement_id
	resp := doAnnRequest(t, ts, http.MethodPatch, path, body)

	// Then: 400 Bad Request
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()
}

// ─── Delete tests ──────────────────────────────────────────────────────────────

func TestDeleteAnnouncement_Success(t *testing.T) {
	// Given: an existing announcement
	ts := setupAnnouncementTestServer(t)
	orgID := uuid.New()
	a := seedAnnouncement(t, ts.mockAnnRepo, orgID)

	path := "/api/v1/organizations/" + orgID.String() + "/announcements/" + a.ID.String()

	// When: DELETE /announcements/{id} is called
	resp := doAnnRequest(t, ts, http.MethodDelete, path, nil)

	// Then: 204 No Content and the item is removed
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	resp.Body.Close()
	assert.Empty(t, ts.mockAnnRepo.items)
}

func TestDeleteAnnouncement_InvalidUUID(t *testing.T) {
	// Given: an invalid announcement UUID
	ts := setupAnnouncementTestServer(t)
	orgID := uuid.New()

	path := "/api/v1/organizations/" + orgID.String() + "/announcements/bad-uuid"

	// When: DELETE with bad announcement_id
	resp := doAnnRequest(t, ts, http.MethodDelete, path, nil)

	// Then: 400 Bad Request
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()
}
