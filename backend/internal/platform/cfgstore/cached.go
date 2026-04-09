package cfgstore

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// CachedStore wraps a Store with Redis caching for hot-path reads.
type CachedStore struct {
	inner Store
	redis *redis.Client
	ttl   time.Duration
}

// NewCachedStore creates a caching decorator.
func NewCachedStore(inner Store, rdb *redis.Client, ttl time.Duration) *CachedStore {
	return &CachedStore{inner: inner, redis: rdb, ttl: ttl}
}

func cacheKey(orgID uuid.UUID, key string) string {
	return fmt.Sprintf("cfg:%s:%s", orgID, key)
}

type cachedEntry struct {
	Value json.RawMessage `json:"value"`
	Scope Scope           `json:"scope"`
}

func (c *CachedStore) Get(ctx context.Context, orgID uuid.UUID, key string) (json.RawMessage, Scope, error) {
	// Try cache
	ck := cacheKey(orgID, key)
	data, err := c.redis.Get(ctx, ck).Bytes()
	if err == nil {
		var entry cachedEntry
		if json.Unmarshal(data, &entry) == nil {
			return entry.Value, entry.Scope, nil
		}
	}

	// Cache miss — delegate to inner store
	val, scope, err := c.inner.Get(ctx, orgID, key)
	if err != nil {
		return nil, "", err
	}

	// Cache the result
	entry := cachedEntry{Value: val, Scope: scope}
	if raw, marshalErr := json.Marshal(entry); marshalErr == nil {
		_ = c.redis.Set(ctx, ck, raw, c.ttl).Err()
	}

	return val, scope, nil
}

func (c *CachedStore) GetAll(ctx context.Context, orgID uuid.UUID) (map[string]json.RawMessage, error) {
	// GetAll is not cached — it's a bulk operation used less frequently
	return c.inner.GetAll(ctx, orgID)
}

func (c *CachedStore) GetPlatform(ctx context.Context, key string) (json.RawMessage, error) {
	return c.inner.GetPlatform(ctx, key)
}

func (c *CachedStore) Set(ctx context.Context, scope Scope, scopeID *uuid.UUID, key string, value json.RawMessage) error {
	err := c.inner.Set(ctx, scope, scopeID, key, value)
	if err != nil {
		return err
	}
	// Invalidate cache for this key at the scope's org
	// For org scope: invalidate the specific org+key
	// For firm scope: would need to invalidate all managed HOAs (expensive — skip for now)
	if scopeID != nil {
		_ = c.redis.Del(ctx, cacheKey(*scopeID, key)).Err()
	}
	return nil
}

func (c *CachedStore) Delete(ctx context.Context, scope Scope, scopeID *uuid.UUID, key string) error {
	err := c.inner.Delete(ctx, scope, scopeID, key)
	if err != nil {
		return err
	}
	if scopeID != nil {
		_ = c.redis.Del(ctx, cacheKey(*scopeID, key)).Err()
	}
	return nil
}
