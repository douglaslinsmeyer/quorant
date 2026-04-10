# Estoppel Certificate Generation — Design Spec

**Date:** 2026-04-09
**Issue:** [#4 — Estoppel certificate generation with state-aware templates and 30+ data points](https://github.com/douglaslinsmeyer/quorant/issues/4)
**Status:** Draft
**Module:** `backend/internal/estoppel/`

---

## Context

Property transfers in HOA communities require estoppel certificates — legally binding financial snapshots that disclose the unit's assessment obligations, delinquencies, violations, and governance status. Title companies and closing agents order these during property closings. Today this is a manual process: an operator queries 6+ screens, transcribes 30+ data points into a template, and ships a PDF. Industry average turnaround is 12.6 days.

Quorant's AI context lake architecture enables a fundamentally different approach: the system aggregates all data points in one pass, AI resolves ambiguous narrative fields with citations from governing documents, and the manager reviews a pre-filled draft. Target: minutes, not days. This is also a revenue-generating compliance requirement — estoppel fees are charged to the requestor.

**Dependencies:**
- **Hard:** GL/fin module (assessment balances, payment history, delinquency data)
- **Soft:** AI policy engine (jurisdiction-scoped rules; can be seeded as structured data initially)

---

## Key Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Module location | New `estoppel/` module | Cross-module aggregation justifies dedicated module; clean separation |
| PDF library | Maroto v2 (pure Go) | Grid-based layout, MIT licensed, no external binaries, good table support |
| Data aggregation | Narrow read interfaces | Estoppel defines what it needs; source modules provide adapters |
| Licensing | Independently licensed feature | Gated by `EntitlementChecker` middleware; customers opt in |
| Requestors | Both external (title companies) + homeowner self-service | Phase 1: manager submits on behalf of external requestors |
| Signing | Manager approval stamp | Name/title/date on PDF; no cryptographic signing in Phase 1 |
| Document scope | Estoppel certificates + lender questionnaires | Same module, different templates, shared workflow |
| State rules | Seeded via PolicyResolver | Jurisdiction-scoped policy extractions for 5 states |
| AI narrative | Yes, for ambiguous fields | Context lake resolves judgment calls with citations; manager reviews |

---

## 1. Module Structure

```
backend/internal/estoppel/
├── domain.go              # EstoppelRequest, EstoppelCertificate, LenderQuestionnaire
├── domain_test.go         # Validation, fee calculation, status transition tests
├── providers.go           # FinancialDataProvider, ComplianceDataProvider, PropertyDataProvider
├── repository.go          # EstoppelRepository interface
├── postgres.go            # SQL implementation
├── postgres_test.go       # Integration tests (//go:build integration)
├── service.go             # EstoppelService orchestration
├── service_test.go        # Unit tests with mock providers
├── generator.go           # CertificateGenerator — Maroto v2 PDF rendering
├── generator_test.go      # Template output tests
├── narrative.go           # NarrativeGenerator — AI narrative resolution
├── narrative_test.go      # Narrative generation tests
├── handler.go             # HTTP endpoints
├── handler_test.go        # HTTP-level tests
├── requests.go            # Request/response DTOs + validation
├── routes.go              # Route registration with entitlement gate
└── templates.go           # State-specific template builder registry
```

---

## 2. Domain Model

### EstoppelRequest

The intake record tracking the request lifecycle.

```go
type EstoppelRequest struct {
    ID                       uuid.UUID
    OrgID                    uuid.UUID
    UnitID                   uuid.UUID
    TaskID                   uuid.UUID          // FK to tasks table
    RequestType              string             // estoppel_certificate | lender_questionnaire
    RequestorType            string             // homeowner | title_company | closing_agent | attorney
    RequestorName            string
    RequestorEmail           string
    RequestorPhone           string
    RequestorCompany         string
    PropertyAddress          string
    OwnerName                string
    ClosingDate              *time.Time
    RushRequested            bool
    Status                   string             // submitted → data_aggregation → manager_review → approved → generating → delivered → cancelled
    FeeCents                 int64
    RushFeeCents             int64
    DelinquentSurchargeCents int64
    TotalFeeCents            int64
    DeadlineAt               time.Time
    AssignedTo               *uuid.UUID
    AmendmentOf              *uuid.UUID         // references original certificate if this is a correction
    Metadata                 map[string]any
    CreatedBy                uuid.UUID
    CreatedAt                time.Time
    UpdatedAt                time.Time
    DeletedAt                *time.Time
}
```

**Valid status transitions:**
```
submitted → data_aggregation → manager_review → approved → generating → delivered
submitted → cancelled
manager_review → cancelled (rejected)
```

### EstoppelCertificate

The generated output — a point-in-time legal document.

```go
type EstoppelCertificate struct {
    ID                uuid.UUID
    RequestID         uuid.UUID
    OrgID             uuid.UUID
    UnitID            uuid.UUID
    DocumentID        uuid.UUID          // FK to documents table (stored PDF)
    Jurisdiction      string             // state code (e.g., "FL", "CA")
    EffectiveDate     time.Time
    ExpiresAt         *time.Time         // computed from state rules
    DataSnapshot      json.RawMessage    // frozen aggregated data at generation time
    NarrativeSections json.RawMessage    // AI-generated + manager-edited narratives
    SignedBy          uuid.UUID
    SignedAt          time.Time
    SignerTitle       string
    TemplateVersion   string
    AmendmentOf       *uuid.UUID         // references prior certificate if correction
    CreatedAt         time.Time
}
```

The `DataSnapshot` is critical — it freezes the financial/compliance state so the certificate is a point-in-time legal document, not a live query.

---

## 3. Data Provider Interfaces

Three interfaces defined in the estoppel module. Source modules (fin, gov, org) provide adapter implementations.

### FinancialDataProvider

```go
type FinancialDataProvider interface {
    GetUnitFinancialSnapshot(ctx context.Context, orgID, unitID uuid.UUID) (*FinancialSnapshot, error)
}

type FinancialSnapshot struct {
    // Assessment Information
    RegularAssessmentCents   int64
    AssessmentFrequency      string              // monthly, quarterly, annually, semi_annually
    PaidThroughDate          time.Time
    NextInstallmentDueDate   time.Time
    SpecialAssessments       []SpecialAssessment  // current + future installments
    CapitalContributionCents int64
    TransferFeeCents         int64
    ReserveFundFeeCents      int64

    // Balance & Delinquency
    CurrentBalanceCents      int64
    PastDueItems             []PastDueItem        // itemized by category
    LateFeesCents            int64
    InterestCents            int64
    CollectionCostsCents     int64
    TotalDelinquentCents     int64

    // Collection Status
    HasActiveCollection      bool
    CollectionStatus         string
    AttorneyName             string
    AttorneyContact          string
    LienStatus               string

    // Payment Plan
    HasPaymentPlan           bool
    PaymentPlanDetails       *PaymentPlanInfo

    // Lender Questionnaire Fields (org-level, not unit-level)
    DelinquencyRate60Days    float64              // % of units 60+ days past due
    ReserveBalanceCents      int64
    ReserveTargetCents       int64
    BudgetStatus             string
    TotalUnits               int
    OwnerOccupiedCount       int
    RentalCount              int
}

type SpecialAssessment struct {
    Description         string
    TotalAmountCents    int64
    RemainingCents      int64
    InstallmentCents    int64
    NextDueDate         time.Time
}

type PastDueItem struct {
    Category     string   // assessment, late_fee, interest, collection_cost, fine
    AmountCents  int64
    AsOfDate     time.Time
}

type PaymentPlanInfo struct {
    TotalOwedCents      int64
    InstallmentCents    int64
    Frequency           string
    InstallmentsTotal   int
    InstallmentsPaid    int
    NextDueDate         time.Time
    Status              string
}
```

### ComplianceDataProvider

```go
type ComplianceDataProvider interface {
    GetUnitComplianceSnapshot(ctx context.Context, orgID, unitID uuid.UUID) (*ComplianceSnapshot, error)
}

type ComplianceSnapshot struct {
    // Violations
    OpenViolations           []ViolationSummary
    PendingFinesCents        int64

    // Legal & Governance
    PendingLitigation        []LitigationSummary
    RightOfFirstRefusal      bool
    BoardApprovalRequired    bool
    RentalRestrictions       string

    // Insurance
    InsuranceCoverage        []InsuranceSummary

    // Lender Questionnaire Fields
    StructuralIntegrityStatus string
    CommercialSpacePercent    float64
}

type ViolationSummary struct {
    Category     string
    Description  string
    Status       string
    FineCents    int64
    CureDeadline *time.Time
}

type LitigationSummary struct {
    Description  string
    Status       string
    FiledDate    time.Time
}

type InsuranceSummary struct {
    CoverageType string
    Provider     string
    PolicyNumber string
    ExpiresAt    time.Time
    CoverageAmountCents int64
}
```

### PropertyDataProvider

```go
type PropertyDataProvider interface {
    GetPropertySnapshot(ctx context.Context, orgID, unitID uuid.UUID) (*PropertySnapshot, error)
}

type PropertySnapshot struct {
    OrgName              string
    OrgAddress           string
    OrgState             string    // jurisdiction for template selection
    UnitLabel            string
    UnitAddress          string
    ParcelNumber         string
    CurrentOwners        []OwnerInfo
    ManagingFirmName     string
    ManagingFirmContact  string
}

type OwnerInfo struct {
    Name      string
    Email     string
    Phone     string
    StartedAt time.Time
}
```

### AggregatedData (Merged Result)

```go
type AggregatedData struct {
    Property   PropertySnapshot
    Financial  FinancialSnapshot
    Compliance ComplianceSnapshot
    Narratives *NarrativeSections  // nil until AI resolution; populated before manager_review
    AsOfTime   time.Time           // timestamp when data was collected
}
```

### EstoppelRules (Deserialized Policy Config)

```go
type EstoppelRules struct {
    StandardTurnaroundBusinessDays int     `json:"standard_turnaround_business_days"`
    StandardFeeCents               int64   `json:"standard_fee_cents"`
    RushTurnaroundBusinessDays     *int    `json:"rush_turnaround_business_days"`
    RushFeeCents                   int64   `json:"rush_fee_cents"`
    DelinquentSurchargeCents       int64   `json:"delinquent_surcharge_cents"`
    EffectivePeriodDays            *int    `json:"effective_period_days"`
    ElectronicDeliveryRequired     bool    `json:"electronic_delivery_required"`
    StatutoryFormRequired          bool    `json:"statutory_form_required"`
    StatutoryFormID                string  `json:"statutory_form_id"`
    FreeAmendmentOnError           bool    `json:"free_amendment_on_error"`
    StatuteRef                     string  `json:"statute_ref"`
    RequiredAttachments            []string `json:"required_attachments"`
    // State-specific fields handled via Metadata map if needed
}
```

### Aggregation Flow

```
Request created
    ├─ goroutine 1: FinancialDataProvider.GetUnitFinancialSnapshot()
    ├─ goroutine 2: ComplianceDataProvider.GetUnitComplianceSnapshot()
    └─ goroutine 3: PropertyDataProvider.GetPropertySnapshot()
    │
    └── errgroup.Wait()
         │
         ├── Merge into AggregatedData struct (AsOfTime = now)
         ├── NarrativeGenerator.GenerateNarratives() — AI resolves ambiguous fields
         └── Advance to manager_review stage
```

All three provider calls execute in parallel via `errgroup`. If any provider fails, the request stays in `data_aggregation` status with an error logged, and retries on next attempt.

---

## 4. Request Workflow

### Task Type Definition

Seeded as a system task type (org_id IS NULL):

```json
{
  "key": "estoppel_request",
  "name": "Estoppel Certificate Request",
  "default_priority": "high",
  "sla_hours": 80,
  "auto_assign_role": "hoa_manager",
  "source_module": "estoppel",
  "workflow_stages": [
    "submitted",
    "data_aggregation",
    "manager_review",
    "approved",
    "generating",
    "delivered"
  ],
  "checklist_template": [
    {"id": "verify_owner", "label": "Verify current owner matches records"},
    {"id": "verify_balance", "label": "Verify account balance is current"},
    {"id": "review_violations", "label": "Review open violations"},
    {"id": "review_narrative", "label": "Review AI-generated narrative sections"},
    {"id": "confirm_fee", "label": "Confirm fee calculation"},
    {"id": "approve_sign", "label": "Approve and sign certificate"}
  ]
}
```

### Intake Flow

**Two paths, same downstream workflow:**

1. **Homeowner self-service:** Authenticated via JWT. `RequestorType = homeowner`, pre-populated from profile.
2. **External requestor (Phase 1):** Manager submits on behalf of title company/closing agent, entering requestor details manually.
   - **Phase 2 (future):** API key per organization for direct title company submission.

### Workflow Stages

| Stage | Trigger | Actions |
|-------|---------|---------|
| `submitted` | Request created | Fee calculated from state rules. Task created, auto-assigned to `hoa_manager`. SLA deadline set based on rush preference + state turnaround rules. |
| `data_aggregation` | Automatic (immediate) | Three providers queried in parallel. AI generates narrative sections. Draft assembled. |
| `manager_review` | Data aggregation complete | Manager sees pre-filled certificate with checklist. Reviews data, AI narratives. Can edit narrative fields. Can preview draft PDF (watermarked DRAFT). |
| `approved` | Manager completes checklist + clicks "Approve & Sign" | Manager's name/title/timestamp recorded. Certificate data frozen. |
| `generating` | Automatic (immediate after approval) | PDF generated via Maroto v2. Uploaded to doc module (S3). DocumentID linked to certificate. |
| `delivered` | PDF ready | Download link available in portal. Communication log entry created. Webhook fired. Requestor notified via email. |

### Fee Calculation

```
total_fee = standard_fee
  + (rush_fee if rush_requested)
  + (delinquent_surcharge if unit has past-due balance > 0)
```

Capped at state maximum. Florida: free amendment if original contained an error.

### SLA Tracking

- Standard turnaround: per state rules (typically 10 business days)
- Rush turnaround: per state rules (typically 3 business days)
- `Task.SLADeadline` computed from state rules at request creation
- Existing `TaskSLABreached` event triggers escalation notifications

---

## 5. State Compliance via PolicyResolver

State-specific rules stored as jurisdiction-scoped policy extractions using the existing AI policy engine.

**Policy key:** `estoppel_rules`

### Seed Data (5 States)

**Florida** (§720.30851 / §718.116(8)):
```json
{
  "standard_turnaround_business_days": 10,
  "standard_fee_cents": 29900,
  "rush_turnaround_business_days": 3,
  "rush_fee_cents": 11900,
  "delinquent_surcharge_cents": 17900,
  "effective_period_days": 30,
  "electronic_delivery_required": true,
  "statutory_form_required": true,
  "statutory_form_id": "fl_720_30851",
  "free_amendment_on_error": true,
  "statutory_questions": 19,
  "statute_ref": "§720.30851/§718.116(8)"
}
```

**California** (Civil Code §4525–4530):
```json
{
  "standard_turnaround_business_days": 10,
  "standard_fee_cents": 0,
  "fee_cap_type": "reasonable",
  "rush_turnaround_business_days": null,
  "effective_period_days": null,
  "electronic_delivery_required": false,
  "statutory_form_required": true,
  "statutory_form_id": "ca_4528",
  "required_attachments": ["governing_docs", "ccrs", "bylaws", "rules", "budget", "reserve_study_summary"],
  "statute_ref": "Civil Code §4525–4530"
}
```

**Texas** (Property Code §207.003):
```json
{
  "standard_turnaround_business_days": 10,
  "standard_fee_cents": 37500,
  "update_fee_cents": 7500,
  "effective_period_days": 60,
  "noncompliance_damages_cents": 500000,
  "statute_ref": "Property Code §207.003"
}
```

**Nevada** (NRS 116.4109):
```json
{
  "standard_turnaround_business_days": 10,
  "standard_fee_cents": 18500,
  "fee_cpi_adjusted": true,
  "cpi_cap_percent": 3,
  "rush_fee_cents": 10000,
  "effective_period_days": 90,
  "electronic_delivery_required": true,
  "statute_ref": "NRS 116.4109"
}
```

**Virginia** (§55.1-1808):
```json
{
  "standard_turnaround_business_days": 14,
  "fee_cpi_adjusted": true,
  "electronic_delivery_required": true,
  "buyer_rescission_days": 3,
  "cic_board_registration_required": true,
  "statute_ref": "§55.1-1808"
}
```

### Resolution Flow

1. `EstoppelService.CreateRequest()` calls `PolicyResolver.GetPolicy(ctx, orgID, "estoppel_rules")`
2. PolicyResolver resolves org's jurisdiction (via org's `state` field) and returns the matching extraction
3. Service uses the config to compute fees, deadlines, and select the correct template
4. If no state-specific rules found, service returns an error — estoppel generation requires known jurisdiction rules

### Template Selection

Deterministic: `jurisdiction → statutory_form_id → template builder function`.

States with statutory forms get their prescribed layout. States without get a generic professional template. Lender questionnaires use a shared template regardless of state.

---

## 6. PDF Generation (Maroto v2)

### Generator Interface

```go
type CertificateGenerator interface {
    GenerateEstoppel(data *AggregatedData, rules *EstoppelRules) ([]byte, error)
    GenerateLenderQuestionnaire(data *AggregatedData, rules *EstoppelRules) ([]byte, error)
}
```

The generator is a pure function: data in, PDF bytes out. No database or service dependencies.

### Template Registry

```go
var templateRegistry = map[string]TemplateBuilder{
    "fl_720_30851": buildFloridaEstoppel,
    "ca_4528":      buildCaliforniaEstoppel,
    "tx_207":       buildTexasEstoppel,
    "nv_116":       buildNevadaEstoppel,
    "va_55_1":      buildVirginiaEstoppel,
    "generic":      buildGenericEstoppel,
    "lender":       buildLenderQuestionnaire,
}

type TemplateBuilder func(m maroto.Maroto, data *AggregatedData, rules *EstoppelRules)
```

### Common Document Structure

All estoppel templates share this section order (state-specific templates may reorder or add sections):

1. **Header** — HOA name, address, logo (if uploaded), document title, date
2. **Property section** — address, parcel number, unit label, current owner(s)
3. **Assessment section** — regular assessment amount/frequency, paid-through date, special assessments, capital contributions, transfer fees
4. **Balance section** — current balance, past-due itemization, late fees, interest, collection costs
5. **Collection section** — active collection case status, attorney info, lien status, payment plan
6. **Violations section** — open violations with descriptions and fines
7. **Legal/governance section** — pending litigation, right of first refusal, board approval, rental restrictions, insurance summary
8. **AI narrative sections** — ambiguous fields with AI-generated text and source citations
9. **Fee breakdown** — preparation fee, rush fee, surcharges, total
10. **Signature block** — signer name, title, date, certification statement
11. **Footer** — effective date, expiration date, statute reference, page numbers

### State-Specific Variations

| State | Variation |
|-------|-----------|
| Florida | 19 statutory questions in prescribed order per §720.30851 |
| California | §4528 standardized form + required document attachments list |
| Texas | 60-day validity notice, §207.003 damages disclosure |
| Nevada | CPI adjustment notice, electronic format declaration |
| Virginia | 3-day buyer rescission notice, CIC Board registration number |

### Typography

Embed open-source TTF fonts:
- **Serif** (e.g., Liberation Serif) for body text — professional legal document appearance
- **Sans-serif** (e.g., Liberation Sans) for section headers

### Lender Questionnaire Template

Separate template covering org-level data:
- Total units, owner-occupied vs. rental percentage
- Delinquency rates (60+ days)
- Reserve adequacy (balance vs. target)
- Structural integrity status
- Commercial space percentage
- Budget status, pending special assessments

---

## 7. AI Narrative Generation

### Ambiguous Fields

| Field | Why AI is needed |
|-------|-----------------|
| Pending special assessments | Board may have discussed but not voted; AI checks meeting minutes |
| Pending litigation disclosure | "Material" is a judgment call; AI evaluates from filings in context lake |
| Insurance coverage adequacy | Summarizes complex policy details into disclosure language |
| Rental restrictions | May span CC&Rs, amendments, and rule changes |
| Right of first refusal | May exist in CC&Rs but never exercised |
| Board approval requirements | Transfer approval requirements vary by governing docs |

### NarrativeGenerator Interface

```go
type NarrativeGenerator interface {
    GenerateNarratives(ctx context.Context, orgID uuid.UUID, data *AggregatedData) (*NarrativeSections, error)
}

type NarrativeSections struct {
    PendingSpecialAssessments  NarrativeField
    LitigationDisclosure       NarrativeField
    InsuranceSummary           NarrativeField
    RentalRestrictions         NarrativeField
    RightOfFirstRefusal        NarrativeField
    BoardApprovalRequirements  NarrativeField
}

type NarrativeField struct {
    Text       string       // AI-generated narrative
    Citations  []Citation   // Source references (doc name, section, page)
    Confidence float64      // AI confidence score (0.0–1.0)
    Editable   bool         // Always true — manager can override
}

type Citation struct {
    DocumentName string
    Section      string
    PageNumber   int
}
```

### Resolution Flow

1. For each ambiguous field, call `PolicyResolver.QueryPolicy()` with a targeted question and structured context
2. Policy engine searches the context lake (CC&Rs, meeting minutes, board resolutions) for relevant chunks
3. Returns narrative text with citations and confidence score
4. If confidence < 0.80, field flagged for mandatory manager attention (highlighted in review)
5. Manager can accept, edit, or replace any narrative before signing

### Fallback

If the AI module is unavailable or the org has no indexed governing documents, narrative fields are left blank with a prompt: "Manual entry required — no governing documents indexed." The certificate remains valid; the manager fills in manually.

---

## 8. API Endpoints & Permissions

### Entitlement Gate

All estoppel routes wrapped with `RequireEntitlement(ec, "estoppel")`. Organizations without the entitlement receive `403 Forbidden`.

### Endpoints

| Method | Path | Permission | Description |
|--------|------|-----------|-------------|
| `POST` | `/api/v1/organizations/{org_id}/estoppel/requests` | `estoppel.request.create` | Submit new request |
| `GET` | `/api/v1/organizations/{org_id}/estoppel/requests` | `estoppel.request.list` | List requests (filterable by status) |
| `GET` | `/api/v1/organizations/{org_id}/estoppel/requests/{id}` | `estoppel.request.read` | Request details + current stage |
| `POST` | `/api/v1/organizations/{org_id}/estoppel/requests/{id}/approve` | `estoppel.request.approve` | Manager approves + signs |
| `POST` | `/api/v1/organizations/{org_id}/estoppel/requests/{id}/reject` | `estoppel.request.approve` | Manager rejects with reason |
| `PATCH` | `/api/v1/organizations/{org_id}/estoppel/requests/{id}/narratives` | `estoppel.request.approve` | Edit AI narratives before signing |
| `GET` | `/api/v1/organizations/{org_id}/estoppel/requests/{id}/preview` | `estoppel.request.approve` | Preview draft PDF (watermarked DRAFT) |
| `GET` | `/api/v1/organizations/{org_id}/estoppel/certificates/{id}/download` | `estoppel.certificate.download` | Presigned S3 download URL |
| `POST` | `/api/v1/organizations/{org_id}/estoppel/certificates/{id}/amend` | `estoppel.request.approve` | Create amendment referencing original |

### Permission Matrix

| Role | create | list | read | approve | download |
|------|--------|------|------|---------|----------|
| homeowner | own unit | own requests | own requests | — | own certs |
| hoa_manager | yes | yes | yes | yes | yes |
| board_president | yes | yes | yes | yes | yes |
| board_member | — | yes | yes | — | yes |
| firm_admin | yes | yes | yes | yes | yes |
| firm_staff | yes | yes | yes | yes | yes |

### External Requestor Authentication

- **Phase 1:** Manager submits on behalf of external requestors, entering their contact details. Requestor receives download link via email.
- **Phase 2 (future):** Per-org API keys for title company direct submission.

---

## 9. Events & Audit Trail

### Domain Events

| Event | Trigger | Consumers |
|-------|---------|-----------|
| `estoppel.request.created` | Request submitted | `com`: notify manager, `task`: SLA tracking |
| `estoppel.data.aggregated` | Aggregation complete | Internal: advance workflow stage |
| `estoppel.request.approved` | Manager signs off | Internal: trigger PDF generation |
| `estoppel.request.rejected` | Manager rejects | `com`: notify requestor with reason |
| `estoppel.certificate.generated` | PDF uploaded to S3 | `com`: notify requestor, `webhook`: fire subscriptions |
| `estoppel.certificate.delivered` | Download confirmed / email sent | `audit`: delivery trail |
| `estoppel.certificate.amended` | Amendment created | `com`: notify original requestor |
| `estoppel.sla.breached` | Deadline passed without delivery | `com`: escalation to manager + firm admin |

### Audit Entries

Every state change recorded via `audit.Auditor`:

- `estoppel.request.created` — who requested, for which unit
- `estoppel.data.aggregated` — snapshot of data sources queried
- `estoppel.narrative.edited` — before/after of manager edits to AI narratives
- `estoppel.request.approved` — who signed, when
- `estoppel.certificate.generated` — document ID, template version
- `estoppel.certificate.downloaded` — who downloaded, when (compliance trail)

### Communication Log

Each delivery action creates a `CommunicationLog` entry:
- Direction: `outbound`
- Channel: `email` or `portal`
- ResourceType: `estoppel_certificate`, ResourceID: certificate UUID
- Status: tracks `sent → delivered → opened`

### Amendment Workflow

1. `POST .../certificates/{id}/amend` creates a new `EstoppelRequest` with `amendment_of = original_certificate_id`
2. Full workflow: data re-aggregation → review → generation
3. New certificate includes correction notice referencing original
4. Florida: free of charge per statute. Other states: per fee policy.

---

## 10. Database Schema

### Tables

```sql
-- New migration: NNNN_estoppel_tables.sql

CREATE TABLE estoppel_requests (
    id                         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id                     UUID NOT NULL REFERENCES organizations(id),
    unit_id                    UUID NOT NULL REFERENCES units(id),
    task_id                    UUID REFERENCES tasks(id),
    request_type               TEXT NOT NULL CHECK (request_type IN ('estoppel_certificate', 'lender_questionnaire')),
    requestor_type             TEXT NOT NULL CHECK (requestor_type IN ('homeowner', 'title_company', 'closing_agent', 'attorney')),
    requestor_name             TEXT NOT NULL,
    requestor_email            TEXT NOT NULL,
    requestor_phone            TEXT,
    requestor_company          TEXT,
    property_address           TEXT NOT NULL,
    owner_name                 TEXT NOT NULL,
    closing_date               DATE,
    rush_requested             BOOLEAN NOT NULL DEFAULT false,
    status                     TEXT NOT NULL DEFAULT 'submitted',
    fee_cents                  INTEGER NOT NULL DEFAULT 0,
    rush_fee_cents             INTEGER NOT NULL DEFAULT 0,
    delinquent_surcharge_cents INTEGER NOT NULL DEFAULT 0,
    total_fee_cents            INTEGER NOT NULL DEFAULT 0,
    deadline_at                TIMESTAMPTZ NOT NULL,
    assigned_to                UUID REFERENCES users(id),
    amendment_of               UUID,  -- FK added via ALTER TABLE after estoppel_certificates exists
    metadata                   JSONB DEFAULT '{}',
    created_by                 UUID NOT NULL REFERENCES users(id),
    created_at                 TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                 TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at                 TIMESTAMPTZ
);

CREATE TABLE estoppel_certificates (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    request_id         UUID NOT NULL REFERENCES estoppel_requests(id),
    org_id             UUID NOT NULL REFERENCES organizations(id),
    unit_id            UUID NOT NULL REFERENCES units(id),
    document_id        UUID NOT NULL REFERENCES documents(id),
    jurisdiction       TEXT NOT NULL,
    effective_date     DATE NOT NULL,
    expires_at         DATE,
    data_snapshot      JSONB NOT NULL,
    narrative_sections JSONB NOT NULL DEFAULT '{}',
    signed_by          UUID NOT NULL REFERENCES users(id),
    signed_at          TIMESTAMPTZ NOT NULL,
    signer_title       TEXT NOT NULL,
    template_version   TEXT NOT NULL,
    amendment_of       UUID REFERENCES estoppel_certificates(id),
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

-- Deferred FK for cross-table amendment reference
ALTER TABLE estoppel_requests
    ADD CONSTRAINT fk_estoppel_requests_amendment
    FOREIGN KEY (amendment_of) REFERENCES estoppel_certificates(id);
```

### Indexes

```sql
CREATE INDEX idx_estoppel_requests_org ON estoppel_requests(org_id, status) WHERE deleted_at IS NULL;
CREATE INDEX idx_estoppel_requests_unit ON estoppel_requests(unit_id, created_at DESC);
CREATE INDEX idx_estoppel_requests_deadline ON estoppel_requests(deadline_at) WHERE status NOT IN ('delivered', 'cancelled');
CREATE INDEX idx_estoppel_certificates_org ON estoppel_certificates(org_id);
CREATE INDEX idx_estoppel_certificates_unit ON estoppel_certificates(unit_id, effective_date DESC);
CREATE INDEX idx_estoppel_certificates_request ON estoppel_certificates(request_id);
```

### RLS Policies

```sql
ALTER TABLE estoppel_requests ENABLE ROW LEVEL SECURITY;
ALTER TABLE estoppel_requests FORCE ROW LEVEL SECURITY;
CREATE POLICY estoppel_requests_org ON estoppel_requests
    USING (org_id = current_setting('app.current_org_id', true)::uuid);

ALTER TABLE estoppel_certificates ENABLE ROW LEVEL SECURITY;
ALTER TABLE estoppel_certificates FORCE ROW LEVEL SECURITY;
CREATE POLICY estoppel_certificates_org ON estoppel_certificates
    USING (org_id = current_setting('app.current_org_id', true)::uuid);
```

---

## 11. Testing Strategy

| Layer | Type | File | What's Tested |
|-------|------|------|--------------|
| Domain | Unit | `domain_test.go` | Request validation, fee calculation, status transitions, deadline computation |
| Providers | Unit | `providers_test.go` | Mock repos → snapshot assembly, edge cases (no owner, no violations, zero balance) |
| Generator | Unit | `generator_test.go` | PDF output per state template — valid PDF bytes, correct page count, contains expected text strings |
| Narrative | Unit | `narrative_test.go` | AI narrative resolution with mock policy resolver, fallback behavior |
| Service | Unit | `service_test.go` | Full orchestration with mock providers + mock generator + mock repos. Workflow transitions, error handling, amendment flow |
| Repository | Integration | `postgres_test.go` | CRUD with real Postgres (testcontainers), RLS enforcement, index usage, cascade behavior |
| Handler | Unit | `handler_test.go` | HTTP status codes, request validation, permission checks, response shapes |
| E2E | Integration | `e2e_test.go` | Full request → aggregation → generation → delivery flow with real DB + mock AI |

**Test fixtures:** `testdata/` directory with sample aggregated data per state for golden-file PDF comparison tests.

---

## 12. Integration Points (Wiring in main.go)

```go
// In cmd/quorant-api/main.go:

// Adapters (wrap existing repos for estoppel's narrow interfaces)
financialProvider := fin.NewEstoppelFinancialAdapter(finService)
complianceProvider := gov.NewEstoppelComplianceAdapter(govService)
propertyProvider := org.NewEstoppelPropertyAdapter(orgService)

// Narrative generator (wraps AI policy resolver)
narrativeGen := estoppel.NewNarrativeGenerator(policyResolver, contextRetriever, logger)

// PDF generator
pdfGen := estoppel.NewMarotoGenerator()

// Estoppel service
estoppelRepo := estoppel.NewPostgresRepository(pool)
estoppelService := estoppel.NewEstoppelService(
    estoppelRepo,
    financialProvider,
    complianceProvider,
    propertyProvider,
    narrativeGen,
    pdfGen,
    docService,
    taskService,
    auditor,
    publisher,
    logger,
)

// HTTP handlers
estoppelHandler := estoppel.NewHandler(estoppelService)
estoppel.RegisterRoutes(mux, estoppelHandler, tokenValidator, permChecker, entitlementChecker, resolveUserID)
```

---

## 13. Out of Scope (Phase 1)

- Title company direct API integration (Qualia, RamQuest, SoftPro)
- HomeWiseDocs / ReadyRESALE integration (Quorant replaces these)
- Cryptographic PDF digital signatures (PAdES)
- Per-org API keys for external requestors
- Bulk estoppel generation
- Auto-generated estoppel on ownership transfer events
- States beyond FL, CA, TX, NV, VA
