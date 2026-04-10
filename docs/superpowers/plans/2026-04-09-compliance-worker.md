# Compliance Worker — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an event-driven compliance worker and daily cron job that re-evaluate org compliance when jurisdiction rules or org attributes change, surfacing non-compliant items as tasks.

**Architecture:** ComplianceWorker struct subscribes to 7 NATS event types in quorant-worker, delegates to ComplianceService for evaluation, creates tasks via task.Service. ComplianceAlertJob runs daily to check for upcoming rules. New `jurisdiction` field on organizations decouples compliance from address state.

**Tech Stack:** Go 1.25, PostgreSQL (pgx v5), NATS JetStream, existing scheduler/consumer infrastructure

**Spec:** `docs/superpowers/specs/2026-04-09-compliance-worker-design.md`

---

## File Map

### New Files

| File | Responsibility |
|---|---|
| `backend/migrations/20260409000026_org_jurisdiction.sql` | Add jurisdiction column, backfill, index, seed compliance_alert task type |
| `backend/internal/ai/compliance_worker.go` | ComplianceWorker event handler struct with 5 handler methods |
| `backend/internal/ai/compliance_alert_job.go` | ComplianceAlertJob daily scheduler job |
| `backend/internal/ai/compliance_worker_test.go` | Worker handler unit tests |
| `backend/internal/ai/compliance_alert_job_test.go` | Job unit tests |

### Modified Files

| File | Change |
|---|---|
| `backend/internal/org/domain.go` | Add `Jurisdiction *string` field to Organization |
| `backend/internal/org/org_repository.go` | Add `ListByJurisdiction` method |
| `backend/internal/org/org_postgres.go` | Add jurisdiction to scan/collect, implement `ListByJurisdiction` |
| `backend/internal/ai/org_iface.go` | Add `ListByJurisdiction` to OrgLookup |
| `backend/internal/ai/compliance_service.go` | Use `Jurisdiction` instead of `State` in `buildOrgComplianceContext` |
| `backend/internal/ai/jurisdiction_rule_repository.go` | Add `ListRulesEffectiveToday` method |
| `backend/internal/ai/jurisdiction_rule_postgres.go` | Implement `ListRulesEffectiveToday` |
| `backend/cmd/quorant-worker/main.go` | Wire ComplianceWorker + ComplianceAlertJob |
| `backend/internal/org/service.go` | Publish `OrganizationUpdated` event on Update |

---

## Task 1: Migration — Organization Jurisdiction + Task Type Seed

**Files:**
- Create: `backend/migrations/20260409000026_org_jurisdiction.sql`

- [ ] **Step 1: Write the migration**

```sql
-- backend/migrations/20260409000026_org_jurisdiction.sql

-- Add jurisdiction field to organizations (decoupled from address state).
ALTER TABLE organizations ADD COLUMN jurisdiction TEXT;
CREATE INDEX idx_organizations_jurisdiction ON organizations (jurisdiction) WHERE jurisdiction IS NOT NULL;

-- Backfill from existing state field.
UPDATE organizations SET jurisdiction = state WHERE state IS NOT NULL;

-- Seed system task type for compliance alerts.
INSERT INTO task_types (key, name, description, default_priority, source_module, auto_assign_role, is_active)
VALUES (
    'compliance_alert',
    'Compliance Alert',
    'Automated compliance status change notification',
    'high',
    'ai',
    'hoa_manager',
    TRUE
) ON CONFLICT DO NOTHING;
```

- [ ] **Step 2: Commit**

```bash
git add backend/migrations/20260409000026_org_jurisdiction.sql
git commit -m "feat: add jurisdiction column to organizations and seed compliance_alert task type (issue #59)"
```

---

## Task 2: Org Domain + Repository — Jurisdiction Field

**Files:**
- Modify: `backend/internal/org/domain.go`
- Modify: `backend/internal/org/org_repository.go`
- Modify: `backend/internal/org/org_postgres.go`

- [ ] **Step 1: Add Jurisdiction field to Organization struct**

In `backend/internal/org/domain.go`, add after the `State` field (line 20):

```go
Jurisdiction *string        `json:"jurisdiction,omitempty"`
```

- [ ] **Step 2: Add ListByJurisdiction to OrgRepository interface**

In `backend/internal/org/org_repository.go`, add before the closing brace:

```go
// ListByJurisdiction returns all non-deleted orgs in the given jurisdiction.
ListByJurisdiction(ctx context.Context, jurisdiction string) ([]Organization, error)
```

- [ ] **Step 3: Update scanOrg to include jurisdiction**

In `backend/internal/org/org_postgres.go`, the `scanOrg` function scans fields in order matching the SELECT column list. Add `&o.Jurisdiction` after `&o.State` (line 449). Do the same in `collectOrgs` (the rows.Scan around line 484).

Also update ALL SELECT column lists that query organizations to include `jurisdiction` after `state`. These are in `FindByID`, `FindBySlug`, `ListByUserAccess`, `ListChildren`, `Create`, and `Update` methods. The column lists look like:

```sql
SELECT id, parent_id, type, name, slug, path,
       address_line1, address_line2, city, state, jurisdiction, zip,
       phone, email, website, logo_url,
       locale, timezone, currency_code, country,
       settings,
       created_at, updated_at, deleted_at
```

Also update the INSERT in `Create` and the UPDATE in `Update` to include `jurisdiction`.

- [ ] **Step 4: Implement ListByJurisdiction**

```go
func (r *PostgresOrgRepository) ListByJurisdiction(ctx context.Context, jurisdiction string) ([]Organization, error) {
	const q = `
		SELECT id, parent_id, type, name, slug, path,
		       address_line1, address_line2, city, state, jurisdiction, zip,
		       phone, email, website, logo_url,
		       locale, timezone, currency_code, country,
		       settings,
		       created_at, updated_at, deleted_at
		FROM organizations
		WHERE jurisdiction = $1 AND deleted_at IS NULL
		ORDER BY name`

	rows, err := r.pool.Query(ctx, q, jurisdiction)
	if err != nil {
		return nil, fmt.Errorf("org: ListByJurisdiction: %w", err)
	}
	defer rows.Close()
	return collectOrgs(rows, "ListByJurisdiction")
}
```

- [ ] **Step 5: Verify compilation**

Run: `cd backend && go build ./internal/org/...`

Expected: Compiles. Some tests may need the new field in scan — fix any broken tests.

- [ ] **Step 6: Run org tests**

Run: `cd backend && go test ./internal/org/... -count=1 -v 2>&1 | tail -20`

Expected: All pass (mock repos may need updating if they check interface satisfaction).

- [ ] **Step 7: Commit**

```bash
git add backend/internal/org/domain.go backend/internal/org/org_repository.go backend/internal/org/org_postgres.go
git commit -m "feat: add Organization.Jurisdiction field and ListByJurisdiction query (issue #59)"
```

---

## Task 3: Update AI Module — OrgLookup + ComplianceService

**Files:**
- Modify: `backend/internal/ai/org_iface.go`
- Modify: `backend/internal/ai/compliance_service.go`
- Modify: `backend/internal/ai/jurisdiction_rule_repository.go`
- Modify: `backend/internal/ai/jurisdiction_rule_postgres.go`

- [ ] **Step 1: Add ListByJurisdiction to OrgLookup interface**

In `backend/internal/ai/org_iface.go`, add:

```go
ListByJurisdiction(ctx context.Context, jurisdiction string) ([]org.Organization, error)
```

- [ ] **Step 2: Update buildOrgComplianceContext to use Jurisdiction**

In `backend/internal/ai/compliance_service.go`, update the `buildOrgComplianceContext` function:

```go
func buildOrgComplianceContext(o *org.Organization) OrgComplianceContext {
	ctx := OrgComplianceContext{
		OrgID:      o.ID,
		HasWebsite: o.Website != nil && *o.Website != "",
	}
	if o.Jurisdiction != nil {
		ctx.Jurisdiction = *o.Jurisdiction
	} else if o.State != nil {
		ctx.Jurisdiction = *o.State // fallback to state if jurisdiction not set
	}
	return ctx
}
```

Also update `EvaluateCompliance` and `CheckCompliance` to check `Jurisdiction` first, then fall back to `State`:

Replace the `orgEntity.State == nil` check with:

```go
jurisdiction := ""
if orgEntity.Jurisdiction != nil {
    jurisdiction = *orgEntity.Jurisdiction
} else if orgEntity.State != nil {
    jurisdiction = *orgEntity.State
}
if jurisdiction == "" {
    return nil, fmt.Errorf("ai: EvaluateCompliance: org %s has no jurisdiction configured", orgID)
}
```

- [ ] **Step 3: Add ListRulesEffectiveToday to JurisdictionRuleRepository**

In `backend/internal/ai/jurisdiction_rule_repository.go`, add:

```go
ListRulesEffectiveToday(ctx context.Context) ([]JurisdictionRule, error)
```

- [ ] **Step 4: Implement ListRulesEffectiveToday**

In `backend/internal/ai/jurisdiction_rule_postgres.go`:

```go
func (r *PostgresJurisdictionRuleRepository) ListRulesEffectiveToday(ctx context.Context) ([]JurisdictionRule, error) {
	q := `SELECT ` + jurisdictionRuleCols + `
		FROM jurisdiction_rules
		WHERE effective_date = CURRENT_DATE
		  AND (expiration_date IS NULL OR expiration_date > CURRENT_DATE)
		ORDER BY jurisdiction, rule_category`

	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("ai: ListRulesEffectiveToday: %w", err)
	}
	defer rows.Close()
	return collectJurisdictionRules(rows)
}
```

- [ ] **Step 5: Verify compilation and tests**

Run: `cd backend && go build ./internal/ai/... && go test ./internal/ai/... -count=1`

Expected: Compiles and tests pass. Update mock in `compliance_service_test.go` to include `ListByJurisdiction` and `ListRulesEffectiveToday` stubs.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/ai/org_iface.go backend/internal/ai/compliance_service.go backend/internal/ai/jurisdiction_rule_repository.go backend/internal/ai/jurisdiction_rule_postgres.go backend/internal/ai/compliance_service_test.go
git commit -m "feat: update OrgLookup and ComplianceService for jurisdiction field (issue #59)"
```

---

## Task 4: Org Event Publishing

**Files:**
- Modify: `backend/internal/org/service.go`

- [ ] **Step 1: Add event publishing to OrgService.UpdateOrganization**

In `backend/internal/org/service.go`, find the `UpdateOrganization` method. After the successful repo update call, add event publishing:

```go
// Publish org updated event for downstream consumers (compliance worker, etc.)
payload, _ := json.Marshal(map[string]any{
    "org_id": updated.ID,
    "name":   updated.Name,
})
event := queue.NewBaseEvent("quorant.org.OrganizationUpdated", "organization", updated.ID, updated.ID, payload)
if err := s.publisher.Publish(ctx, event); err != nil {
    s.logger.Error("failed to publish OrganizationUpdated", "org_id", updated.ID, "error", err)
}
```

Do the same for unit create/delete if those methods exist. Check for `CreateUnit` and `DeleteUnit`/`SoftDeleteUnit` methods and add:

```go
// After unit create:
payload, _ := json.Marshal(map[string]any{"org_id": unit.OrgID, "unit_id": unit.ID})
event := queue.NewBaseEvent("quorant.org.UnitCreated", "unit", unit.ID, unit.OrgID, payload)
s.publisher.Publish(ctx, event)

// After unit delete:
payload, _ := json.Marshal(map[string]any{"org_id": orgID, "unit_id": unitID})
event := queue.NewBaseEvent("quorant.org.UnitDeleted", "unit", unitID, orgID, payload)
s.publisher.Publish(ctx, event)
```

- [ ] **Step 2: Verify compilation and tests**

Run: `cd backend && go build ./internal/org/... && go test ./internal/org/... -count=1`

Expected: Compiles and tests pass.

- [ ] **Step 3: Commit**

```bash
git add backend/internal/org/service.go
git commit -m "feat: publish OrganizationUpdated and Unit events from org service (issue #59)"
```

---

## Task 5: ComplianceWorker Event Handlers

**Files:**
- Create: `backend/internal/ai/compliance_worker.go`
- Create: `backend/internal/ai/compliance_worker_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// backend/internal/ai/compliance_worker_test.go
package ai_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/ai"
	"github.com/quorant/quorant/internal/org"
	"github.com/quorant/quorant/internal/platform/queue"
	"github.com/quorant/quorant/internal/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockTaskService implements task.Service for testing.
type mockTaskService struct {
	created []task.Task
}

func (m *mockTaskService) CreateTask(ctx context.Context, orgID uuid.UUID, req task.CreateTaskRequest, createdBy uuid.UUID) (*task.Task, error) {
	t := task.Task{ID: uuid.New(), OrgID: orgID, Title: req.Title, Status: "open"}
	m.created = append(m.created, t)
	return &t, nil
}
// Stub remaining task.Service methods (ListTaskTypes, CreateTaskType, etc.) — return nil/empty.
func (m *mockTaskService) ListTaskTypes(ctx context.Context, orgID uuid.UUID) ([]task.TaskType, error) { return nil, nil }
func (m *mockTaskService) CreateTaskType(ctx context.Context, orgID uuid.UUID, req task.CreateTaskTypeRequest) (*task.TaskType, error) { return nil, nil }
func (m *mockTaskService) UpdateTaskType(ctx context.Context, id uuid.UUID, tt *task.TaskType) (*task.TaskType, error) { return nil, nil }
func (m *mockTaskService) GetTask(ctx context.Context, id uuid.UUID) (*task.Task, error) { return nil, nil }
func (m *mockTaskService) ListTasks(ctx context.Context, orgID uuid.UUID, limit int, afterID *uuid.UUID) ([]task.Task, bool, error) { return nil, false, nil }
func (m *mockTaskService) ListMyTasks(ctx context.Context, userID uuid.UUID) ([]task.Task, error) { return nil, nil }
func (m *mockTaskService) UpdateTask(ctx context.Context, id uuid.UUID, t *task.Task) (*task.Task, error) { return nil, nil }
func (m *mockTaskService) AssignTask(ctx context.Context, taskID uuid.UUID, req task.AssignTaskRequest, assignedBy uuid.UUID) (*task.Task, error) { return nil, nil }
func (m *mockTaskService) TransitionTask(ctx context.Context, taskID uuid.UUID, req task.TransitionTaskRequest, changedBy uuid.UUID) (*task.Task, error) { return nil, nil }
func (m *mockTaskService) AddComment(ctx context.Context, taskID uuid.UUID, req task.AddCommentRequest, authorID uuid.UUID) (*task.TaskComment, error) { return nil, nil }
func (m *mockTaskService) ListComments(ctx context.Context, taskID uuid.UUID) ([]task.TaskComment, error) { return nil, nil }
func (m *mockTaskService) ToggleChecklistItem(ctx context.Context, taskID uuid.UUID, itemID string) (*task.Task, error) { return nil, nil }

func setupComplianceWorker(t *testing.T) (*ai.ComplianceWorker, *mockTaskService, *mockJurisdictionRuleRepo, *mockOrgLookupWithJurisdiction) {
	t.Helper()
	ruleRepo := &mockJurisdictionRuleRepo{}
	checkRepo := &mockComplianceCheckRepo{}
	taskSvc := &mockTaskService{}
	state := "FL"
	orgLookup := &mockOrgLookupWithJurisdiction{
		orgs: map[uuid.UUID]*org.Organization{},
	}
	publisher := queue.NewInMemoryPublisher()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	complianceSvc := ai.NewComplianceService(ruleRepo, checkRepo, orgLookup, logger)
	complianceSvc.RegisterEvaluator("website_requirements", ai.EvaluateWebsiteRequirements)
	complianceSvc.RegisterEvaluator("meeting_notice", ai.EvaluateMeetingNotice)
	complianceSvc.RegisterEvaluator("fine_limits", ai.EvaluateFineLimits)
	complianceSvc.RegisterEvaluator("reserve_study", ai.EvaluateReserveStudy)
	complianceSvc.RegisterEvaluator("record_retention", ai.EvaluateRecordRetention)
	complianceSvc.RegisterEvaluator("voting_rules", ai.EvaluateVotingRules)
	complianceSvc.RegisterEvaluator("estoppel", ai.EvaluateEstoppel)

	worker := ai.NewComplianceWorker(complianceSvc, ruleRepo, checkRepo, orgLookup, taskSvc, publisher, logger)
	return worker, taskSvc, ruleRepo, orgLookup
}

func TestComplianceWorker_HandleRuleChange(t *testing.T) {
	worker, taskSvc, ruleRepo, orgLookup := setupComplianceWorker(t)

	// Seed an org in FL and a website_requirements rule
	orgID := uuid.New()
	jurisdiction := "FL"
	orgLookup.orgs[orgID] = &org.Organization{
		ID: orgID, Type: "hoa", Name: "Test HOA", Jurisdiction: &jurisdiction,
	}
	orgLookup.byJurisdiction["FL"] = []org.Organization{*orgLookup.orgs[orgID]}

	ruleID := uuid.New()
	ruleRepo.rules = append(ruleRepo.rules, ai.JurisdictionRule{
		ID: ruleID, Jurisdiction: "FL", RuleCategory: "website_requirements",
		RuleKey: "required_for_unit_count", ValueType: "integer",
		Value: json.RawMessage(`100`), StatuteReference: "FL HB 1203",
		EffectiveDate: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	})

	// Fire rule change event
	payload, _ := json.Marshal(map[string]any{
		"rule_id": ruleID, "jurisdiction": "FL", "rule_category": "website_requirements",
	})
	event := queue.NewBaseEvent("quorant.ai.JurisdictionRuleCreated", "jurisdiction_rule", ruleID, uuid.Nil, payload)
	err := worker.HandleRuleChange(context.Background(), event)
	require.NoError(t, err)

	// Website requirements for an org with 0 units → not_applicable, no task created
	assert.Empty(t, taskSvc.created)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd backend && go test ./internal/ai/... -run TestComplianceWorker -v`

Expected: FAIL — `NewComplianceWorker` not defined.

- [ ] **Step 3: Write the ComplianceWorker implementation**

```go
// backend/internal/ai/compliance_worker.go
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

// ComplianceWorker handles NATS events that may affect org compliance status.
type ComplianceWorker struct {
	compliance *ComplianceService
	rules      JurisdictionRuleRepository
	checks     ComplianceCheckRepository
	orgLookup  OrgLookup
	taskSvc    task.Service
	publisher  queue.Publisher
	logger     *slog.Logger

	complianceAlertTaskTypeID uuid.UUID // looked up once, cached
}

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
		compliance: compliance,
		rules:      rules,
		checks:     checks,
		orgLookup:  orgLookup,
		taskSvc:    taskSvc,
		publisher:  publisher,
		logger:     logger,
	}
}

// RegisterHandlers registers NATS event handlers for compliance re-evaluation.
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

	w.logger.Info("registered compliance worker handlers", "count", 7)
}

// HandleRuleChange re-evaluates all orgs in the affected jurisdiction for the changed category.
func (w *ComplianceWorker) HandleRuleChange(ctx context.Context, event queue.BaseEvent) error {
	var payload struct {
		RuleID       uuid.UUID `json:"rule_id"`
		Jurisdiction string    `json:"jurisdiction"`
		RuleCategory string    `json:"rule_category"`
	}
	if err := json.Unmarshal(event.Data, &payload); err != nil {
		return fmt.Errorf("parsing rule change event: %w", err)
	}

	orgs, err := w.orgLookup.ListByJurisdiction(ctx, payload.Jurisdiction)
	if err != nil {
		return fmt.Errorf("listing orgs for jurisdiction %s: %w", payload.Jurisdiction, err)
	}

	var compliant, nonCompliant int
	for _, o := range orgs {
		result, err := w.compliance.CheckCompliance(ctx, o.ID, payload.RuleCategory)
		if err != nil {
			w.logger.Error("compliance check failed", "org_id", o.ID, "category", payload.RuleCategory, "error", err)
			continue
		}
		if result.Status == "non_compliant" {
			nonCompliant++
			w.handleNonCompliant(ctx, o.ID, payload.RuleID, result)
		} else {
			compliant++
		}
	}

	w.logger.Info("rule change re-evaluation complete",
		"jurisdiction", payload.Jurisdiction,
		"category", payload.RuleCategory,
		"orgs_evaluated", len(orgs),
		"compliant", compliant,
		"non_compliant", nonCompliant,
	)
	return nil
}

// HandleOrgChange re-evaluates compliance for a single org across all categories.
func (w *ComplianceWorker) HandleOrgChange(ctx context.Context, event queue.BaseEvent) error {
	var payload struct {
		OrgID uuid.UUID `json:"org_id"`
	}
	if err := json.Unmarshal(event.Data, &payload); err != nil {
		return fmt.Errorf("parsing org change event: %w", err)
	}

	report, err := w.compliance.EvaluateCompliance(ctx, payload.OrgID)
	if err != nil {
		return fmt.Errorf("evaluating compliance for org %s: %w", payload.OrgID, err)
	}

	for _, result := range report.Results {
		if result.Status == "non_compliant" {
			ruleID := uuid.Nil
			if len(result.Rules) > 0 {
				ruleID = result.Rules[0].ID
			}
			w.handleNonCompliant(ctx, payload.OrgID, ruleID, &result)
		}
	}

	w.logger.Info("org change compliance evaluation complete",
		"org_id", payload.OrgID,
		"compliant", report.Summary.Compliant,
		"non_compliant", report.Summary.NonCompliant,
	)
	return nil
}

// HandleUnitChange re-evaluates website_requirements for the org (unit count threshold).
func (w *ComplianceWorker) HandleUnitChange(ctx context.Context, event queue.BaseEvent) error {
	var payload struct {
		OrgID  uuid.UUID `json:"org_id"`
		UnitID uuid.UUID `json:"unit_id"`
	}
	if err := json.Unmarshal(event.Data, &payload); err != nil {
		return fmt.Errorf("parsing unit change event: %w", err)
	}

	result, err := w.compliance.CheckCompliance(ctx, payload.OrgID, "website_requirements")
	if err != nil {
		return fmt.Errorf("checking website_requirements for org %s: %w", payload.OrgID, err)
	}
	if result.Status == "non_compliant" {
		ruleID := uuid.Nil
		if len(result.Rules) > 0 {
			ruleID = result.Rules[0].ID
		}
		w.handleNonCompliant(ctx, payload.OrgID, ruleID, result)
	}
	return nil
}

// HandleDocumentUpload checks if a reserve study was uploaded and re-evaluates.
func (w *ComplianceWorker) HandleDocumentUpload(ctx context.Context, event queue.BaseEvent) error {
	var payload struct {
		OrgID      uuid.UUID `json:"org_id"`
		DocumentID uuid.UUID `json:"document_id"`
		Title      string    `json:"title"`
	}
	if err := json.Unmarshal(event.Data, &payload); err != nil {
		return fmt.Errorf("parsing document upload event: %w", err)
	}

	// Coarse filter: only re-evaluate reserve_study if title suggests it's a reserve study
	// False positives are acceptable — the evaluator determines actual compliance
	if !containsReserveStudyKeyword(payload.Title) {
		return nil
	}

	result, err := w.compliance.CheckCompliance(ctx, payload.OrgID, "reserve_study")
	if err != nil {
		return fmt.Errorf("checking reserve_study for org %s: %w", payload.OrgID, err)
	}
	if result.Status == "non_compliant" {
		ruleID := uuid.Nil
		if len(result.Rules) > 0 {
			ruleID = result.Rules[0].ID
		}
		w.handleNonCompliant(ctx, payload.OrgID, ruleID, result)
	}
	return nil
}

// HandleGoverningDocUpload re-evaluates all categories when governing docs change.
func (w *ComplianceWorker) HandleGoverningDocUpload(ctx context.Context, event queue.BaseEvent) error {
	var payload struct {
		OrgID      uuid.UUID `json:"org_id"`
		DocumentID uuid.UUID `json:"document_id"`
	}
	if err := json.Unmarshal(event.Data, &payload); err != nil {
		return fmt.Errorf("parsing governing doc upload event: %w", err)
	}

	// Full re-evaluation — governing doc changes can affect any category
	return w.HandleOrgChange(ctx, queue.NewBaseEvent(
		"quorant.org.OrganizationUpdated", "organization",
		payload.OrgID, payload.OrgID,
		event.Data,
	))
}

// handleNonCompliant creates a compliance check record, task, and publishes an alert event.
func (w *ComplianceWorker) handleNonCompliant(ctx context.Context, orgID, ruleID uuid.UUID, result *ComplianceResult) {
	// Record compliance check
	details, _ := json.Marshal(result.Details)
	_, err := w.checks.Create(ctx, &ComplianceCheck{
		OrgID:  orgID,
		RuleID: ruleID,
		Status: "non_compliant",
		Details: details,
	})
	if err != nil {
		w.logger.Error("failed to create compliance check", "org_id", orgID, "error", err)
	}

	// Create task
	message := fmt.Sprintf("Compliance alert: %s — non-compliant", result.Category)
	desc := message
	priority := "high"
	createdTask, err := w.taskSvc.CreateTask(ctx, orgID, task.CreateTaskRequest{
		TaskTypeID:   w.complianceAlertTaskTypeID,
		Title:        message,
		Description:  &desc,
		ResourceType: "jurisdiction_rule",
		ResourceID:   ruleID,
		Priority:     &priority,
	}, uuid.Nil)
	if err != nil {
		w.logger.Error("failed to create compliance task", "org_id", orgID, "error", err)
		return
	}

	// Publish ComplianceAlertRaised event
	alertPayload, _ := json.Marshal(map[string]any{
		"org_id":   orgID,
		"rule_id":  ruleID,
		"category": result.Category,
		"status":   "non_compliant",
		"message":  message,
		"task_id":  createdTask.ID,
	})
	alertEvent := queue.NewBaseEvent("quorant.ai.ComplianceAlertRaised", "compliance_check", orgID, orgID, alertPayload)
	if err := w.publisher.Publish(ctx, alertEvent); err != nil {
		w.logger.Error("failed to publish ComplianceAlertRaised", "org_id", orgID, "error", err)
	}
}

// containsReserveStudyKeyword returns true if the title suggests a reserve study document.
func containsReserveStudyKeyword(title string) bool {
	lower := strings.ToLower(title)
	return strings.Contains(lower, "reserve") || strings.Contains(lower, "sirs") || strings.Contains(lower, "structural integrity")
}
```

Note: Add `"strings"` to the imports.

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd backend && go test ./internal/ai/... -run TestComplianceWorker -v`

Expected: Tests PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/ai/compliance_worker.go backend/internal/ai/compliance_worker_test.go
git commit -m "feat: add ComplianceWorker event handlers (issue #59)"
```

---

## Task 6: ComplianceAlertJob (Daily Cron)

**Files:**
- Create: `backend/internal/ai/compliance_alert_job.go`
- Create: `backend/internal/ai/compliance_alert_job_test.go`

- [ ] **Step 1: Write the failing test**

```go
// backend/internal/ai/compliance_alert_job_test.go
package ai_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/ai"
	"github.com/quorant/quorant/internal/org"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestComplianceAlertJob_UpcomingRules(t *testing.T) {
	ruleRepo := &mockJurisdictionRuleRepo{}
	checkRepo := &mockComplianceCheckRepo{}
	taskSvc := &mockTaskService{}
	jurisdiction := "FL"
	orgLookup := &mockOrgLookupWithJurisdiction{
		orgs: map[uuid.UUID]*org.Organization{},
		byJurisdiction: map[string][]org.Organization{},
	}
	publisher := queue.NewInMemoryPublisher()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	complianceSvc := ai.NewComplianceService(ruleRepo, checkRepo, orgLookup, logger)

	job := ai.NewComplianceAlertJob(ruleRepo, checkRepo, orgLookup, complianceSvc, taskSvc, publisher, logger)

	// Seed an upcoming rule (10 days from now)
	ruleRepo.rules = append(ruleRepo.rules, ai.JurisdictionRule{
		ID: uuid.New(), Jurisdiction: "FL", RuleCategory: "website_requirements",
		RuleKey: "required_for_unit_count", ValueType: "integer",
		Value: json.RawMessage(`50`), StatuteReference: "test",
		EffectiveDate: time.Now().AddDate(0, 0, 10),
	})

	// Seed an org in FL
	orgID := uuid.New()
	orgLookup.orgs[orgID] = &org.Organization{ID: orgID, Name: "Test HOA", Jurisdiction: &jurisdiction}
	orgLookup.byJurisdiction["FL"] = []org.Organization{*orgLookup.orgs[orgID]}

	err := job.Run(context.Background())
	require.NoError(t, err)

	// Advisory task should be created
	assert.GreaterOrEqual(t, len(taskSvc.created), 1)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/ai/... -run TestComplianceAlertJob -v`

Expected: FAIL — `NewComplianceAlertJob` not defined.

- [ ] **Step 3: Write the ComplianceAlertJob implementation**

```go
// backend/internal/ai/compliance_alert_job.go
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

	complianceAlertTaskTypeID uuid.UUID
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
		rules:     rules,
		checks:    checks,
		orgLookup: orgLookup,
		compliance: compliance,
		taskSvc:   taskSvc,
		publisher: publisher,
		logger:    logger,
	}
}

func (j *ComplianceAlertJob) Name() string { return "compliance_alert" }

func (j *ComplianceAlertJob) Run(ctx context.Context) error {
	// 1. Upcoming rules — create advisory tasks
	upcoming, err := j.rules.ListUpcomingRules(ctx, 30)
	if err != nil {
		return fmt.Errorf("listing upcoming rules: %w", err)
	}

	advisoryCount := 0
	for _, rule := range upcoming {
		orgs, err := j.orgLookup.ListByJurisdiction(ctx, rule.Jurisdiction)
		if err != nil {
			j.logger.Error("listing orgs for upcoming rule", "jurisdiction", rule.Jurisdiction, "error", err)
			continue
		}
		for _, o := range orgs {
			desc := fmt.Sprintf("New %s rule takes effect on %s: %s (%s)",
				rule.RuleCategory,
				rule.EffectiveDate.Format("2006-01-02"),
				rule.Notes,
				rule.StatuteReference,
			)
			priority := "normal"
			_, err := j.taskSvc.CreateTask(ctx, o.ID, task.CreateTaskRequest{
				TaskTypeID:   j.complianceAlertTaskTypeID,
				Title:        fmt.Sprintf("Upcoming rule: %s (%s)", rule.RuleCategory, rule.Jurisdiction),
				Description:  &desc,
				ResourceType: "jurisdiction_rule",
				ResourceID:   rule.ID,
				Priority:     &priority,
			}, uuid.Nil)
			if err != nil {
				j.logger.Error("creating advisory task", "org_id", o.ID, "rule_id", rule.ID, "error", err)
				continue
			}
			advisoryCount++
		}
	}

	// 2. Rules effective today — trigger re-evaluation
	todayRules, err := j.rules.ListRulesEffectiveToday(ctx)
	if err != nil {
		return fmt.Errorf("listing rules effective today: %w", err)
	}

	evalCount := 0
	for _, rule := range todayRules {
		orgs, err := j.orgLookup.ListByJurisdiction(ctx, rule.Jurisdiction)
		if err != nil {
			j.logger.Error("listing orgs for today's rule", "jurisdiction", rule.Jurisdiction, "error", err)
			continue
		}
		for _, o := range orgs {
			result, err := j.compliance.CheckCompliance(ctx, o.ID, rule.RuleCategory)
			if err != nil {
				j.logger.Error("compliance check failed", "org_id", o.ID, "error", err)
				continue
			}
			if result.Status == "non_compliant" {
				// Create compliance check + task + event (same pattern as worker)
				details, _ := json.Marshal(result.Details)
				j.checks.Create(ctx, &ComplianceCheck{
					OrgID: o.ID, RuleID: rule.ID, Status: "non_compliant", Details: details,
				})

				desc := fmt.Sprintf("Non-compliant: %s", result.Category)
				priority := "high"
				createdTask, err := j.taskSvc.CreateTask(ctx, o.ID, task.CreateTaskRequest{
					TaskTypeID:   j.complianceAlertTaskTypeID,
					Title:        desc,
					ResourceType: "jurisdiction_rule",
					ResourceID:   rule.ID,
					Priority:     &priority,
				}, uuid.Nil)
				if err != nil {
					j.logger.Error("creating enforcement task", "org_id", o.ID, "error", err)
					continue
				}

				alertPayload, _ := json.Marshal(map[string]any{
					"org_id": o.ID, "rule_id": rule.ID, "category": rule.RuleCategory,
					"status": "non_compliant", "task_id": createdTask.ID,
				})
				j.publisher.Publish(ctx, queue.NewBaseEvent(
					"quorant.ai.ComplianceAlertRaised", "compliance_check", o.ID, o.ID, alertPayload,
				))
				evalCount++
			}
		}
	}

	j.logger.Info("compliance alert job complete",
		"upcoming_rules", len(upcoming),
		"advisory_tasks", advisoryCount,
		"today_rules", len(todayRules),
		"enforcement_tasks", evalCount,
	)
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd backend && go test ./internal/ai/... -run TestComplianceAlertJob -v`

Expected: Tests PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/ai/compliance_alert_job.go backend/internal/ai/compliance_alert_job_test.go
git commit -m "feat: add ComplianceAlertJob daily cron for upcoming rule alerts (issue #59)"
```

---

## Task 7: Wire into Worker Binary

**Files:**
- Modify: `backend/cmd/quorant-worker/main.go`

- [ ] **Step 1: Add compliance worker and job to worker main**

After the ingestion worker registration (line 85), add:

```go
// Compliance worker — re-evaluates orgs on rule/org/unit/document changes
jurisdictionRuleRepo := ai.NewPostgresJurisdictionRuleRepository(pool)
complianceCheckRepo := ai.NewPostgresComplianceCheckRepository(pool)
orgRepo := org.NewPostgresOrgRepository(pool)
complianceService := ai.NewComplianceService(jurisdictionRuleRepo, complianceCheckRepo, orgRepo, logger)
complianceService.RegisterEvaluator("meeting_notice", ai.EvaluateMeetingNotice)
complianceService.RegisterEvaluator("fine_limits", ai.EvaluateFineLimits)
complianceService.RegisterEvaluator("reserve_study", ai.EvaluateReserveStudy)
complianceService.RegisterEvaluator("website_requirements", ai.EvaluateWebsiteRequirements)
complianceService.RegisterEvaluator("record_retention", ai.EvaluateRecordRetention)
complianceService.RegisterEvaluator("voting_rules", ai.EvaluateVotingRules)
complianceService.RegisterEvaluator("estoppel", ai.EvaluateEstoppel)

taskRepo := task.NewPostgresTaskRepository(pool)
taskService := task.NewTaskService(taskRepo, audit.NewNoopAuditor(), natsPublisher, logger)

complianceWorker := ai.NewComplianceWorker(complianceService, jurisdictionRuleRepo, complianceCheckRepo, orgRepo, taskService, natsPublisher, logger)
complianceWorker.RegisterHandlers(consumer)
```

Add the compliance alert job to the scheduler section:

```go
complianceAlertJob := ai.NewComplianceAlertJob(jurisdictionRuleRepo, complianceCheckRepo, orgRepo, complianceService, taskService, natsPublisher, logger)
sched.Register(complianceAlertJob, 24*time.Hour)
```

Add imports for `"github.com/quorant/quorant/internal/org"`, `"github.com/quorant/quorant/internal/task"`, `"github.com/quorant/quorant/internal/audit"`.

- [ ] **Step 2: Update handler count log**

Update `logger.Info("registered event handlers", "count", 5)` to `"count", 12` (5 existing + 7 new).

- [ ] **Step 3: Verify compilation**

Run: `cd backend && go build ./cmd/quorant-worker/...`

Expected: Compiles cleanly.

- [ ] **Step 4: Commit**

```bash
git add backend/cmd/quorant-worker/main.go
git commit -m "feat: wire compliance worker and alert job into quorant-worker (issue #59)"
```

---

## Task 8: Final Verification

- [ ] **Step 1: Run full test suite**

Run: `cd backend && go test ./... -count=1 2>&1 | tail -30`

Expected: All packages pass.

- [ ] **Step 2: Build both binaries**

Run: `cd backend && go build ./cmd/quorant-api/... && go build ./cmd/quorant-worker/...`

Expected: Both compile cleanly.

- [ ] **Step 3: Commit any remaining changes**

```bash
git add -A && git status
# If changes exist:
git commit -m "feat: complete compliance worker implementation (closes #59)"
```
