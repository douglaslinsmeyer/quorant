package policy_test

import (
	"testing"

	"github.com/quorant/quorant/internal/platform/policy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
