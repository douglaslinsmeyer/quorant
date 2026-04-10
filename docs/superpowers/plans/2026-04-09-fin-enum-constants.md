# Fin Module Typed Enum Constants — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace all bare string enum values in the fin module with typed string constants and `IsValid()` methods (issue #79).

**Architecture:** New `enums.go` file defines 17 named string types with const blocks and `IsValid()` methods. `domain.go` struct fields change from `string` to the typed enums. All consumers (service, handlers, repos, requests, tests) updated to use constants.

**Tech Stack:** Go, existing fin module patterns, testify for assertions.

---

### Task 1: Create `enums.go` with all typed enum types

**Files:**
- Create: `backend/internal/fin/enums.go`
- Test: `backend/internal/fin/enums_test.go`

- [ ] **Step 1: Write the failing test for all `IsValid()` methods**

Create `backend/internal/fin/enums_test.go`:

```go
package fin

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBudgetStatus_IsValid(t *testing.T) {
	assert.True(t, BudgetStatusDraft.IsValid())
	assert.True(t, BudgetStatusProposed.IsValid())
	assert.True(t, BudgetStatusApproved.IsValid())
	assert.False(t, BudgetStatus("invalid").IsValid())
	assert.False(t, BudgetStatus("").IsValid())
}

func TestExpenseStatus_IsValid(t *testing.T) {
	assert.True(t, ExpenseStatusPending.IsValid())
	assert.True(t, ExpenseStatusApproved.IsValid())
	assert.True(t, ExpenseStatusPaid.IsValid())
	assert.False(t, ExpenseStatus("invalid").IsValid())
	assert.False(t, ExpenseStatus("").IsValid())
}

func TestPaymentStatus_IsValid(t *testing.T) {
	assert.True(t, PaymentStatusCompleted.IsValid())
	assert.False(t, PaymentStatus("invalid").IsValid())
	assert.False(t, PaymentStatus("").IsValid())
}

func TestCollectionCaseStatus_IsValid(t *testing.T) {
	assert.True(t, CollectionCaseStatusLate.IsValid())
	assert.True(t, CollectionCaseStatusClosed.IsValid())
	assert.True(t, CollectionCaseStatusResolved.IsValid())
	assert.False(t, CollectionCaseStatus("invalid").IsValid())
	assert.False(t, CollectionCaseStatus("").IsValid())
}

func TestPaymentPlanStatus_IsValid(t *testing.T) {
	assert.True(t, PaymentPlanStatusActive.IsValid())
	assert.True(t, PaymentPlanStatusDefaulted.IsValid())
	assert.False(t, PaymentPlanStatus("invalid").IsValid())
	assert.False(t, PaymentPlanStatus("").IsValid())
}

func TestFundType_IsValid(t *testing.T) {
	assert.True(t, FundTypeOperating.IsValid())
	assert.True(t, FundTypeReserve.IsValid())
	assert.True(t, FundTypeCapital.IsValid())
	assert.True(t, FundTypeSpecial.IsValid())
	assert.False(t, FundType("invalid").IsValid())
	assert.False(t, FundType("").IsValid())
}

func TestLedgerEntryType_IsValid(t *testing.T) {
	assert.True(t, LedgerEntryTypeCharge.IsValid())
	assert.True(t, LedgerEntryTypePayment.IsValid())
	assert.True(t, LedgerEntryTypeCredit.IsValid())
	assert.True(t, LedgerEntryTypeAdjustment.IsValid())
	assert.True(t, LedgerEntryTypeLateFee.IsValid())
	assert.False(t, LedgerEntryType("invalid").IsValid())
	assert.False(t, LedgerEntryType("").IsValid())
}

func TestGLSourceType_IsValid(t *testing.T) {
	assert.True(t, GLSourceTypeAssessment.IsValid())
	assert.True(t, GLSourceTypePayment.IsValid())
	assert.True(t, GLSourceTypeTransfer.IsValid())
	assert.True(t, GLSourceTypeManual.IsValid())
	assert.False(t, GLSourceType("invalid").IsValid())
	assert.False(t, GLSourceType("").IsValid())
}

func TestLedgerReferenceType_IsValid(t *testing.T) {
	assert.True(t, LedgerRefTypePayment.IsValid())
	assert.False(t, LedgerReferenceType("invalid").IsValid())
	assert.False(t, LedgerReferenceType("").IsValid())
}

func TestPaymentMethodType_IsValid(t *testing.T) {
	assert.True(t, PaymentMethodTypeCard.IsValid())
	assert.True(t, PaymentMethodTypeACH.IsValid())
	assert.False(t, PaymentMethodType("invalid").IsValid())
	assert.False(t, PaymentMethodType("").IsValid())
}

func TestCollectionActionType_IsValid(t *testing.T) {
	assert.True(t, CollectionActionTypeNoticeSent.IsValid())
	assert.True(t, CollectionActionTypeLienFiled.IsValid())
	assert.False(t, CollectionActionType("invalid").IsValid())
	assert.False(t, CollectionActionType("").IsValid())
}

func TestGLAccountType_IsValid(t *testing.T) {
	assert.True(t, GLAccountTypeAsset.IsValid())
	assert.True(t, GLAccountTypeLiability.IsValid())
	assert.True(t, GLAccountTypeEquity.IsValid())
	assert.True(t, GLAccountTypeRevenue.IsValid())
	assert.True(t, GLAccountTypeExpense.IsValid())
	assert.False(t, GLAccountType("invalid").IsValid())
	assert.False(t, GLAccountType("").IsValid())
}

func TestAssessmentFrequency_IsValid(t *testing.T) {
	assert.True(t, AssessmentFreqMonthly.IsValid())
	assert.True(t, AssessmentFreqQuarterly.IsValid())
	assert.True(t, AssessmentFreqSemiAnnually.IsValid())
	assert.True(t, AssessmentFreqAnnually.IsValid())
	assert.False(t, AssessmentFrequency("invalid").IsValid())
	assert.False(t, AssessmentFrequency("").IsValid())
}

func TestPaymentPlanFrequency_IsValid(t *testing.T) {
	assert.True(t, PaymentPlanFreqMonthly.IsValid())
	assert.False(t, PaymentPlanFrequency("invalid").IsValid())
	assert.False(t, PaymentPlanFrequency("").IsValid())
}

func TestAmountStrategy_IsValid(t *testing.T) {
	assert.True(t, AmountStrategyFlat.IsValid())
	assert.True(t, AmountStrategyPerUnitType.IsValid())
	assert.True(t, AmountStrategyPerSqft.IsValid())
	assert.True(t, AmountStrategyCustom.IsValid())
	assert.False(t, AmountStrategy("invalid").IsValid())
	assert.False(t, AmountStrategy("").IsValid())
}

func TestBudgetCategoryType_IsValid(t *testing.T) {
	assert.True(t, BudgetCategoryTypeExpense.IsValid())
	assert.False(t, BudgetCategoryType("invalid").IsValid())
	assert.False(t, BudgetCategoryType("").IsValid())
}

func TestTriggeredBy_IsValid(t *testing.T) {
	assert.True(t, TriggeredBySystem.IsValid())
	assert.True(t, TriggeredByUser.IsValid())
	assert.False(t, TriggeredBy("invalid").IsValid())
	assert.False(t, TriggeredBy("").IsValid())
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/fin/... -run "TestBudgetStatus_IsValid|TestExpenseStatus_IsValid|TestPaymentStatus_IsValid|TestCollectionCaseStatus_IsValid|TestPaymentPlanStatus_IsValid|TestFundType_IsValid|TestLedgerEntryType_IsValid|TestGLSourceType_IsValid|TestLedgerReferenceType_IsValid|TestPaymentMethodType_IsValid|TestCollectionActionType_IsValid|TestGLAccountType_IsValid|TestAssessmentFrequency_IsValid|TestPaymentPlanFrequency_IsValid|TestAmountStrategy_IsValid|TestBudgetCategoryType_IsValid|TestTriggeredBy_IsValid" -short -count=1`

Expected: FAIL — types and constants not defined yet.

- [ ] **Step 3: Create `enums.go` with all types, constants, and `IsValid()` methods**

Create `backend/internal/fin/enums.go`:

```go
package fin

// ── Budget Status ────────────────────────────────────────────────────────────

type BudgetStatus string

const (
	BudgetStatusDraft    BudgetStatus = "draft"
	BudgetStatusProposed BudgetStatus = "proposed"
	BudgetStatusApproved BudgetStatus = "approved"
)

func (s BudgetStatus) IsValid() bool {
	switch s {
	case BudgetStatusDraft, BudgetStatusProposed, BudgetStatusApproved:
		return true
	}
	return false
}

// ── Expense Status ───────────────────────────────────────────────────────────

type ExpenseStatus string

const (
	ExpenseStatusPending  ExpenseStatus = "pending"
	ExpenseStatusApproved ExpenseStatus = "approved"
	ExpenseStatusPaid     ExpenseStatus = "paid"
)

func (s ExpenseStatus) IsValid() bool {
	switch s {
	case ExpenseStatusPending, ExpenseStatusApproved, ExpenseStatusPaid:
		return true
	}
	return false
}

// ── Payment Status ───────────────────────────────────────────────────────────

type PaymentStatus string

const (
	PaymentStatusCompleted PaymentStatus = "completed"
)

func (s PaymentStatus) IsValid() bool {
	switch s {
	case PaymentStatusCompleted:
		return true
	}
	return false
}

// ── Collection Case Status ───────────────────────────────────────────────────

type CollectionCaseStatus string

const (
	CollectionCaseStatusLate     CollectionCaseStatus = "late"
	CollectionCaseStatusClosed   CollectionCaseStatus = "closed"
	CollectionCaseStatusResolved CollectionCaseStatus = "resolved"
)

func (s CollectionCaseStatus) IsValid() bool {
	switch s {
	case CollectionCaseStatusLate, CollectionCaseStatusClosed, CollectionCaseStatusResolved:
		return true
	}
	return false
}

// ── Payment Plan Status ──────────────────────────────────────────────────────

type PaymentPlanStatus string

const (
	PaymentPlanStatusActive    PaymentPlanStatus = "active"
	PaymentPlanStatusDefaulted PaymentPlanStatus = "defaulted"
)

func (s PaymentPlanStatus) IsValid() bool {
	switch s {
	case PaymentPlanStatusActive, PaymentPlanStatusDefaulted:
		return true
	}
	return false
}

// ── Fund Type ────────────────────────────────────────────────────────────────

type FundType string

const (
	FundTypeOperating FundType = "operating"
	FundTypeReserve   FundType = "reserve"
	FundTypeCapital   FundType = "capital"
	FundTypeSpecial   FundType = "special"
)

func (t FundType) IsValid() bool {
	switch t {
	case FundTypeOperating, FundTypeReserve, FundTypeCapital, FundTypeSpecial:
		return true
	}
	return false
}

// ── Ledger Entry Type ────────────────────────────────────────────────────────

type LedgerEntryType string

const (
	LedgerEntryTypeCharge     LedgerEntryType = "charge"
	LedgerEntryTypePayment    LedgerEntryType = "payment"
	LedgerEntryTypeCredit     LedgerEntryType = "credit"
	LedgerEntryTypeAdjustment LedgerEntryType = "adjustment"
	LedgerEntryTypeLateFee    LedgerEntryType = "late_fee"
)

func (t LedgerEntryType) IsValid() bool {
	switch t {
	case LedgerEntryTypeCharge, LedgerEntryTypePayment, LedgerEntryTypeCredit, LedgerEntryTypeAdjustment, LedgerEntryTypeLateFee:
		return true
	}
	return false
}

// ── GL Source Type ───────────────────────────────────────────────────────────

type GLSourceType string

const (
	GLSourceTypeAssessment GLSourceType = "assessment"
	GLSourceTypePayment    GLSourceType = "payment"
	GLSourceTypeTransfer   GLSourceType = "transfer"
	GLSourceTypeManual     GLSourceType = "manual"
)

func (t GLSourceType) IsValid() bool {
	switch t {
	case GLSourceTypeAssessment, GLSourceTypePayment, GLSourceTypeTransfer, GLSourceTypeManual:
		return true
	}
	return false
}

// ── Ledger Reference Type ────────────────────────────────────────────────────

type LedgerReferenceType string

const (
	LedgerRefTypePayment LedgerReferenceType = "payment"
)

func (t LedgerReferenceType) IsValid() bool {
	switch t {
	case LedgerRefTypePayment:
		return true
	}
	return false
}

// ── Payment Method Type ──────────────────────────────────────────────────────

type PaymentMethodType string

const (
	PaymentMethodTypeCard PaymentMethodType = "card"
	PaymentMethodTypeACH  PaymentMethodType = "ach"
)

func (t PaymentMethodType) IsValid() bool {
	switch t {
	case PaymentMethodTypeCard, PaymentMethodTypeACH:
		return true
	}
	return false
}

// ── Collection Action Type ───────────────────────────────────────────────────

type CollectionActionType string

const (
	CollectionActionTypeNoticeSent CollectionActionType = "notice_sent"
	CollectionActionTypeLienFiled  CollectionActionType = "lien_filed"
)

func (t CollectionActionType) IsValid() bool {
	switch t {
	case CollectionActionTypeNoticeSent, CollectionActionTypeLienFiled:
		return true
	}
	return false
}

// ── GL Account Type ──────────────────────────────────────────────────────────

type GLAccountType string

const (
	GLAccountTypeAsset     GLAccountType = "asset"
	GLAccountTypeLiability GLAccountType = "liability"
	GLAccountTypeEquity    GLAccountType = "equity"
	GLAccountTypeRevenue   GLAccountType = "revenue"
	GLAccountTypeExpense   GLAccountType = "expense"
)

func (t GLAccountType) IsValid() bool {
	switch t {
	case GLAccountTypeAsset, GLAccountTypeLiability, GLAccountTypeEquity, GLAccountTypeRevenue, GLAccountTypeExpense:
		return true
	}
	return false
}

// ── Assessment Frequency ─────────────────────────────────────────────────────

type AssessmentFrequency string

const (
	AssessmentFreqMonthly      AssessmentFrequency = "monthly"
	AssessmentFreqQuarterly    AssessmentFrequency = "quarterly"
	AssessmentFreqSemiAnnually AssessmentFrequency = "semi_annually"
	AssessmentFreqAnnually     AssessmentFrequency = "annually"
)

func (f AssessmentFrequency) IsValid() bool {
	switch f {
	case AssessmentFreqMonthly, AssessmentFreqQuarterly, AssessmentFreqSemiAnnually, AssessmentFreqAnnually:
		return true
	}
	return false
}

// ── Payment Plan Frequency ───────────────────────────────────────────────────

type PaymentPlanFrequency string

const (
	PaymentPlanFreqMonthly PaymentPlanFrequency = "monthly"
)

func (f PaymentPlanFrequency) IsValid() bool {
	switch f {
	case PaymentPlanFreqMonthly:
		return true
	}
	return false
}

// ── Amount Strategy ──────────────────────────────────────────────────────────

type AmountStrategy string

const (
	AmountStrategyFlat        AmountStrategy = "flat"
	AmountStrategyPerUnitType AmountStrategy = "per_unit_type"
	AmountStrategyPerSqft     AmountStrategy = "per_sqft"
	AmountStrategyCustom      AmountStrategy = "custom"
)

func (s AmountStrategy) IsValid() bool {
	switch s {
	case AmountStrategyFlat, AmountStrategyPerUnitType, AmountStrategyPerSqft, AmountStrategyCustom:
		return true
	}
	return false
}

// ── Budget Category Type ─────────────────────────────────────────────────────

type BudgetCategoryType string

const (
	BudgetCategoryTypeExpense BudgetCategoryType = "expense"
)

func (t BudgetCategoryType) IsValid() bool {
	switch t {
	case BudgetCategoryTypeExpense:
		return true
	}
	return false
}

// ── Triggered By ─────────────────────────────────────────────────────────────

type TriggeredBy string

const (
	TriggeredBySystem TriggeredBy = "system"
	TriggeredByUser   TriggeredBy = "user"
)

func (t TriggeredBy) IsValid() bool {
	switch t {
	case TriggeredBySystem, TriggeredByUser:
		return true
	}
	return false
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd backend && go test ./internal/fin/... -run "TestBudgetStatus_IsValid|TestExpenseStatus_IsValid|TestPaymentStatus_IsValid|TestCollectionCaseStatus_IsValid|TestPaymentPlanStatus_IsValid|TestFundType_IsValid|TestLedgerEntryType_IsValid|TestGLSourceType_IsValid|TestLedgerReferenceType_IsValid|TestPaymentMethodType_IsValid|TestCollectionActionType_IsValid|TestGLAccountType_IsValid|TestAssessmentFrequency_IsValid|TestPaymentPlanFrequency_IsValid|TestAmountStrategy_IsValid|TestBudgetCategoryType_IsValid|TestTriggeredBy_IsValid" -short -count=1`

Expected: PASS — all 17 `IsValid()` tests pass.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/fin/enums.go backend/internal/fin/enums_test.go
git commit -m "feat(fin): add typed enum types with IsValid methods (issue #79)"
```

---

### Task 2: Update `domain.go` struct fields to use typed enums

**Files:**
- Modify: `backend/internal/fin/domain.go`

This task changes struct field types. It will cause compile errors in consumers — that's intentional and will be fixed in subsequent tasks.

- [ ] **Step 1: Update all struct fields in `domain.go`**

Apply these changes to `backend/internal/fin/domain.go`:

| Struct | Field | From | To |
|--------|-------|------|----|
| `AssessmentSchedule` | `Frequency` (line 19) | `string` | `AssessmentFrequency` |
| `AssessmentSchedule` | `AmountStrategy` (line 20) | `string` | `AmountStrategy` |
| `LedgerEntry` | `EntryType` (line 62) | `string` | `LedgerEntryType` |
| `LedgerEntry` | `ReferenceType` (line 66) | `*string` | `*LedgerReferenceType` |
| `PaymentMethod` | `MethodType` (line 77) | `string` | `PaymentMethodType` |
| `Payment` | `Status` (line 94) | `string` | `PaymentStatus` |
| `BudgetCategory` | `CategoryType` (line 107) | `string` | `BudgetCategoryType` |
| `Budget` | `Status` (line 120) | `string` | `BudgetStatus` |
| `Expense` | `FundType` (line 157) | `*string` | `*FundType` |
| `Expense` | `Status` (line 162) | `string` | `ExpenseStatus` |
| `Fund` | `FundType` (line 186) | `string` | `FundType` |
| `CollectionCase` | `Status` (line 237) | `string` | `CollectionCaseStatus` |
| `CollectionAction` | `ActionType` (line 255) | `string` | `CollectionActionType` |
| `CollectionAction` | `TriggeredBy` (line 258) | `*string` | `*TriggeredBy` |
| `PaymentPlan` | `Frequency` (line 274) | `string` | `PaymentPlanFrequency` |
| `PaymentPlan` | `Status` (line 278) | `string` | `PaymentPlanStatus` |
| `GLAccount` | `AccountType` (line 294) | `string` | `GLAccountType` |
| `GLJournalEntry` | `SourceType` (line 310) | `*string` | `*GLSourceType` |
| `TrialBalanceRow` | `AccountType` (line 335) | `string` | `GLAccountType` |
| `AccountBalance` | `AccountType` (line 345) | `string` | `GLAccountType` |

Update the inline comments to remove the enum values list since the types are now self-documenting:

- Line 19: `// monthly|quarterly|annually|semi_annually` → remove comment
- Line 20: `// flat|per_unit_type|per_sqft|custom` → remove comment
- Line 62: `// charge|payment|credit|adjustment|late_fee` → remove comment
- Line 186: `// operating|reserve|capital|special` → remove comment
- Line 258: `// "system" or "user"` → remove comment

- [ ] **Step 2: Verify the compile errors are what we expect**

Run: `cd backend && go build ./internal/fin/... 2>&1 | head -40`

Expected: compile errors in `service.go`, `requests.go`, `handler_collection.go`, `gl_service.go`, `collection_postgres.go`, `budget_postgres.go`, and test files — all related to type mismatches where bare strings are used.

- [ ] **Step 3: Commit (compile errors are expected at this stage)**

```bash
git add backend/internal/fin/domain.go
git commit -m "refactor(fin): change domain struct fields to typed enums (issue #79)"
```

---

### Task 3: Update `service.go` to use enum constants

**Files:**
- Modify: `backend/internal/fin/service.go`

- [ ] **Step 1: Replace all bare strings in `service.go`**

Apply these replacements:

| Line | From | To |
|------|------|----|
| 208 | `EntryType: "charge"` | `EntryType: LedgerEntryTypeCharge` |
| 223 | `sourceType := "assessment"` | `sourceType := GLSourceTypeAssessment` |
| 306 | `Status: "completed"` | `Status: PaymentStatusCompleted` |
| 316 | `refType := "payment"` | `refType := LedgerRefTypePayment` |
| 321 | `EntryType: "payment"` | `EntryType: LedgerEntryTypePayment` |
| 338 | `sourceType := "payment"` | `sourceType := GLSourceTypePayment` |
| 401 | `Status: "draft"` | `Status: BudgetStatusDraft` |
| 446 | `b.Status != "draft"` | `b.Status != BudgetStatusDraft` |
| 450 | `b.Status = "proposed"` | `b.Status = BudgetStatusProposed` |
| 462 | `b.Status != "proposed"` | `b.Status != BudgetStatusProposed` |
| 466 | `b.Status = "approved"` | `b.Status = BudgetStatusApproved` |
| 544 | `Status: "pending"` | `Status: ExpenseStatusPending` |
| 580 | `e.Status != "pending"` | `e.Status != ExpenseStatusPending` |
| 584 | `e.Status = "approved"` | `e.Status = ExpenseStatusApproved` |
| 596 | `e.Status != "approved"` | `e.Status != ExpenseStatusApproved` |
| 600 | `e.Status = "paid"` | `e.Status = ExpenseStatusPaid` |
| 690 | `sourceType := "transfer"` | `sourceType := GLSourceTypeTransfer` |
| 770 | `Status: "active"` | `Status: PaymentPlanStatusActive` |

Also update the `cashAccountForFundType` function signature and body (line 843):

```go
func cashAccountForFundType(fundType FundType) int {
	switch fundType {
	case FundTypeReserve:
		return 1020
	default:
		return 1010
	}
}
```

And update `PostSystemJournalEntry` call sites — the `sourceType` variables are now `GLSourceType` instead of `string`, so the `&sourceType` pointers are already the right type (`*GLSourceType`) after the domain.go change.

- [ ] **Step 2: Verify service.go compiles**

Run: `cd backend && go vet ./internal/fin/service.go 2>&1`

Expected: no errors from service.go (other files may still have errors).

- [ ] **Step 3: Commit**

```bash
git add backend/internal/fin/service.go
git commit -m "refactor(fin): use typed enum constants in service.go (issue #79)"
```

---

### Task 4: Update `gl_service.go` to use enum constants

**Files:**
- Modify: `backend/internal/fin/gl_service.go`

- [ ] **Step 1: Replace bare strings in `gl_service.go`**

| Line | From | To |
|------|------|----|
| 37 | `AccountType: req.AccountType` | `AccountType: GLAccountType(req.AccountType)` |
| 117 | `sourceType := "manual"` | `sourceType := GLSourceTypeManual` |

Also update the `acctDef` struct in `SeedDefaultAccounts` (line 202) — change `acctType string` to `acctType GLAccountType`, and update all 24 account definitions from bare strings to constants:

```go
type acctDef struct {
	number   int
	name     string
	acctType GLAccountType
	isHeader bool
	isSystem bool
	fundID   *uuid.UUID
	parentNum int
}
```

Then replace all `"asset"` with `GLAccountTypeAsset`, `"liability"` with `GLAccountTypeLiability`, `"equity"` with `GLAccountTypeEquity`, `"revenue"` with `GLAccountTypeRevenue`, `"expense"` with `GLAccountTypeExpense` in the `defs` slice.

- [ ] **Step 2: Verify gl_service.go compiles**

Run: `cd backend && go vet ./internal/fin/gl_service.go 2>&1`

- [ ] **Step 3: Commit**

```bash
git add backend/internal/fin/gl_service.go
git commit -m "refactor(fin): use typed enum constants in gl_service.go (issue #79)"
```

---

### Task 5: Update `requests.go` to use `IsValid()` and typed enums

**Files:**
- Modify: `backend/internal/fin/requests.go`

- [ ] **Step 1: Replace switch-case validation with `IsValid()` calls**

In `CreateAssessmentScheduleRequest.Validate()` (lines 30-36), replace:

```go
switch r.Frequency {
case "monthly", "quarterly", "annually", "semi_annually":
	// valid
case "":
	return api.NewValidationError("validation.required", "frequency", api.P("field", "frequency"))
default:
	return api.NewValidationError("validation.one_of", "frequency", api.P("field", "frequency"), api.P("values", "monthly, quarterly, annually, semi_annually"))
}
```

with:

```go
if r.Frequency == "" {
	return api.NewValidationError("validation.required", "frequency", api.P("field", "frequency"))
}
if !AssessmentFrequency(r.Frequency).IsValid() {
	return api.NewValidationError("validation.one_of", "frequency", api.P("field", "frequency"), api.P("values", "monthly, quarterly, annually, semi_annually"))
}
```

Apply the same pattern to `AmountStrategy` (lines 38-44):

```go
if r.AmountStrategy == "" {
	return api.NewValidationError("validation.required", "amount_strategy", api.P("field", "amount_strategy"))
}
if !AmountStrategy(r.AmountStrategy).IsValid() {
	return api.NewValidationError("validation.one_of", "amount_strategy", api.P("field", "amount_strategy"), api.P("values", "flat, per_unit_type, per_sqft, custom"))
}
```

In `CreateFundRequest.Validate()` (lines 157-163), replace:

```go
switch r.FundType {
case "operating", "reserve", "capital", "special":
	// valid
case "":
	return api.NewValidationError("validation.required", "fund_type", api.P("field", "fund_type"))
default:
	return api.NewValidationError("validation.one_of", "fund_type", api.P("field", "fund_type"), api.P("values", "operating, reserve, capital, special"))
}
```

with:

```go
if r.FundType == "" {
	return api.NewValidationError("validation.required", "fund_type", api.P("field", "fund_type"))
}
if !FundType(r.FundType).IsValid() {
	return api.NewValidationError("validation.one_of", "fund_type", api.P("field", "fund_type"), api.P("values", "operating, reserve, capital, special"))
}
```

In `CreateGLAccountRequest.Validate()` (lines 276-301), replace the account type switch with `IsValid()` for the type check, but keep the account number range checks. Restructure as:

```go
if r.AccountType == "" {
	return api.NewValidationError("validation.required", "account_type", api.P("field", "account_type"))
}
if !GLAccountType(r.AccountType).IsValid() {
	return api.NewValidationError("validation.one_of", "account_type", api.P("field", "account_type"), api.P("values", "asset, liability, equity, revenue, expense"))
}
switch GLAccountType(r.AccountType) {
case GLAccountTypeAsset:
	if r.AccountNumber < 1000 || r.AccountNumber > 1999 {
		return api.NewValidationError("validation.constraint", "account_number", api.P("field", "account_number"), api.P("constraint", "between 1000 and 1999 for asset accounts"))
	}
case GLAccountTypeLiability:
	if r.AccountNumber < 2000 || r.AccountNumber > 2999 {
		return api.NewValidationError("validation.constraint", "account_number", api.P("field", "account_number"), api.P("constraint", "between 2000 and 2999 for liability accounts"))
	}
case GLAccountTypeEquity:
	if r.AccountNumber < 3000 || r.AccountNumber > 3999 {
		return api.NewValidationError("validation.constraint", "account_number", api.P("field", "account_number"), api.P("constraint", "between 3000 and 3999 for equity accounts"))
	}
case GLAccountTypeRevenue:
	if r.AccountNumber < 4000 || r.AccountNumber > 4999 {
		return api.NewValidationError("validation.constraint", "account_number", api.P("field", "account_number"), api.P("constraint", "between 4000 and 4999 for revenue accounts"))
	}
case GLAccountTypeExpense:
	if r.AccountNumber < 5000 || r.AccountNumber > 9999 {
		return api.NewValidationError("validation.constraint", "account_number", api.P("field", "account_number"), api.P("constraint", "between 5000 and 9999 for expense accounts"))
	}
}
```

Also update `UpdateCollectionRequest.Status` (line 200) from `*string` to `*CollectionCaseStatus`.

- [ ] **Step 2: Verify requests.go compiles**

Run: `cd backend && go vet ./internal/fin/requests.go 2>&1`

- [ ] **Step 3: Commit**

```bash
git add backend/internal/fin/requests.go
git commit -m "refactor(fin): use IsValid() for enum validation in requests.go (issue #79)"
```

---

### Task 6: Update `handler_collection.go` to use enum constants

**Files:**
- Modify: `backend/internal/fin/handler_collection.go`

- [ ] **Step 1: Replace bare strings in handler_collection.go**

Lines 97-98, replace:

```go
(*req.Status == "closed" || *req.Status == "resolved")
```

with:

```go
(*req.Status == CollectionCaseStatusClosed || *req.Status == CollectionCaseStatusResolved)
```

- [ ] **Step 2: Commit**

```bash
git add backend/internal/fin/handler_collection.go
git commit -m "refactor(fin): use typed enum constants in handler_collection.go (issue #79)"
```

---

### Task 7: Update `collection_postgres.go` and `budget_postgres.go` to use enum constants

**Files:**
- Modify: `backend/internal/fin/collection_postgres.go`
- Modify: `backend/internal/fin/budget_postgres.go`

- [ ] **Step 1: Update `collection_postgres.go`**

Line 171, replace:

```go
triggeredBy := "system"
```

with:

```go
triggeredBy := TriggeredBySystem
```

Line 172, update the nil check — `a.TriggeredBy` is now `*TriggeredBy`, so:

```go
if a.TriggeredBy != nil {
	triggeredBy = *a.TriggeredBy
}
```

This stays the same since the type changed.

- [ ] **Step 2: Update `budget_postgres.go`**

Lines 361-365, replace:

```go
fundType := "operating"
if e.FundType != nil {
	fundType = *e.FundType
}
```

with:

```go
fundType := FundTypeOperating
if e.FundType != nil {
	fundType = *e.FundType
}
```

- [ ] **Step 3: Verify both files compile**

Run: `cd backend && go vet ./internal/fin/collection_postgres.go ./internal/fin/budget_postgres.go 2>&1`

- [ ] **Step 4: Commit**

```bash
git add backend/internal/fin/collection_postgres.go backend/internal/fin/budget_postgres.go
git commit -m "refactor(fin): use typed enum constants in repository files (issue #79)"
```

---

### Task 8: Update all test files to use enum constants

**Files:**
- Modify: `backend/internal/fin/service_test.go`
- Modify: `backend/internal/fin/domain_test.go`
- Modify: `backend/internal/fin/gl_service_test.go`
- Modify: `backend/internal/fin/handler_budget_test.go`
- Modify: `backend/internal/fin/handler_assessment_test.go`
- Modify: `backend/internal/fin/handler_payment_test.go`
- Modify: `backend/internal/fin/handler_collection_test.go`
- Modify: `backend/internal/fin/handler_fund_test.go`
- Modify: `backend/internal/fin/handler_gl_test.go`
- Modify: `backend/internal/fin/fund_postgres_test.go`
- Modify: `backend/internal/fin/collection_postgres_test.go`
- Modify: `backend/internal/fin/budget_postgres_test.go`
- Modify: `backend/internal/fin/assessment_postgres_test.go`
- Modify: `backend/internal/fin/payment_postgres_test.go`

This is a mechanical replacement task. For every test file, find all bare string literals that match enum values and replace them with the typed constants. The compiler will guide you — any remaining bare string assigned to a typed field will fail to compile.

- [ ] **Step 1: Fix all compile errors in test files**

Use the compiler as your guide:

Run: `cd backend && go build ./internal/fin/... 2>&1`

For each error, replace the bare string with the appropriate constant. Common patterns:

- `Status: "draft"` → `Status: BudgetStatusDraft`
- `Status: "pending"` → `Status: ExpenseStatusPending`
- `Status: "completed"` → `Status: PaymentStatusCompleted`
- `Status: "active"` → `Status: PaymentPlanStatusActive`
- `Status: "late"` → `Status: CollectionCaseStatusLate`
- `FundType: "operating"` → `FundType: FundTypeOperating`
- `EntryType: "charge"` → `EntryType: LedgerEntryTypeCharge`
- `AccountType: "asset"` → `AccountType: GLAccountTypeAsset`
- `MethodType: "card"` → `MethodType: PaymentMethodTypeCard`
- `ActionType: "notice_sent"` → `ActionType: CollectionActionTypeNoticeSent`
- `CategoryType: "expense"` → `CategoryType: BudgetCategoryTypeExpense`
- `Frequency: "monthly"` → `Frequency: PaymentPlanFreqMonthly` or `AssessmentFreqMonthly` (context-dependent)
- `SourceType: "assessment"` → use `GLSourceType` pointer pattern
- `TriggeredBy: "system"` → use `TriggeredBy` pointer pattern

For string assertions in tests (e.g., `assert.Equal(t, "draft", budget.Status)`), update to use the constant: `assert.Equal(t, BudgetStatusDraft, budget.Status)`.

For JSON test bodies that send `"status": "draft"` in request payloads, these stay as strings since they represent API input — only change struct field assignments and assertions.

- [ ] **Step 2: Run the full test suite**

Run: `cd backend && go test ./internal/fin/... -short -count=1`

Expected: ALL tests pass.

- [ ] **Step 3: Commit**

```bash
git add backend/internal/fin/*_test.go
git commit -m "refactor(fin): use typed enum constants in all test files (issue #79)"
```

---

### Task 9: Final verification and lint

**Files:** None (verification only)

- [ ] **Step 1: Run full test suite**

Run: `cd backend && go test ./internal/fin/... -short -count=1`

Expected: ALL tests pass.

- [ ] **Step 2: Run linter**

Run: `make lint`

Expected: No new lint errors.

- [ ] **Step 3: Grep for any remaining bare enum strings in non-test Go files**

Run a search for any remaining bare strings that should be constants:

```bash
cd backend && grep -rn '"draft"\|"proposed"\|"approved"\|"pending"\|"paid"\|"completed"\|"late"\|"closed"\|"resolved"\|"active"\|"defaulted"\|"operating"\|"reserve"\|"capital"\|"special"\|"charge"\|"adjustment"\|"late_fee"\|"assessment"\|"transfer"\|"manual"\|"notice_sent"\|"lien_filed"' internal/fin/*.go --include='*.go' | grep -v '_test.go' | grep -v 'enums.go'
```

Expected: Only hits in JSON string contexts (API error messages with interpolation params, comments) — no bare strings used as enum values in assignments or comparisons.

- [ ] **Step 4: Final commit if any stragglers were found and fixed**

Only if Step 3 found remaining bare strings that needed replacement.
