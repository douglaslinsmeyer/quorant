package fin

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// BankTxStatus represents the matching state of an imported bank transaction.
type BankTxStatus string

const (
	BankTxStatusUnmatched BankTxStatus = "unmatched"
	BankTxStatusMatched   BankTxStatus = "matched"
	BankTxStatusExcluded  BankTxStatus = "excluded"
)

// IsValid returns true if the BankTxStatus value is one of the defined constants.
func (s BankTxStatus) IsValid() bool {
	switch s {
	case BankTxStatusUnmatched, BankTxStatusMatched, BankTxStatusExcluded:
		return true
	}
	return false
}

// BankTransaction represents an imported bank statement line used for
// reconciliation against GL journal lines.
type BankTransaction struct {
	ID              uuid.UUID    `json:"id"`
	OrgID           uuid.UUID    `json:"org_id"`
	AccountID       uuid.UUID    `json:"account_id"`
	TransactionDate time.Time    `json:"transaction_date"`
	AmountCents     int64        `json:"amount_cents"`
	Description     string       `json:"description"`
	Reference       string       `json:"reference"`
	MatchedLineID   *uuid.UUID   `json:"matched_line_id,omitempty"`
	Status          BankTxStatus `json:"status"`
	ImportBatchID   *uuid.UUID   `json:"import_batch_id,omitempty"`
	CreatedAt       time.Time    `json:"created_at"`
}

// BankTransactionRepository persists and retrieves imported bank transactions
// for reconciliation against GL journal lines.
type BankTransactionRepository interface {
	// CreateBankTransaction inserts a new bank transaction and returns the
	// fully-populated row (including generated id and timestamps).
	CreateBankTransaction(ctx context.Context, bt *BankTransaction) (*BankTransaction, error)

	// ListUnmatched returns all bank transactions with status "unmatched" for
	// the given org and GL account. Results are ordered by transaction_date ASC.
	ListUnmatched(ctx context.Context, orgID uuid.UUID, accountID uuid.UUID) ([]BankTransaction, error)

	// MatchToJournalLine marks the bank transaction as "matched" and links it
	// to the given GL journal line. Also marks the journal line as reconciled.
	MatchToJournalLine(ctx context.Context, bankTxID uuid.UUID, journalLineID uuid.UUID) error
}
