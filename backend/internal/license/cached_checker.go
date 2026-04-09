package license

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// CachedEntitlementChecker wraps an EntitlementChecker with Redis caching.
// Cache entries have a configurable TTL (default 60 seconds).
// On cache miss or Redis error, it falls back to the underlying checker (fail-open for cache, not for entitlements).
type CachedEntitlementChecker struct {
	inner EntitlementChecker
	redis *redis.Client
	ttl   time.Duration
}

// NewCachedEntitlementChecker creates a caching decorator around an existing checker.
func NewCachedEntitlementChecker(inner EntitlementChecker, rdb *redis.Client, ttl time.Duration) *CachedEntitlementChecker {
	return &CachedEntitlementChecker{
		inner: inner,
		redis: rdb,
		ttl:   ttl,
	}
}

type cachedResult struct {
	Allowed   bool `json:"allowed"`
	Remaining int  `json:"remaining"`
}

func cacheKey(orgID uuid.UUID, featureKey string) string {
	return fmt.Sprintf("entitlement:%s:%s", orgID, featureKey)
}

// Check first looks in Redis cache. On miss, calls the underlying checker and caches the result.
func (c *CachedEntitlementChecker) Check(ctx context.Context, orgID uuid.UUID, featureKey string) (bool, int, error) {
	key := cacheKey(orgID, featureKey)

	// Try cache first
	val, err := c.redis.Get(ctx, key).Bytes()
	if err == nil {
		var result cachedResult
		if json.Unmarshal(val, &result) == nil {
			return result.Allowed, result.Remaining, nil
		}
	}

	// Cache miss or error — call underlying checker
	allowed, remaining, err := c.inner.Check(ctx, orgID, featureKey)
	if err != nil {
		return false, 0, err
	}

	// Cache the result (best-effort — don't fail if Redis is down)
	result := cachedResult{Allowed: allowed, Remaining: remaining}
	if data, marshalErr := json.Marshal(result); marshalErr == nil {
		_ = c.redis.Set(ctx, key, data, c.ttl).Err()
	}

	return allowed, remaining, nil
}

// InvalidateOrg removes all cached entitlements for an org.
// Call this when a subscription changes.
func (c *CachedEntitlementChecker) InvalidateOrg(ctx context.Context, orgID uuid.UUID) error {
	pattern := fmt.Sprintf("entitlement:%s:*", orgID)
	iter := c.redis.Scan(ctx, 0, pattern, 100).Iterator()
	for iter.Next(ctx) {
		_ = c.redis.Del(ctx, iter.Val()).Err()
	}
	return iter.Err()
}
