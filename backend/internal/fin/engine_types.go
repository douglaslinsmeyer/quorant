package fin

import (
	"time"

	"github.com/google/uuid"
)

// TransactionType identifies the kind of financial event being recorded.
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

// IsValid returns true if the TransactionType value is one of the defined constants.
func (t TransactionType) IsValid() bool {
	switch t {
	case TxTypeAssessment, TxTypePayment, TxTypeFundTransfer, TxTypeInterfundLoan,
		TxTypeExpense, TxTypeLateFee, TxTypeInterestAccrual, TxTypeBadDebtProvision,
		TxTypeBadDebtWriteOff, TxTypeBadDebtRecovery, TxTypeYearEndClose,
		TxTypeVoidReversal, TxTypeAdjustingEntry, TxTypeDepreciation:
		return true
	default:
		return false
	}
}

// AccountingStandard identifies which accounting standard an engine implements.
type AccountingStandard string

const (
	AccountingStandardGAAP AccountingStandard = "gaap"
	AccountingStandardIFRS AccountingStandard = "ifrs"
)

// IsValid returns true if the AccountingStandard value is one of the defined constants.
func (s AccountingStandard) IsValid() bool {
	switch s {
	case AccountingStandardGAAP, AccountingStandardIFRS:
		return true
	default:
		return false
	}
}

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
	FundKey     string // "operating", "reserve", "capital", "special"
	AmountCents int64
}

// ExternalReference carries identifiers from external systems.
type ExternalReference struct {
	Type       string // "check", "ach", "gateway", "lockbox", "wire"
	ExternalID string
	BatchID    *string
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

// ExpenseType classifies an expense for recognition timing.
// TODO: Move ExpenseType and its constants to enums.go when expense categorization is implemented.
type ExpenseType string

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
