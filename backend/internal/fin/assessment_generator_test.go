package fin

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── isDueThisPeriod tests ────────────────────────────────────────────

func TestIsDueThisPeriod_Monthly_OnDay(t *testing.T) {
	sched := AssessmentSchedule{
		Frequency:  AssessmentFreqMonthly,
		DayOfMonth: helperIntPtr(1),
		StartsAt:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		IsActive:   true,
	}
	// Running on the 1st of any month should be due.
	now := time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC)
	assert.True(t, IsDueThisPeriod(sched, now))
}

func TestIsDueThisPeriod_Monthly_WrongDay(t *testing.T) {
	sched := AssessmentSchedule{
		Frequency:  AssessmentFreqMonthly,
		DayOfMonth: helperIntPtr(15),
		StartsAt:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		IsActive:   true,
	}
	now := time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC)
	assert.False(t, IsDueThisPeriod(sched, now))
}

func TestIsDueThisPeriod_Quarterly_DueMonth(t *testing.T) {
	sched := AssessmentSchedule{
		Frequency:  AssessmentFreqQuarterly,
		DayOfMonth: helperIntPtr(1),
		StartsAt:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		IsActive:   true,
	}
	// Jan start, quarterly = Jan, Apr, Jul, Oct
	now := time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC)
	assert.True(t, IsDueThisPeriod(sched, now))
}

func TestIsDueThisPeriod_Quarterly_NotDueMonth(t *testing.T) {
	sched := AssessmentSchedule{
		Frequency:  AssessmentFreqQuarterly,
		DayOfMonth: helperIntPtr(1),
		StartsAt:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		IsActive:   true,
	}
	now := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC) // March, not a quarter boundary
	assert.False(t, IsDueThisPeriod(sched, now))
}

func TestIsDueThisPeriod_SemiAnnually(t *testing.T) {
	sched := AssessmentSchedule{
		Frequency:  AssessmentFreqSemiAnnually,
		DayOfMonth: helperIntPtr(1),
		StartsAt:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		IsActive:   true,
	}
	assert.True(t, IsDueThisPeriod(sched, time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)))
	assert.False(t, IsDueThisPeriod(sched, time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)))
}

func TestIsDueThisPeriod_Annually(t *testing.T) {
	sched := AssessmentSchedule{
		Frequency:  AssessmentFreqAnnually,
		DayOfMonth: helperIntPtr(1),
		StartsAt:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		IsActive:   true,
	}
	assert.True(t, IsDueThisPeriod(sched, time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)))
	assert.False(t, IsDueThisPeriod(sched, time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)))
}

func TestIsDueThisPeriod_BeforeStart(t *testing.T) {
	sched := AssessmentSchedule{
		Frequency:  AssessmentFreqMonthly,
		DayOfMonth: helperIntPtr(1),
		StartsAt:   time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		IsActive:   true,
	}
	now := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	assert.False(t, IsDueThisPeriod(sched, now))
}

func TestIsDueThisPeriod_AfterEnd(t *testing.T) {
	end := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	sched := AssessmentSchedule{
		Frequency:  AssessmentFreqMonthly,
		DayOfMonth: helperIntPtr(1),
		StartsAt:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		EndsAt:     &end,
		IsActive:   true,
	}
	now := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	assert.False(t, IsDueThisPeriod(sched, now))
}

func helperIntPtr(v int) *int { return &v }

// ── resolveAssessmentAmount tests ─────────────────────────────────────

func TestResolveAssessmentAmount_NoPolicy_UsesBaseAmount(t *testing.T) {
	amount, err := resolveAssessmentAmount(context.Background(), nil, uuid.New(), AssessmentAmountContext{
		BaseAmountCents: 25000,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(25000), amount)
}

func TestResolveAssessmentAmount_FlatPolicy(t *testing.T) {
	amountCents := int64(30000)
	ruling, _ := json.Marshal(AssessmentAmountRuling{
		Strategy:    "flat",
		AmountCents: &amountCents,
	})
	registry := newRegistryWithCap(t, "assessment_amount_strategy", ruling)

	amount, err := resolveAssessmentAmount(context.Background(), registry, uuid.New(), AssessmentAmountContext{
		BaseAmountCents: 25000,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(30000), amount)
}
