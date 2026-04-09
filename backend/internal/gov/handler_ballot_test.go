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
	"github.com/quorant/quorant/internal/ai"
	"github.com/quorant/quorant/internal/audit"
	"github.com/quorant/quorant/internal/gov"
	"github.com/quorant/quorant/internal/platform/middleware"
	"github.com/quorant/quorant/internal/platform/queue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── Test server setup ─────────────────────────────────────────────────────────

type ballotTestServer struct {
	server     *httptest.Server
	mockBallot *mockBallotRepo
	userID     uuid.UUID
}

func setupBallotTestServer(t *testing.T) *ballotTestServer {
	t.Helper()

	mockViolation := newMockViolationRepo()
	mockARB := newMockARBRepo()
	mockBallot := newMockBallotRepo()
	mockMeeting := newMockMeetingRepo()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	service := gov.NewGovService(mockViolation, mockARB, mockBallot, mockMeeting, audit.NewNoopAuditor(), queue.NewInMemoryPublisher(), ai.NewNoopPolicyResolver(), logger)
	handler := gov.NewBallotHandler(service, logger)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /organizations/{org_id}/ballots", handler.CreateBallot)
	mux.HandleFunc("GET /organizations/{org_id}/ballots", handler.ListBallots)
	mux.HandleFunc("GET /organizations/{org_id}/ballots/{ballot_id}", handler.GetBallot)
	mux.HandleFunc("PATCH /organizations/{org_id}/ballots/{ballot_id}", handler.UpdateBallot)
	mux.HandleFunc("POST /organizations/{org_id}/ballots/{ballot_id}/votes", handler.CastVote)
	mux.HandleFunc("POST /organizations/{org_id}/ballots/{ballot_id}/proxy", handler.FileProxy)
	mux.HandleFunc("DELETE /organizations/{org_id}/ballots/{ballot_id}/proxy/{proxy_id}", handler.RevokeProxy)

	testUserID := uuid.New()
	handlerWithUserID := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := middleware.WithUserID(r.Context(), testUserID)
		mux.ServeHTTP(w, r.WithContext(ctx))
	})
	server := httptest.NewServer(handlerWithUserID)
	t.Cleanup(server.Close)

	return &ballotTestServer{
		server:     server,
		mockBallot: mockBallot,
		userID:     testUserID,
	}
}

// doBallotRequest sends an HTTP request to the ballot test server.
func doBallotRequest(t *testing.T, serverURL, method, path string, body any) *http.Response {
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

// decodeBallotBody JSON-decodes the response body into dst.
func decodeBallotBody(t *testing.T, resp *http.Response, dst any) {
	t.Helper()
	defer resp.Body.Close()
	require.NoError(t, json.NewDecoder(resp.Body).Decode(dst))
}

// seedBallot pre-populates the mock with a ballot.
func seedBallot(t *testing.T, repo *mockBallotRepo, orgID uuid.UUID) *gov.Ballot {
	t.Helper()
	b := &gov.Ballot{
		ID:          uuid.New(),
		OrgID:       orgID,
		Title:       "Board Election 2026",
		Description: "Annual board member election",
		BallotType:  "election",
		Status:      "open",
		OpensAt:     time.Now().Add(-time.Hour),
		ClosesAt:    time.Now().Add(7 * 24 * time.Hour),
		CreatedBy:   uuid.New(),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	repo.ballots[b.ID] = b
	return b
}

// ── CreateBallot tests ────────────────────────────────────────────────────────

func TestCreateBallotHandler_Success(t *testing.T) {
	ts := setupBallotTestServer(t)
	orgID := uuid.New()

	body := map[string]any{
		"title":       "Board Election 2026",
		"description": "Annual board member election",
		"ballot_type": "election",
		"opens_at":    time.Now().Add(24 * time.Hour).Format(time.RFC3339),
		"closes_at":   time.Now().Add(7 * 24 * time.Hour).Format(time.RFC3339),
	}
	resp := doBallotRequest(t, ts.server.URL, http.MethodPost,
		fmt.Sprintf("/organizations/%s/ballots", orgID), body)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var envelope struct {
		Data *gov.Ballot `json:"data"`
	}
	decodeBallotBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, orgID, envelope.Data.OrgID)
	assert.Equal(t, "draft", envelope.Data.Status)
	assert.Equal(t, "election", envelope.Data.BallotType)
	assert.NotEqual(t, uuid.Nil, envelope.Data.ID)
}

func TestCreateBallot_MissingTitle(t *testing.T) {
	ts := setupBallotTestServer(t)
	orgID := uuid.New()

	body := map[string]any{
		"description": "desc",
		"ballot_type": "election",
		"opens_at":    time.Now().Add(24 * time.Hour).Format(time.RFC3339),
		"closes_at":   time.Now().Add(7 * 24 * time.Hour).Format(time.RFC3339),
	}
	resp := doBallotRequest(t, ts.server.URL, http.MethodPost,
		fmt.Sprintf("/organizations/%s/ballots", orgID), body)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestCreateBallot_InvalidOrgID(t *testing.T) {
	ts := setupBallotTestServer(t)

	body := map[string]any{
		"title":       "Test",
		"description": "desc",
		"ballot_type": "election",
		"opens_at":    time.Now().Add(24 * time.Hour).Format(time.RFC3339),
		"closes_at":   time.Now().Add(7 * 24 * time.Hour).Format(time.RFC3339),
	}
	resp := doBallotRequest(t, ts.server.URL, http.MethodPost,
		"/organizations/not-a-uuid/ballots", body)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// ── ListBallots tests ─────────────────────────────────────────────────────────

func TestListBallots_Success(t *testing.T) {
	ts := setupBallotTestServer(t)
	orgID := uuid.New()
	seedBallot(t, ts.mockBallot, orgID)
	seedBallot(t, ts.mockBallot, orgID)

	resp := doBallotRequest(t, ts.server.URL, http.MethodGet,
		fmt.Sprintf("/organizations/%s/ballots", orgID), nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data []gov.Ballot `json:"data"`
	}
	decodeBallotBody(t, resp, &envelope)
	assert.Len(t, envelope.Data, 2)
}

// ── GetBallot tests ───────────────────────────────────────────────────────────

func TestGetBallot_Success(t *testing.T) {
	ts := setupBallotTestServer(t)
	orgID := uuid.New()
	ballot := seedBallot(t, ts.mockBallot, orgID)

	resp := doBallotRequest(t, ts.server.URL, http.MethodGet,
		fmt.Sprintf("/organizations/%s/ballots/%s", orgID, ballot.ID), nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data *gov.Ballot `json:"data"`
	}
	decodeBallotBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, ballot.ID, envelope.Data.ID)
}

func TestGetBallot_NotFound(t *testing.T) {
	ts := setupBallotTestServer(t)
	orgID := uuid.New()

	resp := doBallotRequest(t, ts.server.URL, http.MethodGet,
		fmt.Sprintf("/organizations/%s/ballots/%s", orgID, uuid.New()), nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// ── CastVote tests ────────────────────────────────────────────────────────────

func TestCastVote_Success(t *testing.T) {
	ts := setupBallotTestServer(t)
	orgID := uuid.New()
	ballot := seedBallot(t, ts.mockBallot, orgID)
	unitID := uuid.New()

	body := map[string]any{
		"unit_id":   unitID,
		"selection": map[string]any{"option": "yes"},
	}
	resp := doBallotRequest(t, ts.server.URL, http.MethodPost,
		fmt.Sprintf("/organizations/%s/ballots/%s/votes", orgID, ballot.ID), body)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var envelope struct {
		Data *gov.BallotVote `json:"data"`
	}
	decodeBallotBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, ballot.ID, envelope.Data.BallotID)
	assert.Equal(t, unitID, envelope.Data.UnitID)
}

func TestCastVote_MissingUnitID(t *testing.T) {
	ts := setupBallotTestServer(t)
	orgID := uuid.New()
	ballot := seedBallot(t, ts.mockBallot, orgID)

	body := map[string]any{
		"selection": map[string]any{"option": "yes"},
	}
	resp := doBallotRequest(t, ts.server.URL, http.MethodPost,
		fmt.Sprintf("/organizations/%s/ballots/%s/votes", orgID, ballot.ID), body)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// ── FileProxy tests ───────────────────────────────────────────────────────────

func TestFileProxyHandler_Success(t *testing.T) {
	ts := setupBallotTestServer(t)
	orgID := uuid.New()
	ballot := seedBallot(t, ts.mockBallot, orgID)

	body := map[string]any{
		"unit_id":  uuid.New(),
		"proxy_id": uuid.New(),
	}
	resp := doBallotRequest(t, ts.server.URL, http.MethodPost,
		fmt.Sprintf("/organizations/%s/ballots/%s/proxy", orgID, ballot.ID), body)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var envelope struct {
		Data *gov.ProxyAuthorization `json:"data"`
	}
	decodeBallotBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, ballot.ID, envelope.Data.BallotID)
	assert.NotEqual(t, uuid.Nil, envelope.Data.ID)
}

func TestFileProxy_MissingProxyID(t *testing.T) {
	ts := setupBallotTestServer(t)
	orgID := uuid.New()
	ballot := seedBallot(t, ts.mockBallot, orgID)

	body := map[string]any{
		"unit_id": uuid.New(),
	}
	resp := doBallotRequest(t, ts.server.URL, http.MethodPost,
		fmt.Sprintf("/organizations/%s/ballots/%s/proxy", orgID, ballot.ID), body)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// ── RevokeProxy tests ─────────────────────────────────────────────────────────

func TestRevokeProxy_Success(t *testing.T) {
	ts := setupBallotTestServer(t)
	orgID := uuid.New()
	ballot := seedBallot(t, ts.mockBallot, orgID)

	// Seed a proxy authorization.
	proxy := &gov.ProxyAuthorization{
		ID:        uuid.New(),
		BallotID:  ballot.ID,
		UnitID:    uuid.New(),
		GrantorID: uuid.New(),
		ProxyID:   uuid.New(),
		FiledAt:   time.Now(),
		CreatedAt: time.Now(),
	}
	ts.mockBallot.proxies[proxy.ID] = proxy

	resp := doBallotRequest(t, ts.server.URL, http.MethodDelete,
		fmt.Sprintf("/organizations/%s/ballots/%s/proxy/%s", orgID, ballot.ID, proxy.ID), nil)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	resp.Body.Close()

	// Verify proxy was removed.
	assert.Empty(t, ts.mockBallot.proxies)
}

func TestRevokeProxy_InvalidProxyID(t *testing.T) {
	ts := setupBallotTestServer(t)
	orgID := uuid.New()
	ballot := seedBallot(t, ts.mockBallot, orgID)

	resp := doBallotRequest(t, ts.server.URL, http.MethodDelete,
		fmt.Sprintf("/organizations/%s/ballots/%s/proxy/not-a-uuid", orgID, ballot.ID), nil)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}
