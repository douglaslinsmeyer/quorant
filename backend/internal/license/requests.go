package license

import (
	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/platform/api"
)

// CreatePlanRequest is the input for creating a new Plan.
type CreatePlanRequest struct {
	Name        string         `json:"name"`
	Description *string        `json:"description,omitempty"`
	PlanType    string         `json:"plan_type"` // required: 'firm', 'hoa', 'firm_bundle'
	PriceCents  int64          `json:"price_cents"`
	IsActive    bool           `json:"is_active"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// Validate ensures required fields are present.
func (r CreatePlanRequest) Validate() error {
	if r.Name == "" {
		return api.NewValidationError("name is required", "name")
	}
	switch r.PlanType {
	case "firm", "hoa", "firm_bundle":
		// valid
	default:
		return api.NewValidationError(`plan_type must be one of: firm, hoa, firm_bundle`, "plan_type")
	}
	return nil
}

// CreateSubscriptionRequest is the input for creating an OrgSubscription.
type CreateSubscriptionRequest struct {
	OrgID  uuid.UUID `json:"org_id"`
	PlanID uuid.UUID `json:"plan_id"`
}

// Validate ensures OrgID and PlanID are non-zero.
func (r CreateSubscriptionRequest) Validate() error {
	if r.OrgID == (uuid.UUID{}) {
		return api.NewValidationError("org_id is required", "org_id")
	}
	if r.PlanID == (uuid.UUID{}) {
		return api.NewValidationError("plan_id is required", "plan_id")
	}
	return nil
}

// UpsertOverrideRequest is the input for creating or replacing an EntitlementOverride.
type UpsertOverrideRequest struct {
	OrgID      uuid.UUID  `json:"org_id"`
	FeatureKey string     `json:"feature_key"`
	LimitValue *int64     `json:"limit_value,omitempty"`
	Reason     *string    `json:"reason,omitempty"`
	GrantedBy  *uuid.UUID `json:"granted_by,omitempty"`
	ExpiresAt  *string    `json:"expires_at,omitempty"` // RFC3339 optional
}

// Validate ensures OrgID and FeatureKey are present.
func (r UpsertOverrideRequest) Validate() error {
	if r.OrgID == (uuid.UUID{}) {
		return api.NewValidationError("org_id is required", "org_id")
	}
	if r.FeatureKey == "" {
		return api.NewValidationError("feature_key is required", "feature_key")
	}
	return nil
}
