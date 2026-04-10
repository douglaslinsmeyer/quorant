# Jurisdiction Compliance Rules Engine — Design Spec

**Date:** 2026-04-09
**Issue:** [#7 — State compliance rules engine across 10+ configurable dimensions](https://github.com/douglaslinsmeyer/quorant/issues/7)
**Status:** Draft

---

## Context

HOA law changes constantly — Florida alone passed major legislation in 2022, 2023, 2024, and 2025. Quorant's AI layer already handles **interpretive** compliance (PolicyResolver + ContextRetriever with 4-level scope resolution), but there is no structured storage for **deterministic** statutory parameters — meeting notice periods, fee caps, reserve study frequencies, etc. These are facts, not inference problems.

This spec fills that gap: a platform-managed `jurisdiction_rules` table for hard parameters, a `ComplianceResolver` interface that wraps both tiers, and a compliance checking engine that evaluates orgs against their jurisdiction's requirements.

### Two-Tier Compliance Model

- **Tier 1 (this spec):** Deterministic parameters stored as structured data. Lookup, not inference. "A Florida board meeting requires 48 hours notice."
- **Tier 2 (existing AI layer):** Interpretive compliance via PolicyResolver + ContextRetriever. "Does this community's noise policy conflict with the new state statute?"

Neither tier alone is sufficient. The `ComplianceResolver` unifies both.

---

## Scope

### In Scope — 7 Enforceable Dimensions

| Category | Description |
|---|---|
| `meeting_notice` | Per-meeting-type notice periods (board, annual, special, emergency) |
| `fine_limits` | Per-violation caps, daily/aggregate maximums, hearing requirements |
| `reserve_study` | Study frequency, waiver rules, inspection types |
| `website_requirements` | Unit count thresholds, required content |
| `record_retention` | Retention periods by document type (financial, election, governing) |
| `voting_rules` | Proxy authorization, quorum percentages, electronic voting |
| `estoppel` | Turnaround times, fee caps, effective periods |

### Out of Scope — 3 Excluded Dimensions

| Dimension | Reason |
|---|---|
| Lien procedures | Workflow difference (judicial vs. non-judicial), not a parameter lookup. Becomes workflow configuration in a future spec. |
| Assessment increase limits | Depends on both state law AND governing documents. Requires AI cross-referencing (Tier 2). |
| Manager licensing | Informational, not platform-enforceable. Real-world credential verification. |

### Initial Seed Data

Top 5 states: FL, CA, TX, AZ, CO. Approximately 100-150 rows of real statutory parameters. Remaining 5 states (NV, VA, IL, NC, NJ) added in a follow-up iteration.

---

## Module Location

Extends the **AI module** (`internal/ai/`). The AI module already owns PolicyResolver, ContextRetriever, and scope resolution. Adding jurisdiction rules here keeps all policy resolution (deterministic + interpretive) colocated.

New files:
- `internal/ai/compliance.go` — ComplianceResolver interface and domain types
- `internal/ai/compliance_domain.go` — JurisdictionRule, ComplianceCheck entities
- `internal/ai/compliance_evaluators.go` — Category-specific evaluator functions
- `internal/ai/compliance_service.go` — Service orchestrating rule lookups + evaluations
- `internal/ai/jurisdiction_rule_repository.go` — Repository interface
- `internal/ai/jurisdiction_rule_postgres.go` — PostgreSQL implementation
- `internal/ai/compliance_check_repository.go` — Repository interface
- `internal/ai/compliance_check_postgres.go` — PostgreSQL implementation
- `internal/ai/handler_compliance.go` — HTTP handlers (tenant-scoped compliance endpoints)
- `internal/ai/handler_jurisdiction_admin.go` — HTTP handlers (platform admin endpoints)

---

## Data Model

### `jurisdiction_rules` Table

```sql
CREATE TABLE jurisdiction_rules (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    jurisdiction      TEXT NOT NULL,
    rule_category     TEXT NOT NULL,
    rule_key          TEXT NOT NULL,
    value_type        TEXT NOT NULL,
    value             JSONB NOT NULL,
    statute_reference TEXT NOT NULL,
    effective_date    DATE NOT NULL,
    expiration_date   DATE,
    notes             TEXT,
    source_doc_id     UUID REFERENCES governing_documents(id),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by        UUID REFERENCES users(id),
    UNIQUE (jurisdiction, rule_category, rule_key, effective_date)
);

CREATE INDEX idx_jurisdiction_rules_active
    ON jurisdiction_rules (jurisdiction, rule_category, rule_key)
    WHERE expiration_date IS NULL OR expiration_date > now();

CREATE INDEX idx_jurisdiction_rules_jurisdiction
    ON jurisdiction_rules (jurisdiction);

CREATE INDEX idx_jurisdiction_rules_upcoming
    ON jurisdiction_rules (effective_date)
    WHERE effective_date > now();
```

**Design decisions:**
- **No `org_id`** — platform-managed, not tenant-scoped. No RLS policy.
- **No `deleted_at`** — rules expire via `expiration_date`. Historical rules remain for audit.
- **`source_doc_id`** references `governing_documents` where the statute PDF lives in the context lake.
- **`created_by`** tracks which platform admin inserted the rule.
- **Unique constraint** on `(jurisdiction, rule_category, rule_key, effective_date)` prevents duplicates.
- **`value JSONB`** matches the codebase's JSONB-heavy pattern. Type safety provided at the Go layer.

**Active rule query predicate:**
```sql
WHERE effective_date <= CURRENT_DATE
  AND (expiration_date IS NULL OR expiration_date > CURRENT_DATE)
```

**Immutable update pattern:** An "update" expires the current rule (`SET expiration_date = CURRENT_DATE`) and inserts a new row with updated value and new `effective_date`. Historical record preserved.

### `compliance_checks` Table

```sql
CREATE TABLE compliance_checks (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id           UUID NOT NULL REFERENCES organizations(id),
    rule_id          UUID NOT NULL REFERENCES jurisdiction_rules(id),
    status           TEXT NOT NULL,
    details          JSONB,
    checked_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    resolved_at      TIMESTAMPTZ,
    resolution_notes TEXT
);

CREATE INDEX idx_compliance_checks_org ON compliance_checks (org_id, checked_at DESC);
CREATE INDEX idx_compliance_checks_rule ON compliance_checks (rule_id);
CREATE INDEX idx_compliance_checks_unresolved
    ON compliance_checks (org_id)
    WHERE resolved_at IS NULL AND status = 'non_compliant';
```

**Design decisions:**
- **Tenant-scoped** — has `org_id`, gets RLS policy.
- **Status values:** `compliant`, `non_compliant`, `not_applicable`, `unknown`.
- **`details JSONB`** stores context (e.g., `{"unit_count": 150, "threshold": 100, "has_website": false}`).
- **`resolved_at`** + `resolution_notes` track remediation.

### Enums

No new PostgreSQL enum types. `rule_category`, `value_type`, and compliance check `status` use TEXT with application-layer validation, consistent with the approach used for `rule_key` values which are open-ended.

### Supported `rule_category` Values

- `meeting_notice`
- `fine_limits`
- `reserve_study`
- `website_requirements`
- `record_retention`
- `voting_rules`
- `estoppel`

### Supported `value_type` Values

- `integer` — e.g., notice days, retention years, unit count thresholds
- `decimal` — e.g., quorum percentages, fee amounts
- `boolean` — e.g., hearing required, proxy allowed
- `text` — e.g., license type names, delivery methods
- `json` — e.g., complex multi-field rules serialized as JSON objects

---

## Go Interfaces

### ComplianceResolver

```go
// ComplianceResolver wraps both tiers of compliance checking.
type ComplianceResolver interface {
    // Tier 1: deterministic parameter lookup.
    GetJurisdictionRule(ctx context.Context, jurisdiction, category, key string) (*RuleValue, error)
    ListJurisdictionRules(ctx context.Context, jurisdiction, category string) ([]RuleValue, error)

    // Combined: compliance evaluation for an org.
    EvaluateCompliance(ctx context.Context, orgID uuid.UUID) (*ComplianceReport, error)
    CheckCompliance(ctx context.Context, orgID uuid.UUID, category string) (*ComplianceResult, error)
}
```

### Domain Types

```go
type JurisdictionRule struct {
    ID               uuid.UUID
    Jurisdiction     string
    RuleCategory     string
    RuleKey          string
    ValueType        string
    Value            json.RawMessage
    StatuteReference string
    EffectiveDate    time.Time
    ExpirationDate   *time.Time
    Notes            string
    SourceDocID      *uuid.UUID
    CreatedAt        time.Time
    UpdatedAt        time.Time
    CreatedBy        *uuid.UUID
}

type RuleValue struct {
    ID               uuid.UUID
    Jurisdiction     string
    Category         string
    Key              string
    ValueType        string
    Value            json.RawMessage
    StatuteReference string
    EffectiveDate    time.Time
    ExpirationDate   *time.Time
    Notes            string
    SourceDocID      *uuid.UUID
}

func (r *RuleValue) IntValue() (int, error)
func (r *RuleValue) BoolValue() (bool, error)
func (r *RuleValue) DecimalValue() (float64, error)
func (r *RuleValue) TextValue() (string, error)

type ComplianceCheck struct {
    ID              uuid.UUID
    OrgID           uuid.UUID
    RuleID          uuid.UUID
    Status          string
    Details         json.RawMessage
    CheckedAt       time.Time
    ResolvedAt      *time.Time
    ResolutionNotes string
}

type ComplianceResult struct {
    Category  string
    Status    string
    Rules     []RuleValue
    Details   map[string]any
    CheckedAt time.Time
}

type ComplianceReport struct {
    OrgID        uuid.UUID
    Jurisdiction string
    Results      []ComplianceResult
    Summary      ComplianceSummary
    CheckedAt    time.Time
}

type ComplianceSummary struct {
    Total         int
    Compliant     int
    NonCompliant  int
    NotApplicable int
    Unknown       int
}
```

### Repository Interfaces

```go
type JurisdictionRuleRepository interface {
    Create(ctx context.Context, rule *JurisdictionRule) (*JurisdictionRule, error)
    Update(ctx context.Context, rule *JurisdictionRule) (*JurisdictionRule, error)
    FindByID(ctx context.Context, id uuid.UUID) (*JurisdictionRule, error)
    GetActiveRule(ctx context.Context, jurisdiction, category, key string) (*JurisdictionRule, error)
    ListActiveRules(ctx context.Context, jurisdiction, category string) ([]JurisdictionRule, error)
    ListActiveRulesByJurisdiction(ctx context.Context, jurisdiction string) ([]JurisdictionRule, error)
    ListAllRules(ctx context.Context, jurisdiction string, limit int, afterID *uuid.UUID) ([]JurisdictionRule, bool, error)
    ListUpcomingRules(ctx context.Context, withinDays int) ([]JurisdictionRule, error)
}

type ComplianceCheckRepository interface {
    Create(ctx context.Context, check *ComplianceCheck) (*ComplianceCheck, error)
    ListByOrg(ctx context.Context, orgID uuid.UUID, limit int, afterID *uuid.UUID) ([]ComplianceCheck, bool, error)
    GetLatestByOrgAndRule(ctx context.Context, orgID, ruleID uuid.UUID) (*ComplianceCheck, error)
    Resolve(ctx context.Context, id uuid.UUID, notes string) (*ComplianceCheck, error)
}
```

---

## Category Evaluators

Each of the 7 dimensions has a registered evaluator function that knows how to determine compliance for an org:

```go
type OrgComplianceContext struct {
    OrgID                uuid.UUID
    Jurisdiction         string
    UnitCount            int
    HasWebsite           bool
    LastReserveStudyDate *time.Time    // from doc registry or fin module
    BuildingStories      int           // for Florida SIRS applicability
    ElectronicVotingEnabled bool
    ProxyVotingEnabled   bool
}


type CategoryEvaluator func(ctx context.Context, org OrgComplianceContext, rules []RuleValue) (*ComplianceResult, error)
```

Evaluators are registered at startup:

```go
resolver.RegisterEvaluator("meeting_notice", evaluateMeetingNotice)
resolver.RegisterEvaluator("fine_limits", evaluateFineLimits)
resolver.RegisterEvaluator("reserve_study", evaluateReserveStudy)
resolver.RegisterEvaluator("website_requirements", evaluateWebsiteRequirements)
resolver.RegisterEvaluator("record_retention", evaluateRecordRetention)
resolver.RegisterEvaluator("voting_rules", evaluateVotingRules)
resolver.RegisterEvaluator("estoppel", evaluateEstoppel)
```

**Example evaluator — `website_requirements`:**
1. Get rule: `required_for_unit_count` → `100`
2. If org has `unit_count < 100` → `not_applicable`
3. If org has `unit_count >= 100` and `has_website == false` → `non_compliant`
4. If org has `unit_count >= 100` and `has_website == true` → `compliant`

**Example evaluator — `meeting_notice`:**
- Meeting notice is enforced at scheduling time by the `gov` module (which calls `GetJurisdictionRule` when creating meetings). The compliance evaluator for this category always returns `compliant` — it exists to confirm that rules are present for the org's jurisdiction, not to audit past meetings. If no meeting notice rules exist for the jurisdiction, it returns `unknown` to flag a data gap.

**Example evaluator — `reserve_study`:**
1. Get rule: `required` → `true`, `interval_years` → `10`, `waiver_allowed` → `false`
2. Check org's last reserve study date (from `fin` module or document registry)
3. If no study on record and rule says required → `non_compliant`
4. If study is older than `interval_years` → `non_compliant`

---

## Event System

### New Event Types

| Event | Subject Pattern | Trigger |
|---|---|---|
| `JurisdictionRuleCreated` | `quorant.ai.JurisdictionRuleCreated.>` | Platform admin creates a rule |
| `JurisdictionRuleUpdated` | `quorant.ai.JurisdictionRuleUpdated.>` | Platform admin expires + replaces a rule |
| `JurisdictionRuleExpired` | `quorant.ai.JurisdictionRuleExpired.>` | Rule reaches expiration_date (cron) |
| `ComplianceCheckCompleted` | `quorant.ai.ComplianceCheckCompleted.>` | Compliance evaluation finishes for an org |
| `ComplianceAlertRaised` | `quorant.ai.ComplianceAlertRaised.>` | Non-compliant result detected |

### Event Payloads

**JurisdictionRuleCreated / Updated:**
```json
{
  "rule_id": "uuid",
  "jurisdiction": "FL",
  "rule_category": "website_requirements",
  "rule_key": "required_for_unit_count",
  "effective_date": "2025-01-01",
  "changed_by": "uuid"
}
```

**ComplianceAlertRaised:**
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

---

## Compliance Worker

Runs in the `quorant-worker` process. Two responsibilities:

### 1. Rule Change Re-evaluation

Subscribes to `quorant.ai.JurisdictionRuleCreated` and `quorant.ai.JurisdictionRuleUpdated`.

**Flow:**
1. Receive event with `jurisdiction` and `rule_category`
2. Query all orgs where `jurisdiction = event.jurisdiction`
3. For each org, run `CheckCompliance(ctx, orgID, event.rule_category)`
4. For each `non_compliant` result:
   - Insert `compliance_checks` record
   - Create task via task module (assigned to org's manager or board president role)
   - Publish `ComplianceAlertRaised` event

### 2. Proactive Deadline Alerts (Daily Cron)

Runs once daily via cron schedule.

**Flow:**
1. Call `ListUpcomingRules(withinDays: 30)` for rules taking effect within 30 days
2. For each upcoming rule, identify affected orgs by jurisdiction
3. Create advisory tasks: "New [category] rule takes effect on [date]: [description]"
4. Call `ListUpcomingRules(withinDays: 0)` — rules that just became active today
5. Trigger full re-evaluation for newly active rules (same flow as rule change)

---

## HTTP API

### Platform Admin — Jurisdiction Rules Management

Not tenant-scoped. Gated by `platform_admin` role.

| Method | Path | Description |
|---|---|---|
| `POST` | `/api/v1/admin/jurisdiction-rules` | Create a jurisdiction rule |
| `GET` | `/api/v1/admin/jurisdiction-rules` | List rules (query: `jurisdiction`, `category`, `include_expired`) |
| `GET` | `/api/v1/admin/jurisdiction-rules/{id}` | Get a single rule |
| `PUT` | `/api/v1/admin/jurisdiction-rules/{id}` | Update (expire old + create new) |
| `DELETE` | `/api/v1/admin/jurisdiction-rules/{id}` | Expire a rule (`expiration_date = now()`) |

**Permission:** `admin.jurisdiction_rule.manage`

### Tenant-Scoped — Compliance Status

Under org scope, for managers and board members.

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/v1/organizations/{org_id}/compliance` | Full compliance report for this org |
| `GET` | `/api/v1/organizations/{org_id}/compliance/{category}` | Single category compliance check |
| `GET` | `/api/v1/organizations/{org_id}/compliance/history` | Compliance check audit trail (paginated) |

**Permission:** `ai.compliance.read`

### Tenant-Scoped — Jurisdiction Rules Reference (Read-Only)

Any authenticated org member can see applicable rules.

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/v1/organizations/{org_id}/jurisdiction-rules` | Active rules for this org's jurisdiction |
| `GET` | `/api/v1/organizations/{org_id}/jurisdiction-rules/{category}` | Rules for a specific category |

**Permission:** `ai.jurisdiction_rule.read`

---

## Module Integration

### How `gov` and `fin` Consume Jurisdiction Rules

The `ComplianceResolver` is injected into `GovService` and `FinService` alongside the existing `PolicyResolver`.

**Gov module — violation fine enforcement:**
```go
hearingRequired, err := s.compliance.GetJurisdictionRule(ctx, "FL", "fine_limits", "hearing_required")
if hearingRequired.BoolValue() {
    // Block fine until hearing is scheduled via hearing_links
}
```

**Gov module — meeting scheduling:**
```go
noticeDays, err := s.compliance.GetJurisdictionRule(ctx, org.Jurisdiction, "meeting_notice", "board_meeting_notice_days")
minNoticeDate := time.Now().AddDate(0, 0, noticeDays.IntValue())
if scheduledAt.Before(minNoticeDate) {
    return api.NewValidationError("meeting must be scheduled at least %d days in advance", noticeDays.IntValue())
}
```

**Fin module — estoppel certificate:**
```go
turnaround, err := s.compliance.GetJurisdictionRule(ctx, org.Jurisdiction, "estoppel", "turnaround_days")
feeCap, err := s.compliance.GetJurisdictionRule(ctx, org.Jurisdiction, "estoppel", "fee_cap_cents")
```

### Wiring in `main.go`

```go
// Initialize jurisdiction rules
jurisdictionRuleRepo := ai.NewPostgresJurisdictionRuleRepository(pool)
complianceCheckRepo := ai.NewPostgresComplianceCheckRepository(pool)
complianceResolver := ai.NewComplianceResolver(jurisdictionRuleRepo, complianceCheckRepo, orgRepo, logger)

// Register category evaluators
complianceResolver.RegisterEvaluator("meeting_notice", ai.EvaluateMeetingNotice)
complianceResolver.RegisterEvaluator("fine_limits", ai.EvaluateFineLimits)
complianceResolver.RegisterEvaluator("reserve_study", ai.EvaluateReserveStudy)
complianceResolver.RegisterEvaluator("website_requirements", ai.EvaluateWebsiteRequirements)
complianceResolver.RegisterEvaluator("record_retention", ai.EvaluateRecordRetention)
complianceResolver.RegisterEvaluator("voting_rules", ai.EvaluateVotingRules)
complianceResolver.RegisterEvaluator("estoppel", ai.EvaluateEstoppel)

// Inject into domain modules
govService := gov.NewGovService(..., complianceResolver, ...)
finService := fin.NewFinService(..., complianceResolver, ...)
```

---

## Seed Data

### Top 5 States, 7 Dimensions

Approximately 100-150 rows seeded via a migration file. Values sourced from current statutes.

**Sample entries (Florida):**

| Category | Key | Value | Statute |
|---|---|---|---|
| `meeting_notice` | `board_meeting_notice_days` | `2` (48 hours) | FS 720.303(2)(c) |
| `meeting_notice` | `annual_meeting_notice_days` | `14` | FS 720.306(5) |
| `meeting_notice` | `special_meeting_notice_days` | `7` | FS 720.306(5) |
| `fine_limits` | `hearing_required` | `true` | FS 720.305(2) |
| `fine_limits` | `per_violation_cap_cents` | `null` (no statutory cap) | FS 720.305(2) |
| `fine_limits` | `daily_aggregate_cap_cents` | `10000` ($100/day) | FS 720.305(2)(b) |
| `reserve_study` | `sirs_required` | `true` | SB 4-D (2022) |
| `reserve_study` | `sirs_interval_years` | `10` | SB 4-D |
| `reserve_study` | `sirs_min_stories` | `3` | SB 4-D |
| `reserve_study` | `waiver_allowed` | `false` | SB 4-D |
| `website_requirements` | `required_for_unit_count` | `100` | FL HB 1203 (2025) |
| `record_retention` | `financial_records_years` | `7` | FS 720.303(4) |
| `record_retention` | `governing_docs_retention` | `"permanent"` | FS 720.303(4) |
| `voting_rules` | `proxy_allowed` | `true` | FS 720.306(8) |
| `estoppel` | `turnaround_business_days` | `10` | FS 720.30851 |
| `estoppel` | `fee_cap_cents` | `25000` ($250) | FS 720.30851 |

Similar entries for CA, TX, AZ, CO.

---

## Testing Strategy

### Unit Tests

- **Repository tests** (`jurisdiction_rule_postgres_test.go`, `compliance_check_postgres_test.go`):
  - CRUD operations
  - Active rule temporal filtering (past, current, future effective dates)
  - Cursor-based pagination
  - Unique constraint enforcement

- **Service tests** (`compliance_service_test.go`):
  - Rule lookup by jurisdiction + category + key
  - Compliance evaluation dispatching to correct evaluator
  - Report aggregation

- **Evaluator tests** (`compliance_evaluators_test.go`):
  - BDD-style per category:
    - Given a Florida HOA with 150 units, no website → non_compliant for website_requirements
    - Given a California HOA → not_applicable for website_requirements
    - Given a Florida HOA with hearing_required=true and no hearing → blocked fine
  - Edge cases: no rules for jurisdiction, expired rules, future-dated rules

### Integration Tests

- Full stack: seed rules → create org → evaluate compliance → verify report
- Event flow: create rule → verify NATS event → verify worker creates tasks
- Temporal: insert future-dated rule → verify not active → advance effective date → verify activation
- Admin API: create, list, update (expire+replace), expire via DELETE

### Handler Tests

- Admin endpoints: permission gating, request validation, CRUD responses
- Tenant endpoints: compliance report format, pagination, permission checks
- Error cases: invalid jurisdiction, unknown category, rule not found

---

## Files to Create/Modify

### New Files

| File | Purpose |
|---|---|
| `backend/migrations/20260409000025_jurisdiction_rules.sql` | Schema: jurisdiction_rules + compliance_checks + indexes + RLS |
| `backend/internal/ai/compliance.go` | ComplianceResolver interface |
| `backend/internal/ai/compliance_domain.go` | JurisdictionRule, ComplianceCheck, RuleValue, ComplianceReport types |
| `backend/internal/ai/compliance_evaluators.go` | 7 CategoryEvaluator functions |
| `backend/internal/ai/compliance_service.go` | Service orchestrating lookups + evaluations |
| `backend/internal/ai/jurisdiction_rule_repository.go` | JurisdictionRuleRepository interface |
| `backend/internal/ai/jurisdiction_rule_postgres.go` | PostgreSQL implementation |
| `backend/internal/ai/compliance_check_repository.go` | ComplianceCheckRepository interface |
| `backend/internal/ai/compliance_check_postgres.go` | PostgreSQL implementation |
| `backend/internal/ai/handler_compliance.go` | Tenant-scoped compliance HTTP handlers |
| `backend/internal/ai/handler_jurisdiction_admin.go` | Platform admin jurisdiction rules HTTP handlers |
| `backend/internal/ai/compliance_requests.go` | Request/response DTOs |
| `backend/seeds/jurisdiction_rules_seed.sql` | Seed data for FL, CA, TX, AZ, CO |
| `backend/internal/ai/jurisdiction_rule_postgres_test.go` | Repository integration tests |
| `backend/internal/ai/compliance_check_postgres_test.go` | Repository integration tests |
| `backend/internal/ai/compliance_service_test.go` | Service unit tests |
| `backend/internal/ai/compliance_evaluators_test.go` | Evaluator BDD tests |
| `backend/internal/ai/handler_compliance_test.go` | Handler tests |

### Modified Files

| File | Change |
|---|---|
| `backend/internal/ai/routes.go` | Register compliance + admin jurisdiction rule routes |
| `backend/internal/gov/service.go` | Accept and use ComplianceResolver for fine/meeting enforcement |
| `backend/internal/fin/service.go` | Accept and use ComplianceResolver for estoppel/assessment rules |
| `backend/cmd/quorant-api/main.go` | Wire ComplianceResolver, register evaluators, inject into gov/fin |
| `backend/cmd/quorant-worker/main.go` | Register compliance worker event handlers + daily cron |
| `backend/migrations/20260409000002_enums.sql` | (No change — using TEXT, not enums) |
| `backend/migrations/20260409000005_seed_roles.sql` | Add `admin.jurisdiction_rule.manage`, `ai.compliance.read`, `ai.jurisdiction_rule.read` permissions |

---

## Verification Plan

1. **Schema:** Run migration, verify tables exist with correct constraints and indexes
2. **Seed data:** Load seed SQL, verify correct row counts per jurisdiction
3. **Admin API:** Use curl/httpie to create, list, update, expire rules
4. **Compliance check:** Create a test org in FL with 150 units, hit `/compliance` endpoint, verify non-compliant for website_requirements
5. **Temporal rules:** Insert a future-dated rule, verify it doesn't appear in active lookups, manually advance date logic, verify it activates
6. **Event flow:** Create a rule, verify NATS event published, verify worker creates compliance tasks
7. **Tests:** Run `go test ./internal/ai/... -v` for unit tests, `go test -tags=integration ./internal/ai/...` for integration tests
