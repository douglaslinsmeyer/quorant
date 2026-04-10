// Package policy provides a reusable two-tier policy resolution engine.
// Tier 1 gathers applicable policy records from the database (deterministic).
// Tier 2 sends them to an AI resolver for precedence reasoning (interpretive).
// Any module can register operation descriptors and resolve policies at runtime.
package policy

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Resolution is the output of the two-tier policy resolution pipeline.
type Resolution struct {
	ID             uuid.UUID         `json:"id"`
	Status         string            `json:"status"` // "approved", "held"
	Ruling         json.RawMessage   `json:"ruling"`
	Reasoning      string            `json:"reasoning"`
	Confidence     float64           `json:"confidence"`
	SourcePolicies []PolicyReference `json:"source_policies"`
	ParentID       *uuid.UUID        `json:"parent_id,omitempty"`
}

// Held returns true if the resolution requires human review before the
// consuming operation can proceed.
func (r *Resolution) Held() bool {
	return r.Status == "held"
}

// Decode unmarshals the ruling JSON into the given target struct.
func (r *Resolution) Decode(target any) error {
	return json.Unmarshal(r.Ruling, target)
}

// PolicyRecord is a jurisdiction rule, org override, or unit-level policy
// stored in the database.
type PolicyRecord struct {
	ID              uuid.UUID       `json:"id"`
	Scope           string          `json:"scope"` // "jurisdiction", "org", "unit"
	Jurisdiction    *string         `json:"jurisdiction,omitempty"`
	OrgID           *uuid.UUID      `json:"org_id,omitempty"`
	UnitID          *uuid.UUID      `json:"unit_id,omitempty"`
	Category        string          `json:"category"`
	Key             string          `json:"key"`
	Value           json.RawMessage `json:"value"`
	PriorityHint    string          `json:"priority_hint"`
	StatuteRef      *string         `json:"statute_reference,omitempty"`
	SourceDocID     *uuid.UUID      `json:"source_doc_id,omitempty"`
	EffectiveDate   time.Time       `json:"effective_date"`
	ExpirationDate  *time.Time      `json:"expiration_date,omitempty"`
	IsActive        bool            `json:"is_active"`
	CreatedBy       *uuid.UUID      `json:"created_by,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

// PolicyReference is a lightweight reference to a policy record included in
// a resolution for audit purposes.
type PolicyReference struct {
	ID           uuid.UUID `json:"id"`
	Scope        string    `json:"scope"`
	Category     string    `json:"category"`
	Key          string    `json:"key"`
	PriorityHint string    `json:"priority_hint"`
	StatuteRef   *string   `json:"statute_reference,omitempty"`
}

// ResolutionRecord is the persisted form of a resolution, including the
// human review lifecycle fields.
type ResolutionRecord struct {
	ID                 uuid.UUID       `json:"id"`
	OrgID              uuid.UUID       `json:"org_id"`
	UnitID             *uuid.UUID      `json:"unit_id,omitempty"`
	Category           string          `json:"category"`
	InputPolicyIDs     []uuid.UUID     `json:"input_policy_ids"`
	Ruling             json.RawMessage `json:"ruling"`
	Reasoning          string          `json:"reasoning"`
	Confidence         float64         `json:"confidence"`
	ModelID            string          `json:"model_id"`
	ParentResolutionID *uuid.UUID      `json:"parent_resolution_id,omitempty"`
	ReviewStatus       string          `json:"review_status"`
	ReviewSLADeadline  *time.Time      `json:"review_sla_deadline,omitempty"`
	ReviewedBy         *uuid.UUID      `json:"reviewed_by,omitempty"`
	ReviewNotes        *string         `json:"review_notes,omitempty"`
	ReviewedAt         *time.Time      `json:"reviewed_at,omitempty"`
	CreatedAt          time.Time       `json:"created_at"`
}

// MatchedTrigger is returned by FindTriggers when a document type or concept
// matches a registered policy spec.
type MatchedTrigger struct {
	Category string
	Key      string
	Spec     PolicySpec
}
