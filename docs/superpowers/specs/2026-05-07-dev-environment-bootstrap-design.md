# Dev Environment Bootstrap

**Date:** 2026-05-07
**Status:** Draft — pending user review

## Problem

A developer cloning Quorant for the first time cannot get a working environment in one command. The current state requires:

1. `make docker-up` — services start, but no wait for healthy.
2. `make migrate-up` — placeholder target that prints `TODO: atlas migrate apply` and does nothing. Atlas itself is configured (`backend/migrations/atlas.hcl`), but the Make target is unwired.
3. `make seed` — pipes `seed.sql` into the postgres container; races on cold start because step 1 didn't wait. Also non-idempotent: zero `ON CONFLICT` clauses across 18 `INSERT` statements, so a re-run fails on primary-key violations.
4. `.env` setup — `.env.example` exists at the repo root but the Go config loader reads `os.Getenv` directly. Developers must manually `set -a; source .env; set +a` or export each var.
5. `make build` produces two binaries (`quorant-api`, `quorant-worker`) that the developer launches in two terminals, with no live reload.

The result is a brittle five-step ritual with implicit ordering and at least three traps (the placeholder `migrate-up`, the cold-start race, the non-idempotent seed).

## Goal

A developer runs `make dev` and ends up with:

- All compose services up *and* healthy
- Migrations applied
- Seed data loaded
- Both api and worker running with live reload, combined log output, single Ctrl-C teardown

Re-running `make dev` on day two is safe and fast (idempotent everywhere). Existing granular targets (`docker-up`, `migrate-up`, `seed`, `build`, `test`) continue to work for troubleshooting.

## Non-goals

- Live reload for the React/design-system frontend (separate project under `design-system/`)
- Production process management (systemd, k8s manifests)
- Replacing Make with Just/Task/Mage
- Auto-installing atlas/air for the developer — we print install commands and exit on missing prerequisites
- Changing the Go config loader to auto-load `.env` — the dev script handles sourcing

## Architecture

`make dev` is the single entry point. Its pipeline:

```
make dev
  ├─ ensure .env exists (cp .env.example if missing, exit with note)
  ├─ docker compose up -d --wait        # services up AND healthy
  ├─ make migrate-up                    # docker compose run atlas (one-shot)
  ├─ make seed                          # idempotent after this spec lands
  └─ scripts/dev.sh                     # foreground: two `air` processes, prefixed logs, trap teardown
```

Each step is a real, individually-runnable Make target. `make dev` only sequences them.

Two new convenience targets:

- `make dev-down` — stops `scripts/dev.sh` (if backgrounded) and runs `docker compose down`
- `make dev-reset` — `docker compose down -v` to drop volumes, then `make dev`

## Components

### A. Atlas as a compose service

Add to `docker-compose.yml`:

```yaml
atlas:
  image: arigaio/atlas:latest
  profiles: ["tools"]                 # not started by `docker compose up`
  network_mode: service:postgres      # share postgres's network namespace
  volumes:
    - ./backend/migrations:/migrations
  working_dir: /migrations
  depends_on:
    postgres:
      condition: service_healthy
```

`profiles: ["tools"]` means `docker compose up` ignores it; only explicit `docker compose run atlas …` activates it. `network_mode: service:postgres` puts atlas inside postgres's network namespace, so `localhost:5432` resolves to postgres.

Add a `compose` env to `backend/migrations/atlas.hcl`:

```hcl
env "compose" {
  src = "file://."   # CWD is /migrations inside the container
  url = "postgres://quorant:quorant@localhost:5432/quorant_dev?sslmode=disable"
}
```

The existing `local` and `test` envs stay — `local` is what a developer with atlas on PATH uses for `migrate diff` workflows. The `compose` env is what `make migrate-up` invokes through the container.

The `dev` URL is intentionally omitted from the `compose` env. Atlas only needs a dev database for `migrate diff` / `schema apply`; for `migrate apply` (which is all `make migrate-up` does) the dev URL is unused. Keeping it off avoids needing docker-socket access from inside the atlas container.

`make migrate-up` becomes:

```make
migrate-up:
	docker compose run --rm atlas migrate apply --env compose
```

### B. Live reload via `air`

Two configs (one per binary):

- `backend/cmd/quorant-api/.air.toml`
- `backend/cmd/quorant-worker/.air.toml`

Each:
- Working directory: `backend/`
- Watches: `**/*.go`
- Excludes: `bin/`, `_test.go` files, `migrations/`
- Build command: `go build -o bin/quorant-{api,worker} ./cmd/quorant-{api,worker}`
- Run command: `./bin/quorant-{api,worker}`

Air is a developer prerequisite: `go install github.com/air-verse/air@latest`. The dev script checks for it and exits with the install command if missing — same pattern Makefile already uses for `golangci-lint`.

### C. `scripts/dev.sh`

```sh
#!/usr/bin/env bash
set -euo pipefail

# Auto-source .env if present so air-spawned binaries inherit env vars.
if [ -f .env ]; then
  set -a
  . ./.env
  set +a
fi

if ! command -v air >/dev/null; then
  echo "air not found. Install with:"
  echo "  go install github.com/air-verse/air@latest"
  exit 1
fi

cleanup() {
  jobs -p | xargs -r kill 2>/dev/null || true
  wait 2>/dev/null || true
}
trap cleanup EXIT INT TERM

(cd backend && air -c cmd/quorant-api/.air.toml    2>&1 | sed 's/^/[api]    /') &
(cd backend && air -c cmd/quorant-worker/.air.toml 2>&1 | sed 's/^/[worker] /') &
wait
```

`set -a; . .env; set +a` exports every var so air (and the binary it spawns) inherits them. No code change to the Go config loader.

`sed` prefixing keeps the two log streams visually separable. `wait` blocks until both children exit; `trap cleanup EXIT INT TERM` ensures Ctrl-C kills the air processes (and their child binaries via SIGTERM cascade).

### D. `.env` provisioning

A new `make ensure-env` target:

```make
ensure-env:
	@if [ ! -f .env ]; then \
		cp .env.example .env; \
		echo ".env created from .env.example — review and adjust if needed."; \
	fi
```

`make dev` depends on this. Idempotent: only copies on first run.

### E. Seed idempotency

`backend/seeds/seed.sql` currently contains 18 unguarded `INSERT` statements. A re-run fails with primary-key violations.

Fix: append `ON CONFLICT DO NOTHING` (no column target) to every `INSERT` in `seed.sql`. Of the 18 inserts, 10 are against tables with a single-column `id` primary key (`users`, `organizations`, `units`, `vendors`, `plans`) and 8 are against tables with composite primary keys (`memberships`, `organizations_management`, `unit_memberships`, `funds`, `vendor_assignments`, `amenities`, `entitlements`, `org_subscriptions`). The argument-less `ON CONFLICT DO NOTHING` form treats *any* unique-constraint violation as the no-op trigger, so it works uniformly across both shapes without per-table conflict targets. No data semantics change.

This is a small change but blocks `make dev` idempotency, so it lives in the same change set as the bootstrap work.

### F. Make wiring

```make
.PHONY: dev dev-down dev-reset ensure-env

dev: ensure-env
	docker compose up -d --wait
	$(MAKE) migrate-up
	$(MAKE) seed
	./scripts/dev.sh

dev-down:
	docker compose down

dev-reset:
	docker compose down -v
	$(MAKE) dev
```

`docker compose up -d --wait` blocks until every service either passes its healthcheck or exits non-zero. The five existing services (postgres, redis, nats, minio, zitadel + zitadel-db) all already define healthchecks except zitadel itself (which uses a distroless image without curl/wget). Zitadel will need a workaround — see Failure Handling.

## Failure handling & idempotency

| Failure mode | Behavior |
|---|---|
| Compose service fails healthcheck | `docker compose up --wait` exits non-zero. `make dev` halts. Developer runs `docker compose logs <svc>`. |
| Atlas migrate fails | `migrate apply` exits non-zero. `make dev` halts. Atlas tracks state in `atlas_schema_revisions`, so a fixed re-run resumes from the last applied version. |
| Seed runs on already-seeded DB | `ON CONFLICT DO NOTHING` makes it a no-op. |
| Air encounters Go compile error | Air prints the error and waits for the next save — no manual restart. The other binary keeps running. |
| Developer Ctrl-Cs `make dev` | `trap` in `scripts/dev.sh` kills both air processes. Compose services keep running (matches `docker-up` semantics). `make dev-down` stops them when desired. |
| Re-running `make dev` on a healthy stack | All steps are idempotent: compose `up` is no-op for healthy services, atlas migrate is a no-op when current, seed is a no-op after the `ON CONFLICT` fix, dev.sh just relaunches air. |

### Zitadel healthcheck caveat

The existing `docker-compose.yml` has a comment noting Zitadel's distroless image lacks `wget`/`curl`, so its healthcheck is absent. `docker compose up --wait` waits only on services that *have* healthchecks, so Zitadel won't block — but it also won't be guaranteed ready when migrations run.

Zitadel isn't a runtime dependency of migrations or seed (those only touch the app Postgres), so this is acceptable for `make dev`. The api binary, when it starts, will retry Zitadel JWKS fetch — air handles transient failures by restart-on-save anyway.

If we later want a stricter gate, the workaround is a healthcheck shelling out via `docker exec` to a TCP probe, but that's out of scope here.

## Toolchain prerequisites

After this lands, a developer needs on their host:

- Docker + Docker Compose v2 (already required)
- Go (already required)
- `air` — `go install github.com/air-verse/air@latest`

Atlas is no longer a host prerequisite; it runs in a container. `golangci-lint` remains optional (only needed for `make lint`).

The README/onboarding section should be updated to reflect this reduced list, but that's documentation work outside the spec.

## Files touched

```
docker-compose.yml                              # add atlas service with profiles: ["tools"]
backend/migrations/atlas.hcl                    # add `compose` env
backend/seeds/seed.sql                          # append ON CONFLICT (id) DO NOTHING to 18 INSERTs
backend/cmd/quorant-api/.air.toml               # new
backend/cmd/quorant-worker/.air.toml            # new
scripts/dev.sh                                  # new, executable
Makefile                                        # rewire migrate-up; add dev, dev-down, dev-reset, ensure-env
```

No Go source changes.

## Testing

- `make dev-reset` from a clean checkout produces a working stack and live-reloading binaries.
- `make dev` re-run on a healthy stack is a no-op until air takes over.
- `touch backend/internal/fin/service.go` triggers a rebuild and restart of the api binary; worker continues running.
- `Ctrl-C` in `make dev` returns the terminal cleanly; `docker compose ps` shows services still up; `make dev-down` brings them down.
- `make migrate-up` standalone (against an already-up compose stack) applies any pending migrations.
- `make seed` standalone re-run after first seed succeeds with zero rows inserted.
