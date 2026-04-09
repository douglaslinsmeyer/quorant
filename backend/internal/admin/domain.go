package admin

import (
	"time"

	"github.com/google/uuid"
)

// FeatureFlag is a platform-level flag that can be toggled globally or per-org.
type FeatureFlag struct {
	ID          uuid.UUID `json:"id"`
	Key         string    `json:"key"`
	Description *string   `json:"description,omitempty"`
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// FeatureFlagOverride is an org-level override of a feature flag's enabled state.
type FeatureFlagOverride struct {
	ID        uuid.UUID `json:"id"`
	FlagID    uuid.UUID `json:"flag_id"`
	OrgID     uuid.UUID `json:"org_id"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
}

// TenantActivity records a platform metric for an organization at a point in time.
type TenantActivity struct {
	ID          int64     `json:"id"`
	OrgID       uuid.UUID `json:"org_id"`
	MetricType  string    `json:"metric_type"` // active_users, storage_bytes, api_calls
	Value       int64     `json:"value"`
	PeriodStart time.Time `json:"period_start"`
	PeriodEnd   time.Time `json:"period_end"`
	CreatedAt   time.Time `json:"created_at"`
}

// TenantDashboard is the response for GET /admin/tenants/{org_id}.
type TenantDashboard struct {
	Org            any              `json:"org"`  // Organization details (from org module)
	ActiveUsers    int64            `json:"active_users"`
	StorageBytes   int64            `json:"storage_bytes"`
	RecentActivity []TenantActivity `json:"recent_activity"`
}
