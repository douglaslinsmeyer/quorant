package license

import (
	"time"

	"github.com/google/uuid"
)

// Plan represents a licensable subscription tier.
type Plan struct {
	ID          uuid.UUID      `json:"id"`
	Name        string         `json:"name"`
	Description *string        `json:"description,omitempty"`
	PlanType    string         `json:"plan_type"` // 'firm', 'hoa', 'firm_bundle'
	PriceCents  int64          `json:"price_cents"`
	IsActive    bool           `json:"is_active"`
	Metadata    map[string]any `json:"metadata"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

// Entitlement defines a feature access rule attached to a Plan.
type Entitlement struct {
	ID         uuid.UUID `json:"id"`
	PlanID     uuid.UUID `json:"plan_id"`
	FeatureKey string    `json:"feature_key"`
	LimitType  string    `json:"limit_type"` // 'boolean', 'numeric', 'rate'
	LimitValue *int64    `json:"limit_value,omitempty"` // nil for boolean
	CreatedAt  time.Time `json:"created_at"`
}

// OrgSubscription links an organization to a Plan.
type OrgSubscription struct {
	ID          uuid.UUID  `json:"id"`
	OrgID       uuid.UUID  `json:"org_id"`
	PlanID      uuid.UUID  `json:"plan_id"`
	Status      string     `json:"status"` // 'active', 'trial', 'suspended', 'cancelled'
	StartsAt    time.Time  `json:"starts_at"`
	EndsAt      *time.Time `json:"ends_at,omitempty"`
	TrialEndsAt *time.Time `json:"trial_ends_at,omitempty"`
	StripeSubID *string    `json:"stripe_sub_id,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// EntitlementOverride allows manual per-org entitlement adjustments.
type EntitlementOverride struct {
	ID         uuid.UUID  `json:"id"`
	OrgID      uuid.UUID  `json:"org_id"`
	FeatureKey string     `json:"feature_key"`
	LimitValue *int64     `json:"limit_value,omitempty"`
	Reason     *string    `json:"reason,omitempty"`
	GrantedBy  *uuid.UUID `json:"granted_by,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
}

// UsageRecord records metered feature usage for an organization.
type UsageRecord struct {
	ID          uuid.UUID `json:"id"`
	OrgID       uuid.UUID `json:"org_id"`
	FeatureKey  string    `json:"feature_key"`
	Quantity    int64     `json:"quantity"`
	PeriodStart time.Time `json:"period_start"`
	PeriodEnd   time.Time `json:"period_end"`
	RecordedAt  time.Time `json:"recorded_at"`
}
