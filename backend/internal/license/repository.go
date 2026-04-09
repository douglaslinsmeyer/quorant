package license

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// LicenseRepository defines persistence operations for the license domain.
type LicenseRepository interface {
	// Plans
	CreatePlan(ctx context.Context, p *Plan) (*Plan, error)
	FindPlanByID(ctx context.Context, id uuid.UUID) (*Plan, error)
	ListPlans(ctx context.Context) ([]Plan, error)
	UpdatePlan(ctx context.Context, p *Plan) (*Plan, error)
	ListEntitlementsByPlan(ctx context.Context, planID uuid.UUID) ([]Entitlement, error)

	// Subscriptions
	CreateSubscription(ctx context.Context, s *OrgSubscription) (*OrgSubscription, error)
	FindActiveSubscription(ctx context.Context, orgID uuid.UUID) (*OrgSubscription, error)
	UpdateSubscription(ctx context.Context, s *OrgSubscription) (*OrgSubscription, error)

	// Overrides
	UpsertOverride(ctx context.Context, o *EntitlementOverride) (*EntitlementOverride, error)
	ListOverridesByOrg(ctx context.Context, orgID uuid.UUID) ([]EntitlementOverride, error)

	// Usage
	RecordUsage(ctx context.Context, r *UsageRecord) (*UsageRecord, error)
	GetUsageByOrg(ctx context.Context, orgID uuid.UUID, featureKey string, periodStart, periodEnd time.Time) (int64, error)

	// Entitlement resolution (used by PostgresEntitlementChecker)
	// FindOverride returns the override for an org+feature, nil if none.
	FindOverride(ctx context.Context, orgID uuid.UUID, featureKey string) (*EntitlementOverride, error)
	// FindEntitlementFromSubscription gets the entitlement from the org's active subscription.
	FindEntitlementFromSubscription(ctx context.Context, orgID uuid.UUID, featureKey string) (*Entitlement, error)
	// FindEntitlementFromFirmBundle gets the entitlement from the managing firm's bundle plan.
	FindEntitlementFromFirmBundle(ctx context.Context, hoaOrgID uuid.UUID, featureKey string) (*Entitlement, error)
}
