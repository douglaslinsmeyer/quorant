package fin

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/quorant/quorant/internal/ai"
	"github.com/quorant/quorant/internal/platform/policy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPaymentApplicationStrategy_DesignatedInvoice(t *testing.T) {
	engine := newTestGaapEngine()
	invoiceID := uuid.New()

	strategy, err := engine.PaymentApplicationStrategy(context.Background(), PaymentContext{
		OrgID:             uuid.New(),
		PaymentID:         uuid.New(),
		PayerID:           uuid.New(),
		AmountCents:       10000,
		DesignatedInvoice: &invoiceID,
	})

	require.NoError(t, err)
	require.NotNil(t, strategy)
	assert.Equal(t, ApplicationMethodDesignated, strategy.Method)
}

func TestPaymentApplicationStrategy_NilRegistry_DefaultsOldestFirst(t *testing.T) {
	// newTestGaapEngine creates an engine with nil registry.
	engine := newTestGaapEngine()

	strategy, err := engine.PaymentApplicationStrategy(context.Background(), PaymentContext{
		OrgID:       uuid.New(),
		PaymentID:   uuid.New(),
		PayerID:     uuid.New(),
		AmountCents: 10000,
	})

	require.NoError(t, err)
	require.NotNil(t, strategy)
	assert.Equal(t, ApplicationMethodOldestFirst, strategy.Method)
	assert.Equal(t, SortOldestFirst, strategy.WithinPriority)
}

func TestPaymentApplicationStrategy_RegistryWithPriorityOrder(t *testing.T) {
	ruling := AllocationRuling{
		PriorityOrder: []ChargeType{
			ChargeTypeRegularAssessment,
			ChargeTypeLateFee,
			ChargeTypeInterest,
		},
		AcceptPartial:  true,
		CreditHandling: "credit_on_account",
	}
	rulingJSON, err := json.Marshal(ruling)
	require.NoError(t, err)

	registry := policy.NewRegistry(
		&stubPolicyRecordRepo{records: nil},
		nil,
		&stubAIPolicyResolver{ruling: rulingJSON, confidence: 0.95},
		nil,
		nil,
	)
	err = registry.Register("payment_allocation_rules", policy.OperationDescriptor{
		Category:         "payment_allocation_rules",
		PromptTemplate:   "Given these policies: {{.Policies}}",
		DefaultThreshold: 0.80,
	})
	require.NoError(t, err)

	engine := NewGaapEngine(nil, registry, EngineConfig{
		RecognitionBasis: RecognitionBasisAccrual,
		FiscalYearStart:  1,
	})

	strategy, err := engine.PaymentApplicationStrategy(context.Background(), PaymentContext{
		OrgID:       uuid.New(),
		PaymentID:   uuid.New(),
		PayerID:     uuid.New(),
		AmountCents: 10000,
	})

	require.NoError(t, err)
	require.NotNil(t, strategy)
	assert.Equal(t, ApplicationMethodPriorityFIFO, strategy.Method)
	assert.Equal(t, []ChargeType{
		ChargeTypeRegularAssessment,
		ChargeTypeLateFee,
		ChargeTypeInterest,
	}, strategy.PriorityOrder)
	assert.Equal(t, SortOldestFirst, strategy.WithinPriority)
}

func TestPaymentApplicationStrategy_DesignatedOverridesPolicy(t *testing.T) {
	// Even with a registry configured, designated invoice takes precedence.
	ruling := AllocationRuling{
		PriorityOrder: []ChargeType{ChargeTypeRegularAssessment},
	}
	rulingJSON, err := json.Marshal(ruling)
	require.NoError(t, err)

	registry := policy.NewRegistry(
		&stubPolicyRecordRepo{records: nil},
		nil,
		&stubAIPolicyResolver{ruling: rulingJSON, confidence: 0.95},
		nil,
		nil,
	)
	err = registry.Register("payment_allocation_rules", policy.OperationDescriptor{
		Category:         "payment_allocation_rules",
		PromptTemplate:   "Given these policies: {{.Policies}}",
		DefaultThreshold: 0.80,
	})
	require.NoError(t, err)

	engine := NewGaapEngine(nil, registry, EngineConfig{
		RecognitionBasis: RecognitionBasisAccrual,
		FiscalYearStart:  1,
	})

	invoiceID := uuid.New()
	strategy, err := engine.PaymentApplicationStrategy(context.Background(), PaymentContext{
		OrgID:             uuid.New(),
		PaymentID:         uuid.New(),
		PayerID:           uuid.New(),
		AmountCents:       10000,
		DesignatedInvoice: &invoiceID,
	})

	require.NoError(t, err)
	require.NotNil(t, strategy)
	assert.Equal(t, ApplicationMethodDesignated, strategy.Method)
}

// ── Test stubs for policy infrastructure ─────────────────────────────

// stubPolicyRecordRepo satisfies policy.PolicyRecordRepository for tests.
type stubPolicyRecordRepo struct {
	records []policy.PolicyRecord
}

func (s *stubPolicyRecordRepo) CreateRecord(_ context.Context, r *policy.PolicyRecord) (*policy.PolicyRecord, error) {
	return r, nil
}

func (s *stubPolicyRecordRepo) FindRecordByID(_ context.Context, _ uuid.UUID) (*policy.PolicyRecord, error) {
	return nil, nil
}

func (s *stubPolicyRecordRepo) GatherForResolution(_ context.Context, _ string, _ string, _ uuid.UUID, _ *uuid.UUID) ([]policy.PolicyRecord, error) {
	return s.records, nil
}

func (s *stubPolicyRecordRepo) DeactivateRecord(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (s *stubPolicyRecordRepo) WithTx(_ pgx.Tx) policy.PolicyRecordRepository {
	return s
}

// stubAIPolicyResolver satisfies ai.PolicyResolver for tests.
type stubAIPolicyResolver struct {
	ruling     json.RawMessage
	confidence float64
}

func (s *stubAIPolicyResolver) GetPolicy(_ context.Context, _ uuid.UUID, _ string) (*ai.PolicyResult, error) {
	return nil, nil
}

func (s *stubAIPolicyResolver) QueryPolicy(_ context.Context, _ uuid.UUID, _ string, _ ai.QueryContext) (*ai.ResolutionResult, error) {
	return &ai.ResolutionResult{
		Resolution: s.ruling,
		Reasoning:  "test ruling",
		Confidence: s.confidence,
	}, nil
}
