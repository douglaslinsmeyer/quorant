package fin

import (
	"testing"

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
