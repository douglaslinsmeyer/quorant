package ai

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
)

const defaultSearchLimit = 10

// EmbeddingFunc generates an embedding vector for text.
// In production, this calls an LLM API. For now, it's a stub.
type EmbeddingFunc func(ctx context.Context, text string) ([]float32, error)

// StubEmbeddingFunc returns a zero vector. Used until real LLM integration.
func StubEmbeddingFunc(ctx context.Context, text string) ([]float32, error) {
	return make([]float32, EmbeddingDimensions), nil
}

// ContextLakeService provides high-level access to context lake ingestion and retrieval.
type ContextLakeService struct {
	repo    ContextChunkRepository
	orgRepo OrgLookup
	embed   EmbeddingFunc
	logger  *slog.Logger
}

// NewContextLakeService creates a new ContextLakeService.
func NewContextLakeService(repo ContextChunkRepository, orgRepo OrgLookup, embed EmbeddingFunc, logger *slog.Logger) *ContextLakeService {
	return &ContextLakeService{
		repo:    repo,
		orgRepo: orgRepo,
		embed:   embed,
		logger:  logger,
	}
}

// IngestChunks batch-inserts chunks into the context lake.
func (s *ContextLakeService) IngestChunks(ctx context.Context, chunks []*ContextChunk) error {
	if err := s.repo.CreateBatch(ctx, chunks); err != nil {
		return fmt.Errorf("context lake: IngestChunks: %w", err)
	}
	return nil
}

// DeleteSourceChunks removes all chunks for a given source document (for re-indexing).
func (s *ContextLakeService) DeleteSourceChunks(ctx context.Context, sourceID uuid.UUID) error {
	if err := s.repo.DeleteBySource(ctx, sourceID); err != nil {
		return fmt.Errorf("context lake: DeleteSourceChunks: %w", err)
	}
	return nil
}

// ResolveScopes looks up the org's managing firm and state for scope chain resolution.
// Returns firmOrgID (nil if self-managed) and jurisdiction (the org's state, if set).
func (s *ContextLakeService) ResolveScopes(ctx context.Context, orgID uuid.UUID) (firmOrgID *uuid.UUID, jurisdiction *string, err error) {
	o, err := s.orgRepo.FindByID(ctx, orgID)
	if err != nil {
		return nil, nil, fmt.Errorf("context lake: ResolveScopes find org: %w", err)
	}
	if o == nil {
		return nil, nil, fmt.Errorf("context lake: ResolveScopes: org %s not found", orgID)
	}

	// Jurisdiction is the org's state field.
	jurisdiction = o.State

	// Look up managing firm (nil for self-managed HOAs).
	mgmt, err := s.orgRepo.FindActiveManagement(ctx, orgID)
	if err != nil {
		return nil, nil, fmt.Errorf("context lake: ResolveScopes find management: %w", err)
	}
	if mgmt != nil {
		firmOrgID = &mgmt.FirmOrgID
	}

	return firmOrgID, jurisdiction, nil
}

// Search implements ContextRetriever.Search. Embeds the query and runs similarity search.
func (s *ContextLakeService) Search(ctx context.Context, orgID uuid.UUID, query string, filters ContextFilters) ([]ContextResult, error) {
	if s.embed == nil {
		return nil, fmt.Errorf("context lake: Search: embedding service not configured")
	}

	embedding, err := s.embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("context lake: Search embed: %w", err)
	}

	firmOrgID, jurisdiction, err := s.ResolveScopes(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("context lake: Search resolve scopes: %w", err)
	}

	limit := filters.MaxResults
	if limit <= 0 {
		limit = defaultSearchLimit
	}

	return s.repo.SimilaritySearch(ctx, embedding, orgID, firmOrgID, jurisdiction, filters, limit)
}

// UnitContext searches for content relevant to a specific unit.
func (s *ContextLakeService) UnitContext(ctx context.Context, orgID uuid.UUID, unitID uuid.UUID, query string) ([]ContextResult, error) {
	filters := ContextFilters{
		UnitID:     &unitID,
		MaxResults: defaultSearchLimit,
	}
	return s.Search(ctx, orgID, query, filters)
}

// TopicContext searches with an optional time range filter.
func (s *ContextLakeService) TopicContext(ctx context.Context, orgID uuid.UUID, topic string, timeRange *TimeRange) ([]ContextResult, error) {
	filters := ContextFilters{
		DateRange:  timeRange,
		MaxResults: defaultSearchLimit,
	}
	return s.Search(ctx, orgID, topic, filters)
}

// ─── PostgresContextRetriever ────────────────────────────────────────────────

// PostgresContextRetriever wraps ContextLakeService and implements ContextRetriever.
// It replaces NoopContextRetriever once the AI layer is wired up.
type PostgresContextRetriever struct {
	service *ContextLakeService
}

// NewPostgresContextRetriever creates a PostgresContextRetriever backed by the given service.
func NewPostgresContextRetriever(service *ContextLakeService) *PostgresContextRetriever {
	return &PostgresContextRetriever{service: service}
}

// Search implements ContextRetriever.
func (r *PostgresContextRetriever) Search(ctx context.Context, orgID uuid.UUID, query string, filters ContextFilters) ([]ContextResult, error) {
	return r.service.Search(ctx, orgID, query, filters)
}

// UnitContext implements ContextRetriever.
func (r *PostgresContextRetriever) UnitContext(ctx context.Context, orgID uuid.UUID, unitID uuid.UUID, query string) ([]ContextResult, error) {
	return r.service.UnitContext(ctx, orgID, unitID, query)
}

// TopicContext implements ContextRetriever.
func (r *PostgresContextRetriever) TopicContext(ctx context.Context, orgID uuid.UUID, topic string, timeRange *TimeRange) ([]ContextResult, error) {
	return r.service.TopicContext(ctx, orgID, topic, timeRange)
}
