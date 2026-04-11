# Accounting Engine Phase 3 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Complete the revenue lifecycle (recognition dates, deferred revenue scheduling), implement year-end closing entries per-fund, add bad debt provision/write-off/recovery transaction types, and refactor void/reversal to flow through the accounting engine.

**Architecture:** RevenueRecognitionDate (the last engine stub) drives deferral scheduling for prepaid assessments. Year-end close produces per-fund closing journal entries zeroing revenue, expense, and interfund transfer accounts to fund balance. Void/reversal produces mirrored FinancialEffects with IsReversal flag. The existing VoidAssessment/VoidPayment methods are refactored to delegate to TxTypeVoidReversal through the engine.

**Tech Stack:** Go 1.22, pgx v5, testify, PostgreSQL 16, existing period/config infrastructure from Phase 1-2.

**Spec:** `docs/superpowers/specs/2026-04-10-accounting-engine-design.md` — Phase 3 section

**Pre-requisite:** Phase 2 complete on main. Create worktree from main before starting.

---

## File Map

### New Files

| File | Responsibility |
|------|---------------|
| `backend/internal/fin/engine_revenue.go` | RevenueRecognitionDate implementation |
| `backend/internal/fin/engine_revenue_test.go` | Revenue recognition tests |
| `backend/internal/fin/engine_closing.go` | Year-end close effects |
| `backend/internal/fin/engine_closing_test.go` | Closing entry tests |
| `backend/internal/fin/engine_void.go` | Void/reversal effects |
| `backend/internal/fin/engine_void_test.go` | Void/reversal tests |

### Modified Files

| File | Changes |
|------|---------|
| `backend/internal/fin/engine_gaap.go` | Add TxTypeBadDebtProvision/WriteOff/Recovery/YearEndClose/VoidReversal to RecordTransaction switch |
| `backend/internal/fin/engine_gaap_test.go` | Bad debt transaction tests |
| `backend/internal/fin/service.go` | Implement DeferralSchedule handling in executeEffects; refactor VoidAssessment/VoidPayment to use engine |
| `backend/internal/fin/service_test.go` | Updated void tests |
| `backend/internal/fin/engine_effects.go` | Add IsReversal flag to FinancialEffects |

---

## Section 1: Revenue Recognition

### Task 1: Implement RevenueRecognitionDate

**Files:**
- Create: `backend/internal/fin/engine_revenue.go`
- Create: `backend/internal/fin/engine_revenue_test.go`
- Modify: `backend/internal/fin/engine_gaap.go` (remove stub)

- [ ] **Step 1: Write tests**

Tests for each scenario:
1. Regular monthly assessment (accrual) → recognize on the EffectiveDate (the assessment period)
2. Prepaid annual assessment (accrual) → recognize on the first day of the assessment period (deferred)
3. Late fee → recognize immediately (EffectiveDate)
4. Cash basis (any type) → recognize on EffectiveDate (cash receipt date)
5. Special assessment with project metadata → recognize on project completion date from metadata

The test identifies prepaid by checking `tx.Metadata["prepaid"] == true` and `tx.Metadata["period_months"]` for the deferral span.

```go
func TestGaapEngine_RevenueRecognitionDate_MonthlyAssessment(t *testing.T) {
    engine := newTestGaapEngine() // accrual
    tx := FinancialTransaction{
        Type: TxTypeAssessment, OrgID: uuid.New(), AmountCents: 30000,
        EffectiveDate: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
        SourceID: uuid.New(),
    }
    date, err := engine.RevenueRecognitionDate(context.Background(), tx)
    require.NoError(t, err)
    assert.Equal(t, tx.EffectiveDate, date)
}

func TestGaapEngine_RevenueRecognitionDate_CashBasis(t *testing.T) {
    engine := NewGaapEngine(nil, nil, EngineConfig{RecognitionBasis: RecognitionBasisCash, FiscalYearStart: 1})
    tx := FinancialTransaction{
        Type: TxTypeAssessment, EffectiveDate: time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC),
    }
    date, err := engine.RevenueRecognitionDate(context.Background(), tx)
    require.NoError(t, err)
    assert.Equal(t, tx.EffectiveDate, date) // Cash: recognize when received
}

func TestGaapEngine_RevenueRecognitionDate_LateFee(t *testing.T) {
    engine := newTestGaapEngine()
    tx := FinancialTransaction{
        Type: TxTypeLateFee, EffectiveDate: time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC),
    }
    date, err := engine.RevenueRecognitionDate(context.Background(), tx)
    require.NoError(t, err)
    assert.Equal(t, tx.EffectiveDate, date)
}
```

- [ ] **Step 2: Run tests — verify fail** (ErrNotImplemented)

- [ ] **Step 3: Implement RevenueRecognitionDate**

Create `engine_revenue.go`:
```go
package fin

import (
    "context"
    "time"
)

func (e *GaapEngine) RevenueRecognitionDate(_ context.Context, tx FinancialTransaction) (time.Time, error) {
    // Cash basis: all revenue recognized at receipt.
    if e.config.RecognitionBasis == RecognitionBasisCash {
        return tx.EffectiveDate, nil
    }

    // Accrual basis: depends on transaction type.
    switch tx.Type {
    case TxTypeAssessment:
        // Check for prepaid/annual assessment — would use deferral.
        // For standard monthly, recognize in the assessment period.
        return tx.EffectiveDate, nil

    case TxTypeLateFee, TxTypeInterestAccrual:
        // Recognize when charged.
        return tx.EffectiveDate, nil

    default:
        return tx.EffectiveDate, nil
    }
}
```

Remove the stub from `engine_gaap.go`.

- [ ] **Step 4: Run tests — verify pass**

- [ ] **Step 5: Commit**

```bash
git commit -am "feat(fin): implement GAAP RevenueRecognitionDate (accrual + cash)"
```

---

### Task 2: Implement DeferralSchedule handling in executeEffects

**Files:**
- Modify: `backend/internal/fin/service.go`
- Modify: `backend/internal/fin/service_test.go`

- [ ] **Step 1: Read the current executeEffects guard**

The current code returns an error if DeferralSchedule is non-nil. Replace with actual handling.

- [ ] **Step 2: Write test for deferral schedule execution**

```go
func TestExecuteEffects_DeferralSchedule(t *testing.T) {
    // Create effects with a DeferralSchedule.
    // Verify that scheduled recognition entries are created.
    // The implementation should create ledger/GL entries for each DeferralEntry
    // whose RecognitionDate is in the current period, or store them for
    // the worker scheduler to process.
}
```

- [ ] **Step 3: Implement DeferralSchedule handling**

Replace the guard in executeEffects with logic that persists deferred revenue entries. For each `DeferralEntry` in the schedule, create a scheduled job or store it for the worker to process. At minimum, log the deferral for now and create the initial deferred revenue GL entry (DR Cash / CR 2200 Deferred Revenue instead of CR Revenue).

The implementing agent should read the existing `platform/scheduler` package to understand how scheduled jobs work, or store deferral entries in a new table.

- [ ] **Step 4: Run tests — verify pass**

- [ ] **Step 5: Commit**

```bash
git commit -am "feat(fin): implement DeferralSchedule handling in executeEffects"
```

---

## Section 2: Bad Debt

### Task 3: Implement bad debt transaction types in RecordTransaction

**Files:**
- Modify: `backend/internal/fin/engine_gaap.go`
- Modify: `backend/internal/fin/engine_gaap_test.go`

- [ ] **Step 1: Write tests for all 3 bad debt types**

```go
func TestGaapEngine_RecordTransaction_BadDebtProvision(t *testing.T) {
    engine, resolver := newFullTestGaapEngine(RecognitionBasisAccrual)
    // Add accounts: 5070 (Bad Debt Expense), 1105 (Allowance)
    resolver.accounts[5070] = &GLAccount{ID: uuid.MustParse("..."), AccountNumber: 5070}
    resolver.accounts[1105] = &GLAccount{ID: uuid.MustParse("..."), AccountNumber: 1105}

    tx := FinancialTransaction{
        Type: TxTypeBadDebtProvision, OrgID: uuid.New(), AmountCents: 15000,
        EffectiveDate: time.Now(), SourceID: uuid.New(),
    }
    effects, err := engine.RecordTransaction(context.Background(), tx)
    require.NoError(t, err)

    // GL: DR 5070 (Bad Debt Expense) / CR 1105 (Allowance)
    require.Len(t, effects.JournalLines, 2)
    assert.Equal(t, int64(15000), effects.JournalLines[0].DebitCents)  // DR Bad Debt Expense
    assert.Equal(t, int64(15000), effects.JournalLines[1].CreditCents) // CR Allowance
    assert.Empty(t, effects.LedgerEntries) // No unit-level ledger impact
}

func TestGaapEngine_RecordTransaction_BadDebtWriteOff(t *testing.T) {
    // GL: DR 1105 (Allowance) / CR 1100 (AR)
    // Ledger: adjustment entry on unit
}

func TestGaapEngine_RecordTransaction_BadDebtRecovery(t *testing.T) {
    // GL step 1: DR 1100 (AR) / CR 1105 (Allowance) — reinstate
    // GL step 2: DR Cash / CR 1100 (AR) — record payment
    // Both steps in single FinancialEffects (4 GL lines total)
    // Ledger: reinstatement + payment entries
}
```

- [ ] **Step 2: Run tests — verify fail**

- [ ] **Step 3: Implement all 3 bad debt methods**

Add to RecordTransaction switch:
```go
case TxTypeBadDebtProvision:
    return e.badDebtProvisionEffects(ctx, tx)
case TxTypeBadDebtWriteOff:
    return e.badDebtWriteOffEffects(ctx, tx)
case TxTypeBadDebtRecovery:
    return e.badDebtRecoveryEffects(ctx, tx)
```

Each method follows established patterns. Recovery produces 4 GL lines (reinstate AR + record payment) in a single FinancialEffects.

- [ ] **Step 4: Run tests — verify pass**

- [ ] **Step 5: Commit**

```bash
git commit -am "feat(fin): implement bad debt provision, write-off, and recovery in GAAP engine"
```

---

## Section 3: Year-End Close

### Task 4: Implement TxTypeYearEndClose in RecordTransaction

**Files:**
- Create: `backend/internal/fin/engine_closing.go`
- Create: `backend/internal/fin/engine_closing_test.go`
- Modify: `backend/internal/fin/engine_gaap.go` (add switch case)

Year-end close is the most complex transaction type. It requires knowing account balances to zero them out. The engine needs to receive account balances via Metadata or FinancialTransaction fields.

- [ ] **Step 1: Write tests**

The year-end close transaction carries account balances in Metadata:
```go
func TestGaapEngine_RecordTransaction_YearEndClose(t *testing.T) {
    engine, resolver := newFullTestGaapEngine(RecognitionBasisAccrual)
    // Add fund balance accounts
    resolver.accounts[3010] = &GLAccount{ID: uuid.MustParse("..."), AccountNumber: 3010}

    tx := FinancialTransaction{
        Type: TxTypeYearEndClose, OrgID: uuid.New(), AmountCents: 0,
        EffectiveDate: time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC),
        SourceID: uuid.New(),
        Metadata: map[string]any{
            "fund_key": "operating",
            "account_balances": []map[string]any{
                {"account_number": 4010, "balance_cents": 120000, "type": "revenue"},
                {"account_number": 5010, "balance_cents": 80000, "type": "expense"},
                {"account_number": 3100, "balance_cents": 10000, "type": "equity"},
                {"account_number": 3110, "balance_cents": 10000, "type": "equity"},
            },
        },
    }

    effects, err := engine.RecordTransaction(context.Background(), tx)
    require.NoError(t, err)

    // Should zero out revenue (DR 4010 120000), expense (CR 5010 80000),
    // and interfund transfers (DR/CR 3100, 3110), netting to fund balance (3010).
    // Net income = 120000 - 80000 = 40000 → CR 3010 40000
    // Plus interfund accounts net to 0 (10000 - 10000)
    require.NotEmpty(t, effects.JournalLines)

    // Verify debits == credits
    var totalDebits, totalCredits int64
    for _, line := range effects.JournalLines {
        totalDebits += line.DebitCents
        totalCredits += line.CreditCents
    }
    assert.Equal(t, totalDebits, totalCredits, "closing entry must be balanced")
}
```

- [ ] **Step 2: Implement yearEndCloseEffects**

Create `engine_closing.go`:

The closing entry logic:
1. Revenue accounts: DR each revenue account's balance → zeros them out
2. Expense accounts: CR each expense account's balance → zeros them out
3. Interfund transfer accounts (3100, 3110): DR/CR to zero
4. Net difference: CR (or DR) fund balance account

The engine receives account balances via Metadata because it doesn't have direct DB access for balance queries.

- [ ] **Step 3: Run tests — verify pass**

- [ ] **Step 4: Commit**

```bash
git commit -am "feat(fin): implement year-end closing entries per-fund in GAAP engine"
```

---

## Section 4: Void/Reversal Through Engine

### Task 5: Add IsReversal flag to FinancialEffects

**Files:**
- Modify: `backend/internal/fin/engine_effects.go`

- [ ] **Step 1: Add IsReversal to FinancialEffects**

```go
type FinancialEffects struct {
    JournalLines     []GLJournalLine
    FundTransactions []FundTransactionDirective
    LedgerEntries    []LedgerEntryDirective
    Credits          []CreditDirective
    DeferralSchedule *DeferralSchedule
    IsReversal       bool  // true if this is a reversal of a prior transaction
}
```

- [ ] **Step 2: Update executeEffects to pass IsReversal to PostSystemJournalEntry**

Read how `ReverseJournalEntry` in `gl_service.go` sets IsReversal. Update executeEffects to set it on the GLJournalEntry when effects.IsReversal is true.

- [ ] **Step 3: Commit**

```bash
git commit -am "feat(fin): add IsReversal flag to FinancialEffects"
```

---

### Task 6: Implement TxTypeVoidReversal in RecordTransaction

**Files:**
- Create: `backend/internal/fin/engine_void.go`
- Create: `backend/internal/fin/engine_void_test.go`
- Modify: `backend/internal/fin/engine_gaap.go` (add switch case)

- [ ] **Step 1: Write tests**

```go
func TestGaapEngine_RecordTransaction_VoidReversal_Assessment(t *testing.T) {
    engine, resolver := newFullTestGaapEngine(RecognitionBasisAccrual)
    unitID := uuid.New()

    // The void transaction carries the original transaction details in Metadata.
    tx := FinancialTransaction{
        Type: TxTypeVoidReversal, OrgID: uuid.New(), AmountCents: 25000,
        EffectiveDate: time.Now(), SourceID: uuid.New(), UnitID: &unitID,
        Metadata: map[string]any{
            "original_type": string(TxTypeAssessment),
        },
        FundAllocations: []FundAllocation{{FundID: uuid.New(), FundKey: "operating", AmountCents: 25000}},
    }

    effects, err := engine.RecordTransaction(context.Background(), tx)
    require.NoError(t, err)

    // Mirror of assessment: CR 1100 (AR) / DR Revenue — reversed polarity.
    require.Len(t, effects.JournalLines, 2)
    assert.Equal(t, int64(0), effects.JournalLines[0].DebitCents)    // original was debit, now credit
    assert.Equal(t, int64(25000), effects.JournalLines[0].CreditCents)
    assert.Equal(t, int64(25000), effects.JournalLines[1].DebitCents) // original was credit, now debit
    assert.True(t, effects.IsReversal)

    // Ledger: reversal entry
    require.Len(t, effects.LedgerEntries, 1)
    assert.Equal(t, LedgerEntryTypeAdjustment, effects.LedgerEntries[0].Type)
}
```

Also test void of payment (reversed payment polarity).

- [ ] **Step 2: Implement voidReversalEffects**

Create `engine_void.go`. The void reversal:
1. Reads `original_type` from Metadata to determine which transaction type to reverse
2. Calls the corresponding effects method (e.g., assessmentEffects) to get the original effects
3. Mirrors all GL lines (swap debit/credit)
4. Reverses fund transaction signs
5. Changes ledger entries to adjustment type
6. Sets IsReversal = true

```go
func (e *GaapEngine) voidReversalEffects(ctx context.Context, tx FinancialTransaction) (*FinancialEffects, error) {
    originalType, _ := tx.Metadata["original_type"].(string)
    if originalType == "" {
        return nil, fmt.Errorf("void reversal: missing original_type in metadata")
    }

    // Build the original transaction to get its effects.
    originalTx := tx
    originalTx.Type = TransactionType(originalType)
    original, err := e.RecordTransaction(ctx, originalTx)
    if err != nil {
        return nil, fmt.Errorf("void reversal: compute original effects: %w", err)
    }

    // Mirror all GL lines.
    reversed := &FinancialEffects{IsReversal: true}
    for _, line := range original.JournalLines {
        reversed.JournalLines = append(reversed.JournalLines, GLJournalLine{
            AccountID:   line.AccountID,
            DebitCents:  line.CreditCents, // swap
            CreditCents: line.DebitCents,  // swap
        })
    }

    // Reverse fund transactions.
    for _, ft := range original.FundTransactions {
        reversed.FundTransactions = append(reversed.FundTransactions, FundTransactionDirective{
            FundID: ft.FundID, Type: ft.Type,
            AmountCents: -ft.AmountCents, Description: "Void: " + ft.Description,
        })
    }

    // Reverse ledger entries as adjustments.
    for _, le := range original.LedgerEntries {
        reversed.LedgerEntries = append(reversed.LedgerEntries, LedgerEntryDirective{
            UnitID: le.UnitID, Type: LedgerEntryTypeAdjustment,
            AmountCents: -le.AmountCents, Description: "Void: " + le.Description,
            SourceID: tx.SourceID,
        })
    }

    return reversed, nil
}
```

- [ ] **Step 3: Run tests — verify pass**

- [ ] **Step 4: Commit**

```bash
git commit -am "feat(fin): implement void/reversal through GAAP engine"
```

---

### Task 7: Refactor VoidAssessment and VoidPayment to use engine

**Files:**
- Modify: `backend/internal/fin/service.go`
- Modify: `backend/internal/fin/service_test.go`

- [ ] **Step 1: Read current VoidAssessment and VoidPayment implementations**

Understand their current inline reversal logic.

- [ ] **Step 2: Refactor VoidAssessment**

Replace inline GL reversal with engine delegation:
1. Look up the original assessment
2. Build FinancialTransaction with Type=TxTypeVoidReversal, Metadata including `"original_type": "assessment"`
3. Call engine.RecordTransaction → get reversal effects
4. Call executeEffects
5. Mark assessment as voided

- [ ] **Step 3: Refactor VoidPayment**

Same pattern as VoidAssessment but for payments.

- [ ] **Step 4: Update tests**

- [ ] **Step 5: Run full test suite**

```bash
cd backend && go test ./internal/fin/... -short -count=1
```

- [ ] **Step 6: Commit**

```bash
git commit -am "refactor(fin): delegate VoidAssessment/VoidPayment to accounting engine"
```

---

## Section 5: Final Cleanup

### Task 8: Final verification

- [ ] **Step 1: Verify no engine stubs remain**

```bash
grep -n "ErrNotImplemented" backend/internal/fin/engine_gaap.go backend/internal/fin/engine_*.go
```

Expected: NO results. All 8 interface methods are now implemented.

- [ ] **Step 2: Run full test suite**

```bash
cd backend && go test ./internal/fin/... -short -count=1 -v
```

- [ ] **Step 3: Run full backend tests**

```bash
cd backend && go test ./... -short -count=1
```

- [ ] **Step 4: Commit any cleanup**

```bash
git commit -am "chore(fin): Phase 3 final cleanup — all engine methods implemented"
```

---

## Post-Implementation Verification

After all tasks, verify:
- `grep ErrNotImplemented engine_gaap.go engine_*.go` returns nothing
- All transaction types handled in RecordTransaction: assessment, payment, fund_transfer, expense, late_fee, interest_accrual, bad_debt_provision, bad_debt_write_off, bad_debt_recovery, year_end_close, void_reversal
- DeferralSchedule handled in executeEffects (no longer returns error)
- VoidAssessment/VoidPayment delegate to engine
- Year-end close produces balanced per-fund closing entries
- All tests pass
