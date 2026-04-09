package admin

import (
	"context"

	"github.com/google/uuid"
)

// AdminRepository persists and retrieves admin/platform data.
type AdminRepository interface {
	// Feature Flags

	// CreateFlag inserts a new feature flag and returns the persisted record.
	CreateFlag(ctx context.Context, f *FeatureFlag) (*FeatureFlag, error)

	// FindFlagByID looks up a feature flag by its UUID.
	// Returns nil, nil if not found.
	FindFlagByID(ctx context.Context, id uuid.UUID) (*FeatureFlag, error)

	// ListFlags returns all feature flags.
	ListFlags(ctx context.Context) ([]FeatureFlag, error)

	// UpdateFlag saves changes to an existing feature flag.
	UpdateFlag(ctx context.Context, f *FeatureFlag) (*FeatureFlag, error)

	// SetOverride inserts or replaces an org-level override for a feature flag.
	SetOverride(ctx context.Context, o *FeatureFlagOverride) (*FeatureFlagOverride, error)

	// ListOverridesByFlag returns all overrides for a given flag.
	ListOverridesByFlag(ctx context.Context, flagID uuid.UUID) ([]FeatureFlagOverride, error)

	// IsFlagEnabled checks if a flag is enabled for an org (override > global default).
	IsFlagEnabled(ctx context.Context, flagKey string, orgID uuid.UUID) (bool, error)

	// Tenant Activity

	// RecordActivity inserts a new tenant activity record.
	RecordActivity(ctx context.Context, a *TenantActivity) (*TenantActivity, error)

	// ListActivityByOrg returns all activity records for an org, most recent first.
	ListActivityByOrg(ctx context.Context, orgID uuid.UUID) ([]TenantActivity, error)

	// Tenant listing (queries organizations table)

	// ListTenants returns all organizations as maps (simplified).
	ListTenants(ctx context.Context) ([]map[string]any, error)
}
