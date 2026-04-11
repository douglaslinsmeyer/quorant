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

// ── Overpayment policy tests (GAAP) ──────────────────────────────────

func TestGaapEngine_ValidateTransaction_Payment_RejectOverpayment(t *testing.T) {
	ruling, _ := json.Marshal(overpaymentRuling{Action: OverpaymentActionReject})
	registry := newRegistryWithCap(t, "overpayment_policy", ruling)

	engine := NewGaapEngine(nil, registry, EngineConfig{
		RecognitionBasis: RecognitionBasisAccrual, FiscalYearStart: 1,
	})

	tx := FinancialTransaction{
		Type: TxTypePayment, OrgID: uuid.New(), AmountCents: 50000,
		EffectiveDate: time.Now(), SourceID: uuid.New(), UnitID: ptr(uuid.New()),
		Metadata: map[string]any{
			"overpayment_cents":  int64(10000),
			"unit_balance_cents": int64(40000),
		},
	}
	err := engine.ValidateTransaction(context.Background(), tx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "overpayment")
	assert.Contains(t, err.Error(), "exceeds")
}

func TestGaapEngine_ValidateTransaction_Payment_AcceptOverpayment(t *testing.T) {
	ruling, _ := json.Marshal(overpaymentRuling{Action: OverpaymentActionAccept})
	registry := newRegistryWithCap(t, "overpayment_policy", ruling)

	engine := NewGaapEngine(nil, registry, EngineConfig{
		RecognitionBasis: RecognitionBasisAccrual, FiscalYearStart: 1,
	})

	tx := FinancialTransaction{
		Type: TxTypePayment, OrgID: uuid.New(), AmountCents: 50000,
		EffectiveDate: time.Now(), SourceID: uuid.New(), UnitID: ptr(uuid.New()),
		Metadata: map[string]any{
			"overpayment_cents":  int64(10000),
			"unit_balance_cents": int64(40000),
		},
	}
	err := engine.ValidateTransaction(context.Background(), tx)
	assert.NoError(t, err)
}

func TestGaapEngine_ValidateTransaction_Payment_CapOverpayment_WithinCap(t *testing.T) {
	maxPct := 0.10 // 10% over balance is ok
	ruling, _ := json.Marshal(overpaymentRuling{
		Action:            OverpaymentActionCap,
		MaxOverpayPercent: &maxPct,
	})
	registry := newRegistryWithCap(t, "overpayment_policy", ruling)

	engine := NewGaapEngine(nil, registry, EngineConfig{
		RecognitionBasis: RecognitionBasisAccrual, FiscalYearStart: 1,
	})

	// Balance 40000, payment 43000 => overpayment 3000 = 7.5% of balance (within 10%)
	tx := FinancialTransaction{
		Type: TxTypePayment, OrgID: uuid.New(), AmountCents: 43000,
		EffectiveDate: time.Now(), SourceID: uuid.New(), UnitID: ptr(uuid.New()),
		Metadata: map[string]any{
			"overpayment_cents":  int64(3000),
			"unit_balance_cents": int64(40000),
		},
	}
	err := engine.ValidateTransaction(context.Background(), tx)
	assert.NoError(t, err)
}

func TestGaapEngine_ValidateTransaction_Payment_CapOverpayment_ExceedsCap(t *testing.T) {
	maxPct := 0.10 // 10% over balance is ok
	ruling, _ := json.Marshal(overpaymentRuling{
		Action:            OverpaymentActionCap,
		MaxOverpayPercent: &maxPct,
	})
	registry := newRegistryWithCap(t, "overpayment_policy", ruling)

	engine := NewGaapEngine(nil, registry, EngineConfig{
		RecognitionBasis: RecognitionBasisAccrual, FiscalYearStart: 1,
	})

	// Balance 40000, payment 50000 => overpayment 10000 = 25% of balance (exceeds 10%)
	tx := FinancialTransaction{
		Type: TxTypePayment, OrgID: uuid.New(), AmountCents: 50000,
		EffectiveDate: time.Now(), SourceID: uuid.New(), UnitID: ptr(uuid.New()),
		Metadata: map[string]any{
			"overpayment_cents":  int64(10000),
			"unit_balance_cents": int64(40000),
		},
	}
	err := engine.ValidateTransaction(context.Background(), tx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "overpayment")
	assert.Contains(t, err.Error(), "exceeds")
}

func TestGaapEngine_ValidateTransaction_Payment_NoPolicy_Allows(t *testing.T) {
	// Nil registry means no overpayment policy — default to accept (GAAP).
	engine := NewGaapEngine(nil, nil, EngineConfig{
		RecognitionBasis: RecognitionBasisAccrual, FiscalYearStart: 1,
	})

	tx := FinancialTransaction{
		Type: TxTypePayment, OrgID: uuid.New(), AmountCents: 999999,
		EffectiveDate: time.Now(), SourceID: uuid.New(), UnitID: ptr(uuid.New()),
		Metadata: map[string]any{
			"overpayment_cents":  int64(900000),
			"unit_balance_cents": int64(99999),
		},
	}
	err := engine.ValidateTransaction(context.Background(), tx)
	assert.NoError(t, err)
}

func TestGaapEngine_ValidateTransaction_Payment_NoOverpayment_Allows(t *testing.T) {
	// Even with reject policy, no overpayment means no rejection.
	ruling, _ := json.Marshal(overpaymentRuling{Action: OverpaymentActionReject})
	registry := newRegistryWithCap(t, "overpayment_policy", ruling)

	engine := NewGaapEngine(nil, registry, EngineConfig{
		RecognitionBasis: RecognitionBasisAccrual, FiscalYearStart: 1,
	})

	tx := FinancialTransaction{
		Type: TxTypePayment, OrgID: uuid.New(), AmountCents: 40000,
		EffectiveDate: time.Now(), SourceID: uuid.New(), UnitID: ptr(uuid.New()),
		Metadata: map[string]any{
			"overpayment_cents":  int64(0),
			"unit_balance_cents": int64(40000),
		},
	}
	err := engine.ValidateTransaction(context.Background(), tx)
	assert.NoError(t, err)
}

// ── Overpayment policy tests (IFRS) ──────────────────────────────────

func TestIfrsEngine_ValidateTransaction_Payment_RejectOverpayment(t *testing.T) {
	ruling, _ := json.Marshal(overpaymentRuling{Action: OverpaymentActionReject})
	registry := newRegistryWithCap(t, "overpayment_policy", ruling)

	engine := NewIfrsEngine(nil, registry, EngineConfig{
		RecognitionBasis: RecognitionBasisAccrual, FiscalYearStart: 1,
	})

	tx := FinancialTransaction{
		Type: TxTypePayment, OrgID: uuid.New(), AmountCents: 50000,
		EffectiveDate: time.Now(), SourceID: uuid.New(), UnitID: ptr(uuid.New()),
		Metadata: map[string]any{
			"overpayment_cents":  int64(10000),
			"unit_balance_cents": int64(40000),
		},
	}
	err := engine.ValidateTransaction(context.Background(), tx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "overpayment")
	assert.Contains(t, err.Error(), "exceeds")
}

func TestIfrsEngine_ValidateTransaction_Payment_AcceptOverpayment(t *testing.T) {
	ruling, _ := json.Marshal(overpaymentRuling{Action: OverpaymentActionAccept})
	registry := newRegistryWithCap(t, "overpayment_policy", ruling)

	engine := NewIfrsEngine(nil, registry, EngineConfig{
		RecognitionBasis: RecognitionBasisAccrual, FiscalYearStart: 1,
	})

	tx := FinancialTransaction{
		Type: TxTypePayment, OrgID: uuid.New(), AmountCents: 50000,
		EffectiveDate: time.Now(), SourceID: uuid.New(), UnitID: ptr(uuid.New()),
		Metadata: map[string]any{
			"overpayment_cents":  int64(10000),
			"unit_balance_cents": int64(40000),
		},
	}
	err := engine.ValidateTransaction(context.Background(), tx)
	assert.NoError(t, err)
}

func TestIfrsEngine_ValidateTransaction_Payment_NoPolicy_Allows(t *testing.T) {
	engine := NewIfrsEngine(nil, nil, EngineConfig{
		RecognitionBasis: RecognitionBasisAccrual, FiscalYearStart: 1,
	})

	tx := FinancialTransaction{
		Type: TxTypePayment, OrgID: uuid.New(), AmountCents: 999999,
		EffectiveDate: time.Now(), SourceID: uuid.New(), UnitID: ptr(uuid.New()),
		Metadata: map[string]any{
			"overpayment_cents":  int64(900000),
			"unit_balance_cents": int64(99999),
		},
	}
	err := engine.ValidateTransaction(context.Background(), tx)
	assert.NoError(t, err)
}
