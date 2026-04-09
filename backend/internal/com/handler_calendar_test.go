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

type calendarTestServer struct {
	server           *httptest.Server
	mockCalendarRepo *mockCalendarRepo
	mockTemplateRepo *mockTemplateRepo
	mockDirRepo      *mockDirectoryRepo
}

func setupCalendarTestServer(t *testing.T) *calendarTestServer {
	t.Helper()

	mockCalRepo := newMockCalendarRepo()
	mockTplRepo := newMockTemplateRepo()
	mockDirRepo := newMockDirectoryRepo()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	svc := newTestService(
		newMockAnnouncementRepo(),
		newMockThreadRepo(),
		newMockNotificationRepo(),
		mockCalRepo,
		mockTplRepo,
		mockDirRepo,
		newMockCommLogRepo(),
	)
	handler := com.NewCalendarHandler(svc, logger)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/organizations/{org_id}/calendar-events", handler.Create)
	mux.HandleFunc("GET /api/v1/organizations/{org_id}/calendar-events", handler.List)
	mux.HandleFunc("GET /api/v1/organizations/{org_id}/calendar-events/{event_id}", handler.Get)
	mux.HandleFunc("PATCH /api/v1/organizations/{org_id}/calendar-events/{event_id}", handler.Update)
	mux.HandleFunc("DELETE /api/v1/organizations/{org_id}/calendar-events/{event_id}", handler.Delete)
	mux.HandleFunc("POST /api/v1/organizations/{org_id}/calendar-events/{event_id}/rsvp", handler.RSVP)
	mux.HandleFunc("GET /api/v1/organizations/{org_id}/message-templates", handler.ListTemplates)
	mux.HandleFunc("POST /api/v1/organizations/{org_id}/message-templates", handler.CreateTemplate)
	mux.HandleFunc("PATCH /api/v1/organizations/{org_id}/message-templates/{template_id}", handler.UpdateTemplate)
	mux.HandleFunc("DELETE /api/v1/organizations/{org_id}/message-templates/{template_id}", handler.DeleteTemplate)
	mux.HandleFunc("GET /api/v1/organizations/{org_id}/directory/preferences", handler.GetDirectoryPrefs)
	mux.HandleFunc("PUT /api/v1/organizations/{org_id}/directory/preferences", handler.UpdateDirectoryPrefs)

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	return &calendarTestServer{
		server:           server,
		mockCalendarRepo: mockCalRepo,
		mockTemplateRepo: mockTplRepo,
		mockDirRepo:      mockDirRepo,
	}
}

// doCalRequest sends an HTTP request to the calendar test server.
func doCalRequest(t *testing.T, ts *calendarTestServer, method, path string, body any) *http.Response {
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

// decodeCalBody JSON-decodes the response body into dst.
func decodeCalBody(t *testing.T, resp *http.Response, dst any) {
	t.Helper()
	defer resp.Body.Close()
	require.NoError(t, json.NewDecoder(resp.Body).Decode(dst))
}

// seedCalendarEvent pre-populates the mock calendar repo with an event.
func seedCalendarEvent(t *testing.T, repo *mockCalendarRepo, orgID uuid.UUID) *com.CalendarEvent {
	t.Helper()
	now := time.Now()
	e := &com.CalendarEvent{
		ID:        uuid.New(),
		OrgID:     orgID,
		Title:     "Annual Meeting",
		EventType: "meeting",
		StartsAt:  now.Add(24 * time.Hour),
		CreatedBy: uuid.New(),
		CreatedAt: now,
		UpdatedAt: now,
	}
	repo.events[e.ID] = e
	return e
}

// seedTemplate pre-populates the mock template repo with a message template.
func seedTemplate(t *testing.T, repo *mockTemplateRepo, orgID uuid.UUID) *com.MessageTemplate {
	t.Helper()
	now := time.Now()
	tpl := &com.MessageTemplate{
		ID:          uuid.New(),
		OrgID:       &orgID,
		TemplateKey: "welcome",
		Channel:     "email",
		Body:        "Welcome to the community!",
		IsActive:    true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	repo.items[tpl.ID] = tpl
	return tpl
}

// ─── Create Calendar Event tests ───────────────────────────────────────────────

func TestCreateCalendarEventHandler_Success(t *testing.T) {
	// Given: a valid create calendar event request
	ts := setupCalendarTestServer(t)
	orgID := uuid.New()

	body := map[string]any{
		"title":      "Board Meeting",
		"event_type": "meeting",
		"starts_at":  time.Now().Add(48 * time.Hour).Format(time.RFC3339),
	}
	path := "/api/v1/organizations/" + orgID.String() + "/calendar-events"

	// When: POST /calendar-events is called
	resp := doCalRequest(t, ts, http.MethodPost, path, body)

	// Then: 201 Created with the event
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var envelope struct {
		Data *com.CalendarEvent `json:"data"`
	}
	decodeCalBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.NotEqual(t, uuid.Nil, envelope.Data.ID)
	assert.Equal(t, orgID, envelope.Data.OrgID)
	assert.Equal(t, "Board Meeting", envelope.Data.Title)
}

func TestCreateCalendarEvent_MissingTitle(t *testing.T) {
	// Given: a request body missing required title
	ts := setupCalendarTestServer(t)
	orgID := uuid.New()

	body := map[string]any{
		"event_type": "meeting",
		"starts_at":  time.Now().Add(48 * time.Hour).Format(time.RFC3339),
	}
	path := "/api/v1/organizations/" + orgID.String() + "/calendar-events"

	// When: POST without title
	resp := doCalRequest(t, ts, http.MethodPost, path, body)

	// Then: 400 Bad Request
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()
}

// ─── List Calendar Events tests ────────────────────────────────────────────────

func TestListCalendarEvents_Success(t *testing.T) {
	// Given: two events seeded for an org
	ts := setupCalendarTestServer(t)
	orgID := uuid.New()
	seedCalendarEvent(t, ts.mockCalendarRepo, orgID)
	seedCalendarEvent(t, ts.mockCalendarRepo, orgID)

	path := "/api/v1/organizations/" + orgID.String() + "/calendar-events"

	// When: GET /calendar-events is called
	resp := doCalRequest(t, ts, http.MethodGet, path, nil)

	// Then: 200 OK with two events
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data []com.CalendarEvent `json:"data"`
	}
	decodeCalBody(t, resp, &envelope)
	assert.Len(t, envelope.Data, 2)
}

// ─── Get Calendar Event tests ──────────────────────────────────────────────────

func TestGetCalendarEvent_Success(t *testing.T) {
	// Given: an existing calendar event
	ts := setupCalendarTestServer(t)
	orgID := uuid.New()
	event := seedCalendarEvent(t, ts.mockCalendarRepo, orgID)

	path := "/api/v1/organizations/" + orgID.String() + "/calendar-events/" + event.ID.String()

	// When: GET /calendar-events/{event_id} is called
	resp := doCalRequest(t, ts, http.MethodGet, path, nil)

	// Then: 200 OK with the event
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data *com.CalendarEvent `json:"data"`
	}
	decodeCalBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, event.ID, envelope.Data.ID)
}

func TestGetCalendarEvent_NotFound(t *testing.T) {
	// Given: no matching event
	ts := setupCalendarTestServer(t)
	orgID := uuid.New()

	path := "/api/v1/organizations/" + orgID.String() + "/calendar-events/" + uuid.New().String()

	// When: GET /calendar-events/{unknown_id}
	resp := doCalRequest(t, ts, http.MethodGet, path, nil)

	// Then: 404 Not Found
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()
}

// ─── Update Calendar Event tests ───────────────────────────────────────────────

func TestUpdateCalendarEvent_Success(t *testing.T) {
	// Given: an existing calendar event
	ts := setupCalendarTestServer(t)
	orgID := uuid.New()
	event := seedCalendarEvent(t, ts.mockCalendarRepo, orgID)

	body := map[string]any{
		"title":      "Updated Meeting",
		"event_type": "meeting",
		"starts_at":  time.Now().Add(72 * time.Hour).Format(time.RFC3339),
	}
	path := "/api/v1/organizations/" + orgID.String() + "/calendar-events/" + event.ID.String()

	// When: PATCH /calendar-events/{event_id} is called
	resp := doCalRequest(t, ts, http.MethodPatch, path, body)

	// Then: 200 OK
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()
}

// ─── Delete Calendar Event tests ───────────────────────────────────────────────

func TestDeleteCalendarEvent_Success(t *testing.T) {
	// Given: an existing calendar event
	ts := setupCalendarTestServer(t)
	orgID := uuid.New()
	event := seedCalendarEvent(t, ts.mockCalendarRepo, orgID)

	path := "/api/v1/organizations/" + orgID.String() + "/calendar-events/" + event.ID.String()

	// When: DELETE /calendar-events/{event_id} is called
	resp := doCalRequest(t, ts, http.MethodDelete, path, nil)

	// Then: 204 No Content
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	resp.Body.Close()
	assert.Empty(t, ts.mockCalendarRepo.events)
}

// ─── RSVP tests ────────────────────────────────────────────────────────────────

func TestRSVP_Success(t *testing.T) {
	// Given: an existing calendar event and a valid RSVP request
	ts := setupCalendarTestServer(t)
	orgID := uuid.New()
	event := seedCalendarEvent(t, ts.mockCalendarRepo, orgID)

	body := map[string]any{
		"status":      "attending",
		"guest_count": 2,
	}
	path := "/api/v1/organizations/" + orgID.String() + "/calendar-events/" + event.ID.String() + "/rsvp"

	// When: POST /calendar-events/{event_id}/rsvp is called
	resp := doCalRequest(t, ts, http.MethodPost, path, body)

	// Then: 201 Created with the RSVP
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var envelope struct {
		Data *com.CalendarEventRSVP `json:"data"`
	}
	decodeCalBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, event.ID, envelope.Data.EventID)
	assert.Equal(t, "attending", envelope.Data.Status)
}

func TestRSVP_InvalidStatus(t *testing.T) {
	// Given: an invalid RSVP status
	ts := setupCalendarTestServer(t)
	orgID := uuid.New()
	event := seedCalendarEvent(t, ts.mockCalendarRepo, orgID)

	body := map[string]any{"status": "not-a-valid-status"}
	path := "/api/v1/organizations/" + orgID.String() + "/calendar-events/" + event.ID.String() + "/rsvp"

	// When: POST /rsvp with an invalid status
	resp := doCalRequest(t, ts, http.MethodPost, path, body)

	// Then: 400 Bad Request
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()
}

// ─── Template tests ────────────────────────────────────────────────────────────

func TestCreateTemplate_Success(t *testing.T) {
	// Given: a valid create template request
	ts := setupCalendarTestServer(t)
	orgID := uuid.New()

	body := map[string]any{
		"template_key": "welcome_email",
		"channel":      "email",
		"body":         "Welcome to the HOA portal!",
	}
	path := "/api/v1/organizations/" + orgID.String() + "/message-templates"

	// When: POST /message-templates is called
	resp := doCalRequest(t, ts, http.MethodPost, path, body)

	// Then: 201 Created with the template
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var envelope struct {
		Data *com.MessageTemplate `json:"data"`
	}
	decodeCalBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.NotEqual(t, uuid.Nil, envelope.Data.ID)
	assert.Equal(t, "welcome_email", envelope.Data.TemplateKey)
}

func TestListTemplates_Success(t *testing.T) {
	// Given: two templates seeded for an org
	ts := setupCalendarTestServer(t)
	orgID := uuid.New()
	seedTemplate(t, ts.mockTemplateRepo, orgID)
	seedTemplate(t, ts.mockTemplateRepo, orgID)

	path := "/api/v1/organizations/" + orgID.String() + "/message-templates"

	// When: GET /message-templates is called
	resp := doCalRequest(t, ts, http.MethodGet, path, nil)

	// Then: 200 OK with two templates
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data []com.MessageTemplate `json:"data"`
	}
	decodeCalBody(t, resp, &envelope)
	assert.Len(t, envelope.Data, 2)
}

func TestUpdateTemplate_Success(t *testing.T) {
	// Given: an existing template
	ts := setupCalendarTestServer(t)
	orgID := uuid.New()
	tpl := seedTemplate(t, ts.mockTemplateRepo, orgID)

	body := map[string]any{
		"template_key": "updated_key",
		"channel":      "email",
		"body":         "Updated body content.",
		"is_active":    true,
	}
	path := "/api/v1/organizations/" + orgID.String() + "/message-templates/" + tpl.ID.String()

	// When: PATCH /message-templates/{template_id} is called
	resp := doCalRequest(t, ts, http.MethodPatch, path, body)

	// Then: 200 OK
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()
}

func TestDeleteTemplate_Success(t *testing.T) {
	// Given: an existing template
	ts := setupCalendarTestServer(t)
	orgID := uuid.New()
	tpl := seedTemplate(t, ts.mockTemplateRepo, orgID)

	path := "/api/v1/organizations/" + orgID.String() + "/message-templates/" + tpl.ID.String()

	// When: DELETE /message-templates/{template_id} is called
	resp := doCalRequest(t, ts, http.MethodDelete, path, nil)

	// Then: 204 No Content
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	resp.Body.Close()
	assert.Empty(t, ts.mockTemplateRepo.items)
}

// ─── Directory Preferences tests ───────────────────────────────────────────────

func TestGetDirectoryPrefs_Success(t *testing.T) {
	// Given: no explicit preferences (returns defaults)
	ts := setupCalendarTestServer(t)
	orgID := uuid.New()

	path := "/api/v1/organizations/" + orgID.String() + "/directory/preferences"

	// When: GET /directory/preferences is called
	resp := doCalRequest(t, ts, http.MethodGet, path, nil)

	// Then: 200 OK with default preferences
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data *com.DirectoryPreference `json:"data"`
	}
	decodeCalBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.False(t, envelope.Data.OptIn)
}

func TestUpdateDirectoryPrefs_Success(t *testing.T) {
	// Given: a valid update request
	ts := setupCalendarTestServer(t)
	orgID := uuid.New()

	optIn := true
	body := map[string]any{"opt_in": optIn}
	path := "/api/v1/organizations/" + orgID.String() + "/directory/preferences"

	// When: PUT /directory/preferences is called
	resp := doCalRequest(t, ts, http.MethodPut, path, body)

	// Then: 200 OK with updated preferences
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data *com.DirectoryPreference `json:"data"`
	}
	decodeCalBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.True(t, envelope.Data.OptIn)
}

func TestUpdateDirectoryPrefs_NoFields(t *testing.T) {
	// Given: a request body with no fields provided
	ts := setupCalendarTestServer(t)
	orgID := uuid.New()

	body := map[string]any{}
	path := "/api/v1/organizations/" + orgID.String() + "/directory/preferences"

	// When: PUT with empty body
	resp := doCalRequest(t, ts, http.MethodPut, path, body)

	// Then: 400 Bad Request
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()
}
