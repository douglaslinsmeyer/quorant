package license

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// PostgresEntitlementChecker implements the EntitlementChecker interface.
//
// Resolution order:
//  1. Check org_entitlement_overrides for orgID + featureKey — if found, use it
//  2. Check org_subscriptions for orgID — if active, resolve from plan's entitlements
//  3. If org is an HOA with an active firm via organizations_management:
//     a. Check firm's org_subscription — if plan_type = 'firm_bundle', resolve from firm's entitlements
//  4. Deny (no active entitlement found)
type PostgresEntitlementChecker struct {
	repo LicenseRepository
}

// NewPostgresEntitlementChecker constructs a PostgresEntitlementChecker backed by repo.
func NewPostgresEntitlementChecker(repo LicenseRepository) *PostgresEntitlementChecker {
	return &PostgresEntitlementChecker{repo: repo}
}

// Check resolves whether orgID has access to featureKey.
// Returns (allowed, remaining, err).
// remaining == -1 signals unlimited.
func (c *PostgresEntitlementChecker) Check(ctx context.Context, orgID uuid.UUID, featureKey string) (bool, int, error) {
	// 1. Check overrides.
	override, err := c.repo.FindOverride(ctx, orgID, featureKey)
	if err != nil {
		return false, 0, err
	}
	if override != nil {
		// Skip expired overrides and fall through.
		if override.ExpiresAt == nil || !override.ExpiresAt.Before(time.Now()) {
			if override.LimitValue == nil {
				return true, -1, nil // boolean: allowed, unlimited
			}
			return true, int(*override.LimitValue), nil
		}
	}

	// 2. Check org's direct subscription.
	ent, err := c.repo.FindEntitlementFromSubscription(ctx, orgID, featureKey)
	if err != nil {
		return false, 0, err
	}
	if ent != nil {
		return resolveEntitlement(ent)
	}

	// 3. Check firm bundle (HOA managed by a firm with a firm_bundle plan).
	ent, err = c.repo.FindEntitlementFromFirmBundle(ctx, orgID, featureKey)
	if err != nil {
		return false, 0, err
	}
	if ent != nil {
		return resolveEntitlement(ent)
	}

	// 4. Deny.
	return false, 0, nil
}

// resolveEntitlement translates an Entitlement into the (allowed, remaining, err) tuple.
func resolveEntitlement(ent *Entitlement) (bool, int, error) {
	switch ent.LimitType {
	case "boolean":
		return true, -1, nil
	case "numeric", "rate":
		if ent.LimitValue != nil {
			return true, int(*ent.LimitValue), nil
		}
		return true, -1, nil
	default:
		return false, 0, nil
	}
}
