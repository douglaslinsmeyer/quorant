package ai

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// GoverningDocument is a governing document registered for AI policy indexing.
type GoverningDocument struct {
	ID              uuid.UUID  `json:"id"`
	OrgID           uuid.UUID  `json:"org_id"`
	DocumentID      uuid.UUID  `json:"document_id"`
	DocType         string     `json:"doc_type"`      // ccr, bylaws, rules, amendment, state_statute
	Title           string     `json:"title"`
	EffectiveDate   time.Time  `json:"effective_date"`
	SupersedesID    *uuid.UUID `json:"supersedes_id,omitempty"`
	IndexingStatus  string     `json:"indexing_status"` // pending, processing, indexed, failed
	IndexedAt       *time.Time `json:"indexed_at,omitempty"`
	ChunkCount      *int       `json:"chunk_count,omitempty"`
	ExtractionCount *int       `json:"extraction_count,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

// PolicyExtraction is a structured policy extracted by AI from a governing document.
type PolicyExtraction struct {
	ID            uuid.UUID       `json:"id"`
	OrgID         uuid.UUID       `json:"org_id"`
	Domain        string          `json:"domain"`        // financial, governance, compliance, use_restrictions, operational
	PolicyKey     string          `json:"policy_key"`
	Config        json.RawMessage `json:"config"`
	Confidence    float64         `json:"confidence"`
	SourceDocID   uuid.UUID       `json:"source_doc_id"`
	SourceText    string          `json:"source_text"`
	SourceSection *string         `json:"source_section,omitempty"`
	SourcePage    *int            `json:"source_page,omitempty"`
	ReviewStatus  string          `json:"review_status"`  // pending, approved, rejected, modified
	ReviewedBy    *uuid.UUID      `json:"reviewed_by,omitempty"`
	ReviewedAt    *time.Time      `json:"reviewed_at,omitempty"`
	HumanOverride json.RawMessage `json:"human_override,omitempty"`
	ModelVersion  string          `json:"model_version"`
	EffectiveAt   time.Time       `json:"effective_at"`
	SupersededBy  *uuid.UUID      `json:"superseded_by,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
}

// PolicyResolution is a logged AI inference result for a policy question.
type PolicyResolution struct {
	ID                uuid.UUID       `json:"id"`
	OrgID             uuid.UUID       `json:"org_id"`
	Query             string          `json:"query"`
	PolicyKeys        []string        `json:"policy_keys"`
	Resolution        json.RawMessage `json:"resolution"`
	Reasoning         string          `json:"reasoning"`
	SourcePassages    json.RawMessage `json:"source_passages"`
	Confidence        float64         `json:"confidence"`
	ResolutionType    string          `json:"resolution_type"`  // cache_hit, ai_inference, human_escalated
	ModelVersion      *string         `json:"model_version,omitempty"`
	LatencyMs         *int            `json:"latency_ms,omitempty"`
	RequestingModule  string          `json:"requesting_module"`
	RequestingContext json.RawMessage `json:"requesting_context"`
	HumanDecision     json.RawMessage `json:"human_decision,omitempty"`
	DecidedBy         *uuid.UUID      `json:"decided_by,omitempty"`
	DecidedAt         *time.Time      `json:"decided_at,omitempty"`
	FedBack           bool            `json:"fed_back"`
	CreatedAt         time.Time       `json:"created_at"`
}
