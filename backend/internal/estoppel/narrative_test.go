package estoppel_test

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/estoppel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// TestNarrativeGenerator_GeneratesAllFields
// ---------------------------------------------------------------------------

// TestNarrativeGenerator_GeneratesAllFields verifies that the Noop generator
// populates every section of NarrativeSections with at least one field.
func TestNarrativeGenerator_GeneratesAllFields(t *testing.T) {
	gen := estoppel.NewNoopNarrativeGenerator()
	var ng estoppel.NarrativeGenerator = gen

	data := newTestAggregatedData()
	sections, err := ng.GenerateNarratives(context.Background(), uuid.New(), data)

	require.NoError(t, err)
	require.NotNil(t, sections)

	sectionChecks := []struct {
		name   string
		fields []estoppel.NarrativeField
	}{
		{"AssessmentSummary", sections.AssessmentSummary},
		{"DelinquencySummary", sections.DelinquencySummary},
		{"ComplianceSummary", sections.ComplianceSummary},
		{"InsuranceSummary", sections.InsuranceSummary},
		{"LitigationSummary", sections.LitigationSummary},
		{"TransferFees", sections.TransferFees},
		{"AdditionalDisclosures", sections.AdditionalDisclosures},
	}

	for _, sc := range sectionChecks {
		t.Run(sc.name, func(t *testing.T) {
			assert.NotEmpty(t, sc.fields, "expected at least one narrative field in %s", sc.name)
			for _, f := range sc.fields {
				assert.NotEmpty(t, f.Label, "field Label must not be empty")
				assert.NotEmpty(t, f.Value, "field Value must not be empty")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestNarrativeGenerator_FallbackWhenNoContext
// ---------------------------------------------------------------------------

// TestNarrativeGenerator_FallbackWhenNoContext verifies that when no AI context
// is available the Noop generator returns the canonical placeholder text in
// every field value.
func TestNarrativeGenerator_FallbackWhenNoContext(t *testing.T) {
	gen := estoppel.NewNoopNarrativeGenerator()

	sections, err := gen.GenerateNarratives(context.Background(), uuid.New(), newTestAggregatedData())

	require.NoError(t, err)
	require.NotNil(t, sections)

	allFields := [][]estoppel.NarrativeField{
		sections.AssessmentSummary,
		sections.DelinquencySummary,
		sections.ComplianceSummary,
		sections.InsuranceSummary,
		sections.LitigationSummary,
		sections.TransferFees,
		sections.AdditionalDisclosures,
	}

	for _, fields := range allFields {
		for _, f := range fields {
			assert.True(
				t,
				strings.Contains(f.Value, "Manual entry required"),
				"expected placeholder text to contain 'Manual entry required', got: %q",
				f.Value,
			)
		}
	}
}
