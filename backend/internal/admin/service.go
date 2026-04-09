package admin

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/audit"
	"github.com/quorant/quorant/internal/platform/queue"
)

// AdminService provides business logic for admin/platform operations.
type AdminService struct {
	repo      AdminRepository
	auditor   audit.Auditor
	publisher queue.Publisher
	logger    *slog.Logger
}

// NewAdminService constructs an AdminService backed by the given repository.
func NewAdminService(repo AdminRepository, auditor audit.Auditor, publisher queue.Publisher, logger *slog.Logger) *AdminService {
	return &AdminService{repo: repo, auditor: auditor, publisher: publisher, logger: logger}
}

// ─── Feature Flags ────────────────────────────────────────────────────────────

// CreateFlag creates a new feature flag.
func (s *AdminService) CreateFlag(ctx context.Context, req CreateFeatureFlagRequest) (*FeatureFlag, error) {
	f := &FeatureFlag{
		Key:         req.Key,
		Description: req.Description,
		Enabled:     req.Enabled,
	}
	created, err := s.repo.CreateFlag(ctx, f)
	if err != nil {
		return nil, fmt.Errorf("admin: CreateFlag: %w", err)
	}
	return created, nil
}

// ListFlags returns all feature flags.
func (s *AdminService) ListFlags(ctx context.Context) ([]FeatureFlag, error) {
	flags, err := s.repo.ListFlags(ctx)
	if err != nil {
		return nil, fmt.Errorf("admin: ListFlags: %w", err)
	}
	return flags, nil
}

// UpdateFlag applies changes to an existing feature flag.
func (s *AdminService) UpdateFlag(ctx context.Context, id uuid.UUID, req UpdateFeatureFlagRequest) (*FeatureFlag, error) {
	existing, err := s.repo.FindFlagByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("admin: UpdateFlag find: %w", err)
	}
	if existing == nil {
		return nil, fmt.Errorf("admin: UpdateFlag: flag not found")
	}

	if req.Description != nil {
		existing.Description = req.Description
	}
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}

	updated, err := s.repo.UpdateFlag(ctx, existing)
	if err != nil {
		return nil, fmt.Errorf("admin: UpdateFlag: %w", err)
	}
	return updated, nil
}

// SetOverride sets an org-level override for a feature flag.
func (s *AdminService) SetOverride(ctx context.Context, flagID uuid.UUID, req SetFlagOverrideRequest) (*FeatureFlagOverride, error) {
	orgID, err := uuid.Parse(req.OrgID)
	if err != nil {
		return nil, fmt.Errorf("admin: SetOverride: invalid org_id: %w", err)
	}

	o := &FeatureFlagOverride{
		FlagID:  flagID,
		OrgID:   orgID,
		Enabled: req.Enabled,
	}
	result, err := s.repo.SetOverride(ctx, o)
	if err != nil {
		return nil, fmt.Errorf("admin: SetOverride: %w", err)
	}
	return result, nil
}

// IsFlagEnabled checks if a flag is enabled for an org (override > global default).
func (s *AdminService) IsFlagEnabled(ctx context.Context, flagKey string, orgID uuid.UUID) (bool, error) {
	enabled, err := s.repo.IsFlagEnabled(ctx, flagKey, orgID)
	if err != nil {
		return false, fmt.Errorf("admin: IsFlagEnabled: %w", err)
	}
	return enabled, nil
}

// ─── Tenant Management ────────────────────────────────────────────────────────

// ListTenants returns all tenants (organizations).
func (s *AdminService) ListTenants(ctx context.Context) ([]map[string]any, error) {
	tenants, err := s.repo.ListTenants(ctx)
	if err != nil {
		return nil, fmt.Errorf("admin: ListTenants: %w", err)
	}
	return tenants, nil
}

// GetTenantDashboard returns a dashboard summary for a tenant.
func (s *AdminService) GetTenantDashboard(ctx context.Context, orgID uuid.UUID) (*TenantDashboard, error) {
	// Fetch recent activity for the org.
	activities, err := s.repo.ListActivityByOrg(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("admin: GetTenantDashboard: %w", err)
	}

	// Compute aggregate metrics from activity log.
	var activeUsers, storageBytes int64
	for _, a := range activities {
		switch a.MetricType {
		case "active_users":
			if a.Value > activeUsers {
				activeUsers = a.Value
			}
		case "storage_bytes":
			if a.Value > storageBytes {
				storageBytes = a.Value
			}
		}
	}

	return &TenantDashboard{
		Org:            map[string]any{"id": orgID},
		ActiveUsers:    activeUsers,
		StorageBytes:   storageBytes,
		RecentActivity: activities,
	}, nil
}

// SuspendTenant suspends a tenant (placeholder).
func (s *AdminService) SuspendTenant(ctx context.Context, orgID uuid.UUID) (map[string]any, error) {
	// TODO: implement tenant suspension (set organization status to suspended)
	s.logger.Info("SuspendTenant called (placeholder)", "org_id", orgID)
	return map[string]any{"status": "ok", "org_id": orgID, "action": "suspended"}, nil
}

// ReactivateTenant reactivates a suspended tenant (placeholder).
func (s *AdminService) ReactivateTenant(ctx context.Context, orgID uuid.UUID) (map[string]any, error) {
	// TODO: implement tenant reactivation (set organization status to active)
	s.logger.Info("ReactivateTenant called (placeholder)", "org_id", orgID)
	return map[string]any{"status": "ok", "org_id": orgID, "action": "reactivated"}, nil
}

// StartImpersonation initiates an admin impersonation session (placeholder).
func (s *AdminService) StartImpersonation(ctx context.Context, targetUserID uuid.UUID) (map[string]any, error) {
	// TODO: implement impersonation token issuance via Zitadel
	s.logger.Info("StartImpersonation called (placeholder)", "target_user_id", targetUserID)
	return map[string]any{"status": "ok", "target_user_id": targetUserID, "action": "impersonation_started"}, nil
}

// StopImpersonation ends an active admin impersonation session (placeholder).
func (s *AdminService) StopImpersonation(ctx context.Context) (map[string]any, error) {
	// TODO: implement impersonation session termination
	s.logger.Info("StopImpersonation called (placeholder)")
	return map[string]any{"status": "ok", "action": "impersonation_stopped"}, nil
}

// SearchUsers searches users by query (placeholder).
func (s *AdminService) SearchUsers(ctx context.Context, query string) (map[string]any, error) {
	// TODO: implement user search across the users table
	s.logger.Info("SearchUsers called (placeholder)", "query", query)
	return map[string]any{"status": "ok", "query": query, "results": []any{}}, nil
}

// ResetPassword initiates a password reset for a user (placeholder).
func (s *AdminService) ResetPassword(ctx context.Context, userID uuid.UUID) (map[string]any, error) {
	// TODO: implement password reset via Zitadel admin API
	s.logger.Info("ResetPassword called (placeholder)", "user_id", userID)
	return map[string]any{"status": "ok", "user_id": userID, "action": "password_reset_initiated"}, nil
}

// UnlockAccount unlocks a locked user account (placeholder).
func (s *AdminService) UnlockAccount(ctx context.Context, userID uuid.UUID) (map[string]any, error) {
	// TODO: implement account unlock via Zitadel admin API
	s.logger.Info("UnlockAccount called (placeholder)", "user_id", userID)
	return map[string]any{"status": "ok", "user_id": userID, "action": "account_unlocked"}, nil
}
