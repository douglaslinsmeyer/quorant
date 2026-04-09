package ai_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/ai"
	"github.com/quorant/quorant/internal/org"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Mock Repository ─────────────────────────────────────────────────────────

type mockContextChunkRepo struct {
	createBatchFunc    func(ctx context.Context, chunks []*ai.ContextChunk) error
	deleteBySourceFunc func(ctx context.Context, sourceID uuid.UUID) error
	similarityFunc     func(ctx context.Context, embedding []float32, orgID uuid.UUID, firmOrgID *uuid.UUID, jurisdiction *string, filters ai.ContextFilters, limit int) ([]ai.ContextResult, error)

	createBatchCalled    bool
	deleteBySourceCalled bool
	similarityCalled     bool

	lastEmbedding []float32
}

func (m *mockContextChunkRepo) Create(ctx context.Context, chunk *ai.ContextChunk) (*ai.ContextChunk, error) {
	return chunk, nil
}

func (m *mockContextChunkRepo) CreateBatch(ctx context.Context, chunks []*ai.ContextChunk) error {
	m.createBatchCalled = true
	if m.createBatchFunc != nil {
		return m.createBatchFunc(ctx, chunks)
	}
	return nil
}

func (m *mockContextChunkRepo) DeleteBySource(ctx context.Context, sourceID uuid.UUID) error {
	m.deleteBySourceCalled = true
	if m.deleteBySourceFunc != nil {
		return m.deleteBySourceFunc(ctx, sourceID)
	}
	return nil
}

func (m *mockContextChunkRepo) SimilaritySearch(ctx context.Context, embedding []float32, orgID uuid.UUID, firmOrgID *uuid.UUID, jurisdiction *string, filters ai.ContextFilters, limit int) ([]ai.ContextResult, error) {
	m.similarityCalled = true
	m.lastEmbedding = embedding
	if m.similarityFunc != nil {
		return m.similarityFunc(ctx, embedding, orgID, firmOrgID, jurisdiction, filters, limit)
	}
	return nil, nil
}

// ─── Mock Org Repository ─────────────────────────────────────────────────────

type mockOrgRepo struct {
	findByIDFunc           func(ctx context.Context, id uuid.UUID) (*org.Organization, error)
	findActiveManagementFn func(ctx context.Context, hoaOrgID uuid.UUID) (*org.OrgManagement, error)
}

func (m *mockOrgRepo) Create(ctx context.Context, o *org.Organization) (*org.Organization, error) {
	return nil, nil
}
func (m *mockOrgRepo) FindByID(ctx context.Context, id uuid.UUID) (*org.Organization, error) {
	if m.findByIDFunc != nil {
		return m.findByIDFunc(ctx, id)
	}
	state := "CA"
	return &org.Organization{ID: id, State: &state}, nil
}
func (m *mockOrgRepo) FindBySlug(ctx context.Context, slug string) (*org.Organization, error) {
	return nil, nil
}
func (m *mockOrgRepo) ListByUserAccess(ctx context.Context, userID uuid.UUID) ([]org.Organization, error) {
	return nil, nil
}
func (m *mockOrgRepo) Update(ctx context.Context, o *org.Organization) (*org.Organization, error) {
	return nil, nil
}
func (m *mockOrgRepo) SoftDelete(ctx context.Context, id uuid.UUID) error { return nil }
func (m *mockOrgRepo) ListChildren(ctx context.Context, parentID uuid.UUID) ([]org.Organization, error) {
	return nil, nil
}
func (m *mockOrgRepo) ConnectManagement(ctx context.Context, firmOrgID, hoaOrgID uuid.UUID) (*org.OrgManagement, error) {
	return nil, nil
}
func (m *mockOrgRepo) DisconnectManagement(ctx context.Context, hoaOrgID uuid.UUID) error {
	return nil
}
func (m *mockOrgRepo) ListManagementHistory(ctx context.Context, hoaOrgID uuid.UUID) ([]org.OrgManagement, error) {
	return nil, nil
}
func (m *mockOrgRepo) FindActiveManagement(ctx context.Context, hoaOrgID uuid.UUID) (*org.OrgManagement, error) {
	if m.findActiveManagementFn != nil {
		return m.findActiveManagementFn(ctx, hoaOrgID)
	}
	return nil, nil // self-managed by default
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func newTestService(repo ai.ContextChunkRepository, orgRepo org.OrgRepository, embedFn ai.EmbeddingFunc) *ai.ContextLakeService {
	return ai.NewContextLakeService(repo, orgRepo, embedFn, newTestLogger())
}

// ─── TestIngestChunks ─────────────────────────────────────────────────────────

func TestIngestChunks_CallsCreateBatch(t *testing.T) {
	mock := &mockContextChunkRepo{}
	svc := newTestService(mock, &mockOrgRepo{}, ai.StubEmbeddingFunc)
	ctx := context.Background()

	orgID := uuid.New()
	chunks := []*ai.ContextChunk{
		{
			Scope:      "org",
			OrgID:      &orgID,
			SourceType: "governing_document",
			SourceID:   uuid.New(),
			ChunkIndex: 0,
			Content:    "Test chunk",
			Embedding:  make([]float32, ai.EmbeddingDimensions),
			TokenCount: 2,
			Metadata:   map[string]any{},
		},
	}

	err := svc.IngestChunks(ctx, chunks)

	require.NoError(t, err)
	assert.True(t, mock.createBatchCalled, "CreateBatch should have been called")
}

func TestIngestChunks_PropagatesRepoError(t *testing.T) {
	repoErr := errors.New("db write failed")
	mock := &mockContextChunkRepo{
		createBatchFunc: func(_ context.Context, _ []*ai.ContextChunk) error {
			return repoErr
		},
	}
	svc := newTestService(mock, &mockOrgRepo{}, ai.StubEmbeddingFunc)

	err := svc.IngestChunks(context.Background(), []*ai.ContextChunk{{
		Scope:      "org",
		OrgID:      func() *uuid.UUID { id := uuid.New(); return &id }(),
		SourceType: "governing_document",
		SourceID:   uuid.New(),
		Embedding:  make([]float32, ai.EmbeddingDimensions),
		Metadata:   map[string]any{},
	}})

	require.Error(t, err)
	assert.ErrorContains(t, err, "db write failed")
}

// ─── TestDeleteSourceChunks ───────────────────────────────────────────────────

func TestDeleteSourceChunks_CallsDeleteBySource(t *testing.T) {
	mock := &mockContextChunkRepo{}
	svc := newTestService(mock, &mockOrgRepo{}, ai.StubEmbeddingFunc)

	sourceID := uuid.New()
	err := svc.DeleteSourceChunks(context.Background(), sourceID)

	require.NoError(t, err)
	assert.True(t, mock.deleteBySourceCalled, "DeleteBySource should have been called")
}

// ─── TestSearch_UsesEmbeddingFunc ─────────────────────────────────────────────

func TestSearch_UsesEmbeddingFunc(t *testing.T) {
	capturedText := ""
	customEmbedding := make([]float32, ai.EmbeddingDimensions)
	customEmbedding[42] = 0.9

	embedFn := func(ctx context.Context, text string) ([]float32, error) {
		capturedText = text
		return customEmbedding, nil
	}

	mock := &mockContextChunkRepo{}
	svc := newTestService(mock, &mockOrgRepo{}, embedFn)

	orgID := uuid.New()
	_, err := svc.Search(context.Background(), orgID, "pet policy rules", ai.ContextFilters{MaxResults: 5})

	require.NoError(t, err)
	assert.Equal(t, "pet policy rules", capturedText, "embedding func should receive the query text")
	assert.True(t, mock.similarityCalled, "SimilaritySearch should have been called")
	assert.Equal(t, customEmbedding, mock.lastEmbedding, "embedding from func should be passed to repo")
}

func TestSearch_ReturnsErrorWhenNoEmbeddingFunc(t *testing.T) {
	mock := &mockContextChunkRepo{}
	svc := newTestService(mock, &mockOrgRepo{}, nil)

	_, err := svc.Search(context.Background(), uuid.New(), "query", ai.ContextFilters{})

	require.Error(t, err)
	assert.ErrorContains(t, err, "embedding service not configured")
}

// ─── TestResolveScopes ────────────────────────────────────────────────────────

func TestResolveScopes_ReturnsJurisdictionAndFirm(t *testing.T) {
	state := "TX"
	firmID := uuid.New()
	orgID := uuid.New()

	orgRepo := &mockOrgRepo{
		findByIDFunc: func(_ context.Context, id uuid.UUID) (*org.Organization, error) {
			return &org.Organization{ID: id, State: &state}, nil
		},
		findActiveManagementFn: func(_ context.Context, _ uuid.UUID) (*org.OrgManagement, error) {
			return &org.OrgManagement{FirmOrgID: firmID}, nil
		},
	}
	svc := newTestService(&mockContextChunkRepo{}, orgRepo, ai.StubEmbeddingFunc)

	firmOrgID, jurisdiction, err := svc.ResolveScopes(context.Background(), orgID)

	require.NoError(t, err)
	require.NotNil(t, firmOrgID)
	assert.Equal(t, firmID, *firmOrgID)
	require.NotNil(t, jurisdiction)
	assert.Equal(t, "TX", *jurisdiction)
}

func TestResolveScopes_SelfManagedReturnsNilFirm(t *testing.T) {
	state := "FL"
	orgRepo := &mockOrgRepo{
		findByIDFunc: func(_ context.Context, id uuid.UUID) (*org.Organization, error) {
			return &org.Organization{ID: id, State: &state}, nil
		},
		// findActiveManagementFn is nil — defaults to returning nil (self-managed)
	}
	svc := newTestService(&mockContextChunkRepo{}, orgRepo, ai.StubEmbeddingFunc)

	firmOrgID, _, err := svc.ResolveScopes(context.Background(), uuid.New())

	require.NoError(t, err)
	assert.Nil(t, firmOrgID, "self-managed org should have nil firmOrgID")
}

// ─── TestUnitContext ──────────────────────────────────────────────────────────

func TestUnitContext_PassesUnitIDInFilters(t *testing.T) {
	capturedFilters := ai.ContextFilters{}
	mock := &mockContextChunkRepo{
		similarityFunc: func(_ context.Context, _ []float32, _ uuid.UUID, _ *uuid.UUID, _ *string, filters ai.ContextFilters, _ int) ([]ai.ContextResult, error) {
			capturedFilters = filters
			return nil, nil
		},
	}
	svc := newTestService(mock, &mockOrgRepo{}, ai.StubEmbeddingFunc)

	unitID := uuid.New()
	_, err := svc.UnitContext(context.Background(), uuid.New(), unitID, "parking violations")

	require.NoError(t, err)
	require.NotNil(t, capturedFilters.UnitID)
	assert.Equal(t, unitID, *capturedFilters.UnitID)
}

// ─── TestTopicContext ─────────────────────────────────────────────────────────

func TestTopicContext_PassesTimeRangeInFilters(t *testing.T) {
	capturedFilters := ai.ContextFilters{}
	mock := &mockContextChunkRepo{
		similarityFunc: func(_ context.Context, _ []float32, _ uuid.UUID, _ *uuid.UUID, _ *string, filters ai.ContextFilters, _ int) ([]ai.ContextResult, error) {
			capturedFilters = filters
			return nil, nil
		},
	}
	svc := newTestService(mock, &mockOrgRepo{}, ai.StubEmbeddingFunc)

	tr := &ai.TimeRange{
		Start: time.Now().Add(-30 * 24 * time.Hour),
		End:   time.Now(),
	}
	_, err := svc.TopicContext(context.Background(), uuid.New(), "budget amendments", tr)

	require.NoError(t, err)
	require.NotNil(t, capturedFilters.DateRange)
	assert.Equal(t, tr.Start, capturedFilters.DateRange.Start)
}

// ─── TestPostgresContextRetriever ────────────────────────────────────────────

func TestPostgresContextRetriever_ImplementsInterface(t *testing.T) {
	svc := newTestService(&mockContextChunkRepo{}, &mockOrgRepo{}, ai.StubEmbeddingFunc)
	retriever := ai.NewPostgresContextRetriever(svc)

	// Verify it satisfies the ContextRetriever interface at compile time via assignment.
	var _ ai.ContextRetriever = retriever
}
