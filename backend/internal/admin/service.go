package admin

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/audit"
	"github.com/quorant/quorant/internal/platform/api"
	"github.com/quorant/quorant/internal/platform/middleware"
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

// SuspendTenant suspends a tenant by setting its subscription status to 'suspended'.
func (s *AdminService) SuspendTenant(ctx context.Context, orgID uuid.UUID) (map[string]any, error) {
	if err := s.repo.SuspendTenant(ctx, orgID); err != nil {
		return nil, fmt.Errorf("suspending tenant: %w", err)
	}
	_ = s.auditor.Record(ctx, audit.AuditEntry{
		OrgID:        orgID,
		ActorID:      middleware.UserIDFromContext(ctx),
		Action:       "admin.tenant.suspended",
		ResourceType: "organization",
		ResourceID:   orgID,
		Module:       "admin",
		OccurredAt:   time.Now().UTC(),
	})
	return map[string]any{"status": "suspended"}, nil
}

// ReactivateTenant reactivates a suspended tenant by setting its subscription status to 'active'.
func (s *AdminService) ReactivateTenant(ctx context.Context, orgID uuid.UUID) (map[string]any, error) {
	if err := s.repo.ReactivateTenant(ctx, orgID); err != nil {
		return nil, fmt.Errorf("reactivating tenant: %w", err)
	}
	_ = s.auditor.Record(ctx, audit.AuditEntry{
		OrgID:        orgID,
		ActorID:      middleware.UserIDFromContext(ctx),
		Action:       "admin.tenant.reactivated",
		ResourceType: "organization",
		ResourceID:   orgID,
		Module:       "admin",
		OccurredAt:   time.Now().UTC(),
	})
	return map[string]any{"status": "active"}, nil
}

// StartImpersonation records an audit attempt and returns an error because
// impersonation requires Zitadel API integration that is not yet implemented.
func (s *AdminService) StartImpersonation(ctx context.Context, targetUserID uuid.UUID) (map[string]any, error) {
	_ = s.auditor.Record(ctx, audit.AuditEntry{
		ActorID:      middleware.UserIDFromContext(ctx),
		Action:       "admin.impersonation.attempted",
		ResourceType: "user",
		ResourceID:   targetUserID,
		Module:       "admin",
		OccurredAt:   time.Now().UTC(),
	})
	return nil, api.NewUnprocessableError("impersonation requires Zitadel API integration (not yet implemented)")
}

// StopImpersonation records an audit attempt and returns an error because
// impersonation requires Zitadel API integration that is not yet implemented.
func (s *AdminService) StopImpersonation(ctx context.Context) (map[string]any, error) {
	actorID := middleware.UserIDFromContext(ctx)
	_ = s.auditor.Record(ctx, audit.AuditEntry{
		ActorID:      actorID,
		Action:       "admin.impersonation.stop_attempted",
		ResourceType: "user",
		ResourceID:   actorID,
		Module:       "admin",
		OccurredAt:   time.Now().UTC(),
	})
	return nil, api.NewUnprocessableError("impersonation requires Zitadel API integration (not yet implemented)")
}

// SearchUsers searches users by email or display name.
func (s *AdminService) SearchUsers(ctx context.Context, query string) ([]UserSearchResult, error) {
	results, err := s.repo.SearchUsers(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("searching users: %w", err)
	}
	return results, nil
}

// ResetPassword records an audit attempt and returns an error because
// password management is handled by Zitadel and is not yet implemented.
func (s *AdminService) ResetPassword(ctx context.Context, userID uuid.UUID) (map[string]any, error) {
	_ = s.auditor.Record(ctx, audit.AuditEntry{
		ActorID:      middleware.UserIDFromContext(ctx),
		Action:       "admin.password_reset.attempted",
		ResourceType: "user",
		ResourceID:   userID,
		Module:       "admin",
		OccurredAt:   time.Now().UTC(),
	})
	return nil, api.NewUnprocessableError("password reset requires Zitadel admin API integration (not yet implemented)")
}

// UnlockAccount re-activates a user account and records an audit entry.
func (s *AdminService) UnlockAccount(ctx context.Context, userID uuid.UUID) (map[string]any, error) {
	if err := s.repo.UnlockAccount(ctx, userID); err != nil {
		return nil, fmt.Errorf("unlocking account: %w", err)
	}
	_ = s.auditor.Record(ctx, audit.AuditEntry{
		ActorID:      middleware.UserIDFromContext(ctx),
		Action:       "admin.account_unlocked",
		ResourceType: "user",
		ResourceID:   userID,
		Module:       "admin",
		OccurredAt:   time.Now().UTC(),
	})
	return map[string]any{"status": "unlocked"}, nil
}
