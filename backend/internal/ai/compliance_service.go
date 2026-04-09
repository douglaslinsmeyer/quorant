package ai

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/org"
)

// ComplianceService implements ComplianceResolver.
type ComplianceService struct {
	rules      JurisdictionRuleRepository
	checks     ComplianceCheckRepository
	orgLookup  OrgLookup
	evaluators map[string]CategoryEvaluator
	logger     *slog.Logger
}

func NewComplianceService(
	rules JurisdictionRuleRepository,
	checks ComplianceCheckRepository,
	orgLookup OrgLookup,
	logger *slog.Logger,
) *ComplianceService {
	return &ComplianceService{
		rules:      rules,
		checks:     checks,
		orgLookup:  orgLookup,
		evaluators: make(map[string]CategoryEvaluator),
		logger:     logger,
	}
}

// RegisterEvaluator registers a CategoryEvaluator for the given category string.
func (s *ComplianceService) RegisterEvaluator(category string, eval CategoryEvaluator) {
	s.evaluators[category] = eval
}

// GetJurisdictionRule returns the active rule for the given jurisdiction, category, and key.
// Returns nil, nil if the rule is not found.
func (s *ComplianceService) GetJurisdictionRule(ctx context.Context, jurisdiction, category, key string) (*RuleValue, error) {
	rule, err := s.rules.GetActiveRule(ctx, jurisdiction, category, key)
	if err != nil {
		return nil, fmt.Errorf("ai: GetJurisdictionRule: %w", err)
	}
	if rule == nil {
		return nil, nil
	}
	rv := RuleValueFromJurisdictionRule(rule)
	return &rv, nil
}

// ListJurisdictionRules returns all active rules for the given jurisdiction and category.
func (s *ComplianceService) ListJurisdictionRules(ctx context.Context, jurisdiction, category string) ([]RuleValue, error) {
	rules, err := s.rules.ListActiveRules(ctx, jurisdiction, category)
	if err != nil {
		return nil, fmt.Errorf("ai: ListJurisdictionRules: %w", err)
	}
	out := make([]RuleValue, len(rules))
	for i, r := range rules {
		out[i] = RuleValueFromJurisdictionRule(&r)
	}
	return out, nil
}

// EvaluateCompliance runs all 7 category evaluators for an org and returns a full ComplianceReport.
func (s *ComplianceService) EvaluateCompliance(ctx context.Context, orgID uuid.UUID) (*ComplianceReport, error) {
	o, err := s.orgLookup.FindByID(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("ai: EvaluateCompliance: %w", err)
	}
	if o == nil {
		return nil, fmt.Errorf("ai: EvaluateCompliance: org %s not found", orgID)
	}
	if o.State == nil {
		return nil, fmt.Errorf("ai: EvaluateCompliance: org %s has no state/jurisdiction", orgID)
	}

	orgCtx := buildOrgComplianceContext(o)
	jurisdiction := orgCtx.Jurisdiction

	now := time.Now()
	report := &ComplianceReport{
		OrgID:        orgID,
		Jurisdiction: jurisdiction,
		CheckedAt:    now,
	}

	for _, category := range ValidRuleCategories {
		result, err := s.evaluateCategory(ctx, orgCtx, jurisdiction, category)
		if err != nil {
			if s.logger != nil {
				s.logger.Error("ai: EvaluateCompliance: category evaluation failed",
					"category", category,
					"org_id", orgID,
					"error", err,
				)
			}
			result = &ComplianceResult{
				Category:  category,
				Status:    "unknown",
				CheckedAt: now,
				Details: map[string]any{
					"reason": err.Error(),
				},
			}
		}
		report.Results = append(report.Results, *result)
	}

	// Build summary counts.
	summary := ComplianceSummary{
		Total: len(report.Results),
	}
	for _, r := range report.Results {
		switch r.Status {
		case "compliant":
			summary.Compliant++
		case "non_compliant":
			summary.NonCompliant++
		case "not_applicable":
			summary.NotApplicable++
		default:
			summary.Unknown++
		}
	}
	report.Summary = summary

	return report, nil
}

// CheckCompliance runs a single category evaluator for an org and returns the result.
func (s *ComplianceService) CheckCompliance(ctx context.Context, orgID uuid.UUID, category string) (*ComplianceResult, error) {
	if !IsValidRuleCategory(category) {
		return nil, fmt.Errorf("ai: CheckCompliance: invalid category %q", category)
	}

	o, err := s.orgLookup.FindByID(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("ai: CheckCompliance: %w", err)
	}
	if o == nil {
		return nil, fmt.Errorf("ai: CheckCompliance: org %s not found", orgID)
	}
	if o.State == nil {
		return nil, fmt.Errorf("ai: CheckCompliance: org %s has no state/jurisdiction", orgID)
	}

	orgCtx := buildOrgComplianceContext(o)
	result, err := s.evaluateCategory(ctx, orgCtx, orgCtx.Jurisdiction, category)
	if err != nil {
		return nil, fmt.Errorf("ai: CheckCompliance: %w", err)
	}
	return result, nil
}

// evaluateCategory runs a single registered evaluator for the given org context and category.
// Returns an "unknown" result if no evaluator is registered for the category.
func (s *ComplianceService) evaluateCategory(ctx context.Context, orgCtx OrgComplianceContext, jurisdiction, category string) (*ComplianceResult, error) {
	eval, ok := s.evaluators[category]
	if !ok {
		return &ComplianceResult{
			Category:  category,
			Status:    "unknown",
			CheckedAt: time.Now(),
			Details: map[string]any{
				"reason": fmt.Sprintf("no evaluator registered for category %q", category),
			},
		}, nil
	}

	rules, err := s.rules.ListActiveRules(ctx, jurisdiction, category)
	if err != nil {
		return nil, fmt.Errorf("evaluateCategory %s: %w", category, err)
	}

	ruleValues := make([]RuleValue, len(rules))
	for i, r := range rules {
		ruleValues[i] = RuleValueFromJurisdictionRule(&r)
	}

	result, err := eval(ctx, orgCtx, ruleValues)
	if err != nil {
		return nil, fmt.Errorf("evaluateCategory %s: %w", category, err)
	}
	return result, nil
}

// buildOrgComplianceContext builds an OrgComplianceContext from an org.Organization.
func buildOrgComplianceContext(o *org.Organization) OrgComplianceContext {
	ctx := OrgComplianceContext{
		OrgID:      o.ID,
		HasWebsite: o.Website != nil && *o.Website != "",
	}
	if o.State != nil {
		ctx.Jurisdiction = *o.State
	}
	// UnitCount, BuildingStories, LastReserveStudyDate, voting flags
	// default to zero values for now. Future: populated from org settings or additional queries.
	return ctx
}
