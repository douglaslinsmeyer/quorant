package ai_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/ai"
	"github.com/quorant/quorant/internal/org"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── mockJurisdictionRuleRepo ─────────────────────────────────────────────────

type mockJurisdictionRuleRepo struct {
	rules []ai.JurisdictionRule
}

func (m *mockJurisdictionRuleRepo) Create(_ context.Context, rule *ai.JurisdictionRule) (*ai.JurisdictionRule, error) {
	rule.ID = uuid.New()
	rule.CreatedAt = time.Now()
	rule.UpdatedAt = time.Now()
	cp := *rule
	m.rules = append(m.rules, cp)
	return &cp, nil
}

func (m *mockJurisdictionRuleRepo) Update(_ context.Context, rule *ai.JurisdictionRule) (*ai.JurisdictionRule, error) {
	for i, r := range m.rules {
		if r.ID == rule.ID {
			m.rules[i] = *rule
			cp := *rule
			return &cp, nil
		}
	}
	return nil, nil
}

func (m *mockJurisdictionRuleRepo) FindByID(_ context.Context, id uuid.UUID) (*ai.JurisdictionRule, error) {
	for _, r := range m.rules {
		if r.ID == id {
			cp := r
			return &cp, nil
		}
	}
	return nil, nil
}

func (m *mockJurisdictionRuleRepo) GetActiveRule(_ context.Context, jurisdiction, category, key string) (*ai.JurisdictionRule, error) {
	now := time.Now()
	for _, r := range m.rules {
		if r.Jurisdiction == jurisdiction && r.RuleCategory == category && r.RuleKey == key {
			if r.EffectiveDate.After(now) {
				continue
			}
			if r.ExpirationDate != nil && r.ExpirationDate.Before(now) {
				continue
			}
			cp := r
			return &cp, nil
		}
	}
	return nil, nil
}

func (m *mockJurisdictionRuleRepo) ListActiveRules(_ context.Context, jurisdiction, category string) ([]ai.JurisdictionRule, error) {
	now := time.Now()
	var out []ai.JurisdictionRule
	for _, r := range m.rules {
		if r.Jurisdiction != jurisdiction || r.RuleCategory != category {
			continue
		}
		if r.EffectiveDate.After(now) {
			continue
		}
		if r.ExpirationDate != nil && r.ExpirationDate.Before(now) {
			continue
		}
		out = append(out, r)
	}
	return out, nil
}

func (m *mockJurisdictionRuleRepo) ListActiveRulesByJurisdiction(_ context.Context, jurisdiction string) ([]ai.JurisdictionRule, error) {
	return nil, nil
}

func (m *mockJurisdictionRuleRepo) ListAllRules(_ context.Context, jurisdiction string, limit int, afterID *uuid.UUID) ([]ai.JurisdictionRule, bool, error) {
	return nil, false, nil
}

func (m *mockJurisdictionRuleRepo) ListUpcomingRules(_ context.Context, withinDays int) ([]ai.JurisdictionRule, error) {
	return nil, nil
}

func (m *mockJurisdictionRuleRepo) ListRulesEffectiveToday(_ context.Context) ([]ai.JurisdictionRule, error) {
	return nil, nil
}

// ─── mockComplianceCheckRepo ──────────────────────────────────────────────────

type mockComplianceCheckRepo struct {
	checks []ai.ComplianceCheck
}

func (m *mockComplianceCheckRepo) Create(_ context.Context, check *ai.ComplianceCheck) (*ai.ComplianceCheck, error) {
	check.ID = uuid.New()
	check.CheckedAt = time.Now()
	cp := *check
	m.checks = append(m.checks, cp)
	return &cp, nil
}

func (m *mockComplianceCheckRepo) ListByOrg(_ context.Context, orgID uuid.UUID, limit int, afterID *uuid.UUID) ([]ai.ComplianceCheck, bool, error) {
	return nil, false, nil
}

func (m *mockComplianceCheckRepo) GetLatestByOrgAndRule(_ context.Context, orgID, ruleID uuid.UUID) (*ai.ComplianceCheck, error) {
	return nil, nil
}

func (m *mockComplianceCheckRepo) Resolve(_ context.Context, id uuid.UUID, notes string) (*ai.ComplianceCheck, error) {
	return nil, nil
}

// ─── mockOrgLookup ────────────────────────────────────────────────────────────

type mockOrgLookup struct {
	orgs         map[uuid.UUID]*org.Organization
	byJurisdiction map[string][]org.Organization
}

func newMockOrgLookup() *mockOrgLookup {
	return &mockOrgLookup{
		orgs:         make(map[uuid.UUID]*org.Organization),
		byJurisdiction: make(map[string][]org.Organization),
	}
}

func (m *mockOrgLookup) FindByID(_ context.Context, id uuid.UUID) (*org.Organization, error) {
	o, ok := m.orgs[id]
	if !ok {
		return nil, nil
	}
	cp := *o
	return &cp, nil
}

func (m *mockOrgLookup) FindActiveManagement(_ context.Context, hoaOrgID uuid.UUID) (*org.OrgManagement, error) {
	return nil, nil
}

func (m *mockOrgLookup) Update(_ context.Context, o *org.Organization) (*org.Organization, error) {
	m.orgs[o.ID] = o
	cp := *o
	return &cp, nil
}

func (m *mockOrgLookup) ListByJurisdiction(_ context.Context, jurisdiction string) ([]org.Organization, error) {
	return m.byJurisdiction[jurisdiction], nil
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func makeJurisdictionRule(jurisdiction, category, key, valueType string, value any) ai.JurisdictionRule {
	v, _ := json.Marshal(value)
	return ai.JurisdictionRule{
		ID:               uuid.New(),
		Jurisdiction:     jurisdiction,
		RuleCategory:     category,
		RuleKey:          key,
		ValueType:        valueType,
		Value:            json.RawMessage(v),
		StatuteReference: "test statute",
		EffectiveDate:    time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
	}
}

func newTestComplianceService(
	ruleRepo *mockJurisdictionRuleRepo,
	checkRepo *mockComplianceCheckRepo,
	orgLookup *mockOrgLookup,
) *ai.ComplianceService {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return ai.NewComplianceService(ruleRepo, checkRepo, orgLookup, logger)
}

// ─── TestComplianceService_GetJurisdictionRule ────────────────────────────────

func TestComplianceService_GetJurisdictionRule(t *testing.T) {
	// Given a seeded FL hearing_required rule,
	// when GetJurisdictionRule is called with matching jurisdiction/category/key,
	// then it returns the rule and BoolValue() is true.
	ruleRepo := &mockJurisdictionRuleRepo{}
	ruleRepo.rules = append(ruleRepo.rules, makeJurisdictionRule("FL", "meeting_notice", "hearing_required", "boolean", true))

	svc := newTestComplianceService(ruleRepo, &mockComplianceCheckRepo{}, newMockOrgLookup())

	result, err := svc.GetJurisdictionRule(context.Background(), "FL", "meeting_notice", "hearing_required")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "FL", result.Jurisdiction)
	assert.Equal(t, "meeting_notice", result.Category)
	assert.Equal(t, "hearing_required", result.Key)

	boolVal, err := result.BoolValue()
	require.NoError(t, err)
	assert.True(t, boolVal)
}

func TestComplianceService_GetJurisdictionRule_NotFound(t *testing.T) {
	// Given no rules seeded,
	// when GetJurisdictionRule is called,
	// then it returns nil, nil (no error).
	ruleRepo := &mockJurisdictionRuleRepo{}
	svc := newTestComplianceService(ruleRepo, &mockComplianceCheckRepo{}, newMockOrgLookup())

	result, err := svc.GetJurisdictionRule(context.Background(), "TX", "meeting_notice", "hearing_required")

	require.NoError(t, err)
	assert.Nil(t, result)
}

// ─── TestComplianceService_EvaluateCompliance ─────────────────────────────────

func TestComplianceService_EvaluateCompliance(t *testing.T) {
	// Given an org with Jurisdiction="FL" and a website rule seeded for FL/website_requirements,
	// and all 7 evaluators registered,
	// when EvaluateCompliance is called,
	// then the report has 7 results and correct summary counts.
	ruleRepo := &mockJurisdictionRuleRepo{}
	// Seed a website_requirements rule: required_for_unit_count = 100
	ruleRepo.rules = append(ruleRepo.rules,
		makeJurisdictionRule("FL", "website_requirements", "required_for_unit_count", "integer", 100),
	)

	orgLookup := newMockOrgLookup()
	orgID := uuid.New()
	jurisdiction := "FL"
	website := "https://myhoaflorida.com"
	orgLookup.orgs[orgID] = &org.Organization{
		ID:           orgID,
		Name:         "Sunset Palms HOA",
		Type:         "hoa",
		Jurisdiction: &jurisdiction,
		Website:      &website,
	}

	svc := newTestComplianceService(ruleRepo, &mockComplianceCheckRepo{}, orgLookup)

	// Register all 7 evaluators.
	svc.RegisterEvaluator("meeting_notice", ai.EvaluateMeetingNotice)
	svc.RegisterEvaluator("fine_limits", ai.EvaluateFineLimits)
	svc.RegisterEvaluator("reserve_study", ai.EvaluateReserveStudy)
	svc.RegisterEvaluator("website_requirements", ai.EvaluateWebsiteRequirements)
	svc.RegisterEvaluator("record_retention", ai.EvaluateRecordRetention)
	svc.RegisterEvaluator("voting_rules", ai.EvaluateVotingRules)
	svc.RegisterEvaluator("estoppel", ai.EvaluateEstoppel)

	report, err := svc.EvaluateCompliance(context.Background(), orgID)

	require.NoError(t, err)
	require.NotNil(t, report)

	assert.Equal(t, orgID, report.OrgID)
	assert.Equal(t, "FL", report.Jurisdiction)
	assert.Len(t, report.Results, 7, "expected one result per ValidRuleCategory")
	assert.Equal(t, 7, report.Summary.Total)

	// Verify total summary counts add up to 7.
	totalCounted := report.Summary.Compliant + report.Summary.NonCompliant +
		report.Summary.NotApplicable + report.Summary.Unknown
	assert.Equal(t, 7, totalCounted, "summary counts should sum to 7")

	// The website_requirements category has a seeded rule and the org has a website
	// with UnitCount=0 (default), which is < 100, so it should be not_applicable.
	var websiteResult *ai.ComplianceResult
	for i := range report.Results {
		if report.Results[i].Category == "website_requirements" {
			websiteResult = &report.Results[i]
			break
		}
	}
	require.NotNil(t, websiteResult, "website_requirements result should be present")
	assert.Equal(t, "not_applicable", websiteResult.Status)
}

func TestComplianceService_EvaluateCompliance_OrgNotFound(t *testing.T) {
	// Given no org seeded,
	// when EvaluateCompliance is called with an unknown orgID,
	// then it returns an error.
	svc := newTestComplianceService(&mockJurisdictionRuleRepo{}, &mockComplianceCheckRepo{}, newMockOrgLookup())

	report, err := svc.EvaluateCompliance(context.Background(), uuid.New())

	require.Error(t, err)
	assert.Nil(t, report)
	assert.Contains(t, err.Error(), "not found")
}

func TestComplianceService_EvaluateCompliance_NoJurisdiction(t *testing.T) {
	// Given an org with no Jurisdiction field,
	// when EvaluateCompliance is called,
	// then it returns an error about missing jurisdiction.
	orgLookup := newMockOrgLookup()
	orgID := uuid.New()
	orgLookup.orgs[orgID] = &org.Organization{
		ID:   orgID,
		Name: "No Jurisdiction HOA",
		Type: "hoa",
	}

	svc := newTestComplianceService(&mockJurisdictionRuleRepo{}, &mockComplianceCheckRepo{}, orgLookup)

	report, err := svc.EvaluateCompliance(context.Background(), orgID)

	require.Error(t, err)
	assert.Nil(t, report)
	assert.Contains(t, err.Error(), "jurisdiction")
}

// ─── TestComplianceService_CheckCompliance ────────────────────────────────────

func TestComplianceService_CheckCompliance_SingleCategory(t *testing.T) {
	// Given an org with Jurisdiction="FL" and a meeting_notice rule seeded,
	// when CheckCompliance is called for "meeting_notice",
	// then a single result is returned.
	ruleRepo := &mockJurisdictionRuleRepo{}
	ruleRepo.rules = append(ruleRepo.rules,
		makeJurisdictionRule("FL", "meeting_notice", "min_days_notice", "integer", 14),
	)

	orgLookup := newMockOrgLookup()
	orgID := uuid.New()
	jurisdiction := "FL"
	orgLookup.orgs[orgID] = &org.Organization{
		ID:           orgID,
		Name:         "Test HOA",
		Type:         "hoa",
		Jurisdiction: &jurisdiction,
	}

	svc := newTestComplianceService(ruleRepo, &mockComplianceCheckRepo{}, orgLookup)
	svc.RegisterEvaluator("meeting_notice", ai.EvaluateMeetingNotice)

	result, err := svc.CheckCompliance(context.Background(), orgID, "meeting_notice")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "meeting_notice", result.Category)
	assert.Equal(t, "compliant", result.Status)
}

func TestComplianceService_CheckCompliance_InvalidCategory(t *testing.T) {
	// Given an invalid category string,
	// when CheckCompliance is called,
	// then it returns an error.
	svc := newTestComplianceService(&mockJurisdictionRuleRepo{}, &mockComplianceCheckRepo{}, newMockOrgLookup())

	result, err := svc.CheckCompliance(context.Background(), uuid.New(), "invalid_category")

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "invalid category")
}

func TestComplianceService_EvaluateCompliance_NoEvaluatorRegistered(t *testing.T) {
	// Given no evaluators registered,
	// when EvaluateCompliance is called,
	// then all results have status "unknown" and the report still succeeds.
	orgLookup := newMockOrgLookup()
	orgID := uuid.New()
	jurisdiction := "FL"
	orgLookup.orgs[orgID] = &org.Organization{
		ID:           orgID,
		Name:         "Bare HOA",
		Type:         "hoa",
		Jurisdiction: &jurisdiction,
	}

	svc := newTestComplianceService(&mockJurisdictionRuleRepo{}, &mockComplianceCheckRepo{}, orgLookup)
	// No evaluators registered.

	report, err := svc.EvaluateCompliance(context.Background(), orgID)

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.Len(t, report.Results, 7)
	assert.Equal(t, 7, report.Summary.Unknown)
	assert.Equal(t, 0, report.Summary.Compliant)
}
