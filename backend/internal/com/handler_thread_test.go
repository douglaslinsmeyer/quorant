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

type threadTestServer struct {
	server         *httptest.Server
	mockThreadRepo *mockThreadRepo
	userID         uuid.UUID
}

func setupThreadTestServer(t *testing.T) *threadTestServer {
	t.Helper()

	mockTRepo := newMockThreadRepo()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	svc := newTestService(
		newMockAnnouncementRepo(),
		mockTRepo,
		newMockNotificationRepo(),
		newMockCalendarRepo(),
		newMockTemplateRepo(),
		newMockDirectoryRepo(),
		newMockCommLogRepo(),
	)
	handler := com.NewThreadHandler(svc, logger)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/organizations/{org_id}/threads", handler.CreateThread)
	mux.HandleFunc("GET /api/v1/organizations/{org_id}/threads", handler.ListThreads)
	mux.HandleFunc("GET /api/v1/organizations/{org_id}/threads/{thread_id}", handler.GetThread)
	mux.HandleFunc("POST /api/v1/organizations/{org_id}/threads/{thread_id}/messages", handler.SendMessage)
	mux.HandleFunc("PATCH /api/v1/organizations/{org_id}/threads/{thread_id}/messages/{message_id}", handler.EditMessage)
	mux.HandleFunc("DELETE /api/v1/organizations/{org_id}/threads/{thread_id}/messages/{message_id}", handler.DeleteMessage)

	testUserID := uuid.New()
	handlerWithUserID := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := middleware.WithUserID(r.Context(), testUserID)
		mux.ServeHTTP(w, r.WithContext(ctx))
	})
	server := httptest.NewServer(handlerWithUserID)
	t.Cleanup(server.Close)

	return &threadTestServer{
		server:         server,
		mockThreadRepo: mockTRepo,
		userID:         testUserID,
	}
}

// doThreadRequest sends an HTTP request to the thread test server.
func doThreadRequest(t *testing.T, ts *threadTestServer, method, path string, body any) *http.Response {
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

// decodeThreadBody JSON-decodes the response body into dst.
func decodeThreadBody(t *testing.T, resp *http.Response, dst any) {
	t.Helper()
	defer resp.Body.Close()
	require.NoError(t, json.NewDecoder(resp.Body).Decode(dst))
}

// seedThread pre-populates the mock thread repo.
func seedThread(t *testing.T, repo *mockThreadRepo, orgID uuid.UUID) *com.Thread {
	t.Helper()
	now := time.Now()
	thread := &com.Thread{
		ID:         uuid.New(),
		OrgID:      orgID,
		Subject:    "Test Thread",
		ThreadType: "general",
		CreatedBy:  uuid.New(),
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	repo.threads[thread.ID] = thread
	return thread
}

// seedMessage pre-populates the mock thread repo with a message.
func seedMessage(t *testing.T, repo *mockThreadRepo, threadID uuid.UUID) *com.Message {
	t.Helper()
	msg := &com.Message{
		ID:        uuid.New(),
		ThreadID:  threadID,
		SenderID:  uuid.New(),
		Body:      "Hello there",
		CreatedAt: time.Now(),
	}
	repo.messages[msg.ID] = msg
	return msg
}

// ─── CreateThread tests ────────────────────────────────────────────────────────

func TestCreateThread_Success(t *testing.T) {
	// Given: a valid create thread request
	ts := setupThreadTestServer(t)
	orgID := uuid.New()

	body := map[string]any{
		"subject":     "Community Meeting",
		"thread_type": "meeting",
	}
	path := "/api/v1/organizations/" + orgID.String() + "/threads"

	// When: POST /threads is called
	resp := doThreadRequest(t, ts, http.MethodPost, path, body)

	// Then: 201 Created with the thread
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var envelope struct {
		Data *com.Thread `json:"data"`
	}
	decodeThreadBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.NotEqual(t, uuid.Nil, envelope.Data.ID)
	assert.Equal(t, orgID, envelope.Data.OrgID)
	assert.Equal(t, "Community Meeting", envelope.Data.Subject)
}

func TestCreateThread_MissingSubject(t *testing.T) {
	// Given: a request body missing required subject
	ts := setupThreadTestServer(t)
	orgID := uuid.New()

	body := map[string]any{"thread_type": "general"}
	path := "/api/v1/organizations/" + orgID.String() + "/threads"

	// When: POST /threads is called without subject
	resp := doThreadRequest(t, ts, http.MethodPost, path, body)

	// Then: 400 Bad Request
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()
}

// ─── ListThreads tests ─────────────────────────────────────────────────────────

func TestListThreads_Success(t *testing.T) {
	// Given: two threads seeded for an org
	ts := setupThreadTestServer(t)
	orgID := uuid.New()
	seedThread(t, ts.mockThreadRepo, orgID)
	seedThread(t, ts.mockThreadRepo, orgID)

	path := "/api/v1/organizations/" + orgID.String() + "/threads"

	// When: GET /threads is called
	resp := doThreadRequest(t, ts, http.MethodGet, path, nil)

	// Then: 200 OK with two threads
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data []com.Thread `json:"data"`
	}
	decodeThreadBody(t, resp, &envelope)
	assert.Len(t, envelope.Data, 2)
}

// ─── GetThread tests ───────────────────────────────────────────────────────────

func TestGetThread_Success(t *testing.T) {
	// Given: an existing thread
	ts := setupThreadTestServer(t)
	orgID := uuid.New()
	thread := seedThread(t, ts.mockThreadRepo, orgID)

	path := "/api/v1/organizations/" + orgID.String() + "/threads/" + thread.ID.String()

	// When: GET /threads/{thread_id} is called
	resp := doThreadRequest(t, ts, http.MethodGet, path, nil)

	// Then: 200 OK with the thread
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data *com.Thread `json:"data"`
	}
	decodeThreadBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, thread.ID, envelope.Data.ID)
}

func TestGetThread_NotFound(t *testing.T) {
	// Given: no matching thread
	ts := setupThreadTestServer(t)
	orgID := uuid.New()

	path := "/api/v1/organizations/" + orgID.String() + "/threads/" + uuid.New().String()

	// When: GET /threads/{unknown_id} is called
	resp := doThreadRequest(t, ts, http.MethodGet, path, nil)

	// Then: 404 Not Found
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()
}

func TestGetThread_InvalidUUID(t *testing.T) {
	// Given: an invalid thread UUID
	ts := setupThreadTestServer(t)
	orgID := uuid.New()

	path := "/api/v1/organizations/" + orgID.String() + "/threads/bad-uuid"

	// When: GET with bad thread_id
	resp := doThreadRequest(t, ts, http.MethodGet, path, nil)

	// Then: 400 Bad Request
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()
}

// ─── SendMessage tests ─────────────────────────────────────────────────────────

func TestSendMessage_Handler_Success(t *testing.T) {
	// Given: an existing thread and a valid message body
	ts := setupThreadTestServer(t)
	orgID := uuid.New()
	thread := seedThread(t, ts.mockThreadRepo, orgID)

	body := map[string]any{"body": "Hello everyone!"}
	path := "/api/v1/organizations/" + orgID.String() + "/threads/" + thread.ID.String() + "/messages"

	// When: POST /threads/{thread_id}/messages is called
	resp := doThreadRequest(t, ts, http.MethodPost, path, body)

	// Then: 201 Created with the message
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var envelope struct {
		Data *com.Message `json:"data"`
	}
	decodeThreadBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.NotEqual(t, uuid.Nil, envelope.Data.ID)
	assert.Equal(t, thread.ID, envelope.Data.ThreadID)
	assert.Equal(t, "Hello everyone!", envelope.Data.Body)
}

func TestSendMessage_MissingBody(t *testing.T) {
	// Given: a request body missing required body field
	ts := setupThreadTestServer(t)
	orgID := uuid.New()
	thread := seedThread(t, ts.mockThreadRepo, orgID)

	body := map[string]any{}
	path := "/api/v1/organizations/" + orgID.String() + "/threads/" + thread.ID.String() + "/messages"

	// When: POST /messages with no body
	resp := doThreadRequest(t, ts, http.MethodPost, path, body)

	// Then: 400 Bad Request
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()
}

// ─── EditMessage tests ─────────────────────────────────────────────────────────

func TestEditMessage_Success(t *testing.T) {
	// Given: an existing message
	ts := setupThreadTestServer(t)
	orgID := uuid.New()
	thread := seedThread(t, ts.mockThreadRepo, orgID)
	msg := seedMessage(t, ts.mockThreadRepo, thread.ID)

	body := map[string]any{"body": "Edited message content"}
	path := "/api/v1/organizations/" + orgID.String() + "/threads/" + thread.ID.String() + "/messages/" + msg.ID.String()

	// When: PATCH /messages/{message_id} is called
	resp := doThreadRequest(t, ts, http.MethodPatch, path, body)

	// Then: 200 OK with updated message
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data *com.Message `json:"data"`
	}
	decodeThreadBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, msg.ID, envelope.Data.ID)
}

func TestEditMessage_InvalidUUID(t *testing.T) {
	// Given: an invalid message UUID
	ts := setupThreadTestServer(t)
	orgID := uuid.New()
	thread := seedThread(t, ts.mockThreadRepo, orgID)

	body := map[string]any{"body": "Edited"}
	path := "/api/v1/organizations/" + orgID.String() + "/threads/" + thread.ID.String() + "/messages/bad-uuid"

	// When: PATCH with bad message_id
	resp := doThreadRequest(t, ts, http.MethodPatch, path, body)

	// Then: 400 Bad Request
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()
}

// ─── DeleteMessage tests ───────────────────────────────────────────────────────

func TestDeleteMessage_Success(t *testing.T) {
	// Given: an existing message
	ts := setupThreadTestServer(t)
	orgID := uuid.New()
	thread := seedThread(t, ts.mockThreadRepo, orgID)
	msg := seedMessage(t, ts.mockThreadRepo, thread.ID)

	path := "/api/v1/organizations/" + orgID.String() + "/threads/" + thread.ID.String() + "/messages/" + msg.ID.String()

	// When: DELETE /messages/{message_id} is called
	resp := doThreadRequest(t, ts, http.MethodDelete, path, nil)

	// Then: 204 No Content
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	resp.Body.Close()
	assert.Empty(t, ts.mockThreadRepo.messages)
}

func TestDeleteMessage_InvalidUUID(t *testing.T) {
	// Given: an invalid message UUID
	ts := setupThreadTestServer(t)
	orgID := uuid.New()
	thread := seedThread(t, ts.mockThreadRepo, orgID)

	path := "/api/v1/organizations/" + orgID.String() + "/threads/" + thread.ID.String() + "/messages/bad-uuid"

	// When: DELETE with bad message_id
	resp := doThreadRequest(t, ts, http.MethodDelete, path, nil)

	// Then: 400 Bad Request
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()
}
