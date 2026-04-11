package fin

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGaapEngine_RevenueRecognitionDate_Assessment_Accrual(t *testing.T) {
	engine := NewGaapEngine(nil, nil, EngineConfig{
		RecognitionBasis: RecognitionBasisAccrual,
		FiscalYearStart:  1,
	})
	effective := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)

	tx := FinancialTransaction{
		Type:          TxTypeAssessment,
		OrgID:         uuid.New(),
		AmountCents:   25000,
		EffectiveDate: effective,
		SourceID:      uuid.New(),
		UnitID:        ptr(uuid.New()),
	}

	got, err := engine.RevenueRecognitionDate(context.Background(), tx)
	require.NoError(t, err)
	assert.Equal(t, effective, got, "accrual assessment should recognize on EffectiveDate")
}

func TestGaapEngine_RevenueRecognitionDate_CashBasis(t *testing.T) {
	engine := NewGaapEngine(nil, nil, EngineConfig{
		RecognitionBasis: RecognitionBasisCash,
		FiscalYearStart:  1,
	})
	effective := time.Date(2026, 5, 15, 0, 0, 0, 0, time.UTC)

	tx := FinancialTransaction{
		Type:          TxTypePayment,
		OrgID:         uuid.New(),
		AmountCents:   15000,
		EffectiveDate: effective,
		SourceID:      uuid.New(),
		UnitID:        ptr(uuid.New()),
	}

	got, err := engine.RevenueRecognitionDate(context.Background(), tx)
	require.NoError(t, err)
	assert.Equal(t, effective, got, "cash basis should recognize on EffectiveDate")
}

func TestGaapEngine_RevenueRecognitionDate_LateFee(t *testing.T) {
	engine := NewGaapEngine(nil, nil, EngineConfig{
		RecognitionBasis: RecognitionBasisAccrual,
		FiscalYearStart:  1,
	})
	effective := time.Date(2026, 4, 16, 0, 0, 0, 0, time.UTC)

	tx := FinancialTransaction{
		Type:          TxTypeLateFee,
		OrgID:         uuid.New(),
		AmountCents:   2500,
		EffectiveDate: effective,
		SourceID:      uuid.New(),
		UnitID:        ptr(uuid.New()),
	}

	got, err := engine.RevenueRecognitionDate(context.Background(), tx)
	require.NoError(t, err)
	assert.Equal(t, effective, got, "late fee should recognize on EffectiveDate")
}

func TestGaapEngine_RevenueRecognitionDate_InterestAccrual(t *testing.T) {
	engine := NewGaapEngine(nil, nil, EngineConfig{
		RecognitionBasis: RecognitionBasisAccrual,
		FiscalYearStart:  1,
	})
	effective := time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC)

	tx := FinancialTransaction{
		Type:          TxTypeInterestAccrual,
		OrgID:         uuid.New(),
		AmountCents:   1200,
		EffectiveDate: effective,
		SourceID:      uuid.New(),
		UnitID:        ptr(uuid.New()),
	}

	got, err := engine.RevenueRecognitionDate(context.Background(), tx)
	require.NoError(t, err)
	assert.Equal(t, effective, got, "interest accrual should recognize on EffectiveDate")
}
