package fin

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	dbpkg "github.com/quorant/quorant/internal/platform/db"
)

// PostgresGLRepository implements GLRepository using a DBTX.
type PostgresGLRepository struct {
	db dbpkg.DBTX
}

// NewPostgresGLRepository creates a new PostgresGLRepository backed by pool.
func NewPostgresGLRepository(pool *pgxpool.Pool) *PostgresGLRepository {
	return &PostgresGLRepository{db: pool}
}

// WithTx returns a new PostgresGLRepository scoped to the given transaction,
// enabling participation in a caller-managed transaction.
func (r *PostgresGLRepository) WithTx(tx pgx.Tx) GLRepository {
	return &PostgresGLRepository{db: tx}
}

// ─── Chart of Accounts ───────────────────────────────────────────────────────

// CreateAccount inserts a new GL account and returns the fully-populated row.
func (r *PostgresGLRepository) CreateAccount(ctx context.Context, a *GLAccount) (*GLAccount, error) {
	const q = `
		INSERT INTO gl_accounts (
			org_id, parent_id, fund_id, account_number, name,
			account_type, is_header, is_system, description
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9
		)
		RETURNING id, org_id, parent_id, fund_id, account_number, name,
		          account_type, is_header, is_system, description,
		          created_at, updated_at, deleted_at`

	row := r.db.QueryRow(ctx, q,
		a.OrgID,
		a.ParentID,
		a.FundID,
		a.AccountNumber,
		a.Name,
		a.AccountType,
		a.IsHeader,
		a.IsSystem,
		a.Description,
	)

	result, err := scanGLAccount(row)
	if err != nil {
		return nil, fmt.Errorf("fin: CreateAccount: %w", err)
	}
	return result, nil
}

// FindAccountByID returns the account with the given id, or nil, nil if not
// found or soft-deleted.
func (r *PostgresGLRepository) FindAccountByID(ctx context.Context, id uuid.UUID) (*GLAccount, error) {
	const q = `
		SELECT id, org_id, parent_id, fund_id, account_number, name,
		       account_type, is_header, is_system, description,
		       created_at, updated_at, deleted_at
		FROM gl_accounts
		WHERE id = $1 AND deleted_at IS NULL`

	row := r.db.QueryRow(ctx, q, id)
	result, err := scanGLAccount(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("fin: FindAccountByID: %w", err)
	}
	return result, nil
}

// ListAccountsByOrg returns all non-deleted accounts for the given org ordered
// by account_number. Returns an empty (non-nil) slice when none exist.
func (r *PostgresGLRepository) ListAccountsByOrg(ctx context.Context, orgID uuid.UUID) ([]GLAccount, error) {
	const q = `
		SELECT id, org_id, parent_id, fund_id, account_number, name,
		       account_type, is_header, is_system, description,
		       created_at, updated_at, deleted_at
		FROM gl_accounts
		WHERE org_id = $1 AND deleted_at IS NULL
		ORDER BY account_number`

	rows, err := r.db.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("fin: ListAccountsByOrg: %w", err)
	}
	defer rows.Close()

	return collectGLAccounts(rows, "ListAccountsByOrg")
}

// FindAccountByOrgAndNumber returns the account with the given org and
// account_number, or nil, nil if no matching (non-deleted) row exists.
func (r *PostgresGLRepository) FindAccountByOrgAndNumber(ctx context.Context, orgID uuid.UUID, number int) (*GLAccount, error) {
	const q = `
		SELECT id, org_id, parent_id, fund_id, account_number, name,
		       account_type, is_header, is_system, description,
		       created_at, updated_at, deleted_at
		FROM gl_accounts
		WHERE org_id = $1 AND account_number = $2 AND deleted_at IS NULL`

	row := r.db.QueryRow(ctx, q, orgID, number)
	result, err := scanGLAccount(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("fin: FindAccountByOrgAndNumber: %w", err)
	}
	return result, nil
}

// UpdateAccount persists changes to an existing GL account and returns the
// updated row.
func (r *PostgresGLRepository) UpdateAccount(ctx context.Context, a *GLAccount) (*GLAccount, error) {
	const q = `
		UPDATE gl_accounts SET
			name        = $1,
			description = $2,
			fund_id     = $3,
			updated_at  = now()
		WHERE id = $4 AND deleted_at IS NULL
		RETURNING id, org_id, parent_id, fund_id, account_number, name,
		          account_type, is_header, is_system, description,
		          created_at, updated_at, deleted_at`

	row := r.db.QueryRow(ctx, q,
		a.Name,
		a.Description,
		a.FundID,
		a.ID,
	)

	result, err := scanGLAccount(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("fin: UpdateAccount: account %s not found or already deleted", a.ID)
	}
	if err != nil {
		return nil, fmt.Errorf("fin: UpdateAccount: %w", err)
	}
	return result, nil
}

// SoftDeleteAccount marks the account as deleted without removing the row.
// Returns an error if the account does not exist.
func (r *PostgresGLRepository) SoftDeleteAccount(ctx context.Context, id uuid.UUID) error {
	const q = `
		UPDATE gl_accounts
		SET deleted_at = now(), updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL`

	_, err := r.db.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("fin: SoftDeleteAccount: %w", err)
	}
	return nil
}

// ─── Journal Entries ─────────────────────────────────────────────────────────

// PostJournalEntry inserts a journal entry header and all lines within a
// single database transaction. It assigns the next sequential entry_number
// for the org. Returns the fully-populated entry including lines.
func (r *PostgresGLRepository) PostJournalEntry(ctx context.Context, entry *GLJournalEntry) (*GLJournalEntry, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("fin: PostJournalEntry begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// 1. Acquire advisory lock scoped to the org for concurrency-safe
	//    entry_number assignment.
	_, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1::text))`, entry.OrgID)
	if err != nil {
		return nil, fmt.Errorf("fin: PostJournalEntry advisory lock: %w", err)
	}

	// 2. Get the next entry number for this org.
	var entryNumber int
	err = tx.QueryRow(ctx, `
		SELECT COALESCE(MAX(entry_number), 0) + 1
		FROM gl_journal_entries
		WHERE org_id = $1`, entry.OrgID).Scan(&entryNumber)
	if err != nil {
		return nil, fmt.Errorf("fin: PostJournalEntry next number: %w", err)
	}

	// 3. Insert journal entry header.
	row := tx.QueryRow(ctx, `
		INSERT INTO gl_journal_entries (
			org_id, entry_number, entry_date, memo, source_type, source_id,
			unit_id, posted_by, reversed_by, is_reversal
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, org_id, entry_number, entry_date, memo, source_type,
		          source_id, unit_id, posted_by, reversed_by, is_reversal, created_at`,
		entry.OrgID, entryNumber, entry.EntryDate, entry.Memo,
		entry.SourceType, entry.SourceID, entry.UnitID, entry.PostedBy,
		entry.ReversedBy, entry.IsReversal,
	)
	result, err := scanGLJournalEntry(row)
	if err != nil {
		return nil, fmt.Errorf("fin: PostJournalEntry insert header: %w", err)
	}

	// 4. Insert all lines.
	result.Lines = make([]GLJournalLine, 0, len(entry.Lines))
	for _, line := range entry.Lines {
		lineRow := tx.QueryRow(ctx, `
			INSERT INTO gl_journal_lines (
				journal_entry_id, account_id, debit_cents, credit_cents, memo
			) VALUES ($1, $2, $3, $4, $5)
			RETURNING id, journal_entry_id, account_id, debit_cents, credit_cents, memo`,
			result.ID, line.AccountID, line.DebitCents, line.CreditCents, line.Memo,
		)
		l, err := scanGLJournalLine(lineRow)
		if err != nil {
			return nil, fmt.Errorf("fin: PostJournalEntry insert line: %w", err)
		}
		result.Lines = append(result.Lines, *l)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("fin: PostJournalEntry commit: %w", err)
	}
	return result, nil
}

// FindJournalEntryByID returns the journal entry with the given id (including
// its lines), or nil, nil if not found.
func (r *PostgresGLRepository) FindJournalEntryByID(ctx context.Context, id uuid.UUID) (*GLJournalEntry, error) {
	const headerQ = `
		SELECT id, org_id, entry_number, entry_date, memo, source_type,
		       source_id, unit_id, posted_by, reversed_by, is_reversal, created_at
		FROM gl_journal_entries
		WHERE id = $1`

	row := r.db.QueryRow(ctx, headerQ, id)
	entry, err := scanGLJournalEntry(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("fin: FindJournalEntryByID: %w", err)
	}

	const linesQ = `
		SELECT id, journal_entry_id, account_id, debit_cents, credit_cents, memo
		FROM gl_journal_lines
		WHERE journal_entry_id = $1
		ORDER BY id`

	rows, err := r.db.Query(ctx, linesQ, entry.ID)
	if err != nil {
		return nil, fmt.Errorf("fin: FindJournalEntryByID lines: %w", err)
	}
	defer rows.Close()

	lines, err := collectGLJournalLines(rows, "FindJournalEntryByID")
	if err != nil {
		return nil, err
	}
	entry.Lines = lines

	return entry, nil
}

// ListJournalEntriesByOrg returns all journal entries for the given org,
// ordered by entry_number DESC. Lines are NOT loaded. Returns an empty
// (non-nil) slice when none exist.
func (r *PostgresGLRepository) ListJournalEntriesByOrg(ctx context.Context, orgID uuid.UUID) ([]GLJournalEntry, error) {
	const q = `
		SELECT id, org_id, entry_number, entry_date, memo, source_type,
		       source_id, unit_id, posted_by, reversed_by, is_reversal, created_at
		FROM gl_journal_entries
		WHERE org_id = $1
		ORDER BY entry_number DESC`

	rows, err := r.db.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("fin: ListJournalEntriesByOrg: %w", err)
	}
	defer rows.Close()

	return collectGLJournalEntries(rows, "ListJournalEntriesByOrg")
}

// ─── Reporting ───────────────────────────────────────────────────────────────

// GetTrialBalance returns debit and credit totals for every account with
// activity on or before asOfDate. Returns an empty (non-nil) slice when no
// data exists.
func (r *PostgresGLRepository) GetTrialBalance(ctx context.Context, orgID uuid.UUID, asOfDate time.Time) ([]TrialBalanceRow, error) {
	const q = `
		SELECT a.id, a.account_number, a.name, a.account_type,
		       COALESCE(SUM(l.debit_cents), 0) AS total_debits,
		       COALESCE(SUM(l.credit_cents), 0) AS total_credits
		FROM gl_accounts a
		JOIN gl_journal_lines l ON l.account_id = a.id
		JOIN gl_journal_entries e ON e.id = l.journal_entry_id
		WHERE a.org_id = $1 AND a.deleted_at IS NULL
		  AND e.entry_date <= $2
		GROUP BY a.id, a.account_number, a.name, a.account_type
		HAVING SUM(l.debit_cents) != 0 OR SUM(l.credit_cents) != 0
		ORDER BY a.account_number`

	rows, err := r.db.Query(ctx, q, orgID, asOfDate)
	if err != nil {
		return nil, fmt.Errorf("fin: GetTrialBalance: %w", err)
	}
	defer rows.Close()

	return collectTrialBalanceRows(rows, "GetTrialBalance")
}

// GetAccountBalances returns the net balance for every account with activity
// in the given date range [from, to]. Returns an empty (non-nil) slice when
// no data exists.
func (r *PostgresGLRepository) GetAccountBalances(ctx context.Context, orgID uuid.UUID, from, to time.Time) ([]AccountBalance, error) {
	const q = `
		SELECT a.id, a.account_number, a.name, a.account_type,
		       COALESCE(SUM(l.debit_cents), 0) - COALESCE(SUM(l.credit_cents), 0) AS balance_cents
		FROM gl_accounts a
		JOIN gl_journal_lines l ON l.account_id = a.id
		JOIN gl_journal_entries e ON e.id = l.journal_entry_id
		WHERE a.org_id = $1 AND a.deleted_at IS NULL
		  AND e.entry_date >= $2 AND e.entry_date <= $3
		GROUP BY a.id, a.account_number, a.name, a.account_type
		HAVING SUM(l.debit_cents) != 0 OR SUM(l.credit_cents) != 0
		ORDER BY a.account_number`

	rows, err := r.db.Query(ctx, q, orgID, from, to)
	if err != nil {
		return nil, fmt.Errorf("fin: GetAccountBalances: %w", err)
	}
	defer rows.Close()

	return collectAccountBalances(rows, "GetAccountBalances")
}

// HasPostedLines returns true if any journal lines reference the given
// account. Used to prevent deletion of accounts with activity.
func (r *PostgresGLRepository) HasPostedLines(ctx context.Context, accountID uuid.UUID) (bool, error) {
	const q = `SELECT EXISTS(SELECT 1 FROM gl_journal_lines WHERE account_id = $1)`

	var exists bool
	err := r.db.QueryRow(ctx, q, accountID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("fin: HasPostedLines: %w", err)
	}
	return exists, nil
}

// ─── Scan helpers ────────────────────────────────────────────────────────────

// scanGLAccount reads a single gl_accounts row.
func scanGLAccount(row pgx.Row) (*GLAccount, error) {
	var a GLAccount
	err := row.Scan(
		&a.ID,
		&a.OrgID,
		&a.ParentID,
		&a.FundID,
		&a.AccountNumber,
		&a.Name,
		&a.AccountType,
		&a.IsHeader,
		&a.IsSystem,
		&a.Description,
		&a.CreatedAt,
		&a.UpdatedAt,
		&a.DeletedAt,
	)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// collectGLAccounts drains pgx.Rows into a slice of GLAccount values.
func collectGLAccounts(rows pgx.Rows, op string) ([]GLAccount, error) {
	accounts := []GLAccount{}
	for rows.Next() {
		var a GLAccount
		if err := rows.Scan(
			&a.ID,
			&a.OrgID,
			&a.ParentID,
			&a.FundID,
			&a.AccountNumber,
			&a.Name,
			&a.AccountType,
			&a.IsHeader,
			&a.IsSystem,
			&a.Description,
			&a.CreatedAt,
			&a.UpdatedAt,
			&a.DeletedAt,
		); err != nil {
			return nil, fmt.Errorf("fin: %s scan: %w", op, err)
		}
		accounts = append(accounts, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("fin: %s rows: %w", op, err)
	}
	return accounts, nil
}

// scanGLJournalEntry reads a single gl_journal_entries row (header only,
// Lines stays nil).
func scanGLJournalEntry(row pgx.Row) (*GLJournalEntry, error) {
	var e GLJournalEntry
	err := row.Scan(
		&e.ID,
		&e.OrgID,
		&e.EntryNumber,
		&e.EntryDate,
		&e.Memo,
		&e.SourceType,
		&e.SourceID,
		&e.UnitID,
		&e.PostedBy,
		&e.ReversedBy,
		&e.IsReversal,
		&e.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &e, nil
}

// collectGLJournalEntries drains pgx.Rows into a slice of GLJournalEntry
// values (header only, Lines stays nil).
func collectGLJournalEntries(rows pgx.Rows, op string) ([]GLJournalEntry, error) {
	entries := []GLJournalEntry{}
	for rows.Next() {
		var e GLJournalEntry
		if err := rows.Scan(
			&e.ID,
			&e.OrgID,
			&e.EntryNumber,
			&e.EntryDate,
			&e.Memo,
			&e.SourceType,
			&e.SourceID,
			&e.UnitID,
			&e.PostedBy,
			&e.ReversedBy,
			&e.IsReversal,
			&e.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("fin: %s scan: %w", op, err)
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("fin: %s rows: %w", op, err)
	}
	return entries, nil
}

// scanGLJournalLine reads a single gl_journal_lines row.
func scanGLJournalLine(row pgx.Row) (*GLJournalLine, error) {
	var l GLJournalLine
	err := row.Scan(
		&l.ID,
		&l.JournalEntryID,
		&l.AccountID,
		&l.DebitCents,
		&l.CreditCents,
		&l.Memo,
	)
	if err != nil {
		return nil, err
	}
	return &l, nil
}

// collectGLJournalLines drains pgx.Rows into a slice of GLJournalLine values.
func collectGLJournalLines(rows pgx.Rows, op string) ([]GLJournalLine, error) {
	lines := []GLJournalLine{}
	for rows.Next() {
		var l GLJournalLine
		if err := rows.Scan(
			&l.ID,
			&l.JournalEntryID,
			&l.AccountID,
			&l.DebitCents,
			&l.CreditCents,
			&l.Memo,
		); err != nil {
			return nil, fmt.Errorf("fin: %s scan: %w", op, err)
		}
		lines = append(lines, l)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("fin: %s rows: %w", op, err)
	}
	return lines, nil
}

// scanTrialBalanceRow reads a single trial balance result row.
func scanTrialBalanceRow(row pgx.Row) (*TrialBalanceRow, error) {
	var t TrialBalanceRow
	err := row.Scan(
		&t.AccountID,
		&t.AccountNumber,
		&t.AccountName,
		&t.AccountType,
		&t.DebitCents,
		&t.CreditCents,
	)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// collectTrialBalanceRows drains pgx.Rows into a slice of TrialBalanceRow
// values.
func collectTrialBalanceRows(rows pgx.Rows, op string) ([]TrialBalanceRow, error) {
	result := []TrialBalanceRow{}
	for rows.Next() {
		var t TrialBalanceRow
		if err := rows.Scan(
			&t.AccountID,
			&t.AccountNumber,
			&t.AccountName,
			&t.AccountType,
			&t.DebitCents,
			&t.CreditCents,
		); err != nil {
			return nil, fmt.Errorf("fin: %s scan: %w", op, err)
		}
		result = append(result, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("fin: %s rows: %w", op, err)
	}
	return result, nil
}

// scanAccountBalance reads a single account balance result row.
func scanAccountBalance(row pgx.Row) (*AccountBalance, error) {
	var b AccountBalance
	err := row.Scan(
		&b.AccountID,
		&b.AccountNumber,
		&b.AccountName,
		&b.AccountType,
		&b.BalanceCents,
	)
	if err != nil {
		return nil, err
	}
	return &b, nil
}

// collectAccountBalances drains pgx.Rows into a slice of AccountBalance
// values.
func collectAccountBalances(rows pgx.Rows, op string) ([]AccountBalance, error) {
	balances := []AccountBalance{}
	for rows.Next() {
		var b AccountBalance
		if err := rows.Scan(
			&b.AccountID,
			&b.AccountNumber,
			&b.AccountName,
			&b.AccountType,
			&b.BalanceCents,
		); err != nil {
			return nil, fmt.Errorf("fin: %s scan: %w", op, err)
		}
		balances = append(balances, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("fin: %s rows: %w", op, err)
	}
	return balances, nil
}
