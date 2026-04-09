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

type violationTestServer struct {
	server        *httptest.Server
	mockViolation *mockViolationRepo
	mockMeeting   *mockMeetingRepo
	userID        uuid.UUID
}

func setupViolationTestServer(t *testing.T) *violationTestServer {
	t.Helper()

	mockViolation := newMockViolationRepo()
	mockARB := newMockARBRepo()
	mockBallot := newMockBallotRepo()
	mockMeeting := newMockMeetingRepo()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	service := gov.NewGovService(mockViolation, mockARB, mockBallot, mockMeeting, audit.NewNoopAuditor(), queue.NewInMemoryPublisher(), logger)
	handler := gov.NewViolationHandler(service, logger)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /organizations/{org_id}/violations", handler.ReportViolation)
	mux.HandleFunc("GET /organizations/{org_id}/violations", handler.ListViolations)
	mux.HandleFunc("GET /organizations/{org_id}/violations/{violation_id}", handler.GetViolation)
	mux.HandleFunc("PATCH /organizations/{org_id}/violations/{violation_id}", handler.UpdateViolation)
	mux.HandleFunc("POST /organizations/{org_id}/violations/{violation_id}/actions", handler.AddAction)
	mux.HandleFunc("POST /organizations/{org_id}/violations/{violation_id}/verify-cure", handler.VerifyCure)
	mux.HandleFunc("POST /organizations/{org_id}/violations/{violation_id}/hearing", handler.ScheduleHearing)
	mux.HandleFunc("GET /organizations/{org_id}/violations/{violation_id}/hearing", handler.GetHearing)
	mux.HandleFunc("PATCH /organizations/{org_id}/violations/{violation_id}/hearing/{hearing_id}", handler.UpdateHearing)

	testUserID := uuid.New()
	handlerWithUserID := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := middleware.WithUserID(r.Context(), testUserID)
		mux.ServeHTTP(w, r.WithContext(ctx))
	})
	server := httptest.NewServer(handlerWithUserID)
	t.Cleanup(server.Close)

	return &violationTestServer{
		server:        server,
		mockViolation: mockViolation,
		mockMeeting:   mockMeeting,
		userID:        testUserID,
	}
}

// doViolationRequest sends an HTTP request to the violation test server.
func doViolationRequest(t *testing.T, serverURL, method, path string, body any) *http.Response {
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

// decodeViolationBody JSON-decodes the response body into dst.
func decodeViolationBody(t *testing.T, resp *http.Response, dst any) {
	t.Helper()
	defer resp.Body.Close()
	require.NoError(t, json.NewDecoder(resp.Body).Decode(dst))
}

// seedViolation pre-populates the mock with a violation and returns it.
func seedViolation(t *testing.T, repo *mockViolationRepo, orgID uuid.UUID) *gov.Violation {
	t.Helper()
	offNum := int16(1)
	v := &gov.Violation{
		ID:            uuid.New(),
		OrgID:         orgID,
		UnitID:        uuid.New(),
		ReportedBy:    uuid.New(),
		Title:         "Overgrown lawn",
		Description:   "Grass exceeds 6 inches",
		Category:      "landscaping",
		Status:        "open",
		Severity:      2,
		OffenseNumber: &offNum,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	repo.violations[v.ID] = v
	return v
}

// ── ReportViolation tests ─────────────────────────────────────────────────────

func TestReportViolation_Success(t *testing.T) {
	ts := setupViolationTestServer(t)
	orgID := uuid.New()
	unitID := uuid.New()

	body := map[string]any{
		"unit_id":     unitID,
		"title":       "Overgrown lawn",
		"description": "Grass exceeds 6 inches",
		"category":    "landscaping",
		"severity":    2,
	}
	resp := doViolationRequest(t, ts.server.URL, http.MethodPost,
		fmt.Sprintf("/organizations/%s/violations", orgID), body)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var envelope struct {
		Data *gov.Violation `json:"data"`
	}
	decodeViolationBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, orgID, envelope.Data.OrgID)
	assert.Equal(t, unitID, envelope.Data.UnitID)
	assert.Equal(t, "open", envelope.Data.Status)
	assert.NotEqual(t, uuid.Nil, envelope.Data.ID)
}

func TestReportViolation_MissingTitle(t *testing.T) {
	ts := setupViolationTestServer(t)
	orgID := uuid.New()

	body := map[string]any{
		"unit_id":     uuid.New(),
		"description": "desc",
		"category":    "noise",
	}
	resp := doViolationRequest(t, ts.server.URL, http.MethodPost,
		fmt.Sprintf("/organizations/%s/violations", orgID), body)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestReportViolation_InvalidOrgID(t *testing.T) {
	ts := setupViolationTestServer(t)

	body := map[string]any{
		"unit_id":     uuid.New(),
		"title":       "Test",
		"description": "desc",
		"category":    "noise",
	}
	resp := doViolationRequest(t, ts.server.URL, http.MethodPost,
		"/organizations/not-a-uuid/violations", body)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// ── ListViolations tests ──────────────────────────────────────────────────────

func TestListViolations_Success(t *testing.T) {
	ts := setupViolationTestServer(t)
	orgID := uuid.New()
	seedViolation(t, ts.mockViolation, orgID)
	seedViolation(t, ts.mockViolation, orgID)

	resp := doViolationRequest(t, ts.server.URL, http.MethodGet,
		fmt.Sprintf("/organizations/%s/violations", orgID), nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data []gov.Violation `json:"data"`
	}
	decodeViolationBody(t, resp, &envelope)
	assert.Len(t, envelope.Data, 2)
}

func TestListViolations_InvalidOrgID(t *testing.T) {
	ts := setupViolationTestServer(t)

	resp := doViolationRequest(t, ts.server.URL, http.MethodGet,
		"/organizations/bad-uuid/violations", nil)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// ── GetViolation tests ────────────────────────────────────────────────────────

func TestGetViolation_Success(t *testing.T) {
	ts := setupViolationTestServer(t)
	orgID := uuid.New()
	v := seedViolation(t, ts.mockViolation, orgID)

	resp := doViolationRequest(t, ts.server.URL, http.MethodGet,
		fmt.Sprintf("/organizations/%s/violations/%s", orgID, v.ID), nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data *gov.Violation `json:"data"`
	}
	decodeViolationBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, v.ID, envelope.Data.ID)
}

func TestGetViolationHandler_NotFound(t *testing.T) {
	ts := setupViolationTestServer(t)
	orgID := uuid.New()

	resp := doViolationRequest(t, ts.server.URL, http.MethodGet,
		fmt.Sprintf("/organizations/%s/violations/%s", orgID, uuid.New()), nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestGetViolation_InvalidUUID(t *testing.T) {
	ts := setupViolationTestServer(t)
	orgID := uuid.New()

	resp := doViolationRequest(t, ts.server.URL, http.MethodGet,
		fmt.Sprintf("/organizations/%s/violations/bad-uuid", orgID), nil)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// ── AddAction tests ───────────────────────────────────────────────────────────

func TestAddAction_Success(t *testing.T) {
	ts := setupViolationTestServer(t)
	orgID := uuid.New()
	v := seedViolation(t, ts.mockViolation, orgID)

	body := map[string]any{
		"action_type": "notice_sent",
	}
	resp := doViolationRequest(t, ts.server.URL, http.MethodPost,
		fmt.Sprintf("/organizations/%s/violations/%s/actions", orgID, v.ID), body)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var envelope struct {
		Data *gov.ViolationAction `json:"data"`
	}
	decodeViolationBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, "notice_sent", envelope.Data.ActionType)
	assert.Equal(t, v.ID, envelope.Data.ViolationID)
}

func TestAddAction_MissingActionType(t *testing.T) {
	ts := setupViolationTestServer(t)
	orgID := uuid.New()
	v := seedViolation(t, ts.mockViolation, orgID)

	body := map[string]any{}
	resp := doViolationRequest(t, ts.server.URL, http.MethodPost,
		fmt.Sprintf("/organizations/%s/violations/%s/actions", orgID, v.ID), body)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// ── VerifyCure tests ──────────────────────────────────────────────────────────

func TestVerifyCure_Success(t *testing.T) {
	ts := setupViolationTestServer(t)
	orgID := uuid.New()
	v := seedViolation(t, ts.mockViolation, orgID)

	resp := doViolationRequest(t, ts.server.URL, http.MethodPost,
		fmt.Sprintf("/organizations/%s/violations/%s/verify-cure", orgID, v.ID), nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data *gov.Violation `json:"data"`
	}
	decodeViolationBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, "resolved", envelope.Data.Status)
	assert.NotNil(t, envelope.Data.CureVerifiedAt)
}

func TestVerifyCure_NotFound(t *testing.T) {
	ts := setupViolationTestServer(t)
	orgID := uuid.New()

	resp := doViolationRequest(t, ts.server.URL, http.MethodPost,
		fmt.Sprintf("/organizations/%s/violations/%s/verify-cure", orgID, uuid.New()), nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// ── ScheduleHearing tests ─────────────────────────────────────────────────────

func TestScheduleHearingHandler_Success(t *testing.T) {
	ts := setupViolationTestServer(t)
	orgID := uuid.New()
	v := seedViolation(t, ts.mockViolation, orgID)
	meetingID := uuid.New()

	body := map[string]any{"meeting_id": meetingID}
	resp := doViolationRequest(t, ts.server.URL, http.MethodPost,
		fmt.Sprintf("/organizations/%s/violations/%s/hearing", orgID, v.ID), body)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var envelope struct {
		Data *gov.HearingLink `json:"data"`
	}
	decodeViolationBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, v.ID, envelope.Data.ViolationID)
	assert.Equal(t, meetingID, envelope.Data.MeetingID)
}

func TestScheduleHearing_MissingMeetingID(t *testing.T) {
	ts := setupViolationTestServer(t)
	orgID := uuid.New()
	v := seedViolation(t, ts.mockViolation, orgID)

	body := map[string]any{}
	resp := doViolationRequest(t, ts.server.URL, http.MethodPost,
		fmt.Sprintf("/organizations/%s/violations/%s/hearing", orgID, v.ID), body)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// ── GetHearing tests ──────────────────────────────────────────────────────────

func TestGetHearing_Success(t *testing.T) {
	ts := setupViolationTestServer(t)
	orgID := uuid.New()
	v := seedViolation(t, ts.mockViolation, orgID)

	// Seed a hearing link directly.
	hearing := &gov.HearingLink{
		ID:          uuid.New(),
		MeetingID:   uuid.New(),
		ViolationID: v.ID,
		CreatedAt:   time.Now(),
	}
	ts.mockMeeting.hearings[v.ID] = hearing
	ts.mockMeeting.hearingsByID[hearing.ID] = hearing

	resp := doViolationRequest(t, ts.server.URL, http.MethodGet,
		fmt.Sprintf("/organizations/%s/violations/%s/hearing", orgID, v.ID), nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data *gov.HearingLink `json:"data"`
	}
	decodeViolationBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, v.ID, envelope.Data.ViolationID)
}

func TestGetHearingHandler_NotFound(t *testing.T) {
	ts := setupViolationTestServer(t)
	orgID := uuid.New()

	resp := doViolationRequest(t, ts.server.URL, http.MethodGet,
		fmt.Sprintf("/organizations/%s/violations/%s/hearing", orgID, uuid.New()), nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// ── UpdateHearing tests ───────────────────────────────────────────────────────

func TestUpdateHearing_Success(t *testing.T) {
	ts := setupViolationTestServer(t)
	orgID := uuid.New()
	v := seedViolation(t, ts.mockViolation, orgID)

	hearing := &gov.HearingLink{
		ID:          uuid.New(),
		MeetingID:   uuid.New(),
		ViolationID: v.ID,
		CreatedAt:   time.Now(),
	}
	ts.mockMeeting.hearings[v.ID] = hearing
	ts.mockMeeting.hearingsByID[hearing.ID] = hearing

	attended := true
	body := map[string]any{"homeowner_attended": attended}
	resp := doViolationRequest(t, ts.server.URL, http.MethodPatch,
		fmt.Sprintf("/organizations/%s/violations/%s/hearing/%s", orgID, v.ID, hearing.ID), body)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data *gov.HearingLink `json:"data"`
	}
	decodeViolationBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, hearing.ID, envelope.Data.ID)
}
