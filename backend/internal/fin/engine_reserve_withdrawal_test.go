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

// ── Reserve withdrawal policy tests (GAAP) ───────────────────────────

func TestGaapEngine_ValidateTransaction_FundTransfer_ReserveRequiresApproval(t *testing.T) {
	ruling, _ := json.Marshal(reserveWithdrawalRuling{RequiresApproval: true})
	registry := newRegistryWithCap(t, "reserve_withdrawal_policy", ruling)

	engine := NewGaapEngine(nil, registry, EngineConfig{
		RecognitionBasis: RecognitionBasisAccrual, FiscalYearStart: 1,
	})

	tx := FinancialTransaction{
		Type: TxTypeFundTransfer, OrgID: uuid.New(), AmountCents: 50000,
		EffectiveDate: time.Now(), SourceID: uuid.New(),
		FundAllocations: []FundAllocation{
			{FundID: uuid.New(), FundKey: "reserve", AmountCents: 50000},
			{FundID: uuid.New(), FundKey: "operating", AmountCents: 50000},
		},
		Metadata: map[string]any{
			"from_fund_type": "reserve",
			"to_fund_type":   "operating",
		},
	}
	err := engine.ValidateTransaction(context.Background(), tx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reserve")
	assert.Contains(t, err.Error(), "approval")
}

func TestGaapEngine_ValidateTransaction_FundTransfer_ReserveWithApproval(t *testing.T) {
	ruling, _ := json.Marshal(reserveWithdrawalRuling{RequiresApproval: true})
	registry := newRegistryWithCap(t, "reserve_withdrawal_policy", ruling)

	engine := NewGaapEngine(nil, registry, EngineConfig{
		RecognitionBasis: RecognitionBasisAccrual, FiscalYearStart: 1,
	})

	approverID := uuid.New()
	tx := FinancialTransaction{
		Type: TxTypeFundTransfer, OrgID: uuid.New(), AmountCents: 50000,
		EffectiveDate: time.Now(), SourceID: uuid.New(),
		FundAllocations: []FundAllocation{
			{FundID: uuid.New(), FundKey: "reserve", AmountCents: 50000},
			{FundID: uuid.New(), FundKey: "operating", AmountCents: 50000},
		},
		Metadata: map[string]any{
			"from_fund_type": "reserve",
			"to_fund_type":   "operating",
			"approved_by":    approverID.String(),
		},
	}
	err := engine.ValidateTransaction(context.Background(), tx)
	assert.NoError(t, err)
}

func TestGaapEngine_ValidateTransaction_FundTransfer_NoPolicy_AllowsReserve(t *testing.T) {
	// Nil registry means no reserve withdrawal policy — allow.
	engine := NewGaapEngine(nil, nil, EngineConfig{
		RecognitionBasis: RecognitionBasisAccrual, FiscalYearStart: 1,
	})

	tx := FinancialTransaction{
		Type: TxTypeFundTransfer, OrgID: uuid.New(), AmountCents: 50000,
		EffectiveDate: time.Now(), SourceID: uuid.New(),
		FundAllocations: []FundAllocation{
			{FundID: uuid.New(), FundKey: "reserve", AmountCents: 50000},
			{FundID: uuid.New(), FundKey: "operating", AmountCents: 50000},
		},
		Metadata: map[string]any{
			"from_fund_type": "reserve",
			"to_fund_type":   "operating",
		},
	}
	err := engine.ValidateTransaction(context.Background(), tx)
	assert.NoError(t, err)
}

func TestGaapEngine_ValidateTransaction_FundTransfer_OperatingNoApprovalNeeded(t *testing.T) {
	// Operating-to-reserve transfer should not require approval even with policy.
	ruling, _ := json.Marshal(reserveWithdrawalRuling{RequiresApproval: true})
	registry := newRegistryWithCap(t, "reserve_withdrawal_policy", ruling)

	engine := NewGaapEngine(nil, registry, EngineConfig{
		RecognitionBasis: RecognitionBasisAccrual, FiscalYearStart: 1,
	})

	tx := FinancialTransaction{
		Type: TxTypeFundTransfer, OrgID: uuid.New(), AmountCents: 50000,
		EffectiveDate: time.Now(), SourceID: uuid.New(),
		FundAllocations: []FundAllocation{
			{FundID: uuid.New(), FundKey: "operating", AmountCents: 50000},
			{FundID: uuid.New(), FundKey: "reserve", AmountCents: 50000},
		},
		Metadata: map[string]any{
			"from_fund_type": "operating",
			"to_fund_type":   "reserve",
		},
	}
	err := engine.ValidateTransaction(context.Background(), tx)
	assert.NoError(t, err)
}

// ── Reserve withdrawal policy tests (IFRS) ───────────────────────────

func TestIfrsEngine_ValidateTransaction_FundTransfer_ReserveRequiresApproval(t *testing.T) {
	ruling, _ := json.Marshal(reserveWithdrawalRuling{RequiresApproval: true})
	registry := newRegistryWithCap(t, "reserve_withdrawal_policy", ruling)

	engine := NewIfrsEngine(nil, registry, EngineConfig{
		RecognitionBasis: RecognitionBasisAccrual, FiscalYearStart: 1,
	})

	tx := FinancialTransaction{
		Type: TxTypeFundTransfer, OrgID: uuid.New(), AmountCents: 50000,
		EffectiveDate: time.Now(), SourceID: uuid.New(),
		FundAllocations: []FundAllocation{
			{FundID: uuid.New(), FundKey: "reserve", AmountCents: 50000},
			{FundID: uuid.New(), FundKey: "operating", AmountCents: 50000},
		},
		Metadata: map[string]any{
			"from_fund_type": "reserve",
			"to_fund_type":   "operating",
		},
	}
	err := engine.ValidateTransaction(context.Background(), tx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reserve")
	assert.Contains(t, err.Error(), "approval")
}

func TestIfrsEngine_ValidateTransaction_FundTransfer_NoPolicy_AllowsReserve(t *testing.T) {
	engine := NewIfrsEngine(nil, nil, EngineConfig{
		RecognitionBasis: RecognitionBasisAccrual, FiscalYearStart: 1,
	})

	tx := FinancialTransaction{
		Type: TxTypeFundTransfer, OrgID: uuid.New(), AmountCents: 50000,
		EffectiveDate: time.Now(), SourceID: uuid.New(),
		FundAllocations: []FundAllocation{
			{FundID: uuid.New(), FundKey: "reserve", AmountCents: 50000},
			{FundID: uuid.New(), FundKey: "operating", AmountCents: 50000},
		},
		Metadata: map[string]any{
			"from_fund_type": "reserve",
			"to_fund_type":   "operating",
		},
	}
	err := engine.ValidateTransaction(context.Background(), tx)
	assert.NoError(t, err)
}
