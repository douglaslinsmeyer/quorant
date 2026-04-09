package ai_test

import (
	"bytes"
	"context"
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
	"github.com/quorant/quorant/internal/org"
	"github.com/quorant/quorant/internal/platform/api"
	"github.com/quorant/quorant/internal/platform/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Handler-specific mock helpers ───────────────────────────────────────────

// handlerMockOrgRepo is a full in-memory OrgRepository for handler tests.
// It is separate from the context_service_test.go's mockOrgRepo (which uses func fields).
type handlerMockOrgRepo struct {
	orgs map[uuid.UUID]*org.Organization
}

func newHandlerMockOrgRepo() *handlerMockOrgRepo {
	return &handlerMockOrgRepo{orgs: make(map[uuid.UUID]*org.Organization)}
}

func (r *handlerMockOrgRepo) Create(_ context.Context, o *org.Organization) (*org.Organization, error) {
	o.ID = uuid.New()
	cp := *o
	r.orgs[o.ID] = &cp
	return &cp, nil
}

func (r *handlerMockOrgRepo) FindByID(_ context.Context, id uuid.UUID) (*org.Organization, error) {
	o, ok := r.orgs[id]
	if !ok {
		return nil, nil
	}
	cp := *o
	return &cp, nil
}

func (r *handlerMockOrgRepo) FindBySlug(_ context.Context, _ string) (*org.Organization, error) {
	return nil, nil
}

func (r *handlerMockOrgRepo) ListByUserAccess(_ context.Context, _ uuid.UUID, limit int, afterID *uuid.UUID) ([]org.Organization, bool, error) {
	return nil, false, nil
}

func (r *handlerMockOrgRepo) Update(_ context.Context, o *org.Organization) (*org.Organization, error) {
	r.orgs[o.ID] = o
	cp := *o
	return &cp, nil
}

func (r *handlerMockOrgRepo) SoftDelete(_ context.Context, _ uuid.UUID) error { return nil }

func (r *handlerMockOrgRepo) ListChildren(_ context.Context, _ uuid.UUID) ([]org.Organization, error) {
	return nil, nil
}

func (r *handlerMockOrgRepo) ConnectManagement(_ context.Context, _, _ uuid.UUID) (*org.OrgManagement, error) {
	return nil, nil
}

func (r *handlerMockOrgRepo) DisconnectManagement(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (r *handlerMockOrgRepo) ListManagementHistory(_ context.Context, _ uuid.UUID) ([]org.OrgManagement, error) {
	return nil, nil
}

func (r *handlerMockOrgRepo) FindActiveManagement(_ context.Context, _ uuid.UUID) (*org.OrgManagement, error) {
	return nil, nil
}

// ─── Test server setup ────────────────────────────────────────────────────────

type aiTestServer struct {
	server     *httptest.Server
	policyRepo *mockPolicyRepo
	orgRepo    *handlerMockOrgRepo
	userID     uuid.UUID
}

func setupAITestServer(t *testing.T) *aiTestServer {
	t.Helper()

	policyRepo := newMockPolicyRepo()
	orgRepo := newHandlerMockOrgRepo()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	policyService := ai.NewPolicyService(policyRepo, logger)
	contextService := ai.NewContextLakeService(
		&mockContextChunkRepo{},
		&mockOrgRepo{},
		ai.StubEmbeddingFunc,
		logger,
	)
	handler := ai.NewAIHandler(policyService, contextService, orgRepo, nil, logger) // nil cfgStore in tests — falls back to org settings

	mux := http.NewServeMux()

	// Governing documents
	mux.HandleFunc("POST /api/v1/organizations/{org_id}/governing-documents", handler.RegisterGoverningDoc)
	mux.HandleFunc("GET /api/v1/organizations/{org_id}/governing-documents", handler.ListGoverningDocs)
	mux.HandleFunc("GET /api/v1/organizations/{org_id}/governing-documents/{doc_id}", handler.GetGoverningDoc)
	mux.HandleFunc("DELETE /api/v1/organizations/{org_id}/governing-documents/{doc_id}", handler.RemoveGoverningDoc)
	mux.HandleFunc("POST /api/v1/organizations/{org_id}/governing-documents/{doc_id}/reindex", handler.ReindexGoverningDoc)

	// Policy extractions
	mux.HandleFunc("GET /api/v1/organizations/{org_id}/policy-extractions", handler.ListExtractions)
	mux.HandleFunc("GET /api/v1/organizations/{org_id}/policy-extractions/{extraction_id}", handler.GetExtraction)
	mux.HandleFunc("POST /api/v1/organizations/{org_id}/policy-extractions/{extraction_id}/approve", handler.ApproveExtraction)
	mux.HandleFunc("POST /api/v1/organizations/{org_id}/policy-extractions/{extraction_id}/reject", handler.RejectExtraction)
	mux.HandleFunc("POST /api/v1/organizations/{org_id}/policy-extractions/{extraction_id}/modify", handler.ModifyExtraction)

	// Active policies
	mux.HandleFunc("GET /api/v1/organizations/{org_id}/policies/{policy_key}", handler.GetActivePolicy)
	mux.HandleFunc("GET /api/v1/organizations/{org_id}/policies", handler.ListActivePolicies)
	mux.HandleFunc("POST /api/v1/organizations/{org_id}/policy-query", handler.QueryPolicy)

	// Policy resolutions
	mux.HandleFunc("GET /api/v1/organizations/{org_id}/policy-resolutions", handler.ListResolutions)
	mux.HandleFunc("GET /api/v1/organizations/{org_id}/policy-resolutions/{resolution_id}", handler.GetResolution)
	mux.HandleFunc("POST /api/v1/organizations/{org_id}/policy-resolutions/{resolution_id}/decide", handler.DecideResolution)

	// AI config
	mux.HandleFunc("GET /api/v1/organizations/{org_id}/ai/config", handler.GetAIConfig)
	mux.HandleFunc("PUT /api/v1/organizations/{org_id}/ai/config", handler.UpdateAIConfig)

	testUserID := uuid.New()
	handlerWithUserID := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := middleware.WithUserID(r.Context(), testUserID)
		mux.ServeHTTP(w, r.WithContext(ctx))
	})
	server := httptest.NewServer(handlerWithUserID)
	t.Cleanup(server.Close)

	return &aiTestServer{
		server:     server,
		policyRepo: policyRepo,
		orgRepo:    orgRepo,
		userID:     testUserID,
	}
}

// ─── HTTP helpers ─────────────────────────────────────────────────────────────

func doAIRequest(t *testing.T, ts *aiTestServer, method, path string, body any) *http.Response {
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

func decodeAIResponse[T any](t *testing.T, resp *http.Response) T {
	t.Helper()
	defer resp.Body.Close()
	var envelope api.Response
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&envelope))
	b, err := json.Marshal(envelope.Data)
	require.NoError(t, err)
	var out T
	require.NoError(t, json.Unmarshal(b, &out))
	return out
}

// ─── Test helpers: seed data ──────────────────────────────────────────────────

func seedGoverningDoc(t *testing.T, ts *aiTestServer, orgID uuid.UUID) *ai.GoverningDocument {
	t.Helper()
	resp := doAIRequest(t, ts, http.MethodPost,
		fmt.Sprintf("/api/v1/organizations/%s/governing-documents", orgID),
		ai.RegisterGoverningDocRequest{
			DocumentID:    uuid.New().String(),
			DocType:       "ccr",
			Title:         "CC&Rs",
			EffectiveDate: "2024-01-01",
		},
	)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	doc := decodeAIResponse[ai.GoverningDocument](t, resp)
	return &doc
}

func seedExtraction(t *testing.T, ts *aiTestServer, orgID uuid.UUID, policyKey string) *ai.PolicyExtraction {
	t.Helper()
	// Insert directly into the mock repo for simplicity.
	e := &ai.PolicyExtraction{
		ID:                uuid.New(),
		OrgID:             orgID,
		Domain:            "financial",
		PolicyKey:         policyKey,
		Config:            json.RawMessage(`{"rate":0.05}`),
		Confidence:        0.92,
		SourceDocID:       uuid.New(),
		SourceText:        "Sample text",
		ReviewStatus:      "pending",
		ModelVersion:      "gpt-4o",
		EffectiveAt:       time.Now(),
		CreatedAt:         time.Now(),
	}
	ts.policyRepo.extractions[e.ID] = e
	return e
}

func seedResolution(t *testing.T, ts *aiTestServer, orgID uuid.UUID) *ai.PolicyResolution {
	t.Helper()
	r := &ai.PolicyResolution{
		ID:                uuid.New(),
		OrgID:             orgID,
		Query:             "What is the late fee?",
		PolicyKeys:        []string{"late_fee"},
		Resolution:        json.RawMessage(`{"answer":"5%"}`),
		Reasoning:         "Based on CC&Rs section 3.1",
		SourcePassages:    json.RawMessage(`[]`),
		Confidence:        0.88,
		ResolutionType:    "ai_inference",
		RequestingModule:  "fin",
		RequestingContext: json.RawMessage(`{}`),
		CreatedAt:         time.Now(),
	}
	ts.policyRepo.resolutions[r.ID] = r
	return r
}

// ─── Governing Document Tests ─────────────────────────────────────────────────

func TestRegisterGoverningDoc_Handler(t *testing.T) {
	ts := setupAITestServer(t)
	orgID := uuid.New()

	resp := doAIRequest(t, ts, http.MethodPost,
		fmt.Sprintf("/api/v1/organizations/%s/governing-documents", orgID),
		ai.RegisterGoverningDocRequest{
			DocumentID:    uuid.New().String(),
			DocType:       "bylaws",
			Title:         "Bylaws 2024",
			EffectiveDate: "2024-01-01",
		},
	)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	doc := decodeAIResponse[ai.GoverningDocument](t, resp)
	assert.Equal(t, "bylaws", doc.DocType)
	assert.Equal(t, "Bylaws 2024", doc.Title)
	assert.Equal(t, "pending", doc.IndexingStatus)
}

func TestListGoverningDocs_Handler(t *testing.T) {
	ts := setupAITestServer(t)
	orgID := uuid.New()

	seedGoverningDoc(t, ts, orgID)
	seedGoverningDoc(t, ts, orgID)

	resp := doAIRequest(t, ts, http.MethodGet,
		fmt.Sprintf("/api/v1/organizations/%s/governing-documents", orgID),
		nil,
	)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	docs := decodeAIResponse[[]ai.GoverningDocument](t, resp)
	assert.Len(t, docs, 2)
}

func TestGetGoverningDoc_Handler_NotFound(t *testing.T) {
	ts := setupAITestServer(t)
	orgID := uuid.New()
	missingID := uuid.New()

	resp := doAIRequest(t, ts, http.MethodGet,
		fmt.Sprintf("/api/v1/organizations/%s/governing-documents/%s", orgID, missingID),
		nil,
	)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// ─── Policy Extraction Tests ──────────────────────────────────────────────────

func TestListExtractions_Handler(t *testing.T) {
	ts := setupAITestServer(t)
	orgID := uuid.New()

	seedExtraction(t, ts, orgID, "late_fee")
	seedExtraction(t, ts, orgID, "pet_policy")

	resp := doAIRequest(t, ts, http.MethodGet,
		fmt.Sprintf("/api/v1/organizations/%s/policy-extractions", orgID),
		nil,
	)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	extractions := decodeAIResponse[[]ai.PolicyExtraction](t, resp)
	assert.Len(t, extractions, 2)
}

func TestApproveExtraction_Handler(t *testing.T) {
	ts := setupAITestServer(t)
	orgID := uuid.New()
	e := seedExtraction(t, ts, orgID, "late_fee")

	resp := doAIRequest(t, ts, http.MethodPost,
		fmt.Sprintf("/api/v1/organizations/%s/policy-extractions/%s/approve", orgID, e.ID),
		nil,
	)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	result := decodeAIResponse[ai.PolicyExtraction](t, resp)
	assert.Equal(t, "approved", result.ReviewStatus)
}

// ─── Active Policy Tests ──────────────────────────────────────────────────────

func TestGetActivePolicy_Handler(t *testing.T) {
	ts := setupAITestServer(t)
	orgID := uuid.New()
	seedExtraction(t, ts, orgID, "late_fee")

	resp := doAIRequest(t, ts, http.MethodGet,
		fmt.Sprintf("/api/v1/organizations/%s/policies/late_fee", orgID),
		nil,
	)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	result := decodeAIResponse[ai.PolicyResult](t, resp)
	assert.Equal(t, "pending", result.ReviewStatus)
	assert.NotEmpty(t, result.Config)
}

// ─── Policy Resolution Tests ──────────────────────────────────────────────────

func TestListResolutions_Handler(t *testing.T) {
	ts := setupAITestServer(t)
	orgID := uuid.New()

	seedResolution(t, ts, orgID)
	seedResolution(t, ts, orgID)

	resp := doAIRequest(t, ts, http.MethodGet,
		fmt.Sprintf("/api/v1/organizations/%s/policy-resolutions", orgID),
		nil,
	)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resolutions := decodeAIResponse[[]ai.PolicyResolution](t, resp)
	assert.Len(t, resolutions, 2)
}

func TestDecideResolution_Handler(t *testing.T) {
	ts := setupAITestServer(t)
	orgID := uuid.New()
	r := seedResolution(t, ts, orgID)

	resp := doAIRequest(t, ts, http.MethodPost,
		fmt.Sprintf("/api/v1/organizations/%s/policy-resolutions/%s/decide", orgID, r.ID),
		ai.DecideResolutionRequest{
			Decision: json.RawMessage(`{"action":"approve","note":"Confirmed"}`),
		},
	)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	result := decodeAIResponse[ai.PolicyResolution](t, resp)
	assert.NotNil(t, result.HumanDecision)
	assert.NotNil(t, result.DecidedAt)
}

// ─── AI Config Tests ──────────────────────────────────────────────────────────

func TestGetAIConfig_Handler(t *testing.T) {
	ts := setupAITestServer(t)
	orgID := uuid.New()

	// Seed an org with no AI config so defaults are returned.
	ts.orgRepo.orgs[orgID] = &org.Organization{
		ID:       orgID,
		Name:     "Test HOA",
		Settings: map[string]any{},
	}

	resp := doAIRequest(t, ts, http.MethodGet,
		fmt.Sprintf("/api/v1/organizations/%s/ai/config", orgID),
		nil,
	)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	cfg := decodeAIResponse[ai.AIConfig](t, resp)
	assert.Equal(t, 0.95, cfg.AutoApplyThreshold)
	assert.Equal(t, 0.80, cfg.SuggestThreshold)
	assert.True(t, cfg.EscalateBelowSuggest)
	assert.True(t, cfg.HomeownerQueryAutoRespond)
	assert.True(t, cfg.HomeownerQueryDisclaimer)
	assert.ElementsMatch(t, []string{"lien_filing", "account_suspension", "fine_over_50000", "membership_termination"}, cfg.HighStakesActions)
}

func TestUpdateAIConfig_Handler(t *testing.T) {
	ts := setupAITestServer(t)
	orgID := uuid.New()

	// Seed an org.
	ts.orgRepo.orgs[orgID] = &org.Organization{
		ID:       orgID,
		Name:     "Test HOA",
		Settings: map[string]any{},
	}

	newCfg := ai.AIConfig{
		AutoApplyThreshold:        0.99,
		SuggestThreshold:          0.75,
		EscalateBelowSuggest:      false,
		HighStakesActions:         []string{"lien_filing"},
		HomeownerQueryAutoRespond: false,
		HomeownerQueryDisclaimer:  true,
	}

	resp := doAIRequest(t, ts, http.MethodPut,
		fmt.Sprintf("/api/v1/organizations/%s/ai/config", orgID),
		newCfg,
	)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	cfg := decodeAIResponse[ai.AIConfig](t, resp)
	assert.Equal(t, 0.99, cfg.AutoApplyThreshold)
	assert.Equal(t, 0.75, cfg.SuggestThreshold)
	assert.False(t, cfg.EscalateBelowSuggest)
	assert.False(t, cfg.HomeownerQueryAutoRespond)
	assert.Equal(t, []string{"lien_filing"}, cfg.HighStakesActions)

	// Verify it was persisted — fetch again.
	resp2 := doAIRequest(t, ts, http.MethodGet,
		fmt.Sprintf("/api/v1/organizations/%s/ai/config", orgID),
		nil,
	)
	defer resp2.Body.Close()

	assert.Equal(t, http.StatusOK, resp2.StatusCode)
	cfg2 := decodeAIResponse[ai.AIConfig](t, resp2)
	assert.Equal(t, 0.99, cfg2.AutoApplyThreshold)
}
