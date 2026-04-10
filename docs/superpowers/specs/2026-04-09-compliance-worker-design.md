# Compliance Worker — Design Spec

**Date:** 2026-04-09
**Issue:** [#59 — Compliance worker: rule change re-evaluation and proactive alerts](https://github.com/douglaslinsmeyer/quorant/issues/59)
**Depends on:** [#7 — State compliance rules engine](https://github.com/douglaslinsmeyer/quorant/issues/7) (complete)
**Status:** Draft

---

## Context

The compliance engine (#7) added deterministic jurisdiction rule lookups, a `ComplianceResolver` interface, and 7 category evaluators. It publishes `JurisdictionRuleCreated` and `JurisdictionRuleUpdated` events when platform admins manage rules. What's missing is the **async worker** that reacts to those events — and to org-level changes that affect compliance status — by re-evaluating affected orgs and surfacing non-compliant items as tasks.

---

## Scope

### In Scope

1. **ComplianceWorker** — event handler struct in `quorant-worker` that subscribes to 7 NATS event types and re-evaluates compliance when something changes
2. **ComplianceAlertJob** — daily scheduler job that checks for upcoming rules and creates advisory tasks
3. **Organization.Jurisdiction field** — new dedicated field replacing reliance on the address `State` field for compliance matching
4. **OrgRepository.ListByJurisdiction** — new query method for finding orgs in a jurisdiction
5. **System TaskType** — seeded `compliance_alert` task type for all compliance-generated tasks
6. **ComplianceAlertRaised event** — published when a non-compliant result is detected

### Out of Scope

- UI for compliance dashboard (separate feature)
- Email/SMS notification delivery (handled by existing com module when tasks are created)
- Compliance worker for Tier 2 (interpretive) policy changes

---

## Migration: Organization Jurisdiction Field

Add a `jurisdiction` column to the `organizations` table, backfilled from the existing `state` field.

```sql
ALTER TABLE organizations ADD COLUMN jurisdiction TEXT;
CREATE INDEX idx_organizations_jurisdiction ON organizations (jurisdiction) WHERE jurisdiction IS NOT NULL;
UPDATE organizations SET jurisdiction = state WHERE state IS NOT NULL;
```

This decouples the compliance jurisdiction from the mailing address state. The `jurisdiction` field is what the compliance engine matches against `jurisdiction_rules.jurisdiction`. Non-US jurisdictions (e.g., Canadian provinces) can be represented without conflicting with US state codes.

Update `buildOrgComplianceContext` in `compliance_service.go` to read from `org.Jurisdiction` instead of `org.State`.

---

## OrgRepository Extension

Add to `OrgRepository` interface:

```go
ListByJurisdiction(ctx context.Context, jurisdiction string) ([]Organization, error)
```

PostgreSQL implementation:

```sql
SELECT <org_cols>
FROM organizations
WHERE jurisdiction = $1
  AND deleted_at IS NULL
ORDER BY name
```

---

## Seed TaskType

```sql
INSERT INTO task_types (key, name, description, default_priority, source_module, auto_assign_role, is_active)
VALUES (
    'compliance_alert',
    'Compliance Alert',
    'Automated compliance status change notification',
    'high',
    'ai',
    'hoa_manager',
    TRUE
);
```

This is a system-defined task type (`org_id IS NULL`). All compliance-generated tasks reference this type, enabling filtering, SLA tracking, and reporting.

---

## ComplianceWorker (Event Handler)

### Structure

```go
type ComplianceWorker struct {
    compliance *ComplianceService
    rules      JurisdictionRuleRepository
    checks     ComplianceCheckRepository
    orgLookup  OrgLookup
    orgRepo    OrgRepository  // for ListByJurisdiction
    taskSvc    task.Service
    publisher  queue.Publisher
    logger     *slog.Logger
}
```

Lives in `backend/internal/ai/compliance_worker.go`. Registered in `quorant-worker/main.go`.

### Event Subscriptions

| # | Event | Subject Pattern | Handler | Scope |
|---|---|---|---|---|
| 1 | `JurisdictionRuleCreated` | `quorant.ai.JurisdictionRuleCreated.>` | `handleRuleChange` | All orgs in jurisdiction, changed category |
| 2 | `JurisdictionRuleUpdated` | `quorant.ai.JurisdictionRuleUpdated.>` | `handleRuleChange` | All orgs in jurisdiction, changed category |
| 3 | `OrganizationUpdated` | `quorant.org.OrganizationUpdated.>` | `handleOrgChange` | Single org, all categories |
| 4 | `UnitCreated` | `quorant.org.UnitCreated.>` | `handleUnitChange` | Single org, `website_requirements` |
| 5 | `UnitDeleted` | `quorant.org.UnitDeleted.>` | `handleUnitChange` | Single org, `website_requirements` |
| 6 | `DocumentUploaded` | `quorant.doc.DocumentUploaded.>` | `handleDocumentUpload` | Single org, `reserve_study` (if reserve study doc) |
| 7 | `GoverningDocUploaded` | `quorant.doc.GoverningDocUploaded.>` | `handleGoverningDocUpload` | Single org, all categories |

### Handler: `handleRuleChange`

Triggered by jurisdiction rule create/update events.

1. Parse event payload: `{ rule_id, jurisdiction, rule_category, rule_key, effective_date, changed_by }`
2. Call `orgRepo.ListByJurisdiction(ctx, jurisdiction)` to get all affected orgs
3. For each org:
   a. Call `compliance.CheckCompliance(ctx, orgID, rule_category)`
   b. If result is `non_compliant`:
      - Insert `compliance_checks` record via `checks.Create`
      - Create task via `taskSvc.CreateTask` with task type `compliance_alert`, `resource_type = "jurisdiction_rule"`, `resource_id = rule_id`
      - Publish `ComplianceAlertRaised` event
4. Log summary: "Evaluated {n} orgs in {jurisdiction} for {category}: {compliant} compliant, {non_compliant} non-compliant"

### Handler: `handleOrgChange`

Triggered when an organization is updated (website added/removed, jurisdiction changed, settings changed).

1. Parse event payload: `{ org_id, ... }`
2. Call `compliance.EvaluateCompliance(ctx, orgID)` — full evaluation across all categories
3. For each `non_compliant` result, follow the same task creation + event publishing flow

### Handler: `handleUnitChange`

Triggered when units are created or deleted (affects unit count thresholds).

1. Parse event payload: `{ org_id, unit_id, ... }`
2. Call `compliance.CheckCompliance(ctx, orgID, "website_requirements")` — only the category affected by unit count
3. Handle non-compliant results as above

### Handler: `handleDocumentUpload`

Triggered when a document is uploaded.

1. Parse event payload: `{ org_id, document_id, content_type, title, ... }`
2. Check document metadata: skip unless `content_type` contains "reserve" or document metadata includes `{"type": "reserve_study"}`. This is a coarse filter — false positives are acceptable since the evaluator will correctly determine compliance status regardless.
3. If potentially a reserve study: call `compliance.CheckCompliance(ctx, orgID, "reserve_study")`
4. Handle non-compliant results as above

### Handler: `handleGoverningDocUpload`

Triggered when a governing document is uploaded (CC&Rs, bylaws, amendments).

1. Parse event payload: `{ org_id, document_id, doc_type, ... }`
2. Call `compliance.EvaluateCompliance(ctx, orgID)` — full evaluation since governing doc changes can affect any category
3. Handle non-compliant results as above

### Task Creation Pattern

All compliance tasks follow this pattern:

```go
task, err := w.taskSvc.CreateTask(ctx, orgID, task.CreateTaskRequest{
    TaskTypeID:   complianceAlertTaskTypeID,  // looked up once at startup
    Title:        fmt.Sprintf("Compliance alert: %s", category),
    Description:  &message,  // e.g., "Your community (150 units) requires a website per FL HB 1203"
    ResourceType: "jurisdiction_rule",
    ResourceID:   ruleID,
    Priority:     "high",
}, uuid.Nil)  // system-created, no user actor
```

### ComplianceAlertRaised Event

Published after task creation for webhook subscribers:

```json
{
  "org_id": "uuid",
  "rule_id": "uuid",
  "category": "website_requirements",
  "status": "non_compliant",
  "message": "Your community (150 units) now requires a website per FL HB 1203",
  "statute_reference": "FL HB 1203",
  "task_id": "uuid"
}
```

Subject pattern: `quorant.ai.ComplianceAlertRaised.>`

---

## ComplianceAlertJob (Daily Cron)

### Structure

```go
type ComplianceAlertJob struct {
    rules     JurisdictionRuleRepository
    orgRepo   OrgRepository
    compliance *ComplianceService
    taskSvc   task.Service
    publisher queue.Publisher
    pool      *pgxpool.Pool
    logger    *slog.Logger
}
```

Registered in scheduler: `sched.Register(complianceAlertJob, 24*time.Hour)`

### Flow

1. **Upcoming rules (advisory):**
   - Call `rules.ListUpcomingRules(ctx, 30)` — rules taking effect within 30 days
   - For each upcoming rule, call `orgRepo.ListByJurisdiction(ctx, rule.Jurisdiction)`
   - Create advisory tasks: "New {category} rule takes effect on {date}: {notes}"
   - Only create if no duplicate advisory task exists for the same rule+org (check via metadata)

2. **Newly active rules (enforcement):**
   - Query for rules where `effective_date = CURRENT_DATE` (a new method `ListRulesEffectiveToday` or a direct query, since `ListUpcomingRules` uses `> CURRENT_DATE` which excludes today)
   - For each newly active rule, trigger the same re-evaluation flow as `handleRuleChange`

### Deduplication

Advisory tasks include `metadata: {"rule_id": "<uuid>", "advisory": true}` so the job can check for existing tasks before creating duplicates.

---

## Files to Create/Modify

### New Files

| File | Purpose |
|---|---|
| `backend/migrations/20260409000026_org_jurisdiction.sql` | Add jurisdiction column to organizations, backfill, index |
| `backend/internal/ai/compliance_worker.go` | ComplianceWorker event handler struct |
| `backend/internal/ai/compliance_alert_job.go` | ComplianceAlertJob scheduler job |
| `backend/internal/ai/compliance_worker_test.go` | Worker handler tests |
| `backend/internal/ai/compliance_alert_job_test.go` | Job tests |

### Modified Files

| File | Change |
|---|---|
| `backend/internal/org/domain.go` | Add `Jurisdiction` field to Organization |
| `backend/internal/org/org_repository.go` | Add `ListByJurisdiction` to interface |
| `backend/internal/org/org_postgres.go` | Implement `ListByJurisdiction` |
| `backend/internal/ai/compliance_service.go` | Update `buildOrgComplianceContext` to use `Jurisdiction` instead of `State` |
| `backend/internal/ai/org_iface.go` | Add `ListByJurisdiction` to `OrgLookup` interface |
| `backend/cmd/quorant-worker/main.go` | Wire ComplianceWorker + ComplianceAlertJob |
| `backend/migrations/20260409000005_seed_roles.sql` | Add `compliance_alert` task type |

---

## Testing Strategy

### Unit Tests

- **ComplianceWorker handlers** — mock repos, verify correct compliance checks are run and tasks are created for each event type
- **ComplianceAlertJob** — mock repos, verify upcoming rules create advisory tasks, newly active rules trigger re-evaluation
- **Deduplication** — verify advisory tasks aren't duplicated on repeat job runs

### Integration Tests

- **Full flow:** Insert rule → publish event → verify worker creates task → verify `ComplianceAlertRaised` event published
- **Org change flow:** Update org website → publish event → verify compliance re-evaluation

---

## Verification Plan

1. Run migration, verify `jurisdiction` column exists on organizations
2. Verify backfill: all orgs with `state` now have matching `jurisdiction`
3. Start worker, create a jurisdiction rule via admin API, verify worker logs re-evaluation
4. Verify compliance tasks appear in task list for affected orgs
5. Run `ComplianceAlertJob` manually, verify upcoming rule advisory tasks created
6. Full test suite passes: `go test ./... -count=1`
