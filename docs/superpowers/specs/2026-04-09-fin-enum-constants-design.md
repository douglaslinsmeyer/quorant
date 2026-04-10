# Fin Module: Typed String Enum Constants

**Issue:** #79 — Add domain enum constants, replace bare strings throughout fin module  
**Date:** 2026-04-09  
**Status:** Approved

## Problem

Financial statuses, entry types, fund types, and other domain concepts are bare strings throughout the fin module. Typos compile fine, there's no type safety, and validation logic is scattered across `requests.go`, `service.go`, and handler files.

## Approach

Introduce named string types with `const` blocks and `IsValid()` methods in a new `enums.go` file. Update all struct fields to use the typed enums. Replace all bare string literals and scattered validation with the constants and `IsValid()` calls.

## New File: `backend/internal/fin/enums.go`

### Enum Types

| Type | Const Prefix | Values |
|------|-------------|--------|
| `BudgetStatus` | `BudgetStatus` | `draft`, `proposed`, `approved` |
| `ExpenseStatus` | `ExpenseStatus` | `pending`, `approved`, `paid` |
| `PaymentStatus` | `PaymentStatus` | `completed` |
| `CollectionCaseStatus` | `CollectionCaseStatus` | `late`, `closed`, `resolved` |
| `PaymentPlanStatus` | `PaymentPlanStatus` | `active`, `defaulted` |
| `FundType` | `FundType` | `operating`, `reserve`, `capital`, `special` |
| `LedgerEntryType` | `LedgerEntryType` | `charge`, `payment`, `credit`, `adjustment`, `late_fee` |
| `GLSourceType` | `GLSourceType` | `assessment`, `payment`, `transfer`, `manual` |
| `LedgerReferenceType` | `LedgerRefType` | `payment` |
| `PaymentMethodType` | `PaymentMethodType` | `card`, `ach` |
| `CollectionActionType` | `CollectionActionType` | `notice_sent`, `lien_filed` |
| `GLAccountType` | `GLAccountType` | `asset`, `liability`, `equity`, `revenue`, `expense` |
| `AssessmentFrequency` | `AssessmentFreq` | `monthly`, `quarterly`, `semi_annually`, `annually` |
| `PaymentPlanFrequency` | `PaymentPlanFreq` | `monthly` |
| `AmountStrategy` | `AmountStrategy` | `flat`, `per_unit_type`, `per_sqft`, `custom` |
| `BudgetCategoryType` | `BudgetCategoryType` | `expense` |
| `TriggeredBy` | `TriggeredBy` | `system`, `user` |

### Pattern

Each enum type follows this structure:

```go
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
```

## Files Modified

### Struct field type changes (`domain.go`)

All struct fields that currently use `string` for these domain concepts will be updated to their typed enum. For example:

- `Budget.Status string` → `Budget.Status BudgetStatus`
- `Fund.FundType string` → `Fund.FundType FundType`
- `LedgerEntry.EntryType string` → `LedgerEntry.EntryType LedgerEntryType`
- And so on for all 17 types

### Validation consolidation (`requests.go`)

Replace inline switch-case validation with `IsValid()` calls. For example:

```go
// Before
switch req.Frequency {
case "monthly", "quarterly", "semi_annually", "annually":
default:
    return api.NewValidationError(...)
}

// After
if !AssessmentFrequency(req.Frequency).IsValid() {
    return api.NewValidationError(...)
}
```

### Business logic (`service.go`, `gl_service.go`, `handler_collection.go`)

Replace all bare string comparisons and assignments with constants:

```go
// Before
if budget.Status != "draft" { ... }
entry.SourceType = "assessment"

// After
if budget.Status != BudgetStatusDraft { ... }
entry.SourceType = GLSourceTypeAssessment
```

### Repository files (`collection_postgres.go`, `budget_postgres.go`)

Replace bare strings with constants where they appear.

### Test files

All test files updated to use constants instead of string literals.

## What Doesn't Change

- **Database values**: Same strings stored — no migration needed
- **JSON serialization**: Go marshals typed strings identically to plain strings
- **API contracts**: Same string values over the wire
- **Postgres scanning**: `pgx` scans strings into named string types without custom `Scan` implementations

## Risks

- **None significant.** Named string types in Go are assignment-compatible with their underlying type for constants. Database scanning and JSON marshaling work transparently.
- The only compile errors introduced are intentional — places where a wrong-type string was being assigned, which is exactly what we want to catch.
