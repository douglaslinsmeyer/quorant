package ai

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
)

// PolicyResult represents a resolved policy.
type PolicyResult struct {
	Config         json.RawMessage `json:"config"`
	Confidence     float64         `json:"confidence"`
	ReviewStatus   string          `json:"review_status"` // "approved", "pending", "ai_inferred"
	SourceSection  string          `json:"source_section"`
	RequiresReview bool            `json:"requires_review"`
}

// ResolutionResult represents an AI resolution to a policy question.
type ResolutionResult struct {
	Resolution     json.RawMessage `json:"resolution"`
	Reasoning      string          `json:"reasoning"`
	SourcePassages []SourcePassage `json:"source_passages"`
	Confidence     float64         `json:"confidence"`
	Escalated      bool            `json:"escalated"`
}

type SourcePassage struct {
	DocID   uuid.UUID `json:"doc_id"`
	Section string    `json:"section"`
	Text    string    `json:"text"`
}

type QueryContext struct {
	Module       string          `json:"module"`
	ResourceType string          `json:"resource_type"`
	ResourceID   uuid.UUID       `json:"resource_id"`
	Extra        json.RawMessage `json:"extra,omitempty"`
}

// PolicyResolver handles structured policy questions.
type PolicyResolver interface {
	GetPolicy(ctx context.Context, orgID uuid.UUID, policyKey string) (*PolicyResult, error)
	QueryPolicy(ctx context.Context, orgID uuid.UUID, query string, qctx QueryContext) (*ResolutionResult, error)
}

// NoopPolicyResolver is a stub. Used until the AI module is built in Phase 13.
type NoopPolicyResolver struct{}

func NewNoopPolicyResolver() *NoopPolicyResolver { return &NoopPolicyResolver{} }
func (r *NoopPolicyResolver) GetPolicy(ctx context.Context, orgID uuid.UUID, policyKey string) (*PolicyResult, error) {
	return nil, nil // no policy available
}
func (r *NoopPolicyResolver) QueryPolicy(ctx context.Context, orgID uuid.UUID, query string, qctx QueryContext) (*ResolutionResult, error) {
	return nil, nil // no resolution available
}
