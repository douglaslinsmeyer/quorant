package estoppel_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/estoppel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// errTestSentinel is a package-level sentinel error used across estoppel tests.
var errTestSentinel = errors.New("test sentinel error")

// ---------------------------------------------------------------------------
// Mock providers — reused by service tests later
// ---------------------------------------------------------------------------

// mockFinancialProvider is a test double for FinancialDataProvider.
type mockFinancialProvider struct {
	snapshot *estoppel.FinancialSnapshot
	err      error
}

func (m *mockFinancialProvider) GetUnitFinancialSnapshot(_ context.Context, _, _ uuid.UUID) (*estoppel.FinancialSnapshot, error) {
	return m.snapshot, m.err
}

// mockComplianceProvider is a test double for ComplianceDataProvider.
type mockComplianceProvider struct {
	snapshot *estoppel.ComplianceSnapshot
	err      error
}

func (m *mockComplianceProvider) GetUnitComplianceSnapshot(_ context.Context, _, _ uuid.UUID) (*estoppel.ComplianceSnapshot, error) {
	return m.snapshot, m.err
}

// mockPropertyProvider is a test double for PropertyDataProvider.
type mockPropertyProvider struct {
	snapshot *estoppel.PropertySnapshot
	err      error
}

func (m *mockPropertyProvider) GetPropertySnapshot(_ context.Context, _, _ uuid.UUID) (*estoppel.PropertySnapshot, error) {
	return m.snapshot, m.err
}

// ---------------------------------------------------------------------------
// Tests verifying mock behaviour
// ---------------------------------------------------------------------------

func TestMockFinancialProvider_ReturnsSnapshot(t *testing.T) {
	expected := &estoppel.FinancialSnapshot{
		RegularAssessmentCents: 30000,
		AssessmentFrequency:    "monthly",
		CurrentBalanceCents:    0,
	}
	provider := &mockFinancialProvider{snapshot: expected}

	var fp estoppel.FinancialDataProvider = provider
	got, err := fp.GetUnitFinancialSnapshot(context.Background(), uuid.New(), uuid.New())

	require.NoError(t, err)
	assert.Equal(t, expected.RegularAssessmentCents, got.RegularAssessmentCents)
	assert.Equal(t, expected.AssessmentFrequency, got.AssessmentFrequency)
}

func TestMockFinancialProvider_PropagatesError(t *testing.T) {
	provider := &mockFinancialProvider{err: errTestSentinel}

	var fp estoppel.FinancialDataProvider = provider
	snap, err := fp.GetUnitFinancialSnapshot(context.Background(), uuid.New(), uuid.New())

	assert.Nil(t, snap)
	assert.ErrorIs(t, err, errTestSentinel)
}

func TestMockComplianceProvider_ReturnsSnapshot(t *testing.T) {
	expected := &estoppel.ComplianceSnapshot{
		Violations: estoppel.ViolationSummary{
			OpenCount:       2,
			HasActiveFine:   true,
			TotalFinesCents: 10000,
		},
	}
	provider := &mockComplianceProvider{snapshot: expected}

	var cp estoppel.ComplianceDataProvider = provider
	got, err := cp.GetUnitComplianceSnapshot(context.Background(), uuid.New(), uuid.New())

	require.NoError(t, err)
	assert.Equal(t, 2, got.Violations.OpenCount)
	assert.True(t, got.Violations.HasActiveFine)
	assert.Equal(t, int64(10000), got.Violations.TotalFinesCents)
}

func TestMockComplianceProvider_PropagatesError(t *testing.T) {
	provider := &mockComplianceProvider{err: errTestSentinel}

	var cp estoppel.ComplianceDataProvider = provider
	snap, err := cp.GetUnitComplianceSnapshot(context.Background(), uuid.New(), uuid.New())

	assert.Nil(t, snap)
	assert.ErrorIs(t, err, errTestSentinel)
}

func TestMockPropertyProvider_ReturnsSnapshot(t *testing.T) {
	expected := &estoppel.PropertySnapshot{
		UnitNumber:    "42A",
		Address:       "42 Oak Lane, Springfield, IL 62701",
		SquareFootage: 1400,
		Bedrooms:      3,
		Bathrooms:     2.0,
		Owners: []estoppel.OwnerInfo{
			{Name: "Alice Nguyen", Email: "alice@example.com", IsOccupant: true},
		},
	}
	provider := &mockPropertyProvider{snapshot: expected}

	var pp estoppel.PropertyDataProvider = provider
	got, err := pp.GetPropertySnapshot(context.Background(), uuid.New(), uuid.New())

	require.NoError(t, err)
	assert.Equal(t, "42A", got.UnitNumber)
	assert.Equal(t, float64(1400), got.SquareFootage)
	require.Len(t, got.Owners, 1)
	assert.Equal(t, "Alice Nguyen", got.Owners[0].Name)
}

func TestMockPropertyProvider_PropagatesError(t *testing.T) {
	provider := &mockPropertyProvider{err: errTestSentinel}

	var pp estoppel.PropertyDataProvider = provider
	snap, err := pp.GetPropertySnapshot(context.Background(), uuid.New(), uuid.New())

	assert.Nil(t, snap)
	assert.ErrorIs(t, err, errTestSentinel)
}

// ---------------------------------------------------------------------------
// Interface compliance — compile-time checks
// ---------------------------------------------------------------------------

// These blank-identifier assignments verify that each mock satisfies the
// corresponding interface at compile time.
var (
	_ estoppel.FinancialDataProvider = (*mockFinancialProvider)(nil)
	_ estoppel.ComplianceDataProvider = (*mockComplianceProvider)(nil)
	_ estoppel.PropertyDataProvider   = (*mockPropertyProvider)(nil)
)

// ---------------------------------------------------------------------------
// AggregatedData assembly helper — used by service tests
// ---------------------------------------------------------------------------

// newTestAggregatedData returns a minimal AggregatedData suitable for use in
// unit tests that do not require real provider integration.
func newTestAggregatedData() *estoppel.AggregatedData {
	return &estoppel.AggregatedData{
		AsOfTime: time.Now(),
		Property: estoppel.PropertySnapshot{
			UnitNumber: "1A",
			Address:    "1 Test Ave",
		},
		Financial: estoppel.FinancialSnapshot{
			RegularAssessmentCents: 25000,
			AssessmentFrequency:    "monthly",
		},
		Compliance: estoppel.ComplianceSnapshot{},
	}
}
