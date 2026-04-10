package policy_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/quorant/quorant/internal/ai"
	"github.com/quorant/quorant/internal/platform/policy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testLogger returns a logger that discards all output.
func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// --- stubPolicyResolver ---

type stubPolicyResolver struct {
	result *ai.ResolutionResult
	err    error
}

func (s *stubPolicyResolver) GetPolicy(_ context.Context, _ uuid.UUID, _ string) (*ai.PolicyResult, error) {
	return nil, nil
}

func (s *stubPolicyResolver) QueryPolicy(_ context.Context, _ uuid.UUID, _ string, _ ai.QueryContext) (*ai.ResolutionResult, error) {
	return s.result, s.err
}

// --- stubRecordRepo ---

type stubRecordRepo struct {
	records []policy.PolicyRecord
}

func (s *stubRecordRepo) CreateRecord(_ context.Context, r *policy.PolicyRecord) (*policy.PolicyRecord, error) {
	return r, nil
}

func (s *stubRecordRepo) FindRecordByID(_ context.Context, id uuid.UUID) (*policy.PolicyRecord, error) {
	for _, rec := range s.records {
		if rec.ID == id {
			return &rec, nil
		}
	}
	return nil, fmt.Errorf("not found")
}

func (s *stubRecordRepo) GatherForResolution(_ context.Context, category string, _ string, _ uuid.UUID, _ *uuid.UUID) ([]policy.PolicyRecord, error) {
	var matched []policy.PolicyRecord
	for _, rec := range s.records {
		if rec.Category == category {
			matched = append(matched, rec)
		}
	}
	return matched, nil
}

func (s *stubRecordRepo) DeactivateRecord(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (s *stubRecordRepo) WithTx(_ pgx.Tx) policy.PolicyRecordRepository {
	return s
}

// --- stubResolutionRepo ---

type stubResolutionRepo struct {
	resolutions []policy.ResolutionRecord
}

func (s *stubResolutionRepo) CreateResolution(_ context.Context, r *policy.ResolutionRecord) (*policy.ResolutionRecord, error) {
	s.resolutions = append(s.resolutions, *r)
	return r, nil
}

func (s *stubResolutionRepo) FindResolutionByID(_ context.Context, id uuid.UUID) (*policy.ResolutionRecord, error) {
	for _, rec := range s.resolutions {
		if rec.ID == id {
			return &rec, nil
		}
	}
	return nil, fmt.Errorf("not found")
}

func (s *stubResolutionRepo) UpdateReviewStatus(_ context.Context, _ uuid.UUID, _ string, _ *uuid.UUID, _ *string) error {
	return nil
}

func (s *stubResolutionRepo) ListPendingReviews(_ context.Context) ([]policy.ResolutionRecord, error) {
	return nil, nil
}

func (s *stubResolutionRepo) WithTx(_ pgx.Tx) policy.ResolutionRepository {
	return s
}

func TestRegistry_Register(t *testing.T) {
	t.Run("registers a descriptor successfully", func(t *testing.T) {
		reg := policy.NewRegistry(nil, nil, nil, nil, nil)

		desc := policy.OperationDescriptor{
			Category:    "fee_schedule",
			Description: "HOA fee schedule policies",
			Policies: map[string]policy.PolicySpec{
				"late_fee": {
					Description:   "Late payment fee",
					DocumentTypes: []string{"fee_schedule"},
					Concepts:      []string{"late fee", "penalty"},
				},
			},
		}

		err := reg.Register("fee_schedule", desc)
		require.NoError(t, err)
	})

	t.Run("returns error on duplicate category registration", func(t *testing.T) {
		reg := policy.NewRegistry(nil, nil, nil, nil, nil)

		desc := policy.OperationDescriptor{
			Category:    "fee_schedule",
			Description: "HOA fee schedule policies",
		}

		err := reg.Register("fee_schedule", desc)
		require.NoError(t, err)

		err = reg.Register("fee_schedule", desc)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "fee_schedule")
	})

	t.Run("allows registering multiple distinct categories", func(t *testing.T) {
		reg := policy.NewRegistry(nil, nil, nil, nil, nil)

		err := reg.Register("fee_schedule", policy.OperationDescriptor{Category: "fee_schedule"})
		require.NoError(t, err)

		err = reg.Register("reserve_fund", policy.OperationDescriptor{Category: "reserve_fund"})
		require.NoError(t, err)
	})
}

func TestRegistry_FindTriggers(t *testing.T) {
	reg := policy.NewRegistry(nil, nil, nil, nil, nil)

	// Register a descriptor with two policies
	err := reg.Register("fee_schedule", policy.OperationDescriptor{
		Category: "fee_schedule",
		Policies: map[string]policy.PolicySpec{
			"late_fee": {
				Description:   "Late payment fee",
				DocumentTypes: []string{"fee_schedule", "payment_policy"},
				Concepts:      []string{"late fee", "penalty"},
			},
			"special_assessment": {
				Description:   "Special assessment policy",
				DocumentTypes: []string{"special_assessment"},
				Concepts:      []string{"special assessment", "capital improvement"},
			},
		},
	})
	require.NoError(t, err)

	err = reg.Register("reserve_fund", policy.OperationDescriptor{
		Category: "reserve_fund",
		Policies: map[string]policy.PolicySpec{
			"reserve_contribution": {
				Description:   "Reserve fund contribution",
				DocumentTypes: []string{"reserve_study"},
				Concepts:      []string{"reserve fund", "depreciation"},
			},
		},
	})
	require.NoError(t, err)

	t.Run("returns matching trigger on exact document type match", func(t *testing.T) {
		triggers := reg.FindTriggers("fee_schedule", nil)

		require.Len(t, triggers, 1)
		assert.Equal(t, "fee_schedule", triggers[0].Category)
		assert.Equal(t, "late_fee", triggers[0].Key)
	})

	t.Run("returns multiple triggers when document type matches multiple specs", func(t *testing.T) {
		triggers := reg.FindTriggers("special_assessment", nil)

		require.Len(t, triggers, 1)
		assert.Equal(t, "fee_schedule", triggers[0].Category)
		assert.Equal(t, "special_assessment", triggers[0].Key)
	})

	t.Run("returns empty slice when no document type or concept matches", func(t *testing.T) {
		triggers := reg.FindTriggers("unknown_doc_type", nil)

		require.NotNil(t, triggers)
		assert.Empty(t, triggers)
	})

	t.Run("returns matching trigger on concept match (case-insensitive)", func(t *testing.T) {
		triggers := reg.FindTriggers("", []string{"Late Fee"})

		require.Len(t, triggers, 1)
		assert.Equal(t, "fee_schedule", triggers[0].Category)
		assert.Equal(t, "late_fee", triggers[0].Key)
	})

	t.Run("matches concept case-insensitively with different casing", func(t *testing.T) {
		triggers := reg.FindTriggers("", []string{"RESERVE FUND"})

		require.Len(t, triggers, 1)
		assert.Equal(t, "reserve_fund", triggers[0].Category)
		assert.Equal(t, "reserve_contribution", triggers[0].Key)
	})

	t.Run("returns empty slice when document type is empty and no concepts provided", func(t *testing.T) {
		triggers := reg.FindTriggers("", nil)

		require.NotNil(t, triggers)
		assert.Empty(t, triggers)
	})

	t.Run("returns empty slice when document type is empty and concepts do not match", func(t *testing.T) {
		triggers := reg.FindTriggers("", []string{"nonexistent concept"})

		require.NotNil(t, triggers)
		assert.Empty(t, triggers)
	})

	t.Run("matches on both document type and concept returning distinct triggers", func(t *testing.T) {
		// "reserve_study" matches reserve_contribution by doc type; "late fee" matches late_fee by concept
		triggers := reg.FindTriggers("reserve_study", []string{"late fee"})

		require.Len(t, triggers, 2)
		keys := make(map[string]bool)
		for _, tr := range triggers {
			keys[tr.Key] = true
		}
		assert.True(t, keys["reserve_contribution"])
		assert.True(t, keys["late_fee"])
	})
}

// --- Resolve tests ---

func newTestRecords() []policy.PolicyRecord {
	return []policy.PolicyRecord{
		{
			ID:           uuid.New(),
			Scope:        "org",
			Category:     "fee_schedule",
			Key:          "late_fee",
			Value:        json.RawMessage(`{"amount": 50}`),
			PriorityHint: "medium",
			IsActive:     true,
		},
	}
}

func newTestDescriptor(onHold func(ctx context.Context, res *policy.Resolution) error) policy.OperationDescriptor {
	return policy.OperationDescriptor{
		Category:         "fee_schedule",
		Description:      "HOA fee schedule policies",
		DefaultThreshold: 0.80,
		PromptTemplate:   "Resolve fees given: {{.Policies}}",
		Policies: map[string]policy.PolicySpec{
			"late_fee": {
				Description:   "Late payment fee",
				DocumentTypes: []string{"fee_schedule"},
				Concepts:      []string{"late fee"},
			},
		},
		OnHold: onHold,
	}
}

func TestRegistry_Resolve_AutoApproved(t *testing.T) {
	ctx := context.Background()
	orgID := uuid.New()

	records := newTestRecords()
	recordRepo := &stubRecordRepo{records: records}
	resolutionRepo := &stubResolutionRepo{}
	aiResolver := &stubPolicyResolver{
		result: &ai.ResolutionResult{
			Resolution: json.RawMessage(`{"late_fee": 50}`),
			Reasoning:  "Based on org policy document section 4.2",
			Confidence: 0.95,
		},
	}

	reg := policy.NewRegistry(recordRepo, resolutionRepo, aiResolver, nil, testLogger())
	err := reg.Register("fee_schedule", newTestDescriptor(nil))
	require.NoError(t, err)

	res, err := reg.Resolve(ctx, orgID, nil, "fee_schedule")
	require.NoError(t, err)
	require.NotNil(t, res)

	assert.Equal(t, "approved", res.Status)
	assert.False(t, res.Held())
	assert.InDelta(t, 0.95, res.Confidence, 0.001)
	assert.Equal(t, "Based on org policy document section 4.2", res.Reasoning)
	assert.NotEmpty(t, res.SourcePolicies)
	assert.NotEqual(t, uuid.Nil, res.ID)

	// Verify resolution was persisted
	require.Len(t, resolutionRepo.resolutions, 1)
	assert.Equal(t, "auto_approved", resolutionRepo.resolutions[0].ReviewStatus)
}

func TestRegistry_Resolve_Held(t *testing.T) {
	ctx := context.Background()
	orgID := uuid.New()

	records := newTestRecords()
	recordRepo := &stubRecordRepo{records: records}
	resolutionRepo := &stubResolutionRepo{}
	aiResolver := &stubPolicyResolver{
		result: &ai.ResolutionResult{
			Resolution: json.RawMessage(`{"late_fee": 50}`),
			Reasoning:  "Conflicting policies found",
			Confidence: 0.60,
		},
	}

	var holdCalled bool
	onHold := func(_ context.Context, res *policy.Resolution) error {
		holdCalled = true
		return nil
	}

	reg := policy.NewRegistry(recordRepo, resolutionRepo, aiResolver, nil, testLogger())
	err := reg.Register("fee_schedule", newTestDescriptor(onHold))
	require.NoError(t, err)

	res, err := reg.Resolve(ctx, orgID, nil, "fee_schedule")
	require.NoError(t, err)
	require.NotNil(t, res)

	assert.Equal(t, "held", res.Status)
	assert.True(t, res.Held())
	assert.InDelta(t, 0.60, res.Confidence, 0.001)
	assert.True(t, holdCalled, "OnHold callback should have been invoked")

	// Verify resolution was persisted with pending_review status
	require.Len(t, resolutionRepo.resolutions, 1)
	assert.Equal(t, "pending_review", resolutionRepo.resolutions[0].ReviewStatus)
}

func TestRegistry_Resolve_AIUnavailable(t *testing.T) {
	ctx := context.Background()
	orgID := uuid.New()

	records := newTestRecords()
	recordRepo := &stubRecordRepo{records: records}
	resolutionRepo := &stubResolutionRepo{}
	aiResolver := &stubPolicyResolver{
		err: fmt.Errorf("connection refused"),
	}

	var holdCalled bool
	onHold := func(_ context.Context, res *policy.Resolution) error {
		holdCalled = true
		return nil
	}

	reg := policy.NewRegistry(recordRepo, resolutionRepo, aiResolver, nil, testLogger())
	err := reg.Register("fee_schedule", newTestDescriptor(onHold))
	require.NoError(t, err)

	res, err := reg.Resolve(ctx, orgID, nil, "fee_schedule")
	require.NoError(t, err, "AI errors should not propagate as Resolve errors")
	require.NotNil(t, res)

	assert.Equal(t, "held", res.Status)
	assert.True(t, res.Held())
	assert.Equal(t, float64(0), res.Confidence)
	assert.True(t, holdCalled, "OnHold callback should have been invoked")

	// Verify resolution was persisted with ai_unavailable status
	require.Len(t, resolutionRepo.resolutions, 1)
	assert.Equal(t, "ai_unavailable", resolutionRepo.resolutions[0].ReviewStatus)
}

func TestRegistry_Resolve_UnregisteredCategory(t *testing.T) {
	ctx := context.Background()
	orgID := uuid.New()

	reg := policy.NewRegistry(nil, nil, nil, nil, testLogger())

	res, err := reg.Resolve(ctx, orgID, nil, "nonexistent_category")
	require.Error(t, err)
	assert.Nil(t, res)
	assert.Contains(t, err.Error(), "nonexistent_category")
}
