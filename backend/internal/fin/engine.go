package fin

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// AccountingStandard identifies which accounting standard an engine implements.
type AccountingStandard string

const (
	AccountingStandardGAAP AccountingStandard = "gaap"
	AccountingStandardIFRS AccountingStandard = "ifrs"
)

// IsValid returns true if the AccountingStandard is a recognized value.
func (s AccountingStandard) IsValid() bool {
	switch s {
	case AccountingStandardGAAP, AccountingStandardIFRS:
		return true
	}
	return false
}

// TransactionType classifies a financial transaction for the engine.
type TransactionType string

const (
	TxTypeAssessment   TransactionType = "assessment"
	TxTypePayment      TransactionType = "payment"
	TxTypeFundTransfer TransactionType = "fund_transfer"
)

// IsValid returns true if the TransactionType is a recognized value.
func (t TransactionType) IsValid() bool {
	switch t {
	case TxTypeAssessment, TxTypePayment, TxTypeFundTransfer:
		return true
	}
	return false
}

// FinancialTransaction is the standard envelope passed to the engine.
// The engine inspects Type to decide which accounting rules apply.
type FinancialTransaction struct {
	Type          TransactionType
	OrgID         uuid.UUID
	AmountCents   int64
	EffectiveDate time.Time
	SourceID      uuid.UUID
	UnitID        *uuid.UUID
	FundID        *uuid.UUID
	Memo          string
	Metadata      map[string]any
}

// GLAccountSeed defines a single account to create when seeding an org's chart of accounts.
type GLAccountSeed struct {
	Number    int
	Name      string
	Type      GLAccountType
	IsHeader  bool
	IsSystem  bool
	FundKey   string // "operating", "reserve", or "" for no fund
	ParentNum int    // 0 means no parent
}

// AccountingEngine defines the contract for accounting standard drivers.
// Phase 1 covers JournalLines and ChartOfAccounts. Future phases will add
// ValidateTransaction, PaymentApplicationStrategy, PayableRecognitionDate,
// RevenueRecognitionDate, and PaymentTerms.
type AccountingEngine interface {
	// Standard returns the accounting standard this engine implements.
	Standard() AccountingStandard

	// JournalLines resolves GL account mappings and returns the journal lines
	// to post for the given transaction. The GLService is passed so the engine
	// can look up accounts by org+number within the current transaction.
	JournalLines(ctx context.Context, gl *GLService, tx FinancialTransaction) ([]GLJournalLine, error)

	// ChartOfAccounts returns the default chart of accounts for this standard.
	ChartOfAccounts() []GLAccountSeed
}
