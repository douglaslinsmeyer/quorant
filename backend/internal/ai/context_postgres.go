package ai

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	pgvector "github.com/pgvector/pgvector-go"
)

// PostgresContextChunkRepository implements ContextChunkRepository using pgxpool.
type PostgresContextChunkRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresContextChunkRepository creates a new repository backed by pool.
func NewPostgresContextChunkRepository(pool *pgxpool.Pool) *PostgresContextChunkRepository {
	return &PostgresContextChunkRepository{pool: pool}
}

// Create inserts a single ContextChunk and returns the persisted record.
func (r *PostgresContextChunkRepository) Create(ctx context.Context, chunk *ContextChunk) (*ContextChunk, error) {
	metaJSON, err := json.Marshal(chunk.Metadata)
	if err != nil {
		return nil, fmt.Errorf("ai: Create marshal metadata: %w", err)
	}

	const q = `
		INSERT INTO context_chunks (
			scope, org_id, jurisdiction,
			source_type, source_id, chunk_index,
			content, section_ref, page_number,
			embedding, token_count, metadata
		) VALUES (
			$1, $2, $3,
			$4, $5, $6,
			$7, $8, $9,
			$10, $11, $12
		)
		RETURNING id, scope, org_id, jurisdiction,
		          source_type, source_id, chunk_index,
		          content, section_ref, page_number,
		          embedding, token_count, metadata, created_at`

	row := r.pool.QueryRow(ctx, q,
		chunk.Scope,
		chunk.OrgID,
		chunk.Jurisdiction,
		chunk.SourceType,
		chunk.SourceID,
		chunk.ChunkIndex,
		chunk.Content,
		chunk.SectionRef,
		chunk.PageNumber,
		pgvector.NewVector(chunk.Embedding),
		chunk.TokenCount,
		metaJSON,
	)

	return scanChunk(row)
}

// CreateBatch inserts multiple chunks in a single transaction.
func (r *PostgresContextChunkRepository) CreateBatch(ctx context.Context, chunks []*ContextChunk) error {
	if len(chunks) == 0 {
		return nil
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("ai: CreateBatch begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	const q = `
		INSERT INTO context_chunks (
			scope, org_id, jurisdiction,
			source_type, source_id, chunk_index,
			content, section_ref, page_number,
			embedding, token_count, metadata
		) VALUES (
			$1, $2, $3,
			$4, $5, $6,
			$7, $8, $9,
			$10, $11, $12
		)`

	for i, chunk := range chunks {
		metaJSON, err := json.Marshal(chunk.Metadata)
		if err != nil {
			return fmt.Errorf("ai: CreateBatch chunk[%d] marshal metadata: %w", i, err)
		}

		_, err = tx.Exec(ctx, q,
			chunk.Scope,
			chunk.OrgID,
			chunk.Jurisdiction,
			chunk.SourceType,
			chunk.SourceID,
			chunk.ChunkIndex,
			chunk.Content,
			chunk.SectionRef,
			chunk.PageNumber,
			pgvector.NewVector(chunk.Embedding),
			chunk.TokenCount,
			metaJSON,
		)
		if err != nil {
			return fmt.Errorf("ai: CreateBatch chunk[%d] exec: %w", i, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("ai: CreateBatch commit: %w", err)
	}
	return nil
}

// DeleteBySource removes all chunks for a given source document.
func (r *PostgresContextChunkRepository) DeleteBySource(ctx context.Context, sourceID uuid.UUID) error {
	const q = `DELETE FROM context_chunks WHERE source_id = $1`
	_, err := r.pool.Exec(ctx, q, sourceID)
	if err != nil {
		return fmt.Errorf("ai: DeleteBySource: %w", err)
	}
	return nil
}

// SimilaritySearch finds chunks similar to the query embedding via cosine distance.
// The scope chain includes: the org itself, its managing firm (if any), jurisdiction, and global.
// Optional filters: SourceTypes, Scopes, UnitID (via metadata), DateRange (via created_at).
func (r *PostgresContextChunkRepository) SimilaritySearch(
	ctx context.Context,
	embedding []float32,
	orgID uuid.UUID,
	firmOrgID *uuid.UUID,
	jurisdiction *string,
	filters ContextFilters,
	limit int,
) ([]ContextResult, error) {
	// Build source_type filter — nil means no restriction.
	var sourceTypes []string
	for _, st := range filters.SourceTypes {
		sourceTypes = append(sourceTypes, string(st))
	}

	// Build scope filter — nil means no restriction.
	var scopes []string
	for _, s := range filters.Scopes {
		scopes = append(scopes, string(s))
	}

	// UnitID filter: match against metadata->>'unit_id'.
	var unitID *uuid.UUID
	if filters.UnitID != nil {
		unitID = filters.UnitID
	}

	// DateRange filter.
	var rangeStart, rangeEnd *interface{}
	var startPtr, endPtr interface{}
	if filters.DateRange != nil {
		startPtr = filters.DateRange.Start
		endPtr = filters.DateRange.End
		rangeStart = &startPtr
		rangeEnd = &endPtr
	}

	// Convert to pgvector type for the query parameter.
	vec := pgvector.NewVector(embedding)

	const q = `
		SELECT id, scope, source_type::text, source_id, content,
		       COALESCE(section_ref, '') AS section_ref,
		       metadata,
		       1 - (embedding <=> $1::vector) AS score
		FROM context_chunks
		WHERE (
			(scope = 'org'          AND org_id = $2)
			OR (scope = 'firm'         AND org_id = $3)
			OR (scope = 'jurisdiction' AND jurisdiction = $4)
			OR (scope = 'global')
		)
		AND ($5::text[] IS NULL OR source_type::text = ANY($5))
		AND ($6::text[] IS NULL OR scope::text = ANY($6))
		AND ($7::uuid IS NULL OR metadata->>'unit_id' = $7::text)
		AND ($8::timestamptz IS NULL OR created_at >= $8)
		AND ($9::timestamptz IS NULL OR created_at <= $9)
		ORDER BY embedding <=> $1::vector
		LIMIT $10`

	// Flatten date range pointers to nil-able values for pgx.
	var startVal, endVal interface{}
	if rangeStart != nil {
		startVal = *rangeStart
	}
	if rangeEnd != nil {
		endVal = *rangeEnd
	}

	rows, err := r.pool.Query(ctx, q,
		vec,
		orgID,
		firmOrgID,   // may be nil — matches nothing when NULL
		jurisdiction, // may be nil — matches nothing when NULL
		sourceTypes,  // nil means all source types
		scopes,       // nil means all scopes
		unitID,       // nil means no unit filter
		startVal,     // nil means no lower date bound
		endVal,       // nil means no upper date bound
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("ai: SimilaritySearch query: %w", err)
	}
	defer rows.Close()

	var results []ContextResult
	for rows.Next() {
		var cr ContextResult
		var metaRaw []byte
		var id, sourceID uuid.UUID

		if err := rows.Scan(
			&id,
			&cr.Scope,
			&cr.SourceType,
			&sourceID,
			&cr.Content,
			&cr.SectionRef,
			&metaRaw,
			&cr.Score,
		); err != nil {
			return nil, fmt.Errorf("ai: SimilaritySearch scan: %w", err)
		}
		cr.SourceID = sourceID

		if len(metaRaw) > 0 {
			if err := json.Unmarshal(metaRaw, &cr.Metadata); err != nil {
				return nil, fmt.Errorf("ai: SimilaritySearch unmarshal metadata: %w", err)
			}
		}
		if cr.Metadata == nil {
			cr.Metadata = map[string]any{}
		}

		results = append(results, cr)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ai: SimilaritySearch rows: %w", err)
	}

	return results, nil
}

// scanChunk reads a single context_chunk row from a pgx.Row.
func scanChunk(row interface {
	Scan(dest ...any) error
}) (*ContextChunk, error) {
	var c ContextChunk
	var vec pgvector.Vector
	var metaRaw []byte

	if err := row.Scan(
		&c.ID,
		&c.Scope,
		&c.OrgID,
		&c.Jurisdiction,
		&c.SourceType,
		&c.SourceID,
		&c.ChunkIndex,
		&c.Content,
		&c.SectionRef,
		&c.PageNumber,
		&vec,
		&c.TokenCount,
		&metaRaw,
		&c.CreatedAt,
	); err != nil {
		return nil, fmt.Errorf("ai: scanChunk: %w", err)
	}

	c.Embedding = vec.Slice()

	if len(metaRaw) > 0 {
		if err := json.Unmarshal(metaRaw, &c.Metadata); err != nil {
			return nil, fmt.Errorf("ai: scanChunk unmarshal metadata: %w", err)
		}
	}
	if c.Metadata == nil {
		c.Metadata = map[string]any{}
	}

	return &c, nil
}
