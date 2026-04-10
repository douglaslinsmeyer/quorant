package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/platform/queue"
	"github.com/quorant/quorant/internal/task"
)

// ComplianceWorker subscribes to domain events and re-evaluates org compliance when
// rules or org data change.
type ComplianceWorker struct {
	compliance *ComplianceService
	rules      JurisdictionRuleRepository
	checks     ComplianceCheckRepository
	orgLookup  OrgLookup
	taskSvc    task.Service
	publisher  queue.Publisher
	logger     *slog.Logger
}

// NewComplianceWorker constructs a ComplianceWorker with all required dependencies.
func NewComplianceWorker(
	compliance *ComplianceService,
	rules JurisdictionRuleRepository,
	checks ComplianceCheckRepository,
	orgLookup OrgLookup,
	taskSvc task.Service,
	publisher queue.Publisher,
	logger *slog.Logger,
) *ComplianceWorker {
	return &ComplianceWorker{
		compliance: compliance, rules: rules, checks: checks,
		orgLookup: orgLookup, taskSvc: taskSvc,
		publisher: publisher, logger: logger,
	}
}

// RegisterHandlers wires the 7 compliance event handlers onto the given consumer.
func (w *ComplianceWorker) RegisterHandlers(consumer *queue.Consumer) {
	consumer.Register(queue.HandlerRegistration{
		Name:    "ai.compliance_rule_created",
		Subject: "quorant.ai.JurisdictionRuleCreated.>",
		Handler: w.HandleRuleChange,
	})
	consumer.Register(queue.HandlerRegistration{
		Name:    "ai.compliance_rule_updated",
		Subject: "quorant.ai.JurisdictionRuleUpdated.>",
		Handler: w.HandleRuleChange,
	})
	consumer.Register(queue.HandlerRegistration{
		Name:    "ai.compliance_org_updated",
		Subject: "quorant.org.OrganizationUpdated.>",
		Handler: w.HandleOrgChange,
	})
	consumer.Register(queue.HandlerRegistration{
		Name:    "ai.compliance_unit_created",
		Subject: "quorant.org.UnitCreated.>",
		Handler: w.HandleUnitChange,
	})
	consumer.Register(queue.HandlerRegistration{
		Name:    "ai.compliance_unit_deleted",
		Subject: "quorant.org.UnitDeleted.>",
		Handler: w.HandleUnitChange,
	})
	consumer.Register(queue.HandlerRegistration{
		Name:    "ai.compliance_doc_uploaded",
		Subject: "quorant.doc.DocumentUploaded.>",
		Handler: w.HandleDocumentUpload,
	})
	consumer.Register(queue.HandlerRegistration{
		Name:    "ai.compliance_governing_doc",
		Subject: "quorant.doc.GoverningDocUploaded.>",
		Handler: w.HandleGoverningDocUpload,
	})
}

// HandleRuleChange reacts to a jurisdiction rule being created or updated.
// It finds all orgs in the rule's jurisdiction and re-checks the affected category.
func (w *ComplianceWorker) HandleRuleChange(ctx context.Context, event queue.BaseEvent) error {
	var payload struct {
		RuleID       uuid.UUID `json:"rule_id"`
		Jurisdiction string    `json:"jurisdiction"`
		RuleCategory string    `json:"rule_category"`
	}
	if err := json.Unmarshal(event.Data, &payload); err != nil {
		return fmt.Errorf("compliance_worker: HandleRuleChange: unmarshal payload: %w", err)
	}

	orgs, err := w.orgLookup.ListByJurisdiction(ctx, payload.Jurisdiction)
	if err != nil {
		return fmt.Errorf("compliance_worker: HandleRuleChange: list orgs: %w", err)
	}

	for _, o := range orgs {
		result, err := w.compliance.CheckCompliance(ctx, o.ID, payload.RuleCategory)
		if err != nil {
			w.logger.Error("compliance_worker: HandleRuleChange: check failed",
				"org_id", o.ID, "category", payload.RuleCategory, "error", err)
			continue
		}
		if result.Status == "non_compliant" {
			if err := w.handleNonCompliant(ctx, o.ID, payload.RuleID, result); err != nil {
				w.logger.Error("compliance_worker: HandleRuleChange: handleNonCompliant failed",
					"org_id", o.ID, "error", err)
			}
		}
	}

	return nil
}

// HandleOrgChange reacts to an org being updated and runs a full compliance evaluation.
func (w *ComplianceWorker) HandleOrgChange(ctx context.Context, event queue.BaseEvent) error {
	var payload struct {
		OrgID uuid.UUID `json:"org_id"`
	}
	if err := json.Unmarshal(event.Data, &payload); err != nil {
		return fmt.Errorf("compliance_worker: HandleOrgChange: unmarshal payload: %w", err)
	}

	report, err := w.compliance.EvaluateCompliance(ctx, payload.OrgID)
	if err != nil {
		return fmt.Errorf("compliance_worker: HandleOrgChange: evaluate: %w", err)
	}

	for _, result := range report.Results {
		result := result // capture
		if result.Status == "non_compliant" {
			if err := w.handleNonCompliant(ctx, payload.OrgID, uuid.Nil, &result); err != nil {
				w.logger.Error("compliance_worker: HandleOrgChange: handleNonCompliant failed",
					"org_id", payload.OrgID, "category", result.Category, "error", err)
			}
		}
	}

	return nil
}

// HandleUnitChange reacts to a unit being created or deleted by re-checking website requirements.
func (w *ComplianceWorker) HandleUnitChange(ctx context.Context, event queue.BaseEvent) error {
	var payload struct {
		OrgID  uuid.UUID `json:"org_id"`
		UnitID uuid.UUID `json:"unit_id"`
	}
	if err := json.Unmarshal(event.Data, &payload); err != nil {
		return fmt.Errorf("compliance_worker: HandleUnitChange: unmarshal payload: %w", err)
	}

	result, err := w.compliance.CheckCompliance(ctx, payload.OrgID, "website_requirements")
	if err != nil {
		return fmt.Errorf("compliance_worker: HandleUnitChange: check: %w", err)
	}

	if result.Status == "non_compliant" {
		if err := w.handleNonCompliant(ctx, payload.OrgID, uuid.Nil, result); err != nil {
			return fmt.Errorf("compliance_worker: HandleUnitChange: handleNonCompliant: %w", err)
		}
	}

	return nil
}

// HandleDocumentUpload reacts to a document upload. Only reserve study documents trigger evaluation.
func (w *ComplianceWorker) HandleDocumentUpload(ctx context.Context, event queue.BaseEvent) error {
	var payload struct {
		OrgID      uuid.UUID `json:"org_id"`
		DocumentID uuid.UUID `json:"document_id"`
		Title      string    `json:"title"`
	}
	if err := json.Unmarshal(event.Data, &payload); err != nil {
		return fmt.Errorf("compliance_worker: HandleDocumentUpload: unmarshal payload: %w", err)
	}

	if !containsReserveStudyKeyword(payload.Title) {
		return nil
	}

	result, err := w.compliance.CheckCompliance(ctx, payload.OrgID, "reserve_study")
	if err != nil {
		return fmt.Errorf("compliance_worker: HandleDocumentUpload: check: %w", err)
	}

	if result.Status == "non_compliant" {
		if err := w.handleNonCompliant(ctx, payload.OrgID, uuid.Nil, result); err != nil {
			return fmt.Errorf("compliance_worker: HandleDocumentUpload: handleNonCompliant: %w", err)
		}
	}

	return nil
}

// HandleGoverningDocUpload reacts to a governing document upload with a full compliance re-evaluation.
func (w *ComplianceWorker) HandleGoverningDocUpload(ctx context.Context, event queue.BaseEvent) error {
	var payload struct {
		OrgID      uuid.UUID `json:"org_id"`
		DocumentID uuid.UUID `json:"document_id"`
	}
	if err := json.Unmarshal(event.Data, &payload); err != nil {
		return fmt.Errorf("compliance_worker: HandleGoverningDocUpload: unmarshal payload: %w", err)
	}

	// Re-use HandleOrgChange logic: full evaluation for the org.
	orgPayload, err := json.Marshal(map[string]any{"org_id": payload.OrgID})
	if err != nil {
		return fmt.Errorf("compliance_worker: HandleGoverningDocUpload: marshal: %w", err)
	}

	syntheticEvent := queue.NewBaseEvent(
		"quorant.org.OrganizationUpdated",
		"organization",
		payload.OrgID,
		payload.OrgID,
		orgPayload,
	)
	return w.HandleOrgChange(ctx, syntheticEvent)
}

// handleNonCompliant records the compliance failure, creates a task, and publishes an alert event.
func (w *ComplianceWorker) handleNonCompliant(ctx context.Context, orgID, ruleID uuid.UUID, result *ComplianceResult) error {
	// 1. Create a ComplianceCheck record.
	check := &ComplianceCheck{
		OrgID:  orgID,
		RuleID: ruleID,
		Status: result.Status,
	}
	savedCheck, err := w.checks.Create(ctx, check)
	if err != nil {
		return fmt.Errorf("handleNonCompliant: create check: %w", err)
	}

	// 2. Create a task to action the compliance failure.
	priority := "high"
	title := fmt.Sprintf("Compliance alert: %s — non-compliant", result.Category)
	createdTask, err := w.taskSvc.CreateTask(ctx, orgID, task.CreateTaskRequest{
		TaskTypeID:   uuid.Nil,
		Title:        title,
		ResourceType: "jurisdiction_rule",
		ResourceID:   ruleID,
		Priority:     &priority,
	}, uuid.Nil)
	if err != nil {
		return fmt.Errorf("handleNonCompliant: create task: %w", err)
	}

	// 3. Publish a ComplianceAlertRaised event.
	alertPayload, err := json.Marshal(map[string]any{
		"org_id":   orgID,
		"rule_id":  ruleID,
		"category": result.Category,
		"status":   result.Status,
		"message":  result.Details,
		"task_id":  createdTask.ID,
	})
	if err != nil {
		return fmt.Errorf("handleNonCompliant: marshal alert payload: %w", err)
	}

	alertEvent := queue.NewBaseEvent(
		"quorant.ai.ComplianceAlertRaised",
		"compliance_check",
		savedCheck.ID,
		orgID,
		alertPayload,
	)
	if err := w.publisher.Publish(ctx, alertEvent); err != nil {
		return fmt.Errorf("handleNonCompliant: publish alert: %w", err)
	}

	return nil
}

// containsReserveStudyKeyword returns true when the (lowercased) title contains any
// keyword associated with reserve study documents.
func containsReserveStudyKeyword(title string) bool {
	lower := strings.ToLower(title)
	return strings.Contains(lower, "reserve") ||
		strings.Contains(lower, "sirs") ||
		strings.Contains(lower, "structural integrity")
}
