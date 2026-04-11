package fin

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestGaapEngineWithClosingAccounts returns an engine and resolver that
// includes all accounts needed for year-end closing tests.
func newTestGaapEngineWithClosingAccounts() (*GaapEngine, *stubAccountResolver) {
	resolver := &stubAccountResolver{accounts: map[int]*GLAccount{
		3010: {ID: uuid.MustParse("00000000-0000-0000-0000-000000003010"), AccountNumber: 3010},
		3100: {ID: uuid.MustParse("00000000-0000-0000-0000-000000003100"), AccountNumber: 3100},
		3110: {ID: uuid.MustParse("00000000-0000-0000-0000-000000003110"), AccountNumber: 3110},
		4010: {ID: uuid.MustParse("00000000-0000-0000-0000-000000004010"), AccountNumber: 4010},
		5010: {ID: uuid.MustParse("00000000-0000-0000-0000-000000005010"), AccountNumber: 5010},
	}}
	engine := NewGaapEngine(resolver, nil, EngineConfig{RecognitionBasis: RecognitionBasisAccrual, FiscalYearStart: 1})
	return engine, resolver
}

func TestGaapEngine_YearEndClose_NetIncome(t *testing.T) {
	engine, resolver := newTestGaapEngineWithClosingAccounts()

	tx := FinancialTransaction{
		Type:          TxTypeYearEndClose,
		OrgID:         uuid.New(),
		EffectiveDate: time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC),
		SourceID:      uuid.New(),
		Metadata: map[string]any{
			"fund_key":             "operating",
			"fund_balance_account": float64(3010),
			"account_balances": []any{
				map[string]any{"account_number": float64(4010), "balance_cents": float64(120000), "account_type": "revenue"},
				map[string]any{"account_number": float64(5010), "balance_cents": float64(80000), "account_type": "expense"},
			},
		},
	}

	effects, err := engine.RecordTransaction(context.Background(), tx)
	require.NoError(t, err)
	require.NotNil(t, effects)

	// Expect 3 lines: DR Revenue 120k, CR Expense 80k, CR Fund Balance 40k.
	require.Len(t, effects.JournalLines, 3)

	// DR 4010 (Revenue) to close.
	assert.Equal(t, resolver.accounts[4010].ID, effects.JournalLines[0].AccountID)
	assert.Equal(t, int64(120000), effects.JournalLines[0].DebitCents)
	assert.Equal(t, int64(0), effects.JournalLines[0].CreditCents)

	// CR 5010 (Expense) to close.
	assert.Equal(t, resolver.accounts[5010].ID, effects.JournalLines[1].AccountID)
	assert.Equal(t, int64(0), effects.JournalLines[1].DebitCents)
	assert.Equal(t, int64(80000), effects.JournalLines[1].CreditCents)

	// CR 3010 (Fund Balance) for net income of 40k.
	assert.Equal(t, resolver.accounts[3010].ID, effects.JournalLines[2].AccountID)
	assert.Equal(t, int64(0), effects.JournalLines[2].DebitCents)
	assert.Equal(t, int64(40000), effects.JournalLines[2].CreditCents)

	// Verify balanced: total debits == total credits.
	var totalDebits, totalCredits int64
	for _, line := range effects.JournalLines {
		totalDebits += line.DebitCents
		totalCredits += line.CreditCents
	}
	assert.Equal(t, totalDebits, totalCredits, "journal entry must be balanced")
}

func TestGaapEngine_YearEndClose_NetLoss(t *testing.T) {
	engine, resolver := newTestGaapEngineWithClosingAccounts()

	tx := FinancialTransaction{
		Type:          TxTypeYearEndClose,
		OrgID:         uuid.New(),
		EffectiveDate: time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC),
		SourceID:      uuid.New(),
		Metadata: map[string]any{
			"fund_key":             "operating",
			"fund_balance_account": float64(3010),
			"account_balances": []any{
				map[string]any{"account_number": float64(4010), "balance_cents": float64(50000), "account_type": "revenue"},
				map[string]any{"account_number": float64(5010), "balance_cents": float64(80000), "account_type": "expense"},
			},
		},
	}

	effects, err := engine.RecordTransaction(context.Background(), tx)
	require.NoError(t, err)
	require.NotNil(t, effects)

	// Expect 3 lines: DR Revenue 50k, CR Expense 80k, DR Fund Balance 30k.
	require.Len(t, effects.JournalLines, 3)

	// DR 4010 (Revenue) to close.
	assert.Equal(t, resolver.accounts[4010].ID, effects.JournalLines[0].AccountID)
	assert.Equal(t, int64(50000), effects.JournalLines[0].DebitCents)

	// CR 5010 (Expense) to close.
	assert.Equal(t, resolver.accounts[5010].ID, effects.JournalLines[1].AccountID)
	assert.Equal(t, int64(80000), effects.JournalLines[1].CreditCents)

	// DR 3010 (Fund Balance) for net loss of 30k.
	assert.Equal(t, resolver.accounts[3010].ID, effects.JournalLines[2].AccountID)
	assert.Equal(t, int64(30000), effects.JournalLines[2].DebitCents)
	assert.Equal(t, int64(0), effects.JournalLines[2].CreditCents)

	// Verify balanced.
	var totalDebits, totalCredits int64
	for _, line := range effects.JournalLines {
		totalDebits += line.DebitCents
		totalCredits += line.CreditCents
	}
	assert.Equal(t, totalDebits, totalCredits, "journal entry must be balanced")
}

func TestGaapEngine_YearEndClose_WithInterfundAccounts(t *testing.T) {
	engine, resolver := newTestGaapEngineWithClosingAccounts()

	tx := FinancialTransaction{
		Type:          TxTypeYearEndClose,
		OrgID:         uuid.New(),
		EffectiveDate: time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC),
		SourceID:      uuid.New(),
		Metadata: map[string]any{
			"fund_key":             "operating",
			"fund_balance_account": float64(3010),
			"account_balances": []any{
				map[string]any{"account_number": float64(4010), "balance_cents": float64(120000), "account_type": "revenue"},
				map[string]any{"account_number": float64(5010), "balance_cents": float64(80000), "account_type": "expense"},
				map[string]any{"account_number": float64(3100), "balance_cents": float64(10000), "account_type": "equity"},
				map[string]any{"account_number": float64(3110), "balance_cents": float64(10000), "account_type": "equity"},
			},
		},
	}

	effects, err := engine.RecordTransaction(context.Background(), tx)
	require.NoError(t, err)
	require.NotNil(t, effects)

	// Expect 5 lines:
	// DR Revenue 120k, CR Expense 80k, CR 3100 10k, DR 3110 10k, CR Fund Balance 40k.
	require.Len(t, effects.JournalLines, 5)

	// DR 4010 (Revenue).
	assert.Equal(t, resolver.accounts[4010].ID, effects.JournalLines[0].AccountID)
	assert.Equal(t, int64(120000), effects.JournalLines[0].DebitCents)

	// CR 5010 (Expense).
	assert.Equal(t, resolver.accounts[5010].ID, effects.JournalLines[1].AccountID)
	assert.Equal(t, int64(80000), effects.JournalLines[1].CreditCents)

	// CR 3100 (Interfund Transfer Out) — has debit balance, CR to close.
	assert.Equal(t, resolver.accounts[3100].ID, effects.JournalLines[2].AccountID)
	assert.Equal(t, int64(10000), effects.JournalLines[2].CreditCents)

	// DR 3110 (Interfund Transfer In) — has credit balance, DR to close.
	assert.Equal(t, resolver.accounts[3110].ID, effects.JournalLines[3].AccountID)
	assert.Equal(t, int64(10000), effects.JournalLines[3].DebitCents)

	// CR 3010 (Fund Balance) — net income: 120k - 80k - 10k + 10k = 40k.
	assert.Equal(t, resolver.accounts[3010].ID, effects.JournalLines[4].AccountID)
	assert.Equal(t, int64(40000), effects.JournalLines[4].CreditCents)

	// Verify balanced.
	var totalDebits, totalCredits int64
	for _, line := range effects.JournalLines {
		totalDebits += line.DebitCents
		totalCredits += line.CreditCents
	}
	assert.Equal(t, totalDebits, totalCredits, "journal entry must be balanced")
}

func TestGaapEngine_YearEndClose_Balanced(t *testing.T) {
	engine, resolver := newTestGaapEngineWithClosingAccounts()

	tx := FinancialTransaction{
		Type:          TxTypeYearEndClose,
		OrgID:         uuid.New(),
		EffectiveDate: time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC),
		SourceID:      uuid.New(),
		Metadata: map[string]any{
			"fund_key":             "operating",
			"fund_balance_account": float64(3010),
			"account_balances": []any{
				map[string]any{"account_number": float64(4010), "balance_cents": float64(100000), "account_type": "revenue"},
				map[string]any{"account_number": float64(5010), "balance_cents": float64(100000), "account_type": "expense"},
			},
		},
	}

	effects, err := engine.RecordTransaction(context.Background(), tx)
	require.NoError(t, err)
	require.NotNil(t, effects)

	// Expect 2 lines: DR Revenue 100k, CR Expense 100k. No fund balance entry
	// because net is zero.
	require.Len(t, effects.JournalLines, 2)

	assert.Equal(t, resolver.accounts[4010].ID, effects.JournalLines[0].AccountID)
	assert.Equal(t, int64(100000), effects.JournalLines[0].DebitCents)

	assert.Equal(t, resolver.accounts[5010].ID, effects.JournalLines[1].AccountID)
	assert.Equal(t, int64(100000), effects.JournalLines[1].CreditCents)

	// Verify balanced.
	var totalDebits, totalCredits int64
	for _, line := range effects.JournalLines {
		totalDebits += line.DebitCents
		totalCredits += line.CreditCents
	}
	assert.Equal(t, totalDebits, totalCredits, "journal entry must be balanced")
}

func TestGaapEngine_YearEndClose_MissingMetadata(t *testing.T) {
	engine, _ := newTestGaapEngineWithClosingAccounts()

	t.Run("missing fund_balance_account", func(t *testing.T) {
		tx := FinancialTransaction{
			Type:     TxTypeYearEndClose,
			OrgID:    uuid.New(),
			SourceID: uuid.New(),
			Metadata: map[string]any{
				"account_balances": []any{},
			},
		}
		_, err := engine.RecordTransaction(context.Background(), tx)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "fund_balance_account")
	})

	t.Run("missing account_balances", func(t *testing.T) {
		tx := FinancialTransaction{
			Type:     TxTypeYearEndClose,
			OrgID:    uuid.New(),
			SourceID: uuid.New(),
			Metadata: map[string]any{
				"fund_balance_account": float64(3010),
			},
		}
		_, err := engine.RecordTransaction(context.Background(), tx)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "account_balances")
	})
}
