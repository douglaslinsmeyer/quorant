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

	orgID := uuid.New()
	category := "fee_schedule"
	policyHash := "abc123"

	res := &policy.Resolution{
		ID:         uuid.New(),
		Status:     "approved",
		Reasoning:  "Fees are within allowable range",
		Confidence: 0.95,
	}

	cache.Set(orgID, nil, category, policyHash, res)

	got, ok := cache.Get(orgID, nil, category, policyHash)
	require.True(t, ok, "expected cache hit")
	assert.Equal(t, res.ID, got.ID)
	assert.Equal(t, res.Status, got.Status)
	assert.Equal(t, res.Reasoning, got.Reasoning)
	assert.Equal(t, res.Confidence, got.Confidence)
}

func TestResolutionCache_SetAndGet_WithUnitID(t *testing.T) {
	cache := policy.NewResolutionCache(5 * time.Minute)

	orgID := uuid.New()
	unitID := uuid.New()
	category := "fee_schedule"
	policyHash := "abc123"

	res := &policy.Resolution{
		ID:     uuid.New(),
		Status: "approved",
	}

	cache.Set(orgID, &unitID, category, policyHash, res)

	got, ok := cache.Get(orgID, &unitID, category, policyHash)
	require.True(t, ok, "expected cache hit with unitID")
	assert.Equal(t, res.ID, got.ID)
}

func TestResolutionCache_OrgIsolation(t *testing.T) {
	cache := policy.NewResolutionCache(5 * time.Minute)

	org1 := uuid.New()
	org2 := uuid.New()
	category := "fee_schedule"
	policyHash := "same-hash"

	res := &policy.Resolution{ID: uuid.New(), Status: "approved"}
	cache.Set(org1, nil, category, policyHash, res)

	got, ok := cache.Get(org2, nil, category, policyHash)
	assert.False(t, ok, "different orgID should miss")
	assert.Nil(t, got)
}

func TestResolutionCache_Miss(t *testing.T) {
	cache := policy.NewResolutionCache(5 * time.Minute)

	orgID := uuid.New()

	got, ok := cache.Get(orgID, nil, "fee_schedule", "nonexistent")
	assert.False(t, ok, "expected cache miss on empty cache")
	assert.Nil(t, got)
}

func TestResolutionCache_Invalidate_ByOrgAndUnit(t *testing.T) {
	cache := policy.NewResolutionCache(5 * time.Minute)

	orgID := uuid.New()
	unitID := uuid.New()
	category := "fee_schedule"

	res := &policy.Resolution{ID: uuid.New(), Status: "approved"}
	cache.Set(orgID, &unitID, category, "hash1", res)

	_, ok := cache.Get(orgID, &unitID, category, "hash1")
	require.True(t, ok, "expected cache hit before invalidation")

	cache.Invalidate(&orgID, &unitID, category)

	got, ok := cache.Get(orgID, &unitID, category, "hash1")
	assert.False(t, ok, "expected cache miss after invalidation")
	assert.Nil(t, got)
}

func TestResolutionCache_Invalidate_ByOrgOnly(t *testing.T) {
	cache := policy.NewResolutionCache(5 * time.Minute)

	orgID := uuid.New()
	otherOrg := uuid.New()
	category := "fee_schedule"

	res := &policy.Resolution{ID: uuid.New(), Status: "approved"}
	cache.Set(orgID, nil, category, "hash1", res)
	cache.Set(otherOrg, nil, category, "hash2", res)

	cache.Invalidate(&orgID, nil, category)

	_, ok := cache.Get(orgID, nil, category, "hash1")
	assert.False(t, ok, "orgID entry should be invalidated")

	_, ok = cache.Get(otherOrg, nil, category, "hash2")
	assert.True(t, ok, "other org entry should survive org-scoped invalidation")
}

func TestResolutionCache_Invalidate_ByCategoryOnly(t *testing.T) {
	cache := policy.NewResolutionCache(5 * time.Minute)

	org1 := uuid.New()
	org2 := uuid.New()
	category := "fee_schedule"

	res := &policy.Resolution{ID: uuid.New(), Status: "approved"}
	cache.Set(org1, nil, category, "hash1", res)
	cache.Set(org2, nil, category, "hash2", res)
	cache.Set(org1, nil, "reserve_fund", "hash3", res)

	cache.Invalidate(nil, nil, category)

	_, ok := cache.Get(org1, nil, category, "hash1")
	assert.False(t, ok, "org1 fee_schedule should be invalidated")

	_, ok = cache.Get(org2, nil, category, "hash2")
	assert.False(t, ok, "org2 fee_schedule should be invalidated")

	_, ok = cache.Get(org1, nil, "reserve_fund", "hash3")
	assert.True(t, ok, "different category should survive category-only invalidation")
}

func TestResolutionCache_Expiry(t *testing.T) {
	cache := policy.NewResolutionCache(1 * time.Millisecond)

	orgID := uuid.New()
	category := "fee_schedule"
	policyHash := "abc123"

	res := &policy.Resolution{ID: uuid.New(), Status: "approved"}
	cache.Set(orgID, nil, category, policyHash, res)

	time.Sleep(5 * time.Millisecond)

	got, ok := cache.Get(orgID, nil, category, policyHash)
	assert.False(t, ok, "expected cache miss after TTL expiry")
	assert.Nil(t, got)
}
