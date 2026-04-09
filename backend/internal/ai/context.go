package ai

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// ContextSourceType and ContextScope match the PostgreSQL enums.
type ContextSourceType string
type ContextScope string

type ContextFilters struct {
	SourceTypes []ContextSourceType
	Scopes      []ContextScope
	UnitID      *uuid.UUID
	DateRange   *TimeRange
	MaxResults  int
}

type TimeRange struct {
	Start time.Time
	End   time.Time
}

type ContextResult struct {
	Content    string         `json:"content"`
	SourceType string         `json:"source_type"`
	SourceID   uuid.UUID      `json:"source_id"`
	Scope      string         `json:"scope"`
	SectionRef string         `json:"section_ref"`
	Metadata   map[string]any `json:"metadata"`
	Score      float64        `json:"score"`
}

// ContextRetriever searches the context lake for institutional knowledge.
type ContextRetriever interface {
	Search(ctx context.Context, orgID uuid.UUID, query string, filters ContextFilters) ([]ContextResult, error)
	UnitContext(ctx context.Context, orgID uuid.UUID, unitID uuid.UUID, query string) ([]ContextResult, error)
	TopicContext(ctx context.Context, orgID uuid.UUID, topic string, timeRange *TimeRange) ([]ContextResult, error)
}

// NoopContextRetriever is a stub. Used until the AI module is built in Phase 12.
type NoopContextRetriever struct{}

func NewNoopContextRetriever() *NoopContextRetriever { return &NoopContextRetriever{} }
func (r *NoopContextRetriever) Search(ctx context.Context, orgID uuid.UUID, query string, filters ContextFilters) ([]ContextResult, error) {
	return nil, nil
}
func (r *NoopContextRetriever) UnitContext(ctx context.Context, orgID uuid.UUID, unitID uuid.UUID, query string) ([]ContextResult, error) {
	return nil, nil
}
func (r *NoopContextRetriever) TopicContext(ctx context.Context, orgID uuid.UUID, topic string, timeRange *TimeRange) ([]ContextResult, error) {
	return nil, nil
}
