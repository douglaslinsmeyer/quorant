# Atomic Financial Operations — Design Spec

**Issue:** #60 — P0: Financial operations not atomic  
**Date:** 2026-04-09  
**Status:** Draft

## Problem

`CreateAssessment`, `RecordPayment`, and `CreateFundTransfer` each perform 2–3 independent database operations. If any intermediate call fails, the earlier writes persist, creating orphaned records:

- Assessment without ledger entry or GL journal entry
- Payment without credit ledger entry or GL journal entry
- Fund transfer without GL journal entry

This also means GL failures are silently swallowed (issue #66), since the assessment/payment is returned as successful even when the GL post fails.

## Solution: Unit of Work Pattern

Introduce a `UnitOfWork` that wraps a `pgx.Tx` and is managed by the service layer. Repository methods run against the shared transaction. All writes in a multi-step financial operation either commit together or roll back together.

**This is the canonical pattern for any future multi-repository atomic operation in the codebase.**

## Design

### DBTX Interface

A small interface in `platform/db/` that both `*pgxpool.Pool` and `pgx.Tx` satisfy:

```go
// platform/db/dbtx.go
package db

import (
    "context"
    "github.com/jackc/pgx/v5"
    "github.com/jackc/pgx/v5/pgconn"
)

type DBTX interface {
    Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
    Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
    QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
    Begin(ctx context.Context) (pgx.Tx, error)
}
```

`Begin` is included because repos that use advisory locks (`CreateLedgerEntry`, `PostJournalEntry`, `CreateTransaction`) call `Begin` internally. When the underlying `DBTX` is a pool, this starts a real transaction. When it's already a `pgx.Tx`, pgx automatically creates a savepoint — advisory locks work correctly either way.

### UnitOfWork and Factory

```go
// platform/db/unitofwork.go
package db

import (
    "context"
    "github.com/jackc/pgx/v5"
    "github.com/jackc/pgx/v5/pgxpool"
)

type UnitOfWork struct {
    tx pgx.Tx
}

func (u *UnitOfWork) Tx() pgx.Tx                    { return u.tx }
func (u *UnitOfWork) Commit(ctx context.Context) error  { return u.tx.Commit(ctx) }
func (u *UnitOfWork) Rollback(ctx context.Context) error { return u.tx.Rollback(ctx) }

type UnitOfWorkFactory struct {
    pool *pgxpool.Pool
}

func NewUnitOfWorkFactory(pool *pgxpool.Pool) *UnitOfWorkFactory {
    return &UnitOfWorkFactory{pool: pool}
}

func (f *UnitOfWorkFactory) Begin(ctx context.Context) (*UnitOfWork, error) {
    tx, err := f.pool.Begin(ctx)
    if err != nil {
        return nil, err
    }
    return &UnitOfWork{tx: tx}, nil
}
```

### Repository Changes

Each Postgres repository (`PostgresAssessmentRepository`, `PostgresGLRepository`, `PostgresFundRepository`, `PostgresPaymentRepository`) changes:

1. **Internal field**: `pool *pgxpool.Pool` → `db DBTX`
2. **Constructor**: Still accepts `*pgxpool.Pool`, stores it as `DBTX`
3. **New method**: `WithTx(tx pgx.Tx) *PostgresXxxRepository` — returns a shallow copy with `db` set to the tx
4. **Internal Begin calls**: `r.pool.Begin(ctx)` → `r.db.Begin(ctx)` — when `db` is a pool this starts a real tx; when it's a tx this creates a savepoint

The `WithTx` method is on the concrete type only. Repository interfaces do not change. Existing callers that don't need transactions continue to work unchanged.

### Service Layer Changes

`FinService` gains a `uowFactory *db.UnitOfWorkFactory` field, injected at construction.

The three multi-write methods are refactored to use UoW:

#### CreateAssessment

```go
uow, err := s.uowFactory.Begin(ctx)
if err != nil { return nil, err }
defer uow.Rollback(ctx)

assessments := s.assessments.WithTx(uow.Tx())
gl := s.gl.WithTx(uow.Tx())

// 1. Insert assessment
assessment, err := assessments.CreateAssessment(ctx, a)
if err != nil { return nil, err }

// 2. Insert ledger entry (advisory lock via savepoint)
_, err = assessments.CreateLedgerEntry(ctx, entry)
if err != nil { return nil, err }

// 3. Post GL journal entry (advisory lock via savepoint)
_, err = gl.PostJournalEntry(ctx, je)
if err != nil { return nil, err }

// 4. Publish domain event transactionally
err = s.outbox.PublishTx(ctx, uow.Tx(), event)
if err != nil { return nil, err }

if err := uow.Commit(ctx); err != nil { return nil, err }
return assessment, nil
```

#### RecordPayment

Same pattern: payment insert + credit ledger entry + GL journal entry + domain event, all within a single UoW.

#### CreateFundTransfer

Same pattern: transfer insert + GL journal entry (4 lines) + domain event, all within a single UoW.

**Error handling**: Any error triggers the deferred `Rollback` — nothing persists. This directly resolves issue #66 (GL failure silently swallowed) as a side effect.

### Outbox Event Publishing

The existing `OutboxPublisher.PublishTx(ctx, tx, event)` method joins the UoW transaction. Event rows land in `domain_events` atomically with the business data. The existing outbox poller picks them up and forwards to NATS JetStream — no changes needed.

Events published:

| Method | Subject |
|--------|---------|
| CreateAssessment | `quorant.assessment.created.{org_id}` |
| RecordPayment | `quorant.payment.created.{org_id}` |
| CreateFundTransfer | `quorant.fund_transfer.created.{org_id}` |

### Wiring

`cmd/quorant-api/main.go` and `cmd/quorant-worker/main.go` (if it constructs `FinService`) create a `UnitOfWorkFactory` from the pool and pass it to `FinService`.

## Testing Strategy

### Unit Tests

Existing service tests use stub repositories injected via interfaces. The stubs handle calls directly — `WithTx` is on the concrete Postgres type, not the interface — so the UoW path is never exercised in unit tests. The `uowFactory` field is set to nil in unit tests. Service test constructor calls are updated to accept the new field.

### Integration Tests

New integration tests for each of the three methods that verify atomicity:

1. **Happy path**: All three operations succeed, verify assessment + ledger + GL all exist
2. **Rollback on failure**: Force a failure at the GL step (e.g., invalid account), verify no assessment or ledger entry was persisted
3. **Savepoint behavior**: Verify that advisory locks in `CreateLedgerEntry` and `PostJournalEntry` work correctly when running inside a UoW (savepoints, not nested transactions)

### Repository Tests

Existing repo tests continue to work (pool satisfies `DBTX`). Add tests exercising the `WithTx` path.

## Files Changed

| File | Change |
|------|--------|
| `platform/db/dbtx.go` | New — `DBTX` interface |
| `platform/db/unitofwork.go` | New — `UnitOfWork` + `UnitOfWorkFactory` |
| `fin/assessment_postgres.go` | `pool` → `db DBTX`, add `WithTx`, update `Begin` calls |
| `fin/gl_postgres.go` | Same |
| `fin/fund_postgres.go` | Same |
| `fin/payment_postgres.go` | Same |
| `fin/service.go` | Add `uowFactory` field, refactor 3 methods |
| `fin/service_test.go` | Update constructor calls |
| `cmd/quorant-api/main.go` | Wire `UnitOfWorkFactory` |
| `cmd/quorant-worker/main.go` | Wire `UnitOfWorkFactory` (if applicable) |

## Side Effects

- **Issue #66 resolved**: GL failure now fails the entire transaction instead of being silently logged.
- **Transactional events**: Domain events are published atomically with business data, enabling reliable downstream processing.
