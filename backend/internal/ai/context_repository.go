package ai

import (
	"context"

	"github.com/google/uuid"
)

// ContextChunkRepository provides persistence for context lake chunks.
type ContextChunkRepository interface {
	// Create inserts a single chunk and returns the persisted record.
	Create(ctx context.Context, chunk *ContextChunk) (*ContextChunk, error)

	// CreateBatch inserts multiple chunks in a single operation.
	CreateBatch(ctx context.Context, chunks []*ContextChunk) error

	// DeleteBySource removes all chunks associated with the given source document.
	DeleteBySource(ctx context.Context, sourceID uuid.UUID) error

	// SimilaritySearch finds chunks similar to the query embedding.
	// Filters by scope chain for the given org context.
	// Returns results ordered by cosine similarity (highest first).
	SimilaritySearch(ctx context.Context, embedding []float32, orgID uuid.UUID, firmOrgID *uuid.UUID, jurisdiction *string, filters ContextFilters, limit int) ([]ContextResult, error)
}
