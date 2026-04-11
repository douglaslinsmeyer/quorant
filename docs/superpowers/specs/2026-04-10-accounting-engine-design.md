# Accounting Engine Design Spec

**Issue:** #88
**Date:** 2026-04-10
**Status:** Approved

## Summary

Extract financial transaction and account logic from `FinService` into a dedicated accounting engine with pluggable drivers that implement different accounting standards (GAAP, IFRS, etc.). The engine is the single authority for all standard-driven financial behavior: journal entry construction, fund transaction directives, ledger entry directives, AP recognition and timing, payment application rules, revenue recognition, expense categorization, period-close constraints, and year-end closing. Business operations describe _what_ happened; the engine decides _how_ to record, validate, and apply it.

The engine is built on a complete, correct GAAP accounting foundation — not a cherry-picked subset for HOA use. The HOA domain is the first consumer, but the engine implements proper corporate accounting standards that any organization type could use.

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Spec scope | All 4 phases | Comprehensive design, phased implementation |
| Engine dependency model | `AccountResolver` + `policy.Registry` | Engine has enough autonomy for complete decisions; queries accounts by number, consults policies for non-deterministic rules |
| Policy interaction | Engine is entry point; delegates to policy for probabilistic/org-governance decisions | Deterministic rules (journal lines, recognition dates) stay in engine; org governance (payment application order) delegates to policy; jurisdiction constraints override both |
| Payment allocation separation | Engine returns strategy descriptor; `allocation.go` executes arithmetic | Clean separation: engine decides _what_ strategy; allocation code computes _how_ to distribute |
| Per-org engine selection | `EngineFactory` resolves org -> standard -> configured engine | Supports per-org configuration beyond just standard selection (recognition basis, fiscal year) |
| Engine output scope | Unified `FinancialEffects` bundle (GL + fund txns + ledger entries) | Engine is single source of truth for all recording effects; solves GL-to-ledger convergence (#83) |
| Interface architecture | Single wide interface (8 methods) | Every accounting standard must have an opinion on all capabilities; no realistic scenario where a driver implements recording but not validation |
| Recognition basis | Configurable per-org as engine config (cash, accrual, modified_accrual) | Not a standard difference — a method choice within a standard; stored with effective dating |
| Existing engine code | Prototype to replace | Current interface doesn't match decisions (resolver-based, factory-based, policy-aware, FinancialEffects output) |
| Accounting foundation | Complete GAAP, not HOA-subset | Engine implements proper corporate accounting; chart of accounts is a real standard chart |

## Core Interface

### AccountingEngine

```go
type AccountingEngine interface {
    // Identity
    Standard() AccountingStandard
    ChartOfAccounts() []GLAccountSeed

    // Recording & Validation
    RecordTransaction(ctx context.Context, tx FinancialTransaction) (*FinancialEffects, error)
    ValidateTransaction(ctx context.Context, tx FinancialTransaction) error

    // Payment Behavior
    PaymentApplicationStrategy(ctx context.Context, pc PaymentContext) (*ApplicationStrategy, error)
    PaymentTerms(ctx context.Context, pc PayableContext) (*PaymentTermsResult, error)

    // Recognition Timing
    PayableRecognitionDate(ctx context.Context, ec ExpenseContext) (time.Time, error)
    RevenueRecognitionDate(ctx context.Context, tx FinancialTransaction) (time.Time, error)
}
```

### AccountResolver

The engine's data dependency for account lookups. Intentionally narrow — one method. The existing `GLPostgresRepository` satisfies it implicitly.

```go
type AccountResolver interface {
    FindAccountByOrgAndNumber(ctx context.Context, orgID uuid.UUID, number int) (*GLAccount, error)
}
```

System accounts are seeded by `ChartOfAccounts()` with `IsSystem=true` and cannot be renumbered, so number-based lookup is reliable. The GAAP driver seeds account 1100 and references 1100 — the mapping is self-consistent.

### EngineFactory

Resolves an org to its configured engine instance using effective-dated configuration.

```go
type EngineFactory struct {
    builders   map[AccountingStandard]EngineBuilder
    configRepo OrgAccountingConfigRepository
}

// EngineBuilder constructs an engine for a given config. Dependencies like
// AccountResolver and policy.Registry are captured in the closure at startup.
type EngineBuilder func(config EngineConfig) AccountingEngine

func NewEngineFactory(
    builders map[AccountingStandard]EngineBuilder,
    configRepo OrgAccountingConfigRepository,
) *EngineFactory

func (f *EngineFactory) ForOrg(ctx context.Context, orgID uuid.UUID) (AccountingEngine, error)
func (f *EngineFactory) ForOrgAtDate(ctx context.Context, orgID uuid.UUID, date time.Time) (AccountingEngine, error)
```

`ForOrgAtDate` resolves the config that was effective on a specific date, ensuring historical transactions use the config active when they were recorded.

Wiring at startup:
```go
// main.go
gaapBuilder := func(cfg fin.EngineConfig) fin.AccountingEngine {
    return fin.NewGaapEngine(glRepo, policyRegistry, cfg)
}
factory := fin.NewEngineFactory(
    map[fin.AccountingStandard]fin.EngineBuilder{
        fin.AccountingStandardGAAP: gaapBuilder,
    },
    orgConfigRepo,
)
```

### EngineConfig

```go
type EngineConfig struct {
    RecognitionBasis      RecognitionBasis
    FiscalYearStart       time.Month
    AvailabilityPeriodDays int  // modified accrual: revenue available if collectible within N days of period end (typically 60)
}

type RecognitionBasis string
const (
    RecognitionBasisCash           RecognitionBasis = "cash"
    RecognitionBasisAccrual        RecognitionBasis = "accrual"
    RecognitionBasisModifiedAccrual RecognitionBasis = "modified_accrual"
)
```

`AvailabilityPeriodDays` is only used under modified accrual basis. Ignored for cash and accrual.

## Types

### FinancialTransaction (inbound envelope)

```go
type FinancialTransaction struct {
    Type               TransactionType
    OrgID              uuid.UUID
    AmountCents        int64
    EffectiveDate      time.Time
    SourceID           uuid.UUID
    UnitID             *uuid.UUID
    FundAllocations    []FundAllocation     // split-fund support (assessments, expenses)
    PaymentAllocations []PaymentAllocation  // how payment was applied (payments only)
    ExternalRef        *ExternalReference   // check number, ACH trace, gateway ID, lockbox batch
    CollectionSource   CollectionSource     // hoa_direct or third_party (FDCPA boundary)
    Memo               string
    Metadata           map[string]any       // driver-specific context; avoid for core fields
}

type ExternalReference struct {
    Type       string // "check", "ach", "gateway", "lockbox", "wire"
    ExternalID string // check number, ACH trace number, gateway transaction ID, etc.
    BatchID    *string // lockbox or ACH batch identifier, if applicable
}

type CollectionSource string
const (
    CollectionSourceDirect     CollectionSource = "hoa_direct"
    CollectionSourceThirdParty CollectionSource = "third_party"
)

type FundAllocation struct {
    FundID      uuid.UUID
    AmountCents int64
}
```

`FundAllocations` supports split-fund assessments (e.g., $200 operating + $50 reserve in a single assessment). For single-fund transactions, the slice has one entry.

`PaymentAllocations` is populated for `TxTypePayment` only — carries the result of `allocation.Apply()` so the engine knows which funds are affected by the payment. This is a typed field rather than untyped metadata to keep the engine's input contract explicit.

`Metadata` is reserved for driver-specific context that doesn't warrant a typed field (e.g., project-completion percentage for special assessment revenue recognition). Core financial data should always use typed fields.

### TransactionType

```go
type TransactionType string
const (
    TxTypeAssessment        TransactionType = "assessment"
    TxTypePayment           TransactionType = "payment"
    TxTypeFundTransfer      TransactionType = "fund_transfer"
    TxTypeInterfundLoan     TransactionType = "interfund_loan"
    TxTypeExpense           TransactionType = "expense"
    TxTypeLateFee           TransactionType = "late_fee"
    TxTypeInterestAccrual   TransactionType = "interest_accrual"
    TxTypeBadDebtProvision  TransactionType = "bad_debt_provision"
    TxTypeBadDebtWriteOff   TransactionType = "bad_debt_write_off"
    TxTypeYearEndClose      TransactionType = "year_end_close"
    TxTypeBadDebtRecovery   TransactionType = "bad_debt_recovery"
    TxTypeVoidReversal      TransactionType = "void_reversal"
    TxTypeAdjustingEntry    TransactionType = "adjusting_entry"
    TxTypeDepreciation      TransactionType = "depreciation"
)
```

### FinancialEffects (outbound bundle)

The unified output from `RecordTransaction`. A closed, enumerable set of directives that FinService executes mechanically — no interpretation, no branching on opaque data.

```go
type FinancialEffects struct {
    JournalLines     []GLJournalLine
    FundTransactions []FundTransactionDirective
    LedgerEntries    []LedgerEntryDirective
    Credits          []CreditDirective           // overpayment handling
    DeferralSchedule *DeferralSchedule           // prepaid revenue recognition
}
```

Every directive type maps 1:1 to a repository operation FinService already has. If the engine needs a new kind of effect, that's a new directive type — not creative use of an existing one.

### Directive Types

```go
type FundTransactionDirective struct {
    FundID      uuid.UUID
    Type        FundTransactionType
    AmountCents int64
    Description string
}

type LedgerEntryDirective struct {
    UnitID      uuid.UUID
    Type        LedgerEntryType
    AmountCents int64
    Description string
    SourceID    uuid.UUID
}

type CreditDirective struct {
    UnitID      uuid.UUID
    AmountCents int64
    Type        CreditType  // credit_on_account, prepayment
}

type CreditType string
const (
    CreditTypeOnAccount  CreditType = "credit_on_account"
    CreditTypePrepayment CreditType = "prepayment"
)

type DeferralSchedule struct {
    DeferredAccountNumber int
    RevenueAccountNumber  int
    TotalAmountCents      int64
    Entries               []DeferralEntry
}

type DeferralEntry struct {
    RecognitionDate time.Time
    AmountCents     int64
}
```

### Payment Context Types

```go
type PaymentContext struct {
    OrgID             uuid.UUID
    PaymentID         uuid.UUID
    PayerID           uuid.UUID
    AmountCents       int64
    DesignatedInvoice *uuid.UUID
    OutstandingItems  []ReceivableItem
}

type ReceivableItem struct {
    ChargeID         uuid.UUID
    ChargeType       ChargeType
    AmountCents      int64
    OutstandingCents int64
    DueDate          time.Time
    FundID           *uuid.UUID
    LienPriority     LienPriority // superlien-eligible vs subordinate (NV, CO, CT, DC, DE)
}

type LienPriority string
const (
    LienPriorityStandard  LienPriority = "standard"
    LienPrioritySuperlien LienPriority = "superlien" // precludes first mortgage (up to 9 months)
)

type ExpenseContext struct {
    InvoiceDate  time.Time
    ServiceDate  *time.Time
    ApprovalDate *time.Time
    VendorTerms  string
    ExpenseType  ExpenseType
    AmountCents  int64
}

type PayableContext struct {
    PayableID   uuid.UUID
    InvoiceDate time.Time
    VendorTerms string
    AmountCents int64
}
```

### Result Types

```go
type ApplicationStrategy struct {
    Method         ApplicationMethod
    PriorityOrder  []ChargeType
    WithinPriority SortOrder
}

type ApplicationMethod string
const (
    ApplicationMethodOldestFirst       ApplicationMethod = "oldest_first"
    ApplicationMethodProportional      ApplicationMethod = "proportional"
    ApplicationMethodDesignated        ApplicationMethod = "designated"
    ApplicationMethodCreditorFavorable ApplicationMethod = "creditor_favorable"
    ApplicationMethodPriorityFIFO      ApplicationMethod = "priority_fifo"
)

type SortOrder string
const (
    SortOldestFirst SortOrder = "oldest_first"
    SortNewestFirst SortOrder = "newest_first"
)

type PaymentTermsResult struct {
    DueDate         time.Time
    DiscountDate    *time.Time
    DiscountPercent *float64
}
```

## Domain Model Additions

### AccountingPeriod

Required for `ValidateTransaction` to enforce period boundaries.

```go
type AccountingPeriod struct {
    ID           uuid.UUID
    OrgID        uuid.UUID
    FiscalYear   int
    PeriodNumber int           // 1-12 (or 1-13 for short periods)
    StartDate    time.Time
    EndDate      time.Time
    Status       PeriodStatus
    ClosedBy     *uuid.UUID
    ClosedAt     *time.Time
}

type PeriodStatus string
const (
    PeriodStatusOpen       PeriodStatus = "open"
    PeriodStatusSoftClosed PeriodStatus = "soft_closed"
    PeriodStatusClosed     PeriodStatus = "closed"
)
```

- **Open:** All transactions allowed.
- **Soft-closed:** Only adjusting entries (marked `TxTypeAdjustingEntry`) allowed.
- **Closed:** No transactions allowed.

### OrgAccountingConfig

Effective-dated, versioned accounting configuration per org.

```go
type OrgAccountingConfig struct {
    ID               uuid.UUID
    OrgID            uuid.UUID
    Standard         AccountingStandard
    RecognitionBasis RecognitionBasis
    FiscalYearStart  time.Month
    EffectiveDate    time.Time
    CreatedAt        time.Time
    CreatedBy        uuid.UUID
}
```

Configuration changes are append-only — new rows, never updates. The `EngineFactory` resolves the effective config for a given date.

Basis changes are only valid at fiscal year boundaries. `ValidateTransaction` rejects transactions that would post under a different basis than the one effective for their period.

## GAAP Driver Design

### Structure

```go
type GaapEngine struct {
    resolver AccountResolver
    registry *policy.Registry
    config   EngineConfig
}

func NewGaapEngine(resolver AccountResolver, registry *policy.Registry, config EngineConfig) *GaapEngine
```

### RecordTransaction — per transaction type

**Assessment (accrual basis):**
```
GL:     DR 1100 (AR-Assessments) / CR Revenue (fund-dependent: 4010 operating, 4020 reserve)
Fund:   Credit to each allocated fund for its portion
Ledger: Charge entry on the unit's ledger
```
Split-fund assessments produce multi-line revenue entries. For example, $200 operating + $50 reserve:
```
DR 1100 AR          $250
CR 4010 Revenue-Op  $200
CR 4020 Revenue-Res  $50
```

**Assessment (cash basis):**
```
GL:     No journal entry — AR not recognized
Fund:   No fund transaction — revenue not yet earned
Ledger: Charge entry on the unit's ledger (tracks obligation, not GL)
```

**Payment (accrual basis):**
```
GL:     DR Cash (fund-dependent) / CR 1100 (AR-Assessments)
Fund:   Debit to the fund(s) the paid charges belong to
Ledger: Payment entry on the unit's ledger
```
For payments split across funds (from allocation results), produces multiple GL lines and fund transaction directives.

**Payment (cash basis):**
```
GL:     DR Cash (fund-dependent) / CR Revenue (fund-dependent) — revenue recognized on receipt
Fund:   Credit to fund(s) for revenue
Ledger: Payment entry on the unit's ledger
```

**Fund Transfer:**
```
GL:     DR 3100 (Interfund Out), CR source Cash, DR destination Cash, CR 3110 (Interfund In)
Fund:   Withdrawal from source fund, deposit to destination fund
Ledger: None — fund transfers don't affect unit balances
```

**Interfund Borrowing (operating borrows from reserve):**
```
GL:     DR 1010 (Cash-Operating) / CR 1020 (Cash-Reserve)
        DR 1300 (Due From Other Funds, on reserve) / CR 2500 (Due To Other Funds, on operating)
Fund:   Loan-out from reserve fund, loan-in to operating fund
Ledger: None
```
The 1300/2500 entries create a balance-sheet-level receivable/payable between funds, required for CIRA fund-columnar financial statements. These are distinct from interfund transfers (3100/3110) which are permanent equity movements.

**Expense (accrual basis):**
```
Approved:  DR 5xxx (Expense, category-dependent) / CR 2100 (AP)
Paid:      DR 2100 (AP) / CR Cash (fund-dependent)
```

**Expense (cash basis):**
```
Paid:      DR 5xxx (Expense, category-dependent) / CR Cash (fund-dependent)
```
No AP recognition — single entry on payment.

**Late Fee:**
```
GL:     DR 1100 (AR) / CR 4100 (Late Fee Revenue)
Fund:   Credit to operating fund
Ledger: Late fee charge on the unit's ledger
```

**Interest Accrual:**
```
GL:     DR 1100 (AR) / CR 4200 (Interest Income)
Fund:   Credit to operating fund
Ledger: Interest charge on the unit's ledger
```

**Bad Debt Provision:**
```
GL:     DR 5070 (Bad Debt Expense) / CR 1105 (Allowance for Doubtful Accounts)
```

**Bad Debt Write-off:**
```
GL:     DR 1105 (Allowance) / CR 1100 (AR)
Ledger: Adjustment entry on the unit's ledger
```

**Bad Debt Recovery (previously written-off amount collected):**
```
GL:     Step 1: DR 1100 (AR) / CR 1105 (Allowance) — reinstate receivable
        Step 2: DR Cash / CR 1100 (AR) — record payment against reinstated receivable
Ledger: Reinstatement entry + payment entry on the unit's ledger
```
Engine produces both steps as a single `FinancialEffects` bundle.

**Depreciation:**
```
GL:     DR 5220 (Depreciation Expense) / CR 1405 (Accumulated Depreciation)
```
For associations with capitalized common-area assets.

**Year-End Close (per fund):**
```
GL:     DR each Revenue account balance / CR Fund Balance (3010/3020/3030/3040)
        DR Fund Balance / CR each Expense account balance
        DR/CR Interfund Transfer Out (3100) to zero / CR/DR Fund Balance
        DR/CR Interfund Transfer In (3110) to zero / CR/DR Fund Balance
```
Each fund closes independently. Revenue, expense, AND interfund transfer accounts (3100/3110) are all temporary accounts that must be zeroed to the fund's balance account. If interfund accounts are not closed, fund balances will be misstated. Requires all periods in the fiscal year to be closed first.

**Void/Reversal:**
```
GL:     Mirror of original entry with IsReversal=true
Fund:   Reversal of original fund transactions
Ledger: Reversal entry on the unit's ledger
```
Validates original hasn't already been reversed.

### RecordTransaction — modified accrual basis

Modified accrual follows accrual rules for most transactions with one key difference: revenue is recognized only when "measurable and available" — collectible within `AvailabilityPeriodDays` of the accounting period end.

- **Assessment:** If assessment due date is within the availability window of the period end, treat as accrual (DR AR / CR Revenue). If due date is beyond the availability window, defer recognition — DR AR / CR 2200 (Deferred Revenue). Revenue is recognized when the assessment enters the availability window.
- **Payment:** Same as accrual basis (DR Cash / CR AR).
- **Expense:** Same as accrual basis (DR Expense / CR AP on approval, DR AP / CR Cash on payment).
- **Late Fee / Interest:** Same as accrual basis — recognize when charged if amount is determinable.
- **All other types:** Follow accrual basis rules.

### ValidateTransaction

- Amount must be positive (except adjusting entries)
- Required fields present per transaction type (UnitID for assessments, FundAllocations for transfers)
- EffectiveDate must fall in an open period (or soft-closed for adjusting entries)
- Immutability: reject modification of any posted financial record
- Basis-change boundary: reject if transaction would post under a different basis than the period's effective config
- For reversals: original must exist and not already be reversed
- **Late fee cap enforcement:** For `TxTypeLateFee`, consult `policy.Registry` for jurisdiction-scoped fee caps (CA: greater of $10 or 10% per Civil Code 5650; FL: as specified in governing documents). Reject amounts exceeding the statutory maximum.
- **Interest rate cap enforcement:** For `TxTypeInterestAccrual`, consult `policy.Registry` for jurisdiction-scoped interest rate caps (FL: 18% annually per FS 718.116(6)(b); TX: "reasonable" per Property Code 209.0064). Reject amounts exceeding the statutory maximum.

### PaymentApplicationStrategy

1. If `DesignatedInvoice` is set -> return `Designated` strategy
2. Consult `policy.Registry` for jurisdiction-scoped payment application rules:
   - CA Civil Code 5655 (assessments before fees unless written agreement)
   - FL FS 718.116 / 720.3085 (per declaration, or oldest-first if silent)
   - TX Property Code 209.0064 (current assessments, then delinquent, then fees)
   - CO CCIOA 38-33.3-316.3
   - NV NRS 116.3115 (superlien priority for first mortgage holders)
   - IL 765 ILCS 160/1-45 (assessments before fees)
   - VA 55.1-1964 (prescribed priority order)
   - MD Real Property 11B-117 (collection cost restrictions affecting priority)
   - WA RCW 64.90.485 (prescribed priority for CICs)
3. If jurisdiction mandate found -> build strategy constrained by statute
4. If receivables include superlien-eligible items (NV, CO, CT, DC, DE) and payment source is a mortgage foreclosure or insurance payout -> segregate superlien amounts per `LienPriority` on `ReceivableItem`
5. Consult `policy.Registry` for org-level payment application policy
6. If org policy found -> validate it doesn't violate jurisdiction constraints, build strategy
7. If no policy -> default to oldest-first (GAAP common practice)

### PaymentTerms

Deterministic — parses `VendorTerms` string:
- "Net 30" -> DueDate = InvoiceDate + 30 days
- "2/10 Net 30" -> DueDate = +30, DiscountDate = +10, DiscountPercent = 2%
- "Net 60" -> DueDate = InvoiceDate + 60 days
- Empty/unknown -> default Net 30

### PayableRecognitionDate

- Accrual basis: recognize when obligation is incurred. If `ServiceDate` set, use service delivery. Otherwise, `InvoiceDate`.
- Cash basis: returns `ErrCashBasisNoPayable` — under cash basis, payables are not recognized as liabilities. FinService skips AP creation entirely when it receives this error.
- Modified accrual: recognize when measurable and available (due within `AvailabilityPeriodDays` of the accounting period end date)

### RevenueRecognitionDate

ASC 606 framework:
- Regular monthly assessments -> recognize in the assessment period
- Prepaid annual assessments (accrual) -> defer; return a `DeferralSchedule` on `FinancialEffects` with monthly recognition entries
- Special assessments for capital projects -> recognize on project completion or proportional progress (metadata-driven)
- Late fees / interest -> recognize when charged
- Cash basis -> recognize when cash received (all types)

### Policy Delegation Summary

| Method | Deterministic? | Policy delegation? |
|--------|---------------|-------------------|
| Standard | Yes | No |
| ChartOfAccounts | Yes | No |
| RecordTransaction | Yes — standard defines journal rules | No |
| ValidateTransaction | Yes — period close, required fields, immutability | No |
| PaymentApplicationStrategy | No — jurisdiction + org governance | Yes — consults registry |
| PaymentTerms | Yes — contractual, parsed from terms string | No |
| PayableRecognitionDate | Yes — standard's accrual rules | No |
| RevenueRecognitionDate | Mostly — edge cases for project-based specials | Possibly — for complex recognition |

## Chart of Accounts (GAAP Standard)

Full GAAP chart — not an HOA subset. System accounts are `IsSystem=true` and immutable.

### Assets (1000-series)

| Number | Name | Type | Fund Key | Parent | Notes |
|--------|------|------|----------|--------|-------|
| 1000 | Assets | asset | | | Header |
| 1010 | Cash-Operating | asset | operating | 1000 | |
| 1020 | Cash-Reserve | asset | reserve | 1000 | |
| 1030 | Cash-Capital | asset | capital | 1000 | |
| 1040 | Cash-Special | asset | special | 1000 | |
| 1100 | Accounts Receivable-Assessments | asset | | 1000 | |
| 1105 | Allowance for Doubtful Accounts | asset | | 1000 | Contra-asset |
| 1110 | Accounts Receivable-Other | asset | | 1000 | |
| 1150 | Accrued Interest Receivable | asset | | 1000 | |
| 1200 | Prepaid Expenses | asset | | 1000 | |
| 1300 | Due From Other Funds | asset | | 1000 | Interfund borrowing receivable (CIRA) |
| 1400 | Fixed Assets | asset | | 1000 | Common area buildings/equipment |
| 1405 | Accumulated Depreciation | asset | | 1000 | Contra-asset |
| 1500 | Insurance Claim Receivable | asset | | 1000 | Receivable from insurance carrier |

### Liabilities (2000-series)

| Number | Name | Type | Fund Key | Parent | Notes |
|--------|------|------|----------|--------|-------|
| 2000 | Liabilities | liability | | | Header |
| 2100 | Accounts Payable | liability | | 2000 | |
| 2110 | Accrued Expenses | liability | | 2000 | |
| 2200 | Prepaid Assessments | liability | | 2000 | Deferred revenue |
| 2300 | Owner Deposits | liability | | 2000 | Security/move-in deposits |
| 2400 | Deferred Revenue-Other | liability | | 2000 | Non-assessment deferrals |
| 2500 | Due To Other Funds | liability | | 2000 | Interfund borrowing payable (CIRA) |
| 2600 | Income Tax Payable | liability | | 2000 | Form 1120-H tax on non-exempt income |

### Equity / Fund Balances (3000-series)

| Number | Name | Type | Fund Key | Parent | Notes |
|--------|------|------|----------|--------|-------|
| 3000 | Fund Balances | equity | | | Header |
| 3010 | Operating Fund Balance | equity | operating | 3000 | Unrestricted |
| 3020 | Reserve Fund Balance | equity | reserve | 3000 | Board-restricted |
| 3030 | Capital Fund Balance | equity | capital | 3000 | |
| 3040 | Special Fund Balance | equity | special | 3000 | |
| 3100 | Interfund Transfer Out | equity | | 3000 | Temporary — closed at year-end |
| 3110 | Interfund Transfer In | equity | | 3000 | Temporary — closed at year-end |

### Revenue (4000-series)

| Number | Name | Type | Fund Key | Parent | Notes |
|--------|------|------|----------|--------|-------|
| 4000 | Revenue | revenue | | | Header |
| 4010 | Assessment Revenue-Operating | revenue | operating | 4000 | |
| 4020 | Assessment Revenue-Reserve | revenue | reserve | 4000 | |
| 4030 | Assessment Revenue-Capital | revenue | capital | 4000 | |
| 4040 | Assessment Revenue-Special | revenue | special | 4000 | |
| 4100 | Late Fee Revenue | revenue | | 4000 | |
| 4200 | Interest Income | revenue | | 4000 | |
| 4310 | Facility Rental Income | revenue | | 4000 | Common area rentals |
| 4320 | Parking and Amenity Fees | revenue | | 4000 | |
| 4330 | Move-In/Move-Out Fees | revenue | | 4000 | |
| 4400 | Insurance Proceeds | revenue | | 4000 | Distinct from assessment revenue |
| 4900 | Other Income | revenue | | 4000 | Catch-all for miscellaneous |

### Expenses (5000-series)

| Number | Name | Type | Fund Key | Parent | Notes |
|--------|------|------|----------|--------|-------|
| 5000 | Operating Expenses | expense | | | Header |
| 5010 | Management Fee | expense | | 5000 | |
| 5020 | Insurance Premium | expense | | 5000 | |
| 5030 | Utilities | expense | | 5000 | |
| 5040 | Landscaping | expense | | 5000 | |
| 5050 | Maintenance and Repairs | expense | | 5000 | |
| 5060 | Professional Services | expense | | 5000 | Legal, audit, tax |
| 5070 | Bad Debt Expense | expense | | 5000 | |
| 5100 | Administrative Expenses | expense | | 5000 | Postage, office, bank fees |
| 5110 | Payroll and Salaries | expense | | 5000 | HOAs with direct employees |
| 5120 | Payroll Taxes and Benefits | expense | | 5000 | Employer-side taxes |
| 5200 | Reserve Expenses | expense | reserve | 5000 | Major repair/replacement |
| 5210 | Casualty Loss | expense | | 5000 | Property damage |
| 5220 | Depreciation Expense | expense | | 5000 | Fixed asset depreciation |
| 5300 | Insurance Deductible | expense | | 5000 | Deductible on claims |

**Total: 53 accounts** (6 headers, 47 detail accounts)

Account numbers in the 1300-1999, 2600-2999, 4500-4899, and 5400-5999 ranges are reserved for org-specific custom accounts created by management.

## FinService Refactoring

### Structural Change

FinService replaces `engine AccountingEngine` with `factory *EngineFactory`:

```go
type FinService struct {
    assessments AssessmentRepository
    payments    PaymentRepository
    budgets     BudgetRepository
    funds       FundRepository
    collections CollectionRepository
    gl          *GLService
    factory     *EngineFactory
    policy      ai.PolicyResolver
    compliance  ai.ComplianceResolver
    registry    *policy.Registry
    logger      *slog.Logger
    uowFactory  *db.UnitOfWorkFactory
}
```

### Orchestration Pattern

Every financial operation follows 4 steps:

```
1. Resolve engine    -> factory.ForOrg(ctx, orgID)
2. Business logic    -> create/validate the domain object
3. Engine decision   -> engine.RecordTransaction(ctx, ftx) -> FinancialEffects
4. Atomic commit     -> execute all effects in a single UoW
```

### executeEffects Helper

A single mechanical method used by all financial operations. No interpretation — just persists each directive:

```go
func (s *FinService) executeEffects(ctx context.Context, uow *db.UnitOfWork, effects *FinancialEffects) error {
    glRepo := s.gl.Repo().WithTx(uow.Tx())
    fundRepo := s.funds.WithTx(uow.Tx())
    assessRepo := s.assessments.WithTx(uow.Tx())

    if len(effects.JournalLines) > 0 {
        _, err := glRepo.PostJournalEntry(ctx, ...)
        if err != nil { return err }
    }
    for _, ft := range effects.FundTransactions {
        _, err := fundRepo.CreateTransaction(ctx, ...)
        if err != nil { return err }
    }
    for _, le := range effects.LedgerEntries {
        _, err := assessRepo.CreateLedgerEntry(ctx, ...)
        if err != nil { return err }
    }
    for _, cr := range effects.Credits {
        _, err := assessRepo.CreateLedgerEntry(ctx, ...) // credit on account
        if err != nil { return err }
    }
    if effects.DeferralSchedule != nil {
        // Persist deferred revenue entries as pending scheduled jobs.
        // The worker's scheduler picks these up and calls
        // engine.RecordTransaction(TxTypeAdjustingEntry) on each
        // RecognitionDate to move amounts from deferred to recognized revenue.
    }
    return nil
}
```

### Payment Orchestration (the complex case)

Payments involve the engine twice — once for strategy, once for recording:

```go
func (s *FinService) RecordPayment(ctx context.Context, ...) (*Payment, error) {
    engine, err := s.factory.ForOrg(ctx, orgID)

    uow, err := s.uowFactory.Begin(ctx)
    defer uow.Rollback(ctx)

    // 1. Create payment record
    payment, err := payRepo.CreatePayment(ctx, ...)

    // 2. Engine determines application strategy
    strategy, err := engine.PaymentApplicationStrategy(ctx, PaymentContext{...})

    // 3. Allocation executes the strategy mechanically
    allocations := allocation.Apply(strategy, outstanding, payment.AmountCents)

    // 4. Persist allocations
    for _, alloc := range allocations { ... }

    // 5. Engine records the financial effects
    ftx := FinancialTransaction{
        Type: TxTypePayment, OrgID: orgID,
        AmountCents: payment.AmountCents,
        EffectiveDate: payment.ReceivedAt,
        SourceID: payment.ID, UnitID: &payment.UnitID,
        PaymentAllocations: allocations, // typed field, not untyped metadata
    }
    effects, err := engine.RecordTransaction(ctx, ftx)

    // 6. Execute effects + commit
    s.executeEffects(ctx, uow, effects)
    s.publisher.PublishTx(ctx, uow.Tx(), event)
    return payment, uow.Commit(ctx)
}
```

### What Gets Removed from FinService

- All hardcoded account number references
- All inline GL journal line construction
- All inline fund transaction creation logic for financial events
- All inline ledger entry creation for financial events
- The `nil` guard clauses (`if s.engine != nil`)

### What Stays in FinService

- Domain object CRUD (create assessment, create payment, etc.)
- Orchestration flow (resolve engine -> business logic -> engine decision -> execute -> commit)
- Event publishing
- Non-financial operations (budget CRUD, collection case management, etc.)

### Allocation Separation

`allocation.go` stays as a separate concern:
- **Engine** returns `ApplicationStrategy` (method + priority order) — the _what_
- **allocation.Apply()** computes the actual distribution across charges — the _how_
- **FinService** calls engine for strategy, passes to allocation, passes results back to engine for GL recording

### Immutability Enforcement

All financial records are append-only:
- GL journal entries: already immutable (corrections via reversal entries)
- Ledger entries: append-only; corrections via adjustment entries (`LedgerEntryTypeAdjustment`)
- Fund transactions: append-only; corrections via reversal transactions
- `ValidateTransaction` rejects any attempt to modify a posted financial record

## Phase Breakdown

### Phase 1 — Engine Interface + GAAP Foundation

**Goal:** Replace the prototype engine with the full interface. Implement the GAAP driver for core recording with a proper chart of accounts. Establish the accounting period model and effective-dated configuration.

**Scope:**

Engine interface:
- All 8 methods defined
- Phase 1 implements: `Standard()`, `ChartOfAccounts()`, `RecordTransaction()`, `ValidateTransaction()`
- Remaining 4 methods return `ErrNotImplemented`

Transaction types:
- `TxTypeAssessment` (split-fund aware, recognition-basis aware)
- `TxTypePayment` (fund-aware based on allocations)
- `TxTypeFundTransfer` (4-line interfund entry with per-fund cash accounts)
- `TxTypeLateFee`
- `TxTypeInterestAccrual`

Infrastructure:
- `FinancialEffects` bundle + `executeEffects` helper
- `AccountResolver` interface (satisfied by GLPostgresRepository)
- `EngineFactory` + `EngineConfig` with `RecognitionBasis`
- `AccountingPeriod` entity + repository
- `OrgAccountingConfig` entity + repository (effective-dated)
- Full 53-account GAAP chart of accounts
- Cash, accrual, and modified accrual behavior in `RecordTransaction` for all Phase 1 transaction types

FinService refactoring:
- Replace `engine` field with `factory`
- Refactor `CreateAssessment`, `RecordPayment`, `CreateFundTransfer` to use engine + `executeEffects`
- Add late fee and interest accrual operations
- Remove all hardcoded account numbers and inline GL construction

Validation:
- Amount positive, required fields, period-open check, immutability enforcement

**Addresses:** #60 (atomic ops), #66 (GL error handling), #69 (expense GL entries), #70 (assessment revenue account), #83 (GL-to-ledger convergence)

### Phase 2 — Payment Application + AP Timing + Overpayment

**Goal:** Engine-driven payment behavior. Jurisdiction-aware application order. Complete AP lifecycle.

**Scope:**

Engine methods:
- `PaymentApplicationStrategy()` — consults policy.Registry for org/jurisdiction rules
- `PaymentTerms()` — parses vendor terms, computes due dates and discounts
- `PayableRecognitionDate()` — accrual: on incurrence; cash: on payment

Payment application:
- Jurisdiction-scoped policy records seeded for key states (CA, FL, TX, CO, NV)
- Engine validates org policy doesn't violate statutory constraints
- Strategy descriptor -> `allocation.Apply()` executes mechanically

Overpayment handling:
- `CreditDirective` on `FinancialEffects`
- Engine distinguishes: credit on account vs prepayment (deferred revenue under accrual)

Expense/AP lifecycle:
- `TxTypeExpense` — DR Expense / CR Cash or AP (based on status and recognition basis)
- Accrual: AP recognized on approval, cleared on payment
- Cash: single entry on payment, no AP

Immutability enforcement expansion:
- LedgerEntry and FundTransaction: append-only, corrections via adjustments/reversals

Bank reconciliation (minimum viable):
- `Reconciled` flag on GL journal lines for cash accounts
- `BankTransaction` entity for imported bank statement lines (date, amount, reference, matched journal line ID)
- Reconciliation workflow: match bank transactions to GL cash entries, flag unmatched items
- Required for any association undergoing annual audit

Custodian tracking:
- `CustodianType` attribute on funds or cash accounts: `association_held` vs `management_company_held`
- Required by FL (FS 468.432), CA (Business & Professions 11502), NV (NRS 116A.640) for management companies holding HOA funds in trust

### Phase 3 — Revenue Recognition + Closing + Void/Reversal

**Goal:** Complete the revenue lifecycle including deferrals. Year-end close. Void/reversal through the engine.

**Scope:**

Engine methods:
- `RevenueRecognitionDate()` — ASC 606 framework for all transaction types

Revenue recognition:
- Regular monthly assessments: recognize in period
- Prepaid annual assessments (accrual): deferred revenue with monthly recognition schedule
- Special assessments for capital projects: project-completion or proportional recognition
- Cash basis: all revenue recognized on receipt
- `DeferralSchedule` on `FinancialEffects` for prepaid assessments

Year-end closing:
- `TxTypeYearEndClose` — closing journal lines per-fund
- Zero out revenue, expense, AND interfund transfer accounts (3100/3110) to fund balance
- Each fund closes independently
- Requires all periods closed first
- Period 13 (adjusting-entry-only period) for year-end audit adjustments

Bad debt:
- `TxTypeBadDebtProvision` — DR Bad Debt Expense / CR Allowance
- `TxTypeBadDebtWriteOff` — DR Allowance / CR AR + ledger adjustment
- `TxTypeBadDebtRecovery` — reinstate AR + record payment (two-step in single FinancialEffects)
- Engine suggests provisions based on collection case escalation

Void/reversal:
- `TxTypeVoidReversal` — mirror of original entry with `IsReversal=true`
- Produces reversing `FinancialEffects` (GL + fund + ledger)
- Validates original exists and hasn't been reversed
- Supports: payment reversal, assessment void, expense void

Fiscal year configuration:
- Per-org, effective-dated
- Short-period support during fiscal year transitions
- `ValidateTransaction` enforces basis-change only at fiscal year boundary

**Addresses:** #75 (void/reversal)

### Phase 4 — IFRS Driver + Per-Org Engine Selection

**Goal:** Second accounting standard driver. Org-level standard selection.

**Scope:**

IFRS driver:
- Different chart of accounts (simpler equity section)
- IFRS 15 revenue recognition (stricter variable consideration constraints)
- IFRS 9 expected credit loss model for receivable impairment
- Different presentation of restricted vs unrestricted net assets

Per-org selection:
- `OrgAccountingConfig` stores standard + config (already built in Phase 1)
- UI: select standard -> standard-specific configuration options
- Migration path: changing standards requires year-end close under old standard first

### Phase Dependencies

```
Phase 1: Foundation (interface, GAAP recording, chart, periods, config)
    |
Phase 2: Payment behavior + AP (strategy, terms, recognition, expenses)
    |
Phase 3: Revenue lifecycle + closing (deferrals, year-end, void/reversal)
    |
Phase 4: IFRS driver + multi-standard selection
```

Each phase is independently shippable.

## File Layout

```
backend/internal/fin/
  engine.go                     # AccountingEngine interface + all types
  engine_config.go              # EngineConfig, RecognitionBasis, EngineFactory
  engine_gaap.go                # GaapEngine implementation
  engine_gaap_test.go           # GAAP driver tests
  engine_ifrs.go                # IFRS driver (Phase 4)
  engine_ifrs_test.go
  engine_effects.go             # FinancialEffects, directive types
  engine_types.go               # FinancialTransaction, context types, result types
  allocation.go                 # Payment allocation (existing, strategy-driven)
  period.go                     # AccountingPeriod domain type
  period_repository.go          # AccountingPeriodRepository interface
  period_postgres.go            # Postgres implementation
  org_accounting_config.go      # OrgAccountingConfig domain type + repository
  org_accounting_config_postgres.go
```

## Related Issues

- #60 — P0: Financial operations not atomic (Phase 1: UoW + executeEffects)
- #66 — P1: GL failure silently swallowed (Phase 1: centralized error handling)
- #69 — P1: Expense payment missing GL entries (Phase 1/2: engine produces all effects)
- #70 — P1: Assessment GL always books to operating revenue (Phase 1: split-fund support)
- #75 — P2: Void/reversal for payments and journal entries (Phase 3)
- #83 — P2: GL-to-ledger convergence (Phase 1: FinancialEffects bundle)

## Accounting Specialist Review Notes

The following items were identified by a four-person review team (CPA GAAP specialist, CPA HOA specialist, financial BA, regulatory compliance specialist) and incorporated or tracked:

### Incorporated into spec (this revision)

- Year-end close expanded to include interfund transfer accounts (3100/3110)
- Interfund receivable/payable accounts added (1300 Due From / 2500 Due To) for CIRA fund-columnar support
- Superlien priority metadata (`LienPriority` on `ReceivableItem`) for NV/CO/CT/DC/DE
- Trust/escrow custodian tracking (`CustodianType`) for FL/CA/NV management company requirements
- Late fee and interest cap enforcement in `ValidateTransaction` via policy registry
- Bad debt recovery transaction type (`TxTypeBadDebtRecovery`)
- Modified accrual entry patterns specified per transaction type
- Bank reconciliation promoted to Phase 2 (minimum viable: reconciled flag + BankTransaction entity)
- Additional state payment application statutes (IL, VA, MD, WA)
- Chart expanded from 40 to 53 accounts: fixed assets, depreciation, insurance claims/proceeds/deductible, payroll, income tax payable, detailed revenue breakout
- External reference fields on `FinancialTransaction` for integration (check, ACH, gateway, lockbox)
- FDCPA `CollectionSource` distinction on transactions
- Interfund borrowing transaction type (`TxTypeInterfundLoan`) distinct from permanent transfers
- Period 13 adjusting-entry period for year-end audit

### Tracked for future consideration beyond Phase 4

- **1099 reporting:** Vendor payment aggregation per tax year for $600+ threshold. Reporting concern — data model (vendor TIN) needs to support it.
- **Reserve study integration:** Data model for reserve study components (useful life, replacement cost, current allocation). Drives reserve fund target and contribution calculations. Required for CA (Civil Code 5550), FL (FS 720.303(6)) statutory disclosures.
- **Multi-currency:** Domain types carry `CurrencyCode` (hardcoded USD). Engine interface should accept currency for validation. Full multi-currency (ASC 830 / IAS 21) is future scope.
- **Tax basis accounting:** IRC Section 528 election for HOAs. Revenue classification by taxability. Lower priority but enum slot reserved.
- **Batch operation model:** Billing runs, lockbox imports, ACH batch processing need a batch envelope (batch ID, source reference, reconciliation totals). Service-layer concern — engine processes individual transactions.
- **Budget-vs-actual integration:** Engine has no budget awareness. Budget-to-assessment calculation and budget-vs-actual reporting are service/reporting layer concerns. Boundary should be explicitly documented.
- **Retainage accounting:** Construction/capital project retainage for reserve-funded major repairs. Requires additional GL accounts and AP workflow.
- **Aging bucket model:** 30/60/90/120+ day delinquency aging for board reporting. Derivable from ledger data but needs an aggregation layer.
- **Assessment proration:** Mid-year unit sales requiring pro-rated assessment calculation. Service-layer concern.
- **ASC 606 variable consideration:** Special assessments with contingencies or tiered structures may introduce variable consideration constraints (ASC 606-10-32-8). Deferred to metadata-driven approach.
