package com_test

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/com"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Test server setup ─────────────────────────────────────────────────────────

type notificationTestServer struct {
	server               *httptest.Server
	mockNotificationRepo *mockNotificationRepo
}

func setupNotificationTestServer(t *testing.T) *notificationTestServer {
	t.Helper()

	mockNotifRepo := newMockNotificationRepo()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	svc := newTestService(
		newMockAnnouncementRepo(),
		newMockThreadRepo(),
		mockNotifRepo,
		newMockCalendarRepo(),
		newMockTemplateRepo(),
		newMockDirectoryRepo(),
		newMockCommLogRepo(),
	)
	handler := com.NewNotificationHandler(svc, logger)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/notification-preferences", handler.GetPrefs)
	mux.HandleFunc("PUT /api/v1/notification-preferences", handler.UpdatePrefs)
	mux.HandleFunc("POST /api/v1/push-tokens", handler.RegisterToken)
	mux.HandleFunc("DELETE /api/v1/push-tokens/{token_id}", handler.UnregisterToken)

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	return &notificationTestServer{
		server:               server,
		mockNotificationRepo: mockNotifRepo,
	}
}

// doNotifRequest sends an HTTP request to the notification test server.
func doNotifRequest(t *testing.T, ts *notificationTestServer, method, path string, body any) *http.Response {
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

// decodeNotifBody JSON-decodes the response body into dst.
func decodeNotifBody(t *testing.T, resp *http.Response, dst any) {
	t.Helper()
	defer resp.Body.Close()
	require.NoError(t, json.NewDecoder(resp.Body).Decode(dst))
}

// ─── GetPrefs tests ────────────────────────────────────────────────────────────

func TestGetNotificationPrefs_Success(t *testing.T) {
	// Given: no existing preferences (returns empty list)
	ts := setupNotificationTestServer(t)

	// When: GET /notification-preferences is called
	resp := doNotifRequest(t, ts, http.MethodGet, "/api/v1/notification-preferences", nil)

	// Then: 200 OK with an empty (or nil) list — no error in the response
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data  []com.NotificationPreference `json:"data"`
		Errors []any                       `json:"errors"`
	}
	decodeNotifBody(t, resp, &envelope)
	assert.Empty(t, envelope.Errors)
}

// ─── UpdatePrefs tests ─────────────────────────────────────────────────────────

func TestUpdateNotificationPrefs_Success(t *testing.T) {
	// Given: a valid list of notification preferences
	ts := setupNotificationTestServer(t)

	prefs := []map[string]any{
		{
			"user_id":    uuid.New(),
			"org_id":     uuid.New(),
			"channel":    "email",
			"event_type": "announcement.published",
			"enabled":    true,
		},
	}

	// When: PUT /notification-preferences is called
	resp := doNotifRequest(t, ts, http.MethodPut, "/api/v1/notification-preferences", prefs)

	// Then: 200 OK
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()
}

// ─── RegisterToken tests ───────────────────────────────────────────────────────

func TestRegisterPushToken_Success(t *testing.T) {
	// Given: a valid push token registration request
	ts := setupNotificationTestServer(t)

	body := map[string]any{
		"token":    "device-push-token-abc123",
		"platform": "ios",
	}

	// When: POST /push-tokens is called
	resp := doNotifRequest(t, ts, http.MethodPost, "/api/v1/push-tokens", body)

	// Then: 201 Created with the push token
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var envelope struct {
		Data *com.PushToken `json:"data"`
	}
	decodeNotifBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.NotEqual(t, uuid.Nil, envelope.Data.ID)
	assert.Equal(t, "device-push-token-abc123", envelope.Data.Token)
	assert.Equal(t, "ios", envelope.Data.Platform)
}

// ─── UnregisterToken tests ─────────────────────────────────────────────────────

func TestUnregisterPushToken_Success(t *testing.T) {
	// Given: an existing push token
	ts := setupNotificationTestServer(t)
	tokenID := uuid.New()
	ts.mockNotificationRepo.tokens[tokenID] = &com.PushToken{
		ID:       tokenID,
		UserID:   uuid.New(),
		Token:    "some-token",
		Platform: "android",
	}

	// When: DELETE /push-tokens/{token_id} is called
	resp := doNotifRequest(t, ts, http.MethodDelete, "/api/v1/push-tokens/"+tokenID.String(), nil)

	// Then: 204 No Content and token is removed
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	resp.Body.Close()
	assert.Empty(t, ts.mockNotificationRepo.tokens)
}

func TestUnregisterPushToken_InvalidUUID(t *testing.T) {
	// Given: an invalid token UUID in the path
	ts := setupNotificationTestServer(t)

	// When: DELETE with bad token_id
	resp := doNotifRequest(t, ts, http.MethodDelete, "/api/v1/push-tokens/bad-uuid", nil)

	// Then: 400 Bad Request
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()
}
