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

## Prerequisites

### Transaction Composition: Already Solved

The `feature/atomic-financial-ops` merge (issue #60) introduced the infrastructure this spec requires:

- **`DBTX` interface** (`platform/db/dbtx.go`): Satisfied by both `*pgxpool.Pool` and `pgx.Tx`. Repositories use this as their backing connection. When `DBTX` is already a `pgx.Tx`, `Begin()` creates a savepoint — advisory locks work either way.
- **`UnitOfWork` + `UnitOfWorkFactory`** (`platform/db/unitofwork.go`): Wraps a database transaction, providing a single commit/rollback boundary for multi-repository operations.
- **`WithTx(tx pgx.Tx)` on all repositories**: `AssessmentRepository`, `PaymentRepository`, `GLRepository`, `FundRepository`, `BudgetRepository`, `CollectionRepository` all support scoping to a transaction.
- **`GLService.WithTx`** (`gl_service.go`): Returns a transaction-scoped GL service instance.

`RecordPayment` already uses this pattern (service.go:314-398): UnitOfWork → `payments.WithTx(uow.Tx())` → `assessments.WithTx(uow.Tx())` → `gl.WithTx(uow.Tx())` → atomic commit. The allocation flow will extend this exact pattern by adding allocation writes within the same UnitOfWork.

**No prerequisite refactoring is needed.** The `allocatePayment` function uses the same `UnitOfWork` already acquired by `RecordPayment`.

### Relationship to Existing `ai.PolicyResolver`

`platform/policy.Resolver` complements, not replaces, the existing `ai.PolicyResolver`. The distinction:

- **`ai.PolicyResolver`**: RAG-based Q&A against the context lake. Used for open-ended policy questions ("does this CC&R allow short-term rentals?").
- **`platform/policy.Resolver`**: Structured policy resolution for operations. Tier 1 is deterministic; Tier 2 uses `ai.PolicyResolver.QueryPolicy` internally as its AI backend.

`FinService` will depend on both: `ai.PolicyResolver` for existing uses (e.g., `CreateAssessment` policy lookup at service.go:177), and `platform/policy.Resolver` for operation policy resolution. No migration of existing `ai.PolicyResolver` consumers is required.

## Architecture

### Resolution Flow

```
Business Operation (e.g., RecordPayment)
    │
    ▼
registry.Resolve(ctx, orgID, &unitID, "payment_allocation")
    │
    ├── Cache Check
    │     Key: (unitID, category, hash(active_policy_record_ids))
    │     Hit → return cached resolution (skip Tier 2)
    │     Miss → continue
    │
    ├── Tier 1: Query policy_records
    │     scope=jurisdiction (CA statutes)
    │     scope=org (CC&R overrides)
    │     scope=unit (bankruptcy freeze, payment plan)
    │
    ├── Tier 2: AI resolves precedence
    │     Input: gathered policy records + prompt template + schemas
    │     Output: structured JSON ruling + reasoning + confidence
    │     On AI failure → hold with review_status='ai_unavailable'
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
2. **Register at startup** — `registry.Register("category", descriptor)`
3. **Resolve at operation time** — `res, err := s.registry.Resolve(ctx, orgID, &unitID, "category")`

## `platform/policy` Package

### Package Structure

```
platform/policy/
├── resolver.go        // Resolver implementation (Tier 1 + Tier 2 + cache + gating)
├── registry.go        // Registry struct, descriptor registration, trigger discovery
├── descriptor.go      // OperationDescriptor, PolicySpec types
├── policy.go          // Resolution, PolicyReference types
├── repository.go      // PolicyRecordRepository, ResolutionRepository interfaces
├── policy_postgres.go // PostgreSQL implementations
├── cache.go           // Resolution cache (keyed on policy record hash)
├── review.go          // Human-in-the-loop review handling, SLA escalation
└── policy_test.go     // Unit tests
```

### Core Types

```go
// Registry holds operation descriptors and resolves policies.
// Instance-based — injected via constructors, not a global singleton.
type Registry struct {
    descriptors map[string]OperationDescriptor
    records     PolicyRecordRepository
    resolutions ResolutionRepository
    ai          ai.PolicyResolver          // Tier 2 backend
    cache       *ResolutionCache
    tasks       task.Service               // for review task dispatch
    logger      *slog.Logger
}

// NewRegistry creates a Registry. Injected into services via constructors.
func NewRegistry(
    records PolicyRecordRepository,
    resolutions ResolutionRepository,
    ai ai.PolicyResolver,
    tasks task.Service,
    logger *slog.Logger,
) *Registry

// Resolve executes the two-tier pipeline for a given operation category.
func (r *Registry) Resolve(ctx context.Context, orgID uuid.UUID, unitID *uuid.UUID, category string) (*Resolution, error)

// Register adds an operation descriptor. Called during service initialization.
func (r *Registry) Register(category string, desc OperationDescriptor) error

// FindTriggers returns matching policy specs for a document type and concepts.
// Used by the ingestion pipeline to discover which policy records to create.
func (r *Registry) FindTriggers(documentType string, concepts []string) []MatchedTrigger

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

type MatchedTrigger struct {
    Category string
    Key      string
    Spec     PolicySpec
}
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

### Resolution Cache

Tier 2 AI inference adds 500ms-2000ms+ latency per call. A resolution cache eliminates redundant inference when the underlying policy records haven't changed.

**Cache key:** `(unitID, category, sorted_hash(active_policy_record_ids))`
**TTL:** Configurable per category (default 1 hour).
**Invalidation:** On `policy_records` insert, update, or deactivation for the relevant scope (unit, org, jurisdiction), all cached resolutions for affected `(unit, category)` pairs are evicted.

```go
type ResolutionCache struct {
    store   map[string]*cachedResolution  // cache key → resolution + expiry
    mu      sync.RWMutex
    defaultTTL time.Duration
}

func (c *ResolutionCache) Get(unitID uuid.UUID, category string, policyHash string) (*Resolution, bool)
func (c *ResolutionCache) Set(unitID uuid.UUID, category string, policyHash string, res *Resolution)
func (c *ResolutionCache) Invalidate(unitID *uuid.UUID, orgID *uuid.UUID, category string)
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
                              'auto_approved', 'pending_review',
                              'confirmed', 'corrected', 'ai_unavailable'
                          )),
    review_sla_deadline   TIMESTAMPTZ,
    reviewed_by           UUID,
    review_notes          TEXT,
    reviewed_at           TIMESTAMPTZ,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_policy_resolutions_review
    ON policy_resolutions (review_status)
    WHERE review_status IN ('pending_review', 'ai_unavailable');

CREATE INDEX idx_policy_resolutions_sla
    ON policy_resolutions (review_sla_deadline)
    WHERE review_status IN ('pending_review', 'ai_unavailable')
      AND review_sla_deadline IS NOT NULL;
```

### `payment_allocations` Table

Tracks how each payment is distributed across specific charges. First consumer of the policy engine.

```sql
CREATE TYPE charge_type AS ENUM (
    'regular_assessment', 'special_assessment',
    'late_fee', 'interest', 'collection_cost', 'attorney_fee', 'fine'
);

CREATE TABLE payment_allocations (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    payment_id      UUID NOT NULL REFERENCES payments(id),
    charge_type     charge_type NOT NULL,
    charge_id       UUID NOT NULL,
    allocated_cents BIGINT NOT NULL CHECK (allocated_cents > 0),
    resolution_id   UUID NOT NULL REFERENCES policy_resolutions(id),
    estoppel_id     UUID,              -- non-null for closing/escrow payments tied to an estoppel
    reversed_at     TIMESTAMPTZ,       -- non-null if this allocation was reversed (NSF, refund)
    reversed_by_id  UUID,              -- the reversal payment_allocation that unwound this one
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_payment_allocations_payment
    ON payment_allocations (payment_id);

CREATE INDEX idx_payment_allocations_charge
    ON payment_allocations (charge_id);

CREATE INDEX idx_payment_allocations_estoppel
    ON payment_allocations (estoppel_id)
    WHERE estoppel_id IS NOT NULL;
```

## Confidence Gating and Human-in-the-Loop Review

### Confidence Threshold

Configurable per category (via `OperationDescriptor.DefaultThreshold`) and per org (via a `policy_records` entry with `key=confidence_threshold` for the category).

When the Tier 2 ruling confidence falls below the threshold:

1. The resolution is stored with `review_status='pending_review'`.
2. The `OnHold` callback is invoked — the consuming module decides what "held" means (e.g., payment saved with status `pending_review`, allocation deferred).
3. A review task is dispatched via the `task` module, assigned by role (e.g., `compliance_reviewer`).
4. A `review_sla_deadline` is set (default: 1 business day from creation).

### Review SLA and Escalation

Held resolutions have a configurable SLA deadline. The worker's scheduled job runner checks for breached SLAs and escalates:

1. **At SLA deadline:** Task priority elevated, notification sent to compliance manager.
2. **At 2x SLA deadline:** Task reassigned to org admin with an urgent flag.
3. **Configurable per category:** Payment allocation defaults to 1 business day. Less time-sensitive categories (e.g., reserve study compliance) may have longer SLAs.

The SLA is stored on `policy_resolutions.review_sla_deadline` so the worker can query breached reviews with a single indexed query.

### LLM Unavailability

When Tier 2 fails due to AI infrastructure issues (network error, timeout, rate limit), the resolution is not treated as an error returned to the caller. Instead:

1. Resolution stored with `review_status='ai_unavailable'`, confidence=0.
2. `OnHold` callback invoked — payment saved as `pending_review`.
3. Review task dispatched with context explaining AI was unavailable.
4. Staff can manually produce the ruling or wait for AI recovery.
5. When AI recovers, a scheduled job retries `ai_unavailable` resolutions automatically.

This prevents payment processing from failing due to transient AI outages.

### Review Task Content

The review task includes:
- The policy records considered (with statute references)
- The AI's ruling and reasoning (or "AI unavailable" explanation)
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
registry.Register("payment_allocation", OperationDescriptor{
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
        "trustee_plan_allocation": {
            Description:   "Court-ordered Chapter 13 trustee distribution terms that override statutory and CC&R allocation",
            DocumentTypes: []string{"confirmed_plan", "court_order", "trustee_distribution"},
            Concepts:      []string{"chapter 13 plan", "trustee distribution", "confirmed plan", "plan payments"},
            Schema:        trusteePlanSchema,
        },
        "estoppel_payoff": {
            Description:   "Estoppel certificate that fixes allocation for a closing/escrow lump-sum payment",
            DocumentTypes: []string{"estoppel_certificate"},
            Concepts:      []string{"estoppel", "closing", "payoff", "property sale", "escrow"},
            Schema:        estoppelPayoffSchema,
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
                "enum": [
                    "regular_assessment", "special_assessment",
                    "late_fee", "interest", "collection_cost", "attorney_fee", "fine"
                ]
            },
            "minItems": 1,
            "uniqueItems": true
        },
        "super_lien_months": {
            "type": "integer",
            "description": "Number of months of assessments with super-lien priority (NV=9, CO/CT=6). Null if not a super-lien state."
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
                "enum": [
                    "regular_assessment", "special_assessment",
                    "late_fee", "interest", "collection_cost", "attorney_fee", "fine"
                ]
            }
        },
        "effective_date": { "type": "string", "format": "date" },
        "suspend_late_fees": { "type": "boolean", "default": false }
    }
}
```

**`trustee_plan_allocation`** — Chapter 13 trustee distribution terms:
```json
{
    "type": "object",
    "required": ["case_number", "distribution_order"],
    "properties": {
        "case_number": { "type": "string" },
        "distribution_order": {
            "type": "array",
            "items": {
                "type": "string",
                "enum": [
                    "regular_assessment", "special_assessment",
                    "late_fee", "interest", "collection_cost", "attorney_fee", "fine"
                ]
            }
        },
        "fixed_monthly_cents": { "type": "integer" },
        "plan_duration_months": { "type": "integer" },
        "applies_to": {
            "type": "string",
            "enum": ["pre_petition_only", "all"],
            "default": "pre_petition_only"
        }
    }
}
```

**`estoppel_payoff`** — Estoppel-linked closing payment:
```json
{
    "type": "object",
    "required": ["estoppel_id", "itemized_amounts"],
    "properties": {
        "estoppel_id": { "type": "string", "format": "uuid" },
        "itemized_amounts": {
            "type": "object",
            "properties": {
                "regular_assessment_cents": { "type": "integer" },
                "special_assessment_cents": { "type": "integer" },
                "late_fee_cents": { "type": "integer" },
                "interest_cents": { "type": "integer" },
                "collection_cost_cents": { "type": "integer" },
                "attorney_fee_cents": { "type": "integer" },
                "fine_cents": { "type": "integer" }
            }
        },
        "valid_through": { "type": "string", "format": "date" }
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
                "enum": [
                    "regular_assessment", "special_assessment",
                    "late_fee", "interest", "collection_cost", "attorney_fee", "fine"
                ]
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
        },
        "estoppel_override": {
            "type": "boolean",
            "description": "If true, allocate per estoppel_payoff itemized amounts instead of FIFO"
        },
        "trustee_override": {
            "type": "boolean",
            "description": "If true, allocate per trustee_plan_allocation terms to pre-petition charges"
        }
    }
}
```

### Allocation Engine

The allocation engine is purely mechanical — no business judgment. It receives the ruling and executes:

1. Load all outstanding charges for the unit (regular assessments, special assessments, late fees, interest, fines, collection costs, attorney fees).
2. **If `estoppel_override`:** Allocate per the estoppel certificate's itemized amounts exactly. Link allocations via `estoppel_id`. Skip steps 3-5.
3. **If `trustee_override`:** Allocate trustee payment to pre-petition charges per court-ordered distribution terms. Skip frozen-charge filtering for this payment.
4. **Otherwise:** Remove frozen charges — by explicit `frozen_charge_ids` or by `frozen_cutoff_date` (charges with `due_date` or `created_at` before the cutoff).
5. Group remaining charges by type, ordered per `priority_order`.
6. Within each type group, sort FIFO (oldest `due_date` first).
7. Walk the ordered list, consuming the payment amount:
   - Full allocation if payment covers the charge.
   - Partial allocation if payment is exhausted mid-charge.
   - Remaining payment after all charges produces a credit balance.
8. Return a list of `PaymentAllocation` records.

### Concurrent Held Payments

When payment A is held for review and payment B arrives for the same unit:

1. Payment B resolves independently — it may or may not be held depending on its own confidence score.
2. If both are held, they are reviewed independently. The review task for each shows the other pending payment as context.
3. When the first held payment is released (confirmed/corrected), the resolution cache for the unit is invalidated.
4. When the second held payment is released, its `OnProceed` callback re-queries outstanding charges at execution time (not at resolution time), so it sees the first payment's allocations.

This serialization is guaranteed by the advisory lock in `CreateLedgerEntryTx` — even if two payments are released simultaneously, ledger writes are serialized at the DB level.

### NSF / Returned Payments and Reversals

When a completed payment is returned (ACH failure, bounced check, chargeback):

1. **Create a reversal payment record** with status `reversed` and a reference to the original payment.
2. **Create contra-allocation rows** — for each original `payment_allocations` row, create a matching row with negative `allocated_cents` on the reversal payment. Mark the original allocation's `reversed_at` and `reversed_by_id`.
3. **Create reversal ledger entries** — positive (debit) entries that reverse the original credits, each linked to the same `assessment_id`.
4. **Post reversal GL entries** — Debit AR / Credit Cash (reverse of the original).
5. **Re-evaluate late fees** — assessments that were partially or fully paid and are now unpaid may need late fee reinstatement. This triggers the `late_fee_calculation` policy category (future consumer of the same policy engine).

The `PaymentStatus` typed enum (`fin/enums.go`) gains three new values:

```go
PaymentStatusPendingReview PaymentStatus = "pending_review"  // held for policy review
PaymentStatusReversed      PaymentStatus = "reversed"         // NSF / ACH return / chargeback
PaymentStatusNSF           PaymentStatus = "nsf"              // specifically bounced check
```

The DB `payment_status` enum in `migrations/enums.sql` must be updated to match.

### Board Credits and Adjustments

Board-approved credits (waived late fees, courtesy adjustments, error corrections) use the same allocation infrastructure:

1. A credit is modeled as a payment with `charge_type` context — the board specifies which charge(s) the credit applies to.
2. The allocation engine applies the credit to the specified charges, creating `payment_allocations` rows with a resolution that records the board action.
3. For undirected credits (e.g., "apply $100 goodwill credit"), the policy engine resolves allocation order the same as a regular payment.

This ensures credits are tracked with the same audit trail as payments and correctly reflected in estoppel and aging calculations.

### Updated `RecordPayment` Flow

Uses the existing `UnitOfWork` + `WithTx` pattern from `feature/atomic-financial-ops`:

```go
func (s *FinService) RecordPayment(ctx context.Context, orgID uuid.UUID, userID uuid.UUID, req CreatePaymentRequest) (*Payment, error) {
    if err := req.Validate(); err != nil {
        return nil, err
    }

    // 1. Resolve allocation policy (before starting the transaction)
    res, err := s.registry.Resolve(ctx, orgID, &req.UnitID, "payment_allocation")
    if err != nil {
        return nil, err
    }

    // 2. Begin UnitOfWork — same pattern as existing RecordPayment
    var uow *db.UnitOfWork
    payments := s.payments
    assessments := s.assessments
    gl := s.gl

    if s.uowFactory != nil {
        uow, err = s.uowFactory.Begin(ctx)
        if err != nil {
            return nil, fmt.Errorf("fin: RecordPayment begin tx: %w", err)
        }
        defer uow.Rollback(ctx)
        payments = s.payments.WithTx(uow.Tx())
        assessments = s.assessments.WithTx(uow.Tx())
        if s.gl != nil {
            gl = s.gl.WithTx(uow.Tx())
        }
    }

    // 3. Create payment record
    payment, err := payments.CreatePayment(ctx, buildPayment(orgID, userID, req))
    if err != nil {
        return nil, err
    }

    // 4. If held for review, commit payment with pending_review status
    if res.Held() {
        payment.Status = PaymentStatusPendingReview
        // ... commit and return
    }

    // 5. Decode ruling, run allocation engine, persist allocations + ledger + GL
    var ruling PaymentAllocationRuling
    if err := res.Decode(&ruling); err != nil {
        return nil, err
    }
    if err := s.executeAllocations(ctx, payment, res.ID, ruling, assessments, gl); err != nil {
        return nil, err
    }

    if uow != nil {
        if err := uow.Commit(ctx); err != nil {
            return nil, fmt.Errorf("fin: RecordPayment commit: %w", err)
        }
    }
    return payment, nil
}

func (s *FinService) executeAllocations(ctx context.Context, payment *Payment, resolutionID uuid.UUID, ruling PaymentAllocationRuling, assessments AssessmentRepository, gl *GLService) error {
    // All operations run within the caller's UnitOfWork transaction:
    // 1. Load outstanding charges for the unit
    // 2. Run allocation engine (pure function: charges + ruling → []Allocation)
    // 3. Insert payment_allocations rows
    // 4. Create per-allocation ledger entries (each with assessment_id)
    //    - advisory lock acquired automatically via savepoint (DBTX.Begin)
    // 5. Post GL journal entries (Debit Cash / Credit AR per allocation)
    // 6. Handle credit balance if overpayment
}
```
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
    → Resolution cache invalidated for (unit-42b, payment_allocation)
    → Available for Tier 1 resolution on next payment
```

## End-to-End Walkthrough

### Setup

- California HOA, jurisdiction=CA
- Jurisdiction policy record seeded: `priority_order` = `["regular_assessment","special_assessment","collection_cost","attorney_fee","late_fee","interest"]` (CA Civil Code 5655(a))
- Homeowner in Unit 42B files Chapter 13 bankruptcy on 2026-03-15
- Management uploads court papers → AI creates `bankruptcy_freeze` policy record

### Payment Received: $500

Unit 42B has $2,000 in outstanding charges:
- $800 regular assessment (due 2026-02-01, pre-petition)
- $200 late fee on that assessment (pre-petition)
- $600 regular assessment (due 2026-04-01, post-petition)
- $400 regular assessment (due 2026-05-01, post-petition)

**Tier 1** gathers 2 policy records:
1. CA priority order (scope=jurisdiction, priority_hint=state)
2. Bankruptcy freeze (scope=unit, priority_hint=federal)

**Tier 2** AI reasons:
> "Federal bankruptcy law (automatic stay, 11 USC 362) supersedes state allocation statute for pre-petition charges. The $800 assessment and $200 late fee predate the 2026-03-15 filing and are frozen. Post-petition charges follow CA Civil Code 5655(a): assessments first."

Confidence: 0.92 (above 0.80 threshold, auto-approved).

**Ruling:**
```json
{
    "priority_order": ["regular_assessment","special_assessment","collection_cost","attorney_fee","late_fee","interest"],
    "frozen_charge_ids": ["<800-assessment-id>", "<200-late-fee-id>"],
    "accept_partial": true,
    "credit_handling": "apply_forward",
    "estoppel_override": false,
    "trustee_override": false
}
```

**Allocation engine** (mechanical):
1. Filters out 2 frozen charges
2. Eligible charges: $600 assessment (April), $400 assessment (May)
3. Priority tier: regular assessments. FIFO within tier: April first.
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
2. Review task dispatched to compliance staff (SLA: 1 business day)
3. Staff reviews, adds note: "Payment plan was rejected by the Chapter 13 trustee in the scheduling order. It does not survive."
4. Staff marks as **corrected**
5. Correction + notes saved to context lake
6. Tier 2 re-runs with correction as additional context → new ruling at confidence 0.91
7. New resolution stored with `parent_resolution_id` linking to original
8. `OnProceed` callback invoked → allocation executes

### Closing / Escrow Payment

Unit 42B is sold. Closing agent requests an estoppel certificate. The estoppel itemizes:
- Regular assessments: $1,100
- Late fees: $200
- Interest: $45

The estoppel is stored and creates an `estoppel_payoff` policy record on the unit. When the closing agent remits $1,345:

1. Tier 1 gathers: priority order + estoppel_payoff
2. Tier 2 rules: `estoppel_override: true` — allocate per estoppel itemization
3. Allocation matches estoppel exactly. `payment_allocations` rows link via `estoppel_id`.

### Future Benefit

Six months later, a different unit has a similar bankruptcy + payment plan situation. Tier 2 retrieves the previous correction from the context lake and resolves correctly the first time at high confidence.

## Downstream Impact

### Estoppel Certificates

With per-assessment allocation, estoppel generation can now produce accurate itemized statements:
- Outstanding balance per charge type (regular vs special assessment)
- Per-diem interest accrual rates
- Payments applied with allocation detail
- Pre-petition vs post-petition breakdown (bankruptcy cases)
- Super-lien priority amounts for applicable jurisdictions

### Delinquency Aging

Collections can now age by individual assessment rather than blanket balance:
- 30/60/90 day aging per assessment
- Late fee triggering per assessment due date and grace period
- Foreclosure threshold calculation on assessment-only amounts (required by CA Civil Code 5720)
- Super-lien amount calculation for NV (9 months), CO/CT (6 months)

### Reconciliation

`CheckReconciliation` can verify that:
- The sum of allocations for a payment equals the payment amount
- Ledger entries tie back to specific allocations
- Reversed allocations net to zero against their originals
- Estoppel-linked payments match the estoppel certificate amounts

## Jurisdiction Rules: Seed Data

Initial seed data for payment allocation priority orders:

| Jurisdiction | Priority Order | Super-Lien | Statute |
|---|---|---|---|
| CA | regular_assessment, special_assessment, collection_cost, attorney_fee, late_fee, interest | No | Civil Code 5655(a) |
| TX | regular_assessment, special_assessment, attorney_fee, fine | No | Property Code 209.0063(a) |
| FL | interest, late_fee, collection_cost, attorney_fee, regular_assessment, special_assessment | No | Statute 720.3085(3)(b) |
| NV | regular_assessment, special_assessment, late_fee, interest, collection_cost, attorney_fee | 9 months | NRS 116.3116 |
| CO | regular_assessment, special_assessment, late_fee, interest, collection_cost, attorney_fee | 6 months | CRS 38-33.3-316(2) |
| CT | regular_assessment, special_assessment, late_fee, interest, collection_cost, attorney_fee | 6 months | CGS 47-258(m) |
| Default | regular_assessment, special_assessment, late_fee, interest, collection_cost, attorney_fee, fine | No | Industry standard FIFO |

Additional jurisdictions added as `policy_records` with `scope=jurisdiction` as the platform expands.

## Testing Strategy

### Unit Tests
- Allocation engine: deterministic tests with various charge combinations, frozen charges, partial payments, overpayments, credit balances, mixed regular/special assessments
- Estoppel-override allocation: exact match to itemized amounts
- Trustee-override allocation: court-ordered distribution to pre-petition charges
- NSF reversal: contra-allocations, ledger reversal entries, late fee reinstatement
- Schema validation: invalid policy records rejected, invalid rulings trigger held flow
- Confidence gating: threshold boundary tests, hold/proceed callback invocation
- LLM unavailability: resolution held with `ai_unavailable`, retry on recovery
- Concurrent held payments: second payment sees first payment's allocations at execution time
- Resolution cache: hit/miss/invalidation behavior
- Registry: trigger discovery matching, duplicate registration rejection

### Integration Tests
- Full `RecordPayment` flow with real policy records and mock AI (deterministic ruling)
- Single-transaction guarantee: allocation + ledger + GL all committed or all rolled back
- Human review lifecycle: hold → confirm → proceed, hold → correct → re-resolve → proceed
- SLA escalation: breached review triggers escalation
- Concurrent payments with advisory locks (existing test pattern)
- NSF flow: payment → allocate → reverse → verify charge states restored

### Testability
- `Registry` is struct-based with interface dependencies — tests inject stubs for AI, repositories, task service
- `NoopRegistry` returns auto-approved rulings with configurable responses (like existing `NoopPolicyResolver` pattern)
- Allocation engine is a pure function: `allocate(charges, ruling) → []Allocation` — fully testable without any infrastructure
