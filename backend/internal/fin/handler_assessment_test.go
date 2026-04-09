package fin_test

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
	"github.com/quorant/quorant/internal/fin"
	"github.com/quorant/quorant/internal/platform/queue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── Test server setup ─────────────────────────────────────────────────────────

type assessmentTestServer struct {
	server          *httptest.Server
	mockAssessRepo  *mockAssessmentRepo
	mockPaymentRepo *mockPaymentRepo
}

func setupAssessmentTestServer(t *testing.T) *assessmentTestServer {
	t.Helper()

	mockAssessRepo := &mockAssessmentRepo{}
	mockPaymentRepo := &mockPaymentRepo{}
	mockBudgetRepo := &mockBudgetRepo{}
	mockFundRepo := &mockFundRepo{}
	mockCollectionRepo := &mockCollectionRepo{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	service := fin.NewFinService(
		mockAssessRepo,
		mockPaymentRepo,
		mockBudgetRepo,
		mockFundRepo,
		mockCollectionRepo,
		audit.NewNoopAuditor(),
		queue.NewInMemoryPublisher(),
		ai.NewNoopPolicyResolver(),
		ai.NewNoopComplianceResolver(),
		logger,
	)
	assessHandler := fin.NewAssessmentHandler(service, logger)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /organizations/{org_id}/assessment-schedules", assessHandler.CreateSchedule)
	mux.HandleFunc("GET /organizations/{org_id}/assessment-schedules", assessHandler.ListSchedules)
	mux.HandleFunc("GET /organizations/{org_id}/assessment-schedules/{schedule_id}", assessHandler.GetSchedule)
	mux.HandleFunc("PATCH /organizations/{org_id}/assessment-schedules/{schedule_id}", assessHandler.UpdateSchedule)
	mux.HandleFunc("POST /organizations/{org_id}/assessment-schedules/{schedule_id}/deactivate", assessHandler.DeactivateSchedule)
	mux.HandleFunc("POST /organizations/{org_id}/assessments", assessHandler.CreateAssessment)
	mux.HandleFunc("GET /organizations/{org_id}/assessments", assessHandler.ListAssessments)
	mux.HandleFunc("GET /organizations/{org_id}/assessments/{assessment_id}", assessHandler.GetAssessment)
	mux.HandleFunc("PATCH /organizations/{org_id}/assessments/{assessment_id}", assessHandler.UpdateAssessment)
	mux.HandleFunc("DELETE /organizations/{org_id}/assessments/{assessment_id}", assessHandler.DeleteAssessment)
	mux.HandleFunc("GET /organizations/{org_id}/units/{unit_id}/ledger", assessHandler.GetUnitLedger)
	mux.HandleFunc("GET /organizations/{org_id}/ledger", assessHandler.GetOrgLedger)

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	return &assessmentTestServer{
		server:          server,
		mockAssessRepo:  mockAssessRepo,
		mockPaymentRepo: mockPaymentRepo,
	}
}

// doFinRequest sends an HTTP request to the test server and returns the response.
func doFinRequest(t *testing.T, serverURL, method, path string, body any) *http.Response {
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

// decodeFinBody JSON-decodes the response body into dst and closes it.
func decodeFinBody(t *testing.T, resp *http.Response, dst any) {
	t.Helper()
	defer resp.Body.Close()
	require.NoError(t, json.NewDecoder(resp.Body).Decode(dst))
}

// seedSchedule seeds a pre-built assessment schedule into the mock repo.
func seedSchedule(t *testing.T, repo *mockAssessmentRepo, orgID uuid.UUID) *fin.AssessmentSchedule {
	t.Helper()
	s := &fin.AssessmentSchedule{
		ID:              uuid.New(),
		OrgID:           orgID,
		Name:            "Monthly HOA Dues",
		Frequency:       "monthly",
		AmountStrategy:  "flat",
		BaseAmountCents: 25000,
		StartsAt:        time.Now(),
		IsActive:        true,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	repo.schedules = append(repo.schedules, *s)
	return s
}

// seedAssessment seeds a pre-built assessment into the mock repo.
func seedAssessment(t *testing.T, repo *mockAssessmentRepo, orgID, unitID uuid.UUID) *fin.Assessment {
	t.Helper()
	a := &fin.Assessment{
		ID:          uuid.New(),
		OrgID:       orgID,
		UnitID:      unitID,
		Description: "Q1 Assessment",
		AmountCents: 25000,
		DueDate:     time.Now().Add(30 * 24 * time.Hour),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	repo.assessments = append(repo.assessments, *a)
	return a
}

// ── CreateSchedule tests ──────────────────────────────────────────────────────

func TestCreateSchedule_Success(t *testing.T) {
	ts := setupAssessmentTestServer(t)
	orgID := uuid.New()

	body := map[string]any{
		"name":              "Monthly HOA Dues",
		"frequency":         "monthly",
		"amount_strategy":   "flat",
		"base_amount_cents": 25000,
		"starts_at":         time.Now().Format(time.RFC3339),
	}
	resp := doFinRequest(t, ts.server.URL, http.MethodPost,
		fmt.Sprintf("/organizations/%s/assessment-schedules", orgID), body)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var envelope struct {
		Data *fin.AssessmentSchedule `json:"data"`
	}
	decodeFinBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, "Monthly HOA Dues", envelope.Data.Name)
	assert.Equal(t, orgID, envelope.Data.OrgID)
	assert.NotEqual(t, uuid.Nil, envelope.Data.ID)
}

func TestCreateSchedule_InvalidBody(t *testing.T) {
	ts := setupAssessmentTestServer(t)
	orgID := uuid.New()

	// Missing required fields (frequency, amount_strategy, base_amount_cents, starts_at)
	body := map[string]any{"name": "Incomplete Schedule"}
	resp := doFinRequest(t, ts.server.URL, http.MethodPost,
		fmt.Sprintf("/organizations/%s/assessment-schedules", orgID), body)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestCreateSchedule_InvalidOrgID(t *testing.T) {
	ts := setupAssessmentTestServer(t)

	body := map[string]any{
		"name":              "Test",
		"frequency":         "monthly",
		"amount_strategy":   "flat",
		"base_amount_cents": 10000,
		"starts_at":         time.Now().Format(time.RFC3339),
	}
	resp := doFinRequest(t, ts.server.URL, http.MethodPost,
		"/organizations/not-a-uuid/assessment-schedules", body)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// ── ListSchedules tests ───────────────────────────────────────────────────────

func TestListSchedules_Success(t *testing.T) {
	ts := setupAssessmentTestServer(t)
	orgID := uuid.New()
	seedSchedule(t, ts.mockAssessRepo, orgID)
	seedSchedule(t, ts.mockAssessRepo, orgID)

	resp := doFinRequest(t, ts.server.URL, http.MethodGet,
		fmt.Sprintf("/organizations/%s/assessment-schedules", orgID), nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data []fin.AssessmentSchedule `json:"data"`
	}
	decodeFinBody(t, resp, &envelope)
	assert.Len(t, envelope.Data, 2)
}

func TestListSchedules_EmptyOrg(t *testing.T) {
	ts := setupAssessmentTestServer(t)
	orgID := uuid.New()

	resp := doFinRequest(t, ts.server.URL, http.MethodGet,
		fmt.Sprintf("/organizations/%s/assessment-schedules", orgID), nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data []fin.AssessmentSchedule `json:"data"`
	}
	decodeFinBody(t, resp, &envelope)
	assert.Empty(t, envelope.Data)
}

// ── GetSchedule tests ─────────────────────────────────────────────────────────

func TestGetSchedule_Success(t *testing.T) {
	ts := setupAssessmentTestServer(t)
	orgID := uuid.New()
	schedule := seedSchedule(t, ts.mockAssessRepo, orgID)

	resp := doFinRequest(t, ts.server.URL, http.MethodGet,
		fmt.Sprintf("/organizations/%s/assessment-schedules/%s", orgID, schedule.ID), nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data *fin.AssessmentSchedule `json:"data"`
	}
	decodeFinBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, schedule.ID, envelope.Data.ID)
}

func TestGetScheduleHandler_NotFound(t *testing.T) {
	ts := setupAssessmentTestServer(t)
	orgID := uuid.New()

	resp := doFinRequest(t, ts.server.URL, http.MethodGet,
		fmt.Sprintf("/organizations/%s/assessment-schedules/%s", orgID, uuid.New()), nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestGetSchedule_InvalidScheduleID(t *testing.T) {
	ts := setupAssessmentTestServer(t)
	orgID := uuid.New()

	resp := doFinRequest(t, ts.server.URL, http.MethodGet,
		fmt.Sprintf("/organizations/%s/assessment-schedules/bad-uuid", orgID), nil)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// ── UpdateSchedule tests ──────────────────────────────────────────────────────

func TestUpdateSchedule_Success(t *testing.T) {
	ts := setupAssessmentTestServer(t)
	orgID := uuid.New()
	schedule := seedSchedule(t, ts.mockAssessRepo, orgID)

	newName := "Updated Dues"
	body := map[string]any{"name": newName}
	resp := doFinRequest(t, ts.server.URL, http.MethodPatch,
		fmt.Sprintf("/organizations/%s/assessment-schedules/%s", orgID, schedule.ID), body)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data *fin.AssessmentSchedule `json:"data"`
	}
	decodeFinBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, newName, envelope.Data.Name)
}

func TestUpdateSchedule_NotFound(t *testing.T) {
	ts := setupAssessmentTestServer(t)
	orgID := uuid.New()

	body := map[string]any{"name": "Ghost Schedule"}
	resp := doFinRequest(t, ts.server.URL, http.MethodPatch,
		fmt.Sprintf("/organizations/%s/assessment-schedules/%s", orgID, uuid.New()), body)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// ── DeactivateSchedule tests ──────────────────────────────────────────────────

func TestDeactivateSchedule_Success(t *testing.T) {
	ts := setupAssessmentTestServer(t)
	orgID := uuid.New()
	schedule := seedSchedule(t, ts.mockAssessRepo, orgID)

	resp := doFinRequest(t, ts.server.URL, http.MethodPost,
		fmt.Sprintf("/organizations/%s/assessment-schedules/%s/deactivate", orgID, schedule.ID), nil)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	resp.Body.Close()

	// Verify it was deactivated.
	assert.False(t, ts.mockAssessRepo.schedules[0].IsActive)
}

func TestDeactivateSchedule_NotFound(t *testing.T) {
	ts := setupAssessmentTestServer(t)
	orgID := uuid.New()

	resp := doFinRequest(t, ts.server.URL, http.MethodPost,
		fmt.Sprintf("/organizations/%s/assessment-schedules/%s/deactivate", orgID, uuid.New()), nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// ── CreateAssessment tests ────────────────────────────────────────────────────

func TestCreateAssessment_Success(t *testing.T) {
	ts := setupAssessmentTestServer(t)
	orgID := uuid.New()
	unitID := uuid.New()

	body := map[string]any{
		"unit_id":      unitID,
		"description":  "Q1 2026 HOA Assessment",
		"amount_cents": 25000,
		"due_date":     time.Now().Add(30 * 24 * time.Hour).Format(time.RFC3339),
	}
	resp := doFinRequest(t, ts.server.URL, http.MethodPost,
		fmt.Sprintf("/organizations/%s/assessments", orgID), body)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var envelope struct {
		Data *fin.Assessment `json:"data"`
	}
	decodeFinBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, orgID, envelope.Data.OrgID)
	assert.Equal(t, unitID, envelope.Data.UnitID)
	assert.Equal(t, int64(25000), envelope.Data.AmountCents)
	assert.NotEqual(t, uuid.Nil, envelope.Data.ID)

	// Verify ledger entry was also created.
	assert.Len(t, ts.mockAssessRepo.ledger, 1)
	assert.Equal(t, "charge", ts.mockAssessRepo.ledger[0].EntryType)
}

func TestCreateAssessment_InvalidBody(t *testing.T) {
	ts := setupAssessmentTestServer(t)
	orgID := uuid.New()

	// Missing required fields
	body := map[string]any{"description": "Missing fields"}
	resp := doFinRequest(t, ts.server.URL, http.MethodPost,
		fmt.Sprintf("/organizations/%s/assessments", orgID), body)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// ── ListAssessments tests ─────────────────────────────────────────────────────

func TestListAssessments_Success(t *testing.T) {
	ts := setupAssessmentTestServer(t)
	orgID := uuid.New()
	unitID := uuid.New()
	seedAssessment(t, ts.mockAssessRepo, orgID, unitID)
	seedAssessment(t, ts.mockAssessRepo, orgID, unitID)

	resp := doFinRequest(t, ts.server.URL, http.MethodGet,
		fmt.Sprintf("/organizations/%s/assessments", orgID), nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data []fin.Assessment `json:"data"`
	}
	decodeFinBody(t, resp, &envelope)
	assert.Len(t, envelope.Data, 2)
}

// ── GetAssessment tests ───────────────────────────────────────────────────────

func TestGetAssessment_Success(t *testing.T) {
	ts := setupAssessmentTestServer(t)
	orgID := uuid.New()
	unitID := uuid.New()
	assessment := seedAssessment(t, ts.mockAssessRepo, orgID, unitID)

	resp := doFinRequest(t, ts.server.URL, http.MethodGet,
		fmt.Sprintf("/organizations/%s/assessments/%s", orgID, assessment.ID), nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data *fin.Assessment `json:"data"`
	}
	decodeFinBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, assessment.ID, envelope.Data.ID)
}

func TestGetAssessmentHandler_NotFound(t *testing.T) {
	ts := setupAssessmentTestServer(t)
	orgID := uuid.New()

	resp := doFinRequest(t, ts.server.URL, http.MethodGet,
		fmt.Sprintf("/organizations/%s/assessments/%s", orgID, uuid.New()), nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// ── GetUnitLedger tests ───────────────────────────────────────────────────────

func TestGetUnitLedger_Success(t *testing.T) {
	ts := setupAssessmentTestServer(t)
	orgID := uuid.New()
	unitID := uuid.New()

	// Seed a ledger entry directly.
	ts.mockAssessRepo.ledger = append(ts.mockAssessRepo.ledger, fin.LedgerEntry{
		ID:            uuid.New(),
		OrgID:         orgID,
		UnitID:        unitID,
		EntryType:     "charge",
		AmountCents:   25000,
		BalanceCents:  25000,
		EffectiveDate: time.Now(),
		CreatedAt:     time.Now(),
	})

	resp := doFinRequest(t, ts.server.URL, http.MethodGet,
		fmt.Sprintf("/organizations/%s/units/%s/ledger", orgID, unitID), nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data []fin.LedgerEntry `json:"data"`
	}
	decodeFinBody(t, resp, &envelope)
	assert.Len(t, envelope.Data, 1)
	assert.Equal(t, int64(25000), envelope.Data[0].AmountCents)
}

func TestGetUnitLedger_EmptyUnit(t *testing.T) {
	ts := setupAssessmentTestServer(t)
	orgID := uuid.New()
	unitID := uuid.New()

	resp := doFinRequest(t, ts.server.URL, http.MethodGet,
		fmt.Sprintf("/organizations/%s/units/%s/ledger", orgID, unitID), nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data []fin.LedgerEntry `json:"data"`
	}
	decodeFinBody(t, resp, &envelope)
	assert.Empty(t, envelope.Data)
}

// ── GetOrgLedger tests ────────────────────────────────────────────────────────

func TestGetOrgLedger_Success(t *testing.T) {
	ts := setupAssessmentTestServer(t)
	orgID := uuid.New()
	unitID := uuid.New()

	ts.mockAssessRepo.ledger = append(ts.mockAssessRepo.ledger,
		fin.LedgerEntry{
			ID:            uuid.New(),
			OrgID:         orgID,
			UnitID:        unitID,
			EntryType:     "charge",
			AmountCents:   25000,
			BalanceCents:  25000,
			EffectiveDate: time.Now(),
			CreatedAt:     time.Now(),
		},
		fin.LedgerEntry{
			ID:            uuid.New(),
			OrgID:         orgID,
			UnitID:        unitID,
			EntryType:     "payment",
			AmountCents:   -25000,
			BalanceCents:  0,
			EffectiveDate: time.Now(),
			CreatedAt:     time.Now(),
		},
	)

	resp := doFinRequest(t, ts.server.URL, http.MethodGet,
		fmt.Sprintf("/organizations/%s/ledger", orgID), nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data []fin.LedgerEntry `json:"data"`
	}
	decodeFinBody(t, resp, &envelope)
	assert.Len(t, envelope.Data, 2)
}

// ── UpdateAssessment tests ────────────────────────────────────────────────────

func TestUpdateAssessment_Success(t *testing.T) {
	ts := setupAssessmentTestServer(t)
	orgID := uuid.New()
	unitID := uuid.New()
	assessment := seedAssessment(t, ts.mockAssessRepo, orgID, unitID)

	newDesc := "Updated Q1 Assessment"
	body := map[string]any{
		"description":  newDesc,
		"amount_cents": 30000,
		"due_date":     time.Now().Add(30 * 24 * time.Hour).Format(time.RFC3339),
	}
	resp := doFinRequest(t, ts.server.URL, http.MethodPatch,
		fmt.Sprintf("/organizations/%s/assessments/%s", orgID, assessment.ID), body)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data *fin.Assessment `json:"data"`
	}
	decodeFinBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, newDesc, envelope.Data.Description)
}

func TestUpdateAssessment_NotFound(t *testing.T) {
	ts := setupAssessmentTestServer(t)
	orgID := uuid.New()

	body := map[string]any{
		"description":  "Ghost",
		"amount_cents": 10000,
		"due_date":     time.Now().Add(30 * 24 * time.Hour).Format(time.RFC3339),
	}
	resp := doFinRequest(t, ts.server.URL, http.MethodPatch,
		fmt.Sprintf("/organizations/%s/assessments/%s", orgID, uuid.New()), body)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()
}

// ── DeleteAssessment tests ────────────────────────────────────────────────────

func TestDeleteAssessment_Success(t *testing.T) {
	ts := setupAssessmentTestServer(t)
	orgID := uuid.New()
	unitID := uuid.New()
	assessment := seedAssessment(t, ts.mockAssessRepo, orgID, unitID)

	resp := doFinRequest(t, ts.server.URL, http.MethodDelete,
		fmt.Sprintf("/organizations/%s/assessments/%s", orgID, assessment.ID), nil)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	resp.Body.Close()
}

func TestDeleteAssessment_NotFound(t *testing.T) {
	ts := setupAssessmentTestServer(t)
	orgID := uuid.New()

	resp := doFinRequest(t, ts.server.URL, http.MethodDelete,
		fmt.Sprintf("/organizations/%s/assessments/%s", orgID, uuid.New()), nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()
}
