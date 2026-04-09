//go:build integration

package ai_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/quorant/quorant/internal/ai"
	"github.com/quorant/quorant/internal/org"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupAITestDB connects to the Docker postgres and cleans up ai data after each test.
func setupAITestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, "postgres://quorant:quorant@localhost:5432/quorant_dev?sslmode=disable")
	require.NoError(t, err, "connecting to test database")

	t.Cleanup(func() {
		cleanCtx := context.Background()
		pool.Exec(cleanCtx, "DELETE FROM context_chunks")
		pool.Close()
	})

	return pool
}

// setupOrg creates a real org in the database and registers cleanup.
func setupOrg(t *testing.T, pool *pgxpool.Pool, orgType string) *org.Organization {
	t.Helper()
	ctx := context.Background()
	orgRepo := org.NewPostgresOrgRepository(pool)

	created, err := orgRepo.Create(ctx, &org.Organization{
		Type:     orgType,
		Name:     "AI Test Org " + uuid.New().String()[:8],
		Settings: map[string]any{},
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		pool.Exec(context.Background(), "DELETE FROM organizations WHERE id = $1", created.ID)
	})

	return created
}

// makeEmbedding creates a 1536-dim float32 slice with specific non-zero values at given indices.
func makeEmbedding(nonZeroIndices map[int]float32) []float32 {
	v := make([]float32, 1536)
	for i, val := range nonZeroIndices {
		if i < 1536 {
			v[i] = val
		}
	}
	return v
}

// zeroEmbedding returns a zero 1536-dim vector.
func zeroEmbedding() []float32 {
	return make([]float32, 1536)
}

// ─── Create ──────────────────────────────────────────────────────────────────

func TestCreate_ChunkWithEmbedding(t *testing.T) {
	pool := setupAITestDB(t)
	repo := ai.NewPostgresContextChunkRepository(pool)
	ctx := context.Background()

	o := setupOrg(t, pool, "hoa")

	chunk := &ai.ContextChunk{
		Scope:      "org",
		OrgID:      &o.ID,
		SourceType: "governing_document",
		SourceID:   uuid.New(),
		ChunkIndex: 0,
		Content:    "No pets over 25 lbs are permitted.",
		Embedding:  makeEmbedding(map[int]float32{0: 1.0, 10: 0.5}),
		TokenCount: 8,
		Metadata:   map[string]any{"section": "pet_policy"},
	}

	got, err := repo.Create(ctx, chunk)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.NotEqual(t, uuid.Nil, got.ID)
	assert.Equal(t, "org", got.Scope)
	assert.Equal(t, o.ID, *got.OrgID)
	assert.Equal(t, chunk.Content, got.Content)
	assert.Equal(t, len(chunk.Embedding), len(got.Embedding))
	assert.Equal(t, float32(1.0), got.Embedding[0])
	assert.False(t, got.CreatedAt.IsZero())
}

// ─── CreateBatch ─────────────────────────────────────────────────────────────

func TestCreateBatch_InsertsMultipleChunks(t *testing.T) {
	pool := setupAITestDB(t)
	repo := ai.NewPostgresContextChunkRepository(pool)
	ctx := context.Background()

	o := setupOrg(t, pool, "hoa")
	sourceID := uuid.New()

	chunks := []*ai.ContextChunk{
		{
			Scope:      "org",
			OrgID:      &o.ID,
			SourceType: "governing_document",
			SourceID:   sourceID,
			ChunkIndex: 0,
			Content:    "Article I: Name of Association.",
			Embedding:  makeEmbedding(map[int]float32{1: 0.9}),
			TokenCount: 5,
			Metadata:   map[string]any{},
		},
		{
			Scope:      "org",
			OrgID:      &o.ID,
			SourceType: "governing_document",
			SourceID:   sourceID,
			ChunkIndex: 1,
			Content:    "Article II: Purpose.",
			Embedding:  makeEmbedding(map[int]float32{2: 0.8}),
			TokenCount: 3,
			Metadata:   map[string]any{},
		},
	}

	err := repo.CreateBatch(ctx, chunks)
	require.NoError(t, err)

	// Verify both rows were inserted.
	var count int
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM context_chunks WHERE source_id = $1", sourceID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}

func TestCreateBatch_EmptySliceIsNoOp(t *testing.T) {
	pool := setupAITestDB(t)
	repo := ai.NewPostgresContextChunkRepository(pool)
	ctx := context.Background()

	err := repo.CreateBatch(ctx, []*ai.ContextChunk{})
	assert.NoError(t, err)
}

// ─── DeleteBySource ───────────────────────────────────────────────────────────

func TestDeleteBySource_RemovesAllChunksForSource(t *testing.T) {
	pool := setupAITestDB(t)
	repo := ai.NewPostgresContextChunkRepository(pool)
	ctx := context.Background()

	o := setupOrg(t, pool, "hoa")
	sourceID := uuid.New()
	otherSourceID := uuid.New()

	// Insert chunks for two different sources.
	chunks := []*ai.ContextChunk{
		{
			Scope:      "org",
			OrgID:      &o.ID,
			SourceType: "governing_document",
			SourceID:   sourceID,
			ChunkIndex: 0,
			Content:    "Content A1",
			Embedding:  zeroEmbedding(),
			TokenCount: 2,
			Metadata:   map[string]any{},
		},
		{
			Scope:      "org",
			OrgID:      &o.ID,
			SourceType: "governing_document",
			SourceID:   sourceID,
			ChunkIndex: 1,
			Content:    "Content A2",
			Embedding:  zeroEmbedding(),
			TokenCount: 2,
			Metadata:   map[string]any{},
		},
		{
			Scope:      "org",
			OrgID:      &o.ID,
			SourceType: "governing_document",
			SourceID:   otherSourceID,
			ChunkIndex: 0,
			Content:    "Content B1",
			Embedding:  zeroEmbedding(),
			TokenCount: 2,
			Metadata:   map[string]any{},
		},
	}
	require.NoError(t, repo.CreateBatch(ctx, chunks))

	err := repo.DeleteBySource(ctx, sourceID)
	require.NoError(t, err)

	// The target source should have no chunks.
	var deletedCount int
	pool.QueryRow(ctx, "SELECT COUNT(*) FROM context_chunks WHERE source_id = $1", sourceID).Scan(&deletedCount)
	assert.Equal(t, 0, deletedCount, "all chunks for the deleted source should be gone")

	// The other source should be untouched.
	var remainingCount int
	pool.QueryRow(ctx, "SELECT COUNT(*) FROM context_chunks WHERE source_id = $1", otherSourceID).Scan(&remainingCount)
	assert.Equal(t, 1, remainingCount, "chunks for other source should remain")
}

// ─── SimilaritySearch ────────────────────────────────────────────────────────

func TestSimilaritySearch_ReturnsResultsOrderedByScore(t *testing.T) {
	pool := setupAITestDB(t)
	repo := ai.NewPostgresContextChunkRepository(pool)
	ctx := context.Background()

	o := setupOrg(t, pool, "hoa")
	sourceID := uuid.New()

	// Chunk 0: embedding aligned with query (high score)
	// Chunk 1: orthogonal to query (low score)
	queryEmbed := makeEmbedding(map[int]float32{0: 1.0})
	highScoreEmbed := makeEmbedding(map[int]float32{0: 1.0}) // cosine similarity ≈ 1.0
	lowScoreEmbed := makeEmbedding(map[int]float32{1: 1.0})  // orthogonal, cosine similarity = 0.0

	chunks := []*ai.ContextChunk{
		{
			Scope:      "org",
			OrgID:      &o.ID,
			SourceType: "governing_document",
			SourceID:   sourceID,
			ChunkIndex: 0,
			Content:    "High relevance chunk",
			Embedding:  highScoreEmbed,
			TokenCount: 3,
			Metadata:   map[string]any{},
		},
		{
			Scope:      "org",
			OrgID:      &o.ID,
			SourceType: "governing_document",
			SourceID:   sourceID,
			ChunkIndex: 1,
			Content:    "Low relevance chunk",
			Embedding:  lowScoreEmbed,
			TokenCount: 3,
			Metadata:   map[string]any{},
		},
	}
	require.NoError(t, repo.CreateBatch(ctx, chunks))

	results, err := repo.SimilaritySearch(ctx, queryEmbed, o.ID, nil, nil, ai.ContextFilters{}, 10)

	require.NoError(t, err)
	require.Len(t, results, 2)
	assert.Greater(t, results[0].Score, results[1].Score, "results should be ordered by descending score")
	assert.Equal(t, "High relevance chunk", results[0].Content)
}

func TestSimilaritySearch_ScopeFiltering(t *testing.T) {
	pool := setupAITestDB(t)
	repo := ai.NewPostgresContextChunkRepository(pool)
	ctx := context.Background()

	hoaOrg := setupOrg(t, pool, "hoa")
	firmOrg := setupOrg(t, pool, "firm")
	otherHOA := setupOrg(t, pool, "hoa")

	jurisdiction := "CA"
	sourceID := uuid.New()

	chunks := []*ai.ContextChunk{
		// Belongs to the searching HOA — should appear.
		{
			Scope:      "org",
			OrgID:      &hoaOrg.ID,
			SourceType: "governing_document",
			SourceID:   sourceID,
			ChunkIndex: 0,
			Content:    "Org-scoped chunk for the searching HOA",
			Embedding:  zeroEmbedding(),
			TokenCount: 5,
			Metadata:   map[string]any{},
		},
		// Belongs to the managing firm — should appear when firmOrgID is passed.
		{
			Scope:      "firm",
			OrgID:      &firmOrg.ID,
			SourceType: "governing_document",
			SourceID:   uuid.New(),
			ChunkIndex: 0,
			Content:    "Firm-scoped chunk",
			Embedding:  zeroEmbedding(),
			TokenCount: 3,
			Metadata:   map[string]any{},
		},
		// Jurisdiction-scoped — should appear when jurisdiction is passed.
		{
			Scope:        "jurisdiction",
			Jurisdiction: &jurisdiction,
			SourceType:   "document",
			SourceID:     uuid.New(),
			ChunkIndex:   0,
			Content:      "Jurisdiction-scoped chunk",
			Embedding:    zeroEmbedding(),
			TokenCount:   4,
			Metadata:     map[string]any{},
		},
		// Global — always appears.
		{
			Scope:      "global",
			SourceType: "document",
			SourceID:   uuid.New(),
			ChunkIndex: 0,
			Content:    "Global chunk",
			Embedding:  zeroEmbedding(),
			TokenCount: 2,
			Metadata:   map[string]any{},
		},
		// Belongs to a different HOA — should NOT appear.
		{
			Scope:      "org",
			OrgID:      &otherHOA.ID,
			SourceType: "governing_document",
			SourceID:   uuid.New(),
			ChunkIndex: 0,
			Content:    "Different HOA chunk — must not appear",
			Embedding:  zeroEmbedding(),
			TokenCount: 5,
			Metadata:   map[string]any{},
		},
	}
	require.NoError(t, repo.CreateBatch(ctx, chunks))

	results, err := repo.SimilaritySearch(ctx, zeroEmbedding(), hoaOrg.ID, &firmOrg.ID, &jurisdiction, ai.ContextFilters{}, 20)
	require.NoError(t, err)

	contents := make([]string, len(results))
	for i, r := range results {
		contents[i] = r.Content
	}

	assert.Contains(t, contents, "Org-scoped chunk for the searching HOA")
	assert.Contains(t, contents, "Firm-scoped chunk")
	assert.Contains(t, contents, "Jurisdiction-scoped chunk")
	assert.Contains(t, contents, "Global chunk")
	assert.NotContains(t, contents, "Different HOA chunk — must not appear")
}

func TestSimilaritySearch_SourceTypeFilter(t *testing.T) {
	pool := setupAITestDB(t)
	repo := ai.NewPostgresContextChunkRepository(pool)
	ctx := context.Background()

	o := setupOrg(t, pool, "hoa")
	sourceID := uuid.New()

	chunks := []*ai.ContextChunk{
		{
			Scope:      "org",
			OrgID:      &o.ID,
			SourceType: "governing_document",
			SourceID:   sourceID,
			ChunkIndex: 0,
			Content:    "Governing doc chunk",
			Embedding:  zeroEmbedding(),
			TokenCount: 3,
			Metadata:   map[string]any{},
		},
		{
			Scope:      "org",
			OrgID:      &o.ID,
			SourceType: "meeting_minutes",
			SourceID:   uuid.New(),
			ChunkIndex: 0,
			Content:    "Meeting minutes chunk",
			Embedding:  zeroEmbedding(),
			TokenCount: 3,
			Metadata:   map[string]any{},
		},
	}
	require.NoError(t, repo.CreateBatch(ctx, chunks))

	filters := ai.ContextFilters{
		SourceTypes: []ai.ContextSourceType{"governing_document"},
	}
	results, err := repo.SimilaritySearch(ctx, zeroEmbedding(), o.ID, nil, nil, filters, 10)
	require.NoError(t, err)

	for _, r := range results {
		assert.Equal(t, "governing_document", r.SourceType, "only governing_document source type should be returned")
	}
	require.NotEmpty(t, results)
}

func TestSimilaritySearch_UnitIDFilter(t *testing.T) {
	pool := setupAITestDB(t)
	repo := ai.NewPostgresContextChunkRepository(pool)
	ctx := context.Background()

	o := setupOrg(t, pool, "hoa")
	unitID := uuid.New()

	chunks := []*ai.ContextChunk{
		{
			Scope:      "org",
			OrgID:      &o.ID,
			SourceType: "governing_document",
			SourceID:   uuid.New(),
			ChunkIndex: 0,
			Content:    "Chunk with unit_id in metadata",
			Embedding:  zeroEmbedding(),
			TokenCount: 5,
			Metadata:   map[string]any{"unit_id": unitID.String()},
		},
		{
			Scope:      "org",
			OrgID:      &o.ID,
			SourceType: "governing_document",
			SourceID:   uuid.New(),
			ChunkIndex: 0,
			Content:    "Chunk with no unit_id",
			Embedding:  zeroEmbedding(),
			TokenCount: 4,
			Metadata:   map[string]any{},
		},
	}
	require.NoError(t, repo.CreateBatch(ctx, chunks))

	filters := ai.ContextFilters{UnitID: &unitID}
	results, err := repo.SimilaritySearch(ctx, zeroEmbedding(), o.ID, nil, nil, filters, 10)
	require.NoError(t, err)

	require.Len(t, results, 1, "only the chunk with matching unit_id should be returned")
	assert.Equal(t, "Chunk with unit_id in metadata", results[0].Content)
}

func TestSimilaritySearch_DateRangeFilter(t *testing.T) {
	pool := setupAITestDB(t)
	repo := ai.NewPostgresContextChunkRepository(pool)
	ctx := context.Background()

	o := setupOrg(t, pool, "hoa")

	// Insert a chunk so we have something to filter.
	require.NoError(t, repo.CreateBatch(ctx, []*ai.ContextChunk{
		{
			Scope:      "org",
			OrgID:      &o.ID,
			SourceType: "governing_document",
			SourceID:   uuid.New(),
			ChunkIndex: 0,
			Content:    "Recent chunk",
			Embedding:  zeroEmbedding(),
			TokenCount: 2,
			Metadata:   map[string]any{},
		},
	}))

	// A date range that covers now should return results.
	tr := &ai.TimeRange{
		Start: time.Now().Add(-1 * time.Minute),
		End:   time.Now().Add(1 * time.Minute),
	}
	filters := ai.ContextFilters{DateRange: tr}
	results, err := repo.SimilaritySearch(ctx, zeroEmbedding(), o.ID, nil, nil, filters, 10)
	require.NoError(t, err)
	require.NotEmpty(t, results, "chunks within the date range should be returned")

	// A date range entirely in the past should return nothing.
	oldRange := &ai.TimeRange{
		Start: time.Now().Add(-24 * time.Hour),
		End:   time.Now().Add(-23 * time.Hour),
	}
	filters2 := ai.ContextFilters{DateRange: oldRange}
	results2, err := repo.SimilaritySearch(ctx, zeroEmbedding(), o.ID, nil, nil, filters2, 10)
	require.NoError(t, err)
	assert.Empty(t, results2, "chunks outside the date range should not be returned")
}

func TestSimilaritySearch_ScopesFilter(t *testing.T) {
	pool := setupAITestDB(t)
	repo := ai.NewPostgresContextChunkRepository(pool)
	ctx := context.Background()

	o := setupOrg(t, pool, "hoa")

	chunks := []*ai.ContextChunk{
		{
			Scope:      "org",
			OrgID:      &o.ID,
			SourceType: "governing_document",
			SourceID:   uuid.New(),
			ChunkIndex: 0,
			Content:    "Org-scoped chunk",
			Embedding:  zeroEmbedding(),
			TokenCount: 3,
			Metadata:   map[string]any{},
		},
		{
			Scope:      "global",
			SourceType: "governing_document",
			SourceID:   uuid.New(),
			ChunkIndex: 0,
			Content:    "Global chunk",
			Embedding:  zeroEmbedding(),
			TokenCount: 2,
			Metadata:   map[string]any{},
		},
	}
	require.NoError(t, repo.CreateBatch(ctx, chunks))

	// Only request org-scoped results.
	filters := ai.ContextFilters{Scopes: []ai.ContextScope{"org"}}
	results, err := repo.SimilaritySearch(ctx, zeroEmbedding(), o.ID, nil, nil, filters, 10)
	require.NoError(t, err)

	for _, r := range results {
		assert.Equal(t, "org", r.Scope, "only org-scoped chunks should be returned")
	}
	require.NotEmpty(t, results)

	// Verify global chunk is absent.
	contents := make([]string, len(results))
	for i, r := range results {
		contents[i] = r.Content
	}
	assert.NotContains(t, contents, "Global chunk")
}

// ─── Validate ────────────────────────────────────────────────────────────────

func TestValidate_GlobalScope(t *testing.T) {
	orgID := uuid.New()
	jur := "CA"

	assert.NoError(t, (&ai.ContextChunk{Scope: "global"}).Validate())
	assert.Error(t, (&ai.ContextChunk{Scope: "global", OrgID: &orgID}).Validate(), "global with org_id should fail")
	assert.Error(t, (&ai.ContextChunk{Scope: "global", Jurisdiction: &jur}).Validate(), "global with jurisdiction should fail")
}

func TestValidate_JurisdictionScope(t *testing.T) {
	orgID := uuid.New()
	jur := "TX"
	empty := ""

	assert.NoError(t, (&ai.ContextChunk{Scope: "jurisdiction", Jurisdiction: &jur}).Validate())
	assert.Error(t, (&ai.ContextChunk{Scope: "jurisdiction"}).Validate(), "jurisdiction without jurisdiction should fail")
	assert.Error(t, (&ai.ContextChunk{Scope: "jurisdiction", Jurisdiction: &empty}).Validate(), "jurisdiction with empty string should fail")
	assert.Error(t, (&ai.ContextChunk{Scope: "jurisdiction", Jurisdiction: &jur, OrgID: &orgID}).Validate(), "jurisdiction with org_id should fail")
}

func TestValidate_FirmScope(t *testing.T) {
	orgID := uuid.New()
	jur := "FL"

	assert.NoError(t, (&ai.ContextChunk{Scope: "firm", OrgID: &orgID}).Validate())
	assert.Error(t, (&ai.ContextChunk{Scope: "firm"}).Validate(), "firm without org_id should fail")
	assert.Error(t, (&ai.ContextChunk{Scope: "firm", OrgID: &orgID, Jurisdiction: &jur}).Validate(), "firm with jurisdiction should fail")
}

func TestValidate_OrgScope(t *testing.T) {
	orgID := uuid.New()
	jur := "NY"

	assert.NoError(t, (&ai.ContextChunk{Scope: "org", OrgID: &orgID}).Validate())
	assert.Error(t, (&ai.ContextChunk{Scope: "org"}).Validate(), "org without org_id should fail")
	assert.Error(t, (&ai.ContextChunk{Scope: "org", OrgID: &orgID, Jurisdiction: &jur}).Validate(), "org with jurisdiction should fail")
}

func TestValidate_UnknownScope(t *testing.T) {
	assert.Error(t, (&ai.ContextChunk{Scope: "tenant"}).Validate(), "unknown scope should fail")
}
