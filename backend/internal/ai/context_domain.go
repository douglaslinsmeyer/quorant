package ai

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// EmbeddingDimensions is the vector size for all context lake embeddings.
// Change this constant when migrating to a different embedding model.
const EmbeddingDimensions = 1536

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
	Embedding    []float32      `json:"embedding"` // EmbeddingDimensions-dim vector
	TokenCount   int            `json:"token_count"`
	Metadata     map[string]any `json:"metadata"`
	CreatedAt    time.Time      `json:"created_at"`
}

// Validate checks that scope constraints are satisfied, returning a friendly error
// instead of a raw PostgreSQL constraint violation.
//
//   - global:       org_id must be nil, jurisdiction must be nil
//   - jurisdiction: org_id must be nil, jurisdiction must be non-empty
//   - firm:         org_id must be non-nil, jurisdiction must be nil
//   - org:          org_id must be non-nil, jurisdiction must be nil
func (c *ContextChunk) Validate() error {
	switch c.Scope {
	case "global":
		if c.OrgID != nil {
			return fmt.Errorf("context chunk: scope 'global' must not have org_id")
		}
		if c.Jurisdiction != nil {
			return fmt.Errorf("context chunk: scope 'global' must not have jurisdiction")
		}
	case "jurisdiction":
		if c.OrgID != nil {
			return fmt.Errorf("context chunk: scope 'jurisdiction' must not have org_id")
		}
		if c.Jurisdiction == nil || *c.Jurisdiction == "" {
			return fmt.Errorf("context chunk: scope 'jurisdiction' requires a non-empty jurisdiction")
		}
	case "firm":
		if c.OrgID == nil || *c.OrgID == uuid.Nil {
			return fmt.Errorf("context chunk: scope 'firm' requires org_id")
		}
		if c.Jurisdiction != nil {
			return fmt.Errorf("context chunk: scope 'firm' must not have jurisdiction")
		}
	case "org":
		if c.OrgID == nil || *c.OrgID == uuid.Nil {
			return fmt.Errorf("context chunk: scope 'org' requires org_id")
		}
		if c.Jurisdiction != nil {
			return fmt.Errorf("context chunk: scope 'org' must not have jurisdiction")
		}
	default:
		return fmt.Errorf("context chunk: unknown scope %q (must be global, jurisdiction, firm, or org)", c.Scope)
	}
	return nil
}
