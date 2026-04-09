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

type meetingTestServer struct {
	server      *httptest.Server
	mockMeeting *mockMeetingRepo
	userID      uuid.UUID
}

func setupMeetingTestServer(t *testing.T) *meetingTestServer {
	t.Helper()

	mockViolation := newMockViolationRepo()
	mockARB := newMockARBRepo()
	mockBallot := newMockBallotRepo()
	mockMeeting := newMockMeetingRepo()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	service := gov.NewGovService(mockViolation, mockARB, mockBallot, mockMeeting, audit.NewNoopAuditor(), queue.NewInMemoryPublisher(), logger)
	handler := gov.NewMeetingHandler(service, logger)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /organizations/{org_id}/meetings", handler.ScheduleMeeting)
	mux.HandleFunc("GET /organizations/{org_id}/meetings", handler.ListMeetings)
	mux.HandleFunc("GET /organizations/{org_id}/meetings/{meeting_id}", handler.GetMeeting)
	mux.HandleFunc("PATCH /organizations/{org_id}/meetings/{meeting_id}", handler.UpdateMeeting)
	mux.HandleFunc("POST /organizations/{org_id}/meetings/{meeting_id}/attendees", handler.AddAttendee)
	mux.HandleFunc("POST /organizations/{org_id}/meetings/{meeting_id}/motions", handler.RecordMotion)
	mux.HandleFunc("PATCH /organizations/{org_id}/meetings/{meeting_id}/motions/{motion_id}", handler.UpdateMotion)

	testUserID := uuid.New()
	handlerWithUserID := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := middleware.WithUserID(r.Context(), testUserID)
		mux.ServeHTTP(w, r.WithContext(ctx))
	})
	server := httptest.NewServer(handlerWithUserID)
	t.Cleanup(server.Close)

	return &meetingTestServer{
		server:      server,
		mockMeeting: mockMeeting,
		userID:      testUserID,
	}
}

// doMeetingRequest sends an HTTP request to the meeting test server.
func doMeetingRequest(t *testing.T, serverURL, method, path string, body any) *http.Response {
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

// decodeMeetingBody JSON-decodes the response body into dst.
func decodeMeetingBody(t *testing.T, resp *http.Response, dst any) {
	t.Helper()
	defer resp.Body.Close()
	require.NoError(t, json.NewDecoder(resp.Body).Decode(dst))
}

// seedMeeting pre-populates the mock with a meeting.
func seedMeeting(t *testing.T, repo *mockMeetingRepo, orgID uuid.UUID) *gov.Meeting {
	t.Helper()
	m := &gov.Meeting{
		ID:          uuid.New(),
		OrgID:       orgID,
		Title:       "Annual HOA Meeting",
		MeetingType: "annual",
		Status:      "scheduled",
		ScheduledAt: time.Now().Add(30 * 24 * time.Hour),
		CreatedBy:   uuid.New(),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	repo.meetings[m.ID] = m
	return m
}

// ── ScheduleMeeting tests ─────────────────────────────────────────────────────

func TestScheduleMeetingHandler_Success(t *testing.T) {
	ts := setupMeetingTestServer(t)
	orgID := uuid.New()

	body := map[string]any{
		"title":        "Annual HOA Meeting",
		"meeting_type": "annual",
		"scheduled_at": time.Now().Add(30 * 24 * time.Hour).Format(time.RFC3339),
	}
	resp := doMeetingRequest(t, ts.server.URL, http.MethodPost,
		fmt.Sprintf("/organizations/%s/meetings", orgID), body)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var envelope struct {
		Data *gov.Meeting `json:"data"`
	}
	decodeMeetingBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, orgID, envelope.Data.OrgID)
	assert.Equal(t, "scheduled", envelope.Data.Status)
	assert.Equal(t, "annual", envelope.Data.MeetingType)
	assert.NotEqual(t, uuid.Nil, envelope.Data.ID)
}

func TestScheduleMeeting_MissingTitle(t *testing.T) {
	ts := setupMeetingTestServer(t)
	orgID := uuid.New()

	body := map[string]any{
		"meeting_type": "annual",
		"scheduled_at": time.Now().Add(30 * 24 * time.Hour).Format(time.RFC3339),
	}
	resp := doMeetingRequest(t, ts.server.URL, http.MethodPost,
		fmt.Sprintf("/organizations/%s/meetings", orgID), body)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestScheduleMeeting_InvalidOrgID(t *testing.T) {
	ts := setupMeetingTestServer(t)

	body := map[string]any{
		"title":        "Test",
		"meeting_type": "annual",
		"scheduled_at": time.Now().Add(30 * 24 * time.Hour).Format(time.RFC3339),
	}
	resp := doMeetingRequest(t, ts.server.URL, http.MethodPost,
		"/organizations/not-a-uuid/meetings", body)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// ── ListMeetings tests ────────────────────────────────────────────────────────

func TestListMeetings_Success(t *testing.T) {
	ts := setupMeetingTestServer(t)
	orgID := uuid.New()
	seedMeeting(t, ts.mockMeeting, orgID)
	seedMeeting(t, ts.mockMeeting, orgID)

	resp := doMeetingRequest(t, ts.server.URL, http.MethodGet,
		fmt.Sprintf("/organizations/%s/meetings", orgID), nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data []gov.Meeting `json:"data"`
	}
	decodeMeetingBody(t, resp, &envelope)
	assert.Len(t, envelope.Data, 2)
}

// ── GetMeeting tests ──────────────────────────────────────────────────────────

func TestGetMeeting_Success(t *testing.T) {
	ts := setupMeetingTestServer(t)
	orgID := uuid.New()
	meeting := seedMeeting(t, ts.mockMeeting, orgID)

	resp := doMeetingRequest(t, ts.server.URL, http.MethodGet,
		fmt.Sprintf("/organizations/%s/meetings/%s", orgID, meeting.ID), nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data *gov.Meeting `json:"data"`
	}
	decodeMeetingBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, meeting.ID, envelope.Data.ID)
}

func TestGetMeeting_NotFound(t *testing.T) {
	ts := setupMeetingTestServer(t)
	orgID := uuid.New()

	resp := doMeetingRequest(t, ts.server.URL, http.MethodGet,
		fmt.Sprintf("/organizations/%s/meetings/%s", orgID, uuid.New()), nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// ── AddAttendee tests ─────────────────────────────────────────────────────────

func TestAddAttendee_Success(t *testing.T) {
	ts := setupMeetingTestServer(t)
	orgID := uuid.New()
	meeting := seedMeeting(t, ts.mockMeeting, orgID)
	userID := uuid.New()

	body := map[string]any{
		"user_id": userID,
		"role":    "member",
	}
	resp := doMeetingRequest(t, ts.server.URL, http.MethodPost,
		fmt.Sprintf("/organizations/%s/meetings/%s/attendees", orgID, meeting.ID), body)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var envelope struct {
		Data *gov.MeetingAttendee `json:"data"`
	}
	decodeMeetingBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, meeting.ID, envelope.Data.MeetingID)
	assert.Equal(t, userID, envelope.Data.UserID)
	assert.Equal(t, "member", envelope.Data.Role)
}

func TestAddAttendee_InvalidMeetingID(t *testing.T) {
	ts := setupMeetingTestServer(t)
	orgID := uuid.New()

	body := map[string]any{
		"user_id": uuid.New(),
		"role":    "member",
	}
	resp := doMeetingRequest(t, ts.server.URL, http.MethodPost,
		fmt.Sprintf("/organizations/%s/meetings/bad-uuid/attendees", orgID), body)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// ── RecordMotion tests ────────────────────────────────────────────────────────

func TestRecordMotion_Success(t *testing.T) {
	ts := setupMeetingTestServer(t)
	orgID := uuid.New()
	meeting := seedMeeting(t, ts.mockMeeting, orgID)
	movedBy := uuid.New()

	body := map[string]any{
		"title":    "Approve 2026 budget",
		"moved_by": movedBy,
	}
	resp := doMeetingRequest(t, ts.server.URL, http.MethodPost,
		fmt.Sprintf("/organizations/%s/meetings/%s/motions", orgID, meeting.ID), body)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var envelope struct {
		Data *gov.MeetingMotion `json:"data"`
	}
	decodeMeetingBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, meeting.ID, envelope.Data.MeetingID)
	assert.Equal(t, "Approve 2026 budget", envelope.Data.Title)
	assert.Equal(t, "pending", envelope.Data.Status)
	assert.Equal(t, int16(1), envelope.Data.MotionNumber)
}

func TestRecordMotion_MissingTitle(t *testing.T) {
	ts := setupMeetingTestServer(t)
	orgID := uuid.New()
	meeting := seedMeeting(t, ts.mockMeeting, orgID)

	body := map[string]any{
		"moved_by": uuid.New(),
	}
	resp := doMeetingRequest(t, ts.server.URL, http.MethodPost,
		fmt.Sprintf("/organizations/%s/meetings/%s/motions", orgID, meeting.ID), body)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// ── UpdateMotion tests ────────────────────────────────────────────────────────

func TestUpdateMotion_Success(t *testing.T) {
	ts := setupMeetingTestServer(t)
	orgID := uuid.New()
	meeting := seedMeeting(t, ts.mockMeeting, orgID)

	// Seed a motion directly.
	motion := &gov.MeetingMotion{
		ID:           uuid.New(),
		MeetingID:    meeting.ID,
		MotionNumber: 1,
		Title:        "Approve budget",
		MovedBy:      uuid.New(),
		Status:       "pending",
		CreatedAt:    time.Now(),
	}
	ts.mockMeeting.motions[meeting.ID] = []*gov.MeetingMotion{motion}

	body := map[string]any{
		"meeting_id": meeting.ID,
		"title":      "Approve budget",
		"moved_by":   motion.MovedBy,
		"status":     "passed",
	}
	resp := doMeetingRequest(t, ts.server.URL, http.MethodPatch,
		fmt.Sprintf("/organizations/%s/meetings/%s/motions/%s", orgID, meeting.ID, motion.ID), body)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data *gov.MeetingMotion `json:"data"`
	}
	decodeMeetingBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, motion.ID, envelope.Data.ID)
}

func TestUpdateMotion_InvalidMotionID(t *testing.T) {
	ts := setupMeetingTestServer(t)
	orgID := uuid.New()
	meeting := seedMeeting(t, ts.mockMeeting, orgID)

	body := map[string]any{"status": "passed"}
	resp := doMeetingRequest(t, ts.server.URL, http.MethodPatch,
		fmt.Sprintf("/organizations/%s/meetings/%s/motions/bad-uuid", orgID, meeting.ID), body)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}
