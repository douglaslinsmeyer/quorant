package ai_test

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/ai"
	"github.com/quorant/quorant/internal/org"
	"github.com/quorant/quorant/internal/platform/queue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestComplianceAlertJob constructs a ComplianceAlertJob with all evaluators registered.
func newTestComplianceAlertJob(
	ruleRepo *mockJurisdictionRuleRepo,
	checkRepo *mockComplianceCheckRepo,
	orgLookupMock *mockOrgLookup,
	taskSvc *mockTaskService,
	publisher queue.Publisher,
) *ai.ComplianceAlertJob {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := newTestComplianceService(ruleRepo, checkRepo, orgLookupMock)
	svc.RegisterEvaluator("meeting_notice", ai.EvaluateMeetingNotice)
	svc.RegisterEvaluator("fine_limits", ai.EvaluateFineLimits)
	svc.RegisterEvaluator("reserve_study", ai.EvaluateReserveStudy)
	svc.RegisterEvaluator("website_requirements", ai.EvaluateWebsiteRequirements)
	svc.RegisterEvaluator("record_retention", ai.EvaluateRecordRetention)
	svc.RegisterEvaluator("voting_rules", ai.EvaluateVotingRules)
	svc.RegisterEvaluator("estoppel", ai.EvaluateEstoppel)

	return ai.NewComplianceAlertJob(ruleRepo, checkRepo, orgLookupMock, svc, taskSvc, publisher, logger)
}

// ─── TestComplianceAlertJob_Name ─────────────────────────────────────────────

func TestComplianceAlertJob_Name(t *testing.T) {
	// Given a ComplianceAlertJob,
	// when Name() is called,
	// then it returns "compliance_alert".
	job := newTestComplianceAlertJob(
		&mockJurisdictionRuleRepo{},
		&mockComplianceCheckRepo{},
		newMockOrgLookup(),
		&mockTaskService{},
		queue.NewInMemoryPublisher(),
	)

	assert.Equal(t, "compliance_alert", job.Name())
}

// ─── TestComplianceAlertJob_UpcomingRules ────────────────────────────────────

func TestComplianceAlertJob_UpcomingRules(t *testing.T) {
	// Given a rule effective 10 days from now in jurisdiction "FL",
	// and an FL org in the byJurisdiction map,
	// when Run() is called,
	// then an advisory task is created for the org.
	ruleRepo := &mockJurisdictionRuleRepo{}
	upcomingRule := ai.JurisdictionRule{
		ID:               uuid.New(),
		Jurisdiction:     "FL",
		RuleCategory:     "meeting_notice",
		RuleKey:          "min_days_notice",
		ValueType:        "integer",
		StatuteReference: "FS 720.303",
		Notes:            "Minimum notice days for annual meetings",
		EffectiveDate:    time.Now().AddDate(0, 0, 10),
	}
	ruleRepo.upcomingRules = []ai.JurisdictionRule{upcomingRule}

	orgLookupMock := newMockOrgLookup()
	orgID := uuid.New()
	jurisdiction := "FL"
	website := "https://sunsetpalms.com"
	flOrg := &org.Organization{
		ID:           orgID,
		Name:         "Sunset Palms HOA",
		Type:         "hoa",
		Jurisdiction: &jurisdiction,
		Website:      &website,
	}
	orgLookupMock.orgs[orgID] = flOrg
	orgLookupMock.byJurisdiction["FL"] = []org.Organization{*flOrg}

	checkRepo := &mockComplianceCheckRepo{}
	taskSvc := &mockTaskService{}
	publisher := queue.NewInMemoryPublisher()

	job := newTestComplianceAlertJob(ruleRepo, checkRepo, orgLookupMock, taskSvc, publisher)

	err := job.Run(context.Background())

	require.NoError(t, err)
	// One advisory task should be created for the FL org.
	require.Len(t, taskSvc.created, 1, "expected one advisory task for the upcoming rule")
	assert.Contains(t, taskSvc.created[0].Title, "Upcoming rule:")
	assert.Contains(t, taskSvc.created[0].Title, "meeting_notice")
	assert.Contains(t, taskSvc.created[0].Title, "FL")
	// No enforcement tasks — the rule is not yet effective (today rules list is empty).
	assert.Empty(t, publisher.Events(), "expected no alert events for advisory-only run")
}

// ─── TestComplianceAlertJob_NoUpcomingRules ──────────────────────────────────

func TestComplianceAlertJob_NoUpcomingRules(t *testing.T) {
	// Given no upcoming rules and no rules effective today,
	// when Run() is called,
	// then it returns nil and no tasks are created.
	ruleRepo := &mockJurisdictionRuleRepo{}
	// upcomingRules and todayRules are nil → mocks return nil, nil.

	orgLookupMock := newMockOrgLookup()
	checkRepo := &mockComplianceCheckRepo{}
	taskSvc := &mockTaskService{}
	publisher := queue.NewInMemoryPublisher()

	job := newTestComplianceAlertJob(ruleRepo, checkRepo, orgLookupMock, taskSvc, publisher)

	err := job.Run(context.Background())

	require.NoError(t, err)
	assert.Empty(t, taskSvc.created, "expected no tasks when there are no rules")
	assert.Empty(t, publisher.Events(), "expected no events when there are no rules")
}
