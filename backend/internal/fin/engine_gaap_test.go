package fin

import (
	"testing"

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
