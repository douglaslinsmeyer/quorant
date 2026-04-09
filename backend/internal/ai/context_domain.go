package ai

import (
	"time"

	"github.com/google/uuid"
)

// ContextChunk is a single embedded chunk of content stored in the context lake.
type ContextChunk struct {
	ID           uuid.UUID      `json:"id"`
	Scope        string         `json:"scope"`        // global, jurisdiction, firm, org
	OrgID        *uuid.UUID     `json:"org_id,omitempty"`
	Jurisdiction *string        `json:"jurisdiction,omitempty"`
	SourceType   string         `json:"source_type"`
	SourceID     uuid.UUID      `json:"source_id"`
	ChunkIndex   int            `json:"chunk_index"`
	Content      string         `json:"content"`
	SectionRef   *string        `json:"section_ref,omitempty"`
	PageNumber   *int           `json:"page_number,omitempty"`
	Embedding    []float32      `json:"embedding"` // 1536-dim vector
	TokenCount   int            `json:"token_count"`
	Metadata     map[string]any `json:"metadata"`
	CreatedAt    time.Time      `json:"created_at"`
}
