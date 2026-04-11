package fin

import (
	"time"

	"github.com/google/uuid"
)

// FinancialEffects is the unified output from RecordTransaction. A closed,
// enumerable set of directives that FinService executes mechanically.
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
	Type        string // uses FundTxType* constants from enums.go
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
