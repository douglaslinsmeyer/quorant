# Certiorari Policy Pattern Design

## Context

The policy engine (`backend/internal/platform/policy/`) currently supports 6 financial categories wired explicitly into `fin/engine_shared.go`. Each category was added one-by-one with manual `registry.Resolve()` calls. The registry errors when a category isn't registered, and `GatherForResolution` only matches the exact `org_id` — it does not walk the firm/org ltree hierarchy.

This design formalizes a "certiorari" pattern: every determination across all domain modules (fin, gov, org, doc, com, task) is offered to the policy engine, but the engine decides whether to exercise jurisdiction. When no policies exist for a category, the engine declines at near-zero cost (~1ms DB query). When policies exist at any level of the firm/org hierarchy, the engine gathers the full stack and sends it to AI for merge-with-priority resolution.

## Design Decisions

| Decision | Choice |
|----------|--------|
| Scope | All modules, all decisions — universal gate |
| Hierarchy resolution | Merge with priority — AI sees full policy stack from all hierarchy levels |
| Decline semantics | `Resolve()` returns `(nil, nil)` — zero AI cost, caller uses its own default |
| Hierarchy walk | ltree `<@` ancestor query on `organizations.path` |
| Sync model | Always synchronous — decline path ~1ms, AI path ~500ms only when records exist |
| Implementation approach | Direct Resolve with nil-decline (Approach A — minimal refactor) |

## Architecture

### 1. Certiorari Semantics in `Registry.Resolve()`

**File**: `backend/internal/platform/policy/registry.go`

Current flow errors when a category isn't registered (line 137-139). The certiorari pattern restructures this:

```
Resolve(ctx, orgID, unitID, category)
  1. GatherForResolution (ltree ancestor walk)
  2. If 0 records gathered → return (nil, nil)     [DECLINED — zero cost]
  3. Lookup descriptor for category
  4. If no descriptor registered → return (nil, nil) [DECLINED — no one claimed jurisdiction]
  5. Descriptor exists + records exist → Tier 2 AI   [GRANTED]
  6. Confidence check → approved / held
  7. Persist resolution + return
```

Key changes:
- **Gather before descriptor lookup.** Records are fetched first. If none exist, return nil immediately.
- **Descriptor lookup is optional.** If records exist but no descriptor is registered, return nil. The descriptor is required for Tier 2 because it holds the PromptTemplate.
- **Remove the "category not registered" error.** Replace with nil-decline path.

Existing callers in `engine_shared.go` already handle nil correctly:
```go
if registry == nil { return default }
res, err := registry.Resolve(ctx, orgID, nil, category)
if err != nil || res == nil || res.Ruling == nil { return default }
```
No caller changes required.

### 2. ltree Hierarchical Gather

**File**: `backend/internal/platform/policy/policy_postgres.go`

Replace the flat `org_id = $3` clause in `GatherForResolution` with an ltree ancestor walk:

```sql
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
  pr.effective_date
```

- `<@` operator: "target path is descendant-or-equal-to ancestor path" — uses the existing GIST index on `organizations.path`.
- Ordering: jurisdiction → root firm (lowest nlevel) → sub-firms → org itself → unit, then by priority_hint and effective_date.
- **No interface change**: `GatherForResolution(ctx, category, jurisdiction, orgID, unitID)` signature stays the same. The implementation internally joins to `organizations`.

AI receives the full stack, e.g.: `[FL statute → Acme firm policy → Southeast regional policy → Sunset HOA board policy → Unit 4B policy]` and reasons about legal precedence.

### 3. OrgJurisdictionLookup Wiring

**File**: `backend/internal/platform/policy/registry.go` (interface at line 16)

The `OrgJurisdictionLookup` interface exists but is passed as `nil` in `main.go` (line 271). Implement a simple Postgres adapter:

```go
// OrgJurisdictionAdapter satisfies OrgJurisdictionLookup by querying the
// organizations table for the org's jurisdiction field.
type OrgJurisdictionAdapter struct {
    db dbpkg.DBTX
}

func NewOrgJurisdictionAdapter(db dbpkg.DBTX) *OrgJurisdictionAdapter {
    return &OrgJurisdictionAdapter{db: db}
}

func (a *OrgJurisdictionAdapter) GetJurisdiction(ctx context.Context, orgID uuid.UUID) (string, error) {
    var jurisdiction *string
    err := a.db.QueryRow(ctx,
        `SELECT jurisdiction FROM organizations WHERE id = $1`, orgID,
    ).Scan(&jurisdiction)
    if err != nil || jurisdiction == nil {
        return "", err
    }
    return *jurisdiction, nil
}
```

Wire in `main.go`:
```go
orgJurisdictionLookup := policy.NewOrgJurisdictionAdapter(pool)
policyRegistry := policy.NewRegistry(policyRecordRepo, policyResolutionRepo, policyResolver, orgJurisdictionLookup, logger)
```

### 4. Registry in Dependencies

**File**: `backend/internal/platform/app/module.go`

Add `PolicyRegistry` to the shared `Dependencies` struct:

```go
type Dependencies struct {
    Pool               *pgxpool.Pool
    Redis              *redis.Client
    Logger             *slog.Logger
    Auditor            audit.Auditor
    Publisher          queue.Publisher
    TokenValidator     auth.TokenValidator
    PermChecker        middleware.PermissionChecker
    ResolveUserID      func(ctx context.Context) (uuid.UUID, error)
    EntitlementChecker middleware.EntitlementChecker
    PolicyRegistry     *policy.Registry  // new
}
```

Each module reads `deps.PolicyRegistry` in its `Register()` method and passes it to its service. Nil-safe — modules that don't use it yet don't break.

### 5. Descriptor Registration Per Module

Each module that participates in the policy engine registers its `OperationDescriptor`s at startup in its `Register()` method:

```go
func (m *GovModule) Register(mux *http.ServeMux, deps app.Dependencies) {
    if deps.PolicyRegistry != nil {
        deps.PolicyRegistry.Register("violation_fine_policy", policy.OperationDescriptor{
            Category:       "violation_fine_policy",
            Description:    "Determines fine amounts for governance violations",
            PromptTemplate: "Given these policies: {{.Policies}} ...",
        })
    }
    // ... rest of module setup
}
```

For the initial implementation, only the existing 6 fin categories need descriptor registration. Other modules adopt incrementally — their `Resolve()` calls return `(nil, nil)` until they register descriptors AND policy records exist in the database.

### 6. Module Adoption Roadmap

| Module | Categories | When |
|--------|-----------|------|
| `fin` | `payment_allocation_rules`, `assessment_fund_allocation`, `overpayment_policy`, `reserve_withdrawal_policy`, `late_fee_cap`, `interest_rate_cap` | This change (existing, already wired) |
| `gov` | `violation_fine_policy`, `cure_deadline`, `hearing_procedure`, `enforcement_escalation`, `architectural_review` | Next iteration |
| `org` | `membership_rules`, `board_term_limits`, `quorum_requirements`, `voting_thresholds` | Future |
| `doc` | `document_retention`, `approval_workflow`, `signature_requirements` | Future |
| `com` | `communication_rules`, `notice_periods`, `delivery_methods` | Future |
| `task` | `sla_deadlines`, `auto_assignment`, `escalation_policies` | Future |

## Files to Modify

| File | Change |
|------|--------|
| `backend/internal/platform/policy/registry.go` | Restructure `Resolve()`: gather-first, nil-decline, descriptor-optional |
| `backend/internal/platform/policy/policy_postgres.go` | Rewrite `GatherForResolution` SQL with ltree ancestor walk |
| `backend/internal/platform/policy/registry.go` | Add `OrgJurisdictionAdapter` type |
| `backend/internal/platform/app/module.go` | Add `PolicyRegistry *policy.Registry` to `Dependencies` |
| `backend/cmd/quorant-api/main.go` | Wire `OrgJurisdictionAdapter`, set `PolicyRegistry` in deps |
| `backend/internal/fin/engine_shared.go` | Register 6 existing category descriptors (if not already registered) |
| `backend/internal/platform/policy/registry_test.go` | Add certiorari-specific test cases |
| `backend/internal/platform/policy/policy_postgres_test.go` | Add ltree hierarchical gather integration tests |

## Testing

### Unit Tests (`registry_test.go`)
- Category with no records → `(nil, nil)` (declined)
- Category with records but no registered descriptor → `(nil, nil)`
- Category with records AND descriptor → proceeds to Tier 2 (existing behavior preserved)
- Verify no AI call when 0 records gathered
- Existing tests continue to pass

### Integration Tests (`policy_postgres_test.go`)
- Org with ltree path `firm.sub.hoa` gathers policies from all three levels
- Jurisdiction-scoped policies included alongside org-hierarchy policies
- Unit-scoped policies included when unitID provided
- Ordering: jurisdiction → root firm → sub-firm → org → unit, then priority_hint
- OrgJurisdictionAdapter returns correct jurisdiction string

### Regression
- All existing `engine_shared_test.go` tests pass unchanged (nil-check pattern preserved)
- `make test` — all unit tests
- `make test-integration` — hierarchical gather against real Postgres with ltree

## Verification

1. Run `make test` — all unit tests pass
2. Run `make test-integration` — hierarchical gather + jurisdiction adapter work against real Postgres
3. Manual: create policy records at firm-level and org-level for the same category, verify `Resolve()` gathers both and returns them to AI
4. Manual: call `Resolve()` for a category with no records — verify nil return and no AI call
