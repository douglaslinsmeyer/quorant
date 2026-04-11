package fin

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestGaapEngine() *GaapEngine {
	return NewGaapEngine(nil, nil, EngineConfig{
		RecognitionBasis: RecognitionBasisAccrual,
		FiscalYearStart:  1,
	})
}

func TestGaapEngine_Standard(t *testing.T) {
	engine := newTestGaapEngine()
	assert.Equal(t, AccountingStandardGAAP, engine.Standard())
}

func TestGaapEngine_ChartOfAccounts_Count(t *testing.T) {
	engine := newTestGaapEngine()
	chart := engine.ChartOfAccounts()
	// Count should be 56 (5 headers + 51 detail accounts).
	require.NotEmpty(t, chart)
	t.Logf("Chart has %d accounts", len(chart))

	headers := 0
	detail := 0
	for _, a := range chart {
		if a.IsHeader {
			headers++
		} else {
			detail++
		}
		// All accounts must be system accounts.
		assert.True(t, a.IsSystem, "account %d %s should be system", a.Number, a.Name)
		// All detail accounts must have a parent.
		if !a.IsHeader {
			assert.NotZero(t, a.ParentNum, "detail account %d %s must have parent", a.Number, a.Name)
		}
	}
	assert.Equal(t, 5, headers, "should have 5 header accounts")
	assert.Equal(t, 51, detail, "should have 51 detail accounts")
}

func TestGaapEngine_ChartOfAccounts_FundCashAccounts(t *testing.T) {
	engine := newTestGaapEngine()
	chart := engine.ChartOfAccounts()
	byNumber := make(map[int]GLAccountSeed)
	for _, a := range chart {
		byNumber[a.Number] = a
	}

	// Each fund type must have its own cash account.
	assert.Equal(t, "operating", byNumber[1010].FundKey)
	assert.Equal(t, "reserve", byNumber[1020].FundKey)
	assert.Equal(t, "capital", byNumber[1030].FundKey)
	assert.Equal(t, "special", byNumber[1040].FundKey)
}

func TestGaapEngine_ChartOfAccounts_FundBalanceAccounts(t *testing.T) {
	engine := newTestGaapEngine()
	chart := engine.ChartOfAccounts()
	byNumber := make(map[int]GLAccountSeed)
	for _, a := range chart {
		byNumber[a.Number] = a
	}

	assert.Equal(t, "operating", byNumber[3010].FundKey)
	assert.Equal(t, "reserve", byNumber[3020].FundKey)
	assert.Equal(t, "capital", byNumber[3030].FundKey)
	assert.Equal(t, "special", byNumber[3040].FundKey)
}

func TestGaapEngine_ChartOfAccounts_RevenuePerFund(t *testing.T) {
	engine := newTestGaapEngine()
	chart := engine.ChartOfAccounts()
	byNumber := make(map[int]GLAccountSeed)
	for _, a := range chart {
		byNumber[a.Number] = a
	}

	assert.Equal(t, "operating", byNumber[4010].FundKey)
	assert.Equal(t, "reserve", byNumber[4020].FundKey)
	assert.Equal(t, "capital", byNumber[4030].FundKey)
	assert.Equal(t, "special", byNumber[4040].FundKey)
}

func TestGaapEngine_ChartOfAccounts_KeyAccounts(t *testing.T) {
	engine := newTestGaapEngine()
	chart := engine.ChartOfAccounts()
	byNumber := make(map[int]GLAccountSeed)
	for _, a := range chart {
		byNumber[a.Number] = a
	}

	// Verify key accounts exist with correct types.
	assert.Equal(t, "asset", byNumber[1100].Type, "AR should be asset")
	assert.Equal(t, "asset", byNumber[1105].Type, "Allowance should be asset (contra)")
	assert.Equal(t, "liability", byNumber[2100].Type, "AP should be liability")
	assert.Equal(t, "liability", byNumber[2200].Type, "Prepaid Assessments should be liability")
	assert.Equal(t, "equity", byNumber[3100].Type, "Interfund Transfer Out should be equity")
	assert.Equal(t, "equity", byNumber[3110].Type, "Interfund Transfer In should be equity")
	assert.Equal(t, "asset", byNumber[1300].Type, "Due From Other Funds should be asset")
	assert.Equal(t, "liability", byNumber[2500].Type, "Due To Other Funds should be liability")
	assert.Equal(t, "asset", byNumber[1400].Type, "Fixed Assets should be asset")
	assert.Equal(t, "expense", byNumber[5070].Type, "Bad Debt Expense")
	assert.Equal(t, "expense", byNumber[5220].Type, "Depreciation Expense")
	assert.Equal(t, "revenue", byNumber[4400].Type, "Insurance Proceeds should be revenue")
}

func TestGaapEngine_ChartOfAccounts_NoDuplicateNumbers(t *testing.T) {
	engine := newTestGaapEngine()
	chart := engine.ChartOfAccounts()
	seen := make(map[int]bool)
	for _, a := range chart {
		assert.False(t, seen[a.Number], "duplicate account number %d", a.Number)
		seen[a.Number] = true
	}
}

func ptr[T any](v T) *T { return &v }

func TestGaapEngine_ValidateTransaction(t *testing.T) {
	engine := newTestGaapEngine()

	t.Run("valid assessment", func(t *testing.T) {
		tx := FinancialTransaction{
			Type: TxTypeAssessment, OrgID: uuid.New(), AmountCents: 10000,
			EffectiveDate: time.Now(), SourceID: uuid.New(), UnitID: ptr(uuid.New()),
			FundAllocations: []FundAllocation{{FundID: uuid.New(), FundKey: "operating", AmountCents: 10000}},
		}
		assert.NoError(t, engine.ValidateTransaction(context.Background(), tx))
	})

	t.Run("negative amount rejected", func(t *testing.T) {
		tx := FinancialTransaction{
			Type: TxTypeAssessment, OrgID: uuid.New(), AmountCents: -100,
			SourceID: uuid.New(), UnitID: ptr(uuid.New()),
		}
		err := engine.ValidateTransaction(context.Background(), tx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "positive")
	})

	t.Run("assessment requires UnitID", func(t *testing.T) {
		tx := FinancialTransaction{
			Type: TxTypeAssessment, OrgID: uuid.New(), AmountCents: 10000,
			SourceID: uuid.New(),
		}
		err := engine.ValidateTransaction(context.Background(), tx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unit_id")
	})

	t.Run("fund transfer requires 2 FundAllocations", func(t *testing.T) {
		tx := FinancialTransaction{
			Type: TxTypeFundTransfer, OrgID: uuid.New(), AmountCents: 5000,
			SourceID: uuid.New(),
		}
		err := engine.ValidateTransaction(context.Background(), tx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "fund_allocations")
	})

	t.Run("adjusting entry allows zero amount", func(t *testing.T) {
		tx := FinancialTransaction{
			Type: TxTypeAdjustingEntry, OrgID: uuid.New(), AmountCents: 0,
			SourceID: uuid.New(),
		}
		assert.NoError(t, engine.ValidateTransaction(context.Background(), tx))
	})

	t.Run("payment requires UnitID", func(t *testing.T) {
		tx := FinancialTransaction{
			Type: TxTypePayment, OrgID: uuid.New(), AmountCents: 5000,
			SourceID: uuid.New(),
		}
		err := engine.ValidateTransaction(context.Background(), tx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unit_id")
	})
}

// stubAccountResolver provides in-memory account lookups for tests.
type stubAccountResolver struct {
	accounts map[int]*GLAccount
}

func (s *stubAccountResolver) FindAccountByOrgAndNumber(_ context.Context, _ uuid.UUID, number int) (*GLAccount, error) {
	a, ok := s.accounts[number]
	if !ok {
		return nil, fmt.Errorf("account %d not found", number)
	}
	return a, nil
}

func newTestGaapEngineWithResolver(basis RecognitionBasis) (*GaapEngine, *stubAccountResolver) {
	resolver := &stubAccountResolver{accounts: map[int]*GLAccount{
		1010: {ID: uuid.MustParse("00000000-0000-0000-0000-000000001010"), AccountNumber: 1010},
		1020: {ID: uuid.MustParse("00000000-0000-0000-0000-000000001020"), AccountNumber: 1020},
		1030: {ID: uuid.MustParse("00000000-0000-0000-0000-000000001030"), AccountNumber: 1030},
		1040: {ID: uuid.MustParse("00000000-0000-0000-0000-000000001040"), AccountNumber: 1040},
		1100: {ID: uuid.MustParse("00000000-0000-0000-0000-000000001100"), AccountNumber: 1100},
		4010: {ID: uuid.MustParse("00000000-0000-0000-0000-000000004010"), AccountNumber: 4010},
		4020: {ID: uuid.MustParse("00000000-0000-0000-0000-000000004020"), AccountNumber: 4020},
		4030: {ID: uuid.MustParse("00000000-0000-0000-0000-000000004030"), AccountNumber: 4030},
		4040: {ID: uuid.MustParse("00000000-0000-0000-0000-000000004040"), AccountNumber: 4040},
		4100: {ID: uuid.MustParse("00000000-0000-0000-0000-000000004100"), AccountNumber: 4100},
		4200: {ID: uuid.MustParse("00000000-0000-0000-0000-000000004200"), AccountNumber: 4200},
		2100: {ID: uuid.MustParse("00000000-0000-0000-0000-000000002100"), AccountNumber: 2100},
		5010: {ID: uuid.MustParse("00000000-0000-0000-0000-000000005010"), AccountNumber: 5010},
		5040: {ID: uuid.MustParse("00000000-0000-0000-0000-000000005040"), AccountNumber: 5040},
		3100: {ID: uuid.MustParse("00000000-0000-0000-0000-000000003100"), AccountNumber: 3100},
		3110: {ID: uuid.MustParse("00000000-0000-0000-0000-000000003110"), AccountNumber: 3110},
	}}
	engine := NewGaapEngine(resolver, nil, EngineConfig{RecognitionBasis: basis, FiscalYearStart: 1})
	return engine, resolver
}

func TestGaapEngine_RecordTransaction_Assessment_Accrual(t *testing.T) {
	engine, resolver := newTestGaapEngineWithResolver(RecognitionBasisAccrual)
	unitID := uuid.New()
	fundID := uuid.New()

	tx := FinancialTransaction{
		Type: TxTypeAssessment, OrgID: uuid.New(), AmountCents: 25000,
		EffectiveDate: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
		SourceID: uuid.New(), UnitID: &unitID,
		FundAllocations: []FundAllocation{{FundID: fundID, FundKey: "operating", AmountCents: 25000}},
		Memo:            "Monthly assessment",
	}

	effects, err := engine.RecordTransaction(context.Background(), tx)
	require.NoError(t, err)
	require.NotNil(t, effects)

	// GL: DR 1100 (AR) / CR 4010 (Revenue).
	require.Len(t, effects.JournalLines, 2)
	assert.Equal(t, resolver.accounts[1100].ID, effects.JournalLines[0].AccountID)
	assert.Equal(t, int64(25000), effects.JournalLines[0].DebitCents)
	assert.Equal(t, int64(0), effects.JournalLines[0].CreditCents)
	assert.Equal(t, resolver.accounts[4010].ID, effects.JournalLines[1].AccountID)
	assert.Equal(t, int64(0), effects.JournalLines[1].DebitCents)
	assert.Equal(t, int64(25000), effects.JournalLines[1].CreditCents)

	// Fund: credit to fund.
	require.Len(t, effects.FundTransactions, 1)
	assert.Equal(t, fundID, effects.FundTransactions[0].FundID)
	assert.Equal(t, int64(25000), effects.FundTransactions[0].AmountCents)

	// Ledger: charge entry.
	require.Len(t, effects.LedgerEntries, 1)
	assert.Equal(t, unitID, effects.LedgerEntries[0].UnitID)
	assert.Equal(t, LedgerEntryTypeCharge, effects.LedgerEntries[0].Type)
	assert.Equal(t, int64(25000), effects.LedgerEntries[0].AmountCents)
	assert.Equal(t, tx.SourceID, effects.LedgerEntries[0].SourceID)
}

func TestGaapEngine_RecordTransaction_Assessment_CashBasis(t *testing.T) {
	engine, _ := newTestGaapEngineWithResolver(RecognitionBasisCash)
	unitID := uuid.New()

	tx := FinancialTransaction{
		Type: TxTypeAssessment, OrgID: uuid.New(), AmountCents: 25000,
		EffectiveDate: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
		SourceID: uuid.New(), UnitID: &unitID,
		FundAllocations: []FundAllocation{{FundID: uuid.New(), FundKey: "operating", AmountCents: 25000}},
	}

	effects, err := engine.RecordTransaction(context.Background(), tx)
	require.NoError(t, err)

	// Cash basis: no GL, no fund transactions. Ledger only.
	assert.Empty(t, effects.JournalLines)
	assert.Empty(t, effects.FundTransactions)
	require.Len(t, effects.LedgerEntries, 1)
	assert.Equal(t, LedgerEntryTypeCharge, effects.LedgerEntries[0].Type)
}

func TestGaapEngine_RecordTransaction_Assessment_SplitFund(t *testing.T) {
	engine, resolver := newTestGaapEngineWithResolver(RecognitionBasisAccrual)
	unitID := uuid.New()
	opFundID := uuid.New()
	resFundID := uuid.New()

	tx := FinancialTransaction{
		Type: TxTypeAssessment, OrgID: uuid.New(), AmountCents: 25000,
		EffectiveDate: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
		SourceID: uuid.New(), UnitID: &unitID,
		FundAllocations: []FundAllocation{
			{FundID: opFundID, FundKey: "operating", AmountCents: 20000},
			{FundID: resFundID, FundKey: "reserve", AmountCents: 5000},
		},
	}

	effects, err := engine.RecordTransaction(context.Background(), tx)
	require.NoError(t, err)

	// GL: DR 1100 $250, CR 4010 $200, CR 4020 $50.
	require.Len(t, effects.JournalLines, 3)
	assert.Equal(t, resolver.accounts[1100].ID, effects.JournalLines[0].AccountID)
	assert.Equal(t, int64(25000), effects.JournalLines[0].DebitCents)
	assert.Equal(t, int64(20000), effects.JournalLines[1].CreditCents)
	assert.Equal(t, int64(5000), effects.JournalLines[2].CreditCents)

	// Fund: 2 directives.
	require.Len(t, effects.FundTransactions, 2)

	// Ledger: 1 charge for total.
	require.Len(t, effects.LedgerEntries, 1)
	assert.Equal(t, int64(25000), effects.LedgerEntries[0].AmountCents)
}

// ── Payment tests ────────────────────────────────────────────────────

func TestGaapEngine_RecordTransaction_Payment_Accrual(t *testing.T) {
	engine, resolver := newTestGaapEngineWithResolver(RecognitionBasisAccrual)
	unitID := uuid.New()
	fundID := uuid.New()

	tx := FinancialTransaction{
		Type: TxTypePayment, OrgID: uuid.New(), AmountCents: 15000,
		EffectiveDate: time.Date(2026, 4, 5, 0, 0, 0, 0, time.UTC),
		SourceID: uuid.New(), UnitID: &unitID,
		FundAllocations: []FundAllocation{{FundID: fundID, FundKey: "operating", AmountCents: 15000}},
		Memo:            "Monthly payment",
	}

	effects, err := engine.RecordTransaction(context.Background(), tx)
	require.NoError(t, err)
	require.NotNil(t, effects)

	// GL: DR 1010 (Cash-Operating) / CR 1100 (AR).
	require.Len(t, effects.JournalLines, 2)
	assert.Equal(t, resolver.accounts[1010].ID, effects.JournalLines[0].AccountID)
	assert.Equal(t, int64(15000), effects.JournalLines[0].DebitCents)
	assert.Equal(t, int64(0), effects.JournalLines[0].CreditCents)
	assert.Equal(t, resolver.accounts[1100].ID, effects.JournalLines[1].AccountID)
	assert.Equal(t, int64(0), effects.JournalLines[1].DebitCents)
	assert.Equal(t, int64(15000), effects.JournalLines[1].CreditCents)

	// Fund: payment directive.
	require.Len(t, effects.FundTransactions, 1)
	assert.Equal(t, fundID, effects.FundTransactions[0].FundID)
	assert.Equal(t, "payment", effects.FundTransactions[0].Type)
	assert.Equal(t, int64(15000), effects.FundTransactions[0].AmountCents)

	// Ledger: payment entry on unit.
	require.Len(t, effects.LedgerEntries, 1)
	assert.Equal(t, unitID, effects.LedgerEntries[0].UnitID)
	assert.Equal(t, LedgerEntryTypePayment, effects.LedgerEntries[0].Type)
	assert.Equal(t, int64(15000), effects.LedgerEntries[0].AmountCents)
	assert.Equal(t, tx.SourceID, effects.LedgerEntries[0].SourceID)
}

func TestGaapEngine_RecordTransaction_Payment_CashBasis(t *testing.T) {
	engine, resolver := newTestGaapEngineWithResolver(RecognitionBasisCash)
	unitID := uuid.New()
	fundID := uuid.New()

	tx := FinancialTransaction{
		Type: TxTypePayment, OrgID: uuid.New(), AmountCents: 15000,
		EffectiveDate: time.Date(2026, 4, 5, 0, 0, 0, 0, time.UTC),
		SourceID: uuid.New(), UnitID: &unitID,
		FundAllocations: []FundAllocation{{FundID: fundID, FundKey: "operating", AmountCents: 15000}},
		Memo:            "Monthly payment",
	}

	effects, err := engine.RecordTransaction(context.Background(), tx)
	require.NoError(t, err)
	require.NotNil(t, effects)

	// Cash basis: DR Cash / CR Revenue (revenue recognized at receipt, not AR).
	require.Len(t, effects.JournalLines, 2)
	assert.Equal(t, resolver.accounts[1010].ID, effects.JournalLines[0].AccountID)
	assert.Equal(t, int64(15000), effects.JournalLines[0].DebitCents)
	assert.Equal(t, resolver.accounts[4010].ID, effects.JournalLines[1].AccountID)
	assert.Equal(t, int64(15000), effects.JournalLines[1].CreditCents)

	// Fund: revenue directive (not payment — revenue recognized at receipt).
	require.Len(t, effects.FundTransactions, 1)
	assert.Equal(t, fundID, effects.FundTransactions[0].FundID)
	assert.Equal(t, "revenue", effects.FundTransactions[0].Type)
	assert.Equal(t, int64(15000), effects.FundTransactions[0].AmountCents)

	// Ledger: payment entry on unit.
	require.Len(t, effects.LedgerEntries, 1)
	assert.Equal(t, LedgerEntryTypePayment, effects.LedgerEntries[0].Type)
	assert.Equal(t, int64(15000), effects.LedgerEntries[0].AmountCents)
}

// ── Fund Transfer tests ──────────────────────────────────────────────

func TestGaapEngine_RecordTransaction_FundTransfer(t *testing.T) {
	engine, resolver := newTestGaapEngineWithResolver(RecognitionBasisAccrual)
	srcFundID := uuid.New()
	dstFundID := uuid.New()

	tx := FinancialTransaction{
		Type: TxTypeFundTransfer, OrgID: uuid.New(), AmountCents: 50000,
		EffectiveDate: time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC),
		SourceID:      uuid.New(),
		FundAllocations: []FundAllocation{
			{FundID: srcFundID, FundKey: "operating", AmountCents: 50000},
			{FundID: dstFundID, FundKey: "reserve", AmountCents: 50000},
		},
		Memo: "Transfer to reserve",
	}

	effects, err := engine.RecordTransaction(context.Background(), tx)
	require.NoError(t, err)
	require.NotNil(t, effects)

	// GL: 4 lines — DR 3100 (Interfund Out), CR 1010 (source cash),
	//               DR 1020 (dest cash), CR 3110 (Interfund In).
	require.Len(t, effects.JournalLines, 4)
	assert.Equal(t, resolver.accounts[3100].ID, effects.JournalLines[0].AccountID)
	assert.Equal(t, int64(50000), effects.JournalLines[0].DebitCents)
	assert.Equal(t, resolver.accounts[1010].ID, effects.JournalLines[1].AccountID)
	assert.Equal(t, int64(50000), effects.JournalLines[1].CreditCents)
	assert.Equal(t, resolver.accounts[1020].ID, effects.JournalLines[2].AccountID)
	assert.Equal(t, int64(50000), effects.JournalLines[2].DebitCents)
	assert.Equal(t, resolver.accounts[3110].ID, effects.JournalLines[3].AccountID)
	assert.Equal(t, int64(50000), effects.JournalLines[3].CreditCents)

	// Fund: 2 directives — withdrawal from source, deposit to dest.
	require.Len(t, effects.FundTransactions, 2)
	assert.Equal(t, srcFundID, effects.FundTransactions[0].FundID)
	assert.Equal(t, FundTxTypeTransferOut, effects.FundTransactions[0].Type)
	assert.Equal(t, int64(50000), effects.FundTransactions[0].AmountCents)
	assert.Equal(t, dstFundID, effects.FundTransactions[1].FundID)
	assert.Equal(t, FundTxTypeTransferIn, effects.FundTransactions[1].Type)
	assert.Equal(t, int64(50000), effects.FundTransactions[1].AmountCents)

	// Ledger: none — fund transfers don't affect unit balances.
	assert.Empty(t, effects.LedgerEntries)
}

// ── Late Fee tests ───────────────────────────────────────────────────

func TestGaapEngine_RecordTransaction_LateFee(t *testing.T) {
	engine, resolver := newTestGaapEngineWithResolver(RecognitionBasisAccrual)
	unitID := uuid.New()
	fundID := uuid.New()

	tx := FinancialTransaction{
		Type: TxTypeLateFee, OrgID: uuid.New(), AmountCents: 2500,
		EffectiveDate: time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC),
		SourceID: uuid.New(), UnitID: &unitID,
		FundAllocations: []FundAllocation{{FundID: fundID, FundKey: "operating", AmountCents: 2500}},
		Memo:            "Late fee",
	}

	effects, err := engine.RecordTransaction(context.Background(), tx)
	require.NoError(t, err)
	require.NotNil(t, effects)

	// GL: DR 1100 (AR) / CR 4100 (Late Fee Revenue).
	require.Len(t, effects.JournalLines, 2)
	assert.Equal(t, resolver.accounts[1100].ID, effects.JournalLines[0].AccountID)
	assert.Equal(t, int64(2500), effects.JournalLines[0].DebitCents)
	assert.Equal(t, int64(0), effects.JournalLines[0].CreditCents)
	assert.Equal(t, resolver.accounts[4100].ID, effects.JournalLines[1].AccountID)
	assert.Equal(t, int64(0), effects.JournalLines[1].DebitCents)
	assert.Equal(t, int64(2500), effects.JournalLines[1].CreditCents)

	// Fund: late_fee directive.
	require.Len(t, effects.FundTransactions, 1)
	assert.Equal(t, fundID, effects.FundTransactions[0].FundID)
	assert.Equal(t, "late_fee", effects.FundTransactions[0].Type)
	assert.Equal(t, int64(2500), effects.FundTransactions[0].AmountCents)

	// Ledger: late fee entry on unit.
	require.Len(t, effects.LedgerEntries, 1)
	assert.Equal(t, unitID, effects.LedgerEntries[0].UnitID)
	assert.Equal(t, LedgerEntryTypeLateFee, effects.LedgerEntries[0].Type)
	assert.Equal(t, int64(2500), effects.LedgerEntries[0].AmountCents)
	assert.Equal(t, tx.SourceID, effects.LedgerEntries[0].SourceID)
}

func TestGaapEngine_RecordTransaction_LateFee_CashBasis(t *testing.T) {
	engine, _ := newTestGaapEngineWithResolver(RecognitionBasisCash)
	unitID := uuid.New()

	tx := FinancialTransaction{
		Type: TxTypeLateFee, OrgID: uuid.New(), AmountCents: 2500,
		EffectiveDate: time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC),
		SourceID: uuid.New(), UnitID: &unitID,
		FundAllocations: []FundAllocation{{FundID: uuid.New(), FundKey: "operating", AmountCents: 2500}},
	}

	effects, err := engine.RecordTransaction(context.Background(), tx)
	require.NoError(t, err)

	// Cash basis: no GL, no fund transactions. Ledger only.
	assert.Empty(t, effects.JournalLines)
	assert.Empty(t, effects.FundTransactions)
	require.Len(t, effects.LedgerEntries, 1)
	assert.Equal(t, LedgerEntryTypeLateFee, effects.LedgerEntries[0].Type)
}

// ── Interest Accrual tests ───────────────────────────────────────────

func TestGaapEngine_RecordTransaction_InterestAccrual(t *testing.T) {
	engine, resolver := newTestGaapEngineWithResolver(RecognitionBasisAccrual)
	unitID := uuid.New()
	fundID := uuid.New()

	tx := FinancialTransaction{
		Type: TxTypeInterestAccrual, OrgID: uuid.New(), AmountCents: 1200,
		EffectiveDate: time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC),
		SourceID: uuid.New(), UnitID: &unitID,
		FundAllocations: []FundAllocation{{FundID: fundID, FundKey: "operating", AmountCents: 1200}},
		Memo:            "Interest accrual",
	}

	effects, err := engine.RecordTransaction(context.Background(), tx)
	require.NoError(t, err)
	require.NotNil(t, effects)

	// GL: DR 1100 (AR) / CR 4200 (Interest Income).
	require.Len(t, effects.JournalLines, 2)
	assert.Equal(t, resolver.accounts[1100].ID, effects.JournalLines[0].AccountID)
	assert.Equal(t, int64(1200), effects.JournalLines[0].DebitCents)
	assert.Equal(t, int64(0), effects.JournalLines[0].CreditCents)
	assert.Equal(t, resolver.accounts[4200].ID, effects.JournalLines[1].AccountID)
	assert.Equal(t, int64(0), effects.JournalLines[1].DebitCents)
	assert.Equal(t, int64(1200), effects.JournalLines[1].CreditCents)

	// Fund: interest directive.
	require.Len(t, effects.FundTransactions, 1)
	assert.Equal(t, fundID, effects.FundTransactions[0].FundID)
	assert.Equal(t, "interest", effects.FundTransactions[0].Type)
	assert.Equal(t, int64(1200), effects.FundTransactions[0].AmountCents)

	// Ledger: charge entry on unit.
	require.Len(t, effects.LedgerEntries, 1)
	assert.Equal(t, unitID, effects.LedgerEntries[0].UnitID)
	assert.Equal(t, LedgerEntryTypeCharge, effects.LedgerEntries[0].Type)
	assert.Equal(t, int64(1200), effects.LedgerEntries[0].AmountCents)
	assert.Equal(t, tx.SourceID, effects.LedgerEntries[0].SourceID)
}

func TestGaapEngine_RecordTransaction_InterestAccrual_CashBasis(t *testing.T) {
	engine, _ := newTestGaapEngineWithResolver(RecognitionBasisCash)
	unitID := uuid.New()

	tx := FinancialTransaction{
		Type: TxTypeInterestAccrual, OrgID: uuid.New(), AmountCents: 1200,
		EffectiveDate: time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC),
		SourceID: uuid.New(), UnitID: &unitID,
		FundAllocations: []FundAllocation{{FundID: uuid.New(), FundKey: "operating", AmountCents: 1200}},
	}

	effects, err := engine.RecordTransaction(context.Background(), tx)
	require.NoError(t, err)

	// Cash basis: no GL, no fund transactions. Ledger only.
	assert.Empty(t, effects.JournalLines)
	assert.Empty(t, effects.FundTransactions)
	require.Len(t, effects.LedgerEntries, 1)
	assert.Equal(t, LedgerEntryTypeCharge, effects.LedgerEntries[0].Type)
}

// ── Overpayment CreditDirective tests ───────────────────────────────

func TestGaapEngine_RecordTransaction_Payment_OverpaymentCredit(t *testing.T) {
	engine, _ := newTestGaapEngineWithResolver(RecognitionBasisAccrual)
	unitID := uuid.New()
	fundID := uuid.New()

	tx := FinancialTransaction{
		Type: TxTypePayment, OrgID: uuid.New(), AmountCents: 25000,
		EffectiveDate: time.Date(2026, 4, 5, 0, 0, 0, 0, time.UTC),
		SourceID: uuid.New(), UnitID: &unitID,
		FundAllocations: []FundAllocation{{FundID: fundID, FundKey: "operating", AmountCents: 25000}},
		Memo:            "Payment with overpayment",
		Metadata:        map[string]any{"overpayment_cents": int64(10000)},
	}

	effects, err := engine.RecordTransaction(context.Background(), tx)
	require.NoError(t, err)
	require.NotNil(t, effects)

	// Credits: 1 directive for the overpayment.
	require.Len(t, effects.Credits, 1)
	assert.Equal(t, unitID, effects.Credits[0].UnitID)
	assert.Equal(t, int64(10000), effects.Credits[0].AmountCents)
	assert.Equal(t, CreditTypePrepayment, effects.Credits[0].Type)
}

func TestGaapEngine_RecordTransaction_Payment_NoOverpayment(t *testing.T) {
	engine, _ := newTestGaapEngineWithResolver(RecognitionBasisAccrual)
	unitID := uuid.New()
	fundID := uuid.New()

	tx := FinancialTransaction{
		Type: TxTypePayment, OrgID: uuid.New(), AmountCents: 15000,
		EffectiveDate: time.Date(2026, 4, 5, 0, 0, 0, 0, time.UTC),
		SourceID: uuid.New(), UnitID: &unitID,
		FundAllocations: []FundAllocation{{FundID: fundID, FundKey: "operating", AmountCents: 15000}},
		Memo:            "Exact payment",
	}

	effects, err := engine.RecordTransaction(context.Background(), tx)
	require.NoError(t, err)
	require.NotNil(t, effects)

	// No credits when there's no overpayment metadata.
	assert.Empty(t, effects.Credits)
}

func TestGaapEngine_RecordTransaction_Payment_OverpaymentZero(t *testing.T) {
	engine, _ := newTestGaapEngineWithResolver(RecognitionBasisAccrual)
	unitID := uuid.New()
	fundID := uuid.New()

	tx := FinancialTransaction{
		Type: TxTypePayment, OrgID: uuid.New(), AmountCents: 15000,
		EffectiveDate: time.Date(2026, 4, 5, 0, 0, 0, 0, time.UTC),
		SourceID: uuid.New(), UnitID: &unitID,
		FundAllocations: []FundAllocation{{FundID: fundID, FundKey: "operating", AmountCents: 15000}},
		Metadata:        map[string]any{"overpayment_cents": int64(0)},
	}

	effects, err := engine.RecordTransaction(context.Background(), tx)
	require.NoError(t, err)
	require.NotNil(t, effects)

	// Zero overpayment should not produce a credit directive.
	assert.Empty(t, effects.Credits)
}

// ── Expense tests ───────────────────────────────────────────────────

func TestGaapEngine_RecordTransaction_Expense_Accrual_Approved(t *testing.T) {
	engine, resolver := newTestGaapEngineWithResolver(RecognitionBasisAccrual)
	fundID := uuid.New()

	tx := FinancialTransaction{
		Type: TxTypeExpense, OrgID: uuid.New(), AmountCents: 50000,
		EffectiveDate: time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC),
		SourceID:      uuid.New(),
		FundAllocations: []FundAllocation{{FundID: fundID, FundKey: "operating", AmountCents: 50000}},
		Memo:     "Landscaping expense",
		Metadata: map[string]any{"status": "approved", "expense_account": 5040},
	}

	effects, err := engine.RecordTransaction(context.Background(), tx)
	require.NoError(t, err)
	require.NotNil(t, effects)

	// GL: DR 5040 (Landscaping) / CR 2100 (AP).
	require.Len(t, effects.JournalLines, 2)
	assert.Equal(t, resolver.accounts[5040].ID, effects.JournalLines[0].AccountID)
	assert.Equal(t, int64(50000), effects.JournalLines[0].DebitCents)
	assert.Equal(t, int64(0), effects.JournalLines[0].CreditCents)
	assert.Equal(t, resolver.accounts[2100].ID, effects.JournalLines[1].AccountID)
	assert.Equal(t, int64(0), effects.JournalLines[1].DebitCents)
	assert.Equal(t, int64(50000), effects.JournalLines[1].CreditCents)

	// Fund: expense directive.
	require.Len(t, effects.FundTransactions, 1)
	assert.Equal(t, fundID, effects.FundTransactions[0].FundID)
	assert.Equal(t, "expense", effects.FundTransactions[0].Type)
	assert.Equal(t, int64(50000), effects.FundTransactions[0].AmountCents)

	// No ledger entries — expenses don't affect unit balances.
	assert.Empty(t, effects.LedgerEntries)
}

func TestGaapEngine_RecordTransaction_Expense_Accrual_Paid(t *testing.T) {
	engine, resolver := newTestGaapEngineWithResolver(RecognitionBasisAccrual)
	fundID := uuid.New()

	tx := FinancialTransaction{
		Type: TxTypeExpense, OrgID: uuid.New(), AmountCents: 50000,
		EffectiveDate: time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC),
		SourceID:      uuid.New(),
		FundAllocations: []FundAllocation{{FundID: fundID, FundKey: "operating", AmountCents: 50000}},
		Memo:     "Landscaping payment",
		Metadata: map[string]any{"status": "paid", "expense_account": 5040},
	}

	effects, err := engine.RecordTransaction(context.Background(), tx)
	require.NoError(t, err)
	require.NotNil(t, effects)

	// GL: DR 2100 (AP) / CR 1010 (Cash-Operating).
	require.Len(t, effects.JournalLines, 2)
	assert.Equal(t, resolver.accounts[2100].ID, effects.JournalLines[0].AccountID)
	assert.Equal(t, int64(50000), effects.JournalLines[0].DebitCents)
	assert.Equal(t, int64(0), effects.JournalLines[0].CreditCents)
	assert.Equal(t, resolver.accounts[1010].ID, effects.JournalLines[1].AccountID)
	assert.Equal(t, int64(0), effects.JournalLines[1].DebitCents)
	assert.Equal(t, int64(50000), effects.JournalLines[1].CreditCents)

	// Fund: expense directive.
	require.Len(t, effects.FundTransactions, 1)
	assert.Equal(t, fundID, effects.FundTransactions[0].FundID)
	assert.Equal(t, "expense", effects.FundTransactions[0].Type)
	assert.Equal(t, int64(50000), effects.FundTransactions[0].AmountCents)

	// No ledger entries.
	assert.Empty(t, effects.LedgerEntries)
}

func TestGaapEngine_RecordTransaction_Expense_Cash_Paid(t *testing.T) {
	engine, resolver := newTestGaapEngineWithResolver(RecognitionBasisCash)
	fundID := uuid.New()

	tx := FinancialTransaction{
		Type: TxTypeExpense, OrgID: uuid.New(), AmountCents: 50000,
		EffectiveDate: time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC),
		SourceID:      uuid.New(),
		FundAllocations: []FundAllocation{{FundID: fundID, FundKey: "operating", AmountCents: 50000}},
		Memo:     "Landscaping payment",
		Metadata: map[string]any{"status": "paid", "expense_account": 5040},
	}

	effects, err := engine.RecordTransaction(context.Background(), tx)
	require.NoError(t, err)
	require.NotNil(t, effects)

	// Cash + paid: DR 5040 (Landscaping) / CR 1010 (Cash-Operating).
	require.Len(t, effects.JournalLines, 2)
	assert.Equal(t, resolver.accounts[5040].ID, effects.JournalLines[0].AccountID)
	assert.Equal(t, int64(50000), effects.JournalLines[0].DebitCents)
	assert.Equal(t, int64(0), effects.JournalLines[0].CreditCents)
	assert.Equal(t, resolver.accounts[1010].ID, effects.JournalLines[1].AccountID)
	assert.Equal(t, int64(0), effects.JournalLines[1].DebitCents)
	assert.Equal(t, int64(50000), effects.JournalLines[1].CreditCents)

	// Fund: expense directive.
	require.Len(t, effects.FundTransactions, 1)
	assert.Equal(t, fundID, effects.FundTransactions[0].FundID)
	assert.Equal(t, "expense", effects.FundTransactions[0].Type)
	assert.Equal(t, int64(50000), effects.FundTransactions[0].AmountCents)

	// No ledger entries.
	assert.Empty(t, effects.LedgerEntries)
}

func TestGaapEngine_RecordTransaction_Expense_Cash_Approved(t *testing.T) {
	engine, _ := newTestGaapEngineWithResolver(RecognitionBasisCash)

	tx := FinancialTransaction{
		Type: TxTypeExpense, OrgID: uuid.New(), AmountCents: 50000,
		EffectiveDate: time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC),
		SourceID:      uuid.New(),
		FundAllocations: []FundAllocation{{FundID: uuid.New(), FundKey: "operating", AmountCents: 50000}},
		Memo:     "Landscaping expense",
		Metadata: map[string]any{"status": "approved", "expense_account": 5040},
	}

	effects, err := engine.RecordTransaction(context.Background(), tx)
	require.NoError(t, err)
	require.NotNil(t, effects)

	// Cash basis + approved: no GL, no fund transactions, no ledger entries.
	assert.Empty(t, effects.JournalLines)
	assert.Empty(t, effects.FundTransactions)
	assert.Empty(t, effects.LedgerEntries)
}

func TestGaapEngine_RecordTransaction_Expense_DefaultAccount(t *testing.T) {
	engine, resolver := newTestGaapEngineWithResolver(RecognitionBasisAccrual)
	fundID := uuid.New()

	// No expense_account in metadata — should default to 5010.
	tx := FinancialTransaction{
		Type: TxTypeExpense, OrgID: uuid.New(), AmountCents: 30000,
		EffectiveDate: time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC),
		SourceID:      uuid.New(),
		FundAllocations: []FundAllocation{{FundID: fundID, FundKey: "operating", AmountCents: 30000}},
		Memo:     "Management fee",
		Metadata: map[string]any{"status": "approved"},
	}

	effects, err := engine.RecordTransaction(context.Background(), tx)
	require.NoError(t, err)
	require.NotNil(t, effects)

	// GL: DR 5010 (default) / CR 2100 (AP).
	require.Len(t, effects.JournalLines, 2)
	assert.Equal(t, resolver.accounts[5010].ID, effects.JournalLines[0].AccountID)
	assert.Equal(t, int64(30000), effects.JournalLines[0].DebitCents)
	assert.Equal(t, resolver.accounts[2100].ID, effects.JournalLines[1].AccountID)
	assert.Equal(t, int64(30000), effects.JournalLines[1].CreditCents)
}
