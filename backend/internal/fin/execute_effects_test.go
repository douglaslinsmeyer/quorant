package fin

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestExecuteEffects_DeferralSchedule_NoError(t *testing.T) {
	// Verify that a DeferralSchedule in the effects bundle is logged and
	// skipped rather than returning an error. No GL lines, fund txns,
	// ledger entries, or credits are present so no repos are called.
	svc := &FinService{
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	effects := &FinancialEffects{
		DeferralSchedule: &DeferralSchedule{
			DeferredAccountNumber: 2200,
			RevenueAccountNumber:  4010,
			TotalAmountCents:      120000,
			Entries: []DeferralEntry{
				{RecognitionDate: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), AmountCents: 10000},
				{RecognitionDate: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC), AmountCents: 10000},
			},
		},
	}

	err := svc.executeEffects(
		context.Background(),
		nil, // no UoW — unit-test path
		uuid.New(),
		GLSourceTypeAssessment,
		uuid.New(),
		nil, // no unitID
		time.Now(),
		"test deferral",
		effects,
	)

	assert.NoError(t, err, "DeferralSchedule should not cause an error")
}
