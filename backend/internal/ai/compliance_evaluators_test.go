package ai_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/ai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeRule(category, key, valueType string, value any) ai.RuleValue {
	v, _ := json.Marshal(value)
	return ai.RuleValue{
		ID:               uuid.New(),
		Jurisdiction:     "FL",
		Category:         category,
		Key:              key,
		ValueType:        valueType,
		Value:            json.RawMessage(v),
		StatuteReference: "test statute",
		EffectiveDate:    time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
	}
}

// --- EvaluateWebsiteRequirements ---

func TestEvaluateWebsiteRequirements_NonCompliant(t *testing.T) {
	// Given an org with 150 units and no website, when the threshold is 100 units,
	// then the result should be non_compliant.
	rules := []ai.RuleValue{
		makeRule("website_requirements", "required_for_unit_count", "integer", 100),
	}
	org := ai.OrgComplianceContext{
		OrgID:      uuid.New(),
		UnitCount:  150,
		HasWebsite: false,
	}

	result, err := ai.EvaluateWebsiteRequirements(context.Background(), org, rules)

	require.NoError(t, err)
	assert.Equal(t, "website_requirements", result.Category)
	assert.Equal(t, "non_compliant", result.Status)
}

func TestEvaluateWebsiteRequirements_Compliant(t *testing.T) {
	// Given an org with 150 units and a website, when the threshold is 100 units,
	// then the result should be compliant.
	rules := []ai.RuleValue{
		makeRule("website_requirements", "required_for_unit_count", "integer", 100),
	}
	org := ai.OrgComplianceContext{
		OrgID:      uuid.New(),
		UnitCount:  150,
		HasWebsite: true,
	}

	result, err := ai.EvaluateWebsiteRequirements(context.Background(), org, rules)

	require.NoError(t, err)
	assert.Equal(t, "website_requirements", result.Category)
	assert.Equal(t, "compliant", result.Status)
}

func TestEvaluateWebsiteRequirements_NotApplicable(t *testing.T) {
	// Given an org with only 50 units, when the threshold is 100 units,
	// then the result should be not_applicable regardless of website presence.
	rules := []ai.RuleValue{
		makeRule("website_requirements", "required_for_unit_count", "integer", 100),
	}
	org := ai.OrgComplianceContext{
		OrgID:      uuid.New(),
		UnitCount:  50,
		HasWebsite: false,
	}

	result, err := ai.EvaluateWebsiteRequirements(context.Background(), org, rules)

	require.NoError(t, err)
	assert.Equal(t, "website_requirements", result.Category)
	assert.Equal(t, "not_applicable", result.Status)
}

func TestEvaluateWebsiteRequirements_NoRules(t *testing.T) {
	// Given no rules configured for the jurisdiction,
	// then the result should be not_applicable.
	org := ai.OrgComplianceContext{
		OrgID:     uuid.New(),
		UnitCount: 200,
	}

	result, err := ai.EvaluateWebsiteRequirements(context.Background(), org, nil)

	require.NoError(t, err)
	assert.Equal(t, "website_requirements", result.Category)
	assert.Equal(t, "not_applicable", result.Status)
}

// --- EvaluateMeetingNotice ---

func TestEvaluateMeetingNotice_RulesPresent(t *testing.T) {
	// Given meeting notice rules exist for the jurisdiction,
	// then the result should be compliant (enforcement happens at scheduling time).
	rules := []ai.RuleValue{
		makeRule("meeting_notice", "min_days_notice", "integer", 14),
	}
	org := ai.OrgComplianceContext{OrgID: uuid.New()}

	result, err := ai.EvaluateMeetingNotice(context.Background(), org, rules)

	require.NoError(t, err)
	assert.Equal(t, "meeting_notice", result.Category)
	assert.Equal(t, "compliant", result.Status)
}

func TestEvaluateMeetingNotice_NoRules(t *testing.T) {
	// Given no meeting notice rules configured for the jurisdiction,
	// then the result should be unknown.
	org := ai.OrgComplianceContext{OrgID: uuid.New()}

	result, err := ai.EvaluateMeetingNotice(context.Background(), org, nil)

	require.NoError(t, err)
	assert.Equal(t, "meeting_notice", result.Category)
	assert.Equal(t, "unknown", result.Status)
}

// --- EvaluateReserveStudy ---

func TestEvaluateReserveStudy_NonCompliant_NoStudy(t *testing.T) {
	// Given a 4-story building with no reserve study on record,
	// when sirs_required=true and sirs_min_stories=3,
	// then the result should be non_compliant.
	rules := []ai.RuleValue{
		makeRule("reserve_study", "sirs_required", "boolean", true),
		makeRule("reserve_study", "sirs_min_stories", "integer", 3),
		makeRule("reserve_study", "sirs_interval_years", "integer", 3),
	}
	org := ai.OrgComplianceContext{
		OrgID:                uuid.New(),
		BuildingStories:      4,
		LastReserveStudyDate: nil,
	}

	result, err := ai.EvaluateReserveStudy(context.Background(), org, rules)

	require.NoError(t, err)
	assert.Equal(t, "reserve_study", result.Category)
	assert.Equal(t, "non_compliant", result.Status)
}

func TestEvaluateReserveStudy_NotApplicable_TooShort(t *testing.T) {
	// Given a 2-story building, when sirs_min_stories=3,
	// then the result should be not_applicable because the building doesn't meet the height threshold.
	rules := []ai.RuleValue{
		makeRule("reserve_study", "sirs_required", "boolean", true),
		makeRule("reserve_study", "sirs_min_stories", "integer", 3),
	}
	org := ai.OrgComplianceContext{
		OrgID:           uuid.New(),
		BuildingStories: 2,
	}

	result, err := ai.EvaluateReserveStudy(context.Background(), org, rules)

	require.NoError(t, err)
	assert.Equal(t, "reserve_study", result.Category)
	assert.Equal(t, "not_applicable", result.Status)
}

// --- EvaluateFineLimits ---

func TestEvaluateFineLimits_RulesPresent(t *testing.T) {
	// Given fine limit rules exist for the jurisdiction,
	// then the result should be compliant (enforcement happens at fine creation time).
	rules := []ai.RuleValue{
		makeRule("fine_limits", "max_fine_per_violation", "integer", 1000),
	}
	org := ai.OrgComplianceContext{OrgID: uuid.New()}

	result, err := ai.EvaluateFineLimits(context.Background(), org, rules)

	require.NoError(t, err)
	assert.Equal(t, "fine_limits", result.Category)
	assert.Equal(t, "compliant", result.Status)
}

// --- EvaluateRecordRetention ---

func TestEvaluateRecordRetention_RulesPresent(t *testing.T) {
	// Given record retention rules exist for the jurisdiction,
	// then the result should be compliant.
	rules := []ai.RuleValue{
		makeRule("record_retention", "minutes_retention_years", "integer", 7),
	}
	org := ai.OrgComplianceContext{OrgID: uuid.New()}

	result, err := ai.EvaluateRecordRetention(context.Background(), org, rules)

	require.NoError(t, err)
	assert.Equal(t, "record_retention", result.Category)
	assert.Equal(t, "compliant", result.Status)
}

// --- EvaluateVotingRules ---

func TestEvaluateVotingRules_RulesPresent(t *testing.T) {
	// Given voting rules exist for the jurisdiction,
	// then the result should be compliant.
	rules := []ai.RuleValue{
		makeRule("voting_rules", "electronic_voting_allowed", "boolean", true),
	}
	org := ai.OrgComplianceContext{OrgID: uuid.New()}

	result, err := ai.EvaluateVotingRules(context.Background(), org, rules)

	require.NoError(t, err)
	assert.Equal(t, "voting_rules", result.Category)
	assert.Equal(t, "compliant", result.Status)
}

// --- EvaluateEstoppel ---

func TestEvaluateEstoppel_RulesPresent(t *testing.T) {
	// Given estoppel rules exist for the jurisdiction,
	// then the result should be compliant.
	rules := []ai.RuleValue{
		makeRule("estoppel", "max_fee", "integer", 299),
	}
	org := ai.OrgComplianceContext{OrgID: uuid.New()}

	result, err := ai.EvaluateEstoppel(context.Background(), org, rules)

	require.NoError(t, err)
	assert.Equal(t, "estoppel", result.Category)
	assert.Equal(t, "compliant", result.Status)
}
