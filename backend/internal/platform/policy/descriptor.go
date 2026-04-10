package policy

import (
	"context"
	"encoding/json"
)

// OperationDescriptor is registered by each module for each operation category.
// It defines the policy types the operation depends on, the expected AI ruling
// schema, and callbacks for the hold/proceed lifecycle.
type OperationDescriptor struct {
	Category         string
	Description      string
	DefaultThreshold float64
	Policies         map[string]PolicySpec
	RulingSchema     json.RawMessage
	PromptTemplate   string
	OnHold           func(ctx context.Context, res *Resolution) error
	OnProceed        func(ctx context.Context, res *Resolution) error
}

// PolicySpec defines a single policy type within a category. It serves as the
// shared contract between the ingestion pipeline, the resolution engine, and
// human reviewers.
type PolicySpec struct {
	Description   string
	DocumentTypes []string
	Concepts      []string
	Schema        json.RawMessage
}
