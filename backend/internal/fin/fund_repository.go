package fin

import (
	"context"

	"github.com/google/uuid"
)

// FundRepository persists and retrieves funds, fund transactions, and fund
// transfers for the Finance module.
type FundRepository interface {
	// ── Funds ─────────────────────────────────────────────────────────────────

	// CreateFund inserts a new fund and returns the fully-populated row
	// (including generated id and timestamps).
	CreateFund(ctx context.Context, f *Fund) (*Fund, error)

	// FindFundByID returns the fund with the given id, or nil, nil if no
	// matching (non-deleted) row exists.
	FindFundByID(ctx context.Context, id uuid.UUID) (*Fund, error)

	// ListFundsByOrg returns all non-deleted funds for the given org, ordered
	// by created_at. Returns an empty (non-nil) slice when none exist.
	ListFundsByOrg(ctx context.Context, orgID uuid.UUID) ([]Fund, error)

	// UpdateFund persists changes to an existing fund and returns the updated
	// row.
	UpdateFund(ctx context.Context, f *Fund) (*Fund, error)

	// ── Fund Transactions ─────────────────────────────────────────────────────

	// CreateTransaction inserts a new fund transaction and atomically updates
	// the parent fund's balance_cents (denormalized). Returns the
	// fully-populated transaction row.
	CreateTransaction(ctx context.Context, t *FundTransaction) (*FundTransaction, error)

	// ListTransactionsByFund returns all transactions for the given fund,
	// ordered by effective_date DESC. Returns an empty (non-nil) slice when
	// none exist.
	ListTransactionsByFund(ctx context.Context, fundID uuid.UUID) ([]FundTransaction, error)

	// ── Fund Transfers ────────────────────────────────────────────────────────

	// CreateTransfer inserts a new fund transfer record and returns the
	// fully-populated row.
	CreateTransfer(ctx context.Context, t *FundTransfer) (*FundTransfer, error)

	// ListTransfersByOrg returns all transfers for the given org, ordered by
	// effective_date DESC. Returns an empty (non-nil) slice when none exist.
	ListTransfersByOrg(ctx context.Context, orgID uuid.UUID) ([]FundTransfer, error)
}
