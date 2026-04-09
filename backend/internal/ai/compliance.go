package ai

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// ComplianceResolver wraps both tiers of compliance checking.
// Tier 1: deterministic jurisdiction rule lookups (this engine).
// Tier 2: interpretive compliance via PolicyResolver (existing AI layer).
type ComplianceResolver interface {
	GetJurisdictionRule(ctx context.Context, jurisdiction, category, key string) (*RuleValue, error)
	ListJurisdictionRules(ctx context.Context, jurisdiction, category string) ([]RuleValue, error)
	EvaluateCompliance(ctx context.Context, orgID uuid.UUID) (*ComplianceReport, error)
	CheckCompliance(ctx context.Context, orgID uuid.UUID, category string) (*ComplianceResult, error)
}

// OrgComplianceContext provides the org-level data that category evaluators need.
type OrgComplianceContext struct {
	OrgID                   uuid.UUID
	Jurisdiction            string
	UnitCount               int
	HasWebsite              bool
	LastReserveStudyDate    *time.Time
	BuildingStories         int
	ElectronicVotingEnabled bool
	ProxyVotingEnabled      bool
}

// CategoryEvaluator evaluates compliance for a single category given an org's context and the active rules.
type CategoryEvaluator func(ctx context.Context, org OrgComplianceContext, rules []RuleValue) (*ComplianceResult, error)

// ValidRuleCategories lists the 7 enforceable dimensions.
var ValidRuleCategories = []string{
	"meeting_notice",
	"fine_limits",
	"reserve_study",
	"website_requirements",
	"record_retention",
	"voting_rules",
	"estoppel",
}

// ValidValueTypes lists the supported value types for jurisdiction rules.
var ValidValueTypes = []string{"integer", "decimal", "boolean", "text", "json"}

// IsValidRuleCategory returns true if the category is one of the 7 supported dimensions.
func IsValidRuleCategory(cat string) bool {
	for _, c := range ValidRuleCategories {
		if c == cat {
			return true
		}
	}
	return false
}

// IsValidValueType returns true if the value type is supported.
func IsValidValueType(vt string) bool {
	for _, v := range ValidValueTypes {
		if v == vt {
			return true
		}
	}
	return false
}

// NoopComplianceResolver is a stub used when the compliance engine is not yet wired.
type NoopComplianceResolver struct{}

func NewNoopComplianceResolver() *NoopComplianceResolver { return &NoopComplianceResolver{} }
func (r *NoopComplianceResolver) GetJurisdictionRule(ctx context.Context, jurisdiction, category, key string) (*RuleValue, error) {
	return nil, nil
}
func (r *NoopComplianceResolver) ListJurisdictionRules(ctx context.Context, jurisdiction, category string) ([]RuleValue, error) {
	return nil, nil
}
func (r *NoopComplianceResolver) EvaluateCompliance(ctx context.Context, orgID uuid.UUID) (*ComplianceReport, error) {
	return nil, nil
}
func (r *NoopComplianceResolver) CheckCompliance(ctx context.Context, orgID uuid.UUID, category string) (*ComplianceResult, error) {
	return nil, nil
}
