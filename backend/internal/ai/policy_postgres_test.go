//go:build integration

package ai_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/quorant/quorant/internal/ai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Test DB Setup ────────────────────────────────────────────────────────────

func setupPolicyTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, "postgres://quorant:quorant@localhost:5432/quorant_dev?sslmode=disable")
	require.NoError(t, err, "connecting to test database")

	t.Cleanup(func() {
		cleanCtx := context.Background()
		pool.Exec(cleanCtx, "DELETE FROM policy_resolutions")
		pool.Exec(cleanCtx, "DELETE FROM policy_extractions")
		pool.Exec(cleanCtx, "DELETE FROM governing_documents")
		pool.Exec(cleanCtx, "DELETE FROM documents")
		pool.Exec(cleanCtx, "DELETE FROM memberships")
		pool.Exec(cleanCtx, "DELETE FROM units")
		pool.Exec(cleanCtx, "DELETE FROM organizations WHERE parent_id IS NOT NULL")
		pool.Exec(cleanCtx, "DELETE FROM organizations")
		pool.Exec(cleanCtx, "DELETE FROM users")
		pool.Close()
	})

	return pool
}

// policyTestFixture holds shared test resources.
type policyTestFixture struct {
	pool   *pgxpool.Pool
	orgID  uuid.UUID
	userID uuid.UUID
	docID  uuid.UUID // ID of a document in the documents table
}

// setupPolicyFixture creates the base org, user, and document needed for policy tests.
func setupPolicyFixture(t *testing.T) policyTestFixture {
	t.Helper()
	pool := setupPolicyTestDB(t)
	ctx := context.Background()

	var orgID uuid.UUID
	err := pool.QueryRow(ctx,
		`INSERT INTO organizations (type, name, slug, path, settings)
		 VALUES ('hoa', $1, $2, $3, '{}')
		 RETURNING id`,
		"Test HOA "+uuid.New().String(),
		"test-hoa-"+uuid.New().String(),
		"test_hoa_"+uuid.New().String(),
	).Scan(&orgID)
	require.NoError(t, err, "create test org")

	var userID uuid.UUID
	err = pool.QueryRow(ctx,
		`INSERT INTO users (idp_user_id, email, display_name)
		 VALUES ($1, $2, 'Test User')
		 RETURNING id`,
		"test-idp-"+uuid.New().String(),
		"test-"+uuid.New().String()+"@example.com",
	).Scan(&userID)
	require.NoError(t, err, "create test user")

	// governing_documents has FK to documents(id), so create a document first.
	var docID uuid.UUID
	err = pool.QueryRow(ctx,
		`INSERT INTO documents (
			org_id, uploaded_by, title, file_name, content_type,
			size_bytes, storage_key, visibility, version_number, is_current, metadata
		) VALUES ($1, $2, 'CC&Rs 2024', 'ccrs.pdf', 'application/pdf', 1024, $3, 'members', 1, TRUE, '{}')
		RETURNING id`,
		orgID,
		userID,
		orgID.String()+"/ccrs.pdf",
	).Scan(&docID)
	require.NoError(t, err, "create test document")

	return policyTestFixture{
		pool:   pool,
		orgID:  orgID,
		userID: userID,
		docID:  docID,
	}
}

// ─── Governing Documents ──────────────────────────────────────────────────────

func TestCreateGoverningDoc_AndFindByID(t *testing.T) {
	f := setupPolicyFixture(t)
	repo := ai.NewPostgresPolicyRepository(f.pool)
	ctx := context.Background()

	input := &ai.GoverningDocument{
		OrgID:          f.orgID,
		DocumentID:     f.docID,
		DocType:        "ccr",
		Title:          "CC&Rs 2024",
		EffectiveDate:  time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		IndexingStatus: "pending",
	}

	created, err := repo.CreateGoverningDoc(ctx, input)
	require.NoError(t, err)
	require.NotNil(t, created)
	assert.NotEqual(t, uuid.Nil, created.ID)
	assert.Equal(t, f.orgID, created.OrgID)
	assert.Equal(t, f.docID, created.DocumentID)
	assert.Equal(t, "ccr", created.DocType)
	assert.Equal(t, "CC&Rs 2024", created.Title)
	assert.Equal(t, "pending", created.IndexingStatus)
	assert.Nil(t, created.SupersedesID)
	assert.Nil(t, created.IndexedAt)
	assert.Nil(t, created.ChunkCount)
	assert.NotZero(t, created.CreatedAt)

	// FindByID
	found, err := repo.FindGoverningDocByID(ctx, created.ID)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, created.ID, found.ID)
	assert.Equal(t, "CC&Rs 2024", found.Title)
}

func TestFindGoverningDocByID_NotFound(t *testing.T) {
	f := setupPolicyFixture(t)
	repo := ai.NewPostgresPolicyRepository(f.pool)
	ctx := context.Background()

	found, err := repo.FindGoverningDocByID(ctx, uuid.New())
	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestListGoverningDocsByOrg(t *testing.T) {
	f := setupPolicyFixture(t)
	repo := ai.NewPostgresPolicyRepository(f.pool)
	ctx := context.Background()

	// Create a second document for the second governing doc
	var docID2 uuid.UUID
	err := f.pool.QueryRow(ctx,
		`INSERT INTO documents (
			org_id, uploaded_by, title, file_name, content_type,
			size_bytes, storage_key, visibility, version_number, is_current, metadata
		) VALUES ($1, $2, 'Bylaws 2023', 'bylaws.pdf', 'application/pdf', 512, $3, 'members', 1, TRUE, '{}')
		RETURNING id`,
		f.orgID,
		f.userID,
		f.orgID.String()+"/bylaws.pdf",
	).Scan(&docID2)
	require.NoError(t, err)

	doc1 := &ai.GoverningDocument{
		OrgID: f.orgID, DocumentID: f.docID,
		DocType: "ccr", Title: "CC&Rs", EffectiveDate: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		IndexingStatus: "pending",
	}
	doc2 := &ai.GoverningDocument{
		OrgID: f.orgID, DocumentID: docID2,
		DocType: "bylaws", Title: "Bylaws", EffectiveDate: time.Date(2023, 6, 1, 0, 0, 0, 0, time.UTC),
		IndexingStatus: "indexed",
	}

	_, err = repo.CreateGoverningDoc(ctx, doc1)
	require.NoError(t, err)
	_, err = repo.CreateGoverningDoc(ctx, doc2)
	require.NoError(t, err)

	list, err := repo.ListGoverningDocsByOrg(ctx, f.orgID)
	require.NoError(t, err)
	assert.Len(t, list, 2)

	// Should be sorted by effective_date DESC: doc1 (2024) first
	assert.Equal(t, "CC&Rs", list[0].Title)
	assert.Equal(t, "Bylaws", list[1].Title)
}

func TestUpdateGoverningDoc_ChangesIndexingStatus(t *testing.T) {
	f := setupPolicyFixture(t)
	repo := ai.NewPostgresPolicyRepository(f.pool)
	ctx := context.Background()

	created, err := repo.CreateGoverningDoc(ctx, &ai.GoverningDocument{
		OrgID:          f.orgID,
		DocumentID:     f.docID,
		DocType:        "ccr",
		Title:          "CC&Rs",
		EffectiveDate:  time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		IndexingStatus: "pending",
	})
	require.NoError(t, err)

	now := time.Now().UTC().Truncate(time.Second)
	count := 42
	created.IndexingStatus = "indexed"
	created.IndexedAt = &now
	created.ChunkCount = &count

	updated, err := repo.UpdateGoverningDoc(ctx, created)
	require.NoError(t, err)
	assert.Equal(t, "indexed", updated.IndexingStatus)
	assert.NotNil(t, updated.IndexedAt)
	assert.Equal(t, 42, *updated.ChunkCount)
}

// ─── Policy Extractions ───────────────────────────────────────────────────────

func TestCreateExtraction_AndFindActiveExtraction(t *testing.T) {
	f := setupPolicyFixture(t)
	repo := ai.NewPostgresPolicyRepository(f.pool)
	ctx := context.Background()

	govDoc, err := repo.CreateGoverningDoc(ctx, &ai.GoverningDocument{
		OrgID: f.orgID, DocumentID: f.docID,
		DocType: "ccr", Title: "CC&Rs",
		EffectiveDate:  time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		IndexingStatus: "indexed",
	})
	require.NoError(t, err)

	cfg := json.RawMessage(`{"max_late_fee_pct": 10}`)
	extraction := &ai.PolicyExtraction{
		OrgID:        f.orgID,
		Domain:       "financial",
		PolicyKey:    "late_fee_policy",
		Config:       cfg,
		Confidence:   0.95,
		SourceDocID:  govDoc.ID,
		SourceText:   "Late fees shall not exceed 10% of outstanding balance.",
		ReviewStatus: "pending",
		ModelVersion: "gpt-4o",
		EffectiveAt:  time.Now().UTC(),
	}

	created, err := repo.CreateExtraction(ctx, extraction)
	require.NoError(t, err)
	require.NotNil(t, created)
	assert.NotEqual(t, uuid.Nil, created.ID)
	assert.Equal(t, "late_fee_policy", created.PolicyKey)
	assert.Equal(t, "financial", created.Domain)
	assert.InDelta(t, 0.95, created.Confidence, 0.01)
	assert.Equal(t, "pending", created.ReviewStatus)
	assert.JSONEq(t, `{"max_late_fee_pct": 10}`, string(created.Config))

	// FindActiveExtraction — should find since review_status = pending and not superseded
	active, err := repo.FindActiveExtraction(ctx, f.orgID, "late_fee_policy")
	require.NoError(t, err)
	require.NotNil(t, active)
	assert.Equal(t, created.ID, active.ID)
}

func TestFindActiveExtraction_NotFound(t *testing.T) {
	f := setupPolicyFixture(t)
	repo := ai.NewPostgresPolicyRepository(f.pool)
	ctx := context.Background()

	active, err := repo.FindActiveExtraction(ctx, f.orgID, "nonexistent_policy")
	require.NoError(t, err)
	assert.Nil(t, active)
}

func TestFindActiveExtraction_RejectedNotReturned(t *testing.T) {
	f := setupPolicyFixture(t)
	repo := ai.NewPostgresPolicyRepository(f.pool)
	ctx := context.Background()

	govDoc, err := repo.CreateGoverningDoc(ctx, &ai.GoverningDocument{
		OrgID: f.orgID, DocumentID: f.docID,
		DocType: "ccr", Title: "CC&Rs",
		EffectiveDate:  time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		IndexingStatus: "indexed",
	})
	require.NoError(t, err)

	extraction := &ai.PolicyExtraction{
		OrgID: f.orgID, Domain: "financial", PolicyKey: "fee_cap",
		Config: json.RawMessage(`{}`), Confidence: 0.8,
		SourceDocID: govDoc.ID, SourceText: "...",
		ReviewStatus: "rejected", ModelVersion: "gpt-4o",
		EffectiveAt: time.Now().UTC(),
	}
	_, err = repo.CreateExtraction(ctx, extraction)
	require.NoError(t, err)

	active, err := repo.FindActiveExtraction(ctx, f.orgID, "fee_cap")
	require.NoError(t, err)
	assert.Nil(t, active, "rejected extractions should not be returned as active")
}

// ─── Policy Resolutions ───────────────────────────────────────────────────────

func TestCreateResolution_AndListByOrg(t *testing.T) {
	f := setupPolicyFixture(t)
	repo := ai.NewPostgresPolicyRepository(f.pool)
	ctx := context.Background()

	res := &ai.PolicyResolution{
		OrgID:             f.orgID,
		Query:             "What is the late fee policy?",
		PolicyKeys:        []string{"late_fee_policy"},
		Resolution:        json.RawMessage(`{"answer": "10% max"}`),
		Reasoning:         "Based on section 4.2 of CC&Rs.",
		SourcePassages:    json.RawMessage(`[]`),
		Confidence:        0.9,
		ResolutionType:    "ai_inference",
		RequestingModule:  "fin",
		RequestingContext: json.RawMessage(`{"resource_type": "assessment"}`),
		FedBack:           false,
	}

	created, err := repo.CreateResolution(ctx, res)
	require.NoError(t, err)
	require.NotNil(t, created)
	assert.NotEqual(t, uuid.Nil, created.ID)
	assert.Equal(t, f.orgID, created.OrgID)
	assert.Equal(t, "ai_inference", created.ResolutionType)
	assert.Equal(t, []string{"late_fee_policy"}, created.PolicyKeys)
	assert.JSONEq(t, `{"answer": "10% max"}`, string(created.Resolution))
	assert.False(t, created.FedBack)

	list, err := repo.ListResolutionsByOrg(ctx, f.orgID)
	require.NoError(t, err)
	assert.Len(t, list, 1)
	assert.Equal(t, created.ID, list[0].ID)
}

func TestListPendingEscalations(t *testing.T) {
	f := setupPolicyFixture(t)
	repo := ai.NewPostgresPolicyRepository(f.pool)
	ctx := context.Background()

	// Create one human_escalated (pending decision) and one ai_inference resolution
	escalated := &ai.PolicyResolution{
		OrgID: f.orgID, Query: "What is the pet policy?",
		PolicyKeys: []string{"pet_policy"},
		Resolution: json.RawMessage(`{}`), Reasoning: "Needs human review.",
		SourcePassages: json.RawMessage(`[]`), Confidence: 0.4,
		ResolutionType: "human_escalated", RequestingModule: "gov",
		RequestingContext: json.RawMessage(`{}`),
	}
	regular := &ai.PolicyResolution{
		OrgID: f.orgID, Query: "What is the late fee?",
		PolicyKeys: []string{"late_fee"},
		Resolution: json.RawMessage(`{"answer":"10%"}`), Reasoning: "Clear from docs.",
		SourcePassages: json.RawMessage(`[]`), Confidence: 0.9,
		ResolutionType: "ai_inference", RequestingModule: "fin",
		RequestingContext: json.RawMessage(`{}`),
	}

	createdEscalated, err := repo.CreateResolution(ctx, escalated)
	require.NoError(t, err)
	_, err = repo.CreateResolution(ctx, regular)
	require.NoError(t, err)

	pending, err := repo.ListPendingEscalations(ctx, f.orgID)
	require.NoError(t, err)
	require.Len(t, pending, 1)
	assert.Equal(t, createdEscalated.ID, pending[0].ID)
	assert.Equal(t, "human_escalated", pending[0].ResolutionType)
	assert.Nil(t, pending[0].DecidedAt)

	// After deciding, it should no longer appear
	now := time.Now().UTC()
	createdEscalated.DecidedAt = &now
	createdEscalated.HumanDecision = json.RawMessage(`{"decision":"allow"}`)
	createdEscalated.DecidedBy = &f.userID
	_, err = repo.UpdateResolution(ctx, createdEscalated)
	require.NoError(t, err)

	pending2, err := repo.ListPendingEscalations(ctx, f.orgID)
	require.NoError(t, err)
	assert.Empty(t, pending2)
}
