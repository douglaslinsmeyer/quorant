package policy

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
)

// NoopRegistry is a test stub that returns pre-configured resolutions.
type NoopRegistry struct {
	ruling     json.RawMessage
	confidence float64
}

func NewNoopRegistry(ruling json.RawMessage, confidence float64) *NoopRegistry {
	return &NoopRegistry{ruling: ruling, confidence: confidence}
}

func NewNoopRegistryDefault() *NoopRegistry {
	return &NoopRegistry{ruling: json.RawMessage(`{}`), confidence: 1.0}
}

func (n *NoopRegistry) Register(category string, desc OperationDescriptor) error { return nil }

func (n *NoopRegistry) FindTriggers(documentType string, concepts []string) []MatchedTrigger {
	return nil
}

func (n *NoopRegistry) Resolve(ctx context.Context, orgID uuid.UUID, unitID *uuid.UUID, category string) (*Resolution, error) {
	return &Resolution{
		ID:         uuid.New(),
		Status:     "approved",
		Ruling:     n.ruling,
		Reasoning:  "noop: auto-approved for testing",
		Confidence: n.confidence,
	}, nil
}
