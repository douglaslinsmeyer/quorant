# Certiorari Policy Pattern Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the certiorari pattern so that `Registry.Resolve()` returns `(nil, nil)` when no policy records exist (zero-cost decline), walks the ltree org hierarchy to gather policies from all ancestors, and is available to every domain module via the shared `Dependencies` struct.

**Architecture:** Refactor the existing `Registry.Resolve()` to gather-before-descriptor-lookup with nil-decline semantics. Rewrite `GatherForResolution` SQL to use ltree `<@` ancestor queries. Wire `OrgJurisdictionLookup` and add `PolicyRegistry` to `app.Dependencies`.

**Tech Stack:** Go, PostgreSQL (ltree, pgx), testify

**Spec:** `docs/superpowers/specs/2026-04-11-certiorari-policy-pattern-design.md`

---

## File Structure

| File | Action | Responsibility |
|------|--------|---------------|
| `backend/internal/platform/policy/registry.go` | Modify | Restructure `Resolve()` with certiorari semantics |
| `backend/internal/platform/policy/registry_test.go` | Modify | Add certiorari-specific tests, update unregistered category test |
| `backend/internal/platform/policy/policy_postgres.go` | Modify | Rewrite `GatherForResolution` SQL with ltree ancestor walk |
| `backend/internal/platform/policy/jurisdiction_adapter.go` | Create | `OrgJurisdictionAdapter` implementing `OrgJurisdictionLookup` |
| `backend/internal/platform/policy/policy_postgres_integration_test.go` | Create | Integration tests for ltree gather + jurisdiction adapter |
| `backend/internal/platform/app/module.go` | Modify | Add `PolicyRegistry` to `Dependencies` |
| `backend/cmd/quorant-api/main.go` | Modify | Wire `OrgJurisdictionAdapter`, set `PolicyRegistry` in deps |
| `backend/internal/fin/policy_descriptors.go` | Create | `RegisterPolicyDescriptors` for 6 fin categories |
| `backend/internal/fin/policy_descriptors_test.go` | Create | Tests for descriptor registration |

---

### Task 1: Certiorari nil-decline in `Resolve()` — tests

**Files:**
- Modify: `backend/internal/platform/policy/registry_test.go`

- [ ] **Step 1: Write failing test — Resolve returns nil when no records gathered**

Add this test to `registry_test.go`. It creates a registry with an empty record repo (no records match) and a registered descriptor. Resolve should return `(nil, nil)` instead of calling Tier 2 AI.

```go
func TestRegistry_Resolve_NoRecords_ReturnsNil(t *testing.T) {
	ctx := context.Background()
	orgID := uuid.New()

	// Empty record repo — GatherForResolution returns no records.
	recordRepo := &stubRecordRepo{records: []policy.PolicyRecord{}}
	aiResolver := &stubPolicyResolver{
		result: &ai.ResolutionResult{
			Resolution: json.RawMessage(`{"late_fee": 50}`),
			Reasoning:  "should not be called",
			Confidence: 0.95,
		},
	}

	reg := policy.NewRegistry(recordRepo, nil, aiResolver, nil, testLogger())
	err := reg.Register("fee_schedule", newTestDescriptor(nil))
	require.NoError(t, err)

	res, err := reg.Resolve(ctx, orgID, nil, "fee_schedule")
	assert.NoError(t, err)
	assert.Nil(t, res, "Resolve should return nil when no records are gathered (certiorari declined)")
}
```

- [ ] **Step 2: Write failing test — Resolve returns nil for unregistered category with no records**

This replaces the old "UnregisteredCategory" test behavior. When a category has no descriptor AND no records, `Resolve` should return `(nil, nil)` — not an error.

```go
func TestRegistry_Resolve_UnregisteredCategory_NoRecords_ReturnsNil(t *testing.T) {
	ctx := context.Background()
	orgID := uuid.New()

	// Empty record repo, no descriptors registered.
	recordRepo := &stubRecordRepo{records: []policy.PolicyRecord{}}
	reg := policy.NewRegistry(recordRepo, nil, nil, nil, testLogger())

	res, err := reg.Resolve(ctx, orgID, nil, "nonexistent_category")
	assert.NoError(t, err, "unregistered category with no records should not error")
	assert.Nil(t, res, "unregistered category with no records should return nil")
}
```

- [ ] **Step 3: Write failing test — Resolve returns nil when records exist but no descriptor registered**

Records exist in the DB for a category, but no module has registered a descriptor for it. Without a descriptor there's no PromptTemplate for Tier 2, so Resolve should return `(nil, nil)`.

```go
func TestRegistry_Resolve_RecordsExist_NoDescriptor_ReturnsNil(t *testing.T) {
	ctx := context.Background()
	orgID := uuid.New()

	recordRepo := &stubRecordRepo{records: []policy.PolicyRecord{
		{
			ID:           uuid.New(),
			Scope:        "org",
			Category:     "orphan_category",
			Key:          "some_policy",
			Value:        json.RawMessage(`{"rule": "value"}`),
			PriorityHint: "board_policy",
			IsActive:     true,
		},
	}}

	// No descriptor registered for "orphan_category".
	reg := policy.NewRegistry(recordRepo, nil, nil, nil, testLogger())

	res, err := reg.Resolve(ctx, orgID, nil, "orphan_category")
	assert.NoError(t, err, "records with no descriptor should not error")
	assert.Nil(t, res, "records with no descriptor should return nil (no PromptTemplate for Tier 2)")
}
```

- [ ] **Step 4: Write failing test — Verify AI is NOT called when no records gathered**

```go
func TestRegistry_Resolve_NoRecords_AINotCalled(t *testing.T) {
	ctx := context.Background()
	orgID := uuid.New()

	recordRepo := &stubRecordRepo{records: []policy.PolicyRecord{}}

	var aiCalled bool
	aiResolver := &countingPolicyResolver{onCall: func() { aiCalled = true }}

	reg := policy.NewRegistry(recordRepo, nil, aiResolver, nil, testLogger())
	err := reg.Register("fee_schedule", newTestDescriptor(nil))
	require.NoError(t, err)

	_, _ = reg.Resolve(ctx, orgID, nil, "fee_schedule")
	assert.False(t, aiCalled, "AI resolver should NOT be called when no records are gathered")
}
```

Add the `countingPolicyResolver` stub just above this test:

```go
// countingPolicyResolver tracks whether QueryPolicy was called.
type countingPolicyResolver struct {
	onCall func()
}

func (c *countingPolicyResolver) GetPolicy(_ context.Context, _ uuid.UUID, _ string) (*ai.PolicyResult, error) {
	return nil, nil
}

func (c *countingPolicyResolver) QueryPolicy(_ context.Context, _ uuid.UUID, _ string, _ ai.QueryContext) (*ai.ResolutionResult, error) {
	if c.onCall != nil {
		c.onCall()
	}
	return &ai.ResolutionResult{
		Resolution: json.RawMessage(`{}`),
		Reasoning:  "test",
		Confidence: 0.95,
	}, nil
}
```

- [ ] **Step 5: Update the existing `TestRegistry_Resolve_UnregisteredCategory` test**

The old test asserts `require.Error(t, err)`. Update it to match the new nil-decline behavior. Replace the entire `TestRegistry_Resolve_UnregisteredCategory` function:

```go
func TestRegistry_Resolve_UnregisteredCategory(t *testing.T) {
	ctx := context.Background()
	orgID := uuid.New()

	// With a record repo that returns no records for this category.
	recordRepo := &stubRecordRepo{records: []policy.PolicyRecord{}}
	reg := policy.NewRegistry(recordRepo, nil, nil, nil, testLogger())

	res, err := reg.Resolve(ctx, orgID, nil, "nonexistent_category")
	assert.NoError(t, err, "unregistered category should not error under certiorari semantics")
	assert.Nil(t, res, "should return nil when no records exist")
}
```

- [ ] **Step 6: Run tests to verify they fail**

Run: `cd backend && go test ./internal/platform/policy/... -short -count=1 -run "TestRegistry_Resolve_NoRecords|TestRegistry_Resolve_UnregisteredCategory|TestRegistry_Resolve_RecordsExist_NoDescriptor|TestRegistry_Resolve_AINotCalled" -v`

Expected: The new tests fail because `Resolve()` still errors on unregistered categories and still calls Tier 2 when no records exist. The updated `UnregisteredCategory` test also fails because it now expects no error but gets one.

---

### Task 2: Certiorari nil-decline in `Resolve()` — implementation

**Files:**
- Modify: `backend/internal/platform/policy/registry.go`

- [ ] **Step 1: Restructure `Resolve()` with certiorari semantics**

Replace the current `Resolve` method (lines 132-256 of `registry.go`) with the certiorari flow:

```go
// Resolve executes the two-tier policy resolution pipeline for the given
// category. Tier 1 gathers applicable policy records from the database via an
// ltree ancestor walk. If no records are gathered, the engine declines
// jurisdiction and returns (nil, nil) — the certiorari pattern. If records
// exist but no descriptor is registered for the category, Resolve also returns
// (nil, nil) because Tier 2 requires a PromptTemplate. When both records and a
// descriptor are present, Tier 2 sends them to the AI resolver for precedence
// reasoning. The result is persisted (when a resolutions repo is available) and
// returned.
func (r *Registry) Resolve(ctx context.Context, orgID uuid.UUID, unitID *uuid.UUID, category string) (*Resolution, error) {
	// Tier 1: gather applicable policy records.
	jurisdiction := r.lookupJurisdiction(ctx, orgID)

	records, err := r.records.GatherForResolution(ctx, category, jurisdiction, orgID, unitID)
	if err != nil {
		return nil, fmt.Errorf("policy: gather records for %q: %w", category, err)
	}

	// Certiorari: no records gathered — decline jurisdiction.
	if len(records) == 0 {
		return nil, nil
	}

	// Descriptor lookup. Without a descriptor we have no PromptTemplate for
	// Tier 2, so decline.
	r.mu.RLock()
	desc, exists := r.descriptors[category]
	r.mu.RUnlock()

	if !exists {
		if r.logger != nil {
			r.logger.InfoContext(ctx, "policy records exist but no descriptor registered, declining",
				"category", category,
				"org_id", orgID,
				"record_count", len(records),
			)
		}
		return nil, nil
	}

	// Build policy ID slice for audit trail.
	policyIDs := make([]uuid.UUID, len(records))
	for i, rec := range records {
		policyIDs[i] = rec.ID
	}

	// Tier 2: AI resolution.
	var (
		ruling       json.RawMessage
		reasoning    string
		confidence   float64
		modelID      string
		status       string
		reviewStatus string
	)

	ruling, reasoning, confidence, modelID, err = r.callTier2(ctx, orgID, desc, records)
	if err != nil {
		// AI unavailable -- hold for human review.
		if r.logger != nil {
			r.logger.WarnContext(ctx, "tier 2 AI call failed, holding resolution",
				"category", category,
				"org_id", orgID,
				"error", err,
			)
		}
		status = "held"
		reviewStatus = "ai_unavailable"
		confidence = 0
		ruling = nil
		reasoning = ""
	} else {
		threshold := desc.DefaultThreshold
		if threshold == 0 {
			threshold = 0.80
		}

		if confidence < threshold {
			status = "held"
			reviewStatus = "pending_review"
		} else {
			status = "approved"
			reviewStatus = "auto_approved"
		}
	}

	resID := uuid.New()
	res := &Resolution{
		ID:         resID,
		Status:     status,
		Ruling:     ruling,
		Reasoning:  reasoning,
		Confidence: confidence,
	}

	// Build source policy references from the gathered records.
	refs := make([]PolicyReference, len(records))
	for i, rec := range records {
		refs[i] = PolicyReference{
			ID:           rec.ID,
			Scope:        rec.Scope,
			Category:     rec.Category,
			Key:          rec.Key,
			PriorityHint: rec.PriorityHint,
			StatuteRef:   rec.StatuteRef,
		}
	}
	res.SourcePolicies = refs

	// Persist if resolutions repo is available.
	if r.resolutions != nil {
		record := &ResolutionRecord{
			ID:             resID,
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
		if _, persistErr := r.resolutions.CreateResolution(ctx, record); persistErr != nil {
			if r.logger != nil {
				r.logger.ErrorContext(ctx, "failed to persist resolution",
					"resolution_id", resID,
					"error", persistErr,
				)
			}
		}
	}

	// Invoke OnHold callback if held.
	if res.Held() && desc.OnHold != nil {
		if holdErr := desc.OnHold(ctx, res); holdErr != nil {
			if r.logger != nil {
				r.logger.ErrorContext(ctx, "OnHold callback failed",
					"category", category,
					"resolution_id", resID,
					"error", holdErr,
				)
			}
		}
	}

	return res, nil
}
```

- [ ] **Step 2: Run all policy tests**

Run: `cd backend && go test ./internal/platform/policy/... -short -count=1 -v`

Expected: All tests pass, including the new certiorari tests and the updated unregistered category test. The existing `Resolve_AutoApproved`, `Resolve_Held`, and `Resolve_AIUnavailable` tests continue to pass unchanged because they set up records in their stubRecordRepo.

- [ ] **Step 3: Run fin engine tests to verify backward compatibility**

Run: `cd backend && go test ./internal/fin/... -short -count=1 -v`

Expected: All tests pass. The existing fin tests use `stubPolicyRecordRepo{records: nil}` which causes `GatherForResolution` to return `nil` (treated as empty slice by the stub). Tests that register a descriptor AND set up the stub AI resolver still work because the stub `GatherForResolution` returns `s.records` directly — when `records` is `nil`, `len(nil) == 0` → certiorari declines → returns nil. BUT tests like `TestGaapEngine_ValidateTransaction_LateFee_WithinCap` rely on the registry returning a resolution. Check: the `newRegistryWithCap` helper in `engine_fee_cap_test.go` passes `records: nil` to the stub. Under certiorari, that means `len(records) == 0` → nil return → the fee cap is not enforced → the test still passes (no error) but for the wrong reason.

**If fin tests behave differently:** The `stubPolicyRecordRepo` returns `s.records` which is `nil` when initialized as `records: nil`. Under certiorari, `len(nil) == 0` is true, so Resolve returns nil. The callers in `engine_shared.go` treat nil resolution as "no policy configured → allow". So fee cap tests where the cap should reject a transaction will fail. In that case, update the stubs in Task 5 after verifying.

- [ ] **Step 4: Commit**

```bash
git add backend/internal/platform/policy/registry.go backend/internal/platform/policy/registry_test.go
git commit -m "feat(policy): implement certiorari nil-decline semantics in Resolve()

Resolve() now gathers records first and returns (nil, nil) when no
records exist (certiorari declined). Descriptor lookup is deferred
and also returns nil if no descriptor is registered. Removes the
'category not registered' error."
```

---

### Task 3: Fix fin test stubs for certiorari compatibility

The fin engine tests (`engine_fee_cap_test.go`, `engine_payment_strategy_test.go`) use `stubPolicyRecordRepo{records: nil}`. Under certiorari, `len(nil) == 0` causes `Resolve` to return nil, which means the policy ruling is never applied. Tests that expect a ruling to be enforced will break.

**Files:**
- Modify: `backend/internal/fin/engine_fee_cap_test.go`
- Modify: `backend/internal/fin/engine_payment_strategy_test.go`

- [ ] **Step 1: Update `newRegistryWithCap` to supply a dummy policy record**

In `engine_fee_cap_test.go`, change the `newRegistryWithCap` helper so the stub has at least one record matching the category. This ensures `Resolve` proceeds past the certiorari gate to Tier 2:

```go
func newRegistryWithCap(t *testing.T, category string, rulingJSON json.RawMessage) *policy.Registry {
	t.Helper()
	registry := policy.NewRegistry(
		&stubPolicyRecordRepo{records: []policy.PolicyRecord{
			{
				ID:       uuid.New(),
				Scope:    "org",
				Category: category,
				Key:      "test_policy",
				Value:    json.RawMessage(`{}`),
				IsActive: true,
			},
		}},
		nil,
		&stubAIPolicyResolver{ruling: rulingJSON, confidence: 0.95},
		nil,
		nil,
	)
	err := registry.Register(category, policy.OperationDescriptor{
		Category:         category,
		PromptTemplate:   "Given these policies: {{.Policies}}",
		DefaultThreshold: 0.80,
	})
	require.NoError(t, err)
	return registry
}
```

- [ ] **Step 2: Update the payment strategy registry setup to supply a dummy record**

In `engine_payment_strategy_test.go`, update `TestPaymentApplicationStrategy_RegistryWithPriorityOrder` (around line 63):

```go
	registry := policy.NewRegistry(
		&stubPolicyRecordRepo{records: []policy.PolicyRecord{
			{
				ID:       uuid.New(),
				Scope:    "org",
				Category: "payment_allocation_rules",
				Key:      "test_policy",
				Value:    json.RawMessage(`{}`),
				IsActive: true,
			},
		}},
		nil,
		&stubAIPolicyResolver{ruling: rulingJSON, confidence: 0.95},
		nil,
		nil,
	)
```

Also update `TestPaymentApplicationStrategy_DesignatedInvoice_OverridesPolicy` (around line 110) with the same pattern if it creates a registry with `records: nil`.

- [ ] **Step 3: Run fin tests**

Run: `cd backend && go test ./internal/fin/... -short -count=1 -v`

Expected: All pass. The stub now provides a record so `Resolve` reaches Tier 2.

- [ ] **Step 4: Commit**

```bash
git add backend/internal/fin/engine_fee_cap_test.go backend/internal/fin/engine_payment_strategy_test.go
git commit -m "test(fin): update policy stubs to provide records for certiorari gate

Under certiorari semantics, Resolve() returns nil when no records
exist. Update test stubs to include at least one dummy PolicyRecord
so the registry proceeds to Tier 2 AI resolution."
```

---

### Task 4: ltree hierarchical `GatherForResolution` — tests

**Files:**
- Create: `backend/internal/platform/policy/policy_postgres_integration_test.go`

These are integration tests requiring Docker (Postgres with ltree). They verify the ancestor walk.

- [ ] **Step 1: Write integration test for ltree ancestor gather**

```go
//go:build integration

package policy_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/platform/policy"
	"github.com/quorant/quorant/internal/platform/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGatherForResolution_LtreeAncestorWalk(t *testing.T) {
	pool := testutil.IntegrationDB(t)
	ctx := context.Background()
	repo := policy.NewPostgresPolicyRecordRepository(pool)

	// Create org hierarchy: firm -> subfirm -> hoa
	firmID := uuid.New()
	subfirmID := uuid.New()
	hoaID := uuid.New()

	// Insert organizations with ltree paths.
	_, err := pool.Exec(ctx, `
		INSERT INTO organizations (id, name, slug, type, path, created_by)
		VALUES ($1, 'Acme Firm', 'acme', 'firm', 'acme', $4),
		       ($2, 'Southeast Region', 'southeast', 'firm', 'acme.southeast', $4),
		       ($3, 'Sunset HOA', 'sunset', 'hoa', 'acme.southeast.sunset', $4)`,
		firmID, subfirmID, hoaID, uuid.New(),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM policy_records WHERE org_id IN ($1, $2, $3)`, firmID, subfirmID, hoaID)
		pool.Exec(ctx, `DELETE FROM organizations WHERE id IN ($1, $2, $3)`, firmID, subfirmID, hoaID)
	})

	now := time.Now()
	category := "late_fee_cap"

	// Insert policies at firm level, subfirm level, and hoa level.
	_, err = repo.CreateRecord(ctx, &policy.PolicyRecord{
		Scope: "org", OrgID: &firmID, Category: category,
		Key: "firm_cap", Value: json.RawMessage(`{"max_cents": 10000}`),
		PriorityHint: "board_policy", EffectiveDate: now.AddDate(0, -1, 0), IsActive: true,
	})
	require.NoError(t, err)

	_, err = repo.CreateRecord(ctx, &policy.PolicyRecord{
		Scope: "org", OrgID: &subfirmID, Category: category,
		Key: "region_cap", Value: json.RawMessage(`{"max_cents": 7500}`),
		PriorityHint: "board_policy", EffectiveDate: now.AddDate(0, -1, 0), IsActive: true,
	})
	require.NoError(t, err)

	_, err = repo.CreateRecord(ctx, &policy.PolicyRecord{
		Scope: "org", OrgID: &hoaID, Category: category,
		Key: "hoa_cap", Value: json.RawMessage(`{"max_cents": 5000}`),
		PriorityHint: "board_policy", EffectiveDate: now.AddDate(0, -1, 0), IsActive: true,
	})
	require.NoError(t, err)

	// Gather for the HOA — should return all three levels.
	records, err := repo.GatherForResolution(ctx, category, "DEFAULT", hoaID, nil)
	require.NoError(t, err)

	assert.Len(t, records, 3, "should gather policies from firm, subfirm, and hoa")

	// Verify ordering: firm (nlevel=1) first, then subfirm (nlevel=2), then hoa (nlevel=3).
	keys := make([]string, len(records))
	for i, r := range records {
		keys[i] = r.Key
	}
	assert.Equal(t, []string{"firm_cap", "region_cap", "hoa_cap"}, keys,
		"records should be ordered by ltree depth (root first)")
}

func TestGatherForResolution_IncludesJurisdictionAndUnit(t *testing.T) {
	pool := testutil.IntegrationDB(t)
	ctx := context.Background()
	repo := policy.NewPostgresPolicyRecordRepository(pool)

	orgID := uuid.New()
	unitID := uuid.New()
	jurisdiction := "FL"
	category := "late_fee_cap"
	now := time.Now()

	_, err := pool.Exec(ctx, `
		INSERT INTO organizations (id, name, slug, type, path, jurisdiction, created_by)
		VALUES ($1, 'Test HOA', 'test-hoa', 'hoa', 'test_hoa', $2, $3)`,
		orgID, jurisdiction, uuid.New(),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM policy_records WHERE org_id = $1 OR unit_id = $2 OR jurisdiction = $3`, orgID, unitID, jurisdiction)
		pool.Exec(ctx, `DELETE FROM organizations WHERE id = $1`, orgID)
	})

	// Jurisdiction policy
	_, err = repo.CreateRecord(ctx, &policy.PolicyRecord{
		Scope: "jurisdiction", Jurisdiction: &jurisdiction, Category: category,
		Key: "state_cap", Value: json.RawMessage(`{"max_cents": 25000}`),
		PriorityHint: "state", EffectiveDate: now.AddDate(0, -1, 0), IsActive: true,
	})
	require.NoError(t, err)

	// Org policy
	_, err = repo.CreateRecord(ctx, &policy.PolicyRecord{
		Scope: "org", OrgID: &orgID, Category: category,
		Key: "org_cap", Value: json.RawMessage(`{"max_cents": 5000}`),
		PriorityHint: "board_policy", EffectiveDate: now.AddDate(0, -1, 0), IsActive: true,
	})
	require.NoError(t, err)

	// Unit policy
	_, err = repo.CreateRecord(ctx, &policy.PolicyRecord{
		Scope: "unit", UnitID: &unitID, Category: category,
		Key: "unit_cap", Value: json.RawMessage(`{"max_cents": 1000}`),
		PriorityHint: "board_policy", EffectiveDate: now.AddDate(0, -1, 0), IsActive: true,
	})
	require.NoError(t, err)

	records, err := repo.GatherForResolution(ctx, category, jurisdiction, orgID, &unitID)
	require.NoError(t, err)

	assert.Len(t, records, 3, "should gather jurisdiction, org, and unit policies")

	// Verify ordering: jurisdiction first, then org, then unit.
	scopes := make([]string, len(records))
	for i, r := range records {
		scopes[i] = r.Scope
	}
	assert.Equal(t, []string{"jurisdiction", "org", "unit"}, scopes)
}
```

- [ ] **Step 2: Run integration tests to verify they fail**

Run: `cd backend && go test ./internal/platform/policy/... -count=1 -tags=integration -run "TestGatherForResolution_Ltree|TestGatherForResolution_Includes" -v`

Expected: FAIL — the current SQL doesn't join to `organizations` for the ltree walk, so the firm and subfirm records won't be gathered.

- [ ] **Step 3: Commit test file**

```bash
git add backend/internal/platform/policy/policy_postgres_integration_test.go
git commit -m "test(policy): add integration tests for ltree hierarchical gather"
```

---

### Task 5: ltree hierarchical `GatherForResolution` — implementation

**Files:**
- Modify: `backend/internal/platform/policy/policy_postgres.go`

- [ ] **Step 1: Rewrite `GatherForResolution` SQL with ltree ancestor walk**

Replace the `GatherForResolution` method (lines 105-131 of `policy_postgres.go`):

```go
// GatherForResolution returns all active, in-effect policy records matching the
// given category, jurisdiction, org (including ltree ancestors), and optionally
// unit. This is the Tier 1 deterministic gather step of the policy resolution
// pipeline. The ltree ancestor walk gathers policies from every org in the
// target org's hierarchy path — from the root firm down to the org itself.
func (r *PostgresPolicyRecordRepository) GatherForResolution(ctx context.Context, category string, jurisdiction string, orgID uuid.UUID, unitID *uuid.UUID) ([]PolicyRecord, error) {
	const q = `
		SELECT pr.id, pr.scope, pr.jurisdiction, pr.org_id, pr.unit_id,
		       pr.category, pr.key, pr.value, pr.priority_hint,
		       pr.statute_reference, pr.source_doc_id,
		       pr.effective_date, pr.expiration_date,
		       pr.is_active, pr.created_by, pr.created_at, pr.updated_at
		FROM policy_records pr
		WHERE pr.category = $1
		  AND pr.is_active = true
		  AND (pr.expiration_date IS NULL OR pr.expiration_date > CURRENT_DATE)
		  AND pr.effective_date <= CURRENT_DATE
		  AND (
		      (pr.scope = 'jurisdiction' AND pr.jurisdiction = $2)
		      OR (pr.scope = 'org' AND pr.org_id IN (
		          SELECT anc.id FROM organizations anc
		          WHERE (SELECT path FROM organizations WHERE id = $3) <@ anc.path
		      ))
		      OR (pr.scope = 'unit' AND pr.unit_id = $4)
		  )
		ORDER BY
		  CASE pr.scope
		    WHEN 'jurisdiction' THEN 0
		    WHEN 'org' THEN 1
		    WHEN 'unit' THEN 2
		  END,
		  CASE WHEN pr.scope = 'org' THEN (
		    SELECT nlevel(anc.path) FROM organizations anc WHERE anc.id = pr.org_id
		  ) END ASC NULLS LAST,
		  pr.priority_hint,
		  pr.effective_date`

	rows, err := r.db.Query(ctx, q, category, jurisdiction, orgID, unitID)
	if err != nil {
		return nil, fmt.Errorf("policy: GatherForResolution: %w", err)
	}
	defer rows.Close()

	return collectPolicyRecords(rows, "GatherForResolution")
}
```

- [ ] **Step 2: Run integration tests**

Run: `cd backend && go test ./internal/platform/policy/... -count=1 -tags=integration -run "TestGatherForResolution" -v`

Expected: All pass — the ltree ancestor walk now gathers policies from the full hierarchy.

- [ ] **Step 3: Run unit tests**

Run: `cd backend && go test ./internal/platform/policy/... -short -count=1 -v`

Expected: All pass. Unit tests use `stubRecordRepo` which doesn't hit the database, so the SQL change doesn't affect them.

- [ ] **Step 4: Commit**

```bash
git add backend/internal/platform/policy/policy_postgres.go
git commit -m "feat(policy): rewrite GatherForResolution with ltree ancestor walk

Uses the <@ operator to gather org-scoped policy records from every
ancestor in the org's ltree path. Records are ordered by scope
(jurisdiction → org → unit), then by ltree depth (root firm first),
then by priority_hint and effective_date."
```

---

### Task 6: `OrgJurisdictionAdapter` — test + implementation

**Files:**
- Create: `backend/internal/platform/policy/jurisdiction_adapter.go`
- Create: `backend/internal/platform/policy/jurisdiction_adapter_test.go`

- [ ] **Step 1: Write integration test for `OrgJurisdictionAdapter`**

```go
//go:build integration

package policy_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/platform/policy"
	"github.com/quorant/quorant/internal/platform/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrgJurisdictionAdapter_GetJurisdiction(t *testing.T) {
	pool := testutil.IntegrationDB(t)
	ctx := context.Background()
	adapter := policy.NewOrgJurisdictionAdapter(pool)

	orgID := uuid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO organizations (id, name, slug, type, path, jurisdiction, created_by)
		VALUES ($1, 'Florida HOA', 'fl-hoa', 'hoa', 'fl_hoa', 'FL', $2)`,
		orgID, uuid.New(),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM organizations WHERE id = $1`, orgID)
	})

	jurisdiction, err := adapter.GetJurisdiction(ctx, orgID)
	require.NoError(t, err)
	assert.Equal(t, "FL", jurisdiction)
}

func TestOrgJurisdictionAdapter_NilJurisdiction(t *testing.T) {
	pool := testutil.IntegrationDB(t)
	ctx := context.Background()
	adapter := policy.NewOrgJurisdictionAdapter(pool)

	orgID := uuid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO organizations (id, name, slug, type, path, created_by)
		VALUES ($1, 'No Jurisdiction HOA', 'nj-hoa', 'hoa', 'nj_hoa', $2)`,
		orgID, uuid.New(),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM organizations WHERE id = $1`, orgID)
	})

	jurisdiction, err := adapter.GetJurisdiction(ctx, orgID)
	require.NoError(t, err)
	assert.Equal(t, "", jurisdiction, "nil jurisdiction should return empty string")
}

func TestOrgJurisdictionAdapter_OrgNotFound(t *testing.T) {
	pool := testutil.IntegrationDB(t)
	ctx := context.Background()
	adapter := policy.NewOrgJurisdictionAdapter(pool)

	_, err := adapter.GetJurisdiction(ctx, uuid.New())
	// pgx.ErrNoRows is returned — adapter should handle gracefully.
	assert.Error(t, err)
}
```

Append this to `policy_postgres_integration_test.go` (same file, same build tag).

- [ ] **Step 2: Write `OrgJurisdictionAdapter` implementation**

Create `backend/internal/platform/policy/jurisdiction_adapter.go`:

```go
package policy

import (
	"context"

	"github.com/google/uuid"
	dbpkg "github.com/quorant/quorant/internal/platform/db"
)

// OrgJurisdictionAdapter satisfies OrgJurisdictionLookup by querying the
// organizations table for the org's jurisdiction field.
type OrgJurisdictionAdapter struct {
	db dbpkg.DBTX
}

// NewOrgJurisdictionAdapter creates a new OrgJurisdictionAdapter backed by the
// given database connection.
func NewOrgJurisdictionAdapter(db dbpkg.DBTX) *OrgJurisdictionAdapter {
	return &OrgJurisdictionAdapter{db: db}
}

// GetJurisdiction returns the jurisdiction string for the given org ID.
// Returns an empty string when the org has no jurisdiction set.
func (a *OrgJurisdictionAdapter) GetJurisdiction(ctx context.Context, orgID uuid.UUID) (string, error) {
	var jurisdiction *string
	err := a.db.QueryRow(ctx,
		`SELECT jurisdiction FROM organizations WHERE id = $1`, orgID,
	).Scan(&jurisdiction)
	if err != nil {
		return "", err
	}
	if jurisdiction == nil {
		return "", nil
	}
	return *jurisdiction, nil
}
```

- [ ] **Step 3: Run integration tests**

Run: `cd backend && go test ./internal/platform/policy/... -count=1 -tags=integration -run "TestOrgJurisdictionAdapter" -v`

Expected: All pass.

- [ ] **Step 4: Run unit tests to verify no breakage**

Run: `cd backend && go test ./internal/platform/policy/... -short -count=1 -v`

Expected: All pass.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/platform/policy/jurisdiction_adapter.go backend/internal/platform/policy/policy_postgres_integration_test.go
git commit -m "feat(policy): add OrgJurisdictionAdapter for jurisdiction lookup

Implements OrgJurisdictionLookup by querying the organizations table
for the org's jurisdiction field. Returns empty string when the org
has no jurisdiction set."
```

---

### Task 7: Add `PolicyRegistry` to `Dependencies` and wire in `main.go`

**Files:**
- Modify: `backend/internal/platform/app/module.go`
- Modify: `backend/cmd/quorant-api/main.go`

- [ ] **Step 1: Add `PolicyRegistry` to `Dependencies`**

In `backend/internal/platform/app/module.go`, add the import and field:

Add to imports:
```go
"github.com/quorant/quorant/internal/platform/policy"
```

Add to the `Dependencies` struct (after `EntitlementChecker`):
```go
PolicyRegistry     *policy.Registry
```

- [ ] **Step 2: Wire `OrgJurisdictionAdapter` and `PolicyRegistry` in `main.go`**

In `backend/cmd/quorant-api/main.go`, around line 271, replace:

```go
policyRegistry := policy.NewRegistry(policyRecordRepo, policyResolutionRepo, policyResolver, nil, logger)
```

with:

```go
orgJurisdictionLookup := policy.NewOrgJurisdictionAdapter(pool)
policyRegistry := policy.NewRegistry(policyRecordRepo, policyResolutionRepo, policyResolver, orgJurisdictionLookup, logger)
```

Then, in the section where `deps` is constructed (search for `app.Dependencies{`), add:

```go
PolicyRegistry: policyRegistry,
```

- [ ] **Step 3: Build to verify compilation**

Run: `cd backend && go build ./cmd/quorant-api/...`

Expected: Compiles without errors.

- [ ] **Step 4: Run all tests**

Run: `cd backend && go test ./... -short -count=1`

Expected: All pass.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/platform/app/module.go backend/cmd/quorant-api/main.go
git commit -m "feat(policy): add PolicyRegistry to Dependencies and wire OrgJurisdictionLookup

PolicyRegistry is now available to all domain modules via the shared
Dependencies struct. OrgJurisdictionAdapter is wired in main.go to
provide real jurisdiction lookup instead of always defaulting."
```

---

### Task 8: Register fin descriptors in production code

The 6 existing fin categories have descriptors registered only in test code. Register them in the fin module's production initialization so Tier 2 AI resolution works in production.

**Files:**
- Create: `backend/internal/fin/policy_descriptors.go`
- Create: `backend/internal/fin/policy_descriptors_test.go`

- [ ] **Step 1: Write test for descriptor registration**

Create `backend/internal/fin/policy_descriptors_test.go`:

```go
package fin

import (
	"testing"

	"github.com/quorant/quorant/internal/platform/policy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegisterPolicyDescriptors(t *testing.T) {
	registry := policy.NewRegistry(nil, nil, nil, nil, nil)

	RegisterPolicyDescriptors(registry)

	// Verify all 6 categories have triggers registered by checking FindTriggers.
	// Each descriptor should have at least one PolicySpec with concepts.
	categories := []string{
		"payment_allocation_rules",
		"assessment_fund_allocation",
		"overpayment_policy",
		"reserve_withdrawal_policy",
		"late_fee_cap",
		"interest_rate_cap",
	}

	for _, cat := range categories {
		triggers := registry.FindTriggers("", []string{cat})
		assert.NotEmpty(t, triggers, "category %q should have at least one trigger registered", cat)
	}
}

func TestRegisterPolicyDescriptors_Idempotent(t *testing.T) {
	registry := policy.NewRegistry(nil, nil, nil, nil, nil)

	RegisterPolicyDescriptors(registry)

	// Second call should not panic or error — already registered categories are skipped.
	require.NotPanics(t, func() {
		RegisterPolicyDescriptors(registry)
	})
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/fin/... -short -count=1 -run "TestRegisterPolicyDescriptors" -v`

Expected: FAIL — `RegisterPolicyDescriptors` doesn't exist yet.

- [ ] **Step 3: Implement `RegisterPolicyDescriptors`**

Create `backend/internal/fin/policy_descriptors.go`:

```go
package fin

import "github.com/quorant/quorant/internal/platform/policy"

// RegisterPolicyDescriptors registers all fin module policy descriptors with
// the given registry. This makes the 6 financial policy categories available
// for Tier 2 AI resolution in the certiorari pipeline.
//
// Safe to call multiple times — silently skips categories that are already
// registered.
func RegisterPolicyDescriptors(registry *policy.Registry) {
	if registry == nil {
		return
	}

	descriptors := []policy.OperationDescriptor{
		{
			Category:         "payment_allocation_rules",
			Description:      "Determines how payments are applied to outstanding charges",
			DefaultThreshold: 0.80,
			PromptTemplate: `You are a financial policy engine for an HOA management platform.
Given these active policies: {{.Policies}}

Determine the payment allocation strategy. Return JSON with:
- "priority_order": array of charge types in priority order (e.g., ["regular_assessment", "special_assessment", "late_fee", "interest"])
- "accept_partial": boolean, whether partial payments are accepted
- "credit_handling": string, how credits are handled ("credit_on_account" or "refund")

Apply legal hierarchy: federal > state > local > CC&R > board policy.`,
			Policies: map[string]policy.PolicySpec{
				"payment_allocation_rules": {
					Description:   "Payment allocation and priority rules",
					DocumentTypes: []string{"payment_policy", "collection_policy"},
					Concepts:      []string{"payment_allocation_rules", "payment priority", "payment application"},
				},
			},
		},
		{
			Category:         "assessment_fund_allocation",
			Description:      "Determines how assessment revenue is split across funds",
			DefaultThreshold: 0.80,
			PromptTemplate: `You are a financial policy engine for an HOA management platform.
Given these active policies: {{.Policies}}

Determine how assessment revenue should be allocated across funds. Return JSON with:
- "allocations": array of {"fund_type": string, "percent": number} where percents sum to 1.0
  Valid fund types: "operating", "reserve", "capital_improvement", "special"

Apply legal hierarchy: federal > state > local > CC&R > board policy.`,
			Policies: map[string]policy.PolicySpec{
				"assessment_fund_allocation": {
					Description:   "Assessment revenue fund allocation rules",
					DocumentTypes: []string{"budget", "reserve_study", "assessment_policy"},
					Concepts:      []string{"assessment_fund_allocation", "fund allocation", "reserve contribution"},
				},
			},
		},
		{
			Category:         "overpayment_policy",
			Description:      "Determines how overpayments are handled",
			DefaultThreshold: 0.80,
			PromptTemplate: `You are a financial policy engine for an HOA management platform.
Given these active policies: {{.Policies}}

Determine overpayment handling. Return JSON with:
- "action": "accept" | "reject" | "cap"
- "max_overpay_percent": number (0.0-1.0), required when action is "cap"

Apply legal hierarchy: federal > state > local > CC&R > board policy.`,
			Policies: map[string]policy.PolicySpec{
				"overpayment_policy": {
					Description:   "Overpayment handling rules",
					DocumentTypes: []string{"payment_policy", "collection_policy"},
					Concepts:      []string{"overpayment_policy", "overpayment", "excess payment"},
				},
			},
		},
		{
			Category:         "reserve_withdrawal_policy",
			Description:      "Determines whether reserve fund withdrawals require board approval",
			DefaultThreshold: 0.80,
			PromptTemplate: `You are a financial policy engine for an HOA management platform.
Given these active policies: {{.Policies}}

Determine reserve fund withdrawal requirements. Return JSON with:
- "requires_approval": boolean, whether board approval is required

Apply legal hierarchy: federal > state > local > CC&R > board policy.`,
			Policies: map[string]policy.PolicySpec{
				"reserve_withdrawal_policy": {
					Description:   "Reserve fund withdrawal approval rules",
					DocumentTypes: []string{"reserve_study", "bylaws", "financial_policy"},
					Concepts:      []string{"reserve_withdrawal_policy", "reserve fund", "board approval"},
				},
			},
		},
		{
			Category:         "late_fee_cap",
			Description:      "Maximum allowed late fee amount",
			DefaultThreshold: 0.80,
			PromptTemplate: `You are a financial policy engine for an HOA management platform.
Given these active policies: {{.Policies}}

Determine the maximum late fee. Return JSON with:
- "max_cents": integer, the maximum late fee in cents (null if no cap)

Apply legal hierarchy: federal > state > local > CC&R > board policy. State caps take precedence.`,
			Policies: map[string]policy.PolicySpec{
				"late_fee_cap": {
					Description:   "Late fee cap rules",
					DocumentTypes: []string{"fee_schedule", "statute", "bylaws"},
					Concepts:      []string{"late_fee_cap", "late fee", "penalty cap"},
				},
			},
		},
		{
			Category:         "interest_rate_cap",
			Description:      "Maximum allowed interest charge",
			DefaultThreshold: 0.80,
			PromptTemplate: `You are a financial policy engine for an HOA management platform.
Given these active policies: {{.Policies}}

Determine the maximum interest charge. Return JSON with:
- "max_cents": integer, the maximum interest charge in cents (null if no cap)

Apply legal hierarchy: federal > state > local > CC&R > board policy. State caps take precedence.`,
			Policies: map[string]policy.PolicySpec{
				"interest_rate_cap": {
					Description:   "Interest rate cap rules",
					DocumentTypes: []string{"fee_schedule", "statute", "bylaws"},
					Concepts:      []string{"interest_rate_cap", "interest rate", "usury"},
				},
			},
		},
	}

	for _, desc := range descriptors {
		// Silently skip if already registered (idempotent).
		_ = registry.Register(desc.Category, desc)
	}
}
```

- [ ] **Step 4: Wire descriptor registration in `main.go`**

In `backend/cmd/quorant-api/main.go`, after the `policyRegistry` creation line and before the fin module setup, add:

```go
fin.RegisterPolicyDescriptors(policyRegistry)
```

- [ ] **Step 5: Run tests**

Run: `cd backend && go test ./internal/fin/... -short -count=1 -run "TestRegisterPolicyDescriptors" -v`

Expected: All pass.

- [ ] **Step 6: Build to verify compilation**

Run: `cd backend && go build ./cmd/quorant-api/...`

Expected: Compiles.

- [ ] **Step 7: Commit**

```bash
git add backend/internal/fin/policy_descriptors.go backend/internal/fin/policy_descriptors_test.go backend/cmd/quorant-api/main.go
git commit -m "feat(fin): register 6 policy descriptors in production code

Moves fin policy descriptor registration from test-only to production
via RegisterPolicyDescriptors(). Called from main.go at startup.
Includes PromptTemplates with legal hierarchy instructions for AI
resolution."
```

---

### Task 9: Full regression — all tests pass

**Files:** None (verification only)

- [ ] **Step 1: Run all unit tests**

Run: `cd backend && go test ./... -short -count=1`

Expected: All pass. Zero failures.

- [ ] **Step 2: Run all integration tests**

Run: `cd backend && go test ./... -count=1 -tags=integration`

Expected: All pass, including the new ltree gather and jurisdiction adapter tests.

- [ ] **Step 3: Run linter**

Run: `make lint`

Expected: Clean. No lint errors.

- [ ] **Step 4: Build both binaries**

Run: `make build`

Expected: Both `quorant-api` and `quorant-worker` compile successfully.
