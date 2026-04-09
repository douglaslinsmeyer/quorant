package admin

import (
	"context"

	"github.com/google/uuid"
)

// Service defines the business operations for the admin module.
// Handlers depend on this interface rather than the concrete AdminService struct.
type Service interface {
	// Feature Flags
	CreateFlag(ctx context.Context, req CreateFeatureFlagRequest) (*FeatureFlag, error)
	ListFlags(ctx context.Context) ([]FeatureFlag, error)
	UpdateFlag(ctx context.Context, id uuid.UUID, req UpdateFeatureFlagRequest) (*FeatureFlag, error)
	SetOverride(ctx context.Context, flagID uuid.UUID, req SetFlagOverrideRequest) (*FeatureFlagOverride, error)
	IsFlagEnabled(ctx context.Context, flagKey string, orgID uuid.UUID) (bool, error)

	// Tenant Management
	ListTenants(ctx context.Context) ([]map[string]any, error)
	GetTenantDashboard(ctx context.Context, orgID uuid.UUID) (*TenantDashboard, error)
	SuspendTenant(ctx context.Context, orgID uuid.UUID) (map[string]any, error)
	ReactivateTenant(ctx context.Context, orgID uuid.UUID) (map[string]any, error)

	// User Management
	StartImpersonation(ctx context.Context, targetUserID uuid.UUID) (map[string]any, error)
	StopImpersonation(ctx context.Context) (map[string]any, error)
	SearchUsers(ctx context.Context, query string) ([]UserSearchResult, error)
	ResetPassword(ctx context.Context, userID uuid.UUID) (map[string]any, error)
	UnlockAccount(ctx context.Context, userID uuid.UUID) (map[string]any, error)
}
