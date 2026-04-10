package policy

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// cachedResolution wraps a Resolution with an expiry timestamp.
type cachedResolution struct {
	resolution *Resolution
	expiresAt  time.Time
}

// ResolutionCache is an in-memory TTL cache for policy resolutions keyed by
// unit, category, and policy hash. It avoids redundant Tier 2 AI calls when
// the same policy set has already been resolved for a given unit.
type ResolutionCache struct {
	mu         sync.RWMutex
	store      map[string]*cachedResolution
	defaultTTL time.Duration
}

// NewResolutionCache constructs a ResolutionCache with the given default TTL.
func NewResolutionCache(defaultTTL time.Duration) *ResolutionCache {
	return &ResolutionCache{
		store:      make(map[string]*cachedResolution),
		defaultTTL: defaultTTL,
	}
}

// cacheKey builds the lookup key for the given unit, category, and policy hash.
// Format: "unitID:category:policyHash"
func cacheKey(unitID uuid.UUID, category, policyHash string) string {
	return fmt.Sprintf("%s:%s:%s", unitID.String(), category, policyHash)
}

// Get retrieves a resolution from the cache. Returns the resolution and true on
// a valid hit, or nil and false on a miss or an expired entry.
func (c *ResolutionCache) Get(unitID uuid.UUID, category, policyHash string) (*Resolution, bool) {
	key := cacheKey(unitID, category, policyHash)

	c.mu.RLock()
	entry, exists := c.store[key]
	c.mu.RUnlock()

	if !exists {
		return nil, false
	}

	if time.Now().After(entry.expiresAt) {
		return nil, false
	}

	return entry.resolution, true
}

// Set stores a resolution in the cache with the default TTL.
func (c *ResolutionCache) Set(unitID uuid.UUID, category, policyHash string, res *Resolution) {
	key := cacheKey(unitID, category, policyHash)

	c.mu.Lock()
	defer c.mu.Unlock()

	c.store[key] = &cachedResolution{
		resolution: res,
		expiresAt:  time.Now().Add(c.defaultTTL),
	}
}

// Invalidate removes cached resolutions from the store. When unitID is
// provided, only entries whose key begins with "unitID:category:" are removed.
// When unitID is nil, all entries containing ":category:" are removed,
// covering org- and jurisdiction-level invalidation.
func (c *ResolutionCache) Invalidate(unitID *uuid.UUID, orgID *uuid.UUID, category string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if unitID != nil {
		prefix := fmt.Sprintf("%s:%s:", unitID.String(), category)
		for key := range c.store {
			if strings.HasPrefix(key, prefix) {
				delete(c.store, key)
			}
		}
		return
	}

	substring := fmt.Sprintf(":%s:", category)
	for key := range c.store {
		if strings.Contains(key, substring) {
			delete(c.store, key)
		}
	}
}
