package policy_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/platform/policy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolutionCache_SetAndGet(t *testing.T) {
	cache := policy.NewResolutionCache(5 * time.Minute)

	unitID := uuid.New()
	category := "fee_schedule"
	policyHash := "abc123"

	res := &policy.Resolution{
		ID:         uuid.New(),
		Status:     "approved",
		Reasoning:  "Fees are within allowable range",
		Confidence: 0.95,
	}

	cache.Set(unitID, category, policyHash, res)

	got, ok := cache.Get(unitID, category, policyHash)
	require.True(t, ok, "expected cache hit")
	assert.Equal(t, res.ID, got.ID)
	assert.Equal(t, res.Status, got.Status)
	assert.Equal(t, res.Reasoning, got.Reasoning)
	assert.Equal(t, res.Confidence, got.Confidence)
}

func TestResolutionCache_Miss(t *testing.T) {
	cache := policy.NewResolutionCache(5 * time.Minute)

	unitID := uuid.New()

	got, ok := cache.Get(unitID, "fee_schedule", "nonexistent")
	assert.False(t, ok, "expected cache miss on empty cache")
	assert.Nil(t, got)
}

func TestResolutionCache_Invalidate(t *testing.T) {
	cache := policy.NewResolutionCache(5 * time.Minute)

	unitID := uuid.New()
	category := "fee_schedule"
	policyHash := "abc123"

	res := &policy.Resolution{
		ID:     uuid.New(),
		Status: "approved",
	}

	cache.Set(unitID, category, policyHash, res)

	// Confirm it's there before invalidation
	_, ok := cache.Get(unitID, category, policyHash)
	require.True(t, ok, "expected cache hit before invalidation")

	// Invalidate by unitID
	cache.Invalidate(&unitID, nil, category)

	got, ok := cache.Get(unitID, category, policyHash)
	assert.False(t, ok, "expected cache miss after invalidation")
	assert.Nil(t, got)
}

func TestResolutionCache_Expiry(t *testing.T) {
	cache := policy.NewResolutionCache(1 * time.Millisecond)

	unitID := uuid.New()
	category := "fee_schedule"
	policyHash := "abc123"

	res := &policy.Resolution{
		ID:     uuid.New(),
		Status: "approved",
	}

	cache.Set(unitID, category, policyHash, res)

	// Wait for TTL to expire
	time.Sleep(5 * time.Millisecond)

	got, ok := cache.Get(unitID, category, policyHash)
	assert.False(t, ok, "expected cache miss after TTL expiry")
	assert.Nil(t, got)
}
