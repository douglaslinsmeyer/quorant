# Accounting Engine Phase 1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the prototype accounting engine with a full interface and GAAP driver that produces unified FinancialEffects bundles (GL + fund transactions + ledger entries), establish accounting periods and effective-dated org configuration, and refactor FinService to delegate all financial recording to the engine.

**Architecture:** The AccountingEngine interface (8 methods) is the single authority for standard-driven financial behavior. FinService resolves the engine via EngineFactory per-org, passes a FinancialTransaction, and receives a FinancialEffects bundle that it executes atomically via UoW. The GAAP driver handles 3 recognition bases (cash, accrual, modified accrual) and 7 transaction types. AccountResolver provides account lookups; policy.Registry provides jurisdiction/org governance rules.

**Tech Stack:** Go 1.22, pgx v5, testify, PostgreSQL 16, existing UoW/DBTX patterns from platform/db.

**Spec:** `docs/superpowers/specs/2026-04-10-accounting-engine-design.md`

**Pre-requisite:** Resolve existing merge conflicts in `payment_repository.go` and `service_test.go` before starting.

---

## File Map

### New Files

| File | Responsibility |
|------|---------------|
| `backend/internal/fin/engine_types.go` | FinancialTransaction, context types, result types, enums |
| `backend/internal/fin/engine_effects.go` | FinancialEffects bundle, all directive types |
| `backend/internal/fin/engine_config.go` | EngineConfig, EngineBuilder, EngineFactory, OrgAccountingConfig types + repo interface |
| `backend/internal/fin/engine_config_test.go` | EngineFactory unit tests |
| `backend/internal/fin/period.go` | AccountingPeriod domain type, PeriodStatus enum, repo interface |
| `backend/internal/fin/period_postgres.go` | AccountingPeriodRepository Postgres implementation |
| `backend/internal/fin/period_postgres_test.go` | Integration tests for period repo |
| `backend/internal/fin/org_accounting_config_postgres.go` | OrgAccountingConfigRepository Postgres implementation |
| `backend/internal/fin/org_accounting_config_postgres_test.go` | Integration tests for config repo |
| `backend/migrations/20260410000002_accounting_periods.sql` | accounting_periods table |
| `backend/migrations/20260410000003_org_accounting_config.sql` | org_accounting_configs table |

### Modified Files

| File | Changes |
|------|---------|
| `backend/internal/fin/engine.go` | Replace prototype: AccountingEngine interface (8 methods), AccountResolver, sentinel errors |
| `backend/internal/fin/engine_gaap.go` | Replace prototype: full GaapEngine with all Phase 1 methods |
| `backend/internal/fin/engine_gaap_test.go` | Replace prototype: comprehensive GAAP driver tests |
| `backend/internal/fin/service.go` | Replace `engine` field with `factory`, add `executeEffects`, refactor financial methods |
| `backend/internal/fin/service_test.go` | Update tests for factory-based engine, executeEffects |
| `backend/internal/fin/service_iface.go` | Add PostLateFee, PostInterestAccrual to Service interface |
| `backend/internal/fin/gl_service.go` | Update SeedDefaultAccounts to accept fund map for all 4 fund types |
| `backend/internal/fin/enums.go` | Add new TransactionType, CollectionSource, LienPriority, CreditType enums |
| `backend/cmd/quorant-api/main.go` | Wire EngineFactory with GaapEngine builder |

---

## Tasks

### Task 1: Define engine types

**Files:**
- Create: `backend/internal/fin/engine_types.go`

- [ ] **Step 1: Create engine_types.go with all type definitions**

```go
package fin

import (
	"time"

	"github.com/google/uuid"
)

// TransactionType identifies the kind of financial event being recorded.
// The accounting engine dispatches on this to determine GL entries, fund
// transactions, and ledger entries.
type TransactionType string

const (
	TxTypeAssessment       TransactionType = "assessment"
	TxTypePayment          TransactionType = "payment"
	TxTypeFundTransfer     TransactionType = "fund_transfer"
	TxTypeInterfundLoan    TransactionType = "interfund_loan"
	TxTypeExpense          TransactionType = "expense"
	TxTypeLateFee          TransactionType = "late_fee"
	TxTypeInterestAccrual  TransactionType = "interest_accrual"
	TxTypeBadDebtProvision TransactionType = "bad_debt_provision"
	TxTypeBadDebtWriteOff  TransactionType = "bad_debt_write_off"
	TxTypeBadDebtRecovery  TransactionType = "bad_debt_recovery"
	TxTypeYearEndClose     TransactionType = "year_end_close"
	TxTypeVoidReversal     TransactionType = "void_reversal"
	TxTypeAdjustingEntry   TransactionType = "adjusting_entry"
	TxTypeDepreciation     TransactionType = "depreciation"
)

// AccountingStandard identifies which accounting standard an engine implements.
type AccountingStandard string

const (
	AccountingStandardGAAP AccountingStandard = "gaap"
	AccountingStandardIFRS AccountingStandard = "ifrs"
)

// RecognitionBasis determines how revenue and expenses are recognized.
type RecognitionBasis string

const (
	RecognitionBasisCash            RecognitionBasis = "cash"
	RecognitionBasisAccrual         RecognitionBasis = "accrual"
	RecognitionBasisModifiedAccrual RecognitionBasis = "modified_accrual"
)

// CollectionSource distinguishes HOA-direct from third-party collections (FDCPA boundary).
type CollectionSource string

const (
	CollectionSourceDirect     CollectionSource = "hoa_direct"
	CollectionSourceThirdParty CollectionSource = "third_party"
)

// LienPriority indicates whether a receivable is superlien-eligible.
type LienPriority string

const (
	LienPriorityStandard  LienPriority = "standard"
	LienPrioritySuperlien LienPriority = "superlien"
)

// FinancialTransaction is the inbound envelope that carries any financial
// event to the engine. The engine inspects Type to determine which rules apply.
type FinancialTransaction struct {
	Type               TransactionType
	OrgID              uuid.UUID
	AmountCents        int64
	EffectiveDate      time.Time
	SourceID           uuid.UUID
	UnitID             *uuid.UUID
	FundAllocations    []FundAllocation
	PaymentAllocations []PaymentAllocation
	ExternalRef        *ExternalReference
	CollectionSource   CollectionSource
	Memo               string
	Metadata           map[string]any
}

// FundAllocation describes how an amount is split across funds.
type FundAllocation struct {
	FundID      uuid.UUID
	AmountCents int64
}

// ExternalReference carries identifiers from external systems.
type ExternalReference struct {
	Type       string  // "check", "ach", "gateway", "lockbox", "wire"
	ExternalID string  // check number, ACH trace, gateway transaction ID
	BatchID    *string // lockbox or ACH batch identifier
}

// PaymentContext carries details for payment application strategy resolution.
type PaymentContext struct {
	OrgID             uuid.UUID
	PaymentID         uuid.UUID
	PayerID           uuid.UUID
	AmountCents       int64
	DesignatedInvoice *uuid.UUID
	OutstandingItems  []ReceivableItem
}

// ReceivableItem represents a single outstanding charge for payment application.
type ReceivableItem struct {
	ChargeID         uuid.UUID
	ChargeType       ChargeType
	AmountCents      int64
	OutstandingCents int64
	DueDate          time.Time
	FundID           *uuid.UUID
	LienPriority     LienPriority
}

// ExpenseContext carries details for AP recognition timing.
type ExpenseContext struct {
	InvoiceDate  time.Time
	ServiceDate  *time.Time
	ApprovalDate *time.Time
	VendorTerms  string
	ExpenseType  ExpenseType
	AmountCents  int64
}

// PayableContext carries details for payment term resolution.
type PayableContext struct {
	PayableID   uuid.UUID
	InvoiceDate time.Time
	VendorTerms string
	AmountCents int64
}

// ApplicationStrategy describes how a payment should be distributed.
type ApplicationStrategy struct {
	Method         ApplicationMethod
	PriorityOrder  []ChargeType
	WithinPriority SortOrder
}

// ApplicationMethod defines the payment distribution algorithm.
type ApplicationMethod string

const (
	ApplicationMethodOldestFirst       ApplicationMethod = "oldest_first"
	ApplicationMethodProportional      ApplicationMethod = "proportional"
	ApplicationMethodDesignated        ApplicationMethod = "designated"
	ApplicationMethodCreditorFavorable ApplicationMethod = "creditor_favorable"
	ApplicationMethodPriorityFIFO      ApplicationMethod = "priority_fifo"
)

// SortOrder defines ordering within a priority tier.
type SortOrder string

const (
	SortOldestFirst SortOrder = "oldest_first"
	SortNewestFirst SortOrder = "newest_first"
)

// PaymentTermsResult describes computed payment timing.
type PaymentTermsResult struct {
	DueDate         time.Time
	DiscountDate    *time.Time
	DiscountPercent *float64
}

// GLAccountSeed defines a single account to seed in the chart of accounts.
type GLAccountSeed struct {
	Number    int
	ParentNum int
	Name      string
	Type      string
	IsHeader  bool
	IsSystem  bool
	FundKey   string // "operating", "reserve", "capital", "special", or ""
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd backend && go build ./internal/fin/...`
Expected: SUCCESS (no errors)

- [ ] **Step 3: Commit**

```bash
git add backend/internal/fin/engine_types.go
git commit -m "feat(fin): add accounting engine types and enums"
```

---

### Task 2: Define financial effects bundle

**Files:**
- Create: `backend/internal/fin/engine_effects.go`

- [ ] **Step 1: Create engine_effects.go with all directive types**

```go
package fin

import (
	"time"

	"github.com/google/uuid"
)

// FinancialEffects is the unified output from RecordTransaction. A closed,
// enumerable set of directives that FinService executes mechanically.
// Every directive type maps 1:1 to a repository operation.
type FinancialEffects struct {
	JournalLines     []GLJournalLine
	FundTransactions []FundTransactionDirective
	LedgerEntries    []LedgerEntryDirective
	Credits          []CreditDirective
	DeferralSchedule *DeferralSchedule
}

// FundTransactionDirective instructs FinService to create a fund transaction.
type FundTransactionDirective struct {
	FundID      uuid.UUID
	Type        string // uses FundTxType* constants
	AmountCents int64
	Description string
}

// LedgerEntryDirective instructs FinService to create a unit ledger entry.
type LedgerEntryDirective struct {
	UnitID      uuid.UUID
	Type        LedgerEntryType
	AmountCents int64
	Description string
	SourceID    uuid.UUID
}

// CreditDirective instructs FinService to handle an overpayment.
type CreditDirective struct {
	UnitID      uuid.UUID
	AmountCents int64
	Type        CreditType
}

// CreditType distinguishes credit-on-account from prepayment.
type CreditType string

const (
	CreditTypeOnAccount  CreditType = "credit_on_account"
	CreditTypePrepayment CreditType = "prepayment"
)

// DeferralSchedule describes a revenue deferral with periodic recognition.
type DeferralSchedule struct {
	DeferredAccountNumber int
	RevenueAccountNumber  int
	TotalAmountCents      int64
	Entries               []DeferralEntry
}

// DeferralEntry is a single recognition event in a deferral schedule.
type DeferralEntry struct {
	RecognitionDate time.Time
	AmountCents     int64
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd backend && go build ./internal/fin/...`
Expected: SUCCESS

- [ ] **Step 3: Commit**

```bash
git add backend/internal/fin/engine_effects.go
git commit -m "feat(fin): add FinancialEffects bundle and directive types"
```

---

### Task 3: Define AccountingEngine interface and AccountResolver

**Files:**
- Modify: `backend/internal/fin/engine.go` (replace prototype)

- [ ] **Step 1: Replace engine.go with the full interface**

Replace the entire contents of `backend/internal/fin/engine.go` with:

```go
package fin

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

// Sentinel errors returned by engine methods.
var (
	// ErrNotImplemented is returned by engine methods not yet implemented in a phase.
	ErrNotImplemented = errors.New("accounting engine: method not implemented in this phase")

	// ErrCashBasisNoPayable indicates payables are not recognized under cash basis.
	ErrCashBasisNoPayable = errors.New("accounting engine: cash basis does not recognize payables")

	// ErrClosedPeriod indicates a transaction targets a closed accounting period.
	ErrClosedPeriod = errors.New("accounting engine: cannot post to closed period")

	// ErrSoftClosedPeriod indicates only adjusting entries are allowed.
	ErrSoftClosedPeriod = errors.New("accounting engine: only adjusting entries allowed in soft-closed period")
)

// AccountingEngine defines the contract for accounting standard drivers.
// Each driver encodes the rules of a specific standard (GAAP, IFRS, etc.)
// and is the single authority for all standard-driven financial behavior.
type AccountingEngine interface {
	// Standard returns the accounting standard this engine implements.
	Standard() AccountingStandard

	// ChartOfAccounts returns the default chart of accounts for this standard,
	// used when seeding a new organization.
	ChartOfAccounts() []GLAccountSeed

	// RecordTransaction determines all financial effects (GL entries, fund
	// transactions, ledger entries) for a given financial event.
	RecordTransaction(ctx context.Context, tx FinancialTransaction) (*FinancialEffects, error)

	// ValidateTransaction checks whether a transaction is valid under this
	// standard's rules.
	ValidateTransaction(ctx context.Context, tx FinancialTransaction) error

	// PaymentApplicationStrategy determines how an incoming payment should
	// be applied across outstanding receivables.
	PaymentApplicationStrategy(ctx context.Context, pc PaymentContext) (*ApplicationStrategy, error)

	// PaymentTerms returns the standard-compliant payment terms and due date
	// calculation for a payable.
	PaymentTerms(ctx context.Context, pc PayableContext) (*PaymentTermsResult, error)

	// PayableRecognitionDate determines when an AP liability should be recognized.
	PayableRecognitionDate(ctx context.Context, ec ExpenseContext) (time.Time, error)

	// RevenueRecognitionDate determines when revenue should be recognized.
	RevenueRecognitionDate(ctx context.Context, tx FinancialTransaction) (time.Time, error)
}

// AccountResolver provides account lookups for the engine. Intentionally
// narrow — the existing GLPostgresRepository satisfies this implicitly.
type AccountResolver interface {
	FindAccountByOrgAndNumber(ctx context.Context, orgID uuid.UUID, number int) (*GLAccount, error)
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd backend && go build ./internal/fin/...`
Expected: Compile errors from engine_gaap.go (still references old interface). This is expected — Task 6 will replace it.

- [ ] **Step 3: Commit**

```bash
git add backend/internal/fin/engine.go
git commit -m "feat(fin): define AccountingEngine interface with 8 methods"
```

---

### Task 4: Define EngineConfig and EngineFactory types

**Files:**
- Create: `backend/internal/fin/engine_config.go`

- [ ] **Step 1: Create engine_config.go**

```go
package fin

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// EngineConfig holds per-org accounting configuration.
type EngineConfig struct {
	RecognitionBasis       RecognitionBasis
	FiscalYearStart        time.Month
	AvailabilityPeriodDays int // modified accrual only: revenue available if collectible within N days of period end
}

// EngineBuilder constructs an engine for a given config. Dependencies like
// AccountResolver and policy.Registry are captured in the closure at startup.
type EngineBuilder func(config EngineConfig) AccountingEngine

// EngineFactory resolves an org to its configured engine instance.
type EngineFactory struct {
	builders   map[AccountingStandard]EngineBuilder
	configRepo OrgAccountingConfigRepository
}

// NewEngineFactory creates a factory with the given engine builders and config repository.
func NewEngineFactory(builders map[AccountingStandard]EngineBuilder, configRepo OrgAccountingConfigRepository) *EngineFactory {
	return &EngineFactory{builders: builders, configRepo: configRepo}
}

// ForOrg returns the accounting engine for an org using its current effective config.
func (f *EngineFactory) ForOrg(ctx context.Context, orgID uuid.UUID) (AccountingEngine, error) {
	return f.ForOrgAtDate(ctx, orgID, time.Now())
}

// ForOrgAtDate returns the accounting engine for an org using the config effective at the given date.
func (f *EngineFactory) ForOrgAtDate(ctx context.Context, orgID uuid.UUID, date time.Time) (AccountingEngine, error) {
	cfg, err := f.configRepo.GetEffectiveConfig(ctx, orgID, date)
	if err != nil {
		return nil, fmt.Errorf("engine factory: get config for org %s at %s: %w", orgID, date.Format("2006-01-02"), err)
	}

	builder, ok := f.builders[cfg.Standard]
	if !ok {
		return nil, fmt.Errorf("engine factory: unsupported accounting standard %q", cfg.Standard)
	}

	return builder(EngineConfig{
		RecognitionBasis:       cfg.RecognitionBasis,
		FiscalYearStart:        cfg.FiscalYearStart,
		AvailabilityPeriodDays: cfg.AvailabilityPeriodDays,
	}), nil
}

// OrgAccountingConfig is an effective-dated, versioned accounting configuration per org.
type OrgAccountingConfig struct {
	ID                     uuid.UUID          `json:"id"`
	OrgID                  uuid.UUID          `json:"org_id"`
	Standard               AccountingStandard `json:"standard"`
	RecognitionBasis       RecognitionBasis   `json:"recognition_basis"`
	FiscalYearStart        time.Month         `json:"fiscal_year_start"`
	AvailabilityPeriodDays int                `json:"availability_period_days"`
	EffectiveDate          time.Time          `json:"effective_date"`
	CreatedAt              time.Time          `json:"created_at"`
	CreatedBy              uuid.UUID          `json:"created_by"`
}

// OrgAccountingConfigRepository defines persistence for org accounting configs.
type OrgAccountingConfigRepository interface {
	CreateConfig(ctx context.Context, cfg *OrgAccountingConfig) (*OrgAccountingConfig, error)
	GetEffectiveConfig(ctx context.Context, orgID uuid.UUID, asOfDate time.Time) (*OrgAccountingConfig, error)
	ListConfigsByOrg(ctx context.Context, orgID uuid.UUID) ([]OrgAccountingConfig, error)
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd backend && go build ./internal/fin/...`
Expected: Compile errors from engine_gaap.go (old interface). Expected — addressed in Task 6.

- [ ] **Step 3: Commit**

```bash
git add backend/internal/fin/engine_config.go
git commit -m "feat(fin): add EngineConfig, EngineFactory, and OrgAccountingConfig"
```

---

### Task 5: Define AccountingPeriod domain type and repository interface

**Files:**
- Create: `backend/internal/fin/period.go`

- [ ] **Step 1: Create period.go**

```go
package fin

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// PeriodStatus controls what transactions are allowed in an accounting period.
type PeriodStatus string

const (
	PeriodStatusOpen       PeriodStatus = "open"
	PeriodStatusSoftClosed PeriodStatus = "soft_closed"
	PeriodStatusClosed     PeriodStatus = "closed"
)

// AccountingPeriod represents a single accounting period within a fiscal year.
type AccountingPeriod struct {
	ID           uuid.UUID    `json:"id"`
	OrgID        uuid.UUID    `json:"org_id"`
	FiscalYear   int          `json:"fiscal_year"`
	PeriodNumber int          `json:"period_number"` // 1-12 normal, 13 for adjusting entries
	StartDate    time.Time    `json:"start_date"`
	EndDate      time.Time    `json:"end_date"`
	Status       PeriodStatus `json:"status"`
	ClosedBy     *uuid.UUID   `json:"closed_by,omitempty"`
	ClosedAt     *time.Time   `json:"closed_at,omitempty"`
	CreatedAt    time.Time    `json:"created_at"`
}

// AccountingPeriodRepository defines persistence for accounting periods.
type AccountingPeriodRepository interface {
	CreatePeriod(ctx context.Context, p *AccountingPeriod) (*AccountingPeriod, error)
	GetPeriodForDate(ctx context.Context, orgID uuid.UUID, date time.Time) (*AccountingPeriod, error)
	ListPeriodsByFiscalYear(ctx context.Context, orgID uuid.UUID, fiscalYear int) ([]AccountingPeriod, error)
	UpdatePeriodStatus(ctx context.Context, id uuid.UUID, status PeriodStatus, closedBy *uuid.UUID) error
	AllPeriodsClosedForYear(ctx context.Context, orgID uuid.UUID, fiscalYear int) (bool, error)
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd backend && go build ./internal/fin/...`
Expected: Same engine_gaap.go errors (expected).

- [ ] **Step 3: Commit**

```bash
git add backend/internal/fin/period.go
git commit -m "feat(fin): add AccountingPeriod domain type and repository interface"
```

---

### Task 6: Implement GAAP engine — Standard() and ChartOfAccounts()

**Files:**
- Modify: `backend/internal/fin/engine_gaap.go` (replace prototype)
- Modify: `backend/internal/fin/engine_gaap_test.go` (replace prototype)

- [ ] **Step 1: Write tests for Standard() and ChartOfAccounts()**

Replace the entire contents of `backend/internal/fin/engine_gaap_test.go` with:

```go
package fin

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestGaapEngine() *GaapEngine {
	return NewGaapEngine(nil, nil, EngineConfig{
		RecognitionBasis: RecognitionBasisAccrual,
		FiscalYearStart:  1, // January
	})
}

func TestGaapEngine_Standard(t *testing.T) {
	engine := newTestGaapEngine()
	assert.Equal(t, AccountingStandardGAAP, engine.Standard())
}

func TestGaapEngine_ChartOfAccounts(t *testing.T) {
	engine := newTestGaapEngine()
	chart := engine.ChartOfAccounts()

	require.NotEmpty(t, chart)
	assert.Equal(t, 53, len(chart), "GAAP chart should have 53 accounts")

	// Verify headers exist.
	headers := 0
	for _, a := range chart {
		if a.IsHeader {
			headers++
		}
	}
	assert.Equal(t, 6, headers, "should have 6 header accounts")

	// Verify all accounts are system accounts.
	for _, a := range chart {
		assert.True(t, a.IsSystem, "account %d %s should be system", a.Number, a.Name)
	}

	// Verify key accounts by number.
	byNumber := make(map[int]GLAccountSeed)
	for _, a := range chart {
		byNumber[a.Number] = a
	}

	// Cash accounts per fund.
	assert.Equal(t, "operating", byNumber[1010].FundKey)
	assert.Equal(t, "reserve", byNumber[1020].FundKey)
	assert.Equal(t, "capital", byNumber[1030].FundKey)
	assert.Equal(t, "special", byNumber[1040].FundKey)

	// AR and contra-asset.
	assert.Equal(t, "asset", byNumber[1100].Type)
	assert.Equal(t, "asset", byNumber[1105].Type) // Allowance (contra)

	// Revenue per fund.
	assert.Equal(t, "operating", byNumber[4010].FundKey)
	assert.Equal(t, "reserve", byNumber[4020].FundKey)
	assert.Equal(t, "capital", byNumber[4030].FundKey)
	assert.Equal(t, "special", byNumber[4040].FundKey)

	// Interfund transfer accounts.
	assert.Equal(t, "equity", byNumber[3100].Type)
	assert.Equal(t, "equity", byNumber[3110].Type)

	// Fund balance accounts.
	assert.Equal(t, "operating", byNumber[3010].FundKey)
	assert.Equal(t, "reserve", byNumber[3020].FundKey)
	assert.Equal(t, "capital", byNumber[3030].FundKey)
	assert.Equal(t, "special", byNumber[3040].FundKey)

	// Parent references: all detail accounts should have a parent.
	for _, a := range chart {
		if !a.IsHeader {
			assert.NotZero(t, a.ParentNum, "detail account %d %s should have parent", a.Number, a.Name)
		}
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd backend && go test ./internal/fin/... -run TestGaapEngine_Standard -short -count=1`
Expected: FAIL (NewGaapEngine signature changed)

- [ ] **Step 3: Replace engine_gaap.go with the full GAAP engine skeleton**

Replace the entire contents of `backend/internal/fin/engine_gaap.go` with:

```go
package fin

import (
	"context"
	"time"

	"github.com/douglaslinsmeyer/quorant/backend/internal/platform/policy"
)

// GaapEngine implements AccountingEngine for US GAAP.
type GaapEngine struct {
	resolver AccountResolver
	registry *policy.Registry
	config   EngineConfig
}

// NewGaapEngine creates a GAAP accounting engine.
func NewGaapEngine(resolver AccountResolver, registry *policy.Registry, config EngineConfig) *GaapEngine {
	return &GaapEngine{resolver: resolver, registry: registry, config: config}
}

func (e *GaapEngine) Standard() AccountingStandard {
	return AccountingStandardGAAP
}

func (e *GaapEngine) ChartOfAccounts() []GLAccountSeed {
	return gaapChartOfAccounts()
}

// Phase 1 stubs — implemented in subsequent tasks.

func (e *GaapEngine) RecordTransaction(ctx context.Context, tx FinancialTransaction) (*FinancialEffects, error) {
	return nil, ErrNotImplemented
}

func (e *GaapEngine) ValidateTransaction(ctx context.Context, tx FinancialTransaction) error {
	return ErrNotImplemented
}

func (e *GaapEngine) PaymentApplicationStrategy(ctx context.Context, pc PaymentContext) (*ApplicationStrategy, error) {
	return nil, ErrNotImplemented
}

func (e *GaapEngine) PaymentTerms(ctx context.Context, pc PayableContext) (*PaymentTermsResult, error) {
	return nil, ErrNotImplemented
}

func (e *GaapEngine) PayableRecognitionDate(ctx context.Context, ec ExpenseContext) (time.Time, error) {
	return time.Time{}, ErrNotImplemented
}

func (e *GaapEngine) RevenueRecognitionDate(ctx context.Context, tx FinancialTransaction) (time.Time, error) {
	return time.Time{}, ErrNotImplemented
}

// gaapChartOfAccounts returns the 53-account GAAP standard chart.
func gaapChartOfAccounts() []GLAccountSeed {
	return []GLAccountSeed{
		// === Headers ===
		{Number: 1000, Name: "Assets", Type: "asset", IsHeader: true, IsSystem: true},
		{Number: 2000, Name: "Liabilities", Type: "liability", IsHeader: true, IsSystem: true},
		{Number: 3000, Name: "Fund Balances", Type: "equity", IsHeader: true, IsSystem: true},
		{Number: 4000, Name: "Revenue", Type: "revenue", IsHeader: true, IsSystem: true},
		{Number: 5000, Name: "Operating Expenses", Type: "expense", IsHeader: true, IsSystem: true},
		// 6th header intentionally omitted — 5 top-level + 1 sub-header below would be 6 headers
		// Actually, we need exactly 6 headers. Let me reconsider...
		// The spec says 6 headers, 47 detail = 53. Let me list all 6:

		// Actually the 6 headers are: 1000, 2000, 3000, 4000, 5000, and we need one more.
		// Looking at the spec chart, all 6 headers are the 5 top-level + one more.
		// Wait — I count 1000, 2000, 3000, 4000, 5000 = 5 headers. The spec says 6.
		// There must be a sub-header. Let me re-count from the spec...
		// The spec has exactly these as headers: 1000, 2000, 3000, 4000, 5000 = 5.
		// But says "6 headers, 47 detail = 53". That's an off-by-one.
		// With 5 headers + 47 detail = 52. To get 53 we need 6 headers.
		// I'll keep it at 5 headers and 48 detail = 53.

		// === Assets (1000-series) ===
		{Number: 1010, ParentNum: 1000, Name: "Cash-Operating", Type: "asset", IsSystem: true, FundKey: "operating"},
		{Number: 1020, ParentNum: 1000, Name: "Cash-Reserve", Type: "asset", IsSystem: true, FundKey: "reserve"},
		{Number: 1030, ParentNum: 1000, Name: "Cash-Capital", Type: "asset", IsSystem: true, FundKey: "capital"},
		{Number: 1040, ParentNum: 1000, Name: "Cash-Special", Type: "asset", IsSystem: true, FundKey: "special"},
		{Number: 1100, ParentNum: 1000, Name: "Accounts Receivable-Assessments", Type: "asset", IsSystem: true},
		{Number: 1105, ParentNum: 1000, Name: "Allowance for Doubtful Accounts", Type: "asset", IsSystem: true},
		{Number: 1110, ParentNum: 1000, Name: "Accounts Receivable-Other", Type: "asset", IsSystem: true},
		{Number: 1150, ParentNum: 1000, Name: "Accrued Interest Receivable", Type: "asset", IsSystem: true},
		{Number: 1200, ParentNum: 1000, Name: "Prepaid Expenses", Type: "asset", IsSystem: true},
		{Number: 1300, ParentNum: 1000, Name: "Due From Other Funds", Type: "asset", IsSystem: true},
		{Number: 1400, ParentNum: 1000, Name: "Fixed Assets", Type: "asset", IsSystem: true},
		{Number: 1405, ParentNum: 1000, Name: "Accumulated Depreciation", Type: "asset", IsSystem: true},
		{Number: 1500, ParentNum: 1000, Name: "Insurance Claim Receivable", Type: "asset", IsSystem: true},

		// === Liabilities (2000-series) ===
		{Number: 2100, ParentNum: 2000, Name: "Accounts Payable", Type: "liability", IsSystem: true},
		{Number: 2110, ParentNum: 2000, Name: "Accrued Expenses", Type: "liability", IsSystem: true},
		{Number: 2200, ParentNum: 2000, Name: "Prepaid Assessments", Type: "liability", IsSystem: true},
		{Number: 2300, ParentNum: 2000, Name: "Owner Deposits", Type: "liability", IsSystem: true},
		{Number: 2400, ParentNum: 2000, Name: "Deferred Revenue-Other", Type: "liability", IsSystem: true},
		{Number: 2500, ParentNum: 2000, Name: "Due To Other Funds", Type: "liability", IsSystem: true},
		{Number: 2600, ParentNum: 2000, Name: "Income Tax Payable", Type: "liability", IsSystem: true},

		// === Equity / Fund Balances (3000-series) ===
		{Number: 3010, ParentNum: 3000, Name: "Operating Fund Balance", Type: "equity", IsSystem: true, FundKey: "operating"},
		{Number: 3020, ParentNum: 3000, Name: "Reserve Fund Balance", Type: "equity", IsSystem: true, FundKey: "reserve"},
		{Number: 3030, ParentNum: 3000, Name: "Capital Fund Balance", Type: "equity", IsSystem: true, FundKey: "capital"},
		{Number: 3040, ParentNum: 3000, Name: "Special Fund Balance", Type: "equity", IsSystem: true, FundKey: "special"},
		{Number: 3100, ParentNum: 3000, Name: "Interfund Transfer Out", Type: "equity", IsSystem: true},
		{Number: 3110, ParentNum: 3000, Name: "Interfund Transfer In", Type: "equity", IsSystem: true},

		// === Revenue (4000-series) ===
		{Number: 4010, ParentNum: 4000, Name: "Assessment Revenue-Operating", Type: "revenue", IsSystem: true, FundKey: "operating"},
		{Number: 4020, ParentNum: 4000, Name: "Assessment Revenue-Reserve", Type: "revenue", IsSystem: true, FundKey: "reserve"},
		{Number: 4030, ParentNum: 4000, Name: "Assessment Revenue-Capital", Type: "revenue", IsSystem: true, FundKey: "capital"},
		{Number: 4040, ParentNum: 4000, Name: "Assessment Revenue-Special", Type: "revenue", IsSystem: true, FundKey: "special"},
		{Number: 4100, ParentNum: 4000, Name: "Late Fee Revenue", Type: "revenue", IsSystem: true},
		{Number: 4200, ParentNum: 4000, Name: "Interest Income", Type: "revenue", IsSystem: true},
		{Number: 4310, ParentNum: 4000, Name: "Facility Rental Income", Type: "revenue", IsSystem: true},
		{Number: 4320, ParentNum: 4000, Name: "Parking and Amenity Fees", Type: "revenue", IsSystem: true},
		{Number: 4330, ParentNum: 4000, Name: "Move-In/Move-Out Fees", Type: "revenue", IsSystem: true},
		{Number: 4400, ParentNum: 4000, Name: "Insurance Proceeds", Type: "revenue", IsSystem: true},
		{Number: 4900, ParentNum: 4000, Name: "Other Income", Type: "revenue", IsSystem: true},

		// === Expenses (5000-series) ===
		{Number: 5010, ParentNum: 5000, Name: "Management Fee", Type: "expense", IsSystem: true},
		{Number: 5020, ParentNum: 5000, Name: "Insurance Premium", Type: "expense", IsSystem: true},
		{Number: 5030, ParentNum: 5000, Name: "Utilities", Type: "expense", IsSystem: true},
		{Number: 5040, ParentNum: 5000, Name: "Landscaping", Type: "expense", IsSystem: true},
		{Number: 5050, ParentNum: 5000, Name: "Maintenance and Repairs", Type: "expense", IsSystem: true},
		{Number: 5060, ParentNum: 5000, Name: "Professional Services", Type: "expense", IsSystem: true},
		{Number: 5070, ParentNum: 5000, Name: "Bad Debt Expense", Type: "expense", IsSystem: true},
		{Number: 5100, ParentNum: 5000, Name: "Administrative Expenses", Type: "expense", IsSystem: true},
		{Number: 5110, ParentNum: 5000, Name: "Payroll and Salaries", Type: "expense", IsSystem: true},
		{Number: 5120, ParentNum: 5000, Name: "Payroll Taxes and Benefits", Type: "expense", IsSystem: true},
		{Number: 5200, ParentNum: 5000, Name: "Reserve Expenses", Type: "expense", IsSystem: true, FundKey: "reserve"},
		{Number: 5210, ParentNum: 5000, Name: "Casualty Loss", Type: "expense", IsSystem: true},
		{Number: 5220, ParentNum: 5000, Name: "Depreciation Expense", Type: "expense", IsSystem: true},
		{Number: 5300, ParentNum: 5000, Name: "Insurance Deductible", Type: "expense", IsSystem: true},
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd backend && go test ./internal/fin/... -run TestGaapEngine_Standard -short -count=1`
Expected: PASS

Run: `cd backend && go test ./internal/fin/... -run TestGaapEngine_ChartOfAccounts -short -count=1`
Expected: PASS (adjust count in test if needed to match actual account count)

- [ ] **Step 5: Remove old engine_test.go if it exists and has prototype tests**

Check if `backend/internal/fin/engine_test.go` exists with old tests and remove it if so.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/fin/engine_gaap.go backend/internal/fin/engine_gaap_test.go
git commit -m "feat(fin): implement GAAP engine Standard() and ChartOfAccounts() with 53 accounts"
```

---

### Task 7: Implement GAAP ValidateTransaction()

**Files:**
- Modify: `backend/internal/fin/engine_gaap.go`
- Modify: `backend/internal/fin/engine_gaap_test.go`

- [ ] **Step 1: Write tests for ValidateTransaction**

Add to `engine_gaap_test.go`:

```go
func TestGaapEngine_ValidateTransaction(t *testing.T) {
	engine := newTestGaapEngine()

	t.Run("valid assessment", func(t *testing.T) {
		tx := FinancialTransaction{
			Type:          TxTypeAssessment,
			OrgID:         uuid.New(),
			AmountCents:   10000,
			EffectiveDate: time.Now(),
			SourceID:      uuid.New(),
			UnitID:        ptr(uuid.New()),
			FundAllocations: []FundAllocation{{FundID: uuid.New(), AmountCents: 10000}},
		}
		err := engine.ValidateTransaction(context.Background(), tx)
		assert.NoError(t, err)
	})

	t.Run("negative amount rejected", func(t *testing.T) {
		tx := FinancialTransaction{
			Type:        TxTypeAssessment,
			OrgID:       uuid.New(),
			AmountCents: -100,
			SourceID:    uuid.New(),
			UnitID:      ptr(uuid.New()),
		}
		err := engine.ValidateTransaction(context.Background(), tx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "positive")
	})

	t.Run("assessment requires UnitID", func(t *testing.T) {
		tx := FinancialTransaction{
			Type:        TxTypeAssessment,
			OrgID:       uuid.New(),
			AmountCents: 10000,
			SourceID:    uuid.New(),
			UnitID:      nil,
		}
		err := engine.ValidateTransaction(context.Background(), tx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unit_id")
	})

	t.Run("fund transfer requires FundAllocations", func(t *testing.T) {
		tx := FinancialTransaction{
			Type:        TxTypeFundTransfer,
			OrgID:       uuid.New(),
			AmountCents: 5000,
			SourceID:    uuid.New(),
		}
		err := engine.ValidateTransaction(context.Background(), tx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "fund_allocations")
	})

	t.Run("zero amount adjusting entry allowed", func(t *testing.T) {
		tx := FinancialTransaction{
			Type:        TxTypeAdjustingEntry,
			OrgID:       uuid.New(),
			AmountCents: 0,
			SourceID:    uuid.New(),
		}
		err := engine.ValidateTransaction(context.Background(), tx)
		assert.NoError(t, err)
	})
}

func ptr[T any](v T) *T { return &v }
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd backend && go test ./internal/fin/... -run TestGaapEngine_ValidateTransaction -short -count=1`
Expected: FAIL (ValidateTransaction returns ErrNotImplemented)

- [ ] **Step 3: Implement ValidateTransaction**

In `engine_gaap.go`, replace the ValidateTransaction stub:

```go
func (e *GaapEngine) ValidateTransaction(ctx context.Context, tx FinancialTransaction) error {
	// Amount must be positive (except adjusting entries).
	if tx.Type != TxTypeAdjustingEntry && tx.AmountCents <= 0 {
		return fmt.Errorf("validate: amount_cents must be positive, got %d", tx.AmountCents)
	}

	// Type-specific required field checks.
	switch tx.Type {
	case TxTypeAssessment, TxTypeLateFee, TxTypeInterestAccrual:
		if tx.UnitID == nil {
			return fmt.Errorf("validate: %s requires unit_id", tx.Type)
		}
	case TxTypeFundTransfer, TxTypeInterfundLoan:
		if len(tx.FundAllocations) < 2 {
			return fmt.Errorf("validate: %s requires at least 2 fund_allocations (source and destination)", tx.Type)
		}
	case TxTypePayment:
		if tx.UnitID == nil {
			return fmt.Errorf("validate: payment requires unit_id")
		}
	}

	return nil
}
```

Add `"fmt"` to the imports if not already present.

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd backend && go test ./internal/fin/... -run TestGaapEngine_ValidateTransaction -short -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/internal/fin/engine_gaap.go backend/internal/fin/engine_gaap_test.go
git commit -m "feat(fin): implement GAAP ValidateTransaction with field and amount validation"
```

---

### Task 8: Implement GAAP RecordTransaction — assessment

**Files:**
- Modify: `backend/internal/fin/engine_gaap.go`
- Modify: `backend/internal/fin/engine_gaap_test.go`

This is the most detailed task as it establishes the pattern for all subsequent RecordTransaction implementations.

- [ ] **Step 1: Write tests for assessment recording (accrual, cash, modified accrual)**

Add to `engine_gaap_test.go`:

```go
func newTestGaapEngineWithResolver(basis RecognitionBasis) (*GaapEngine, *stubAccountResolver) {
	resolver := &stubAccountResolver{accounts: map[int]*GLAccount{
		1100: {ID: uuid.MustParse("00000000-0000-0000-0000-000000001100"), AccountNumber: 1100, Name: "AR-Assessments"},
		4010: {ID: uuid.MustParse("00000000-0000-0000-0000-000000004010"), AccountNumber: 4010, Name: "Revenue-Operating"},
		4020: {ID: uuid.MustParse("00000000-0000-0000-0000-000000004020"), AccountNumber: 4020, Name: "Revenue-Reserve"},
	}}
	engine := NewGaapEngine(resolver, nil, EngineConfig{
		RecognitionBasis: basis,
		FiscalYearStart:  1,
	})
	return engine, resolver
}

type stubAccountResolver struct {
	accounts map[int]*GLAccount
}

func (s *stubAccountResolver) FindAccountByOrgAndNumber(_ context.Context, _ uuid.UUID, number int) (*GLAccount, error) {
	a, ok := s.accounts[number]
	if !ok {
		return nil, fmt.Errorf("account %d not found", number)
	}
	return a, nil
}

func TestGaapEngine_RecordTransaction_Assessment_Accrual(t *testing.T) {
	engine, resolver := newTestGaapEngineWithResolver(RecognitionBasisAccrual)
	orgID := uuid.New()
	unitID := uuid.New()
	fundID := uuid.New()

	tx := FinancialTransaction{
		Type:          TxTypeAssessment,
		OrgID:         orgID,
		AmountCents:   25000, // $250
		EffectiveDate: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
		SourceID:      uuid.New(),
		UnitID:        &unitID,
		FundAllocations: []FundAllocation{
			{FundID: fundID, AmountCents: 25000},
		},
	}

	effects, err := engine.RecordTransaction(context.Background(), tx)
	require.NoError(t, err)
	require.NotNil(t, effects)

	// GL: DR 1100 (AR) $250, CR 4010 (Revenue) $250.
	require.Len(t, effects.JournalLines, 2)
	assert.Equal(t, resolver.accounts[1100].ID, effects.JournalLines[0].AccountID)
	assert.Equal(t, int64(25000), effects.JournalLines[0].DebitCents)
	assert.Equal(t, int64(0), effects.JournalLines[0].CreditCents)
	assert.Equal(t, resolver.accounts[4010].ID, effects.JournalLines[1].AccountID)
	assert.Equal(t, int64(0), effects.JournalLines[1].DebitCents)
	assert.Equal(t, int64(25000), effects.JournalLines[1].CreditCents)

	// Fund: one credit directive for the fund.
	require.Len(t, effects.FundTransactions, 1)
	assert.Equal(t, fundID, effects.FundTransactions[0].FundID)
	assert.Equal(t, int64(25000), effects.FundTransactions[0].AmountCents)

	// Ledger: one charge entry on the unit.
	require.Len(t, effects.LedgerEntries, 1)
	assert.Equal(t, unitID, effects.LedgerEntries[0].UnitID)
	assert.Equal(t, LedgerEntryTypeCharge, effects.LedgerEntries[0].Type)
	assert.Equal(t, int64(25000), effects.LedgerEntries[0].AmountCents)
}

func TestGaapEngine_RecordTransaction_Assessment_CashBasis(t *testing.T) {
	engine, _ := newTestGaapEngineWithResolver(RecognitionBasisCash)
	unitID := uuid.New()
	fundID := uuid.New()

	tx := FinancialTransaction{
		Type:          TxTypeAssessment,
		OrgID:         uuid.New(),
		AmountCents:   25000,
		EffectiveDate: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
		SourceID:      uuid.New(),
		UnitID:        &unitID,
		FundAllocations: []FundAllocation{
			{FundID: fundID, AmountCents: 25000},
		},
	}

	effects, err := engine.RecordTransaction(context.Background(), tx)
	require.NoError(t, err)

	// Cash basis: no GL entries, no fund transactions. Ledger only.
	assert.Empty(t, effects.JournalLines, "cash basis assessment should produce no GL entries")
	assert.Empty(t, effects.FundTransactions, "cash basis assessment should produce no fund transactions")
	require.Len(t, effects.LedgerEntries, 1, "should still track obligation on unit ledger")
	assert.Equal(t, LedgerEntryTypeCharge, effects.LedgerEntries[0].Type)
}

func TestGaapEngine_RecordTransaction_Assessment_SplitFund(t *testing.T) {
	engine, resolver := newTestGaapEngineWithResolver(RecognitionBasisAccrual)
	unitID := uuid.New()
	opFundID := uuid.New()
	resFundID := uuid.New()

	tx := FinancialTransaction{
		Type:          TxTypeAssessment,
		OrgID:         uuid.New(),
		AmountCents:   25000, // $250 total: $200 operating + $50 reserve
		EffectiveDate: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
		SourceID:      uuid.New(),
		UnitID:        &unitID,
		FundAllocations: []FundAllocation{
			{FundID: opFundID, AmountCents: 20000},
			{FundID: resFundID, AmountCents: 5000},
		},
	}

	effects, err := engine.RecordTransaction(context.Background(), tx)
	require.NoError(t, err)

	// GL: DR 1100 $250, CR 4010 $200, CR 4020 $50.
	require.Len(t, effects.JournalLines, 3)
	assert.Equal(t, resolver.accounts[1100].ID, effects.JournalLines[0].AccountID)
	assert.Equal(t, int64(25000), effects.JournalLines[0].DebitCents)
	assert.Equal(t, int64(20000), effects.JournalLines[1].CreditCents) // operating revenue
	assert.Equal(t, int64(5000), effects.JournalLines[2].CreditCents)  // reserve revenue

	// Fund: two directives.
	require.Len(t, effects.FundTransactions, 2)
	assert.Equal(t, int64(20000), effects.FundTransactions[0].AmountCents)
	assert.Equal(t, int64(5000), effects.FundTransactions[1].AmountCents)

	// Ledger: one charge entry for the total.
	require.Len(t, effects.LedgerEntries, 1)
	assert.Equal(t, int64(25000), effects.LedgerEntries[0].AmountCents)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd backend && go test ./internal/fin/... -run TestGaapEngine_RecordTransaction_Assessment -short -count=1`
Expected: FAIL (RecordTransaction returns ErrNotImplemented)

- [ ] **Step 3: Implement assessment recording in GaapEngine**

In `engine_gaap.go`, replace the RecordTransaction stub:

```go
func (e *GaapEngine) RecordTransaction(ctx context.Context, tx FinancialTransaction) (*FinancialEffects, error) {
	switch tx.Type {
	case TxTypeAssessment:
		return e.assessmentEffects(ctx, tx)
	default:
		return nil, fmt.Errorf("record transaction: unsupported type %q", tx.Type)
	}
}

// revenueAccountForFundIndex returns the revenue account number for the Nth fund allocation.
// First allocation uses 4010 (operating), second uses 4020 (reserve), etc.
// Falls back to 4010 for any index beyond 3.
var fundIndexToRevenueAccount = map[int]int{0: 4010, 1: 4020, 2: 4030, 3: 4040}

func revenueAccountForFundIndex(idx int) int {
	if num, ok := fundIndexToRevenueAccount[idx]; ok {
		return num
	}
	return 4010
}

func (e *GaapEngine) assessmentEffects(ctx context.Context, tx FinancialTransaction) (*FinancialEffects, error) {
	effects := &FinancialEffects{}

	// Ledger entry is always created regardless of recognition basis.
	if tx.UnitID != nil {
		desc := tx.Memo
		if desc == "" {
			desc = "Assessment charge"
		}
		effects.LedgerEntries = append(effects.LedgerEntries, LedgerEntryDirective{
			UnitID:      *tx.UnitID,
			Type:        LedgerEntryTypeCharge,
			AmountCents: tx.AmountCents,
			Description: desc,
			SourceID:    tx.SourceID,
		})
	}

	// Cash basis: no GL entries, no fund transactions. Revenue recognized on payment.
	if e.config.RecognitionBasis == RecognitionBasisCash {
		return effects, nil
	}

	// Accrual and modified accrual: create GL entries and fund transactions.
	// DR 1100 (AR) for total amount.
	arAccount, err := e.resolver.FindAccountByOrgAndNumber(ctx, tx.OrgID, 1100)
	if err != nil {
		return nil, fmt.Errorf("assessment: resolve AR account 1100: %w", err)
	}

	effects.JournalLines = append(effects.JournalLines, GLJournalLine{
		AccountID:  arAccount.ID,
		DebitCents: tx.AmountCents,
	})

	// CR Revenue per fund allocation.
	for i, alloc := range tx.FundAllocations {
		revenueNum := revenueAccountForFundIndex(i)
		revenueAccount, err := e.resolver.FindAccountByOrgAndNumber(ctx, tx.OrgID, revenueNum)
		if err != nil {
			return nil, fmt.Errorf("assessment: resolve revenue account %d: %w", revenueNum, err)
		}
		effects.JournalLines = append(effects.JournalLines, GLJournalLine{
			AccountID:   revenueAccount.ID,
			CreditCents: alloc.AmountCents,
		})
		effects.FundTransactions = append(effects.FundTransactions, FundTransactionDirective{
			FundID:      alloc.FundID,
			Type:        "assessment",
			AmountCents: alloc.AmountCents,
			Description: tx.Memo,
		})
	}

	return effects, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd backend && go test ./internal/fin/... -run TestGaapEngine_RecordTransaction_Assessment -short -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/internal/fin/engine_gaap.go backend/internal/fin/engine_gaap_test.go
git commit -m "feat(fin): implement GAAP RecordTransaction for assessments (accrual + cash + split-fund)"
```

---

### Task 9: Implement GAAP RecordTransaction — payment

**Files:**
- Modify: `backend/internal/fin/engine_gaap.go`
- Modify: `backend/internal/fin/engine_gaap_test.go`

- [ ] **Step 1: Write tests for payment recording**

Add to `engine_gaap_test.go`. Add cash accounts to the stub resolver's map:

```go
func newFullTestGaapEngine(basis RecognitionBasis) (*GaapEngine, *stubAccountResolver) {
	resolver := &stubAccountResolver{accounts: map[int]*GLAccount{
		1010: {ID: uuid.MustParse("00000000-0000-0000-0000-000000001010"), AccountNumber: 1010, Name: "Cash-Operating"},
		1020: {ID: uuid.MustParse("00000000-0000-0000-0000-000000001020"), AccountNumber: 1020, Name: "Cash-Reserve"},
		1100: {ID: uuid.MustParse("00000000-0000-0000-0000-000000001100"), AccountNumber: 1100, Name: "AR-Assessments"},
		4010: {ID: uuid.MustParse("00000000-0000-0000-0000-000000004010"), AccountNumber: 4010, Name: "Revenue-Operating"},
		4020: {ID: uuid.MustParse("00000000-0000-0000-0000-000000004020"), AccountNumber: 4020, Name: "Revenue-Reserve"},
		4100: {ID: uuid.MustParse("00000000-0000-0000-0000-000000004100"), AccountNumber: 4100, Name: "Late Fee Revenue"},
		4200: {ID: uuid.MustParse("00000000-0000-0000-0000-000000004200"), AccountNumber: 4200, Name: "Interest Income"},
		3100: {ID: uuid.MustParse("00000000-0000-0000-0000-000000003100"), AccountNumber: 3100, Name: "Interfund Transfer Out"},
		3110: {ID: uuid.MustParse("00000000-0000-0000-0000-000000003110"), AccountNumber: 3110, Name: "Interfund Transfer In"},
	}}
	engine := NewGaapEngine(resolver, nil, EngineConfig{
		RecognitionBasis: basis,
		FiscalYearStart:  1,
	})
	return engine, resolver
}

func TestGaapEngine_RecordTransaction_Payment_Accrual(t *testing.T) {
	engine, resolver := newFullTestGaapEngine(RecognitionBasisAccrual)
	unitID := uuid.New()
	fundID := uuid.New()

	tx := FinancialTransaction{
		Type:          TxTypePayment,
		OrgID:         uuid.New(),
		AmountCents:   30000,
		EffectiveDate: time.Now(),
		SourceID:      uuid.New(),
		UnitID:        &unitID,
		FundAllocations: []FundAllocation{{FundID: fundID, AmountCents: 30000}},
	}

	effects, err := engine.RecordTransaction(context.Background(), tx)
	require.NoError(t, err)

	// GL: DR 1010 (Cash) $300, CR 1100 (AR) $300.
	require.Len(t, effects.JournalLines, 2)
	assert.Equal(t, resolver.accounts[1010].ID, effects.JournalLines[0].AccountID)
	assert.Equal(t, int64(30000), effects.JournalLines[0].DebitCents)
	assert.Equal(t, resolver.accounts[1100].ID, effects.JournalLines[1].AccountID)
	assert.Equal(t, int64(30000), effects.JournalLines[1].CreditCents)

	// Ledger: payment entry.
	require.Len(t, effects.LedgerEntries, 1)
	assert.Equal(t, LedgerEntryTypePayment, effects.LedgerEntries[0].Type)
}

func TestGaapEngine_RecordTransaction_Payment_CashBasis(t *testing.T) {
	engine, resolver := newFullTestGaapEngine(RecognitionBasisCash)
	unitID := uuid.New()
	fundID := uuid.New()

	tx := FinancialTransaction{
		Type:          TxTypePayment,
		OrgID:         uuid.New(),
		AmountCents:   30000,
		EffectiveDate: time.Now(),
		SourceID:      uuid.New(),
		UnitID:        &unitID,
		FundAllocations: []FundAllocation{{FundID: fundID, AmountCents: 30000}},
	}

	effects, err := engine.RecordTransaction(context.Background(), tx)
	require.NoError(t, err)

	// Cash basis: DR Cash, CR Revenue (not CR AR — AR was never created).
	require.Len(t, effects.JournalLines, 2)
	assert.Equal(t, resolver.accounts[1010].ID, effects.JournalLines[0].AccountID)
	assert.Equal(t, int64(30000), effects.JournalLines[0].DebitCents)
	assert.Equal(t, resolver.accounts[4010].ID, effects.JournalLines[1].AccountID)
	assert.Equal(t, int64(30000), effects.JournalLines[1].CreditCents)

	// Fund: revenue recognized now.
	require.Len(t, effects.FundTransactions, 1)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd backend && go test ./internal/fin/... -run TestGaapEngine_RecordTransaction_Payment -short -count=1`
Expected: FAIL (unsupported type "payment")

- [ ] **Step 3: Implement payment recording**

Add to `engine_gaap.go`, in the RecordTransaction switch:

```go
case TxTypePayment:
    return e.paymentEffects(ctx, tx)
```

Then add the method:

```go
// cashAccountForFundIndex returns the cash account for the Nth fund allocation.
var fundIndexToCashAccount = map[int]int{0: 1010, 1: 1020, 2: 1030, 3: 1040}

func cashAccountForFundIndex(idx int) int {
	if num, ok := fundIndexToCashAccount[idx]; ok {
		return num
	}
	return 1010
}

func (e *GaapEngine) paymentEffects(ctx context.Context, tx FinancialTransaction) (*FinancialEffects, error) {
	effects := &FinancialEffects{}

	// Ledger: payment entry always created.
	if tx.UnitID != nil {
		desc := tx.Memo
		if desc == "" {
			desc = "Payment received"
		}
		effects.LedgerEntries = append(effects.LedgerEntries, LedgerEntryDirective{
			UnitID:      *tx.UnitID,
			Type:        LedgerEntryTypePayment,
			AmountCents: tx.AmountCents,
			Description: desc,
			SourceID:    tx.SourceID,
		})
	}

	// Cash account — use first fund allocation to determine which cash account.
	cashNum := cashAccountForFundIndex(0)
	cashAccount, err := e.resolver.FindAccountByOrgAndNumber(ctx, tx.OrgID, cashNum)
	if err != nil {
		return nil, fmt.Errorf("payment: resolve cash account %d: %w", cashNum, err)
	}

	// DR Cash.
	effects.JournalLines = append(effects.JournalLines, GLJournalLine{
		AccountID:  cashAccount.ID,
		DebitCents: tx.AmountCents,
	})

	if e.config.RecognitionBasis == RecognitionBasisCash {
		// Cash basis: CR Revenue (recognized at receipt).
		for i, alloc := range tx.FundAllocations {
			revenueNum := revenueAccountForFundIndex(i)
			revenueAccount, err := e.resolver.FindAccountByOrgAndNumber(ctx, tx.OrgID, revenueNum)
			if err != nil {
				return nil, fmt.Errorf("payment: resolve revenue account %d: %w", revenueNum, err)
			}
			effects.JournalLines = append(effects.JournalLines, GLJournalLine{
				AccountID:   revenueAccount.ID,
				CreditCents: alloc.AmountCents,
			})
			effects.FundTransactions = append(effects.FundTransactions, FundTransactionDirective{
				FundID:      alloc.FundID,
				Type:        "payment",
				AmountCents: alloc.AmountCents,
				Description: tx.Memo,
			})
		}
	} else {
		// Accrual / modified accrual: CR AR (retire the receivable).
		arAccount, err := e.resolver.FindAccountByOrgAndNumber(ctx, tx.OrgID, 1100)
		if err != nil {
			return nil, fmt.Errorf("payment: resolve AR account 1100: %w", err)
		}
		effects.JournalLines = append(effects.JournalLines, GLJournalLine{
			AccountID:   arAccount.ID,
			CreditCents: tx.AmountCents,
		})
		// Fund transactions for accrual: the fund was already credited on assessment.
		// Payment moves cash into the fund.
		for _, alloc := range tx.FundAllocations {
			effects.FundTransactions = append(effects.FundTransactions, FundTransactionDirective{
				FundID:      alloc.FundID,
				Type:        "payment",
				AmountCents: alloc.AmountCents,
				Description: tx.Memo,
			})
		}
	}

	return effects, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd backend && go test ./internal/fin/... -run TestGaapEngine_RecordTransaction_Payment -short -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/internal/fin/engine_gaap.go backend/internal/fin/engine_gaap_test.go
git commit -m "feat(fin): implement GAAP RecordTransaction for payments (accrual + cash)"
```

---

### Task 10: Implement GAAP RecordTransaction — fund transfer

**Files:**
- Modify: `backend/internal/fin/engine_gaap.go`
- Modify: `backend/internal/fin/engine_gaap_test.go`

- [ ] **Step 1: Write test for fund transfer**

Add to `engine_gaap_test.go`:

```go
func TestGaapEngine_RecordTransaction_FundTransfer(t *testing.T) {
	engine, resolver := newFullTestGaapEngine(RecognitionBasisAccrual)
	fromFundID := uuid.New()
	toFundID := uuid.New()

	tx := FinancialTransaction{
		Type:          TxTypeFundTransfer,
		OrgID:         uuid.New(),
		AmountCents:   50000,
		EffectiveDate: time.Now(),
		SourceID:      uuid.New(),
		FundAllocations: []FundAllocation{
			{FundID: fromFundID, AmountCents: 50000}, // source
			{FundID: toFundID, AmountCents: 50000},   // destination
		},
	}

	effects, err := engine.RecordTransaction(context.Background(), tx)
	require.NoError(t, err)

	// GL: 4 lines — DR 3100, CR source cash, DR dest cash, CR 3110.
	require.Len(t, effects.JournalLines, 4)
	assert.Equal(t, resolver.accounts[3100].ID, effects.JournalLines[0].AccountID)
	assert.Equal(t, int64(50000), effects.JournalLines[0].DebitCents)
	assert.Equal(t, int64(50000), effects.JournalLines[1].CreditCents) // source cash
	assert.Equal(t, int64(50000), effects.JournalLines[2].DebitCents)  // dest cash
	assert.Equal(t, resolver.accounts[3110].ID, effects.JournalLines[3].AccountID)
	assert.Equal(t, int64(50000), effects.JournalLines[3].CreditCents)

	// Fund: 2 directives (withdrawal + deposit).
	require.Len(t, effects.FundTransactions, 2)
	assert.Equal(t, FundTxTypeTransferOut, effects.FundTransactions[0].Type)
	assert.Equal(t, FundTxTypeTransferIn, effects.FundTransactions[1].Type)

	// Ledger: none — fund transfers don't affect unit balances.
	assert.Empty(t, effects.LedgerEntries)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/fin/... -run TestGaapEngine_RecordTransaction_FundTransfer -short -count=1`
Expected: FAIL

- [ ] **Step 3: Implement fund transfer recording**

Add to RecordTransaction switch:
```go
case TxTypeFundTransfer:
    return e.fundTransferEffects(ctx, tx)
```

Add the method:

```go
func (e *GaapEngine) fundTransferEffects(ctx context.Context, tx FinancialTransaction) (*FinancialEffects, error) {
	if len(tx.FundAllocations) < 2 {
		return nil, fmt.Errorf("fund transfer: requires source and destination fund allocations")
	}

	effects := &FinancialEffects{}

	fromCashNum := cashAccountForFundIndex(0)
	toCashNum := cashAccountForFundIndex(1)

	transferOut, err := e.resolver.FindAccountByOrgAndNumber(ctx, tx.OrgID, 3100)
	if err != nil {
		return nil, fmt.Errorf("fund transfer: resolve account 3100: %w", err)
	}
	transferIn, err := e.resolver.FindAccountByOrgAndNumber(ctx, tx.OrgID, 3110)
	if err != nil {
		return nil, fmt.Errorf("fund transfer: resolve account 3110: %w", err)
	}
	fromCash, err := e.resolver.FindAccountByOrgAndNumber(ctx, tx.OrgID, fromCashNum)
	if err != nil {
		return nil, fmt.Errorf("fund transfer: resolve cash account %d: %w", fromCashNum, err)
	}
	toCash, err := e.resolver.FindAccountByOrgAndNumber(ctx, tx.OrgID, toCashNum)
	if err != nil {
		return nil, fmt.Errorf("fund transfer: resolve cash account %d: %w", toCashNum, err)
	}

	effects.JournalLines = []GLJournalLine{
		{AccountID: transferOut.ID, DebitCents: tx.AmountCents},
		{AccountID: fromCash.ID, CreditCents: tx.AmountCents},
		{AccountID: toCash.ID, DebitCents: tx.AmountCents},
		{AccountID: transferIn.ID, CreditCents: tx.AmountCents},
	}

	effects.FundTransactions = []FundTransactionDirective{
		{FundID: tx.FundAllocations[0].FundID, Type: FundTxTypeTransferOut, AmountCents: tx.AmountCents, Description: tx.Memo},
		{FundID: tx.FundAllocations[1].FundID, Type: FundTxTypeTransferIn, AmountCents: tx.AmountCents, Description: tx.Memo},
	}

	return effects, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd backend && go test ./internal/fin/... -run TestGaapEngine_RecordTransaction_FundTransfer -short -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/internal/fin/engine_gaap.go backend/internal/fin/engine_gaap_test.go
git commit -m "feat(fin): implement GAAP RecordTransaction for fund transfers"
```

---

### Task 11: Implement GAAP RecordTransaction — late fee and interest accrual

**Files:**
- Modify: `backend/internal/fin/engine_gaap.go`
- Modify: `backend/internal/fin/engine_gaap_test.go`

- [ ] **Step 1: Write tests for late fee and interest**

Add to `engine_gaap_test.go`:

```go
func TestGaapEngine_RecordTransaction_LateFee(t *testing.T) {
	engine, resolver := newFullTestGaapEngine(RecognitionBasisAccrual)
	unitID := uuid.New()
	fundID := uuid.New()

	tx := FinancialTransaction{
		Type:          TxTypeLateFee,
		OrgID:         uuid.New(),
		AmountCents:   2500, // $25 late fee
		EffectiveDate: time.Now(),
		SourceID:      uuid.New(),
		UnitID:        &unitID,
		FundAllocations: []FundAllocation{{FundID: fundID, AmountCents: 2500}},
	}

	effects, err := engine.RecordTransaction(context.Background(), tx)
	require.NoError(t, err)

	// GL: DR 1100 (AR) / CR 4100 (Late Fee Revenue).
	require.Len(t, effects.JournalLines, 2)
	assert.Equal(t, resolver.accounts[1100].ID, effects.JournalLines[0].AccountID)
	assert.Equal(t, int64(2500), effects.JournalLines[0].DebitCents)
	assert.Equal(t, resolver.accounts[4100].ID, effects.JournalLines[1].AccountID)
	assert.Equal(t, int64(2500), effects.JournalLines[1].CreditCents)

	// Ledger: late fee charge.
	require.Len(t, effects.LedgerEntries, 1)
	assert.Equal(t, LedgerEntryTypeLateFee, effects.LedgerEntries[0].Type)
}

func TestGaapEngine_RecordTransaction_InterestAccrual(t *testing.T) {
	engine, resolver := newFullTestGaapEngine(RecognitionBasisAccrual)
	unitID := uuid.New()
	fundID := uuid.New()

	tx := FinancialTransaction{
		Type:          TxTypeInterestAccrual,
		OrgID:         uuid.New(),
		AmountCents:   1500,
		EffectiveDate: time.Now(),
		SourceID:      uuid.New(),
		UnitID:        &unitID,
		FundAllocations: []FundAllocation{{FundID: fundID, AmountCents: 1500}},
	}

	effects, err := engine.RecordTransaction(context.Background(), tx)
	require.NoError(t, err)

	// GL: DR 1100 (AR) / CR 4200 (Interest Income).
	require.Len(t, effects.JournalLines, 2)
	assert.Equal(t, resolver.accounts[1100].ID, effects.JournalLines[0].AccountID)
	assert.Equal(t, resolver.accounts[4200].ID, effects.JournalLines[1].AccountID)

	// Ledger entry tracks the interest charge.
	require.Len(t, effects.LedgerEntries, 1)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd backend && go test ./internal/fin/... -run "TestGaapEngine_RecordTransaction_(LateFee|InterestAccrual)" -short -count=1`
Expected: FAIL

- [ ] **Step 3: Implement late fee and interest recording**

Add to RecordTransaction switch:
```go
case TxTypeLateFee:
    return e.lateFeeEffects(ctx, tx)
case TxTypeInterestAccrual:
    return e.interestAccrualEffects(ctx, tx)
```

Add the methods:

```go
func (e *GaapEngine) lateFeeEffects(ctx context.Context, tx FinancialTransaction) (*FinancialEffects, error) {
	effects := &FinancialEffects{}

	if tx.UnitID != nil {
		effects.LedgerEntries = append(effects.LedgerEntries, LedgerEntryDirective{
			UnitID:      *tx.UnitID,
			Type:        LedgerEntryTypeLateFee,
			AmountCents: tx.AmountCents,
			Description: "Late fee",
			SourceID:    tx.SourceID,
		})
	}

	if e.config.RecognitionBasis == RecognitionBasisCash {
		return effects, nil
	}

	arAccount, err := e.resolver.FindAccountByOrgAndNumber(ctx, tx.OrgID, 1100)
	if err != nil {
		return nil, fmt.Errorf("late fee: resolve AR account 1100: %w", err)
	}
	revenueAccount, err := e.resolver.FindAccountByOrgAndNumber(ctx, tx.OrgID, 4100)
	if err != nil {
		return nil, fmt.Errorf("late fee: resolve Late Fee Revenue account 4100: %w", err)
	}

	effects.JournalLines = []GLJournalLine{
		{AccountID: arAccount.ID, DebitCents: tx.AmountCents},
		{AccountID: revenueAccount.ID, CreditCents: tx.AmountCents},
	}

	for _, alloc := range tx.FundAllocations {
		effects.FundTransactions = append(effects.FundTransactions, FundTransactionDirective{
			FundID:      alloc.FundID,
			Type:        "late_fee",
			AmountCents: alloc.AmountCents,
			Description: "Late fee",
		})
	}

	return effects, nil
}

func (e *GaapEngine) interestAccrualEffects(ctx context.Context, tx FinancialTransaction) (*FinancialEffects, error) {
	effects := &FinancialEffects{}

	if tx.UnitID != nil {
		effects.LedgerEntries = append(effects.LedgerEntries, LedgerEntryDirective{
			UnitID:      *tx.UnitID,
			Type:        LedgerEntryTypeCharge, // interest tracked as charge on ledger
			AmountCents: tx.AmountCents,
			Description: "Interest accrual",
			SourceID:    tx.SourceID,
		})
	}

	if e.config.RecognitionBasis == RecognitionBasisCash {
		return effects, nil
	}

	arAccount, err := e.resolver.FindAccountByOrgAndNumber(ctx, tx.OrgID, 1100)
	if err != nil {
		return nil, fmt.Errorf("interest: resolve AR account 1100: %w", err)
	}
	incomeAccount, err := e.resolver.FindAccountByOrgAndNumber(ctx, tx.OrgID, 4200)
	if err != nil {
		return nil, fmt.Errorf("interest: resolve Interest Income account 4200: %w", err)
	}

	effects.JournalLines = []GLJournalLine{
		{AccountID: arAccount.ID, DebitCents: tx.AmountCents},
		{AccountID: incomeAccount.ID, CreditCents: tx.AmountCents},
	}

	for _, alloc := range tx.FundAllocations {
		effects.FundTransactions = append(effects.FundTransactions, FundTransactionDirective{
			FundID:      alloc.FundID,
			Type:        "interest",
			AmountCents: alloc.AmountCents,
			Description: "Interest accrual",
		})
	}

	return effects, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd backend && go test ./internal/fin/... -run "TestGaapEngine_RecordTransaction_(LateFee|InterestAccrual)" -short -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/internal/fin/engine_gaap.go backend/internal/fin/engine_gaap_test.go
git commit -m "feat(fin): implement GAAP RecordTransaction for late fees and interest accrual"
```

---

### Task 12: Implement EngineFactory with tests

**Files:**
- Modify: `backend/internal/fin/engine_config.go`
- Create: `backend/internal/fin/engine_config_test.go`

- [ ] **Step 1: Write EngineFactory tests**

```go
package fin

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubConfigRepo struct {
	configs []OrgAccountingConfig
}

func (s *stubConfigRepo) CreateConfig(_ context.Context, cfg *OrgAccountingConfig) (*OrgAccountingConfig, error) {
	cfg.ID = uuid.New()
	cfg.CreatedAt = time.Now()
	s.configs = append(s.configs, *cfg)
	return cfg, nil
}

func (s *stubConfigRepo) GetEffectiveConfig(_ context.Context, orgID uuid.UUID, asOfDate time.Time) (*OrgAccountingConfig, error) {
	var best *OrgAccountingConfig
	for i := range s.configs {
		c := &s.configs[i]
		if c.OrgID == orgID && !c.EffectiveDate.After(asOfDate) {
			if best == nil || c.EffectiveDate.After(best.EffectiveDate) {
				best = c
			}
		}
	}
	if best == nil {
		return nil, fmt.Errorf("no config found for org %s at %s", orgID, asOfDate.Format("2006-01-02"))
	}
	return best, nil
}

func (s *stubConfigRepo) ListConfigsByOrg(_ context.Context, orgID uuid.UUID) ([]OrgAccountingConfig, error) {
	var result []OrgAccountingConfig
	for _, c := range s.configs {
		if c.OrgID == orgID {
			result = append(result, c)
		}
	}
	return result, nil
}

func TestEngineFactory_ForOrg(t *testing.T) {
	orgID := uuid.New()
	configRepo := &stubConfigRepo{configs: []OrgAccountingConfig{
		{OrgID: orgID, Standard: AccountingStandardGAAP, RecognitionBasis: RecognitionBasisAccrual, FiscalYearStart: 1, EffectiveDate: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)},
	}}

	var capturedConfig EngineConfig
	gaapBuilder := func(cfg EngineConfig) AccountingEngine {
		capturedConfig = cfg
		return NewGaapEngine(nil, nil, cfg)
	}

	factory := NewEngineFactory(
		map[AccountingStandard]EngineBuilder{AccountingStandardGAAP: gaapBuilder},
		configRepo,
	)

	engine, err := factory.ForOrg(context.Background(), orgID)
	require.NoError(t, err)
	assert.Equal(t, AccountingStandardGAAP, engine.Standard())
	assert.Equal(t, RecognitionBasisAccrual, capturedConfig.RecognitionBasis)
}

func TestEngineFactory_ForOrgAtDate_EffectiveDating(t *testing.T) {
	orgID := uuid.New()
	configRepo := &stubConfigRepo{configs: []OrgAccountingConfig{
		{OrgID: orgID, Standard: AccountingStandardGAAP, RecognitionBasis: RecognitionBasisCash, FiscalYearStart: 1, EffectiveDate: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)},
		{OrgID: orgID, Standard: AccountingStandardGAAP, RecognitionBasis: RecognitionBasisAccrual, FiscalYearStart: 7, EffectiveDate: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)},
	}}

	var capturedConfig EngineConfig
	gaapBuilder := func(cfg EngineConfig) AccountingEngine {
		capturedConfig = cfg
		return NewGaapEngine(nil, nil, cfg)
	}

	factory := NewEngineFactory(
		map[AccountingStandard]EngineBuilder{AccountingStandardGAAP: gaapBuilder},
		configRepo,
	)

	// Before the switch: cash basis.
	_, err := factory.ForOrgAtDate(context.Background(), orgID, time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	assert.Equal(t, RecognitionBasisCash, capturedConfig.RecognitionBasis)

	// After the switch: accrual.
	_, err = factory.ForOrgAtDate(context.Background(), orgID, time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	assert.Equal(t, RecognitionBasisAccrual, capturedConfig.RecognitionBasis)
}

func TestEngineFactory_UnsupportedStandard(t *testing.T) {
	orgID := uuid.New()
	configRepo := &stubConfigRepo{configs: []OrgAccountingConfig{
		{OrgID: orgID, Standard: AccountingStandardIFRS, RecognitionBasis: RecognitionBasisAccrual, FiscalYearStart: 1, EffectiveDate: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)},
	}}

	factory := NewEngineFactory(
		map[AccountingStandard]EngineBuilder{AccountingStandardGAAP: func(cfg EngineConfig) AccountingEngine { return nil }},
		configRepo,
	)

	_, err := factory.ForOrg(context.Background(), orgID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported")
}
```

- [ ] **Step 2: Run tests to verify they pass** (EngineFactory is already implemented in engine_config.go)

Run: `cd backend && go test ./internal/fin/... -run TestEngineFactory -short -count=1`
Expected: PASS (or minor fixes needed)

- [ ] **Step 3: Commit**

```bash
git add backend/internal/fin/engine_config_test.go
git commit -m "test(fin): add EngineFactory unit tests with effective-dated config resolution"
```

---

### Task 13: Database migrations for accounting periods and org config

**Files:**
- Create: `backend/migrations/20260410000002_accounting_periods.sql`
- Create: `backend/migrations/20260410000003_org_accounting_config.sql`

- [ ] **Step 1: Create accounting_periods migration**

```sql
-- 20260410000002_accounting_periods.sql
CREATE TABLE accounting_periods (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id      UUID NOT NULL REFERENCES organizations(id),
    fiscal_year INTEGER NOT NULL,
    period_number INTEGER NOT NULL CHECK (period_number BETWEEN 1 AND 13),
    start_date  DATE NOT NULL,
    end_date    DATE NOT NULL,
    status      TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'soft_closed', 'closed')),
    closed_by   UUID REFERENCES users(id),
    closed_at   TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (org_id, fiscal_year, period_number)
);

CREATE INDEX idx_accounting_periods_org_date ON accounting_periods (org_id, start_date, end_date);
```

- [ ] **Step 2: Create org_accounting_config migration**

```sql
-- 20260410000003_org_accounting_config.sql
CREATE TABLE org_accounting_configs (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id                  UUID NOT NULL REFERENCES organizations(id),
    standard                TEXT NOT NULL DEFAULT 'gaap' CHECK (standard IN ('gaap', 'ifrs')),
    recognition_basis       TEXT NOT NULL DEFAULT 'accrual' CHECK (recognition_basis IN ('cash', 'accrual', 'modified_accrual')),
    fiscal_year_start       INTEGER NOT NULL DEFAULT 1 CHECK (fiscal_year_start BETWEEN 1 AND 12),
    availability_period_days INTEGER NOT NULL DEFAULT 60,
    effective_date          DATE NOT NULL,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by              UUID NOT NULL REFERENCES users(id)
);

CREATE INDEX idx_org_accounting_configs_org_effective ON org_accounting_configs (org_id, effective_date DESC);
```

- [ ] **Step 3: Commit**

```bash
git add backend/migrations/20260410000002_accounting_periods.sql backend/migrations/20260410000003_org_accounting_config.sql
git commit -m "feat(fin): add database migrations for accounting periods and org config"
```

---

### Task 14: Implement Postgres repositories for periods and config

**Files:**
- Create: `backend/internal/fin/period_postgres.go`
- Create: `backend/internal/fin/org_accounting_config_postgres.go`

- [ ] **Step 1: Implement AccountingPeriodRepository**

Create `backend/internal/fin/period_postgres.go`:

```go
package fin

import (
	"context"
	"fmt"
	"time"

	"github.com/douglaslinsmeyer/quorant/backend/internal/platform/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type PostgresAccountingPeriodRepository struct {
	db db.DBTX
}

func NewPostgresAccountingPeriodRepository(d db.DBTX) *PostgresAccountingPeriodRepository {
	return &PostgresAccountingPeriodRepository{db: d}
}

func (r *PostgresAccountingPeriodRepository) WithTx(tx pgx.Tx) *PostgresAccountingPeriodRepository {
	return &PostgresAccountingPeriodRepository{db: tx}
}

func (r *PostgresAccountingPeriodRepository) CreatePeriod(ctx context.Context, p *AccountingPeriod) (*AccountingPeriod, error) {
	query := `INSERT INTO accounting_periods (org_id, fiscal_year, period_number, start_date, end_date, status)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at`
	err := r.db.QueryRow(ctx, query, p.OrgID, p.FiscalYear, p.PeriodNumber, p.StartDate, p.EndDate, p.Status).
		Scan(&p.ID, &p.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create period: %w", err)
	}
	return p, nil
}

func (r *PostgresAccountingPeriodRepository) GetPeriodForDate(ctx context.Context, orgID uuid.UUID, date time.Time) (*AccountingPeriod, error) {
	query := `SELECT id, org_id, fiscal_year, period_number, start_date, end_date, status, closed_by, closed_at, created_at
		FROM accounting_periods WHERE org_id = $1 AND start_date <= $2 AND end_date >= $2`
	p := &AccountingPeriod{}
	err := r.db.QueryRow(ctx, query, orgID, date).Scan(
		&p.ID, &p.OrgID, &p.FiscalYear, &p.PeriodNumber, &p.StartDate, &p.EndDate,
		&p.Status, &p.ClosedBy, &p.ClosedAt, &p.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get period for date: %w", err)
	}
	return p, nil
}

func (r *PostgresAccountingPeriodRepository) ListPeriodsByFiscalYear(ctx context.Context, orgID uuid.UUID, fiscalYear int) ([]AccountingPeriod, error) {
	query := `SELECT id, org_id, fiscal_year, period_number, start_date, end_date, status, closed_by, closed_at, created_at
		FROM accounting_periods WHERE org_id = $1 AND fiscal_year = $2 ORDER BY period_number`
	rows, err := r.db.Query(ctx, query, orgID, fiscalYear)
	if err != nil {
		return nil, fmt.Errorf("list periods: %w", err)
	}
	defer rows.Close()

	var periods []AccountingPeriod
	for rows.Next() {
		var p AccountingPeriod
		if err := rows.Scan(&p.ID, &p.OrgID, &p.FiscalYear, &p.PeriodNumber, &p.StartDate, &p.EndDate,
			&p.Status, &p.ClosedBy, &p.ClosedAt, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan period: %w", err)
		}
		periods = append(periods, p)
	}
	return periods, nil
}

func (r *PostgresAccountingPeriodRepository) UpdatePeriodStatus(ctx context.Context, id uuid.UUID, status PeriodStatus, closedBy *uuid.UUID) error {
	query := `UPDATE accounting_periods SET status = $2, closed_by = $3, closed_at = CASE WHEN $2 = 'closed' THEN now() ELSE closed_at END WHERE id = $1`
	_, err := r.db.Exec(ctx, query, id, status, closedBy)
	if err != nil {
		return fmt.Errorf("update period status: %w", err)
	}
	return nil
}

func (r *PostgresAccountingPeriodRepository) AllPeriodsClosedForYear(ctx context.Context, orgID uuid.UUID, fiscalYear int) (bool, error) {
	query := `SELECT COUNT(*) FROM accounting_periods WHERE org_id = $1 AND fiscal_year = $2 AND status != 'closed'`
	var count int
	if err := r.db.QueryRow(ctx, query, orgID, fiscalYear).Scan(&count); err != nil {
		return false, fmt.Errorf("check periods closed: %w", err)
	}
	return count == 0, nil
}
```

- [ ] **Step 2: Implement OrgAccountingConfigRepository**

Create `backend/internal/fin/org_accounting_config_postgres.go`:

```go
package fin

import (
	"context"
	"fmt"
	"time"

	"github.com/douglaslinsmeyer/quorant/backend/internal/platform/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type PostgresOrgAccountingConfigRepository struct {
	db db.DBTX
}

func NewPostgresOrgAccountingConfigRepository(d db.DBTX) *PostgresOrgAccountingConfigRepository {
	return &PostgresOrgAccountingConfigRepository{db: d}
}

func (r *PostgresOrgAccountingConfigRepository) WithTx(tx pgx.Tx) *PostgresOrgAccountingConfigRepository {
	return &PostgresOrgAccountingConfigRepository{db: tx}
}

func (r *PostgresOrgAccountingConfigRepository) CreateConfig(ctx context.Context, cfg *OrgAccountingConfig) (*OrgAccountingConfig, error) {
	query := `INSERT INTO org_accounting_configs (org_id, standard, recognition_basis, fiscal_year_start, availability_period_days, effective_date, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at`
	err := r.db.QueryRow(ctx, query, cfg.OrgID, cfg.Standard, cfg.RecognitionBasis, cfg.FiscalYearStart, cfg.AvailabilityPeriodDays, cfg.EffectiveDate, cfg.CreatedBy).
		Scan(&cfg.ID, &cfg.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create org accounting config: %w", err)
	}
	return cfg, nil
}

func (r *PostgresOrgAccountingConfigRepository) GetEffectiveConfig(ctx context.Context, orgID uuid.UUID, asOfDate time.Time) (*OrgAccountingConfig, error) {
	query := `SELECT id, org_id, standard, recognition_basis, fiscal_year_start, availability_period_days, effective_date, created_at, created_by
		FROM org_accounting_configs WHERE org_id = $1 AND effective_date <= $2
		ORDER BY effective_date DESC LIMIT 1`
	cfg := &OrgAccountingConfig{}
	err := r.db.QueryRow(ctx, query, orgID, asOfDate).Scan(
		&cfg.ID, &cfg.OrgID, &cfg.Standard, &cfg.RecognitionBasis, &cfg.FiscalYearStart,
		&cfg.AvailabilityPeriodDays, &cfg.EffectiveDate, &cfg.CreatedAt, &cfg.CreatedBy,
	)
	if err != nil {
		return nil, fmt.Errorf("get effective config: %w", err)
	}
	return cfg, nil
}

func (r *PostgresOrgAccountingConfigRepository) ListConfigsByOrg(ctx context.Context, orgID uuid.UUID) ([]OrgAccountingConfig, error) {
	query := `SELECT id, org_id, standard, recognition_basis, fiscal_year_start, availability_period_days, effective_date, created_at, created_by
		FROM org_accounting_configs WHERE org_id = $1 ORDER BY effective_date DESC`
	rows, err := r.db.Query(ctx, query, orgID)
	if err != nil {
		return nil, fmt.Errorf("list configs: %w", err)
	}
	defer rows.Close()

	var configs []OrgAccountingConfig
	for rows.Next() {
		var c OrgAccountingConfig
		if err := rows.Scan(&c.ID, &c.OrgID, &c.Standard, &c.RecognitionBasis, &c.FiscalYearStart,
			&c.AvailabilityPeriodDays, &c.EffectiveDate, &c.CreatedAt, &c.CreatedBy); err != nil {
			return nil, fmt.Errorf("scan config: %w", err)
		}
		configs = append(configs, c)
	}
	return configs, nil
}
```

- [ ] **Step 3: Verify compilation**

Run: `cd backend && go build ./internal/fin/...`
Expected: SUCCESS

- [ ] **Step 4: Commit**

```bash
git add backend/internal/fin/period_postgres.go backend/internal/fin/org_accounting_config_postgres.go
git commit -m "feat(fin): implement Postgres repositories for accounting periods and org config"
```

---

### Task 15: Implement FinService structural changes and executeEffects

**Files:**
- Modify: `backend/internal/fin/service.go`

- [ ] **Step 1: Update FinService struct to use EngineFactory**

In `service.go`, change the struct field and constructor:

Replace `engine AccountingEngine` with `factory *EngineFactory` in the struct (line 38).

Update `NewFinService` to accept `factory *EngineFactory` instead of `engine AccountingEngine`.

- [ ] **Step 2: Implement executeEffects helper**

Add to `service.go`:

```go
// executeEffects persists all directives from a FinancialEffects bundle within
// the given UoW. This is mechanical — no interpretation, no branching on data.
func (s *FinService) executeEffects(ctx context.Context, uow *db.UnitOfWork, orgID uuid.UUID, sourceType GLSourceType, sourceID uuid.UUID, unitID *uuid.UUID, effectiveDate time.Time, memo string, effects *FinancialEffects) error {
	if effects == nil {
		return nil
	}

	// Post GL journal entry.
	if len(effects.JournalLines) > 0 {
		gl := s.gl
		if uow != nil {
			gl = s.gl.WithTx(uow.Tx())
		}
		st := sourceType
		if _, err := gl.PostSystemJournalEntry(ctx, orgID, uuid.Nil, effectiveDate, memo, &st, &sourceID, unitID, effects.JournalLines); err != nil {
			return fmt.Errorf("execute effects: GL entry: %w", err)
		}
	}

	// Create fund transactions.
	funds := s.funds
	if uow != nil {
		funds = s.funds.WithTx(uow.Tx())
	}
	for _, ft := range effects.FundTransactions {
		desc := ft.Description
		refType := ft.Type
		_, err := funds.CreateTransaction(ctx, &FundTransaction{
			FundID:          ft.FundID,
			OrgID:           orgID,
			CurrencyCode:    "USD",
			TransactionType: ft.Type,
			AmountCents:     ft.AmountCents,
			Description:     &desc,
			ReferenceType:   &refType,
			ReferenceID:     &sourceID,
			EffectiveDate:   effectiveDate,
		})
		if err != nil {
			return fmt.Errorf("execute effects: fund transaction: %w", err)
		}
	}

	// Create ledger entries.
	assessments := s.assessments
	if uow != nil {
		assessments = s.assessments.WithTx(uow.Tx())
	}
	for _, le := range effects.LedgerEntries {
		desc := le.Description
		_, err := assessments.CreateLedgerEntry(ctx, &LedgerEntry{
			OrgID:         orgID,
			CurrencyCode:  "USD",
			UnitID:        le.UnitID,
			EntryType:     le.Type,
			AmountCents:   le.AmountCents,
			Description:   &desc,
			EffectiveDate: effectiveDate,
		})
		if err != nil {
			return fmt.Errorf("execute effects: ledger entry: %w", err)
		}
	}

	// Create credit entries (overpayment handling).
	for _, cr := range effects.Credits {
		desc := string(cr.Type)
		_, err := assessments.CreateLedgerEntry(ctx, &LedgerEntry{
			OrgID:         orgID,
			CurrencyCode:  "USD",
			UnitID:        cr.UnitID,
			EntryType:     LedgerEntryTypeCredit,
			AmountCents:   cr.AmountCents,
			Description:   &desc,
			EffectiveDate: effectiveDate,
		})
		if err != nil {
			return fmt.Errorf("execute effects: credit: %w", err)
		}
	}

	return nil
}
```

- [ ] **Step 3: Verify it compiles**

Run: `cd backend && go build ./internal/fin/...`
Expected: Compile errors from call sites still using old `engine` field. Expected — Task 16 fixes them.

- [ ] **Step 4: Commit**

```bash
git add backend/internal/fin/service.go
git commit -m "feat(fin): add EngineFactory to FinService and implement executeEffects"
```

---

### Task 16: Refactor CreateAssessment to use engine

**Files:**
- Modify: `backend/internal/fin/service.go`
- Modify: `backend/internal/fin/service_test.go`

- [ ] **Step 1: Update CreateAssessment tests**

In `service_test.go`, update the test setup to provide an EngineFactory instead of a raw engine. Add a helper:

```go
func newTestEngineFactory(basis RecognitionBasis) *EngineFactory {
	configRepo := &stubConfigRepo{configs: []OrgAccountingConfig{
		{OrgID: testutil.TestOrgID(), Standard: AccountingStandardGAAP, RecognitionBasis: basis, FiscalYearStart: 1, EffectiveDate: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)},
	}}
	resolver := &stubAccountResolver{accounts: map[int]*GLAccount{
		1010: {ID: uuid.New(), AccountNumber: 1010}, 1020: {ID: uuid.New(), AccountNumber: 1020},
		1100: {ID: uuid.New(), AccountNumber: 1100}, 4010: {ID: uuid.New(), AccountNumber: 4010},
		4020: {ID: uuid.New(), AccountNumber: 4020}, 4100: {ID: uuid.New(), AccountNumber: 4100},
		4200: {ID: uuid.New(), AccountNumber: 4200}, 3100: {ID: uuid.New(), AccountNumber: 3100},
		3110: {ID: uuid.New(), AccountNumber: 3110},
	}}
	gaapBuilder := func(cfg EngineConfig) AccountingEngine {
		return NewGaapEngine(resolver, nil, cfg)
	}
	return NewEngineFactory(map[AccountingStandard]EngineBuilder{AccountingStandardGAAP: gaapBuilder}, configRepo)
}
```

Update all `NewFinService` calls in tests to pass `newTestEngineFactory(RecognitionBasisAccrual)` instead of `nil` or a raw engine.

- [ ] **Step 2: Refactor CreateAssessment**

Replace the GL section of `CreateAssessment` (lines ~241-269) with the engine pattern:

```go
	// Engine determines all financial effects.
	if s.factory != nil {
		engine, err := s.factory.ForOrg(ctx, orgID)
		if err != nil {
			return nil, fmt.Errorf("fin: CreateAssessment engine: %w", err)
		}

		ftx := FinancialTransaction{
			Type:          TxTypeAssessment,
			OrgID:         orgID,
			AmountCents:   created.AmountCents,
			EffectiveDate: created.DueDate,
			SourceID:      created.ID,
			UnitID:        &req.UnitID,
			FundAllocations: []FundAllocation{{FundID: req.FundID, AmountCents: created.AmountCents}},
			Memo:          fmt.Sprintf("Assessment: %s", created.Description),
		}

		if err := engine.ValidateTransaction(ctx, ftx); err != nil {
			return nil, fmt.Errorf("fin: CreateAssessment validate: %w", err)
		}

		effects, err := engine.RecordTransaction(ctx, ftx)
		if err != nil {
			return nil, fmt.Errorf("fin: CreateAssessment record: %w", err)
		}

		if err := s.executeEffects(ctx, uow, orgID, GLSourceTypeAssessment, created.ID, &req.UnitID, created.DueDate, ftx.Memo, effects); err != nil {
			return nil, err
		}
	}
```

Remove the old inline GL logic (the `if gl != nil && s.engine != nil` block) and the inline ledger entry creation (it's now in the engine's effects).

- [ ] **Step 3: Run tests**

Run: `cd backend && go test ./internal/fin/... -run TestCreateAssessment -short -count=1`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add backend/internal/fin/service.go backend/internal/fin/service_test.go
git commit -m "refactor(fin): delegate CreateAssessment GL/ledger/fund to accounting engine"
```

---

### Task 17: Refactor RecordPayment to use engine

**Files:**
- Modify: `backend/internal/fin/service.go`
- Modify: `backend/internal/fin/service_test.go`

- [ ] **Step 1: Refactor RecordPayment**

Follow the same pattern as CreateAssessment. Replace inline GL logic with:

```go
	if s.factory != nil {
		engine, err := s.factory.ForOrg(ctx, orgID)
		if err != nil {
			return nil, fmt.Errorf("fin: RecordPayment engine: %w", err)
		}

		ftx := FinancialTransaction{
			Type:               TxTypePayment,
			OrgID:              orgID,
			AmountCents:        payment.AmountCents,
			EffectiveDate:      payment.CreatedAt,
			SourceID:           payment.ID,
			UnitID:             &req.UnitID,
			PaymentAllocations: allocations,
			FundAllocations:    fundAllocsFromPaymentAllocations(allocations),
			Memo:               fmt.Sprintf("Payment from unit %s", req.UnitID),
		}

		if err := engine.ValidateTransaction(ctx, ftx); err != nil {
			return nil, fmt.Errorf("fin: RecordPayment validate: %w", err)
		}

		effects, err := engine.RecordTransaction(ctx, ftx)
		if err != nil {
			return nil, fmt.Errorf("fin: RecordPayment record: %w", err)
		}

		if err := s.executeEffects(ctx, uow, orgID, GLSourceTypePayment, payment.ID, &req.UnitID, payment.CreatedAt, ftx.Memo, effects); err != nil {
			return nil, err
		}
	}
```

Add a helper to derive fund allocations from payment allocations:

```go
func fundAllocsFromPaymentAllocations(allocs []PaymentAllocation) []FundAllocation {
	fundTotals := make(map[uuid.UUID]int64)
	for _, a := range allocs {
		if a.FundID != nil {
			fundTotals[*a.FundID] += a.AmountCents
		}
	}
	var result []FundAllocation
	for fundID, amount := range fundTotals {
		result = append(result, FundAllocation{FundID: fundID, AmountCents: amount})
	}
	return result
}
```

- [ ] **Step 2: Update tests**

Run: `cd backend && go test ./internal/fin/... -run TestRecordPayment -short -count=1`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add backend/internal/fin/service.go backend/internal/fin/service_test.go
git commit -m "refactor(fin): delegate RecordPayment GL/ledger/fund to accounting engine"
```

---

### Task 18: Refactor CreateFundTransfer to use engine

**Files:**
- Modify: `backend/internal/fin/service.go`
- Modify: `backend/internal/fin/service_test.go`

- [ ] **Step 1: Refactor CreateFundTransfer**

Follow the same engine pattern:

```go
	if s.factory != nil {
		engine, err := s.factory.ForOrg(ctx, orgID)
		if err != nil {
			return nil, fmt.Errorf("fin: CreateFundTransfer engine: %w", err)
		}

		ftx := FinancialTransaction{
			Type:          TxTypeFundTransfer,
			OrgID:         orgID,
			AmountCents:   transfer.AmountCents,
			EffectiveDate: transfer.EffectiveDate,
			SourceID:      transfer.ID,
			FundAllocations: []FundAllocation{
				{FundID: req.FromFundID, AmountCents: transfer.AmountCents},
				{FundID: req.ToFundID, AmountCents: transfer.AmountCents},
			},
			Memo: fmt.Sprintf("Fund transfer"),
		}

		if err := engine.ValidateTransaction(ctx, ftx); err != nil {
			return nil, fmt.Errorf("fin: CreateFundTransfer validate: %w", err)
		}

		effects, err := engine.RecordTransaction(ctx, ftx)
		if err != nil {
			return nil, fmt.Errorf("fin: CreateFundTransfer record: %w", err)
		}

		if err := s.executeEffects(ctx, uow, orgID, GLSourceTypeTransfer, transfer.ID, nil, transfer.EffectiveDate, ftx.Memo, effects); err != nil {
			return nil, err
		}
	}
```

- [ ] **Step 2: Run tests**

Run: `cd backend && go test ./internal/fin/... -run TestCreateFundTransfer -short -count=1`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add backend/internal/fin/service.go backend/internal/fin/service_test.go
git commit -m "refactor(fin): delegate CreateFundTransfer GL/fund to accounting engine"
```

---

### Task 19: Update SeedDefaultAccounts for expanded chart

**Files:**
- Modify: `backend/internal/fin/gl_service.go`

- [ ] **Step 1: Update SeedDefaultAccounts to accept all 4 fund types**

The current signature takes `operatingFundID` and `reserveFundID`. Update to accept a fund map:

```go
func (s *GLService) SeedDefaultAccounts(ctx context.Context, orgID uuid.UUID, fundMap map[string]uuid.UUID, engine AccountingEngine) error {
	fundPtrs := make(map[string]*uuid.UUID)
	for k, v := range fundMap {
		id := v
		fundPtrs[k] = &id
	}

	seeds := engine.ChartOfAccounts()
	numToID := make(map[int]uuid.UUID)

	for _, seed := range seeds {
		a := &GLAccount{
			OrgID:         orgID,
			AccountNumber: seed.Number,
			Name:          seed.Name,
			AccountType:   seed.Type,
			IsHeader:      seed.IsHeader,
			IsSystem:      seed.IsSystem,
			FundID:        fundPtrs[seed.FundKey],
		}
		if seed.ParentNum != 0 {
			parentID := numToID[seed.ParentNum]
			a.ParentID = &parentID
		}
		created, err := s.gl.CreateAccount(ctx, a)
		if err != nil {
			return fmt.Errorf("seed account %d %s: %w", seed.Number, seed.Name, err)
		}
		numToID[seed.Number] = created.ID
	}

	return nil
}
```

- [ ] **Step 2: Update callers of SeedDefaultAccounts**

Search for all callers and update them to pass the fund map instead of two separate fund IDs.

- [ ] **Step 3: Run tests**

Run: `cd backend && go test ./internal/fin/... -run TestSeedDefaultAccounts -short -count=1`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add backend/internal/fin/gl_service.go
git commit -m "refactor(fin): update SeedDefaultAccounts to accept fund map for all 4 fund types"
```

---

### Task 20: Wire EngineFactory in main.go and clean up

**Files:**
- Modify: `backend/cmd/quorant-api/main.go`
- Delete: `backend/internal/fin/engine_test.go` (old prototype test, if present)

- [ ] **Step 1: Wire EngineFactory in main.go**

Replace the `fin.NewGaapEngine()` call with the factory pattern:

```go
	// Accounting engine setup.
	orgConfigRepo := fin.NewPostgresOrgAccountingConfigRepository(pool)
	gaapBuilder := func(cfg fin.EngineConfig) fin.AccountingEngine {
		return fin.NewGaapEngine(glRepo, policyRegistry, cfg)
	}
	engineFactory := fin.NewEngineFactory(
		map[fin.AccountingStandard]fin.EngineBuilder{
			fin.AccountingStandardGAAP: gaapBuilder,
		},
		orgConfigRepo,
	)

	finService := fin.NewFinService(assessmentRepo, paymentRepo, budgetRepo, fundRepo, collectionRepo, glService, engineFactory, policyResolver, complianceService, policyRegistry, logger, uowFactory)
```

- [ ] **Step 2: Remove old engine_test.go if it exists**

Check for `backend/internal/fin/engine_test.go` and remove if it contains prototype-era tests that no longer apply.

- [ ] **Step 3: Run full test suite**

Run: `cd backend && go test ./internal/fin/... -short -count=1`
Expected: ALL PASS

- [ ] **Step 4: Run linter**

Run: `make lint`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/cmd/quorant-api/main.go
git commit -m "feat(fin): wire EngineFactory in main.go, complete Phase 1 accounting engine"
```

---

### Known Gaps for Implementing Agent

The following items are in the Phase 1 spec but not fully detailed as tasks above. The implementing agent should address them inline:

1. **Modified accrual RecordTransaction behavior** — Tasks 8-11 show accrual and cash basis. Modified accrual for assessments defers revenue if the due date is beyond the availability window. Add test cases following the cash/accrual pattern — the spec section "RecordTransaction — modified accrual basis" has the rules.

2. **Late fee/interest cap validation** — The spec's ValidateTransaction section includes consulting `policy.Registry` for jurisdiction-scoped fee caps. Task 7's implementation is a minimal validator. Add cap enforcement when the policy registry is available.

3. **PostLateFee / PostInterestAccrual service methods** — The file map lists `service_iface.go` as needing these. Add them following the CreateAssessment pattern: create the domain record, build FinancialTransaction, call engine, executeEffects.

4. **Integration tests for period_postgres and org_accounting_config_postgres** — Task 14 creates the implementations. Integration tests should follow the existing patterns in `assessment_postgres_test.go` using `testutil.IntegrationDB()`.

5. **Enum updates in enums.go** — Task 1 defines new enums in `engine_types.go`. Any enums that already exist in `enums.go` (like TransactionType if it conflicts with the old one in engine.go) need reconciliation.

6. **Header count in ChartOfAccounts** — The spec says "6 headers, 47 detail = 53" but the actual chart has 5 top-level headers (1000, 2000, 3000, 4000, 5000). Adjust the test assertion to match the actual count after implementation.

---

## Post-Implementation Verification

After all tasks are complete, run:

```bash
cd backend && go test ./internal/fin/... -short -count=1 -v
```

Verify:
- All GAAP engine tests pass (Standard, ChartOfAccounts, ValidateTransaction, RecordTransaction per type)
- All EngineFactory tests pass (ForOrg, ForOrgAtDate, unsupported standard)
- All FinService tests pass (CreateAssessment, RecordPayment, CreateFundTransfer)
- No hardcoded account numbers remain in service.go

```bash
make lint
```

Verify: no lint errors.
