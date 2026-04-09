package license

import (
	"context"

	"github.com/google/uuid"
)

// Service defines the business operations for the license module.
// Handlers depend on this interface rather than the concrete LicenseService struct.
type Service interface {
	// Plans
	CreatePlan(ctx context.Context, req CreatePlanRequest) (*Plan, error)
	ListPlans(ctx context.Context) ([]Plan, error)
	GetPlan(ctx context.Context, id uuid.UUID) (*Plan, error)
	UpdatePlan(ctx context.Context, id uuid.UUID, p *Plan) (*Plan, error)
	ListPlanEntitlements(ctx context.Context, planID uuid.UUID) ([]Entitlement, error)

	// Subscriptions
	CreateSubscription(ctx context.Context, orgID uuid.UUID, req CreateSubscriptionRequest) (*OrgSubscription, error)
	GetSubscription(ctx context.Context, orgID uuid.UUID) (*OrgSubscription, error)
	UpdateSubscription(ctx context.Context, orgID uuid.UUID, s2 *OrgSubscription) (*OrgSubscription, error)

	// Entitlements
	CheckEntitlements(ctx context.Context, orgID uuid.UUID) ([]EntitlementResult, error)
	SetOverride(ctx context.Context, orgID uuid.UUID, req UpsertOverrideRequest) (*EntitlementOverride, error)
	GetUsage(ctx context.Context, orgID uuid.UUID) ([]UsageRecord, error)
}
