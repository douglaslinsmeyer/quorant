package fin

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/platform/policy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newRegistryWithCap creates a policy registry that returns the given ruling
// JSON for the specified category.
func newRegistryWithCap(t *testing.T, category string, rulingJSON json.RawMessage) *policy.Registry {
	t.Helper()
	registry := policy.NewRegistry(
		&stubPolicyRecordRepo{records: nil},
		nil,
		&stubAIPolicyResolver{ruling: rulingJSON, confidence: 0.95},
		nil,
		nil,
	)
	err := registry.Register(category, policy.OperationDescriptor{
		Category:         category,
		PromptTemplate:   "Given these policies: {{.Policies}}",
		DefaultThreshold: 0.80,
	})
	require.NoError(t, err)
	return registry
}

// ── Late Fee Cap tests (GAAP) ──────────────────────────────────────

func TestGaapEngine_ValidateTransaction_LateFee_WithinCap(t *testing.T) {
	maxCents := int64(5000)
	ruling, _ := json.Marshal(feeCapRuling{MaxCents: &maxCents})
	registry := newRegistryWithCap(t, "late_fee_cap", ruling)

	engine := NewGaapEngine(nil, registry, EngineConfig{
		RecognitionBasis: RecognitionBasisAccrual, FiscalYearStart: 1,
	})

	tx := FinancialTransaction{
		Type: TxTypeLateFee, OrgID: uuid.New(), AmountCents: 2500,
		EffectiveDate: time.Now(), SourceID: uuid.New(), UnitID: ptr(uuid.New()),
	}
	err := engine.ValidateTransaction(context.Background(), tx)
	assert.NoError(t, err)
}

func TestGaapEngine_ValidateTransaction_LateFee_ExceedsCap(t *testing.T) {
	maxCents := int64(5000)
	ruling, _ := json.Marshal(feeCapRuling{MaxCents: &maxCents})
	registry := newRegistryWithCap(t, "late_fee_cap", ruling)

	engine := NewGaapEngine(nil, registry, EngineConfig{
		RecognitionBasis: RecognitionBasisAccrual, FiscalYearStart: 1,
	})

	tx := FinancialTransaction{
		Type: TxTypeLateFee, OrgID: uuid.New(), AmountCents: 7500,
		EffectiveDate: time.Now(), SourceID: uuid.New(), UnitID: ptr(uuid.New()),
	}
	err := engine.ValidateTransaction(context.Background(), tx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds jurisdiction cap")
	assert.Contains(t, err.Error(), "late fee")
}

func TestGaapEngine_ValidateTransaction_LateFee_NoCap(t *testing.T) {
	// Nil registry means no cap enforcement.
	engine := NewGaapEngine(nil, nil, EngineConfig{
		RecognitionBasis: RecognitionBasisAccrual, FiscalYearStart: 1,
	})

	tx := FinancialTransaction{
		Type: TxTypeLateFee, OrgID: uuid.New(), AmountCents: 999999,
		EffectiveDate: time.Now(), SourceID: uuid.New(), UnitID: ptr(uuid.New()),
	}
	err := engine.ValidateTransaction(context.Background(), tx)
	assert.NoError(t, err)
}

// ── Interest Cap tests (GAAP) ──────────────────────────────────────

func TestGaapEngine_ValidateTransaction_Interest_WithinCap(t *testing.T) {
	maxCents := int64(10000)
	ruling, _ := json.Marshal(interestCapRuling{MaxCents: &maxCents})
	registry := newRegistryWithCap(t, "interest_rate_cap", ruling)

	engine := NewGaapEngine(nil, registry, EngineConfig{
		RecognitionBasis: RecognitionBasisAccrual, FiscalYearStart: 1,
	})

	tx := FinancialTransaction{
		Type: TxTypeInterestAccrual, OrgID: uuid.New(), AmountCents: 5000,
		EffectiveDate: time.Now(), SourceID: uuid.New(), UnitID: ptr(uuid.New()),
	}
	err := engine.ValidateTransaction(context.Background(), tx)
	assert.NoError(t, err)
}

func TestGaapEngine_ValidateTransaction_Interest_ExceedsCap(t *testing.T) {
	maxCents := int64(10000)
	ruling, _ := json.Marshal(interestCapRuling{MaxCents: &maxCents})
	registry := newRegistryWithCap(t, "interest_rate_cap", ruling)

	engine := NewGaapEngine(nil, registry, EngineConfig{
		RecognitionBasis: RecognitionBasisAccrual, FiscalYearStart: 1,
	})

	tx := FinancialTransaction{
		Type: TxTypeInterestAccrual, OrgID: uuid.New(), AmountCents: 15000,
		EffectiveDate: time.Now(), SourceID: uuid.New(), UnitID: ptr(uuid.New()),
	}
	err := engine.ValidateTransaction(context.Background(), tx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds jurisdiction cap")
	assert.Contains(t, err.Error(), "interest accrual")
}

// ── Late Fee Cap tests (IFRS) ──────────────────────────────────────

func TestIfrsEngine_ValidateTransaction_LateFee_WithinCap(t *testing.T) {
	maxCents := int64(5000)
	ruling, _ := json.Marshal(feeCapRuling{MaxCents: &maxCents})
	registry := newRegistryWithCap(t, "late_fee_cap", ruling)

	engine := NewIfrsEngine(nil, registry, EngineConfig{
		RecognitionBasis: RecognitionBasisAccrual, FiscalYearStart: 1,
	})

	tx := FinancialTransaction{
		Type: TxTypeLateFee, OrgID: uuid.New(), AmountCents: 2500,
		EffectiveDate: time.Now(), SourceID: uuid.New(), UnitID: ptr(uuid.New()),
	}
	err := engine.ValidateTransaction(context.Background(), tx)
	assert.NoError(t, err)
}

func TestIfrsEngine_ValidateTransaction_LateFee_ExceedsCap(t *testing.T) {
	maxCents := int64(5000)
	ruling, _ := json.Marshal(feeCapRuling{MaxCents: &maxCents})
	registry := newRegistryWithCap(t, "late_fee_cap", ruling)

	engine := NewIfrsEngine(nil, registry, EngineConfig{
		RecognitionBasis: RecognitionBasisAccrual, FiscalYearStart: 1,
	})

	tx := FinancialTransaction{
		Type: TxTypeLateFee, OrgID: uuid.New(), AmountCents: 7500,
		EffectiveDate: time.Now(), SourceID: uuid.New(), UnitID: ptr(uuid.New()),
	}
	err := engine.ValidateTransaction(context.Background(), tx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds jurisdiction cap")
	assert.Contains(t, err.Error(), "late fee")
}

// ── Interest Cap tests (IFRS) ──────────────────────────────────────

func TestIfrsEngine_ValidateTransaction_Interest_WithinCap(t *testing.T) {
	maxCents := int64(10000)
	ruling, _ := json.Marshal(interestCapRuling{MaxCents: &maxCents})
	registry := newRegistryWithCap(t, "interest_rate_cap", ruling)

	engine := NewIfrsEngine(nil, registry, EngineConfig{
		RecognitionBasis: RecognitionBasisAccrual, FiscalYearStart: 1,
	})

	tx := FinancialTransaction{
		Type: TxTypeInterestAccrual, OrgID: uuid.New(), AmountCents: 5000,
		EffectiveDate: time.Now(), SourceID: uuid.New(), UnitID: ptr(uuid.New()),
	}
	err := engine.ValidateTransaction(context.Background(), tx)
	assert.NoError(t, err)
}

func TestIfrsEngine_ValidateTransaction_Interest_ExceedsCap(t *testing.T) {
	maxCents := int64(10000)
	ruling, _ := json.Marshal(interestCapRuling{MaxCents: &maxCents})
	registry := newRegistryWithCap(t, "interest_rate_cap", ruling)

	engine := NewIfrsEngine(nil, registry, EngineConfig{
		RecognitionBasis: RecognitionBasisAccrual, FiscalYearStart: 1,
	})

	tx := FinancialTransaction{
		Type: TxTypeInterestAccrual, OrgID: uuid.New(), AmountCents: 15000,
		EffectiveDate: time.Now(), SourceID: uuid.New(), UnitID: ptr(uuid.New()),
	}
	err := engine.ValidateTransaction(context.Background(), tx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds jurisdiction cap")
	assert.Contains(t, err.Error(), "interest accrual")
}
