// backend/internal/platform/db/unitofwork.go
package db

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// UnitOfWork wraps a database transaction, providing a single commit/rollback
// boundary for multi-repository operations. This is the canonical pattern for
// any operation that must write to multiple tables atomically.
//
// Usage:
//
//	uow, err := factory.Begin(ctx)
//	if err != nil { return err }
//	defer uow.Rollback(ctx)
//
//	repoA := concreteRepoA.WithTx(uow.Tx())
//	repoB := concreteRepoB.WithTx(uow.Tx())
//	// ... do work with repoA and repoB ...
//
//	return uow.Commit(ctx)
type UnitOfWork struct {
	tx pgx.Tx
}

// Tx returns the underlying transaction for passing to repository WithTx methods.
func (u *UnitOfWork) Tx() pgx.Tx { return u.tx }

// Commit commits the transaction. After Commit, the UnitOfWork is no longer usable.
func (u *UnitOfWork) Commit(ctx context.Context) error { return u.tx.Commit(ctx) }

// Rollback aborts the transaction. Safe to call after Commit (pgx returns nil).
func (u *UnitOfWork) Rollback(ctx context.Context) error { return u.tx.Rollback(ctx) }

// UnitOfWorkFactory creates UnitOfWork instances from a connection pool.
type UnitOfWorkFactory struct {
	pool *pgxpool.Pool
}

// NewUnitOfWorkFactory creates a factory backed by the given pool.
func NewUnitOfWorkFactory(pool *pgxpool.Pool) *UnitOfWorkFactory {
	return &UnitOfWorkFactory{pool: pool}
}

// Begin starts a new database transaction and returns a UnitOfWork wrapping it.
func (f *UnitOfWorkFactory) Begin(ctx context.Context) (*UnitOfWork, error) {
	tx, err := f.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &UnitOfWork{tx: tx}, nil
}
