package license

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/audit"
	"github.com/quorant/quorant/internal/platform/api"
	"github.com/quorant/quorant/internal/platform/queue"
)

// EntitlementResult holds the resolution outcome for a single feature key.
type EntitlementResult struct {
	FeatureKey string `json:"feature_key"`
	Allowed    bool   `json:"allowed"`
	Remaining  int    `json:"remaining"` // -1 = unlimited
}

// LicenseService provides business logic for the license domain.
type LicenseService struct {
	repo      LicenseRepository
	checker   EntitlementChecker
	auditor   audit.Auditor
	publisher queue.Publisher
	logger    *slog.Logger
}

// NewLicenseService constructs a LicenseService.
func NewLicenseService(repo LicenseRepository, checker EntitlementChecker, auditor audit.Auditor, publisher queue.Publisher, logger *slog.Logger) *LicenseService {
	return &LicenseService{repo: repo, checker: checker, auditor: auditor, publisher: publisher, logger: logger}
}

// ─── Plan operations ──────────────────────────────────────────────────────────

// CreatePlan validates the request and persists a new Plan.
func (s *LicenseService) CreatePlan(ctx context.Context, req CreatePlanRequest) (*Plan, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	p := &Plan{
		Name:        req.Name,
		Description: req.Description,
		PlanType:    req.PlanType,
		PriceCents:  req.PriceCents,
		IsActive:    req.IsActive,
		Metadata:    req.Metadata,
	}
	if p.Metadata == nil {
		p.Metadata = map[string]any{}
	}

	created, err := s.repo.CreatePlan(ctx, p)
	if err != nil {
		return nil, fmt.Errorf("license service: CreatePlan: %w", err)
	}

	s.logger.InfoContext(ctx, "plan created", "plan_id", created.ID, "plan_type", created.PlanType)
	return created, nil
}

// ListPlans returns all plans ordered by name.
func (s *LicenseService) ListPlans(ctx context.Context) ([]Plan, error) {
	plans, err := s.repo.ListPlans(ctx)
	if err != nil {
		return nil, fmt.Errorf("license service: ListPlans: %w", err)
	}
	return plans, nil
}

// GetPlan returns a Plan by ID, or a NotFoundError if it does not exist.
func (s *LicenseService) GetPlan(ctx context.Context, id uuid.UUID) (*Plan, error) {
	p, err := s.repo.FindPlanByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("license service: GetPlan: %w", err)
	}
	if p == nil {
		return nil, api.NewNotFoundError(fmt.Sprintf("plan %s not found", id))
	}
	return p, nil
}

// UpdatePlan applies the provided plan to the repository and returns the updated record.
func (s *LicenseService) UpdatePlan(ctx context.Context, id uuid.UUID, p *Plan) (*Plan, error) {
	// Ensure ID is consistent.
	p.ID = id

	updated, err := s.repo.UpdatePlan(ctx, p)
	if err != nil {
		return nil, fmt.Errorf("license service: UpdatePlan: %w", err)
	}

	s.logger.InfoContext(ctx, "plan updated", "plan_id", id)
	return updated, nil
}

// ListPlanEntitlements returns all entitlements for a given plan.
func (s *LicenseService) ListPlanEntitlements(ctx context.Context, planID uuid.UUID) ([]Entitlement, error) {
	ents, err := s.repo.ListEntitlementsByPlan(ctx, planID)
	if err != nil {
		return nil, fmt.Errorf("license service: ListPlanEntitlements: %w", err)
	}
	return ents, nil
}

// ─── Subscription operations ──────────────────────────────────────────────────

// CreateSubscription creates a new subscription for an org.
func (s *LicenseService) CreateSubscription(ctx context.Context, orgID uuid.UUID, req CreateSubscriptionRequest) (*OrgSubscription, error) {
	req.OrgID = orgID
	if err := req.Validate(); err != nil {
		return nil, err
	}

	sub := &OrgSubscription{
		OrgID:    orgID,
		PlanID:   req.PlanID,
		Status:   "active",
		StartsAt: time.Now().UTC(),
	}

	created, err := s.repo.CreateSubscription(ctx, sub)
	if err != nil {
		return nil, fmt.Errorf("license service: CreateSubscription: %w", err)
	}

	s.logger.InfoContext(ctx, "subscription created", "subscription_id", created.ID, "org_id", orgID, "plan_id", req.PlanID)
	return created, nil
}

// GetSubscription returns the active subscription for an org, or NotFoundError.
func (s *LicenseService) GetSubscription(ctx context.Context, orgID uuid.UUID) (*OrgSubscription, error) {
	sub, err := s.repo.FindActiveSubscription(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("license service: GetSubscription: %w", err)
	}
	if sub == nil {
		return nil, api.NewNotFoundError(fmt.Sprintf("no active subscription for org %s", orgID))
	}
	return sub, nil
}

// UpdateSubscription persists subscription changes and returns the updated record.
func (s *LicenseService) UpdateSubscription(ctx context.Context, orgID uuid.UUID, s2 *OrgSubscription) (*OrgSubscription, error) {
	s2.OrgID = orgID

	updated, err := s.repo.UpdateSubscription(ctx, s2)
	if err != nil {
		return nil, fmt.Errorf("license service: UpdateSubscription: %w", err)
	}

	s.logger.InfoContext(ctx, "subscription updated", "subscription_id", updated.ID, "org_id", orgID)
	return updated, nil
}

// ─── Entitlement operations ───────────────────────────────────────────────────

// CheckEntitlements checks all feature entitlements for an org using the override + plan resolution chain.
// The feature keys to check are gathered from the org's active subscription plan.
func (s *LicenseService) CheckEntitlements(ctx context.Context, orgID uuid.UUID) ([]EntitlementResult, error) {
	// Collect feature keys from the active subscription.
	sub, err := s.repo.FindActiveSubscription(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("license service: CheckEntitlements find subscription: %w", err)
	}

	var featureKeys []string
	if sub != nil {
		ents, err := s.repo.ListEntitlementsByPlan(ctx, sub.PlanID)
		if err != nil {
			return nil, fmt.Errorf("license service: CheckEntitlements list entitlements: %w", err)
		}
		for _, e := range ents {
			featureKeys = append(featureKeys, e.FeatureKey)
		}
	}

	// Also include any overrides the org may have.
	overrides, err := s.repo.ListOverridesByOrg(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("license service: CheckEntitlements list overrides: %w", err)
	}
	keySet := make(map[string]struct{}, len(featureKeys))
	for _, k := range featureKeys {
		keySet[k] = struct{}{}
	}
	for _, o := range overrides {
		if _, exists := keySet[o.FeatureKey]; !exists {
			featureKeys = append(featureKeys, o.FeatureKey)
			keySet[o.FeatureKey] = struct{}{}
		}
	}

	results := make([]EntitlementResult, 0, len(featureKeys))
	for _, key := range featureKeys {
		allowed, remaining, err := s.checker.Check(ctx, orgID, key)
		if err != nil {
			return nil, fmt.Errorf("license service: CheckEntitlements check %q: %w", key, err)
		}
		results = append(results, EntitlementResult{
			FeatureKey: key,
			Allowed:    allowed,
			Remaining:  remaining,
		})
	}

	return results, nil
}

// SetOverride creates or replaces an entitlement override for an org.
func (s *LicenseService) SetOverride(ctx context.Context, orgID uuid.UUID, req UpsertOverrideRequest) (*EntitlementOverride, error) {
	req.OrgID = orgID
	if err := req.Validate(); err != nil {
		return nil, err
	}

	o := &EntitlementOverride{
		OrgID:      orgID,
		FeatureKey: req.FeatureKey,
		LimitValue: req.LimitValue,
		Reason:     req.Reason,
		GrantedBy:  req.GrantedBy,
	}

	if req.ExpiresAt != nil {
		t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err != nil {
			return nil, api.NewValidationError("expires_at must be in RFC3339 format", "expires_at")
		}
		o.ExpiresAt = &t
	}

	saved, err := s.repo.UpsertOverride(ctx, o)
	if err != nil {
		return nil, fmt.Errorf("license service: SetOverride: %w", err)
	}

	s.logger.InfoContext(ctx, "entitlement override set", "org_id", orgID, "feature_key", req.FeatureKey)
	return saved, nil
}

// GetUsage returns all usage records for an org.
func (s *LicenseService) GetUsage(ctx context.Context, orgID uuid.UUID) ([]UsageRecord, error) {
	// We return all available usage records for the org.
	// The repository query is by feature key + period range; we fetch overrides to
	// enumerate the relevant feature keys and return one record per key for the
	// current month.
	overrides, err := s.repo.ListOverridesByOrg(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("license service: GetUsage list overrides: %w", err)
	}

	sub, err := s.repo.FindActiveSubscription(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("license service: GetUsage find subscription: %w", err)
	}

	keySet := make(map[string]struct{})
	for _, o := range overrides {
		keySet[o.FeatureKey] = struct{}{}
	}
	if sub != nil {
		ents, err := s.repo.ListEntitlementsByPlan(ctx, sub.PlanID)
		if err != nil {
			return nil, fmt.Errorf("license service: GetUsage list entitlements: %w", err)
		}
		for _, e := range ents {
			keySet[e.FeatureKey] = struct{}{}
		}
	}

	now := time.Now().UTC()
	periodStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	periodEnd := periodStart.AddDate(0, 1, 0).Add(-time.Nanosecond)

	var records []UsageRecord
	for key := range keySet {
		qty, err := s.repo.GetUsageByOrg(ctx, orgID, key, periodStart, periodEnd)
		if err != nil {
			return nil, fmt.Errorf("license service: GetUsage get usage for %q: %w", key, err)
		}
		records = append(records, UsageRecord{
			OrgID:       orgID,
			FeatureKey:  key,
			Quantity:    qty,
			PeriodStart: periodStart,
			PeriodEnd:   periodEnd,
			RecordedAt:  now,
		})
	}

	return records, nil
}
