package ai_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/ai"
	"github.com/quorant/quorant/internal/org"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupComplianceTestServer(t *testing.T, svc *ai.ComplianceService, checkRepo *mockComplianceCheckRepo) *httptest.Server {
	t.Helper()
	handler := ai.NewComplianceHandler(svc, checkRepo, slog.New(slog.NewTextHandler(io.Discard, nil)))
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/organizations/{org_id}/compliance", handler.GetComplianceReport)
	mux.HandleFunc("GET /api/v1/organizations/{org_id}/compliance/history", handler.GetComplianceHistory)
	mux.HandleFunc("GET /api/v1/organizations/{org_id}/compliance/{category}", handler.CheckCategory)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts
}

func TestComplianceHandler_GetReport(t *testing.T) {
	// Given an org with State="FL" and all 7 evaluators registered,
	// when GET /api/v1/organizations/{org_id}/compliance is called,
	// then the response is 200 and the report has 7 results.
	ruleRepo := &mockJurisdictionRuleRepo{}
	checkRepo := &mockComplianceCheckRepo{}
	orgLookup := newMockOrgLookup()

	orgID := uuid.New()
	state := "FL"
	website := "https://myhoaflorida.com"
	orgLookup.orgs[orgID] = &org.Organization{
		ID:      orgID,
		Name:    "Sunset Palms HOA",
		Type:    "hoa",
		State:   &state,
		Website: &website,
	}

	svc := newTestComplianceService(ruleRepo, checkRepo, orgLookup)
	svc.RegisterEvaluator("meeting_notice", ai.EvaluateMeetingNotice)
	svc.RegisterEvaluator("fine_limits", ai.EvaluateFineLimits)
	svc.RegisterEvaluator("reserve_study", ai.EvaluateReserveStudy)
	svc.RegisterEvaluator("website_requirements", ai.EvaluateWebsiteRequirements)
	svc.RegisterEvaluator("record_retention", ai.EvaluateRecordRetention)
	svc.RegisterEvaluator("voting_rules", ai.EvaluateVotingRules)
	svc.RegisterEvaluator("estoppel", ai.EvaluateEstoppel)

	ts := setupComplianceTestServer(t, svc, checkRepo)

	resp, err := http.Get(ts.URL + "/api/v1/organizations/" + orgID.String() + "/compliance")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))

	data, ok := result["data"].(map[string]any)
	require.True(t, ok, "response should have a data field")

	results, ok := data["results"].([]any)
	require.True(t, ok, "data should have a results array")
	assert.Len(t, results, 7, "expected 7 compliance results (one per ValidRuleCategory)")
}

func TestComplianceHandler_CheckCategory_Invalid(t *testing.T) {
	// Given an invalid category in the URL path,
	// when GET /api/v1/organizations/{org_id}/compliance/{category} is called,
	// then the response is 400.
	ruleRepo := &mockJurisdictionRuleRepo{}
	checkRepo := &mockComplianceCheckRepo{}
	orgLookup := newMockOrgLookup()

	svc := newTestComplianceService(ruleRepo, checkRepo, orgLookup)
	ts := setupComplianceTestServer(t, svc, checkRepo)

	orgID := uuid.New()
	resp, err := http.Get(ts.URL + "/api/v1/organizations/" + orgID.String() + "/compliance/totally_invalid_category")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}
