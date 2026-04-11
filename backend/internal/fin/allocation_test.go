package fin_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/fin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAllocate_SingleAssessmentFullPayment(t *testing.T) {
	chargeID := uuid.New()
	charges := []fin.OutstandingCharge{
		{
			ID:          chargeID,
			ChargeType:  fin.ChargeTypeRegularAssessment,
			AmountCents: 10000,
			DueDate:     time.Now().Add(-30 * 24 * time.Hour),
		},
	}
	ruling := fin.AllocationRuling{
		PriorityOrder: []fin.ChargeType{fin.ChargeTypeRegularAssessment},
		AcceptPartial: true,
	}

	results, credit := fin.Allocate(charges, 10000, ruling)

	require.Len(t, results, 1)
	assert.Equal(t, chargeID, results[0].ChargeID)
	assert.Equal(t, fin.ChargeTypeRegularAssessment, results[0].ChargeType)
	assert.Equal(t, int64(10000), results[0].AllocatedCents)
	assert.Equal(t, int64(0), credit)
}

func TestAllocate_PartialPayment(t *testing.T) {
	chargeID := uuid.New()
	charges := []fin.OutstandingCharge{
		{
			ID:          chargeID,
			ChargeType:  fin.ChargeTypeRegularAssessment,
			AmountCents: 10000,
			DueDate:     time.Now().Add(-30 * 24 * time.Hour),
		},
	}
	ruling := fin.AllocationRuling{
		PriorityOrder: []fin.ChargeType{fin.ChargeTypeRegularAssessment},
		AcceptPartial: true,
	}

	results, credit := fin.Allocate(charges, 5000, ruling)

	require.Len(t, results, 1)
	assert.Equal(t, chargeID, results[0].ChargeID)
	assert.Equal(t, int64(5000), results[0].AllocatedCents)
	assert.Equal(t, int64(0), credit)
}

func TestAllocate_Overpayment(t *testing.T) {
	chargeID := uuid.New()
	charges := []fin.OutstandingCharge{
		{
			ID:          chargeID,
			ChargeType:  fin.ChargeTypeRegularAssessment,
			AmountCents: 5000,
			DueDate:     time.Now().Add(-30 * 24 * time.Hour),
		},
	}
	ruling := fin.AllocationRuling{
		PriorityOrder: []fin.ChargeType{fin.ChargeTypeRegularAssessment},
		AcceptPartial: true,
	}

	results, credit := fin.Allocate(charges, 8000, ruling)

	require.Len(t, results, 1)
	assert.Equal(t, chargeID, results[0].ChargeID)
	assert.Equal(t, int64(5000), results[0].AllocatedCents)
	assert.Equal(t, int64(3000), credit)
}

func TestAllocate_PriorityOrder(t *testing.T) {
	lateFeeID := uuid.New()
	assessmentID := uuid.New()

	charges := []fin.OutstandingCharge{
		{
			ID:          lateFeeID,
			ChargeType:  fin.ChargeTypeLateFee,
			AmountCents: 2000,
			DueDate:     time.Now().Add(-30 * 24 * time.Hour),
		},
		{
			ID:          assessmentID,
			ChargeType:  fin.ChargeTypeRegularAssessment,
			AmountCents: 10000,
			DueDate:     time.Now().Add(-30 * 24 * time.Hour),
		},
	}
	// Assessments take priority over late fees.
	ruling := fin.AllocationRuling{
		PriorityOrder: []fin.ChargeType{
			fin.ChargeTypeRegularAssessment,
			fin.ChargeTypeLateFee,
		},
		AcceptPartial: true,
	}

	results, credit := fin.Allocate(charges, 11000, ruling)

	require.Len(t, results, 2)
	// First allocation must be the assessment.
	assert.Equal(t, assessmentID, results[0].ChargeID)
	assert.Equal(t, int64(10000), results[0].AllocatedCents)
	// Second allocation is the late fee, partially covered.
	assert.Equal(t, lateFeeID, results[1].ChargeID)
	assert.Equal(t, int64(1000), results[1].AllocatedCents)
	assert.Equal(t, int64(0), credit)
}

func TestAllocate_FIFOWithinTier(t *testing.T) {
	olderID := uuid.New()
	newerID := uuid.New()
	now := time.Now()

	charges := []fin.OutstandingCharge{
		{
			ID:          newerID,
			ChargeType:  fin.ChargeTypeRegularAssessment,
			AmountCents: 5000,
			DueDate:     now.Add(-30 * 24 * time.Hour),
		},
		{
			ID:          olderID,
			ChargeType:  fin.ChargeTypeRegularAssessment,
			AmountCents: 5000,
			DueDate:     now.Add(-60 * 24 * time.Hour),
		},
	}
	ruling := fin.AllocationRuling{
		PriorityOrder: []fin.ChargeType{fin.ChargeTypeRegularAssessment},
		AcceptPartial: true,
	}

	results, credit := fin.Allocate(charges, 7000, ruling)

	require.Len(t, results, 2)
	// Older charge should be allocated first (FIFO within same tier).
	assert.Equal(t, olderID, results[0].ChargeID)
	assert.Equal(t, int64(5000), results[0].AllocatedCents)
	// Newer charge gets the remainder.
	assert.Equal(t, newerID, results[1].ChargeID)
	assert.Equal(t, int64(2000), results[1].AllocatedCents)
	assert.Equal(t, int64(0), credit)
}

func TestAllocate_FrozenChargesExcluded(t *testing.T) {
	frozenID := uuid.New()
	activeID := uuid.New()

	charges := []fin.OutstandingCharge{
		{
			ID:          frozenID,
			ChargeType:  fin.ChargeTypeRegularAssessment,
			AmountCents: 5000,
			DueDate:     time.Now().Add(-60 * 24 * time.Hour),
		},
		{
			ID:          activeID,
			ChargeType:  fin.ChargeTypeRegularAssessment,
			AmountCents: 5000,
			DueDate:     time.Now().Add(-30 * 24 * time.Hour),
		},
	}
	ruling := fin.AllocationRuling{
		PriorityOrder:   []fin.ChargeType{fin.ChargeTypeRegularAssessment},
		FrozenChargeIDs: []uuid.UUID{frozenID},
		AcceptPartial:   true,
	}

	results, credit := fin.Allocate(charges, 8000, ruling)

	// Only the active charge should be allocated.
	require.Len(t, results, 1)
	assert.Equal(t, activeID, results[0].ChargeID)
	assert.Equal(t, int64(5000), results[0].AllocatedCents)
	assert.Equal(t, int64(3000), credit)
}

func TestAllocate_NoCharges(t *testing.T) {
	ruling := fin.AllocationRuling{
		PriorityOrder: []fin.ChargeType{fin.ChargeTypeRegularAssessment},
		AcceptPartial: true,
	}

	results, credit := fin.Allocate(nil, 5000, ruling)

	assert.Nil(t, results)
	assert.Equal(t, int64(5000), credit)
}

// ── ApplyStrategy tests ─────────────────────────────────────────────

func TestApplyStrategy_OldestFirst(t *testing.T) {
	now := time.Now()
	olderID := uuid.New()
	newerID := uuid.New()

	charges := []fin.OutstandingCharge{
		{
			ID:          newerID,
			ChargeType:  fin.ChargeTypeRegularAssessment,
			AmountCents: 5000,
			DueDate:     now.Add(-10 * 24 * time.Hour),
		},
		{
			ID:          olderID,
			ChargeType:  fin.ChargeTypeRegularAssessment,
			AmountCents: 5000,
			DueDate:     now.Add(-60 * 24 * time.Hour),
		},
	}

	strategy := &fin.ApplicationStrategy{
		Method:         fin.ApplicationMethodOldestFirst,
		WithinPriority: fin.SortOldestFirst,
	}

	results, credit := fin.ApplyStrategy(strategy, charges, 7000)

	require.Len(t, results, 2)
	// Older charge allocated first (FIFO by due date).
	assert.Equal(t, olderID, results[0].ChargeID)
	assert.Equal(t, int64(5000), results[0].AllocatedCents)
	// Newer charge gets the remainder.
	assert.Equal(t, newerID, results[1].ChargeID)
	assert.Equal(t, int64(2000), results[1].AllocatedCents)
	assert.Equal(t, int64(0), credit)
}

func TestApplyStrategy_PriorityFIFO(t *testing.T) {
	now := time.Now()
	assessmentID := uuid.New()
	lateFeeID := uuid.New()

	charges := []fin.OutstandingCharge{
		{
			ID:          lateFeeID,
			ChargeType:  fin.ChargeTypeLateFee,
			AmountCents: 2000,
			DueDate:     now.Add(-30 * 24 * time.Hour),
		},
		{
			ID:          assessmentID,
			ChargeType:  fin.ChargeTypeRegularAssessment,
			AmountCents: 10000,
			DueDate:     now.Add(-30 * 24 * time.Hour),
		},
	}

	strategy := &fin.ApplicationStrategy{
		Method: fin.ApplicationMethodPriorityFIFO,
		PriorityOrder: []fin.ChargeType{
			fin.ChargeTypeRegularAssessment,
			fin.ChargeTypeLateFee,
		},
		WithinPriority: fin.SortOldestFirst,
	}

	results, credit := fin.ApplyStrategy(strategy, charges, 11000)

	require.Len(t, results, 2)
	// Assessment first (higher priority tier).
	assert.Equal(t, assessmentID, results[0].ChargeID)
	assert.Equal(t, int64(10000), results[0].AllocatedCents)
	// Late fee second (lower priority tier, partial).
	assert.Equal(t, lateFeeID, results[1].ChargeID)
	assert.Equal(t, int64(1000), results[1].AllocatedCents)
	assert.Equal(t, int64(0), credit)
}
