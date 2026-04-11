package fin

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGaapEngine_VoidReversal_Assessment(t *testing.T) {
	engine, resolver := newTestGaapEngineWithResolver(RecognitionBasisAccrual)
	unitID := uuid.New()
	fundID := uuid.New()
	sourceID := uuid.New()

	tx := FinancialTransaction{
		Type:          TxTypeVoidReversal,
		OrgID:         uuid.New(),
		AmountCents:   25000,
		EffectiveDate: time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC),
		SourceID:      sourceID,
		UnitID:        &unitID,
		FundAllocations: []FundAllocation{
			{FundID: fundID, FundKey: "operating", AmountCents: 25000},
		},
		Memo: "Void assessment",
		Metadata: map[string]any{
			"original_type": "assessment",
		},
	}

	effects, err := engine.RecordTransaction(context.Background(), tx)
	require.NoError(t, err)
	require.NotNil(t, effects)

	// IsReversal must be true.
	assert.True(t, effects.IsReversal)

	// Original assessment (accrual): DR 1100 (AR) / CR 4010 (Revenue).
	// Reversal mirrors: CR 1100 (AR) / DR 4010 (Revenue).
	require.Len(t, effects.JournalLines, 2)

	// First line: original was DR 1100, reversal is CR 1100.
	assert.Equal(t, resolver.accounts[1100].ID, effects.JournalLines[0].AccountID)
	assert.Equal(t, int64(0), effects.JournalLines[0].DebitCents)
	assert.Equal(t, int64(25000), effects.JournalLines[0].CreditCents)

	// Second line: original was CR 4010, reversal is DR 4010.
	assert.Equal(t, resolver.accounts[4010].ID, effects.JournalLines[1].AccountID)
	assert.Equal(t, int64(25000), effects.JournalLines[1].DebitCents)
	assert.Equal(t, int64(0), effects.JournalLines[1].CreditCents)

	// Fund transaction: negated.
	require.Len(t, effects.FundTransactions, 1)
	assert.Equal(t, fundID, effects.FundTransactions[0].FundID)
	assert.Equal(t, int64(-25000), effects.FundTransactions[0].AmountCents)
	assert.Contains(t, effects.FundTransactions[0].Description, "Reversal")

	// Ledger entry: adjustment with negated amount.
	require.Len(t, effects.LedgerEntries, 1)
	assert.Equal(t, unitID, effects.LedgerEntries[0].UnitID)
	assert.Equal(t, LedgerEntryTypeAdjustment, effects.LedgerEntries[0].Type)
	assert.Equal(t, int64(-25000), effects.LedgerEntries[0].AmountCents)
	assert.Contains(t, effects.LedgerEntries[0].Description, "Reversal")

	// Verify GL lines are balanced.
	var totalDebits, totalCredits int64
	for _, line := range effects.JournalLines {
		totalDebits += line.DebitCents
		totalCredits += line.CreditCents
	}
	assert.Equal(t, totalDebits, totalCredits, "reversal journal entry must be balanced")
}

func TestGaapEngine_VoidReversal_Payment(t *testing.T) {
	engine, resolver := newTestGaapEngineWithResolver(RecognitionBasisAccrual)
	unitID := uuid.New()
	fundID := uuid.New()
	sourceID := uuid.New()

	tx := FinancialTransaction{
		Type:          TxTypeVoidReversal,
		OrgID:         uuid.New(),
		AmountCents:   15000,
		EffectiveDate: time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC),
		SourceID:      sourceID,
		UnitID:        &unitID,
		FundAllocations: []FundAllocation{
			{FundID: fundID, FundKey: "operating", AmountCents: 15000},
		},
		Memo: "Void payment",
		Metadata: map[string]any{
			"original_type": "payment",
		},
	}

	effects, err := engine.RecordTransaction(context.Background(), tx)
	require.NoError(t, err)
	require.NotNil(t, effects)

	// IsReversal must be true.
	assert.True(t, effects.IsReversal)

	// Original payment (accrual): DR 1010 (Cash) / CR 1100 (AR).
	// Reversal mirrors: CR 1010 (Cash) / DR 1100 (AR).
	require.Len(t, effects.JournalLines, 2)

	// First line: original was DR 1010, reversal is CR 1010.
	assert.Equal(t, resolver.accounts[1010].ID, effects.JournalLines[0].AccountID)
	assert.Equal(t, int64(0), effects.JournalLines[0].DebitCents)
	assert.Equal(t, int64(15000), effects.JournalLines[0].CreditCents)

	// Second line: original was CR 1100, reversal is DR 1100.
	assert.Equal(t, resolver.accounts[1100].ID, effects.JournalLines[1].AccountID)
	assert.Equal(t, int64(15000), effects.JournalLines[1].DebitCents)
	assert.Equal(t, int64(0), effects.JournalLines[1].CreditCents)

	// Fund transaction: negated.
	require.Len(t, effects.FundTransactions, 1)
	assert.Equal(t, fundID, effects.FundTransactions[0].FundID)
	assert.Equal(t, int64(-15000), effects.FundTransactions[0].AmountCents)

	// Ledger entry: adjustment with negated amount.
	require.Len(t, effects.LedgerEntries, 1)
	assert.Equal(t, unitID, effects.LedgerEntries[0].UnitID)
	assert.Equal(t, LedgerEntryTypeAdjustment, effects.LedgerEntries[0].Type)
	assert.Equal(t, int64(-15000), effects.LedgerEntries[0].AmountCents)

	// Verify GL lines are balanced.
	var totalDebits, totalCredits int64
	for _, line := range effects.JournalLines {
		totalDebits += line.DebitCents
		totalCredits += line.CreditCents
	}
	assert.Equal(t, totalDebits, totalCredits, "reversal journal entry must be balanced")
}

func TestGaapEngine_VoidReversal_MissingOriginalType(t *testing.T) {
	engine, _ := newTestGaapEngineWithResolver(RecognitionBasisAccrual)

	tx := FinancialTransaction{
		Type:     TxTypeVoidReversal,
		OrgID:    uuid.New(),
		SourceID: uuid.New(),
		Metadata: map[string]any{},
	}

	_, err := engine.RecordTransaction(context.Background(), tx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "original_type")
}

func TestGaapEngine_VoidReversal_CashBasis_Payment(t *testing.T) {
	engine, resolver := newTestGaapEngineWithResolver(RecognitionBasisCash)
	unitID := uuid.New()
	fundID := uuid.New()

	tx := FinancialTransaction{
		Type:          TxTypeVoidReversal,
		OrgID:         uuid.New(),
		AmountCents:   10000,
		EffectiveDate: time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC),
		SourceID:      uuid.New(),
		UnitID:        &unitID,
		FundAllocations: []FundAllocation{
			{FundID: fundID, FundKey: "operating", AmountCents: 10000},
		},
		Memo: "Void cash-basis payment",
		Metadata: map[string]any{
			"original_type": "payment",
		},
	}

	effects, err := engine.RecordTransaction(context.Background(), tx)
	require.NoError(t, err)
	require.NotNil(t, effects)

	assert.True(t, effects.IsReversal)

	// Cash-basis payment: DR 1010 (Cash) / CR 4010 (Revenue).
	// Reversal: CR 1010 / DR 4010.
	require.Len(t, effects.JournalLines, 2)
	assert.Equal(t, resolver.accounts[1010].ID, effects.JournalLines[0].AccountID)
	assert.Equal(t, int64(10000), effects.JournalLines[0].CreditCents)
	assert.Equal(t, resolver.accounts[4010].ID, effects.JournalLines[1].AccountID)
	assert.Equal(t, int64(10000), effects.JournalLines[1].DebitCents)

	// Verify balanced.
	var totalDebits, totalCredits int64
	for _, line := range effects.JournalLines {
		totalDebits += line.DebitCents
		totalCredits += line.CreditCents
	}
	assert.Equal(t, totalDebits, totalCredits)
}
