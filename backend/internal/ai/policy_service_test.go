package ai_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/ai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Mock PolicyRepository ────────────────────────────────────────────────────

type mockPolicyRepo struct {
	docs        map[uuid.UUID]*ai.GoverningDocument
	extractions map[uuid.UUID]*ai.PolicyExtraction
	resolutions map[uuid.UUID]*ai.PolicyResolution
}

func newMockPolicyRepo() *mockPolicyRepo {
	return &mockPolicyRepo{
		docs:        make(map[uuid.UUID]*ai.GoverningDocument),
		extractions: make(map[uuid.UUID]*ai.PolicyExtraction),
		resolutions: make(map[uuid.UUID]*ai.PolicyResolution),
	}
}

func (r *mockPolicyRepo) CreateGoverningDoc(_ context.Context, doc *ai.GoverningDocument) (*ai.GoverningDocument, error) {
	doc.ID = uuid.New()
	doc.CreatedAt = time.Now()
	cp := *doc
	r.docs[doc.ID] = &cp
	return &cp, nil
}

func (r *mockPolicyRepo) FindGoverningDocByID(_ context.Context, id uuid.UUID) (*ai.GoverningDocument, error) {
	d, ok := r.docs[id]
	if !ok {
		return nil, nil
	}
	cp := *d
	return &cp, nil
}

func (r *mockPolicyRepo) ListGoverningDocsByOrg(_ context.Context, orgID uuid.UUID) ([]ai.GoverningDocument, error) {
	var out []ai.GoverningDocument
	for _, d := range r.docs {
		if d.OrgID == orgID {
			out = append(out, *d)
		}
	}
	return out, nil
}

func (r *mockPolicyRepo) UpdateGoverningDoc(_ context.Context, doc *ai.GoverningDocument) (*ai.GoverningDocument, error) {
	r.docs[doc.ID] = doc
	cp := *doc
	return &cp, nil
}

func (r *mockPolicyRepo) CreateExtraction(_ context.Context, e *ai.PolicyExtraction) (*ai.PolicyExtraction, error) {
	e.ID = uuid.New()
	e.CreatedAt = time.Now()
	cp := *e
	r.extractions[e.ID] = &cp
	return &cp, nil
}

func (r *mockPolicyRepo) FindExtractionByID(_ context.Context, id uuid.UUID) (*ai.PolicyExtraction, error) {
	e, ok := r.extractions[id]
	if !ok {
		return nil, nil
	}
	cp := *e
	return &cp, nil
}

func (r *mockPolicyRepo) ListExtractionsByOrg(_ context.Context, orgID uuid.UUID) ([]ai.PolicyExtraction, error) {
	var out []ai.PolicyExtraction
	for _, e := range r.extractions {
		if e.OrgID == orgID {
			out = append(out, *e)
		}
	}
	return out, nil
}

func (r *mockPolicyRepo) ListActiveExtractionsByOrg(_ context.Context, orgID uuid.UUID) ([]ai.PolicyExtraction, error) {
	var out []ai.PolicyExtraction
	for _, e := range r.extractions {
		if e.OrgID == orgID && e.SupersededBy == nil &&
			(e.ReviewStatus == "approved" || e.ReviewStatus == "pending") {
			out = append(out, *e)
		}
	}
	return out, nil
}

func (r *mockPolicyRepo) FindActiveExtraction(_ context.Context, orgID uuid.UUID, policyKey string) (*ai.PolicyExtraction, error) {
	for _, e := range r.extractions {
		if e.OrgID == orgID && e.PolicyKey == policyKey &&
			e.SupersededBy == nil &&
			(e.ReviewStatus == "approved" || e.ReviewStatus == "pending") {
			cp := *e
			return &cp, nil
		}
	}
	return nil, nil
}

func (r *mockPolicyRepo) UpdateExtraction(_ context.Context, e *ai.PolicyExtraction) (*ai.PolicyExtraction, error) {
	r.extractions[e.ID] = e
	cp := *e
	return &cp, nil
}

func (r *mockPolicyRepo) CreateResolution(_ context.Context, res *ai.PolicyResolution) (*ai.PolicyResolution, error) {
	res.ID = uuid.New()
	res.CreatedAt = time.Now()
	cp := *res
	r.resolutions[res.ID] = &cp
	return &cp, nil
}

func (r *mockPolicyRepo) FindResolutionByID(_ context.Context, id uuid.UUID) (*ai.PolicyResolution, error) {
	res, ok := r.resolutions[id]
	if !ok {
		return nil, nil
	}
	cp := *res
	return &cp, nil
}

func (r *mockPolicyRepo) ListResolutionsByOrg(_ context.Context, orgID uuid.UUID) ([]ai.PolicyResolution, error) {
	var out []ai.PolicyResolution
	for _, res := range r.resolutions {
		if res.OrgID == orgID {
			out = append(out, *res)
		}
	}
	return out, nil
}

func (r *mockPolicyRepo) ListPendingEscalations(_ context.Context, orgID uuid.UUID) ([]ai.PolicyResolution, error) {
	var out []ai.PolicyResolution
	for _, res := range r.resolutions {
		if res.OrgID == orgID && res.ResolutionType == "human_escalated" && res.DecidedAt == nil {
			out = append(out, *res)
		}
	}
	return out, nil
}

func (r *mockPolicyRepo) UpdateResolution(_ context.Context, res *ai.PolicyResolution) (*ai.PolicyResolution, error) {
	r.resolutions[res.ID] = res
	cp := *res
	return &cp, nil
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func newTestPolicyService() *ai.PolicyService {
	return ai.NewPolicyService(newMockPolicyRepo(), slog.New(slog.NewTextHandler(io.Discard, nil)))
}

// ─── Governing Document Tests ─────────────────────────────────────────────────

func TestRegisterGoverningDoc_Success(t *testing.T) {
	svc := newTestPolicyService()
	ctx := context.Background()
	orgID := uuid.New()
	docID := uuid.New()

	doc, err := svc.RegisterGoverningDoc(ctx, orgID, ai.RegisterGoverningDocRequest{
		DocumentID:    docID.String(),
		DocType:       "ccr",
		Title:         "CC&Rs 2024",
		EffectiveDate: "2024-01-01",
	})

	require.NoError(t, err)
	require.NotNil(t, doc)
	assert.Equal(t, orgID, doc.OrgID)
	assert.Equal(t, docID, doc.DocumentID)
	assert.Equal(t, "ccr", doc.DocType)
	assert.Equal(t, "CC&Rs 2024", doc.Title)
	assert.Equal(t, "pending", doc.IndexingStatus)
}

func TestRegisterGoverningDoc_InvalidDocumentID(t *testing.T) {
	svc := newTestPolicyService()
	ctx := context.Background()

	_, err := svc.RegisterGoverningDoc(ctx, uuid.New(), ai.RegisterGoverningDocRequest{
		DocumentID:    "not-a-uuid",
		DocType:       "ccr",
		Title:         "CC&Rs",
		EffectiveDate: "2024-01-01",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "document_id")
}

func TestRegisterGoverningDoc_InvalidEffectiveDate(t *testing.T) {
	svc := newTestPolicyService()
	ctx := context.Background()

	_, err := svc.RegisterGoverningDoc(ctx, uuid.New(), ai.RegisterGoverningDocRequest{
		DocumentID:    uuid.New().String(),
		DocType:       "ccr",
		Title:         "CC&Rs",
		EffectiveDate: "not-a-date",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "effective_date")
}

func TestGetGoverningDoc_NotFound(t *testing.T) {
	svc := newTestPolicyService()
	ctx := context.Background()

	_, err := svc.GetGoverningDoc(ctx, uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestReindexGoverningDoc_ResetsToPending(t *testing.T) {
	svc := newTestPolicyService()
	ctx := context.Background()
	orgID := uuid.New()

	created, err := svc.RegisterGoverningDoc(ctx, orgID, ai.RegisterGoverningDocRequest{
		DocumentID:    uuid.New().String(),
		DocType:       "ccr",
		Title:         "CC&Rs",
		EffectiveDate: "2024-01-01",
	})
	require.NoError(t, err)

	// Simulate the doc being indexed
	now := time.Now()
	count := 10
	created.IndexingStatus = "indexed"
	created.IndexedAt = &now
	created.ChunkCount = &count

	reindexed, err := svc.ReindexGoverningDoc(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "pending", reindexed.IndexingStatus)
	assert.Nil(t, reindexed.IndexedAt)
	assert.Nil(t, reindexed.ChunkCount)
}

// ─── Extraction Tests ─────────────────────────────────────────────────────────

func TestApproveExtraction_SetsStatus(t *testing.T) {
	repo := newMockPolicyRepo()
	svc := ai.NewPolicyService(repo, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx := context.Background()
	orgID := uuid.New()
	reviewerID := uuid.New()

	// Seed an extraction directly in the mock
	e, _ := repo.CreateExtraction(ctx, &ai.PolicyExtraction{
		OrgID: orgID, Domain: "financial", PolicyKey: "late_fee",
		Config: json.RawMessage(`{}`), Confidence: 0.9,
		SourceDocID: uuid.New(), SourceText: "...",
		ReviewStatus: "pending", ModelVersion: "gpt-4o",
		EffectiveAt: time.Now(),
	})

	updated, err := svc.ApproveExtraction(ctx, e.ID, reviewerID)
	require.NoError(t, err)
	assert.Equal(t, "approved", updated.ReviewStatus)
	assert.Equal(t, &reviewerID, updated.ReviewedBy)
	assert.NotNil(t, updated.ReviewedAt)
}

func TestRejectExtraction_SetsStatus(t *testing.T) {
	repo := newMockPolicyRepo()
	svc := ai.NewPolicyService(repo, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx := context.Background()
	orgID := uuid.New()
	reviewerID := uuid.New()

	e, _ := repo.CreateExtraction(ctx, &ai.PolicyExtraction{
		OrgID: orgID, Domain: "governance", PolicyKey: "quorum_rule",
		Config: json.RawMessage(`{}`), Confidence: 0.7,
		SourceDocID: uuid.New(), SourceText: "...",
		ReviewStatus: "pending", ModelVersion: "gpt-4o",
		EffectiveAt: time.Now(),
	})

	updated, err := svc.RejectExtraction(ctx, e.ID, reviewerID)
	require.NoError(t, err)
	assert.Equal(t, "rejected", updated.ReviewStatus)
	assert.Equal(t, &reviewerID, updated.ReviewedBy)
	assert.NotNil(t, updated.ReviewedAt)
}

func TestModifyExtraction_SetsOverrideAndStatus(t *testing.T) {
	repo := newMockPolicyRepo()
	svc := ai.NewPolicyService(repo, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx := context.Background()
	orgID := uuid.New()
	reviewerID := uuid.New()

	e, _ := repo.CreateExtraction(ctx, &ai.PolicyExtraction{
		OrgID: orgID, Domain: "financial", PolicyKey: "late_fee",
		Config: json.RawMessage(`{"rate": 10}`), Confidence: 0.9,
		SourceDocID: uuid.New(), SourceText: "...",
		ReviewStatus: "pending", ModelVersion: "gpt-4o",
		EffectiveAt: time.Now(),
	})

	override := json.RawMessage(`{"rate": 8}`)
	updated, err := svc.ModifyExtraction(ctx, e.ID, override, reviewerID)
	require.NoError(t, err)
	assert.Equal(t, "modified", updated.ReviewStatus)
	assert.JSONEq(t, `{"rate": 8}`, string(updated.HumanOverride))
}

func TestListActivePolicies_ReturnsOnlyActiveExtractions(t *testing.T) {
	repo := newMockPolicyRepo()
	svc := ai.NewPolicyService(repo, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx := context.Background()
	orgID := uuid.New()
	supersededID := uuid.New()

	// active — approved, not superseded
	repo.CreateExtraction(ctx, &ai.PolicyExtraction{
		OrgID: orgID, Domain: "financial", PolicyKey: "late_fee",
		Config: json.RawMessage(`{}`), Confidence: 0.9,
		SourceDocID: uuid.New(), SourceText: "...",
		ReviewStatus: "approved", ModelVersion: "gpt-4o", EffectiveAt: time.Now(),
	})
	// active — pending, not superseded
	repo.CreateExtraction(ctx, &ai.PolicyExtraction{
		OrgID: orgID, Domain: "governance", PolicyKey: "quorum_rule",
		Config: json.RawMessage(`{}`), Confidence: 0.7,
		SourceDocID: uuid.New(), SourceText: "...",
		ReviewStatus: "pending", ModelVersion: "gpt-4o", EffectiveAt: time.Now(),
	})
	// inactive — rejected
	repo.CreateExtraction(ctx, &ai.PolicyExtraction{
		OrgID: orgID, Domain: "financial", PolicyKey: "fee_cap",
		Config: json.RawMessage(`{}`), Confidence: 0.6,
		SourceDocID: uuid.New(), SourceText: "...",
		ReviewStatus: "rejected", ModelVersion: "gpt-4o", EffectiveAt: time.Now(),
	})
	// inactive — superseded
	repo.CreateExtraction(ctx, &ai.PolicyExtraction{
		OrgID: orgID, Domain: "financial", PolicyKey: "old_policy",
		Config: json.RawMessage(`{}`), Confidence: 0.8,
		SourceDocID: uuid.New(), SourceText: "...",
		ReviewStatus: "approved", ModelVersion: "gpt-4o", EffectiveAt: time.Now(),
		SupersededBy: &supersededID,
	})

	results, err := svc.ListActivePolicies(ctx, orgID)
	require.NoError(t, err)
	assert.Len(t, results, 2)
	keys := make([]string, len(results))
	for i, r := range results {
		keys[i] = r.PolicyKey
	}
	assert.ElementsMatch(t, []string{"late_fee", "quorum_rule"}, keys)
}

func TestGetActivePolicy_ReturnsFromExtraction(t *testing.T) {
	repo := newMockPolicyRepo()
	svc := ai.NewPolicyService(repo, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx := context.Background()
	orgID := uuid.New()

	section := "Section 4.2"
	repo.CreateExtraction(ctx, &ai.PolicyExtraction{
		OrgID: orgID, Domain: "financial", PolicyKey: "late_fee",
		Config: json.RawMessage(`{"rate": 10}`), Confidence: 0.95,
		SourceDocID: uuid.New(), SourceText: "...",
		SourceSection: &section,
		ReviewStatus:  "approved", ModelVersion: "gpt-4o",
		EffectiveAt: time.Now(),
	})

	result, err := svc.GetActivePolicy(ctx, orgID, "late_fee")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.JSONEq(t, `{"rate": 10}`, string(result.Config))
	assert.InDelta(t, 0.95, result.Confidence, 0.01)
	assert.Equal(t, "approved", result.ReviewStatus)
	assert.Equal(t, "Section 4.2", result.SourceSection)
	assert.False(t, result.RequiresReview)
}

func TestGetActivePolicy_HumanOverrideTakesPrecedence(t *testing.T) {
	repo := newMockPolicyRepo()
	svc := ai.NewPolicyService(repo, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx := context.Background()
	orgID := uuid.New()

	repo.CreateExtraction(ctx, &ai.PolicyExtraction{
		OrgID: orgID, Domain: "financial", PolicyKey: "late_fee",
		Config: json.RawMessage(`{"rate": 10}`), Confidence: 0.9,
		HumanOverride: json.RawMessage(`{"rate": 5}`),
		SourceDocID: uuid.New(), SourceText: "...",
		ReviewStatus: "approved", ModelVersion: "gpt-4o",
		EffectiveAt: time.Now(),
	})

	result, err := svc.GetActivePolicy(ctx, orgID, "late_fee")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.JSONEq(t, `{"rate": 5}`, string(result.Config), "human override should take precedence over AI config")
}

func TestGetActivePolicy_NotFound_ReturnsNil(t *testing.T) {
	svc := newTestPolicyService()
	ctx := context.Background()

	result, err := svc.GetActivePolicy(ctx, uuid.New(), "nonexistent_policy")
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestGetActivePolicy_PendingExtraction_RequiresReview(t *testing.T) {
	repo := newMockPolicyRepo()
	svc := ai.NewPolicyService(repo, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx := context.Background()
	orgID := uuid.New()

	repo.CreateExtraction(ctx, &ai.PolicyExtraction{
		OrgID: orgID, Domain: "financial", PolicyKey: "fee_policy",
		Config: json.RawMessage(`{"rate": 10}`), Confidence: 0.7,
		SourceDocID: uuid.New(), SourceText: "...",
		ReviewStatus: "pending", ModelVersion: "gpt-4o",
		EffectiveAt: time.Now(),
	})

	result, err := svc.GetActivePolicy(ctx, orgID, "fee_policy")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.RequiresReview)
}

// ─── Resolution Tests ─────────────────────────────────────────────────────────

func TestDecideResolution_SetsHumanDecision(t *testing.T) {
	repo := newMockPolicyRepo()
	svc := ai.NewPolicyService(repo, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx := context.Background()
	orgID := uuid.New()
	decidedBy := uuid.New()

	res, _ := repo.CreateResolution(ctx, &ai.PolicyResolution{
		OrgID: orgID, Query: "What is pet policy?",
		PolicyKeys: []string{"pet_policy"},
		Resolution: json.RawMessage(`{}`), Reasoning: "Unclear.",
		SourcePassages: json.RawMessage(`[]`), Confidence: 0.4,
		ResolutionType: "human_escalated", RequestingModule: "gov",
		RequestingContext: json.RawMessage(`{}`),
	})

	decision := json.RawMessage(`{"allow": true, "max_pets": 2}`)
	updated, err := svc.DecideResolution(ctx, res.ID, decision, decidedBy)
	require.NoError(t, err)
	assert.JSONEq(t, `{"allow": true, "max_pets": 2}`, string(updated.HumanDecision))
	assert.Equal(t, &decidedBy, updated.DecidedBy)
	assert.NotNil(t, updated.DecidedAt)
}

func TestGetResolution_NotFound(t *testing.T) {
	svc := newTestPolicyService()
	ctx := context.Background()

	_, err := svc.GetResolution(ctx, uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// ─── PostgresPolicyResolver Tests ────────────────────────────────────────────

func TestPostgresPolicyResolver_GetPolicy_DelegatesToService(t *testing.T) {
	repo := newMockPolicyRepo()
	svc := ai.NewPolicyService(repo, slog.New(slog.NewTextHandler(io.Discard, nil)))
	resolver := ai.NewPostgresPolicyResolver(svc)
	ctx := context.Background()
	orgID := uuid.New()

	repo.CreateExtraction(ctx, &ai.PolicyExtraction{
		OrgID: orgID, Domain: "financial", PolicyKey: "late_fee",
		Config: json.RawMessage(`{"rate": 10}`), Confidence: 0.9,
		SourceDocID: uuid.New(), SourceText: "...",
		ReviewStatus: "approved", ModelVersion: "gpt-4o",
		EffectiveAt: time.Now(),
	})

	result, err := resolver.GetPolicy(ctx, orgID, "late_fee")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.JSONEq(t, `{"rate": 10}`, string(result.Config))
}

func TestPostgresPolicyResolver_QueryPolicy_ReturnsNil(t *testing.T) {
	svc := newTestPolicyService()
	resolver := ai.NewPostgresPolicyResolver(svc)
	ctx := context.Background()

	result, err := resolver.QueryPolicy(ctx, uuid.New(), "What is the late fee?", ai.QueryContext{
		Module: "fin",
	})
	require.NoError(t, err)
	assert.Nil(t, result, "QueryPolicy is a placeholder and should return nil")
}
