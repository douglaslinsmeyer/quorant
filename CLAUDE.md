# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Development Commands

All commands use the root `Makefile`. Go source is under `backend/`.

```bash
make build              # compile quorant-api and quorant-worker to backend/bin/
make test               # unit tests (short mode, no Docker required)
make test-integration   # integration tests (requires Docker services running)
make lint               # golangci-lint (errcheck, govet, staticcheck, unused, gosimple, ineffassign, typecheck)
make docker-up          # start Postgres, Redis, NATS, MinIO, Zitadel
make docker-down        # stop Docker services
make seed               # load seed data into Postgres
make clean              # remove compiled binaries
```

Run a single test:
```bash
cd backend && go test ./internal/fin/... -run TestCreateAssessment -short -count=1
```

Run a single integration test:
```bash
cd backend && go test ./internal/fin/... -run TestCreateAssessment -count=1 -tags=integration
```

## Architecture

**Go modular monolith** with two binaries:

- `cmd/quorant-api` — HTTP API server (REST/JSON, standard library `net/http` mux)
- `cmd/quorant-worker` — event consumer (NATS JetStream) + scheduled job runner

### Domain Modules (`backend/internal/`)

Each module follows the same file convention:

| File | Purpose |
|------|---------|
| `domain.go` | Data structures (JSON-serializable) |
| `service.go` | Business logic |
| `handler.go` | HTTP handlers |
| `routes.go` | Route registration |
| `{entity}_postgres.go` | Repository implementation |

**Tenant modules:** `iam`, `org`, `fin`, `gov`, `com`, `doc`, `task`
**Platform modules:** `audit`, `admin`, `license`, `billing`, `webhook`
**AI layer:** `ai` (LLM integration, embeddings, policy resolution)

### Platform Packages (`backend/internal/platform/`)

Shared infrastructure — not domain-specific:

- `api` — JSON response envelope (`Response{Data, Meta, Errors}`), error types, cursor pagination
- `auth` — JWT/JWKS validation (Zitadel)
- `config` — env-var based configuration
- `db` — pgx pool initialization
- `middleware` — auth, RBAC, tenant, logging, metrics, tracing, CORS, rate limiting, recovery, request ID
- `queue` — transactional outbox pattern: events written to `domain_events` table in the same DB transaction, polled and published to NATS JetStream asynchronously
- `scheduler` — cron-like job scheduler for the worker binary
- `storage` — S3/MinIO client
- `testutil` — shared test helpers
- `ai` — LLM client abstraction

### Key Patterns

**Error handling:** All API errors implement `api.APIError` interface. Error messages use i18n message keys (e.g., `"fin.assessment.not_found"`) with interpolation params — never hardcoded user-facing strings. Use the typed constructors: `api.NewValidationError(messageKey, field, params...)`, `api.NewNotFoundError(messageKey, params...)`, etc.

**Event-driven:** Business operations publish domain events via `OutboxPublisher.PublishTx(ctx, tx, event)` inside the same DB transaction. The worker's outbox poller picks up unpublished events and forwards them to NATS JetStream. NATS subject format: `quorant.{aggregate_type}.{event_type}.{org_id}`.

**Multi-tenancy:** Org-scoped via middleware. Context carries `user_id` and `org_id` injected by auth/tenant middleware. RLS and ltree-based hierarchy for permission inheritance.

**RBAC:** Permissions checked via middleware before handlers execute. Permission checker queries Postgres. 11 roles spanning firm staff, board members, homeowners, and vendor contacts.

**Repository pattern:** Interface-driven. Each domain defines repository interfaces; Postgres implementations are in `{entity}_postgres.go`. This keeps business logic testable with stubs.

## Testing Conventions

- Unit tests: `_test.go` files alongside source, run with `-short` flag
- Integration tests: use `//go:build integration` build tag, require Docker services
- Test helpers in `internal/platform/testutil`:
  - `DiscardLogger()` — no-op slog logger
  - `NoopAuditor()` — in-memory audit stub
  - `InMemoryPublisher()` — captures published events for assertions
  - `TestUserID()`, `TestOrgID()` — deterministic UUIDs
  - `AuthContext()`, `AuthContextWithOrg()` — pre-populated request contexts
  - `IntegrationDB()` — real Postgres connection (skips test if unavailable)
- Prefer real implementations over mocks; only mock external boundaries
- Use `testify` for assertions

## Infrastructure

**Docker Compose services:** PostgreSQL 16 (pgvector), Redis 7, NATS 2.10 (JetStream), MinIO (S3), Zitadel (OIDC on port 8085)

**Database:** PostgreSQL with extensions: `pgvector` (vector search), `ltree` (hierarchical org paths). Migrations in `backend/migrations/` (Atlas-managed SQL files).

**Design document:** `docs/ARCHITECTURE.md` is the single source of truth for system design decisions. Consult it for domain model details, schema design, RBAC model, and API conventions.
