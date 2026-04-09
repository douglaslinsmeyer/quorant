package license_test

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
	"github.com/quorant/quorant/internal/license"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Test server setup ────────────────────────────────────────────────────────

type licenseTestServer struct {
	server *httptest.Server
	repo   *mockLicenseRepo
}

func setupLicenseTestServer(t *testing.T) *licenseTestServer {
	t.Helper()

	repo := newMockLicenseRepo()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	checker := license.NewPostgresEntitlementChecker(repo)
	svc := license.NewLicenseService(repo, checker, logger)
	handler := license.NewLicenseHandler(svc, logger)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/admin/plans", handler.ListPlans)
	mux.HandleFunc("POST /api/v1/admin/plans", handler.CreatePlan)
	mux.HandleFunc("PATCH /api/v1/admin/plans/{plan_id}", handler.UpdatePlan)
	mux.HandleFunc("GET /api/v1/admin/plans/{plan_id}/entitlements", handler.ListEntitlements)
	mux.HandleFunc("POST /api/v1/organizations/{org_id}/subscription", handler.CreateSubscription)
	mux.HandleFunc("GET /api/v1/organizations/{org_id}/subscription", handler.GetSubscription)
	mux.HandleFunc("PATCH /api/v1/organizations/{org_id}/subscription", handler.UpdateSubscription)
	mux.HandleFunc("GET /api/v1/organizations/{org_id}/entitlements", handler.CheckEntitlements)
	mux.HandleFunc("POST /api/v1/admin/organizations/{org_id}/entitlement-overrides", handler.SetOverride)
	mux.HandleFunc("GET /api/v1/organizations/{org_id}/usage", handler.GetUsage)

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	return &licenseTestServer{server: server, repo: repo}
}

// doLicenseRequest is a helper to execute HTTP requests against the test server.
func doLicenseRequest(t *testing.T, ts *licenseTestServer, method, path string, body any) *http.Response {
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

func decodeLicenseBody(t *testing.T, resp *http.Response, dst any) {
	t.Helper()
	defer resp.Body.Close()
	require.NoError(t, json.NewDecoder(resp.Body).Decode(dst))
}

// ─── Plan handler tests ───────────────────────────────────────────────────────

func TestCreatePlan_Handler(t *testing.T) {
	ts := setupLicenseTestServer(t)

	body := map[string]any{
		"name":      "Enterprise",
		"plan_type": "firm",
	}
	resp := doLicenseRequest(t, ts, http.MethodPost, "/api/v1/admin/plans", body)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var envelope struct {
		Data *license.Plan `json:"data"`
	}
	decodeLicenseBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, "Enterprise", envelope.Data.Name)
	assert.NotEqual(t, uuid.Nil, envelope.Data.ID)
}

func TestCreatePlan_Handler_ValidationError(t *testing.T) {
	ts := setupLicenseTestServer(t)

	body := map[string]any{
		"name":      "Bad",
		"plan_type": "invalid_type",
	}
	resp := doLicenseRequest(t, ts, http.MethodPost, "/api/v1/admin/plans", body)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()
}

func TestListPlans_Handler(t *testing.T) {
	ts := setupLicenseTestServer(t)

	// Seed a couple of plans.
	ts.repo.plans[uuid.New()] = &license.Plan{Name: "Plan A", PlanType: "hoa"}
	ts.repo.plans[uuid.New()] = &license.Plan{Name: "Plan B", PlanType: "firm"}

	resp := doLicenseRequest(t, ts, http.MethodGet, "/api/v1/admin/plans", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data []license.Plan `json:"data"`
	}
	decodeLicenseBody(t, resp, &envelope)
	assert.Len(t, envelope.Data, 2)
}

func TestUpdatePlan_Handler_InvalidUUID(t *testing.T) {
	ts := setupLicenseTestServer(t)

	resp := doLicenseRequest(t, ts, http.MethodPatch, "/api/v1/admin/plans/not-a-uuid",
		map[string]any{"name": "New Name"})
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()
}

// ─── Subscription handler tests ───────────────────────────────────────────────

func TestCreateSubscription_Handler(t *testing.T) {
	ts := setupLicenseTestServer(t)

	orgID := uuid.New()
	planID := uuid.New()

	resp := doLicenseRequest(t, ts, http.MethodPost,
		"/api/v1/organizations/"+orgID.String()+"/subscription",
		map[string]any{"plan_id": planID, "org_id": orgID},
	)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var envelope struct {
		Data *license.OrgSubscription `json:"data"`
	}
	decodeLicenseBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, orgID, envelope.Data.OrgID)
	assert.Equal(t, planID, envelope.Data.PlanID)
}

func TestGetSubscription_Handler(t *testing.T) {
	ts := setupLicenseTestServer(t)

	orgID := uuid.New()
	planID := uuid.New()

	ts.repo.subscriptions[orgID] = &license.OrgSubscription{
		ID:       uuid.New(),
		OrgID:    orgID,
		PlanID:   planID,
		Status:   "active",
		StartsAt: time.Now(),
	}

	resp := doLicenseRequest(t, ts, http.MethodGet,
		"/api/v1/organizations/"+orgID.String()+"/subscription", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data *license.OrgSubscription `json:"data"`
	}
	decodeLicenseBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, orgID, envelope.Data.OrgID)
}

func TestGetSubscription_Handler_NotFound(t *testing.T) {
	ts := setupLicenseTestServer(t)

	resp := doLicenseRequest(t, ts, http.MethodGet,
		"/api/v1/organizations/"+uuid.New().String()+"/subscription", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()
}

// ─── CheckEntitlements handler tests ─────────────────────────────────────────

func TestCheckEntitlements_Handler(t *testing.T) {
	ts := setupLicenseTestServer(t)

	orgID := uuid.New()
	planID := uuid.New()

	ts.repo.plans[planID] = &license.Plan{ID: planID, Name: "Pro", PlanType: "hoa"}
	ts.repo.entitlements[planID] = []license.Entitlement{
		{ID: uuid.New(), PlanID: planID, FeatureKey: "feature.one", LimitType: "boolean"},
	}
	ts.repo.subscriptions[orgID] = &license.OrgSubscription{
		ID: uuid.New(), OrgID: orgID, PlanID: planID, Status: "active", StartsAt: time.Now(),
	}

	resp := doLicenseRequest(t, ts, http.MethodGet,
		"/api/v1/organizations/"+orgID.String()+"/entitlements", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data []license.EntitlementResult `json:"data"`
	}
	decodeLicenseBody(t, resp, &envelope)
	require.Len(t, envelope.Data, 1)
	assert.Equal(t, "feature.one", envelope.Data[0].FeatureKey)
	assert.True(t, envelope.Data[0].Allowed)
}

func TestCheckEntitlements_Handler_InvalidOrgID(t *testing.T) {
	ts := setupLicenseTestServer(t)

	resp := doLicenseRequest(t, ts, http.MethodGet,
		"/api/v1/organizations/not-a-uuid/entitlements", nil)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()
}

// ─── SetOverride handler tests ────────────────────────────────────────────────

func TestSetOverride_Handler(t *testing.T) {
	ts := setupLicenseTestServer(t)

	orgID := uuid.New()
	resp := doLicenseRequest(t, ts, http.MethodPost,
		"/api/v1/admin/organizations/"+orgID.String()+"/entitlement-overrides",
		map[string]any{"org_id": orgID, "feature_key": "feature.x"},
	)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var envelope struct {
		Data *license.EntitlementOverride `json:"data"`
	}
	decodeLicenseBody(t, resp, &envelope)
	require.NotNil(t, envelope.Data)
	assert.Equal(t, "feature.x", envelope.Data.FeatureKey)
}

// ─── GetUsage handler tests ───────────────────────────────────────────────────

func TestGetUsage_Handler(t *testing.T) {
	ts := setupLicenseTestServer(t)

	orgID := uuid.New()
	// No subscription or overrides → empty list (not an error).
	resp := doLicenseRequest(t, ts, http.MethodGet,
		"/api/v1/organizations/"+orgID.String()+"/usage", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var envelope struct {
		Data []license.UsageRecord `json:"data"`
	}
	decodeLicenseBody(t, resp, &envelope)
	// data may be nil/empty — just ensure 200 is returned.
	_ = envelope.Data
}
