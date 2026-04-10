package license_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/quorant/quorant/internal/license"
)

type countingChecker struct {
	allowed   bool
	remaining int
	calls     int
}

func (c *countingChecker) Check(_ context.Context, _ uuid.UUID, _ string) (bool, int, error) {
	c.calls++
	return c.allowed, c.remaining, nil
}

func setupRedis(t *testing.T) *redis.Client {
	t.Helper()
	s := miniredis.RunT(t)
	return redis.NewClient(&redis.Options{Addr: s.Addr()})
}

func TestCachedChecker_CacheHit(t *testing.T) {
	rdb := setupRedis(t)
	inner := &countingChecker{allowed: true, remaining: 100}
	checker := license.NewCachedEntitlementChecker(inner, rdb, 60*time.Second)

	ctx := context.Background()
	orgID := uuid.New()

	// First call — cache miss, hits inner
	allowed, remaining, err := checker.Check(ctx, orgID, "units.max")
	require.NoError(t, err)
	assert.True(t, allowed)
	assert.Equal(t, 100, remaining)
	assert.Equal(t, 1, inner.calls)

	// Second call — cache hit, does NOT hit inner
	allowed2, remaining2, err := checker.Check(ctx, orgID, "units.max")
	require.NoError(t, err)
	assert.True(t, allowed2)
	assert.Equal(t, 100, remaining2)
	assert.Equal(t, 1, inner.calls) // still 1 — cached
}

func TestCachedChecker_DifferentKeys(t *testing.T) {
	rdb := setupRedis(t)
	inner := &countingChecker{allowed: true, remaining: -1}
	checker := license.NewCachedEntitlementChecker(inner, rdb, 60*time.Second)

	ctx := context.Background()
	orgID := uuid.New()

	checker.Check(ctx, orgID, "feature.a")
	checker.Check(ctx, orgID, "feature.b")

	assert.Equal(t, 2, inner.calls) // different keys = separate cache entries
}

func TestCachedChecker_DeniedResultCached(t *testing.T) {
	rdb := setupRedis(t)
	inner := &countingChecker{allowed: false, remaining: 0}
	checker := license.NewCachedEntitlementChecker(inner, rdb, 60*time.Second)

	ctx := context.Background()
	orgID := uuid.New()

	allowed, _, err := checker.Check(ctx, orgID, "webhooks.enabled")
	require.NoError(t, err)
	assert.False(t, allowed)

	// Cached — still denied
	allowed2, _, err := checker.Check(ctx, orgID, "webhooks.enabled")
	require.NoError(t, err)
	assert.False(t, allowed2)
	assert.Equal(t, 1, inner.calls) // only 1 call to inner
}

func TestCachedChecker_InvalidateOrg(t *testing.T) {
	rdb := setupRedis(t)
	inner := &countingChecker{allowed: true, remaining: 50}
	checker := license.NewCachedEntitlementChecker(inner, rdb, 60*time.Second)

	ctx := context.Background()
	orgID := uuid.New()

	// Populate cache
	checker.Check(ctx, orgID, "feature.a")
	checker.Check(ctx, orgID, "feature.b")
	assert.Equal(t, 2, inner.calls)

	// Invalidate
	err := checker.InvalidateOrg(ctx, orgID)
	require.NoError(t, err)

	// Next calls should miss cache
	checker.Check(ctx, orgID, "feature.a")
	checker.Check(ctx, orgID, "feature.b")
	assert.Equal(t, 4, inner.calls) // 2 more calls after invalidation
}
