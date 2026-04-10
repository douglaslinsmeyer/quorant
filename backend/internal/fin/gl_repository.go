package fin

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// GLRepository persists and retrieves GL accounts, journal entries, and
// journal lines for the Finance module's double-entry general ledger.
type GLRepository interface {
	// ── Chart of Accounts ────────────────────────────────────────────

	// CreateAccount inserts a new GL account and returns the
	// fully-populated row (including generated id and timestamps).
	CreateAccount(ctx context.Context, a *GLAccount) (*GLAccount, error)

	// FindAccountByID returns the account with the given id, or nil, nil
	// if no matching (non-deleted) row exists.
	FindAccountByID(ctx context.Context, id uuid.UUID) (*GLAccount, error)

	// ListAccountsByOrg returns all non-deleted accounts for the given
	// org, ordered by account_number. Returns an empty (non-nil) slice
	// when none exist.
	ListAccountsByOrg(ctx context.Context, orgID uuid.UUID) ([]GLAccount, error)

	// FindAccountByOrgAndNumber returns the account with the given org
	// and account_number, or nil, nil if no matching (non-deleted) row
	// exists.
	FindAccountByOrgAndNumber(ctx context.Context, orgID uuid.UUID, number int) (*GLAccount, error)

	// UpdateAccount persists changes to an existing GL account and
	// returns the updated row.
	UpdateAccount(ctx context.Context, a *GLAccount) (*GLAccount, error)

	// SoftDeleteAccount marks the account as deleted without removing
	// the row. Returns an error if the account does not exist.
	SoftDeleteAccount(ctx context.Context, id uuid.UUID) error

	// ── Journal Entries ──────────────────────────────────────────────

	// PostJournalEntry inserts a journal entry header and all lines
	// within a single database transaction. It assigns the next
	// sequential entry_number for the org. Returns the fully-populated
	// entry including lines.
	PostJournalEntry(ctx context.Context, entry *GLJournalEntry) (*GLJournalEntry, error)

	// FindJournalEntryByID returns the journal entry with the given id
	// (including its lines), or nil, nil if not found.
	FindJournalEntryByID(ctx context.Context, id uuid.UUID) (*GLJournalEntry, error)

	// ListJournalEntriesByOrg returns all journal entries for the given
	// org, ordered by entry_number DESC. Returns an empty (non-nil)
	// slice when none exist.
	ListJournalEntriesByOrg(ctx context.Context, orgID uuid.UUID) ([]GLJournalEntry, error)

	// FindJournalEntriesBySource returns all journal entries whose source_type
	// and source_id match the given values. Returns an empty (non-nil) slice
	// when none exist.
	FindJournalEntriesBySource(ctx context.Context, sourceType GLSourceType, sourceID uuid.UUID) ([]GLJournalEntry, error)

	// UpdateJournalEntryReversedBy sets the reversed_by field on the given journal entry.
	UpdateJournalEntryReversedBy(ctx context.Context, entryID, reversalID uuid.UUID) error

	// ── Reporting ────────────────────────────────────────────────────

	// GetTrialBalance returns debit and credit totals for every account
	// with activity on or before asOfDate. Returns an empty (non-nil)
	// slice when no data exists.
	GetTrialBalance(ctx context.Context, orgID uuid.UUID, asOfDate time.Time) ([]TrialBalanceRow, error)

	// GetAccountBalances returns the net balance for every account with
	// activity in the given date range [from, to]. Returns an empty
	// (non-nil) slice when no data exists.
	GetAccountBalances(ctx context.Context, orgID uuid.UUID, from, to time.Time) ([]AccountBalance, error)

	// HasPostedLines returns true if any journal lines reference the
	// given account. Used to prevent deletion of accounts with activity.
	HasPostedLines(ctx context.Context, accountID uuid.UUID) (bool, error)

	// WithTx returns a copy of the repository that runs queries against the
	// given transaction. Used by UnitOfWork to enlist the repo in a shared tx.
	WithTx(tx pgx.Tx) GLRepository
}
