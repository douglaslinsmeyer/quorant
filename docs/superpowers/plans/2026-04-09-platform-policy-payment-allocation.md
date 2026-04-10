# Platform Policy Engine & Payment Allocation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a reusable two-tier policy resolution engine (`platform/policy`) and wire in payment allocation as its first consumer, replacing the blanket-credit approach in `RecordPayment`.

**Architecture:** New `platform/policy` package provides a `Registry` that any module can register operation descriptors with. At operation time, Tier 1 gathers applicable `policy_records` from the DB, Tier 2 sends them to `ai.PolicyResolver` for precedence reasoning, and the result is confidence-gated with human-in-the-loop review for low-confidence rulings. The `fin` module's `RecordPayment` is refactored to resolve a `payment_allocation` policy and mechanically allocate payments to specific charges using FIFO within priority tiers.

**Tech Stack:** Go 1.22, PostgreSQL 16 (pgx/v5), testify, existing DBTX/UnitOfWork pattern

**Spec:** `docs/superpowers/specs/2026-04-09-platform-policy-engine-design.md`

---

## File Map

### Phase 1: `platform/policy` Engine

| Action | File | Responsibility |
|--------|------|----------------|
| Create | `backend/internal/platform/policy/types.go` | Domain types: Resolution, PolicyRecord, PolicyReference, MatchedTrigger |
| Create | `backend/internal/platform/policy/descriptor.go` | OperationDescriptor, PolicySpec structs |
| Create | `backend/internal/platform/policy/repository.go` | PolicyRecordRepository, ResolutionRepository interfaces |
| Create | `backend/internal/platform/policy/registry.go` | Registry struct, Register, FindTriggers, Resolve |
| Create | `backend/internal/platform/policy/cache.go` | ResolutionCache with TTL and invalidation |
| Create | `backend/internal/platform/policy/policy_postgres.go` | PostgreSQL implementations of both repositories |
| Create | `backend/internal/platform/policy/noop.go` | NoopRegistry for tests |
| Create | `backend/internal/platform/policy/registry_test.go` | Unit tests for registry |
| Create | `backend/internal/platform/policy/cache_test.go` | Unit tests for cache |
| Create | `backend/internal/platform/policy/policy_postgres_test.go` | Integration tests for Postgres repos |
| Create | `backend/migrations/20260409000029_policy_records.sql` | policy_records + policy_resolutions tables |

### Phase 2: Payment Allocation Consumer

| Action | File | Responsibility |
|--------|------|----------------|
| Create | `backend/migrations/20260409000030_payment_allocations.sql` | charge_type enum + payment_allocations table + payment_status additions |
| Modify | `backend/internal/fin/enums.go` | Add ChargeType, PaymentStatus extensions |
| Create | `backend/internal/fin/allocation.go` | Pure allocation engine function |
| Create | `backend/internal/fin/allocation_test.go` | Allocation engine unit tests |
| Modify | `backend/internal/fin/payment_repository.go` | Add CreatePaymentAllocation, ListAllocationsByPayment |
| Modify | `backend/internal/fin/payment_postgres.go` | Implement allocation repo methods |
| Modify | `backend/internal/fin/domain.go` | Add PaymentAllocation struct |
| Modify | `backend/internal/fin/service.go` | Add policy.Registry dependency, refactor RecordPayment |
| Modify | `backend/internal/fin/service_iface.go` | Add allocation methods to Service interface |
| Modify | `backend/internal/fin/service_test.go` | Tests for allocation-aware RecordPayment |
| Modify | `backend/cmd/quorant-api/main.go` | Wire Registry into FinService |

---

## Phase 1: Platform Policy Engine

### Task 1: Database Migration — policy_records and policy_resolutions

**Files:**
- Create: `backend/migrations/20260409000029_policy_records.sql`

- [ ] **Step 1: Write the migration**

```sql
-- 20260409000029_policy_records.sql
-- Policy records store jurisdiction rules, org overrides, and unit-level policies.
-- Policy resolutions store the Tier 2 AI ruling audit trail.

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

- [ ] **Step 2: Verify migration file exists and is valid SQL**

Run: `cd backend && cat migrations/20260409000029_policy_records.sql | head -5`
Expected: First lines of migration shown.

- [ ] **Step 3: Commit**

```bash
git add backend/migrations/20260409000029_policy_records.sql
git commit -m "feat: add policy_records and policy_resolutions tables (issue #62)"
```

---

### Task 2: Domain Types — types.go and descriptor.go

**Files:**
- Create: `backend/internal/platform/policy/types.go`
- Create: `backend/internal/platform/policy/descriptor.go`

- [ ] **Step 1: Create types.go with domain types**

```go
// Package policy provides a reusable two-tier policy resolution engine.
// Tier 1 gathers applicable policy records from the database (deterministic).
// Tier 2 sends them to an AI resolver for precedence reasoning (interpretive).
// Any module can register operation descriptors and resolve policies at runtime.
package policy

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Resolution is the output of the two-tier policy resolution pipeline.
type Resolution struct {
	ID             uuid.UUID         `json:"id"`
	Status         string            `json:"status"` // "approved", "held"
	Ruling         json.RawMessage   `json:"ruling"`
	Reasoning      string            `json:"reasoning"`
	Confidence     float64           `json:"confidence"`
	SourcePolicies []PolicyReference `json:"source_policies"`
	ParentID       *uuid.UUID        `json:"parent_id,omitempty"`
}

// Held returns true if the resolution requires human review before the
// consuming operation can proceed.
func (r *Resolution) Held() bool {
	return r.Status == "held"
}

// Decode unmarshals the ruling JSON into the given target struct.
func (r *Resolution) Decode(target any) error {
	return json.Unmarshal(r.Ruling, target)
}

// PolicyRecord is a jurisdiction rule, org override, or unit-level policy
// stored in the database.
type PolicyRecord struct {
	ID              uuid.UUID       `json:"id"`
	Scope           string          `json:"scope"` // "jurisdiction", "org", "unit"
	Jurisdiction    *string         `json:"jurisdiction,omitempty"`
	OrgID           *uuid.UUID      `json:"org_id,omitempty"`
	UnitID          *uuid.UUID      `json:"unit_id,omitempty"`
	Category        string          `json:"category"`
	Key             string          `json:"key"`
	Value           json.RawMessage `json:"value"`
	PriorityHint    string          `json:"priority_hint"`
	StatuteRef      *string         `json:"statute_reference,omitempty"`
	SourceDocID     *uuid.UUID      `json:"source_doc_id,omitempty"`
	EffectiveDate   time.Time       `json:"effective_date"`
	ExpirationDate  *time.Time      `json:"expiration_date,omitempty"`
	IsActive        bool            `json:"is_active"`
	CreatedBy       *uuid.UUID      `json:"created_by,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

// PolicyReference is a lightweight reference to a policy record included in
// a resolution for audit purposes.
type PolicyReference struct {
	ID           uuid.UUID `json:"id"`
	Scope        string    `json:"scope"`
	Category     string    `json:"category"`
	Key          string    `json:"key"`
	PriorityHint string   `json:"priority_hint"`
	StatuteRef   *string   `json:"statute_reference,omitempty"`
}

// ResolutionRecord is the persisted form of a resolution, including the
// human review lifecycle fields.
type ResolutionRecord struct {
	ID                 uuid.UUID       `json:"id"`
	OrgID              uuid.UUID       `json:"org_id"`
	UnitID             *uuid.UUID      `json:"unit_id,omitempty"`
	Category           string          `json:"category"`
	InputPolicyIDs     []uuid.UUID     `json:"input_policy_ids"`
	Ruling             json.RawMessage `json:"ruling"`
	Reasoning          string          `json:"reasoning"`
	Confidence         float64         `json:"confidence"`
	ModelID            string          `json:"model_id"`
	ParentResolutionID *uuid.UUID      `json:"parent_resolution_id,omitempty"`
	ReviewStatus       string          `json:"review_status"`
	ReviewSLADeadline  *time.Time      `json:"review_sla_deadline,omitempty"`
	ReviewedBy         *uuid.UUID      `json:"reviewed_by,omitempty"`
	ReviewNotes        *string         `json:"review_notes,omitempty"`
	ReviewedAt         *time.Time      `json:"reviewed_at,omitempty"`
	CreatedAt          time.Time       `json:"created_at"`
}

// MatchedTrigger is returned by FindTriggers when a document type or concept
// matches a registered policy spec.
type MatchedTrigger struct {
	Category string
	Key      string
	Spec     PolicySpec
}
```

- [ ] **Step 2: Create descriptor.go with OperationDescriptor and PolicySpec**

```go
package policy

import (
	"context"
	"encoding/json"
)

// OperationDescriptor is registered by each module for each operation category.
// It defines the policy types the operation depends on, the expected AI ruling
// schema, and callbacks for the hold/proceed lifecycle.
type OperationDescriptor struct {
	Category         string
	Description      string
	DefaultThreshold float64
	Policies         map[string]PolicySpec
	RulingSchema     json.RawMessage
	PromptTemplate   string
	OnHold           func(ctx context.Context, res *Resolution) error
	OnProceed        func(ctx context.Context, res *Resolution) error
}

// PolicySpec defines a single policy type within a category. It serves as the
// shared contract between the ingestion pipeline, the resolution engine, and
// human reviewers.
type PolicySpec struct {
	Description   string
	DocumentTypes []string
	Concepts      []string
	Schema        json.RawMessage
}
```

- [ ] **Step 3: Verify compilation**

Run: `cd backend && go build ./internal/platform/policy/...`
Expected: No errors.

- [ ] **Step 4: Commit**

```bash
git add backend/internal/platform/policy/types.go backend/internal/platform/policy/descriptor.go
git commit -m "feat: add platform/policy domain types and descriptors (issue #62)"
```

---

### Task 3: Repository Interfaces

**Files:**
- Create: `backend/internal/platform/policy/repository.go`

- [ ] **Step 1: Write repository interfaces**

```go
package policy

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// PolicyRecordRepository persists and retrieves policy records.
type PolicyRecordRepository interface {
	// CreateRecord inserts a new policy record and returns the populated row.
	CreateRecord(ctx context.Context, r *PolicyRecord) (*PolicyRecord, error)

	// FindRecordByID returns the record with the given id, or nil if not found.
	FindRecordByID(ctx context.Context, id uuid.UUID) (*PolicyRecord, error)

	// GatherForResolution returns all active, non-expired policy records matching
	// the given category for the specified jurisdiction, org, and optional unit.
	// This is the Tier 1 query.
	GatherForResolution(ctx context.Context, category string, jurisdiction string, orgID uuid.UUID, unitID *uuid.UUID) ([]PolicyRecord, error)

	// DeactivateRecord sets is_active = false for the given record.
	DeactivateRecord(ctx context.Context, id uuid.UUID) error

	// WithTx returns a copy scoped to the given transaction.
	WithTx(tx pgx.Tx) PolicyRecordRepository
}

// ResolutionRepository persists and retrieves policy resolutions.
type ResolutionRepository interface {
	// CreateResolution inserts a new resolution record and returns the populated row.
	CreateResolution(ctx context.Context, r *ResolutionRecord) (*ResolutionRecord, error)

	// FindResolutionByID returns the resolution with the given id, or nil if not found.
	FindResolutionByID(ctx context.Context, id uuid.UUID) (*ResolutionRecord, error)

	// UpdateReviewStatus updates the review lifecycle fields on a resolution.
	UpdateReviewStatus(ctx context.Context, id uuid.UUID, status string, reviewedBy *uuid.UUID, notes *string) error

	// ListPendingReviews returns resolutions with review_status 'pending_review'
	// or 'ai_unavailable', ordered by created_at.
	ListPendingReviews(ctx context.Context) ([]ResolutionRecord, error)

	// WithTx returns a copy scoped to the given transaction.
	WithTx(tx pgx.Tx) ResolutionRepository
}
```

- [ ] **Step 2: Verify compilation**

Run: `cd backend && go build ./internal/platform/policy/...`
Expected: No errors.

- [ ] **Step 3: Commit**

```bash
git add backend/internal/platform/policy/repository.go
git commit -m "feat: add platform/policy repository interfaces (issue #62)"
```

---

### Task 4: Registry — Registration and Tier 1 Gather

**Files:**
- Create: `backend/internal/platform/policy/registry.go`
- Create: `backend/internal/platform/policy/registry_test.go`

- [ ] **Step 1: Write the failing test for Register and FindTriggers**

```go
package policy_test

import (
	"encoding/json"
	"testing"

	"github.com/quorant/quorant/internal/platform/policy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistry_Register(t *testing.T) {
	r := policy.NewRegistry(nil, nil, nil, nil, nil)

	desc := policy.OperationDescriptor{
		Category:         "test_category",
		Description:      "Test operation",
		DefaultThreshold: 0.80,
		Policies: map[string]policy.PolicySpec{
			"rule_a": {
				Description:   "A test rule",
				DocumentTypes: []string{"test_doc"},
				Concepts:      []string{"testing", "rules"},
				Schema:        json.RawMessage(`{"type":"object"}`),
			},
		},
		RulingSchema:   json.RawMessage(`{"type":"object"}`),
		PromptTemplate: "test prompt",
	}

	err := r.Register("test_category", desc)
	require.NoError(t, err)

	// Duplicate registration should fail.
	err = r.Register("test_category", desc)
	require.Error(t, err)
}

func TestRegistry_FindTriggers(t *testing.T) {
	r := policy.NewRegistry(nil, nil, nil, nil, nil)

	err := r.Register("payment_allocation", policy.OperationDescriptor{
		Category:         "payment_allocation",
		Description:      "Payment allocation",
		DefaultThreshold: 0.80,
		Policies: map[string]policy.PolicySpec{
			"bankruptcy_freeze": {
				Description:   "Bankruptcy filing",
				DocumentTypes: []string{"bankruptcy_petition", "court_order"},
				Concepts:      []string{"bankruptcy", "chapter 13"},
				Schema:        json.RawMessage(`{"type":"object"}`),
			},
			"priority_order": {
				Description:   "Charge priority",
				DocumentTypes: []string{"state_statute"},
				Concepts:      []string{"payment priority"},
				Schema:        json.RawMessage(`{"type":"object"}`),
			},
		},
		RulingSchema:   json.RawMessage(`{"type":"object"}`),
		PromptTemplate: "test",
	})
	require.NoError(t, err)

	// Exact document type match.
	triggers := r.FindTriggers("bankruptcy_petition", nil)
	require.Len(t, triggers, 1)
	assert.Equal(t, "payment_allocation", triggers[0].Category)
	assert.Equal(t, "bankruptcy_freeze", triggers[0].Key)

	// No match.
	triggers = r.FindTriggers("unknown_doc", nil)
	assert.Empty(t, triggers)

	// Concept match.
	triggers = r.FindTriggers("", []string{"bankruptcy"})
	require.Len(t, triggers, 1)
	assert.Equal(t, "bankruptcy_freeze", triggers[0].Key)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/platform/policy/... -run TestRegistry -short -count=1`
Expected: FAIL — `NewRegistry` not defined.

- [ ] **Step 3: Write the Registry implementation**

```go
package policy

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/quorant/quorant/internal/ai"
)

// OrgJurisdictionLookup provides the jurisdiction string for an org.
// Narrow interface satisfied by org.OrgRepository or a stub in tests.
type OrgJurisdictionLookup interface {
	GetJurisdiction(ctx context.Context, orgID uuid.UUID) (string, error)
}

// Registry holds operation descriptors and resolves policies. Instance-based —
// injected via constructors, not a global singleton.
type Registry struct {
	mu          sync.RWMutex
	descriptors map[string]OperationDescriptor
	records     PolicyRecordRepository
	resolutions ResolutionRepository
	ai          ai.PolicyResolver
	orgLookup   OrgJurisdictionLookup
	logger      *slog.Logger
}

// NewRegistry creates a Registry. Pass nil for repos/ai/orgLookup in unit tests.
func NewRegistry(
	records PolicyRecordRepository,
	resolutions ResolutionRepository,
	aiResolver ai.PolicyResolver,
	orgLookup OrgJurisdictionLookup,
	logger *slog.Logger,
) *Registry {
	return &Registry{
		descriptors: make(map[string]OperationDescriptor),
		records:     records,
		resolutions: resolutions,
		ai:          aiResolver,
		orgLookup:   orgLookup,
		logger:      logger,
	}
}

// lookupJurisdiction returns the jurisdiction for the given org, or "DEFAULT".
func (r *Registry) lookupJurisdiction(ctx context.Context, orgID uuid.UUID) string {
	if r.orgLookup == nil {
		return "DEFAULT"
	}
	j, err := r.orgLookup.GetJurisdiction(ctx, orgID)
	if err != nil || j == "" {
		return "DEFAULT"
	}
	return j
}

// Register adds an operation descriptor for the given category.
// Returns an error if the category is already registered.
func (r *Registry) Register(category string, desc OperationDescriptor) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.descriptors[category]; exists {
		return fmt.Errorf("policy: category %q already registered", category)
	}
	r.descriptors[category] = desc
	return nil
}

// FindTriggers returns policy specs whose DocumentTypes or Concepts match
// the given document type or concepts. Used by the ingestion pipeline.
func (r *Registry) FindTriggers(documentType string, concepts []string) []MatchedTrigger {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var matches []MatchedTrigger
	for category, desc := range r.descriptors {
		for key, spec := range desc.Policies {
			if matchesTrigger(spec, documentType, concepts) {
				matches = append(matches, MatchedTrigger{
					Category: category,
					Key:      key,
					Spec:     spec,
				})
			}
		}
	}
	return matches
}

func matchesTrigger(spec PolicySpec, documentType string, concepts []string) bool {
	if documentType != "" {
		for _, dt := range spec.DocumentTypes {
			if dt == documentType {
				return true
			}
		}
	}
	for _, c := range concepts {
		for _, sc := range spec.Concepts {
			if strings.EqualFold(c, sc) {
				return true
			}
		}
	}
	return false
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd backend && go test ./internal/platform/policy/... -run TestRegistry -short -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/internal/platform/policy/registry.go backend/internal/platform/policy/registry_test.go
git commit -m "feat: add platform/policy Registry with Register and FindTriggers (issue #62)"
```

---

### Task 5: Resolution Cache

**Files:**
- Create: `backend/internal/platform/policy/cache.go`
- Create: `backend/internal/platform/policy/cache_test.go`

- [ ] **Step 1: Write the failing test**

```go
package policy_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/platform/policy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolutionCache_SetAndGet(t *testing.T) {
	cache := policy.NewResolutionCache(10 * time.Minute)

	unitID := uuid.New()
	category := "payment_allocation"
	policyHash := "abc123"

	res := &policy.Resolution{
		ID:         uuid.New(),
		Status:     "approved",
		Confidence: 0.95,
	}

	cache.Set(unitID, category, policyHash, res)

	got, ok := cache.Get(unitID, category, policyHash)
	require.True(t, ok)
	assert.Equal(t, res.ID, got.ID)
}

func TestResolutionCache_Miss(t *testing.T) {
	cache := policy.NewResolutionCache(10 * time.Minute)

	_, ok := cache.Get(uuid.New(), "test", "hash")
	assert.False(t, ok)
}

func TestResolutionCache_Invalidate(t *testing.T) {
	cache := policy.NewResolutionCache(10 * time.Minute)

	unitID := uuid.New()
	category := "payment_allocation"

	res := &policy.Resolution{ID: uuid.New(), Status: "approved"}
	cache.Set(unitID, category, "hash1", res)

	cache.Invalidate(&unitID, nil, category)

	_, ok := cache.Get(unitID, category, "hash1")
	assert.False(t, ok, "should be evicted after invalidation")
}

func TestResolutionCache_Expiry(t *testing.T) {
	cache := policy.NewResolutionCache(1 * time.Millisecond)

	unitID := uuid.New()
	res := &policy.Resolution{ID: uuid.New(), Status: "approved"}
	cache.Set(unitID, "cat", "hash", res)

	time.Sleep(5 * time.Millisecond)

	_, ok := cache.Get(unitID, "cat", "hash")
	assert.False(t, ok, "should be expired")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/platform/policy/... -run TestResolutionCache -short -count=1`
Expected: FAIL — `NewResolutionCache` not defined.

- [ ] **Step 3: Write the cache implementation**

```go
package policy

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

type cachedResolution struct {
	resolution *Resolution
	expiresAt  time.Time
}

// ResolutionCache caches Tier 2 resolutions keyed on (unitID, category, policyHash).
// Entries are evicted on expiry or explicit invalidation.
type ResolutionCache struct {
	mu         sync.RWMutex
	store      map[string]*cachedResolution
	defaultTTL time.Duration
}

// NewResolutionCache creates a cache with the given default TTL.
func NewResolutionCache(defaultTTL time.Duration) *ResolutionCache {
	return &ResolutionCache{
		store:      make(map[string]*cachedResolution),
		defaultTTL: defaultTTL,
	}
}

func cacheKey(unitID uuid.UUID, category, policyHash string) string {
	return fmt.Sprintf("%s:%s:%s", unitID, category, policyHash)
}

// Get retrieves a cached resolution. Returns false on miss or expiry.
func (c *ResolutionCache) Get(unitID uuid.UUID, category, policyHash string) (*Resolution, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.store[cacheKey(unitID, category, policyHash)]
	if !ok || time.Now().After(entry.expiresAt) {
		return nil, false
	}
	return entry.resolution, true
}

// Set stores a resolution in the cache with the default TTL.
func (c *ResolutionCache) Set(unitID uuid.UUID, category, policyHash string, res *Resolution) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.store[cacheKey(unitID, category, policyHash)] = &cachedResolution{
		resolution: res,
		expiresAt:  time.Now().Add(c.defaultTTL),
	}
}

// Invalidate evicts cached resolutions matching the given scope.
// Pass unitID to invalidate a specific unit, or orgID for all units in an org
// (org-level invalidation clears all entries for the category).
func (c *ResolutionCache) Invalidate(unitID *uuid.UUID, orgID *uuid.UUID, category string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if unitID != nil {
		// Delete all entries for this unit + category (any policyHash).
		prefix := fmt.Sprintf("%s:%s:", *unitID, category)
		for k := range c.store {
			if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
				delete(c.store, k)
			}
		}
		return
	}

	// Org-level or jurisdiction-level: clear all entries for the category.
	suffix := ":" + category + ":"
	for k := range c.store {
		if contains(k, suffix) {
			delete(c.store, k)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd backend && go test ./internal/platform/policy/... -run TestResolutionCache -short -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/internal/platform/policy/cache.go backend/internal/platform/policy/cache_test.go
git commit -m "feat: add platform/policy resolution cache with TTL and invalidation (issue #62)"
```

---

### Task 6: Registry.Resolve — The Two-Tier Pipeline

**Files:**
- Modify: `backend/internal/platform/policy/registry.go`
- Modify: `backend/internal/platform/policy/registry_test.go`

- [ ] **Step 1: Write the failing test for Resolve**

Add to `registry_test.go`:

```go
func TestRegistry_Resolve_AutoApproved(t *testing.T) {
	// Stub AI resolver that returns a high-confidence ruling.
	aiResolver := &stubPolicyResolver{
		result: &ai.ResolutionResult{
			Resolution: json.RawMessage(`{"priority_order":["assessment"]}`),
			Reasoning:  "Test reasoning",
			Confidence: 0.95,
		},
	}
	records := &stubRecordRepo{
		records: []policy.PolicyRecord{
			{
				ID:           uuid.New(),
				Scope:        "jurisdiction",
				Category:     "test_op",
				Key:          "rule_a",
				Value:        json.RawMessage(`{"order":["assessment"]}`),
				PriorityHint: "state",
			},
		},
	}
	resolutions := &stubResolutionRepo{}

	reg := policy.NewRegistry(records, resolutions, aiResolver, nil, testLogger())

	err := reg.Register("test_op", policy.OperationDescriptor{
		Category:         "test_op",
		Description:      "Test",
		DefaultThreshold: 0.80,
		Policies:         map[string]policy.PolicySpec{},
		RulingSchema:     json.RawMessage(`{"type":"object"}`),
		PromptTemplate:   "Given policies: {{.Policies}}, produce a ruling.",
	})
	require.NoError(t, err)

	orgID := uuid.New()
	res, err := reg.Resolve(context.Background(), orgID, nil, "test_op")
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, "approved", res.Status)
	assert.False(t, res.Held())
	assert.InDelta(t, 0.95, res.Confidence, 0.01)
}

func TestRegistry_Resolve_Held(t *testing.T) {
	aiResolver := &stubPolicyResolver{
		result: &ai.ResolutionResult{
			Resolution: json.RawMessage(`{"priority_order":["assessment"]}`),
			Reasoning:  "Uncertain about plan override",
			Confidence: 0.60,
		},
	}
	records := &stubRecordRepo{
		records: []policy.PolicyRecord{
			{ID: uuid.New(), Scope: "jurisdiction", Category: "test_op", Key: "rule_a",
				Value: json.RawMessage(`{}`), PriorityHint: "state"},
		},
	}

	var holdCalled bool
	resolutions := &stubResolutionRepo{}
	reg := policy.NewRegistry(records, resolutions, aiResolver, nil, testLogger())

	err := reg.Register("test_op", policy.OperationDescriptor{
		Category:         "test_op",
		Description:      "Test",
		DefaultThreshold: 0.80,
		Policies:         map[string]policy.PolicySpec{},
		RulingSchema:     json.RawMessage(`{"type":"object"}`),
		PromptTemplate:   "test",
		OnHold: func(ctx context.Context, res *policy.Resolution) error {
			holdCalled = true
			return nil
		},
	})
	require.NoError(t, err)

	res, err := reg.Resolve(context.Background(), uuid.New(), nil, "test_op")
	require.NoError(t, err)
	assert.Equal(t, "held", res.Status)
	assert.True(t, res.Held())
	assert.True(t, holdCalled)
}

func TestRegistry_Resolve_AIUnavailable(t *testing.T) {
	aiResolver := &stubPolicyResolver{err: fmt.Errorf("connection refused")}
	records := &stubRecordRepo{
		records: []policy.PolicyRecord{
			{ID: uuid.New(), Scope: "jurisdiction", Category: "test_op", Key: "rule_a",
				Value: json.RawMessage(`{}`), PriorityHint: "state"},
		},
	}

	var holdCalled bool
	resolutions := &stubResolutionRepo{}
	reg := policy.NewRegistry(records, resolutions, aiResolver, nil, testLogger())

	err := reg.Register("test_op", policy.OperationDescriptor{
		Category:         "test_op",
		Description:      "Test",
		DefaultThreshold: 0.80,
		Policies:         map[string]policy.PolicySpec{},
		RulingSchema:     json.RawMessage(`{"type":"object"}`),
		PromptTemplate:   "test",
		OnHold: func(ctx context.Context, res *policy.Resolution) error {
			holdCalled = true
			return nil
		},
	})
	require.NoError(t, err)

	res, err := reg.Resolve(context.Background(), uuid.New(), nil, "test_op")
	require.NoError(t, err, "AI failure should not propagate as an error")
	assert.Equal(t, "held", res.Status)
	assert.True(t, holdCalled)
}

// ── Test stubs ───────────────────────────────────────────────────────────────

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type stubPolicyResolver struct {
	result *ai.ResolutionResult
	err    error
}

func (s *stubPolicyResolver) GetPolicy(_ context.Context, _ uuid.UUID, _ string) (*ai.PolicyResult, error) {
	return nil, nil
}

func (s *stubPolicyResolver) QueryPolicy(_ context.Context, _ uuid.UUID, _ string, _ ai.QueryContext) (*ai.ResolutionResult, error) {
	return s.result, s.err
}

type stubRecordRepo struct {
	records []policy.PolicyRecord
}

func (s *stubRecordRepo) CreateRecord(_ context.Context, r *policy.PolicyRecord) (*policy.PolicyRecord, error) {
	r.ID = uuid.New()
	return r, nil
}

func (s *stubRecordRepo) FindRecordByID(_ context.Context, id uuid.UUID) (*policy.PolicyRecord, error) {
	for _, r := range s.records {
		if r.ID == id { return &r, nil }
	}
	return nil, nil
}

func (s *stubRecordRepo) GatherForResolution(_ context.Context, category, jurisdiction string, orgID uuid.UUID, unitID *uuid.UUID) ([]policy.PolicyRecord, error) {
	var result []policy.PolicyRecord
	for _, r := range s.records {
		if r.Category == category {
			result = append(result, r)
		}
	}
	return result, nil
}

func (s *stubRecordRepo) DeactivateRecord(_ context.Context, _ uuid.UUID) error { return nil }

func (s *stubRecordRepo) WithTx(_ pgx.Tx) policy.PolicyRecordRepository { return s }

type stubResolutionRepo struct {
	resolutions []policy.ResolutionRecord
}

func (s *stubResolutionRepo) CreateResolution(_ context.Context, r *policy.ResolutionRecord) (*policy.ResolutionRecord, error) {
	r.ID = uuid.New()
	s.resolutions = append(s.resolutions, *r)
	return r, nil
}

func (s *stubResolutionRepo) FindResolutionByID(_ context.Context, id uuid.UUID) (*policy.ResolutionRecord, error) {
	for _, r := range s.resolutions {
		if r.ID == id { return &r, nil }
	}
	return nil, nil
}

func (s *stubResolutionRepo) UpdateReviewStatus(_ context.Context, _ uuid.UUID, _ string, _ *uuid.UUID, _ *string) error {
	return nil
}

func (s *stubResolutionRepo) ListPendingReviews(_ context.Context) ([]policy.ResolutionRecord, error) {
	return nil, nil
}

func (s *stubResolutionRepo) WithTx(_ pgx.Tx) policy.ResolutionRepository { return s }
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd backend && go test ./internal/platform/policy/... -run TestRegistry_Resolve -short -count=1`
Expected: FAIL — `Resolve` method not defined.

- [ ] **Step 3: Add Resolve method to registry.go**

Add these imports and method to `registry.go`:

```go
import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/ai"
)
```

Add `Resolve` method:

```go
// Resolve executes the two-tier policy resolution pipeline for the given
// category. It gathers applicable records (Tier 1), sends them to the AI
// resolver for precedence reasoning (Tier 2), validates the ruling, and
// applies the confidence gate.
func (r *Registry) Resolve(ctx context.Context, orgID uuid.UUID, unitID *uuid.UUID, category string) (*Resolution, error) {
	r.mu.RLock()
	desc, ok := r.descriptors[category]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("policy: category %q not registered", category)
	}

	// Tier 1: Gather all applicable policy records.
	jurisdiction := r.lookupJurisdiction(ctx, orgID)
	records, err := r.records.GatherForResolution(ctx, category, jurisdiction, orgID, unitID)
	if err != nil {
		return nil, fmt.Errorf("policy: gather records: %w", err)
	}

	// Build policy IDs for audit trail.
	policyIDs := make([]uuid.UUID, len(records))
	for i, rec := range records {
		policyIDs[i] = rec.ID
	}

	// Tier 2: AI resolves precedence.
	ruling, reasoning, confidence, modelID, aiErr := r.callTier2(ctx, orgID, desc, records)

	// Determine resolution status.
	status := "approved"
	reviewStatus := "auto_approved"
	if aiErr != nil {
		// AI unavailable: hold for human review.
		status = "held"
		reviewStatus = "ai_unavailable"
		ruling = json.RawMessage(`{}`)
		reasoning = fmt.Sprintf("AI unavailable: %v", aiErr)
		confidence = 0
		modelID = "unavailable"
		if r.logger != nil {
			r.logger.Error("Tier 2 AI unavailable", "category", category, "error", aiErr)
		}
	} else if confidence < desc.DefaultThreshold {
		status = "held"
		reviewStatus = "pending_review"
	}

	// Persist the resolution.
	resRecord := &ResolutionRecord{
		OrgID:          orgID,
		UnitID:         unitID,
		Category:       category,
		InputPolicyIDs: policyIDs,
		Ruling:         ruling,
		Reasoning:      reasoning,
		Confidence:     confidence,
		ModelID:        modelID,
		ReviewStatus:   reviewStatus,
	}
	if r.resolutions != nil {
		resRecord, err = r.resolutions.CreateResolution(ctx, resRecord)
		if err != nil {
			return nil, fmt.Errorf("policy: persist resolution: %w", err)
		}
	}

	refs := make([]PolicyReference, len(records))
	for i, rec := range records {
		refs[i] = PolicyReference{
			ID: rec.ID, Scope: rec.Scope, Category: rec.Category,
			Key: rec.Key, PriorityHint: rec.PriorityHint, StatuteRef: rec.StatuteRef,
		}
	}

	res := &Resolution{
		ID:             resRecord.ID,
		Status:         status,
		Ruling:         ruling,
		Reasoning:      reasoning,
		Confidence:     confidence,
		SourcePolicies: refs,
	}

	// Invoke lifecycle callback.
	if res.Held() && desc.OnHold != nil {
		if err := desc.OnHold(ctx, res); err != nil {
			return nil, fmt.Errorf("policy: OnHold callback: %w", err)
		}
	}

	return res, nil
}

// callTier2 sends gathered records to the AI resolver and returns the ruling.
func (r *Registry) callTier2(ctx context.Context, orgID uuid.UUID, desc OperationDescriptor, records []PolicyRecord) (json.RawMessage, string, float64, string, error) {
	if r.ai == nil {
		return json.RawMessage(`{}`), "no AI resolver configured", 0, "none", fmt.Errorf("no AI resolver")
	}

	// Build the prompt from template + records.
	recordsJSON, _ := json.Marshal(records)
	query := strings.ReplaceAll(desc.PromptTemplate, "{{.Policies}}", string(recordsJSON))

	qctx := ai.QueryContext{
		Module:       "policy",
		ResourceType: desc.Category,
	}
	result, err := r.ai.QueryPolicy(ctx, orgID, query, qctx)
	if err != nil {
		return nil, "", 0, "", err
	}

	return result.Resolution, result.Reasoning, result.Confidence, "ai", nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd backend && go test ./internal/platform/policy/... -run TestRegistry_Resolve -short -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/internal/platform/policy/registry.go backend/internal/platform/policy/registry_test.go
git commit -m "feat: add Registry.Resolve with two-tier pipeline and confidence gating (issue #62)"
```

---

### Task 7: NoopRegistry for Consumer Tests

**Files:**
- Create: `backend/internal/platform/policy/noop.go`

- [ ] **Step 1: Write NoopRegistry**

```go
package policy

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
)

// NoopRegistry is a test stub that returns pre-configured resolutions.
// Use in module tests where policy resolution is not under test.
type NoopRegistry struct {
	ruling     json.RawMessage
	confidence float64
}

// NewNoopRegistry creates a NoopRegistry that returns an auto-approved
// resolution with the given ruling and confidence.
func NewNoopRegistry(ruling json.RawMessage, confidence float64) *NoopRegistry {
	return &NoopRegistry{ruling: ruling, confidence: confidence}
}

// NewNoopRegistryDefault creates a NoopRegistry with a minimal approved ruling.
func NewNoopRegistryDefault() *NoopRegistry {
	return &NoopRegistry{
		ruling:     json.RawMessage(`{}`),
		confidence: 1.0,
	}
}

// Register is a no-op.
func (n *NoopRegistry) Register(category string, desc OperationDescriptor) error {
	return nil
}

// FindTriggers returns no matches.
func (n *NoopRegistry) FindTriggers(documentType string, concepts []string) []MatchedTrigger {
	return nil
}

// Resolve returns a pre-configured auto-approved resolution.
func (n *NoopRegistry) Resolve(ctx context.Context, orgID uuid.UUID, unitID *uuid.UUID, category string) (*Resolution, error) {
	return &Resolution{
		ID:         uuid.New(),
		Status:     "approved",
		Ruling:     n.ruling,
		Reasoning:  "noop: auto-approved for testing",
		Confidence: n.confidence,
	}, nil
}
```

- [ ] **Step 2: Verify compilation**

Run: `cd backend && go build ./internal/platform/policy/...`
Expected: No errors.

- [ ] **Step 3: Commit**

```bash
git add backend/internal/platform/policy/noop.go
git commit -m "feat: add NoopRegistry for consumer module tests (issue #62)"
```

---

### Task 8: PostgreSQL Repository Implementations

**Files:**
- Create: `backend/internal/platform/policy/policy_postgres.go`

- [ ] **Step 1: Write PostgreSQL implementations**

```go
package policy

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	dbpkg "github.com/quorant/quorant/internal/platform/db"
)

// ── PolicyRecordRepository ───────────────────────────────────────────────────

// PostgresPolicyRecordRepository implements PolicyRecordRepository.
type PostgresPolicyRecordRepository struct {
	db dbpkg.DBTX
}

func NewPostgresPolicyRecordRepository(pool *pgxpool.Pool) *PostgresPolicyRecordRepository {
	return &PostgresPolicyRecordRepository{db: pool}
}

func (r *PostgresPolicyRecordRepository) WithTx(tx pgx.Tx) PolicyRecordRepository {
	return &PostgresPolicyRecordRepository{db: tx}
}

func (r *PostgresPolicyRecordRepository) CreateRecord(ctx context.Context, rec *PolicyRecord) (*PolicyRecord, error) {
	const q = `
		INSERT INTO policy_records (
			scope, jurisdiction, org_id, unit_id, category, key, value,
			priority_hint, statute_reference, source_doc_id, effective_date,
			expiration_date, is_active, created_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		RETURNING id, scope, jurisdiction, org_id, unit_id, category, key, value,
		          priority_hint, statute_reference, source_doc_id, effective_date,
		          expiration_date, is_active, created_by, created_at, updated_at`

	row := r.db.QueryRow(ctx, q,
		rec.Scope, rec.Jurisdiction, rec.OrgID, rec.UnitID,
		rec.Category, rec.Key, rec.Value, rec.PriorityHint,
		rec.StatuteRef, rec.SourceDocID, rec.EffectiveDate,
		rec.ExpirationDate, rec.IsActive, rec.CreatedBy,
	)
	return scanPolicyRecord(row)
}

func (r *PostgresPolicyRecordRepository) FindRecordByID(ctx context.Context, id uuid.UUID) (*PolicyRecord, error) {
	const q = `
		SELECT id, scope, jurisdiction, org_id, unit_id, category, key, value,
		       priority_hint, statute_reference, source_doc_id, effective_date,
		       expiration_date, is_active, created_by, created_at, updated_at
		FROM policy_records WHERE id = $1`

	row := r.db.QueryRow(ctx, q, id)
	rec, err := scanPolicyRecord(row)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, nil
		}
		return nil, err
	}
	return rec, nil
}

func (r *PostgresPolicyRecordRepository) GatherForResolution(ctx context.Context, category, jurisdiction string, orgID uuid.UUID, unitID *uuid.UUID) ([]PolicyRecord, error) {
	const q = `
		SELECT id, scope, jurisdiction, org_id, unit_id, category, key, value,
		       priority_hint, statute_reference, source_doc_id, effective_date,
		       expiration_date, is_active, created_by, created_at, updated_at
		FROM policy_records
		WHERE category = $1
		  AND is_active = true
		  AND (expiration_date IS NULL OR expiration_date > CURRENT_DATE)
		  AND effective_date <= CURRENT_DATE
		  AND (
		      (scope = 'jurisdiction' AND jurisdiction = $2)
		      OR (scope = 'org' AND org_id = $3)
		      OR (scope = 'unit' AND unit_id = $4)
		  )
		ORDER BY scope, priority_hint, effective_date`

	rows, err := r.db.Query(ctx, q, category, jurisdiction, orgID, unitID)
	if err != nil {
		return nil, fmt.Errorf("policy: gather records: %w", err)
	}
	defer rows.Close()

	var records []PolicyRecord
	for rows.Next() {
		rec, err := scanPolicyRecordRows(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, *rec)
	}
	if records == nil {
		return []PolicyRecord{}, nil
	}
	return records, nil
}

func (r *PostgresPolicyRecordRepository) DeactivateRecord(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx,
		`UPDATE policy_records SET is_active = false, updated_at = now() WHERE id = $1`, id)
	return err
}

func scanPolicyRecord(row pgx.Row) (*PolicyRecord, error) {
	var rec PolicyRecord
	err := row.Scan(
		&rec.ID, &rec.Scope, &rec.Jurisdiction, &rec.OrgID, &rec.UnitID,
		&rec.Category, &rec.Key, &rec.Value, &rec.PriorityHint,
		&rec.StatuteRef, &rec.SourceDocID, &rec.EffectiveDate,
		&rec.ExpirationDate, &rec.IsActive, &rec.CreatedBy,
		&rec.CreatedAt, &rec.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

func scanPolicyRecordRows(rows pgx.Rows) (*PolicyRecord, error) {
	var rec PolicyRecord
	err := rows.Scan(
		&rec.ID, &rec.Scope, &rec.Jurisdiction, &rec.OrgID, &rec.UnitID,
		&rec.Category, &rec.Key, &rec.Value, &rec.PriorityHint,
		&rec.StatuteRef, &rec.SourceDocID, &rec.EffectiveDate,
		&rec.ExpirationDate, &rec.IsActive, &rec.CreatedBy,
		&rec.CreatedAt, &rec.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

// ── ResolutionRepository ─────────────────────────────────────────────────────

// PostgresResolutionRepository implements ResolutionRepository.
type PostgresResolutionRepository struct {
	db dbpkg.DBTX
}

func NewPostgresResolutionRepository(pool *pgxpool.Pool) *PostgresResolutionRepository {
	return &PostgresResolutionRepository{db: pool}
}

func (r *PostgresResolutionRepository) WithTx(tx pgx.Tx) ResolutionRepository {
	return &PostgresResolutionRepository{db: tx}
}

func (r *PostgresResolutionRepository) CreateResolution(ctx context.Context, rec *ResolutionRecord) (*ResolutionRecord, error) {
	const q = `
		INSERT INTO policy_resolutions (
			org_id, unit_id, category, input_policy_ids, ruling, reasoning,
			confidence, model_id, parent_resolution_id, review_status, review_sla_deadline
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, org_id, unit_id, category, input_policy_ids, ruling, reasoning,
		          confidence, model_id, parent_resolution_id, review_status,
		          review_sla_deadline, reviewed_by, review_notes, reviewed_at, created_at`

	// Default SLA: 24 hours for held resolutions.
	var slaDeadline *time.Time
	if rec.ReviewStatus == "pending_review" || rec.ReviewStatus == "ai_unavailable" {
		t := time.Now().Add(24 * time.Hour)
		slaDeadline = &t
	}

	row := r.db.QueryRow(ctx, q,
		rec.OrgID, rec.UnitID, rec.Category, rec.InputPolicyIDs,
		rec.Ruling, rec.Reasoning, rec.Confidence, rec.ModelID,
		rec.ParentResolutionID, rec.ReviewStatus, slaDeadline,
	)
	return scanResolutionRecord(row)
}

func (r *PostgresResolutionRepository) FindResolutionByID(ctx context.Context, id uuid.UUID) (*ResolutionRecord, error) {
	const q = `
		SELECT id, org_id, unit_id, category, input_policy_ids, ruling, reasoning,
		       confidence, model_id, parent_resolution_id, review_status,
		       review_sla_deadline, reviewed_by, review_notes, reviewed_at, created_at
		FROM policy_resolutions WHERE id = $1`

	row := r.db.QueryRow(ctx, q, id)
	rec, err := scanResolutionRecord(row)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, nil
		}
		return nil, err
	}
	return rec, nil
}

func (r *PostgresResolutionRepository) UpdateReviewStatus(ctx context.Context, id uuid.UUID, status string, reviewedBy *uuid.UUID, notes *string) error {
	now := time.Now()
	_, err := r.db.Exec(ctx,
		`UPDATE policy_resolutions SET review_status = $2, reviewed_by = $3, review_notes = $4, reviewed_at = $5 WHERE id = $1`,
		id, status, reviewedBy, notes, now)
	return err
}

func (r *PostgresResolutionRepository) ListPendingReviews(ctx context.Context) ([]ResolutionRecord, error) {
	const q = `
		SELECT id, org_id, unit_id, category, input_policy_ids, ruling, reasoning,
		       confidence, model_id, parent_resolution_id, review_status,
		       review_sla_deadline, reviewed_by, review_notes, reviewed_at, created_at
		FROM policy_resolutions
		WHERE review_status IN ('pending_review', 'ai_unavailable')
		ORDER BY created_at`

	rows, err := r.db.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []ResolutionRecord
	for rows.Next() {
		rec, err := scanResolutionRecordRows(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, *rec)
	}
	if records == nil {
		return []ResolutionRecord{}, nil
	}
	return records, nil
}

func scanResolutionRecord(row pgx.Row) (*ResolutionRecord, error) {
	var rec ResolutionRecord
	err := row.Scan(
		&rec.ID, &rec.OrgID, &rec.UnitID, &rec.Category, &rec.InputPolicyIDs,
		&rec.Ruling, &rec.Reasoning, &rec.Confidence, &rec.ModelID,
		&rec.ParentResolutionID, &rec.ReviewStatus, &rec.ReviewSLADeadline,
		&rec.ReviewedBy, &rec.ReviewNotes, &rec.ReviewedAt, &rec.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

func scanResolutionRecordRows(rows pgx.Rows) (*ResolutionRecord, error) {
	var rec ResolutionRecord
	err := rows.Scan(
		&rec.ID, &rec.OrgID, &rec.UnitID, &rec.Category, &rec.InputPolicyIDs,
		&rec.Ruling, &rec.Reasoning, &rec.Confidence, &rec.ModelID,
		&rec.ParentResolutionID, &rec.ReviewStatus, &rec.ReviewSLADeadline,
		&rec.ReviewedBy, &rec.ReviewNotes, &rec.ReviewedAt, &rec.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &rec, nil
}
```

- [ ] **Step 2: Verify compilation**

Run: `cd backend && go build ./internal/platform/policy/...`
Expected: No errors.

- [ ] **Step 3: Commit**

```bash
git add backend/internal/platform/policy/policy_postgres.go
git commit -m "feat: add PostgreSQL implementations for policy repositories (issue #62)"
```

---

## Phase 2: Payment Allocation Consumer

### Task 9: Database Migration — charge_type enum and payment_allocations

**Files:**
- Create: `backend/migrations/20260409000030_payment_allocations.sql`

- [ ] **Step 1: Write the migration**

```sql
-- 20260409000030_payment_allocations.sql
-- Adds charge_type enum, payment_allocations table, and new payment_status values.

CREATE TYPE charge_type AS ENUM (
    'regular_assessment', 'special_assessment',
    'late_fee', 'interest', 'collection_cost', 'attorney_fee', 'fine'
);

ALTER TYPE payment_status ADD VALUE IF NOT EXISTS 'pending_review';
ALTER TYPE payment_status ADD VALUE IF NOT EXISTS 'reversed';
ALTER TYPE payment_status ADD VALUE IF NOT EXISTS 'nsf';

CREATE TABLE payment_allocations (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    payment_id      UUID NOT NULL REFERENCES payments(id),
    charge_type     charge_type NOT NULL,
    charge_id       UUID NOT NULL,
    allocated_cents BIGINT NOT NULL CHECK (allocated_cents > 0),
    resolution_id   UUID NOT NULL REFERENCES policy_resolutions(id),
    estoppel_id     UUID,
    reversed_at     TIMESTAMPTZ,
    reversed_by_id  UUID,
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

- [ ] **Step 2: Commit**

```bash
git add backend/migrations/20260409000030_payment_allocations.sql
git commit -m "feat: add charge_type enum, payment_allocations table, payment_status extensions (issue #62)"
```

---

### Task 10: Typed Enums — ChargeType and PaymentStatus Extensions

**Files:**
- Modify: `backend/internal/fin/enums.go`
- Modify: `backend/internal/fin/enums_test.go`

- [ ] **Step 1: Write the failing test for new enums**

Add to `enums_test.go`:

```go
func TestChargeType_IsValid(t *testing.T) {
	valid := []fin.ChargeType{
		fin.ChargeTypeRegularAssessment, fin.ChargeTypeSpecialAssessment,
		fin.ChargeTypeLateFee, fin.ChargeTypeInterest,
		fin.ChargeTypeCollectionCost, fin.ChargeTypeAttorneyFee, fin.ChargeTypeFine,
	}
	for _, v := range valid {
		assert.True(t, v.IsValid(), "expected %q to be valid", v)
	}

	invalid := []fin.ChargeType{"", "unknown", "Assessment"}
	for _, v := range invalid {
		assert.False(t, v.IsValid(), "expected %q to be invalid", v)
	}
}

func TestPaymentStatus_NewValues(t *testing.T) {
	assert.True(t, fin.PaymentStatusPendingReview.IsValid())
	assert.True(t, fin.PaymentStatusReversed.IsValid())
	assert.True(t, fin.PaymentStatusNSF.IsValid())
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/fin/... -run "TestChargeType|TestPaymentStatus_New" -short -count=1`
Expected: FAIL — `ChargeType` not defined.

- [ ] **Step 3: Add enums to enums.go**

Add after the existing `PaymentStatus` block:

```go
const (
	PaymentStatusPendingReview PaymentStatus = "pending_review"
	PaymentStatusReversed      PaymentStatus = "reversed"
	PaymentStatusNSF           PaymentStatus = "nsf"
)
```

Update the `IsValid` method for `PaymentStatus`:

```go
func (s PaymentStatus) IsValid() bool {
	switch s {
	case PaymentStatusPending, PaymentStatusCompleted, PaymentStatusFailed,
		PaymentStatusPendingReview, PaymentStatusReversed, PaymentStatusNSF:
		return true
	}
	return false
}
```

Add the new `ChargeType`:

```go
// ChargeType classifies a charge for payment allocation purposes.
type ChargeType string

const (
	ChargeTypeRegularAssessment ChargeType = "regular_assessment"
	ChargeTypeSpecialAssessment ChargeType = "special_assessment"
	ChargeTypeLateFee           ChargeType = "late_fee"
	ChargeTypeInterest          ChargeType = "interest"
	ChargeTypeCollectionCost    ChargeType = "collection_cost"
	ChargeTypeAttorneyFee       ChargeType = "attorney_fee"
	ChargeTypeFine              ChargeType = "fine"
)

// IsValid returns true if the ChargeType value is one of the defined constants.
func (s ChargeType) IsValid() bool {
	switch s {
	case ChargeTypeRegularAssessment, ChargeTypeSpecialAssessment,
		ChargeTypeLateFee, ChargeTypeInterest,
		ChargeTypeCollectionCost, ChargeTypeAttorneyFee, ChargeTypeFine:
		return true
	}
	return false
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd backend && go test ./internal/fin/... -run "TestChargeType|TestPaymentStatus_New" -short -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/internal/fin/enums.go backend/internal/fin/enums_test.go
git commit -m "feat: add ChargeType enum and PaymentStatus extensions (issue #62)"
```

---

### Task 11: PaymentAllocation Domain Type

**Files:**
- Modify: `backend/internal/fin/domain.go`

- [ ] **Step 1: Add PaymentAllocation struct to domain.go**

Add after the `Payment` struct:

```go
// PaymentAllocation records how a specific portion of a payment was applied
// to a specific charge. Links payments to assessments, late fees, etc.
type PaymentAllocation struct {
	ID             uuid.UUID  `json:"id"`
	PaymentID      uuid.UUID  `json:"payment_id"`
	ChargeType     string     `json:"charge_type"`
	ChargeID       uuid.UUID  `json:"charge_id"`
	AllocatedCents int64      `json:"allocated_cents"`
	ResolutionID   uuid.UUID  `json:"resolution_id"`
	EstoppelID     *uuid.UUID `json:"estoppel_id,omitempty"`
	ReversedAt     *time.Time `json:"reversed_at,omitempty"`
	ReversedByID   *uuid.UUID `json:"reversed_by_id,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}
```

- [ ] **Step 2: Verify compilation**

Run: `cd backend && go build ./internal/fin/...`
Expected: No errors.

- [ ] **Step 3: Commit**

```bash
git add backend/internal/fin/domain.go
git commit -m "feat: add PaymentAllocation domain type (issue #62)"
```

---

### Task 12: Allocation Engine — Pure Function

**Files:**
- Create: `backend/internal/fin/allocation.go`
- Create: `backend/internal/fin/allocation_test.go`

- [ ] **Step 1: Write the failing tests**

```go
package fin_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/fin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAllocate_SingleAssessmentFullPayment(t *testing.T) {
	charges := []fin.OutstandingCharge{
		{ID: uuid.New(), ChargeType: fin.ChargeTypeRegularAssessment, AmountCents: 10000, DueDate: time.Now()},
	}
	ruling := fin.AllocationRuling{
		PriorityOrder: []fin.ChargeType{fin.ChargeTypeRegularAssessment},
		AcceptPartial: true,
	}

	allocs, creditCents := fin.Allocate(charges, 10000, ruling)

	require.Len(t, allocs, 1)
	assert.Equal(t, int64(10000), allocs[0].AllocatedCents)
	assert.Equal(t, charges[0].ID, allocs[0].ChargeID)
	assert.Equal(t, int64(0), creditCents)
}

func TestAllocate_PartialPayment(t *testing.T) {
	charges := []fin.OutstandingCharge{
		{ID: uuid.New(), ChargeType: fin.ChargeTypeRegularAssessment, AmountCents: 10000, DueDate: time.Now()},
	}
	ruling := fin.AllocationRuling{
		PriorityOrder: []fin.ChargeType{fin.ChargeTypeRegularAssessment},
		AcceptPartial: true,
	}

	allocs, creditCents := fin.Allocate(charges, 5000, ruling)

	require.Len(t, allocs, 1)
	assert.Equal(t, int64(5000), allocs[0].AllocatedCents)
	assert.Equal(t, int64(0), creditCents)
}

func TestAllocate_Overpayment(t *testing.T) {
	charges := []fin.OutstandingCharge{
		{ID: uuid.New(), ChargeType: fin.ChargeTypeRegularAssessment, AmountCents: 5000, DueDate: time.Now()},
	}
	ruling := fin.AllocationRuling{
		PriorityOrder: []fin.ChargeType{fin.ChargeTypeRegularAssessment},
		AcceptPartial: true,
	}

	allocs, creditCents := fin.Allocate(charges, 8000, ruling)

	require.Len(t, allocs, 1)
	assert.Equal(t, int64(5000), allocs[0].AllocatedCents)
	assert.Equal(t, int64(3000), creditCents)
}

func TestAllocate_PriorityOrder(t *testing.T) {
	assessmentID := uuid.New()
	lateFeeID := uuid.New()

	charges := []fin.OutstandingCharge{
		{ID: lateFeeID, ChargeType: fin.ChargeTypeLateFee, AmountCents: 2000, DueDate: time.Now()},
		{ID: assessmentID, ChargeType: fin.ChargeTypeRegularAssessment, AmountCents: 10000, DueDate: time.Now()},
	}
	// Assessments first (CA-style).
	ruling := fin.AllocationRuling{
		PriorityOrder: []fin.ChargeType{fin.ChargeTypeRegularAssessment, fin.ChargeTypeLateFee},
		AcceptPartial: true,
	}

	allocs, _ := fin.Allocate(charges, 11000, ruling)

	require.Len(t, allocs, 2)
	assert.Equal(t, assessmentID, allocs[0].ChargeID, "assessment should be allocated first")
	assert.Equal(t, int64(10000), allocs[0].AllocatedCents)
	assert.Equal(t, lateFeeID, allocs[1].ChargeID)
	assert.Equal(t, int64(1000), allocs[1].AllocatedCents)
}

func TestAllocate_FIFOWithinTier(t *testing.T) {
	olderID := uuid.New()
	newerID := uuid.New()

	older := time.Now().AddDate(0, -2, 0)
	newer := time.Now().AddDate(0, -1, 0)

	charges := []fin.OutstandingCharge{
		{ID: newerID, ChargeType: fin.ChargeTypeRegularAssessment, AmountCents: 5000, DueDate: newer},
		{ID: olderID, ChargeType: fin.ChargeTypeRegularAssessment, AmountCents: 5000, DueDate: older},
	}
	ruling := fin.AllocationRuling{
		PriorityOrder: []fin.ChargeType{fin.ChargeTypeRegularAssessment},
		AcceptPartial: true,
	}

	allocs, _ := fin.Allocate(charges, 7000, ruling)

	require.Len(t, allocs, 2)
	assert.Equal(t, olderID, allocs[0].ChargeID, "older assessment allocated first")
	assert.Equal(t, int64(5000), allocs[0].AllocatedCents)
	assert.Equal(t, newerID, allocs[1].ChargeID)
	assert.Equal(t, int64(2000), allocs[1].AllocatedCents)
}

func TestAllocate_FrozenChargesExcluded(t *testing.T) {
	frozenID := uuid.New()
	activeID := uuid.New()

	charges := []fin.OutstandingCharge{
		{ID: frozenID, ChargeType: fin.ChargeTypeRegularAssessment, AmountCents: 8000, DueDate: time.Now().AddDate(0, -3, 0)},
		{ID: activeID, ChargeType: fin.ChargeTypeRegularAssessment, AmountCents: 6000, DueDate: time.Now()},
	}
	ruling := fin.AllocationRuling{
		PriorityOrder:  []fin.ChargeType{fin.ChargeTypeRegularAssessment},
		FrozenChargeIDs: []uuid.UUID{frozenID},
		AcceptPartial:  true,
	}

	allocs, _ := fin.Allocate(charges, 8000, ruling)

	require.Len(t, allocs, 1)
	assert.Equal(t, activeID, allocs[0].ChargeID, "frozen charge should be skipped")
	assert.Equal(t, int64(6000), allocs[0].AllocatedCents)
}

func TestAllocate_NoCharges(t *testing.T) {
	ruling := fin.AllocationRuling{
		PriorityOrder: []fin.ChargeType{fin.ChargeTypeRegularAssessment},
		AcceptPartial: true,
	}

	allocs, creditCents := fin.Allocate(nil, 5000, ruling)

	assert.Empty(t, allocs)
	assert.Equal(t, int64(5000), creditCents)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd backend && go test ./internal/fin/... -run TestAllocate -short -count=1`
Expected: FAIL — `Allocate` not defined.

- [ ] **Step 3: Write the allocation engine**

```go
package fin

import (
	"sort"
	"time"

	"github.com/google/uuid"
)

// OutstandingCharge represents a charge eligible for payment allocation.
type OutstandingCharge struct {
	ID          uuid.UUID
	ChargeType  ChargeType
	AmountCents int64
	DueDate     time.Time
}

// AllocationRuling is the typed ruling from the policy engine, decoded from
// the Resolution.Ruling JSON.
type AllocationRuling struct {
	PriorityOrder   []ChargeType `json:"priority_order"`
	FrozenChargeIDs []uuid.UUID  `json:"frozen_charge_ids"`
	FrozenCutoffDate *time.Time  `json:"frozen_cutoff_date,omitempty"`
	AcceptPartial   bool         `json:"accept_partial"`
	CreditHandling  string       `json:"credit_handling,omitempty"`
	EstoppelOverride bool        `json:"estoppel_override,omitempty"`
	TrusteeOverride  bool        `json:"trustee_override,omitempty"`
}

// AllocationResult is one line of a payment allocation — how much of the
// payment goes to which charge.
type AllocationResult struct {
	ChargeID       uuid.UUID
	ChargeType     ChargeType
	AllocatedCents int64
}

// Allocate is a pure function that distributes a payment amount across
// outstanding charges according to the ruling. It returns the allocation
// results and any remaining credit (overpayment).
func Allocate(charges []OutstandingCharge, paymentCents int64, ruling AllocationRuling) ([]AllocationResult, int64) {
	if len(charges) == 0 {
		return nil, paymentCents
	}

	// Build frozen set for O(1) lookup.
	frozen := make(map[uuid.UUID]bool, len(ruling.FrozenChargeIDs))
	for _, id := range ruling.FrozenChargeIDs {
		frozen[id] = true
	}

	// Filter out frozen charges.
	var eligible []OutstandingCharge
	for _, c := range charges {
		if frozen[c.ID] {
			continue
		}
		if ruling.FrozenCutoffDate != nil && c.DueDate.Before(*ruling.FrozenCutoffDate) {
			continue
		}
		eligible = append(eligible, c)
	}

	// Build priority index: charge type → position.
	priority := make(map[ChargeType]int, len(ruling.PriorityOrder))
	for i, ct := range ruling.PriorityOrder {
		priority[ct] = i
	}

	// Sort: primary by priority tier, secondary by due date (FIFO).
	sort.Slice(eligible, func(i, j int) bool {
		pi := priority[eligible[i].ChargeType]
		pj := priority[eligible[j].ChargeType]
		if pi != pj {
			return pi < pj
		}
		return eligible[i].DueDate.Before(eligible[j].DueDate)
	})

	// Walk and allocate.
	remaining := paymentCents
	var results []AllocationResult

	for _, charge := range eligible {
		if remaining <= 0 {
			break
		}
		alloc := charge.AmountCents
		if alloc > remaining {
			alloc = remaining
		}
		results = append(results, AllocationResult{
			ChargeID:       charge.ID,
			ChargeType:     charge.ChargeType,
			AllocatedCents: alloc,
		})
		remaining -= alloc
	}

	return results, remaining
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd backend && go test ./internal/fin/... -run TestAllocate -short -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/internal/fin/allocation.go backend/internal/fin/allocation_test.go
git commit -m "feat: add pure allocation engine with priority order and FIFO (issue #62)"
```

---

### Task 13: Payment Allocation Repository Methods

**Files:**
- Modify: `backend/internal/fin/payment_repository.go`
- Modify: `backend/internal/fin/payment_postgres.go`

- [ ] **Step 1: Add allocation methods to PaymentRepository interface**

Add to `payment_repository.go` after the Payment Methods section:

```go
	// ── Payment Allocations ──────────────────────────────────────────────────

	// CreatePaymentAllocation inserts a new allocation record.
	CreatePaymentAllocation(ctx context.Context, a *PaymentAllocation) (*PaymentAllocation, error)

	// ListAllocationsByPayment returns all allocations for the given payment.
	ListAllocationsByPayment(ctx context.Context, paymentID uuid.UUID) ([]PaymentAllocation, error)
```

- [ ] **Step 2: Add PostgreSQL implementations to payment_postgres.go**

Add to the end of `payment_postgres.go`:

```go
// ─── Payment Allocations ─────────────────────────────────────────────────────

func (r *PostgresPaymentRepository) CreatePaymentAllocation(ctx context.Context, a *PaymentAllocation) (*PaymentAllocation, error) {
	const q = `
		INSERT INTO payment_allocations (
			payment_id, charge_type, charge_id, allocated_cents, resolution_id,
			estoppel_id, reversed_at, reversed_by_id
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, payment_id, charge_type, charge_id, allocated_cents, resolution_id,
		          estoppel_id, reversed_at, reversed_by_id, created_at`

	row := r.db.QueryRow(ctx, q,
		a.PaymentID, a.ChargeType, a.ChargeID, a.AllocatedCents,
		a.ResolutionID, a.EstoppelID, a.ReversedAt, a.ReversedByID,
	)

	var out PaymentAllocation
	err := row.Scan(
		&out.ID, &out.PaymentID, &out.ChargeType, &out.ChargeID,
		&out.AllocatedCents, &out.ResolutionID, &out.EstoppelID,
		&out.ReversedAt, &out.ReversedByID, &out.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("fin: CreatePaymentAllocation: %w", err)
	}
	return &out, nil
}

func (r *PostgresPaymentRepository) ListAllocationsByPayment(ctx context.Context, paymentID uuid.UUID) ([]PaymentAllocation, error) {
	const q = `
		SELECT id, payment_id, charge_type, charge_id, allocated_cents, resolution_id,
		       estoppel_id, reversed_at, reversed_by_id, created_at
		FROM payment_allocations
		WHERE payment_id = $1
		ORDER BY created_at`

	rows, err := r.db.Query(ctx, q, paymentID)
	if err != nil {
		return nil, fmt.Errorf("fin: ListAllocationsByPayment: %w", err)
	}
	defer rows.Close()

	var results []PaymentAllocation
	for rows.Next() {
		var a PaymentAllocation
		if err := rows.Scan(
			&a.ID, &a.PaymentID, &a.ChargeType, &a.ChargeID,
			&a.AllocatedCents, &a.ResolutionID, &a.EstoppelID,
			&a.ReversedAt, &a.ReversedByID, &a.CreatedAt,
		); err != nil {
			return nil, err
		}
		results = append(results, a)
	}
	if results == nil {
		return []PaymentAllocation{}, nil
	}
	return results, nil
}
```

- [ ] **Step 3: Verify compilation**

Run: `cd backend && go build ./internal/fin/...`
Expected: Compilation errors in test mocks (missing new interface methods). This is expected — we'll fix them in the next task.

- [ ] **Step 4: Update mock in service_test.go to satisfy the new interface**

Add to `mockPaymentRepo` in `service_test.go`:

```go
func (m *mockPaymentRepo) CreatePaymentAllocation(_ context.Context, a *fin.PaymentAllocation) (*fin.PaymentAllocation, error) {
	a.ID = uuid.New()
	a.CreatedAt = time.Now()
	return a, nil
}

func (m *mockPaymentRepo) ListAllocationsByPayment(_ context.Context, paymentID uuid.UUID) ([]fin.PaymentAllocation, error) {
	return []fin.PaymentAllocation{}, nil
}
```

Do the same for any other mock implementations of `PaymentRepository` in other test files (check `handler_budget_test.go`, `handler_payment_test.go`).

- [ ] **Step 5: Verify compilation now passes**

Run: `cd backend && go build ./internal/fin/...`
Expected: No errors.

- [ ] **Step 6: Run existing tests to confirm no regressions**

Run: `cd backend && go test ./internal/fin/... -short -count=1`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add backend/internal/fin/payment_repository.go backend/internal/fin/payment_postgres.go backend/internal/fin/service_test.go
git commit -m "feat: add payment allocation repository methods (issue #62)"
```

---

### Task 14: Wire Registry into FinService and Refactor RecordPayment

**Files:**
- Modify: `backend/internal/fin/service.go`
- Modify: `backend/internal/fin/service_test.go`

- [ ] **Step 1: Write the failing test for allocation-aware RecordPayment**

Add to `service_test.go`:

```go
func TestRecordPayment_AllocatesToOldestAssessment(t *testing.T) {
	svc, mockAssessments, mockPayments, _, _, _ := newTestService()
	ctx := context.Background()
	orgID := uuid.New()
	userID := uuid.New()
	unitID := uuid.New()

	// Seed two assessments — older should be allocated first.
	older := fin.Assessment{
		ID: uuid.New(), OrgID: orgID, UnitID: unitID,
		AmountCents: 10000, DueDate: time.Now().AddDate(0, -2, 0),
		CurrencyCode: "USD",
	}
	newer := fin.Assessment{
		ID: uuid.New(), OrgID: orgID, UnitID: unitID,
		AmountCents: 5000, DueDate: time.Now().AddDate(0, -1, 0),
		CurrencyCode: "USD",
	}
	mockAssessments.assessments = append(mockAssessments.assessments, older, newer)

	payment, err := svc.RecordPayment(ctx, orgID, userID, fin.CreatePaymentRequest{
		UnitID:      unitID,
		AmountCents: 12000,
	})
	require.NoError(t, err)
	require.NotNil(t, payment)
	assert.Equal(t, fin.PaymentStatusCompleted, fin.PaymentStatus(payment.Status))
}
```

- [ ] **Step 2: Run test to verify it fails or needs updating**

Run: `cd backend && go test ./internal/fin/... -run TestRecordPayment_Allocates -short -count=1`
Expected: May fail due to missing registry dependency.

- [ ] **Step 3: Add policy.Registry to FinService**

In `service.go`, add `registry` field and update constructor:

Add import: `"github.com/quorant/quorant/internal/platform/policy"`

Update the struct:
```go
type FinService struct {
	assessments AssessmentRepository
	payments    PaymentRepository
	budgets     BudgetRepository
	funds       FundRepository
	collections CollectionRepository
	gl          *GLService
	policy      ai.PolicyResolver
	compliance  ai.ComplianceResolver
	registry    *policy.Registry
	logger      *slog.Logger
	uowFactory  *db.UnitOfWorkFactory
}
```

Update `NewFinService` to accept `registry *policy.Registry` and add it to the parameter list and struct initialization. Update the existing `newTestService()` helper in `service_test.go` to pass `nil` or a `policy.NewNoopRegistryDefault()`.

- [ ] **Step 4: Run all tests to confirm no regressions**

Run: `cd backend && go test ./internal/fin/... -short -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/internal/fin/service.go backend/internal/fin/service_test.go
git commit -m "feat: add policy.Registry to FinService and allocation-aware RecordPayment (issue #62)"
```

---

### Task 15: Wire Registry in main.go

**Files:**
- Modify: `backend/cmd/quorant-api/main.go`

- [ ] **Step 1: Add policy Registry construction and pass to FinService**

In the service wiring section of `main.go`:

```go
// Policy engine
policyRecordRepo := policy.NewPostgresPolicyRecordRepository(pool)
policyResolutionRepo := policy.NewPostgresResolutionRepository(pool)
policyRegistry := policy.NewRegistry(policyRecordRepo, policyResolutionRepo, policyResolver, logger)
```

Pass `policyRegistry` to `NewFinService`.

Add import: `"github.com/quorant/quorant/internal/platform/policy"`

- [ ] **Step 2: Verify compilation**

Run: `cd backend && go build ./cmd/quorant-api/...`
Expected: No errors.

- [ ] **Step 3: Commit**

```bash
git add backend/cmd/quorant-api/main.go
git commit -m "feat: wire policy.Registry into quorant-api startup (issue #62)"
```

---

### Task 16: Jurisdiction Seed Data

**Files:**
- Create: `backend/migrations/20260409000031_payment_allocation_seed.sql`

- [ ] **Step 1: Write seed migration**

```sql
-- 20260409000031_payment_allocation_seed.sql
-- Seed jurisdiction-level payment allocation priority orders.

INSERT INTO policy_records (scope, jurisdiction, category, key, value, priority_hint, statute_reference, effective_date, is_active)
VALUES
    ('jurisdiction', 'CA', 'payment_allocation', 'priority_order',
     '{"order":["regular_assessment","special_assessment","collection_cost","attorney_fee","late_fee","interest"]}',
     'state', 'CA Civil Code 5655(a)', '2025-01-01', true),

    ('jurisdiction', 'TX', 'payment_allocation', 'priority_order',
     '{"order":["regular_assessment","special_assessment","attorney_fee","fine"]}',
     'state', 'TX Property Code 209.0063(a)', '2025-01-01', true),

    ('jurisdiction', 'FL', 'payment_allocation', 'priority_order',
     '{"order":["interest","late_fee","collection_cost","attorney_fee","regular_assessment","special_assessment"]}',
     'state', 'FL Statute 720.3085(3)(b)', '2025-01-01', true),

    ('jurisdiction', 'NV', 'payment_allocation', 'priority_order',
     '{"order":["regular_assessment","special_assessment","late_fee","interest","collection_cost","attorney_fee"],"super_lien_months":9}',
     'state', 'NRS 116.3116', '2025-01-01', true),

    ('jurisdiction', 'CO', 'payment_allocation', 'priority_order',
     '{"order":["regular_assessment","special_assessment","late_fee","interest","collection_cost","attorney_fee"],"super_lien_months":6}',
     'state', 'CRS 38-33.3-316(2)', '2025-01-01', true),

    ('jurisdiction', 'CT', 'payment_allocation', 'priority_order',
     '{"order":["regular_assessment","special_assessment","late_fee","interest","collection_cost","attorney_fee"],"super_lien_months":6}',
     'state', 'CGS 47-258(m)', '2025-01-01', true),

    ('jurisdiction', 'DEFAULT', 'payment_allocation', 'priority_order',
     '{"order":["regular_assessment","special_assessment","late_fee","interest","collection_cost","attorney_fee","fine"]}',
     'state', NULL, '2025-01-01', true);
```

- [ ] **Step 2: Commit**

```bash
git add backend/migrations/20260409000031_payment_allocation_seed.sql
git commit -m "feat: seed jurisdiction payment allocation priority orders (issue #62)"
```

---

### Task 17: Final Verification

- [ ] **Step 1: Run all unit tests**

Run: `cd backend && go test ./... -short -count=1`
Expected: PASS

- [ ] **Step 2: Run go vet**

Run: `cd backend && go vet ./...`
Expected: No issues.

- [ ] **Step 3: Verify build**

Run: `cd backend && go build ./cmd/quorant-api/... && go build ./cmd/quorant-worker/...`
Expected: No errors.

- [ ] **Step 4: Commit any final fixes if needed**
