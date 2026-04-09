package ai

import (
	"context"
	"time"
)

func findRuleByKey(rules []RuleValue, key string) *RuleValue {
	for i := range rules {
		if rules[i].Key == key {
			return &rules[i]
		}
	}
	return nil
}

func newResult(category, status string, rules []RuleValue, details map[string]any) *ComplianceResult {
	return &ComplianceResult{
		Category:  category,
		Status:    status,
		Rules:     rules,
		Details:   details,
		CheckedAt: time.Now(),
	}
}

// EvaluateWebsiteRequirements checks if the org meets website mandate thresholds.
func EvaluateWebsiteRequirements(ctx context.Context, org OrgComplianceContext, rules []RuleValue) (*ComplianceResult, error) {
	threshold := findRuleByKey(rules, "required_for_unit_count")
	if threshold == nil {
		return newResult("website_requirements", "not_applicable", rules, nil), nil
	}

	requiredCount, err := threshold.IntValue()
	if err != nil {
		return nil, err
	}

	details := map[string]any{
		"unit_count":  org.UnitCount,
		"threshold":   requiredCount,
		"has_website": org.HasWebsite,
	}

	if org.UnitCount < requiredCount {
		return newResult("website_requirements", "not_applicable", rules, details), nil
	}
	if !org.HasWebsite {
		return newResult("website_requirements", "non_compliant", rules, details), nil
	}
	return newResult("website_requirements", "compliant", rules, details), nil
}

// EvaluateMeetingNotice confirms that meeting notice rules exist for the jurisdiction.
// Actual enforcement happens at meeting scheduling time in the gov module.
func EvaluateMeetingNotice(ctx context.Context, org OrgComplianceContext, rules []RuleValue) (*ComplianceResult, error) {
	if len(rules) == 0 {
		return newResult("meeting_notice", "unknown", rules, map[string]any{
			"reason": "no meeting notice rules configured for jurisdiction",
		}), nil
	}
	return newResult("meeting_notice", "compliant", rules, nil), nil
}

// EvaluateFineLimits confirms that fine limit rules exist for the jurisdiction.
// Actual enforcement happens at fine creation time in the gov module.
func EvaluateFineLimits(ctx context.Context, org OrgComplianceContext, rules []RuleValue) (*ComplianceResult, error) {
	if len(rules) == 0 {
		return newResult("fine_limits", "unknown", rules, map[string]any{
			"reason": "no fine limit rules configured for jurisdiction",
		}), nil
	}
	return newResult("fine_limits", "compliant", rules, nil), nil
}

// EvaluateReserveStudy checks if the org has a current reserve study per state mandates.
func EvaluateReserveStudy(ctx context.Context, org OrgComplianceContext, rules []RuleValue) (*ComplianceResult, error) {
	requiredRule := findRuleByKey(rules, "sirs_required")
	if requiredRule == nil {
		return newResult("reserve_study", "not_applicable", rules, nil), nil
	}

	required, err := requiredRule.BoolValue()
	if err != nil {
		return nil, err
	}
	if !required {
		return newResult("reserve_study", "not_applicable", rules, nil), nil
	}

	// Check min stories threshold (e.g., FL SIRS applies to 3+ story buildings)
	minStoriesRule := findRuleByKey(rules, "sirs_min_stories")
	if minStoriesRule != nil {
		minStories, err := minStoriesRule.IntValue()
		if err != nil {
			return nil, err
		}
		if org.BuildingStories < minStories {
			return newResult("reserve_study", "not_applicable", rules, map[string]any{
				"building_stories": org.BuildingStories,
				"min_stories":      minStories,
			}), nil
		}
	}

	intervalRule := findRuleByKey(rules, "sirs_interval_years")
	var intervalYears int
	if intervalRule != nil {
		intervalYears, err = intervalRule.IntValue()
		if err != nil {
			return nil, err
		}
	}

	details := map[string]any{
		"required":         true,
		"interval_years":   intervalYears,
		"last_study_date":  org.LastReserveStudyDate,
		"building_stories": org.BuildingStories,
	}

	if org.LastReserveStudyDate == nil {
		return newResult("reserve_study", "non_compliant", rules, details), nil
	}

	if intervalYears > 0 {
		cutoff := time.Now().AddDate(-intervalYears, 0, 0)
		if org.LastReserveStudyDate.Before(cutoff) {
			return newResult("reserve_study", "non_compliant", rules, details), nil
		}
	}

	return newResult("reserve_study", "compliant", rules, details), nil
}

// EvaluateRecordRetention confirms that record retention rules exist.
func EvaluateRecordRetention(ctx context.Context, org OrgComplianceContext, rules []RuleValue) (*ComplianceResult, error) {
	if len(rules) == 0 {
		return newResult("record_retention", "unknown", rules, map[string]any{
			"reason": "no record retention rules configured for jurisdiction",
		}), nil
	}
	return newResult("record_retention", "compliant", rules, nil), nil
}

// EvaluateVotingRules confirms that voting rules exist for the jurisdiction.
func EvaluateVotingRules(ctx context.Context, org OrgComplianceContext, rules []RuleValue) (*ComplianceResult, error) {
	if len(rules) == 0 {
		return newResult("voting_rules", "unknown", rules, map[string]any{
			"reason": "no voting rules configured for jurisdiction",
		}), nil
	}
	return newResult("voting_rules", "compliant", rules, nil), nil
}

// EvaluateEstoppel confirms that estoppel rules exist for the jurisdiction.
func EvaluateEstoppel(ctx context.Context, org OrgComplianceContext, rules []RuleValue) (*ComplianceResult, error) {
	if len(rules) == 0 {
		return newResult("estoppel", "not_applicable", rules, map[string]any{
			"reason": "no estoppel rules configured for jurisdiction",
		}), nil
	}
	return newResult("estoppel", "compliant", rules, nil), nil
}
