package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/platform/queue"
	"github.com/quorant/quorant/internal/task"
)

// ComplianceAlertJob is a daily scheduler job that checks for upcoming jurisdiction rules
// and creates advisory tasks for affected orgs.
type ComplianceAlertJob struct {
	rules      JurisdictionRuleRepository
	checks     ComplianceCheckRepository
	orgLookup  OrgLookup
	compliance *ComplianceService
	taskSvc    task.Service
	publisher  queue.Publisher
	logger     *slog.Logger
}

func NewComplianceAlertJob(
	rules JurisdictionRuleRepository,
	checks ComplianceCheckRepository,
	orgLookup OrgLookup,
	compliance *ComplianceService,
	taskSvc task.Service,
	publisher queue.Publisher,
	logger *slog.Logger,
) *ComplianceAlertJob {
	return &ComplianceAlertJob{
		rules: rules, checks: checks, orgLookup: orgLookup,
		compliance: compliance, taskSvc: taskSvc,
		publisher: publisher, logger: logger,
	}
}

func (j *ComplianceAlertJob) Name() string { return "compliance_alert" }

// Run executes the daily compliance alert job.
//
// It performs two passes:
//  1. Upcoming rules (effective within 30 days): creates advisory tasks for affected orgs.
//  2. Rules effective today: runs enforcement checks, records compliance failures, and
//     publishes ComplianceAlertRaised events for non-compliant orgs.
func (j *ComplianceAlertJob) Run(ctx context.Context) error {
	advisoryTasks := 0
	enforcementTasks := 0

	// ── Pass 1: Upcoming rules → advisory tasks ───────────────────────────────
	upcomingRules, err := j.rules.ListUpcomingRules(ctx, 30)
	if err != nil {
		return fmt.Errorf("compliance_alert_job: list upcoming rules: %w", err)
	}

	for _, rule := range upcomingRules {
		orgs, err := j.orgLookup.ListByJurisdiction(ctx, rule.Jurisdiction)
		if err != nil {
			j.logger.Error("compliance_alert_job: list orgs by jurisdiction",
				"jurisdiction", rule.Jurisdiction, "error", err)
			continue
		}

		title := fmt.Sprintf("Upcoming rule: %s (%s)", rule.RuleCategory, rule.Jurisdiction)
		desc := fmt.Sprintf("New %s rule takes effect on %s: %s (%s)",
			rule.RuleCategory,
			rule.EffectiveDate.Format("2006-01-02"),
			rule.Notes,
			rule.StatuteReference,
		)
		priority := "normal"

		for _, o := range orgs {
			_, err := j.taskSvc.CreateTask(ctx, o.ID, task.CreateTaskRequest{
				TaskTypeID:   uuid.Nil,
				Title:        title,
				Description:  &desc,
				ResourceType: "jurisdiction_rule",
				ResourceID:   rule.ID,
				Priority:     &priority,
			}, uuid.Nil)
			if err != nil {
				j.logger.Error("compliance_alert_job: create advisory task",
					"org_id", o.ID, "rule_id", rule.ID, "error", err)
				continue
			}
			advisoryTasks++
		}
	}

	// ── Pass 2: Rules effective today → enforcement ───────────────────────────
	todayRules, err := j.rules.ListRulesEffectiveToday(ctx)
	if err != nil {
		return fmt.Errorf("compliance_alert_job: list rules effective today: %w", err)
	}

	for _, rule := range todayRules {
		orgs, err := j.orgLookup.ListByJurisdiction(ctx, rule.Jurisdiction)
		if err != nil {
			j.logger.Error("compliance_alert_job: list orgs by jurisdiction (today)",
				"jurisdiction", rule.Jurisdiction, "error", err)
			continue
		}

		for _, o := range orgs {
			result, err := j.compliance.CheckCompliance(ctx, o.ID, rule.RuleCategory)
			if err != nil {
				j.logger.Error("compliance_alert_job: check compliance",
					"org_id", o.ID, "category", rule.RuleCategory, "error", err)
				continue
			}

			if result.Status != "non_compliant" {
				continue
			}

			// Record the compliance failure.
			savedCheck, err := j.checks.Create(ctx, &ComplianceCheck{
				OrgID:  o.ID,
				RuleID: rule.ID,
				Status: result.Status,
			})
			if err != nil {
				j.logger.Error("compliance_alert_job: create compliance check",
					"org_id", o.ID, "rule_id", rule.ID, "error", err)
				continue
			}

			// Create an enforcement task.
			priority := "high"
			enfTitle := fmt.Sprintf("Compliance alert: %s — non-compliant", result.Category)
			createdTask, err := j.taskSvc.CreateTask(ctx, o.ID, task.CreateTaskRequest{
				TaskTypeID:   uuid.Nil,
				Title:        enfTitle,
				ResourceType: "jurisdiction_rule",
				ResourceID:   rule.ID,
				Priority:     &priority,
			}, uuid.Nil)
			if err != nil {
				j.logger.Error("compliance_alert_job: create enforcement task",
					"org_id", o.ID, "rule_id", rule.ID, "error", err)
				continue
			}
			enforcementTasks++

			// Publish a ComplianceAlertRaised event.
			alertPayload, err := json.Marshal(map[string]any{
				"org_id":   o.ID,
				"rule_id":  rule.ID,
				"category": result.Category,
				"status":   result.Status,
				"message":  result.Details,
				"task_id":  createdTask.ID,
			})
			if err != nil {
				j.logger.Error("compliance_alert_job: marshal alert payload",
					"org_id", o.ID, "error", err)
				continue
			}

			alertEvent := queue.NewBaseEvent(
				"quorant.ai.ComplianceAlertRaised",
				"compliance_check",
				savedCheck.ID,
				o.ID,
				alertPayload,
			)
			if err := j.publisher.Publish(ctx, alertEvent); err != nil {
				j.logger.Error("compliance_alert_job: publish alert",
					"org_id", o.ID, "error", err)
			}
		}
	}

	j.logger.Info("compliance_alert_job: run complete",
		"upcoming_rules", len(upcomingRules),
		"advisory_tasks", advisoryTasks,
		"today_rules", len(todayRules),
		"enforcement_tasks", enforcementTasks,
	)

	return nil
}
