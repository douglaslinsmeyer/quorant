// backend/internal/platform/db/dbtx.go
package db

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// DBTX is an interface satisfied by both *pgxpool.Pool and pgx.Tx.
// Repositories use this as their backing connection so they can run against
// either the pool (default) or an active transaction (via UnitOfWork).
//
// Begin is included because some repository methods (e.g., CreateLedgerEntry,
// PostJournalEntry) acquire advisory locks via their own internal transaction.
// When DBTX is a pool, Begin starts a real transaction. When DBTX is already a
// pgx.Tx, pgx automatically creates a savepoint — advisory locks work either way.
type DBTX interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Begin(ctx context.Context) (pgx.Tx, error)
}
