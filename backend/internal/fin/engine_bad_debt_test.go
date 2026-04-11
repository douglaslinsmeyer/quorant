package fin

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestGaapEngineWithBadDebtAccounts returns an engine and resolver that
// includes all accounts needed for bad debt tests: 1010, 1100, 1105, 5070.
func newTestGaapEngineWithBadDebtAccounts(basis RecognitionBasis) (*GaapEngine, *stubAccountResolver) {
	resolver := &stubAccountResolver{accounts: map[int]*GLAccount{
		1010: {ID: uuid.MustParse("00000000-0000-0000-0000-000000001010"), AccountNumber: 1010},
		1100: {ID: uuid.MustParse("00000000-0000-0000-0000-000000001100"), AccountNumber: 1100},
		1105: {ID: uuid.MustParse("00000000-0000-0000-0000-000000001105"), AccountNumber: 1105},
		5070: {ID: uuid.MustParse("00000000-0000-0000-0000-000000005070"), AccountNumber: 5070},
	}}
	engine := NewGaapEngine(resolver, nil, EngineConfig{RecognitionBasis: basis, FiscalYearStart: 1})
	return engine, resolver
}

// ── Bad Debt Provision tests ────────────────────────────────────────

func TestGaapEngine_RecordTransaction_BadDebtProvision(t *testing.T) {
	engine, resolver := newTestGaapEngineWithBadDebtAccounts(RecognitionBasisAccrual)

	tx := FinancialTransaction{
		Type:          TxTypeBadDebtProvision,
		OrgID:         uuid.New(),
		AmountCents:   50000,
		EffectiveDate: time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC),
		SourceID:      uuid.New(),
		Memo:          "Q1 bad debt provision",
	}

	effects, err := engine.RecordTransaction(context.Background(), tx)
	require.NoError(t, err)
	require.NotNil(t, effects)

	// GL: DR 5070 (Bad Debt Expense) / CR 1105 (Allowance).
	require.Len(t, effects.JournalLines, 2)
	assert.Equal(t, resolver.accounts[5070].ID, effects.JournalLines[0].AccountID)
	assert.Equal(t, int64(50000), effects.JournalLines[0].DebitCents)
	assert.Equal(t, int64(0), effects.JournalLines[0].CreditCents)
	assert.Equal(t, resolver.accounts[1105].ID, effects.JournalLines[1].AccountID)
	assert.Equal(t, int64(0), effects.JournalLines[1].DebitCents)
	assert.Equal(t, int64(50000), effects.JournalLines[1].CreditCents)

	// No ledger entries (provision is an estimate, not unit-specific).
	assert.Empty(t, effects.LedgerEntries)

	// No fund transactions.
	assert.Empty(t, effects.FundTransactions)
}

// ── Bad Debt Write-Off tests ────────────────────────────────────────

func TestGaapEngine_RecordTransaction_BadDebtWriteOff(t *testing.T) {
	engine, resolver := newTestGaapEngineWithBadDebtAccounts(RecognitionBasisAccrual)
	unitID := uuid.New()

	tx := FinancialTransaction{
		Type:          TxTypeBadDebtWriteOff,
		OrgID:         uuid.New(),
		AmountCents:   12000,
		EffectiveDate: time.Date(2026, 4, 5, 0, 0, 0, 0, time.UTC),
		SourceID:      uuid.New(),
		UnitID:        &unitID,
		Memo:          "Write off unit 42",
	}

	effects, err := engine.RecordTransaction(context.Background(), tx)
	require.NoError(t, err)
	require.NotNil(t, effects)

	// GL: DR 1105 (Allowance) / CR 1100 (AR).
	require.Len(t, effects.JournalLines, 2)
	assert.Equal(t, resolver.accounts[1105].ID, effects.JournalLines[0].AccountID)
	assert.Equal(t, int64(12000), effects.JournalLines[0].DebitCents)
	assert.Equal(t, int64(0), effects.JournalLines[0].CreditCents)
	assert.Equal(t, resolver.accounts[1100].ID, effects.JournalLines[1].AccountID)
	assert.Equal(t, int64(0), effects.JournalLines[1].DebitCents)
	assert.Equal(t, int64(12000), effects.JournalLines[1].CreditCents)

	// Ledger: adjustment entry on the unit.
	require.Len(t, effects.LedgerEntries, 1)
	assert.Equal(t, unitID, effects.LedgerEntries[0].UnitID)
	assert.Equal(t, LedgerEntryTypeAdjustment, effects.LedgerEntries[0].Type)
	assert.Equal(t, int64(12000), effects.LedgerEntries[0].AmountCents)
	assert.Equal(t, tx.SourceID, effects.LedgerEntries[0].SourceID)
	assert.Equal(t, "Write off unit 42", effects.LedgerEntries[0].Description)

	// No fund transactions.
	assert.Empty(t, effects.FundTransactions)
}

// ── Bad Debt Recovery tests ─────────────────────────────────────────

func TestGaapEngine_RecordTransaction_BadDebtRecovery(t *testing.T) {
	engine, resolver := newTestGaapEngineWithBadDebtAccounts(RecognitionBasisAccrual)
	unitID := uuid.New()

	tx := FinancialTransaction{
		Type:          TxTypeBadDebtRecovery,
		OrgID:         uuid.New(),
		AmountCents:   8000,
		EffectiveDate: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
		SourceID:      uuid.New(),
		UnitID:        &unitID,
		Memo:          "Recovered from unit 42",
	}

	effects, err := engine.RecordTransaction(context.Background(), tx)
	require.NoError(t, err)
	require.NotNil(t, effects)

	// GL: 4 lines total.
	// Step 1: DR 1100 (AR) / CR 1105 (Allowance) -- reinstate receivable.
	// Step 2: DR 1010 (Cash) / CR 1100 (AR) -- record payment.
	require.Len(t, effects.JournalLines, 4)

	// Step 1: reinstatement.
	assert.Equal(t, resolver.accounts[1100].ID, effects.JournalLines[0].AccountID)
	assert.Equal(t, int64(8000), effects.JournalLines[0].DebitCents)
	assert.Equal(t, int64(0), effects.JournalLines[0].CreditCents)
	assert.Equal(t, resolver.accounts[1105].ID, effects.JournalLines[1].AccountID)
	assert.Equal(t, int64(0), effects.JournalLines[1].DebitCents)
	assert.Equal(t, int64(8000), effects.JournalLines[1].CreditCents)

	// Step 2: cash receipt.
	assert.Equal(t, resolver.accounts[1010].ID, effects.JournalLines[2].AccountID)
	assert.Equal(t, int64(8000), effects.JournalLines[2].DebitCents)
	assert.Equal(t, int64(0), effects.JournalLines[2].CreditCents)
	assert.Equal(t, resolver.accounts[1100].ID, effects.JournalLines[3].AccountID)
	assert.Equal(t, int64(0), effects.JournalLines[3].DebitCents)
	assert.Equal(t, int64(8000), effects.JournalLines[3].CreditCents)

	// Ledger: 2 entries -- reinstatement (adjustment) + payment.
	require.Len(t, effects.LedgerEntries, 2)

	assert.Equal(t, unitID, effects.LedgerEntries[0].UnitID)
	assert.Equal(t, LedgerEntryTypeAdjustment, effects.LedgerEntries[0].Type)
	assert.Equal(t, int64(8000), effects.LedgerEntries[0].AmountCents)
	assert.Equal(t, tx.SourceID, effects.LedgerEntries[0].SourceID)

	assert.Equal(t, unitID, effects.LedgerEntries[1].UnitID)
	assert.Equal(t, LedgerEntryTypePayment, effects.LedgerEntries[1].Type)
	assert.Equal(t, int64(8000), effects.LedgerEntries[1].AmountCents)
	assert.Equal(t, tx.SourceID, effects.LedgerEntries[1].SourceID)

	// No fund transactions.
	assert.Empty(t, effects.FundTransactions)
}
