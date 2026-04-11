package fin

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestIfrsEngine() *IfrsEngine {
	return NewIfrsEngine(nil, nil, EngineConfig{
		RecognitionBasis: RecognitionBasisAccrual,
		FiscalYearStart:  1,
	})
}

func TestIfrsEngine_Standard(t *testing.T) {
	engine := newTestIfrsEngine()
	assert.Equal(t, AccountingStandardIFRS, engine.Standard())
}

func TestIfrsEngine_ChartOfAccounts_NonEmpty(t *testing.T) {
	engine := newTestIfrsEngine()
	chart := engine.ChartOfAccounts()
	require.NotEmpty(t, chart)
	t.Logf("IFRS chart has %d accounts", len(chart))

	// All accounts must be system accounts.
	for _, a := range chart {
		assert.True(t, a.IsSystem, "account %d %s should be system", a.Number, a.Name)
		if !a.IsHeader {
			assert.NotZero(t, a.ParentNum, "detail account %d %s must have parent", a.Number, a.Name)
		}
	}
}

func TestIfrsEngine_ChartOfAccounts_RetainedEarnings(t *testing.T) {
	engine := newTestIfrsEngine()
	chart := engine.ChartOfAccounts()
	byNumber := make(map[int]GLAccountSeed)
	for _, a := range chart {
		byNumber[a.Number] = a
	}

	// IFRS uses a single "Retained Earnings" account instead of per-fund balances.
	re, ok := byNumber[3010]
	require.True(t, ok, "account 3010 must exist")
	assert.Equal(t, "Retained Earnings", re.Name)
	assert.Equal(t, "equity", re.Type)

	// Per-fund balance accounts should NOT exist in the IFRS chart.
	_, has3020 := byNumber[3020]
	_, has3030 := byNumber[3030]
	_, has3040 := byNumber[3040]
	assert.False(t, has3020, "IFRS chart should not have per-fund balance 3020")
	assert.False(t, has3030, "IFRS chart should not have per-fund balance 3030")
	assert.False(t, has3040, "IFRS chart should not have per-fund balance 3040")

	// Interfund transfer accounts should still exist.
	assert.Equal(t, "equity", byNumber[3100].Type, "Interfund Transfer Out should be equity")
	assert.Equal(t, "equity", byNumber[3110].Type, "Interfund Transfer In should be equity")
}

func TestIfrsEngine_ChartOfAccounts_NoDuplicateNumbers(t *testing.T) {
	engine := newTestIfrsEngine()
	chart := engine.ChartOfAccounts()
	seen := make(map[int]bool)
	for _, a := range chart {
		assert.False(t, seen[a.Number], "duplicate account number %d", a.Number)
		seen[a.Number] = true
	}
}

func TestIfrsEngine_ChartOfAccounts_HeaderAndDetailCounts(t *testing.T) {
	engine := newTestIfrsEngine()
	chart := engine.ChartOfAccounts()

	headers := 0
	detail := 0
	for _, a := range chart {
		if a.IsHeader {
			headers++
		} else {
			detail++
		}
	}
	assert.Equal(t, 5, headers, "should have 5 header accounts")
	assert.Equal(t, 48, detail, "should have 48 detail accounts")
}

// newTestIfrsEngineWithResolver returns an IFRS engine with a stub account
// resolver, reusing the same stub as the GAAP tests.
func newTestIfrsEngineWithResolver(basis RecognitionBasis) (*IfrsEngine, *stubAccountResolver) {
	resolver := &stubAccountResolver{accounts: map[int]*GLAccount{
		1010: {ID: uuid.MustParse("00000000-0000-0000-0000-000000001010"), AccountNumber: 1010},
		1020: {ID: uuid.MustParse("00000000-0000-0000-0000-000000001020"), AccountNumber: 1020},
		1030: {ID: uuid.MustParse("00000000-0000-0000-0000-000000001030"), AccountNumber: 1030},
		1040: {ID: uuid.MustParse("00000000-0000-0000-0000-000000001040"), AccountNumber: 1040},
		1100: {ID: uuid.MustParse("00000000-0000-0000-0000-000000001100"), AccountNumber: 1100},
		1105: {ID: uuid.MustParse("00000000-0000-0000-0000-000000001105"), AccountNumber: 1105},
		2100: {ID: uuid.MustParse("00000000-0000-0000-0000-000000002100"), AccountNumber: 2100},
		3010: {ID: uuid.MustParse("00000000-0000-0000-0000-000000003010"), AccountNumber: 3010},
		3100: {ID: uuid.MustParse("00000000-0000-0000-0000-000000003100"), AccountNumber: 3100},
		3110: {ID: uuid.MustParse("00000000-0000-0000-0000-000000003110"), AccountNumber: 3110},
		4010: {ID: uuid.MustParse("00000000-0000-0000-0000-000000004010"), AccountNumber: 4010},
		4020: {ID: uuid.MustParse("00000000-0000-0000-0000-000000004020"), AccountNumber: 4020},
		4030: {ID: uuid.MustParse("00000000-0000-0000-0000-000000004030"), AccountNumber: 4030},
		4040: {ID: uuid.MustParse("00000000-0000-0000-0000-000000004040"), AccountNumber: 4040},
		4100: {ID: uuid.MustParse("00000000-0000-0000-0000-000000004100"), AccountNumber: 4100},
		4200: {ID: uuid.MustParse("00000000-0000-0000-0000-000000004200"), AccountNumber: 4200},
		5010: {ID: uuid.MustParse("00000000-0000-0000-0000-000000005010"), AccountNumber: 5010},
		5040: {ID: uuid.MustParse("00000000-0000-0000-0000-000000005040"), AccountNumber: 5040},
		5070: {ID: uuid.MustParse("00000000-0000-0000-0000-000000005070"), AccountNumber: 5070},
	}}
	engine := NewIfrsEngine(resolver, nil, EngineConfig{RecognitionBasis: basis, FiscalYearStart: 1})
	return engine, resolver
}

// ── RecordTransaction tests ─────────────────────────────────────────

func TestIfrsEngine_RecordTransaction_Assessment_Accrual(t *testing.T) {
	engine, resolver := newTestIfrsEngineWithResolver(RecognitionBasisAccrual)
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
	assert.Equal(t, resolver.accounts[4010].ID, effects.JournalLines[1].AccountID)
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
}

func TestIfrsEngine_RecordTransaction_Payment_Accrual(t *testing.T) {
	engine, resolver := newTestIfrsEngineWithResolver(RecognitionBasisAccrual)
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
	assert.Equal(t, resolver.accounts[1100].ID, effects.JournalLines[1].AccountID)
	assert.Equal(t, int64(15000), effects.JournalLines[1].CreditCents)

	// Fund: payment directive.
	require.Len(t, effects.FundTransactions, 1)
	assert.Equal(t, fundID, effects.FundTransactions[0].FundID)
	assert.Equal(t, "payment", effects.FundTransactions[0].Type)

	// Ledger: payment entry on unit.
	require.Len(t, effects.LedgerEntries, 1)
	assert.Equal(t, LedgerEntryTypePayment, effects.LedgerEntries[0].Type)
	assert.Equal(t, int64(15000), effects.LedgerEntries[0].AmountCents)
}

func TestIfrsEngine_RecordTransaction_Assessment_CashBasis(t *testing.T) {
	engine, _ := newTestIfrsEngineWithResolver(RecognitionBasisCash)
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

func TestIfrsEngine_RecordTransaction_RejectsModifiedAccrual(t *testing.T) {
	engine := NewIfrsEngine(nil, nil, EngineConfig{
		RecognitionBasis: RecognitionBasisModifiedAccrual,
		FiscalYearStart:  1,
	})
	unitID := uuid.New()

	tx := FinancialTransaction{
		Type: TxTypeAssessment, OrgID: uuid.New(), AmountCents: 10000,
		EffectiveDate: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
		SourceID: uuid.New(), UnitID: &unitID,
	}

	_, err := engine.RecordTransaction(context.Background(), tx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "modified_accrual")
	assert.Contains(t, err.Error(), "IFRS")
}

func TestIfrsEngine_RecordTransaction_Payment_CashBasis(t *testing.T) {
	engine, resolver := newTestIfrsEngineWithResolver(RecognitionBasisCash)
	unitID := uuid.New()
	fundID := uuid.New()

	tx := FinancialTransaction{
		Type: TxTypePayment, OrgID: uuid.New(), AmountCents: 15000,
		EffectiveDate: time.Date(2026, 4, 5, 0, 0, 0, 0, time.UTC),
		SourceID: uuid.New(), UnitID: &unitID,
		FundAllocations: []FundAllocation{{FundID: fundID, FundKey: "operating", AmountCents: 15000}},
	}

	effects, err := engine.RecordTransaction(context.Background(), tx)
	require.NoError(t, err)

	// Cash basis: DR Cash / CR Revenue.
	require.Len(t, effects.JournalLines, 2)
	assert.Equal(t, resolver.accounts[1010].ID, effects.JournalLines[0].AccountID)
	assert.Equal(t, int64(15000), effects.JournalLines[0].DebitCents)
	assert.Equal(t, resolver.accounts[4010].ID, effects.JournalLines[1].AccountID)
	assert.Equal(t, int64(15000), effects.JournalLines[1].CreditCents)

	// Fund: revenue directive.
	require.Len(t, effects.FundTransactions, 1)
	assert.Equal(t, "revenue", effects.FundTransactions[0].Type)
}

func TestIfrsEngine_RecordTransaction_FundTransfer(t *testing.T) {
	engine, resolver := newTestIfrsEngineWithResolver(RecognitionBasisAccrual)
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

	// GL: 4 lines.
	require.Len(t, effects.JournalLines, 4)
	assert.Equal(t, resolver.accounts[3100].ID, effects.JournalLines[0].AccountID)
	assert.Equal(t, resolver.accounts[1010].ID, effects.JournalLines[1].AccountID)
	assert.Equal(t, resolver.accounts[1020].ID, effects.JournalLines[2].AccountID)
	assert.Equal(t, resolver.accounts[3110].ID, effects.JournalLines[3].AccountID)

	// Fund: 2 directives.
	require.Len(t, effects.FundTransactions, 2)
	assert.Equal(t, FundTxTypeTransferOut, effects.FundTransactions[0].Type)
	assert.Equal(t, FundTxTypeTransferIn, effects.FundTransactions[1].Type)
}

func TestIfrsEngine_RecordTransaction_LateFee(t *testing.T) {
	engine, resolver := newTestIfrsEngineWithResolver(RecognitionBasisAccrual)
	unitID := uuid.New()
	fundID := uuid.New()

	tx := FinancialTransaction{
		Type: TxTypeLateFee, OrgID: uuid.New(), AmountCents: 2500,
		EffectiveDate: time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC),
		SourceID: uuid.New(), UnitID: &unitID,
		FundAllocations: []FundAllocation{{FundID: fundID, FundKey: "operating", AmountCents: 2500}},
	}

	effects, err := engine.RecordTransaction(context.Background(), tx)
	require.NoError(t, err)

	// GL: DR 1100 (AR) / CR 4100 (Late Fee Revenue).
	require.Len(t, effects.JournalLines, 2)
	assert.Equal(t, resolver.accounts[1100].ID, effects.JournalLines[0].AccountID)
	assert.Equal(t, resolver.accounts[4100].ID, effects.JournalLines[1].AccountID)

	require.Len(t, effects.LedgerEntries, 1)
	assert.Equal(t, LedgerEntryTypeLateFee, effects.LedgerEntries[0].Type)
}

func TestIfrsEngine_RecordTransaction_Expense_Accrual_Approved(t *testing.T) {
	engine, resolver := newTestIfrsEngineWithResolver(RecognitionBasisAccrual)
	fundID := uuid.New()

	tx := FinancialTransaction{
		Type: TxTypeExpense, OrgID: uuid.New(), AmountCents: 50000,
		EffectiveDate: time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC),
		SourceID:      uuid.New(),
		FundAllocations: []FundAllocation{{FundID: fundID, FundKey: "operating", AmountCents: 50000}},
		Metadata:        map[string]any{"status": "approved", "expense_account": 5040},
	}

	effects, err := engine.RecordTransaction(context.Background(), tx)
	require.NoError(t, err)

	// GL: DR 5040 (Landscaping) / CR 2100 (AP).
	require.Len(t, effects.JournalLines, 2)
	assert.Equal(t, resolver.accounts[5040].ID, effects.JournalLines[0].AccountID)
	assert.Equal(t, int64(50000), effects.JournalLines[0].DebitCents)
	assert.Equal(t, resolver.accounts[2100].ID, effects.JournalLines[1].AccountID)
	assert.Equal(t, int64(50000), effects.JournalLines[1].CreditCents)
}

func TestIfrsEngine_RecordTransaction_BadDebtProvision(t *testing.T) {
	engine, resolver := newTestIfrsEngineWithResolver(RecognitionBasisAccrual)

	tx := FinancialTransaction{
		Type: TxTypeBadDebtProvision, OrgID: uuid.New(), AmountCents: 5000,
		EffectiveDate: time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC),
		SourceID:      uuid.New(),
	}

	effects, err := engine.RecordTransaction(context.Background(), tx)
	require.NoError(t, err)

	// GL: DR 5070 (Bad Debt Expense) / CR 1105 (Allowance).
	require.Len(t, effects.JournalLines, 2)
	assert.Equal(t, resolver.accounts[5070].ID, effects.JournalLines[0].AccountID)
	assert.Equal(t, int64(5000), effects.JournalLines[0].DebitCents)
	assert.Equal(t, resolver.accounts[1105].ID, effects.JournalLines[1].AccountID)
	assert.Equal(t, int64(5000), effects.JournalLines[1].CreditCents)
}

// ── ValidateTransaction tests ───────────────────────────────────────

func TestIfrsEngine_ValidateTransaction_RejectsModifiedAccrual(t *testing.T) {
	engine := NewIfrsEngine(nil, nil, EngineConfig{
		RecognitionBasis: RecognitionBasisModifiedAccrual,
		FiscalYearStart:  1,
	})

	tx := FinancialTransaction{
		Type: TxTypeAssessment, OrgID: uuid.New(), AmountCents: 10000,
		SourceID: uuid.New(), UnitID: ptr(uuid.New()),
	}
	err := engine.ValidateTransaction(context.Background(), tx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "modified_accrual")
	assert.Contains(t, err.Error(), "IFRS")
}

func TestIfrsEngine_ValidateTransaction_Valid(t *testing.T) {
	engine := newTestIfrsEngine()

	tx := FinancialTransaction{
		Type: TxTypeAssessment, OrgID: uuid.New(), AmountCents: 10000,
		EffectiveDate: time.Now(), SourceID: uuid.New(), UnitID: ptr(uuid.New()),
		FundAllocations: []FundAllocation{{FundID: uuid.New(), FundKey: "operating", AmountCents: 10000}},
	}
	assert.NoError(t, engine.ValidateTransaction(context.Background(), tx))
}

func TestIfrsEngine_ValidateTransaction_NegativeAmount(t *testing.T) {
	engine := newTestIfrsEngine()

	tx := FinancialTransaction{
		Type: TxTypeAssessment, OrgID: uuid.New(), AmountCents: -100,
		SourceID: uuid.New(), UnitID: ptr(uuid.New()),
	}
	err := engine.ValidateTransaction(context.Background(), tx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "positive")
}

func TestIfrsEngine_ValidateTransaction_AssessmentRequiresUnit(t *testing.T) {
	engine := newTestIfrsEngine()

	tx := FinancialTransaction{
		Type: TxTypeAssessment, OrgID: uuid.New(), AmountCents: 10000,
		SourceID: uuid.New(),
	}
	err := engine.ValidateTransaction(context.Background(), tx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unit_id")
}

// ── PaymentTerms tests ─────────────────────────────────────────────

func TestIfrsEngine_PaymentTerms_Net30(t *testing.T) {
	engine := newTestIfrsEngine()
	invoiceDate := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)

	result, err := engine.PaymentTerms(context.Background(), PayableContext{
		PayableID:   uuid.New(),
		InvoiceDate: invoiceDate,
		VendorTerms: "Net 30",
		AmountCents: 50000,
	})
	require.NoError(t, err)
	assert.Equal(t, invoiceDate.AddDate(0, 0, 30), result.DueDate)
	assert.Nil(t, result.DiscountDate)
	assert.Nil(t, result.DiscountPercent)
}

func TestIfrsEngine_PaymentTerms_DiscountTerms(t *testing.T) {
	engine := newTestIfrsEngine()
	invoiceDate := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)

	result, err := engine.PaymentTerms(context.Background(), PayableContext{
		PayableID:   uuid.New(),
		InvoiceDate: invoiceDate,
		VendorTerms: "2/10 Net 30",
		AmountCents: 50000,
	})
	require.NoError(t, err)
	assert.Equal(t, invoiceDate.AddDate(0, 0, 30), result.DueDate)
	require.NotNil(t, result.DiscountDate)
	assert.Equal(t, invoiceDate.AddDate(0, 0, 10), *result.DiscountDate)
	require.NotNil(t, result.DiscountPercent)
	assert.Equal(t, float64(2), *result.DiscountPercent)
}

func TestIfrsEngine_PaymentTerms_Default(t *testing.T) {
	engine := newTestIfrsEngine()
	invoiceDate := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)

	result, err := engine.PaymentTerms(context.Background(), PayableContext{
		PayableID:   uuid.New(),
		InvoiceDate: invoiceDate,
		VendorTerms: "", // empty defaults to Net 30
		AmountCents: 50000,
	})
	require.NoError(t, err)
	assert.Equal(t, invoiceDate.AddDate(0, 0, 30), result.DueDate)
}

// ── PayableRecognitionDate tests ────────────────────────────────────

func TestIfrsEngine_PayableRecognitionDate_Accrual_ServiceDate(t *testing.T) {
	engine := newTestIfrsEngine()
	serviceDate := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)

	date, err := engine.PayableRecognitionDate(context.Background(), ExpenseContext{
		InvoiceDate: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
		ServiceDate: &serviceDate,
		AmountCents: 50000,
	})
	require.NoError(t, err)
	assert.Equal(t, serviceDate, date)
}

func TestIfrsEngine_PayableRecognitionDate_Accrual_FallbackToInvoice(t *testing.T) {
	engine := newTestIfrsEngine()
	invoiceDate := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)

	date, err := engine.PayableRecognitionDate(context.Background(), ExpenseContext{
		InvoiceDate: invoiceDate,
		AmountCents: 50000,
	})
	require.NoError(t, err)
	assert.Equal(t, invoiceDate, date)
}

func TestIfrsEngine_PayableRecognitionDate_CashBasis(t *testing.T) {
	engine := NewIfrsEngine(nil, nil, EngineConfig{
		RecognitionBasis: RecognitionBasisCash,
		FiscalYearStart:  1,
	})

	_, err := engine.PayableRecognitionDate(context.Background(), ExpenseContext{
		InvoiceDate: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
		AmountCents: 50000,
	})
	assert.ErrorIs(t, err, ErrCashBasisNoPayable)
}

// ── RevenueRecognitionDate tests ────────────────────────────────────

func TestIfrsEngine_RevenueRecognitionDate(t *testing.T) {
	engine := newTestIfrsEngine()
	effectiveDate := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)

	date, err := engine.RevenueRecognitionDate(context.Background(), FinancialTransaction{
		Type:          TxTypeAssessment,
		EffectiveDate: effectiveDate,
	})
	require.NoError(t, err)
	assert.Equal(t, effectiveDate, date)
}

// ── PaymentApplicationStrategy tests ────────────────────────────────

func TestIfrsEngine_PaymentApplicationStrategy_DefaultOldestFirst(t *testing.T) {
	engine := newTestIfrsEngine()

	strategy, err := engine.PaymentApplicationStrategy(context.Background(), PaymentContext{
		OrgID:     uuid.New(),
		PaymentID: uuid.New(),
		PayerID:   uuid.New(),
	})
	require.NoError(t, err)
	assert.Equal(t, ApplicationMethodOldestFirst, strategy.Method)
}

func TestIfrsEngine_PaymentApplicationStrategy_Designated(t *testing.T) {
	engine := newTestIfrsEngine()
	invoiceID := uuid.New()

	strategy, err := engine.PaymentApplicationStrategy(context.Background(), PaymentContext{
		OrgID:             uuid.New(),
		PaymentID:         uuid.New(),
		PayerID:           uuid.New(),
		DesignatedInvoice: &invoiceID,
	})
	require.NoError(t, err)
	assert.Equal(t, ApplicationMethodDesignated, strategy.Method)
}
