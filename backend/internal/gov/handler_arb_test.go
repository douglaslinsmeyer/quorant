package gov_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/audit"
	"github.com/quorant/quorant/internal/gov"
	"github.com/quorant/quorant/internal/platform/middleware"
	"github.com/quorant/quorant/internal/platform/queue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── Test server setup ─────────────────────────────────────────────────────────

type arbTestServer struct {
	server  *httptest.Server
	mockARB *mockARBRepo
	userID  uuid.UUID
}

func setupARBTestServer(t *testing.T) *arbTestServer {
	t.Helper()

	mockViolation := newMockViolationRepo()
	mockARB := newMockARBRepo()
	mockBallot := newMockBallotRepo()
	mockMeeting := newMockMeetingRepo()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	service := gov.NewGovService(mockViolation, mockARB, mockBallot, mockMeeting, audit.NewNoopAuditor(), queue.NewInMemoryPublisher(), logger)
	handler := gov.NewARBHandler(service, logger)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /organizations/{org_id}/arb-requests", handler.SubmitARBRequest)
	mux.HandleFunc("GET /organizations/{org_id}/arb-requests", handler.ListARBRequests)
	mux.HandleFunc("GET /organizations/{org_id}/arb-requests/{request_id}", handler.GetARBRequest)
	mux.HandleFunc("PATCH /organizations/{org_id}/arb-requests/{request_id}", handler.UpdateARBRequest)
	mux.HandleFunc("POST /organizations/{org_id}/arb-requests/{request_id}/votes", handler.CastARBVote)
	mux.HandleFunc("POST /organizations/{org_id}/arb-requests/{request_id}/request-revision", handler.RequestRevision)

	testUserID := uuid.New()
	handlerWithUserID := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := middleware.WithUserID(r.Context(), testUserID)
		mux.ServeHTTP(w, r.WithContext(ctx))
	})
	server := httptest.NewServer(handlerWithUserID)
	t.Cleanup(server.Close)

	return &arbTestServer{
		server:  server,
		mockARB: mockARB,
		userID:  testUserID,
	}
}

// doARBRequest sends an HTTP request to the ARB test server.
func doARBRequest(t *testing.T, serverURL, method, path string, body any) *http.Response {
	t.Helper()

	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		require.NoError(t, err)
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, serverURL+path, bodyReader)
	require.NoError(t, err)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

// decodeARBBody JSON-decodes the response body into dst.
func decodeARBBody(t *testing.T, resp *http.Response, dst any) {
	t.Helper()
	defer resp.Body.Close()
	require.NoError(t, json.NewDecoder(resp.Body).Decode(dst))
}

// seedARBRequest pre-populates the mock with an ARB request.
func seedARBRequest(t *testing.T, repo *mockARBRepo, orgID uuid.UUID) *gov.ARBRequest {
	t.Helper()
	r := &gov.ARBRequest{
		ID:          uuid.New(),
		OrgID:       orgID,
		UnitID:      uuid.New(),
		SubmittedBy: uuid.New(),
		Title:       "New deck",
		Description: "Adding a 10x12 cedar deck",
		Category:    "construction",
		Status:      "submitted",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	repo.requests[r.ID] = r
	return r
}

// ── SubmitARBRequest tests ────────────────────────────────────────────────────

func TestSubmitARBRequestHandler_Success(t *testing.T) {
	ts := setupARBTestServer(t)
	orgID := uuid.New()
	unitID := uuid.New()

	body := map[string]any{
		"unit_id":     unitID,
		"title":       "New deck",
		"description": "Adding a 10x12 cedar deck",
		"category":    "construction",
	}
	resp := doARBRequest(t, ts.server.URL, http.MethodPost,
		fmt.Sprintf("/organizations/%s/arb-requests", orgID), body)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var envelope struct {
		Data *gov.ARBRequest `json:"data"`
	}
	decodeARBBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, orgID, envelope.Data.OrgID)
	assert.Equal(t, "submitted", envelope.Data.Status)
	assert.NotEqual(t, uuid.Nil, envelope.Data.ID)
}

func TestSubmitARBRequest_MissingTitle(t *testing.T) {
	ts := setupARBTestServer(t)
	orgID := uuid.New()

	body := map[string]any{
		"unit_id":     uuid.New(),
		"description": "desc",
		"category":    "construction",
	}
	resp := doARBRequest(t, ts.server.URL, http.MethodPost,
		fmt.Sprintf("/organizations/%s/arb-requests", orgID), body)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestSubmitARBRequest_InvalidOrgID(t *testing.T) {
	ts := setupARBTestServer(t)

	body := map[string]any{
		"unit_id":     uuid.New(),
		"title":       "Test",
		"description": "desc",
		"category":    "construction",
	}
	resp := doARBRequest(t, ts.server.URL, http.MethodPost,
		"/organizations/not-a-uuid/arb-requests", body)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// ── ListARBRequests tests ─────────────────────────────────────────────────────

func TestListARBRequests_Success(t *testing.T) {
	ts := setupARBTestServer(t)
	orgID := uuid.New()
	seedARBRequest(t, ts.mockARB, orgID)
	seedARBRequest(t, ts.mockARB, orgID)

	resp := doARBRequest(t, ts.server.URL, http.MethodGet,
		fmt.Sprintf("/organizations/%s/arb-requests", orgID), nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data []gov.ARBRequest `json:"data"`
	}
	decodeARBBody(t, resp, &envelope)
	assert.Len(t, envelope.Data, 2)
}

// ── GetARBRequest tests ───────────────────────────────────────────────────────

func TestGetARBRequest_Success(t *testing.T) {
	ts := setupARBTestServer(t)
	orgID := uuid.New()
	arb := seedARBRequest(t, ts.mockARB, orgID)

	resp := doARBRequest(t, ts.server.URL, http.MethodGet,
		fmt.Sprintf("/organizations/%s/arb-requests/%s", orgID, arb.ID), nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data *gov.ARBRequest `json:"data"`
	}
	decodeARBBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, arb.ID, envelope.Data.ID)
}

func TestGetARBRequest_NotFound(t *testing.T) {
	ts := setupARBTestServer(t)
	orgID := uuid.New()

	resp := doARBRequest(t, ts.server.URL, http.MethodGet,
		fmt.Sprintf("/organizations/%s/arb-requests/%s", orgID, uuid.New()), nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// ── CastARBVote tests ─────────────────────────────────────────────────────────

func TestCastARBVote_Success(t *testing.T) {
	ts := setupARBTestServer(t)
	orgID := uuid.New()
	arb := seedARBRequest(t, ts.mockARB, orgID)

	body := map[string]any{"vote": "approve"}
	resp := doARBRequest(t, ts.server.URL, http.MethodPost,
		fmt.Sprintf("/organizations/%s/arb-requests/%s/votes", orgID, arb.ID), body)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var envelope struct {
		Data *gov.ARBVote `json:"data"`
	}
	decodeARBBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, "approve", envelope.Data.Vote)
	assert.Equal(t, arb.ID, envelope.Data.ARBRequestID)
}

func TestCastARBVote_InvalidVote(t *testing.T) {
	ts := setupARBTestServer(t)
	orgID := uuid.New()
	arb := seedARBRequest(t, ts.mockARB, orgID)

	body := map[string]any{"vote": "maybe"}
	resp := doARBRequest(t, ts.server.URL, http.MethodPost,
		fmt.Sprintf("/organizations/%s/arb-requests/%s/votes", orgID, arb.ID), body)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// ── RequestRevision tests ─────────────────────────────────────────────────────

func TestRequestRevision_Success(t *testing.T) {
	ts := setupARBTestServer(t)
	orgID := uuid.New()
	arb := seedARBRequest(t, ts.mockARB, orgID)

	resp := doARBRequest(t, ts.server.URL, http.MethodPost,
		fmt.Sprintf("/organizations/%s/arb-requests/%s/request-revision", orgID, arb.ID), nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data *gov.ARBRequest `json:"data"`
	}
	decodeARBBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, "revision_requested", envelope.Data.Status)
	assert.Equal(t, int16(1), envelope.Data.RevisionCount)
}

func TestRequestRevision_NotFound(t *testing.T) {
	ts := setupARBTestServer(t)
	orgID := uuid.New()

	resp := doARBRequest(t, ts.server.URL, http.MethodPost,
		fmt.Sprintf("/organizations/%s/arb-requests/%s/request-revision", orgID, uuid.New()), nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}
