# Platform Policy Engine & Payment Allocation Design

**Date:** 2026-04-09
**Issue:** #62 (P0: Payments not applied to specific assessments)
**Scope:** `platform/policy` reusable engine + `fin` payment allocation as first consumer

## Problem

Business operations across Quorant vary by jurisdiction, org-level CC&R provisions, and unit-level circumstances (bankruptcy, payment plans, disputes). Today, these rules are either hardcoded or absent — `RecordPayment` creates a blanket credit with no allocation to specific assessments, making delinquency aging, estoppel letters, and per-assessment late fees impossible.

The problem is not unique to payment allocation. Late fee calculation, refund processing, collection escalation, and many other operations face the same pattern: "how should this operation execute given the applicable laws, org policies, and unit circumstances?"

## Solution Overview

A two-tier policy resolution engine in `platform/policy` that any module can adopt:

- **Tier 1 (Deterministic):** Gathers all applicable policy records from the database — jurisdiction statutes, org-level overrides, unit-level circumstances.
- **Tier 2 (Interpretive):** AI reviews the gathered policies, reasons about precedence and conflicts, and produces a structured ruling that the business operation executes mechanically.

Payment allocation in the `fin` module is the first consumer.

## Architecture

### Resolution Flow

```
Business Operation (e.g., RecordPayment)
    │
    ▼
policy.Resolve(ctx, orgID, &unitID, "payment_allocation")
    │
    ├── Tier 1: Query policy_records
    │     scope=jurisdiction (CA statutes)
    │     scope=org (CC&R overrides)
    │     scope=unit (bankruptcy freeze, payment plan)
    │
    ├── Tier 2: AI resolves precedence
    │     Input: gathered policy records + prompt template + schemas
    │     Output: structured JSON ruling + reasoning + confidence
    │
    ├── Schema Validation
    │     Validate ruling against RulingSchema
    │     On failure → treat as low-confidence → held for review
    │
    ├── Confidence Gate
    │     confidence >= threshold → auto-approved → OnProceed callback
    │     confidence < threshold  → held → OnHold callback → review task
    │
    ▼
Resolution { Status, Ruling, Reasoning, Confidence, SourcePolicies }
```

### Module Integration Pattern

Any module integrates in three steps:

1. **Define an `OperationDescriptor`** — declares the category, policy specs (schemas + ingestion triggers), ruling schema, Tier 2 prompt template, confidence threshold, and hold/proceed callbacks.
2. **Register at startup** — `policy.Register("category", descriptor)`
3. **Resolve at operation time** — `res, err := s.policy.Resolve(ctx, orgID, &unitID, "category")`

## `platform/policy` Package

### Package Structure

```
platform/policy/
├── resolver.go        // Resolver interface and implementation
├── registry.go        // Descriptor registration, trigger discovery
├── descriptor.go      // OperationDescriptor, PolicySpec types
├── policy.go          // Resolution, PolicyReference types
├── repository.go      // PolicyRecordRepository interface
├── policy_postgres.go // PostgreSQL implementation
├── review.go          // Human-in-the-loop review handling
└── policy_test.go     // Unit tests
```

### Core Types

```go
// Resolver is the entry point for any module.
type Resolver interface {
    Resolve(ctx context.Context, orgID uuid.UUID, unitID *uuid.UUID, category string) (*Resolution, error)
}

// Resolution is the output of the two-tier pipeline.
type Resolution struct {
    ID             uuid.UUID
    Status         string            // "approved", "held"
    Ruling         json.RawMessage   // structured JSON, validated against RulingSchema
    Reasoning      string            // Tier 2 AI explanation
    Confidence     float64           // 0.0-1.0
    SourcePolicies []PolicyReference // which records Tier 1 gathered
    ParentID       *uuid.UUID        // non-nil for re-resolutions after correction
}

func (r *Resolution) Held() bool
func (r *Resolution) Decode(target any) error // unmarshal ruling into typed struct
```

### Operation Descriptor

```go
// OperationDescriptor is registered by each module for each operation category.
type OperationDescriptor struct {
    Category         string
    Description      string                    // human-readable, shown in review tasks
    DefaultThreshold float64                   // confidence gate default (0.0-1.0)
    Policies         map[string]PolicySpec     // key → policy type definition
    RulingSchema     json.RawMessage           // JSON Schema for Tier 2 output
    PromptTemplate   string                    // Tier 2 prompt with template variables
    OnHold           func(ctx context.Context, res *Resolution) error
    OnProceed        func(ctx context.Context, res *Resolution) error
}

// PolicySpec defines a single policy type within a category — its schema,
// ingestion triggers, and description. Serves as the shared contract between
// the ingestion pipeline, the resolution engine, and human reviewers.
type PolicySpec struct {
    Description   string          // ingestion-facing: when to create this record
    DocumentTypes []string        // document types that produce this record
    Concepts      []string        // semantic triggers for ingestion discovery
    Schema        json.RawMessage // JSON Schema for policy record values
}
```

### Registry

```go
// Register adds an operation descriptor. Called at module startup.
func Register(category string, desc OperationDescriptor) error

// FindTriggers returns matching policy specs for a document type and concepts.
// Used by the ingestion pipeline to discover which policy records to create.
func (r *Registry) FindTriggers(documentType string, concepts []string) []MatchedTrigger

type MatchedTrigger struct {
    Category string
    Key      string
    Spec     PolicySpec
}
```

## Data Model

### `policy_records` Table

Stores all policy records across all scopes and categories. Policy record values are validated against the registered `PolicySpec.Schema` at write time.

```sql
CREATE TABLE policy_records (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    scope             TEXT NOT NULL CHECK (scope IN ('jurisdiction', 'org', 'unit')),
    jurisdiction      TEXT,
    org_id            UUID REFERENCES organizations(id),
    unit_id           UUID REFERENCES units(id),
    category          TEXT NOT NULL,
    key               TEXT NOT NULL,
    value             JSONB NOT NULL,
    priority_hint     TEXT NOT NULL CHECK (priority_hint IN (
                          'federal', 'state', 'local', 'cc_r', 'board_policy'
                      )),
    statute_reference TEXT,
    source_doc_id     UUID,
    effective_date    DATE NOT NULL DEFAULT CURRENT_DATE,
    expiration_date   DATE,
    is_active         BOOLEAN NOT NULL DEFAULT true,
    created_by        UUID,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT chk_jurisdiction_scope CHECK (
        scope != 'jurisdiction' OR jurisdiction IS NOT NULL
    ),
    CONSTRAINT chk_org_scope CHECK (
        scope NOT IN ('org', 'unit') OR org_id IS NOT NULL
    ),
    CONSTRAINT chk_unit_scope CHECK (
        scope != 'unit' OR unit_id IS NOT NULL
    )
);

CREATE INDEX idx_policy_records_lookup
    ON policy_records (category, is_active)
    WHERE expiration_date IS NULL OR expiration_date > CURRENT_DATE;

CREATE INDEX idx_policy_records_unit
    ON policy_records (unit_id, category)
    WHERE unit_id IS NOT NULL AND is_active = true;
```

### Tier 1 Query

```sql
SELECT * FROM policy_records
WHERE category = $1
  AND is_active = true
  AND (expiration_date IS NULL OR expiration_date > CURRENT_DATE)
  AND effective_date <= CURRENT_DATE
  AND (
      (scope = 'jurisdiction' AND jurisdiction = $2)
      OR (scope = 'org' AND org_id = $3)
      OR (scope = 'unit' AND unit_id = $4)
  )
ORDER BY scope, priority_hint, effective_date;
```

### `policy_resolutions` Table

Audit trail for every Tier 2 resolution, including the human review lifecycle.

```sql
CREATE TABLE policy_resolutions (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id                UUID NOT NULL REFERENCES organizations(id),
    unit_id               UUID REFERENCES units(id),
    category              TEXT NOT NULL,
    input_policy_ids      UUID[] NOT NULL,
    ruling                JSONB NOT NULL,
    reasoning             TEXT NOT NULL,
    confidence            DECIMAL(3,2) NOT NULL,
    model_id              TEXT NOT NULL,
    parent_resolution_id  UUID REFERENCES policy_resolutions(id),
    review_status         TEXT NOT NULL DEFAULT 'auto_approved'
                          CHECK (review_status IN (
                              'auto_approved', 'pending_review', 'confirmed', 'corrected'
                          )),
    reviewed_by           UUID,
    review_notes          TEXT,
    reviewed_at           TIMESTAMPTZ,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_policy_resolutions_review
    ON policy_resolutions (review_status)
    WHERE review_status = 'pending_review';
```

### `payment_allocations` Table

Tracks how each payment is distributed across specific charges. First consumer of the policy engine.

```sql
CREATE TYPE charge_type AS ENUM (
    'assessment', 'late_fee', 'interest', 'collection_cost', 'attorney_fee', 'fine'
);

CREATE TABLE payment_allocations (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    payment_id      UUID NOT NULL REFERENCES payments(id),
    charge_type     charge_type NOT NULL,
    charge_id       UUID NOT NULL,
    allocated_cents BIGINT NOT NULL CHECK (allocated_cents > 0),
    resolution_id   UUID NOT NULL REFERENCES policy_resolutions(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_payment_allocations_payment
    ON payment_allocations (payment_id);

CREATE INDEX idx_payment_allocations_charge
    ON payment_allocations (charge_id);
```

## Confidence Gating and Human-in-the-Loop Review

### Confidence Threshold

Configurable per category (via `OperationDescriptor.DefaultThreshold`) and per org (via a `policy_records` entry with `key=confidence_threshold` for the category).

When the Tier 2 ruling confidence falls below the threshold:

1. The resolution is stored with `review_status='pending_review'`.
2. The `OnHold` callback is invoked — the consuming module decides what "held" means (e.g., payment saved with status `pending_review`, allocation deferred).
3. A review task is dispatched via the `task` module, assigned by role (e.g., `compliance_reviewer`).

### Review Task Content

The review task includes:
- The policy records considered (with statute references)
- The AI's ruling and reasoning
- The confidence score
- The operation context (payment amount, unit, charges)

### Review Outcomes

**Confirmed (with notes):**
- Resolution updated: `review_status='confirmed'`, `reviewed_by`, `review_notes`, `reviewed_at`
- Notes saved to context lake for future reference
- `OnProceed` callback invoked — held operation executes with the original ruling

**Corrected (with notes + corrected ruling):**
- Original resolution updated: `review_status='corrected'`, `reviewed_by`, `review_notes`, `reviewed_at`
- Correction + notes saved to context lake
- Tier 2 re-runs with the staff's correction as additional context
- New resolution created with `parent_resolution_id` linking to the original
- The new resolution goes through the confidence gate again (typically passes — human guidance raises confidence)
- `OnProceed` callback invoked with the new ruling

### Schema Validation Failure

If the Tier 2 ruling fails `RulingSchema` validation, it is treated as a low-confidence result:
- Logged with the raw AI response for debugging
- Triggers the held/review flow
- The reviewer sees the raw output alongside the expected schema

## Payment Allocation: First Consumer

### Registration

```go
policy.Register("payment_allocation", OperationDescriptor{
    Category:         "payment_allocation",
    Description:      "Determines how incoming payments are allocated across outstanding charges",
    DefaultThreshold: 0.80,
    Policies: map[string]PolicySpec{
        "priority_order": {
            Description:   "State statute or CC&R provision defining the order in which charge types are paid",
            DocumentTypes: []string{"cc_r", "bylaws", "state_statute", "board_resolution"},
            Concepts:      []string{"payment application order", "assessment priority", "allocation of payments"},
            Schema:        priorityOrderSchema,
        },
        "bankruptcy_freeze": {
            Description:   "Bankruptcy court filing that freezes pre-petition charges for a unit owner",
            DocumentTypes: []string{"bankruptcy_petition", "court_order", "trustee_notice"},
            Concepts:      []string{"bankruptcy", "chapter 7", "chapter 13", "automatic stay", "pre-petition"},
            Schema:        bankruptcyFreezeSchema,
        },
        "plan_override": {
            Description:   "Payment plan agreement that modifies the default allocation order for a unit",
            DocumentTypes: []string{"payment_plan_agreement", "board_resolution"},
            Concepts:      []string{"payment plan", "installment agreement", "repayment arrangement"},
            Schema:        planOverrideSchema,
        },
    },
    RulingSchema:     allocationRulingSchema,
    PromptTemplate:   paymentAllocationPrompt,
    OnHold:           s.holdPaymentAllocation,
    OnProceed:        s.executePaymentAllocation,
})
```

### Policy Schemas

**`priority_order`** — Charge type allocation order:
```json
{
    "type": "object",
    "required": ["order"],
    "properties": {
        "order": {
            "type": "array",
            "items": {
                "type": "string",
                "enum": ["assessment", "late_fee", "interest", "collection_cost", "attorney_fee", "fine"]
            },
            "minItems": 1,
            "uniqueItems": true
        }
    }
}
```

**`bankruptcy_freeze`** — Bankruptcy filing details:
```json
{
    "type": "object",
    "required": ["chapter", "filing_date", "case_number"],
    "properties": {
        "chapter": { "type": "integer", "enum": [7, 11, 13] },
        "filing_date": { "type": "string", "format": "date" },
        "case_number": { "type": "string" },
        "court": { "type": "string" },
        "pre_petition_charges_frozen": { "type": "boolean", "default": true },
        "case_status": {
            "type": "string",
            "enum": ["active", "dismissed", "discharged"],
            "default": "active"
        }
    }
}
```

**`plan_override`** — Payment plan allocation override:
```json
{
    "type": "object",
    "required": ["plan_id", "custom_priority_order"],
    "properties": {
        "plan_id": { "type": "string", "format": "uuid" },
        "custom_priority_order": {
            "type": "array",
            "items": {
                "type": "string",
                "enum": ["assessment", "late_fee", "interest", "collection_cost", "attorney_fee", "fine"]
            }
        },
        "effective_date": { "type": "string", "format": "date" },
        "suspend_late_fees": { "type": "boolean", "default": false }
    }
}
```

### Ruling Schema

The structured output Tier 2 must produce:

```json
{
    "type": "object",
    "required": ["priority_order", "frozen_charge_ids", "accept_partial"],
    "properties": {
        "priority_order": {
            "type": "array",
            "items": {
                "type": "string",
                "enum": ["assessment", "late_fee", "interest", "collection_cost", "attorney_fee", "fine"]
            }
        },
        "frozen_charge_ids": {
            "type": "array",
            "items": { "type": "string", "format": "uuid" }
        },
        "frozen_cutoff_date": { "type": "string", "format": "date" },
        "accept_partial": { "type": "boolean" },
        "credit_handling": {
            "type": "string",
            "enum": ["apply_forward", "hold", "refund"],
            "default": "apply_forward"
        }
    }
}
```

### Allocation Engine

The allocation engine is purely mechanical — no business judgment. It receives the ruling and executes:

1. Load all outstanding charges for the unit (assessments, late fees, interest, fines, collection costs, attorney fees).
2. Remove frozen charges — by explicit `frozen_charge_ids` or by `frozen_cutoff_date` (charges with `due_date` or `created_at` before the cutoff).
3. Group remaining charges by type, ordered per `priority_order`.
4. Within each type group, sort FIFO (oldest `due_date` first).
5. Walk the ordered list, consuming the payment amount:
   - Full allocation if payment covers the charge.
   - Partial allocation if payment is exhausted mid-charge.
   - Remaining payment after all charges produces a credit balance.
6. Return a list of `PaymentAllocation` records.

### Updated `RecordPayment` Flow

```go
func (s *FinService) RecordPayment(ctx context.Context, orgID uuid.UUID, userID uuid.UUID, req CreatePaymentRequest) (*Payment, error) {
    if err := req.Validate(); err != nil {
        return nil, err
    }

    // 1. Create payment record
    payment, err := s.createPaymentRecord(ctx, orgID, userID, req)
    if err != nil {
        return nil, err
    }

    // 2. Resolve allocation policy
    res, err := s.policy.Resolve(ctx, orgID, &req.UnitID, "payment_allocation")
    if err != nil {
        return nil, err
    }

    // 3. If held for review, payment is saved but unallocated
    if res.Held() {
        return payment, nil // OnHold already set status to pending_review
    }

    // 4. Decode ruling and allocate
    var ruling PaymentAllocationRuling
    if err := res.Decode(&ruling); err != nil {
        return nil, err
    }
    return s.allocatePayment(ctx, payment, res.ID, ruling)
}

func (s *FinService) allocatePayment(ctx context.Context, payment *Payment, resolutionID uuid.UUID, ruling PaymentAllocationRuling) (*Payment, error) {
    // In a single transaction:
    // - Load outstanding charges for the unit
    // - Run allocation engine (mechanical)
    // - Insert payment_allocations rows
    // - Create per-allocation ledger entries (each with assessment_id)
    // - Post GL journal entries (Debit Cash / Credit AR)
    // - Handle credit balance if overpayment
    // - Update payment status to completed
}
```

## Ingestion Integration

### Discovery Flow

When the AI document ingestion pipeline processes an uploaded document:

1. **Classify** the document (type + extracted concepts).
2. **Query the registry:** `registry.FindTriggers(documentType, concepts)` returns matching `PolicySpec` entries across all registered categories.
3. **Extract** structured data from the document per each matched `PolicySpec.Schema`.
4. **Create policy records**, validated against the schema at write time.

### Example: Bankruptcy Filing

```
Document uploaded: "Chapter 13 Petition - Unit 42B"
    → AI classifies: type="bankruptcy_petition", concepts=["chapter 13", "bankruptcy", "automatic stay"]
    → registry.FindTriggers("bankruptcy_petition", [...])
        → Match: category="payment_allocation", key="bankruptcy_freeze"
        → Schema: {chapter: int, filing_date: date, case_number: string, ...}
    → AI extracts from document: chapter=13, filing_date=2026-03-15, case_number="26-12345"
    → Creates policy_record:
        scope=unit, unit_id=<unit-42b>, category=payment_allocation,
        key=bankruptcy_freeze, priority_hint=federal,
        value={chapter:13, filing_date:"2026-03-15", case_number:"26-12345",
               court:"CD Cal", pre_petition_charges_frozen:true, case_status:"active"},
        source_doc_id=<uploaded document>
    → Validated against PolicySpec.Schema → passes
    → Available for Tier 1 resolution on next payment
```

## End-to-End Walkthrough

### Setup

- California HOA, jurisdiction=CA
- Jurisdiction policy record seeded: `priority_order` = `["assessment","collection_cost","attorney_fee","late_fee","interest"]` (CA Civil Code 5655(a))
- Homeowner in Unit 42B files Chapter 13 bankruptcy on 2026-03-15
- Management uploads court papers → AI creates `bankruptcy_freeze` policy record

### Payment Received: $500

Unit 42B has $2,000 in outstanding charges:
- $800 assessment (due 2026-02-01, pre-petition)
- $200 late fee on that assessment (pre-petition)
- $600 assessment (due 2026-04-01, post-petition)
- $400 assessment (due 2026-05-01, post-petition)

**Tier 1** gathers 2 policy records:
1. CA priority order (scope=jurisdiction, priority_hint=state)
2. Bankruptcy freeze (scope=unit, priority_hint=federal)

**Tier 2** AI reasons:
> "Federal bankruptcy law (automatic stay, 11 USC 362) supersedes state allocation statute for pre-petition charges. The $800 assessment and $200 late fee predate the 2026-03-15 filing and are frozen. Post-petition charges follow CA Civil Code 5655(a): assessments first."

Confidence: 0.92 (above 0.80 threshold, auto-approved).

**Ruling:**
```json
{
    "priority_order": ["assessment","collection_cost","attorney_fee","late_fee","interest"],
    "frozen_charge_ids": ["<800-assessment-id>", "<200-late-fee-id>"],
    "accept_partial": true,
    "credit_handling": "apply_forward"
}
```

**Allocation engine** (mechanical):
1. Filters out 2 frozen charges
2. Eligible charges: $600 assessment (April), $400 assessment (May)
3. Priority tier: assessments. FIFO within tier: April first.
4. Allocate $500 to April assessment (partial: $500 of $600)
5. Result: 1 allocation, $100 remaining on April assessment

**Persisted in one transaction:**
- Payment record (status: completed)
- 1 `payment_allocations` row ($500 to April assessment, linked to resolution)
- 1 ledger entry (payment credit, assessment_id = April assessment)
- GL journal entry (Debit Cash $500 / Credit AR $500)

### Low-Confidence Scenario

Same setup, but the homeowner also has a payment plan agreement signed before the bankruptcy filing. Tier 1 gathers 3 policy records. Tier 2 must reason about whether the pre-petition payment plan survives bankruptcy.

Confidence: 0.65 (below 0.80 threshold, held).

1. Payment created with status `pending_review`
2. Review task dispatched to compliance staff
3. Staff reviews, adds note: "Payment plan was rejected by the Chapter 13 trustee in the scheduling order. It does not survive."
4. Staff marks as **corrected**
5. Correction + notes saved to context lake
6. Tier 2 re-runs with correction as additional context → new ruling at confidence 0.91
7. New resolution stored with `parent_resolution_id` linking to original
8. `OnProceed` callback invoked → allocation executes

### Future Benefit

Six months later, a different unit has a similar bankruptcy + payment plan situation. Tier 2 retrieves the previous correction from the context lake and resolves correctly the first time at high confidence.

## Downstream Impact

### Estoppel Certificates

With per-assessment allocation, estoppel generation can now produce accurate itemized statements:
- Outstanding balance per charge type
- Per-diem interest accrual rates
- Payments applied with allocation detail
- Pre-petition vs post-petition breakdown (bankruptcy cases)

### Delinquency Aging

Collections can now age by individual assessment rather than blanket balance:
- 30/60/90 day aging per assessment
- Late fee triggering per assessment due date and grace period
- Foreclosure threshold calculation on assessment-only amounts (required by CA Civil Code 5720)

### Reconciliation

`CheckReconciliation` can verify that the sum of allocations for a payment equals the payment amount, and that ledger entries tie back to specific allocations.

## Jurisdiction Rules: Seed Data

Initial seed data for payment allocation priority orders:

| Jurisdiction | Priority Order | Statute |
|---|---|---|
| CA | assessment, collection_cost, attorney_fee, late_fee, interest | Civil Code 5655(a) |
| TX | assessment, attorney_fee, fine | Property Code 209.0063(a) |
| FL | interest, late_fee, collection_cost, attorney_fee, assessment | Statute 720.3085(3)(b) |
| Default | assessment, late_fee, interest, collection_cost, attorney_fee, fine | Industry standard FIFO |

Additional jurisdictions added as `policy_records` with `scope=jurisdiction` as the platform expands.

## Testing Strategy

### Unit Tests
- Allocation engine: deterministic tests with various charge combinations, frozen charges, partial payments, overpayments, credit balances
- Schema validation: invalid policy records rejected, invalid rulings trigger held flow
- Confidence gating: threshold boundary tests, hold/proceed callback invocation
- Registry: trigger discovery matching, duplicate registration rejection

### Integration Tests
- Full `RecordPayment` flow with real policy records and mock AI (deterministic ruling)
- Human review lifecycle: hold → confirm → proceed, hold → correct → re-resolve → proceed
- Concurrent payments with advisory locks (existing test pattern)

### Testability
- `Resolver` is an interface — tests inject a stub that returns deterministic rulings
- `NoopResolver` returns auto-approved rulings with configurable responses (like existing `NoopPolicyResolver` pattern)
