package fin

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	dbpkg "github.com/quorant/quorant/internal/platform/db"
)

// PostgresFundRepository implements FundRepository using a DBTX.
type PostgresFundRepository struct {
	db dbpkg.DBTX
}

// NewPostgresFundRepository creates a new PostgresFundRepository backed by
// pool.
func NewPostgresFundRepository(pool *pgxpool.Pool) *PostgresFundRepository {
	return &PostgresFundRepository{db: pool}
}

// WithTx returns a new PostgresFundRepository scoped to the given transaction,
// enabling participation in a caller-managed transaction.
func (r *PostgresFundRepository) WithTx(tx pgx.Tx) FundRepository {
	return &PostgresFundRepository{db: tx}
}

// ─── Funds ────────────────────────────────────────────────────────────────────

// CreateFund inserts a new fund and returns the fully-populated row.
func (r *PostgresFundRepository) CreateFund(ctx context.Context, f *Fund) (*Fund, error) {
	const q = `
		INSERT INTO funds (
			org_id, currency_code, name, fund_type, balance_cents, target_balance_cents, is_default
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7
		)
		RETURNING id, org_id, currency_code, name, fund_type, balance_cents, target_balance_cents,
		          is_default, created_at, updated_at, deleted_at`

	row := r.db.QueryRow(ctx, q,
		f.OrgID,
		f.CurrencyCode,
		f.Name,
		f.FundType,
		f.BalanceCents,
		f.TargetBalanceCents,
		f.IsDefault,
	)

	result, err := scanFund(row)
	if err != nil {
		return nil, fmt.Errorf("fin: CreateFund: %w", err)
	}
	return result, nil
}

// FindFundByID returns the fund with the given id, or nil, nil if not found or
// soft-deleted.
func (r *PostgresFundRepository) FindFundByID(ctx context.Context, id uuid.UUID) (*Fund, error) {
	const q = `
		SELECT id, org_id, currency_code, name, fund_type, balance_cents, target_balance_cents,
		       is_default, created_at, updated_at, deleted_at
		FROM funds
		WHERE id = $1 AND deleted_at IS NULL`

	row := r.db.QueryRow(ctx, q, id)
	result, err := scanFund(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("fin: FindFundByID: %w", err)
	}
	return result, nil
}

// ListFundsByOrg returns all non-deleted funds for the given org ordered by
// created_at. Returns an empty (non-nil) slice when none exist.
func (r *PostgresFundRepository) ListFundsByOrg(ctx context.Context, orgID uuid.UUID) ([]Fund, error) {
	const q = `
		SELECT id, org_id, currency_code, name, fund_type, balance_cents, target_balance_cents,
		       is_default, created_at, updated_at, deleted_at
		FROM funds
		WHERE org_id = $1 AND deleted_at IS NULL
		ORDER BY created_at`

	rows, err := r.db.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("fin: ListFundsByOrg: %w", err)
	}
	defer rows.Close()

	return collectFunds(rows, "ListFundsByOrg")
}

// UpdateFund persists changes to an existing fund and returns the updated row.
func (r *PostgresFundRepository) UpdateFund(ctx context.Context, f *Fund) (*Fund, error) {
	const q = `
		UPDATE funds SET
			currency_code        = $1,
			name                 = $2,
			fund_type            = $3,
			balance_cents        = $4,
			target_balance_cents = $5,
			is_default           = $6,
			updated_at           = now()
		WHERE id = $7 AND deleted_at IS NULL
		RETURNING id, org_id, currency_code, name, fund_type, balance_cents, target_balance_cents,
		          is_default, created_at, updated_at, deleted_at`

	row := r.db.QueryRow(ctx, q,
		f.CurrencyCode,
		f.Name,
		f.FundType,
		f.BalanceCents,
		f.TargetBalanceCents,
		f.IsDefault,
		f.ID,
	)

	result, err := scanFund(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("fin: UpdateFund: fund %s not found or already deleted", f.ID)
	}
	if err != nil {
		return nil, fmt.Errorf("fin: UpdateFund: %w", err)
	}
	return result, nil
}

// ─── Fund Transactions ────────────────────────────────────────────────────────

// CreateTransaction inserts a new fund transaction and atomically updates the
// parent fund's balance_cents denormalized field within a single transaction.
func (r *PostgresFundRepository) CreateTransaction(ctx context.Context, t *FundTransaction) (*FundTransaction, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("fin: CreateTransaction begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// Update the fund's balance and capture the new balance in one shot.
	var newBalance int64
	err = tx.QueryRow(ctx, `
		UPDATE funds
		SET balance_cents = balance_cents + $1, updated_at = now()
		WHERE id = $2 AND deleted_at IS NULL
		RETURNING balance_cents`,
		t.AmountCents,
		t.FundID,
	).Scan(&newBalance)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("fin: CreateTransaction: fund %s not found or deleted", t.FundID)
	}
	if err != nil {
		return nil, fmt.Errorf("fin: CreateTransaction update balance: %w", err)
	}

	const q = `
		INSERT INTO fund_transactions (
			fund_id, org_id, currency_code, transaction_type, amount_cents, balance_after_cents,
			description, reference_type, reference_id, effective_date
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10
		)
		RETURNING id, fund_id, org_id, currency_code, transaction_type, amount_cents,
		          balance_after_cents, description, reference_type, reference_id,
		          effective_date, created_at`

	row := tx.QueryRow(ctx, q,
		t.FundID,
		t.OrgID,
		t.CurrencyCode,
		t.TransactionType,
		t.AmountCents,
		newBalance,
		t.Description,
		t.ReferenceType,
		t.ReferenceID,
		t.EffectiveDate,
	)

	result, err := scanFundTransaction(row)
	if err != nil {
		return nil, fmt.Errorf("fin: CreateTransaction insert: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("fin: CreateTransaction commit: %w", err)
	}
	return result, nil
}

// ListTransactionsByFund returns all transactions for the given fund ordered by
// effective_date DESC. Returns an empty (non-nil) slice when none exist.
func (r *PostgresFundRepository) ListTransactionsByFund(ctx context.Context, fundID uuid.UUID) ([]FundTransaction, error) {
	const q = `
		SELECT id, fund_id, org_id, currency_code, transaction_type, amount_cents,
		       balance_after_cents, description, reference_type, reference_id,
		       effective_date, created_at
		FROM fund_transactions
		WHERE fund_id = $1
		ORDER BY effective_date DESC, created_at DESC`

	rows, err := r.db.Query(ctx, q, fundID)
	if err != nil {
		return nil, fmt.Errorf("fin: ListTransactionsByFund: %w", err)
	}
	defer rows.Close()

	return collectFundTransactions(rows, "ListTransactionsByFund")
}

// ─── Fund Transfers ───────────────────────────────────────────────────────────

// CreateTransfer inserts a new fund transfer record and returns the
// fully-populated row.
func (r *PostgresFundRepository) CreateTransfer(ctx context.Context, t *FundTransfer) (*FundTransfer, error) {
	const q = `
		INSERT INTO fund_transfers (
			org_id, currency_code, from_fund_id, to_fund_id, amount_cents,
			description, approved_by, approved_at, effective_date
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9
		)
		RETURNING id, org_id, currency_code, from_fund_id, to_fund_id, amount_cents,
		          description, approved_by, approved_at, effective_date, created_at`

	row := r.db.QueryRow(ctx, q,
		t.OrgID,
		t.CurrencyCode,
		t.FromFundID,
		t.ToFundID,
		t.AmountCents,
		t.Description,
		t.ApprovedBy,
		t.ApprovedAt,
		t.EffectiveDate,
	)

	result, err := scanFundTransfer(row)
	if err != nil {
		return nil, fmt.Errorf("fin: CreateTransfer: %w", err)
	}
	return result, nil
}

// ListTransfersByOrg returns all transfers for the given org ordered by
// effective_date DESC. Returns an empty (non-nil) slice when none exist.
func (r *PostgresFundRepository) ListTransfersByOrg(ctx context.Context, orgID uuid.UUID) ([]FundTransfer, error) {
	const q = `
		SELECT id, org_id, currency_code, from_fund_id, to_fund_id, amount_cents,
		       description, approved_by, approved_at, effective_date, created_at
		FROM fund_transfers
		WHERE org_id = $1
		ORDER BY effective_date DESC, created_at DESC`

	rows, err := r.db.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("fin: ListTransfersByOrg: %w", err)
	}
	defer rows.Close()

	return collectFundTransfers(rows, "ListTransfersByOrg")
}

// ─── Scan helpers ─────────────────────────────────────────────────────────────

// scanFund reads a single funds row.
func scanFund(row pgx.Row) (*Fund, error) {
	var f Fund
	err := row.Scan(
		&f.ID,
		&f.OrgID,
		&f.CurrencyCode,
		&f.Name,
		&f.FundType,
		&f.BalanceCents,
		&f.TargetBalanceCents,
		&f.IsDefault,
		&f.CreatedAt,
		&f.UpdatedAt,
		&f.DeletedAt,
	)
	if err != nil {
		return nil, err
	}
	return &f, nil
}

// collectFunds drains pgx.Rows into a slice of Fund values.
func collectFunds(rows pgx.Rows, op string) ([]Fund, error) {
	funds := []Fund{}
	for rows.Next() {
		var f Fund
		if err := rows.Scan(
			&f.ID,
			&f.OrgID,
			&f.CurrencyCode,
			&f.Name,
			&f.FundType,
			&f.BalanceCents,
			&f.TargetBalanceCents,
			&f.IsDefault,
			&f.CreatedAt,
			&f.UpdatedAt,
			&f.DeletedAt,
		); err != nil {
			return nil, fmt.Errorf("fin: %s scan: %w", op, err)
		}
		funds = append(funds, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("fin: %s rows: %w", op, err)
	}
	return funds, nil
}

// scanFundTransaction reads a single fund_transactions row.
func scanFundTransaction(row pgx.Row) (*FundTransaction, error) {
	var t FundTransaction
	err := row.Scan(
		&t.ID,
		&t.FundID,
		&t.OrgID,
		&t.CurrencyCode,
		&t.TransactionType,
		&t.AmountCents,
		&t.BalanceAfterCents,
		&t.Description,
		&t.ReferenceType,
		&t.ReferenceID,
		&t.EffectiveDate,
		&t.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// collectFundTransactions drains pgx.Rows into a slice of FundTransaction
// values.
func collectFundTransactions(rows pgx.Rows, op string) ([]FundTransaction, error) {
	txns := []FundTransaction{}
	for rows.Next() {
		var t FundTransaction
		if err := rows.Scan(
			&t.ID,
			&t.FundID,
			&t.OrgID,
			&t.CurrencyCode,
			&t.TransactionType,
			&t.AmountCents,
			&t.BalanceAfterCents,
			&t.Description,
			&t.ReferenceType,
			&t.ReferenceID,
			&t.EffectiveDate,
			&t.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("fin: %s scan: %w", op, err)
		}
		txns = append(txns, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("fin: %s rows: %w", op, err)
	}
	return txns, nil
}

// scanFundTransfer reads a single fund_transfers row.
func scanFundTransfer(row pgx.Row) (*FundTransfer, error) {
	var t FundTransfer
	err := row.Scan(
		&t.ID,
		&t.OrgID,
		&t.CurrencyCode,
		&t.FromFundID,
		&t.ToFundID,
		&t.AmountCents,
		&t.Description,
		&t.ApprovedBy,
		&t.ApprovedAt,
		&t.EffectiveDate,
		&t.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// collectFundTransfers drains pgx.Rows into a slice of FundTransfer values.
func collectFundTransfers(rows pgx.Rows, op string) ([]FundTransfer, error) {
	transfers := []FundTransfer{}
	for rows.Next() {
		var t FundTransfer
		if err := rows.Scan(
			&t.ID,
			&t.OrgID,
			&t.CurrencyCode,
			&t.FromFundID,
			&t.ToFundID,
			&t.AmountCents,
			&t.Description,
			&t.ApprovedBy,
			&t.ApprovedAt,
			&t.EffectiveDate,
			&t.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("fin: %s scan: %w", op, err)
		}
		transfers = append(transfers, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("fin: %s rows: %w", op, err)
	}
	return transfers, nil
}
