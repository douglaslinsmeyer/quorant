# Accounting Engine Phase 4 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the IFRS accounting standard driver as a second engine, register it with the EngineFactory, and add standard-switching validation so orgs can select GAAP or IFRS.

**Architecture:** The IfrsEngine implements the same 8-method AccountingEngine interface as GaapEngine. Most behavior is similar but with IFRS-specific differences: a different chart of accounts (simpler equity — no fund balance distinction mandated by IFRS), IFRS 15 revenue recognition (stricter variable consideration), and IFRS 9 expected credit loss for bad debt. Shared logic (payment terms parsing, allocation strategy) is extracted to reusable functions. The IFRS builder is registered alongside GAAP in main.go, and the EngineFactory resolves per-org.

**Tech Stack:** Go 1.22, pgx v5, testify, PostgreSQL 16, existing EngineFactory/EngineConfig infrastructure.

**Spec:** `docs/superpowers/specs/2026-04-10-accounting-engine-design.md` — Phase 4 section

**Pre-requisite:** Phase 3 complete on main. Create worktree from main before starting.

---

## File Map

### New Files

| File | Responsibility |
|------|---------------|
| `backend/internal/fin/engine_ifrs.go` | IfrsEngine struct, constructor, Standard(), ChartOfAccounts(), all effect methods |
| `backend/internal/fin/engine_ifrs_test.go` | IFRS driver tests (chart, recording, recognition differences from GAAP) |
| `backend/internal/fin/engine_shared.go` | Shared helpers extracted from GAAP: payment terms parsing, allocation strategy, payable recognition |

### Modified Files

| File | Changes |
|------|---------|
| `backend/internal/fin/engine_gaap.go` | Move shared helpers to engine_shared.go, delegate to them |
| `backend/internal/fin/engine_terms.go` | Move PaymentTerms and PayableRecognitionDate to engine_shared.go (identical for both standards) |
| `backend/internal/fin/engine_payment_strategy.go` | Move PaymentApplicationStrategy to engine_shared.go (identical for both standards) |
| `backend/internal/fin/engine_config.go` | Add standard-switching validation on OrgAccountingConfigRepository |
| `backend/cmd/quorant-api/main.go` | Register IFRS builder alongside GAAP |

---

## Tasks

### Task 1: Extract shared engine helpers

**Files:**
- Create: `backend/internal/fin/engine_shared.go`
- Modify: `backend/internal/fin/engine_terms.go`
- Modify: `backend/internal/fin/engine_payment_strategy.go`

Several AccountingEngine methods have identical behavior across GAAP and IFRS:
- **PaymentTerms** — vendor terms parsing is standard-agnostic (Net 30 is Net 30)
- **PayableRecognitionDate** — accrual vs cash recognition is the same
- **PaymentApplicationStrategy** — designated/policy/default logic is the same

Extract these as standalone functions that both engines can call.

- [ ] **Step 1: Create engine_shared.go with shared functions**

Move the implementation logic (not the receiver methods) from engine_terms.go and engine_payment_strategy.go into engine_shared.go as package-level functions:

```go
package fin

import (
    "context"
    "encoding/json"
    "fmt"
    "regexp"
    "strconv"
    "strings"
    "time"

    "github.com/quorant/quorant/internal/platform/policy"
)

// Shared engine helpers used by both GAAP and IFRS drivers.

var termsRegex = regexp.MustCompile(`(?i)(\d+)/(\d+)\s+net\s+(\d+)`)
var netRegex = regexp.MustCompile(`(?i)net\s+(\d+)`)

func parsePaymentTerms(pc PayableContext) (*PaymentTermsResult, error) {
    terms := strings.TrimSpace(pc.VendorTerms)
    if m := termsRegex.FindStringSubmatch(terms); len(m) == 4 {
        discPct, _ := strconv.ParseFloat(m[1], 64)
        discDays, _ := strconv.Atoi(m[2])
        netDays, _ := strconv.Atoi(m[3])
        discDate := pc.InvoiceDate.AddDate(0, 0, discDays)
        return &PaymentTermsResult{
            DueDate: pc.InvoiceDate.AddDate(0, 0, netDays),
            DiscountDate: &discDate, DiscountPercent: &discPct,
        }, nil
    }
    if m := netRegex.FindStringSubmatch(terms); len(m) == 2 {
        netDays, _ := strconv.Atoi(m[1])
        return &PaymentTermsResult{DueDate: pc.InvoiceDate.AddDate(0, 0, netDays)}, nil
    }
    return &PaymentTermsResult{DueDate: pc.InvoiceDate.AddDate(0, 0, 30)}, nil
}

func resolvePayableRecognitionDate(basis RecognitionBasis, ec ExpenseContext) (time.Time, error) {
    if basis == RecognitionBasisCash {
        return time.Time{}, ErrCashBasisNoPayable
    }
    if ec.ServiceDate != nil {
        return *ec.ServiceDate, nil
    }
    return ec.InvoiceDate, nil
}

func resolvePaymentStrategy(ctx context.Context, registry *policy.Registry, pc PaymentContext) (*ApplicationStrategy, error) {
    if pc.DesignatedInvoice != nil {
        return &ApplicationStrategy{Method: ApplicationMethodDesignated}, nil
    }
    if registry != nil {
        resolution, err := registry.Resolve(ctx, pc.OrgID, nil, "payment_allocation_rules")
        if err == nil && resolution != nil && resolution.Ruling != nil {
            var ruling AllocationRuling
            if jsonErr := json.Unmarshal(resolution.Ruling, &ruling); jsonErr == nil {
                return rulingToStrategy(ruling), nil
            }
        }
    }
    return &ApplicationStrategy{Method: ApplicationMethodOldestFirst, WithinPriority: SortOldestFirst}, nil
}
```

- [ ] **Step 2: Update GAAP engine to delegate to shared functions**

In engine_terms.go, engine_payment_strategy.go — change the receiver methods to call the shared functions:

```go
func (e *GaapEngine) PaymentTerms(_ context.Context, pc PayableContext) (*PaymentTermsResult, error) {
    return parsePaymentTerms(pc)
}

func (e *GaapEngine) PayableRecognitionDate(_ context.Context, ec ExpenseContext) (time.Time, error) {
    return resolvePayableRecognitionDate(e.config.RecognitionBasis, ec)
}

func (e *GaapEngine) PaymentApplicationStrategy(ctx context.Context, pc PaymentContext) (*ApplicationStrategy, error) {
    return resolvePaymentStrategy(ctx, e.registry, pc)
}
```

- [ ] **Step 3: Run tests — all existing tests must still pass**

Run: `cd backend && go test ./internal/fin/... -short -count=1`

- [ ] **Step 4: Commit**

```bash
git commit -am "refactor(fin): extract shared engine helpers for cross-standard reuse"
```

---

### Task 2: Implement IfrsEngine — struct, Standard(), ChartOfAccounts()

**Files:**
- Create: `backend/internal/fin/engine_ifrs.go`
- Create: `backend/internal/fin/engine_ifrs_test.go`

- [ ] **Step 1: Write tests for IFRS Standard() and ChartOfAccounts()**

```go
func newTestIfrsEngine() *IfrsEngine {
    return NewIfrsEngine(nil, nil, EngineConfig{
        RecognitionBasis: RecognitionBasisAccrual, FiscalYearStart: 1,
    })
}

func TestIfrsEngine_Standard(t *testing.T) {
    engine := newTestIfrsEngine()
    assert.Equal(t, AccountingStandardIFRS, engine.Standard())
}

func TestIfrsEngine_ChartOfAccounts(t *testing.T) {
    engine := newTestIfrsEngine()
    chart := engine.ChartOfAccounts()
    require.NotEmpty(t, chart)

    // IFRS chart should differ from GAAP:
    // - Simpler equity section (no fund-specific balance accounts mandated)
    // - Uses "Retained Earnings" instead of separate fund balances
    byNumber := make(map[int]GLAccountSeed)
    for _, a := range chart {
        byNumber[a.Number] = a
    }

    // Core accounts still present.
    assert.Contains(t, byNumber, 1010) // Cash-Operating
    assert.Contains(t, byNumber, 1100) // AR
    assert.Contains(t, byNumber, 2100) // AP

    // All accounts must be system + have valid types.
    for _, a := range chart {
        assert.True(t, a.IsSystem)
        if !a.IsHeader {
            assert.NotZero(t, a.ParentNum)
        }
    }

    // No duplicate numbers.
    seen := make(map[int]bool)
    for _, a := range chart {
        assert.False(t, seen[a.Number], "duplicate %d", a.Number)
        seen[a.Number] = true
    }
}
```

- [ ] **Step 2: Implement IfrsEngine**

```go
package fin

import (
    "context"
    "fmt"
    "time"

    "github.com/quorant/quorant/internal/platform/policy"
)

type IfrsEngine struct {
    resolver AccountResolver
    registry *policy.Registry
    config   EngineConfig
}

func NewIfrsEngine(resolver AccountResolver, registry *policy.Registry, config EngineConfig) *IfrsEngine {
    return &IfrsEngine{resolver: resolver, registry: registry, config: config}
}

var _ AccountingEngine = (*IfrsEngine)(nil)

func (e *IfrsEngine) Standard() AccountingStandard { return AccountingStandardIFRS }

func (e *IfrsEngine) ChartOfAccounts() []GLAccountSeed { return ifrsChartOfAccounts() }
```

The IFRS chart differences from GAAP:
- **Equity:** Uses "Retained Earnings" (3010) instead of per-fund balance accounts. Fund-specific equity is presentation, not structural — one retained earnings account instead of 3010/3020/3030/3040. Interfund accounts still present.
- **Revenue:** Same structure as GAAP (per-fund revenue accounts).
- **Assets/Liabilities/Expenses:** Largely the same.

Create the IFRS chart with ~45 accounts (fewer equity accounts than GAAP's 56).

- [ ] **Step 3: Run tests — verify pass**

- [ ] **Step 4: Commit**

```bash
git commit -am "feat(fin): implement IFRS engine Standard() and ChartOfAccounts()"
```

---

### Task 3: Implement IfrsEngine RecordTransaction

**Files:**
- Modify: `backend/internal/fin/engine_ifrs.go`
- Modify: `backend/internal/fin/engine_ifrs_test.go`

The IFRS RecordTransaction handles the same transaction types as GAAP. Most journal entry patterns are identical (double-entry bookkeeping is universal). The key differences are in account selection (IFRS chart may use different numbers) and recognition timing.

- [ ] **Step 1: Write tests for IFRS assessment and payment recording**

```go
func newTestIfrsEngineWithResolver(basis RecognitionBasis) (*IfrsEngine, *stubAccountResolver) {
    // Use the same stubAccountResolver pattern as GAAP tests.
    // The IFRS chart uses the same account numbers for core accounts.
    resolver := &stubAccountResolver{accounts: map[int]*GLAccount{
        1010: {ID: uuid.MustParse("..."), AccountNumber: 1010},
        1100: {ID: uuid.MustParse("..."), AccountNumber: 1100},
        4010: {ID: uuid.MustParse("..."), AccountNumber: 4010},
        // ... same core accounts
    }}
    engine := NewIfrsEngine(resolver, nil, EngineConfig{RecognitionBasis: basis, FiscalYearStart: 1})
    return engine, resolver
}

func TestIfrsEngine_RecordTransaction_Assessment_Accrual(t *testing.T) {
    // Same pattern as GAAP: DR AR / CR Revenue
    // IFRS 15 doesn't change the basic journal entry for assessments
}

func TestIfrsEngine_RecordTransaction_Payment_Accrual(t *testing.T) {
    // Same pattern as GAAP: DR Cash / CR AR
}
```

- [ ] **Step 2: Implement RecordTransaction**

The IFRS engine can reuse the same effect methods as GAAP since the journal entry mechanics are identical. The differences are in the chart and recognition methods. Implement by creating the same dispatch pattern:

```go
func (e *IfrsEngine) RecordTransaction(ctx context.Context, tx FinancialTransaction) (*FinancialEffects, error) {
    if e.config.RecognitionBasis == RecognitionBasisModifiedAccrual {
        return nil, fmt.Errorf("record transaction: modified_accrual not supported under IFRS")
    }
    switch tx.Type {
    case TxTypeAssessment:
        return e.assessmentEffects(ctx, tx)
    case TxTypePayment:
        return e.paymentEffects(ctx, tx)
    case TxTypeFundTransfer:
        return e.fundTransferEffects(ctx, tx)
    case TxTypeExpense:
        return e.expenseEffects(ctx, tx)
    case TxTypeLateFee:
        return e.lateFeeEffects(ctx, tx)
    case TxTypeInterestAccrual:
        return e.interestAccrualEffects(ctx, tx)
    case TxTypeBadDebtProvision:
        return e.badDebtProvisionEffects(ctx, tx)
    case TxTypeBadDebtWriteOff:
        return e.badDebtWriteOffEffects(ctx, tx)
    case TxTypeBadDebtRecovery:
        return e.badDebtRecoveryEffects(ctx, tx)
    case TxTypeYearEndClose:
        return e.yearEndCloseEffects(ctx, tx)
    case TxTypeVoidReversal:
        return e.voidReversalEffects(ctx, tx)
    default:
        return nil, fmt.Errorf("record transaction: unsupported type %q", tx.Type)
    }
}
```

The IFRS effect methods follow the same patterns as GAAP. Key differences:
- Modified accrual is NOT supported under IFRS (rejected)
- Bad debt uses IFRS 9 expected credit loss model (same GL accounts, different recognition triggers — but the journal entry mechanics are identical: DR Expense / CR Allowance)
- Year-end close uses retained earnings (3010) instead of per-fund balances

For each effect method, the implementing agent should read the corresponding GAAP method and create an IFRS version. Most will be nearly identical since the journal entry patterns are the same — only the recognition rules differ.

- [ ] **Step 3: Run tests — verify pass**

- [ ] **Step 4: Commit**

```bash
git commit -am "feat(fin): implement IFRS RecordTransaction for all transaction types"
```

---

### Task 4: Implement remaining IfrsEngine methods

**Files:**
- Modify: `backend/internal/fin/engine_ifrs.go`
- Modify: `backend/internal/fin/engine_ifrs_test.go`

- [ ] **Step 1: Implement ValidateTransaction, PaymentTerms, PayableRecognitionDate, PaymentApplicationStrategy, RevenueRecognitionDate**

These delegate to shared helpers where behavior is identical:

```go
func (e *IfrsEngine) ValidateTransaction(ctx context.Context, tx FinancialTransaction) error {
    // Same validation rules as GAAP (amount positive, required fields, period check).
    // IFRS does not support modified_accrual.
    if e.config.RecognitionBasis == RecognitionBasisModifiedAccrual {
        return fmt.Errorf("validate: modified_accrual not supported under IFRS")
    }
    // Same field validation as GAAP...
}

func (e *IfrsEngine) PaymentTerms(_ context.Context, pc PayableContext) (*PaymentTermsResult, error) {
    return parsePaymentTerms(pc) // shared helper
}

func (e *IfrsEngine) PayableRecognitionDate(_ context.Context, ec ExpenseContext) (time.Time, error) {
    return resolvePayableRecognitionDate(e.config.RecognitionBasis, ec) // shared helper
}

func (e *IfrsEngine) PaymentApplicationStrategy(ctx context.Context, pc PaymentContext) (*ApplicationStrategy, error) {
    return resolvePaymentStrategy(ctx, e.registry, pc) // shared helper
}

func (e *IfrsEngine) RevenueRecognitionDate(_ context.Context, tx FinancialTransaction) (time.Time, error) {
    // IFRS 15: same basic timing as GAAP ASC 606 for HOA assessments.
    // Stricter variable consideration constraints apply to special assessments
    // with contingencies, but the recognition date logic is the same.
    return tx.EffectiveDate, nil
}
```

- [ ] **Step 2: Write tests for IFRS-specific behavior**

```go
func TestIfrsEngine_ValidateTransaction_RejectsModifiedAccrual(t *testing.T) {
    engine := NewIfrsEngine(nil, nil, EngineConfig{RecognitionBasis: RecognitionBasisModifiedAccrual})
    tx := FinancialTransaction{Type: TxTypeAssessment, AmountCents: 10000, SourceID: uuid.New(), UnitID: ptr(uuid.New())}
    err := engine.ValidateTransaction(context.Background(), tx)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "modified_accrual")
}

func TestIfrsEngine_PaymentTerms(t *testing.T) {
    engine := newTestIfrsEngine()
    result, err := engine.PaymentTerms(context.Background(), PayableContext{
        InvoiceDate: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
        VendorTerms: "Net 30",
    })
    require.NoError(t, err)
    assert.Equal(t, time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC), result.DueDate)
}
```

- [ ] **Step 3: Run tests — verify pass**

- [ ] **Step 4: Commit**

```bash
git commit -am "feat(fin): implement remaining IFRS engine methods"
```

---

### Task 5: Register IFRS builder in main.go

**Files:**
- Modify: `backend/cmd/quorant-api/main.go`

- [ ] **Step 1: Add IFRS builder to the engine builders map**

Find the existing builders map in main.go and add the IFRS builder:

```go
engineBuilders := map[fin.AccountingStandard]fin.EngineBuilder{
    fin.AccountingStandardGAAP: func(config fin.EngineConfig) fin.AccountingEngine {
        return fin.NewGaapEngine(glService, policyRegistry, config)
    },
    fin.AccountingStandardIFRS: func(config fin.EngineConfig) fin.AccountingEngine {
        return fin.NewIfrsEngine(glService, policyRegistry, config)
    },
}
```

- [ ] **Step 2: Verify build**

Run: `cd backend && go build ./cmd/quorant-api/...`

- [ ] **Step 3: Commit**

```bash
git commit -am "feat(fin): register IFRS engine builder in main.go"
```

---

### Task 6: Add standard-switching validation

**Files:**
- Modify: `backend/internal/fin/engine_config.go`
- Create or modify test file for config validation

- [ ] **Step 1: Write test for standard-switching validation**

The spec says: "Migration path: changing standards requires year-end close under old standard first."

```go
func TestEngineFactory_StandardSwitchRequiresYearEndClose(t *testing.T) {
    // Org has GAAP config, then adds IFRS config.
    // The factory should validate that the prior fiscal year is closed
    // before allowing the switch.
    // This validation happens in OrgAccountingConfigRepository.CreateConfig
    // or in a service-layer method.
}
```

- [ ] **Step 2: Add validation to CreateConfig**

In engine_config.go, add a `ValidateConfigChange` function or extend the factory:

```go
// ValidateStandardSwitch checks that changing accounting standards is safe.
// Requires that all periods in the prior fiscal year are closed.
func ValidateStandardSwitch(ctx context.Context, periods AccountingPeriodRepository, orgID uuid.UUID, currentConfig *OrgAccountingConfig, newStandard AccountingStandard) error {
    if currentConfig.Standard == newStandard {
        return nil // same standard, no switch
    }
    // Check that the current fiscal year's periods are all closed.
    fiscalYear := currentConfig.EffectiveDate.Year()
    allClosed, err := periods.AllPeriodsClosedForYear(ctx, orgID, fiscalYear)
    if err != nil {
        return fmt.Errorf("validate standard switch: %w", err)
    }
    if !allClosed {
        return fmt.Errorf("validate standard switch: all periods in fiscal year %d must be closed before changing standards from %s to %s", fiscalYear, currentConfig.Standard, newStandard)
    }
    return nil
}
```

- [ ] **Step 3: Run tests — verify pass**

- [ ] **Step 4: Commit**

```bash
git commit -am "feat(fin): add standard-switching validation requiring year-end close"
```

---

### Task 7: Final verification

- [ ] **Step 1: Verify both engines implement the interface**

```bash
grep "var _ AccountingEngine" backend/internal/fin/engine_gaap.go backend/internal/fin/engine_ifrs.go
```

Expected: both files have compile-time interface checks.

- [ ] **Step 2: Run full test suite**

```bash
cd backend && go test ./internal/fin/... -short -count=1 -v | grep -c "PASS"
cd backend && go test ./... -short -count=1
```

- [ ] **Step 3: Verify IFRS builder in main.go**

```bash
grep "AccountingStandardIFRS" backend/cmd/quorant-api/main.go
```

- [ ] **Step 4: Commit any cleanup**

```bash
git commit -am "chore(fin): Phase 4 final verification — IFRS driver complete"
```

---

## Post-Implementation Verification

After all tasks:
- Both GaapEngine and IfrsEngine satisfy AccountingEngine interface
- IFRS chart of accounts differs from GAAP (simpler equity section)
- IFRS rejects modified_accrual basis
- PaymentTerms, PaymentApplicationStrategy, PayableRecognitionDate shared across both engines
- Standard-switching validation requires year-end close
- IFRS builder registered in main.go
- All tests pass
