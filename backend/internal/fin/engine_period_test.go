package fin

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// periodStub implements AccountingPeriodRepository for period boundary tests.
type periodStub struct {
	period *AccountingPeriod
	err    error
}

func (s *periodStub) CreatePeriod(_ context.Context, p *AccountingPeriod) (*AccountingPeriod, error) {
	return p, nil
}

func (s *periodStub) GetPeriodForDate(_ context.Context, _ uuid.UUID, _ time.Time) (*AccountingPeriod, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.period == nil {
		return nil, fmt.Errorf("no period found")
	}
	return s.period, nil
}

func (s *periodStub) ListPeriodsByFiscalYear(_ context.Context, _ uuid.UUID, _ int) ([]AccountingPeriod, error) {
	return nil, nil
}

func (s *periodStub) UpdatePeriodStatus(_ context.Context, _ uuid.UUID, _ PeriodStatus, _ *uuid.UUID) error {
	return nil
}

func (s *periodStub) AllPeriodsClosedForYear(_ context.Context, _ uuid.UUID, _ int) (bool, error) {
	return false, nil
}

func baseTx() FinancialTransaction {
	return FinancialTransaction{
		Type: TxTypeAssessment, OrgID: uuid.New(), AmountCents: 10000,
		EffectiveDate: time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC),
		SourceID: uuid.New(), UnitID: ptr(uuid.New()),
	}
}

// ── GAAP period boundary tests ─────────────────────────────────────

func TestGaapEngine_ValidateTransaction_OpenPeriod(t *testing.T) {
	periods := &periodStub{period: &AccountingPeriod{Status: PeriodStatusOpen}}
	engine := NewGaapEngine(nil, nil, EngineConfig{RecognitionBasis: RecognitionBasisAccrual, FiscalYearStart: 1}, periods)

	err := engine.ValidateTransaction(context.Background(), baseTx())
	assert.NoError(t, err)
}

func TestGaapEngine_ValidateTransaction_ClosedPeriod(t *testing.T) {
	periods := &periodStub{period: &AccountingPeriod{Status: PeriodStatusClosed}}
	engine := NewGaapEngine(nil, nil, EngineConfig{RecognitionBasis: RecognitionBasisAccrual, FiscalYearStart: 1}, periods)

	err := engine.ValidateTransaction(context.Background(), baseTx())
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrClosedPeriod))
}

func TestGaapEngine_ValidateTransaction_SoftClosedPeriod_AdjustingEntry(t *testing.T) {
	periods := &periodStub{period: &AccountingPeriod{Status: PeriodStatusSoftClosed}}
	engine := NewGaapEngine(nil, nil, EngineConfig{RecognitionBasis: RecognitionBasisAccrual, FiscalYearStart: 1}, periods)

	tx := FinancialTransaction{
		Type: TxTypeAdjustingEntry, OrgID: uuid.New(), AmountCents: 5000,
		EffectiveDate: time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC),
		SourceID:      uuid.New(),
	}
	err := engine.ValidateTransaction(context.Background(), tx)
	assert.NoError(t, err)
}

func TestGaapEngine_ValidateTransaction_SoftClosedPeriod_NonAdjusting(t *testing.T) {
	periods := &periodStub{period: &AccountingPeriod{Status: PeriodStatusSoftClosed}}
	engine := NewGaapEngine(nil, nil, EngineConfig{RecognitionBasis: RecognitionBasisAccrual, FiscalYearStart: 1}, periods)

	err := engine.ValidateTransaction(context.Background(), baseTx())
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrSoftClosedPeriod))
}

func TestGaapEngine_ValidateTransaction_NilPeriods_Allowed(t *testing.T) {
	engine := NewGaapEngine(nil, nil, EngineConfig{RecognitionBasis: RecognitionBasisAccrual, FiscalYearStart: 1})

	err := engine.ValidateTransaction(context.Background(), baseTx())
	assert.NoError(t, err)
}

func TestGaapEngine_ValidateTransaction_PeriodNotFound_Allowed(t *testing.T) {
	periods := &periodStub{err: fmt.Errorf("no period found")}
	engine := NewGaapEngine(nil, nil, EngineConfig{RecognitionBasis: RecognitionBasisAccrual, FiscalYearStart: 1}, periods)

	err := engine.ValidateTransaction(context.Background(), baseTx())
	assert.NoError(t, err)
}

// ── IFRS period boundary tests ─────────────────────────────────────

func TestIfrsEngine_ValidateTransaction_OpenPeriod(t *testing.T) {
	periods := &periodStub{period: &AccountingPeriod{Status: PeriodStatusOpen}}
	engine := NewIfrsEngine(nil, nil, EngineConfig{RecognitionBasis: RecognitionBasisAccrual, FiscalYearStart: 1}, periods)

	err := engine.ValidateTransaction(context.Background(), baseTx())
	assert.NoError(t, err)
}

func TestIfrsEngine_ValidateTransaction_ClosedPeriod(t *testing.T) {
	periods := &periodStub{period: &AccountingPeriod{Status: PeriodStatusClosed}}
	engine := NewIfrsEngine(nil, nil, EngineConfig{RecognitionBasis: RecognitionBasisAccrual, FiscalYearStart: 1}, periods)

	err := engine.ValidateTransaction(context.Background(), baseTx())
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrClosedPeriod))
}

func TestIfrsEngine_ValidateTransaction_SoftClosedPeriod_AdjustingEntry(t *testing.T) {
	periods := &periodStub{period: &AccountingPeriod{Status: PeriodStatusSoftClosed}}
	engine := NewIfrsEngine(nil, nil, EngineConfig{RecognitionBasis: RecognitionBasisAccrual, FiscalYearStart: 1}, periods)

	tx := FinancialTransaction{
		Type: TxTypeAdjustingEntry, OrgID: uuid.New(), AmountCents: 5000,
		EffectiveDate: time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC),
		SourceID:      uuid.New(),
	}
	err := engine.ValidateTransaction(context.Background(), tx)
	assert.NoError(t, err)
}

func TestIfrsEngine_ValidateTransaction_SoftClosedPeriod_NonAdjusting(t *testing.T) {
	periods := &periodStub{period: &AccountingPeriod{Status: PeriodStatusSoftClosed}}
	engine := NewIfrsEngine(nil, nil, EngineConfig{RecognitionBasis: RecognitionBasisAccrual, FiscalYearStart: 1}, periods)

	err := engine.ValidateTransaction(context.Background(), baseTx())
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrSoftClosedPeriod))
}

func TestIfrsEngine_ValidateTransaction_NilPeriods_Allowed(t *testing.T) {
	engine := NewIfrsEngine(nil, nil, EngineConfig{RecognitionBasis: RecognitionBasisAccrual, FiscalYearStart: 1})

	err := engine.ValidateTransaction(context.Background(), baseTx())
	assert.NoError(t, err)
}

func TestIfrsEngine_ValidateTransaction_PeriodNotFound_Allowed(t *testing.T) {
	periods := &periodStub{err: fmt.Errorf("no period found")}
	engine := NewIfrsEngine(nil, nil, EngineConfig{RecognitionBasis: RecognitionBasisAccrual, FiscalYearStart: 1}, periods)

	err := engine.ValidateTransaction(context.Background(), baseTx())
	assert.NoError(t, err)
}
