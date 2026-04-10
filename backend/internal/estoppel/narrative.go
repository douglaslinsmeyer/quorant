package estoppel

import (
	"context"

	"github.com/google/uuid"
)

// NarrativeGenerator produces human-readable narrative sections from
// aggregated estoppel data. Implementations may use AI, templates, or a
// combination of both.
type NarrativeGenerator interface {
	// GenerateNarratives returns a populated NarrativeSections for the given
	// organisation and aggregated data. All returned fields must be non-nil
	// slices (empty slices are acceptable).
	GenerateNarratives(ctx context.Context, orgID uuid.UUID, data *AggregatedData) (*NarrativeSections, error)
}

// placeholderText is the standard fallback value used by NoopNarrativeGenerator
// when no AI context lake is available.
const placeholderText = "Manual entry required — no governing documents indexed."

// NoopNarrativeGenerator is a NarrativeGenerator that returns placeholder text
// for every field. It is used when the AI context lake is unavailable or has
// not yet indexed the organisation's governing documents.
type NoopNarrativeGenerator struct{}

// NewNoopNarrativeGenerator returns a new NoopNarrativeGenerator.
func NewNoopNarrativeGenerator() *NoopNarrativeGenerator {
	return &NoopNarrativeGenerator{}
}

// GenerateNarratives returns a NarrativeSections where every section contains
// exactly one NarrativeField populated with placeholder text.
// All fields have Editable = true and Confidence = 0.
func (g *NoopNarrativeGenerator) GenerateNarratives(_ context.Context, _ uuid.UUID, _ *AggregatedData) (*NarrativeSections, error) {
	placeholder := func(label string) []NarrativeField {
		return []NarrativeField{
			{
				Label: label,
				Value: placeholderText,
			},
		}
	}

	return &NarrativeSections{
		AssessmentSummary:     placeholder("Assessment Summary"),
		DelinquencySummary:    placeholder("Delinquency Summary"),
		ComplianceSummary:     placeholder("Compliance Summary"),
		InsuranceSummary:      placeholder("Insurance Summary"),
		LitigationSummary:     placeholder("Litigation Summary"),
		TransferFees:          placeholder("Transfer Fees"),
		AdditionalDisclosures: placeholder("Additional Disclosures"),
	}, nil
}
