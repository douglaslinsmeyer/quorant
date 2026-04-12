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
// org, optional unit, category, and policy hash. It avoids redundant Tier 2 AI
// calls when the same policy set has already been resolved.
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

// cacheKey builds the lookup key. Format:
//   - With unitID: "orgID:unitID:category:policyHash"
//   - Without unitID: "orgID:category:policyHash"
func cacheKey(orgID uuid.UUID, unitID *uuid.UUID, category, policyHash string) string {
	if unitID != nil {
		return fmt.Sprintf("%s:%s:%s:%s", orgID.String(), unitID.String(), category, policyHash)
	}
	return fmt.Sprintf("%s:%s:%s", orgID.String(), category, policyHash)
}

// Get retrieves a resolution from the cache. Returns the resolution and true on
// a valid hit, or nil and false on a miss or an expired entry.
func (c *ResolutionCache) Get(orgID uuid.UUID, unitID *uuid.UUID, category, policyHash string) (*Resolution, bool) {
	key := cacheKey(orgID, unitID, category, policyHash)

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
func (c *ResolutionCache) Set(orgID uuid.UUID, unitID *uuid.UUID, category, policyHash string, res *Resolution) {
	key := cacheKey(orgID, unitID, category, policyHash)

	c.mu.Lock()
	defer c.mu.Unlock()

	c.store[key] = &cachedResolution{
		resolution: res,
		expiresAt:  time.Now().Add(c.defaultTTL),
	}
}

// Invalidate removes cached resolutions from the store.
//
// Scoping rules (applied in order):
//   - orgID + unitID + category: removes entries matching all three
//   - orgID + category (unitID nil): removes all entries for that org/category
//   - category only (orgID nil): removes all entries across all orgs for that category
func (c *ResolutionCache) Invalidate(orgID *uuid.UUID, unitID *uuid.UUID, category string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var substring string
	switch {
	case orgID != nil && unitID != nil:
		substring = fmt.Sprintf("%s:%s:%s:", orgID.String(), unitID.String(), category)
	case orgID != nil:
		substring = fmt.Sprintf("%s:", orgID.String())
	default:
		substring = fmt.Sprintf(":%s:", category)
	}

	for key := range c.store {
		if strings.Contains(key, substring) {
			delete(c.store, key)
		}
	}
}
