# Phase 2: Database Schema & IAM Module Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Core database schema (enums, IAM, org foundation, memberships), Zitadel JWT authentication, RBAC permission resolution, and the `/api/v1/auth/me` endpoint.

**Architecture:** SQL migrations create all enum types and foundational tables (users, roles, permissions, organizations, memberships). Go packages handle JWT validation via Zitadel JWKS, RBAC permission resolution with ltree inheritance, and user profile serving. Seed data populates the 11 system roles and ~80 permissions from the architecture doc's permission matrix.

**Tech Stack:** PostgreSQL 16, Atlas migrations, pgx/v5, JWKS validation (lestrrat-go/jwx), Go 1.23+ http.ServeMux

---

## File Structure

```
backend/
  migrations/
    20260409000002_enums.sql              # All PostgreSQL enum types
    20260409000003_iam_tables.sql         # users, roles, permissions, role_permissions
    20260409000004_org_foundation.sql     # organizations, organizations_management, memberships
    20260409000005_seed_roles.sql         # System roles + permissions + role_permissions
  internal/
    iam/
      domain.go                           # User, Role, Permission domain types
      domain_test.go
      repository.go                       # UserRepository interface
      postgres.go                         # PostgreSQL implementation
      postgres_test.go                    # Integration tests
      service.go                          # UserService (lookup, sync)
      service_test.go
      handler.go                          # HTTP handlers (auth/me, webhook)
      handler_test.go
      routes.go                           # Route registration
    platform/
      middleware/
        auth.go                           # JWT validation middleware
        auth_test.go
        rbac.go                           # RBAC permission check middleware
        rbac_test.go
        tenant.go                         # Org context + RLS session var middleware
        tenant_test.go
      auth/
        jwks.go                           # JWKS key fetcher + JWT validator
        jwks_test.go
        claims.go                         # JWT claims parsing + context helpers
        claims_test.go
```

---

### Task 1: Enum Types Migration

**Files:**
- Create: `backend/migrations/20260409000002_enums.sql`

- [ ] Create migration file with all 26 enum types from architecture doc Section 3
- [ ] Run migration against Docker PG: `docker compose exec -T postgres psql -U quorant -d quorant_dev < backend/migrations/20260409000002_enums.sql`
- [ ] Verify: `SELECT typname FROM pg_type WHERE typtype = 'e' ORDER BY typname;`
- [ ] Commit: "feat: add all PostgreSQL enum types"

---

### Task 2: IAM Tables Migration

**Files:**
- Create: `backend/migrations/20260409000003_iam_tables.sql`

- [ ] Create migration with users, roles, permissions, role_permissions tables exactly matching architecture doc Section 3 IAM Tables (including all indexes)
- [ ] Run migration against Docker PG
- [ ] Verify tables exist: `\dt` in psql
- [ ] Commit: "feat: add IAM tables (users, roles, permissions)"

---

### Task 3: Org Foundation Tables Migration

**Files:**
- Create: `backend/migrations/20260409000004_org_foundation.sql`

- [ ] Create migration with organizations, organizations_management, memberships tables exactly matching architecture doc (including GIST index on ltree path, partial unique indexes)
- [ ] Run migration against Docker PG
- [ ] Verify tables and indexes exist
- [ ] Commit: "feat: add org foundation tables (organizations, memberships)"

---

### Task 4: Seed Roles & Permissions

**Files:**
- Create: `backend/migrations/20260409000005_seed_roles.sql`

- [ ] Insert all 11 system roles: platform_admin, platform_support, platform_finance, firm_admin, firm_staff, firm_support, hoa_manager, vendor_contact, board_president, board_member, homeowner
- [ ] Insert all permissions from the architecture doc permission matrix (~80 permissions across org, fin, gov, com, doc, task, audit, admin, license, billing, ai, webhook modules)
- [ ] Insert role_permissions mappings per the permission matrix (which roles get which permissions)
- [ ] Run migration against Docker PG
- [ ] Verify: `SELECT r.name, count(rp.*) FROM roles r LEFT JOIN role_permissions rp ON r.id = rp.role_id GROUP BY r.name ORDER BY r.name;`
- [ ] Commit: "feat: seed system roles and permissions"

---

### Task 5: IAM Domain Types

**Files:**
- Create: `backend/internal/iam/domain.go`
- Create: `backend/internal/iam/domain_test.go`

- [ ] Write test: User struct has correct fields, validates email
- [ ] Define domain types: User (matching users table), Role, Permission, Membership structs
- [ ] Run test → passes
- [ ] Commit: "feat: add IAM domain types"

---

### Task 6: JWT Claims & Context

**Files:**
- Create: `backend/internal/platform/auth/claims.go`
- Create: `backend/internal/platform/auth/claims_test.go`

- [ ] Write tests: Claims struct parses standard JWT fields (sub, email, name), context helpers store/retrieve claims
- [ ] Implement Claims struct with fields: Subject (idp_user_id), Email, Name, Roles (from custom claim if present)
- [ ] Implement context helpers: `WithClaims(ctx, claims) context.Context`, `ClaimsFromContext(ctx) (*Claims, bool)`
- [ ] Run tests → pass
- [ ] Commit: "feat: add JWT claims parsing and context helpers"

---

### Task 7: JWKS Validator

**Files:**
- Create: `backend/internal/platform/auth/jwks.go`
- Create: `backend/internal/platform/auth/jwks_test.go`

- [ ] Write tests: ValidateToken succeeds with valid JWT, fails with expired/invalid JWT (use test RSA key pair)
- [ ] Implement JWKSValidator: fetches keys from Zitadel JWKS endpoint, validates JWT signature (RS256), checks expiry, extracts Claims
- [ ] Use `github.com/lestrrat-go/jwx/v2` for JWKS + JWT handling
- [ ] Run tests → pass
- [ ] Commit: "feat: add JWKS-based JWT validator"

---

### Task 8: Auth Middleware

**Files:**
- Create: `backend/internal/platform/middleware/auth.go`
- Create: `backend/internal/platform/middleware/auth_test.go`

- [ ] Write tests: request with valid Bearer token → claims in context + next handler called; missing/invalid token → 401 JSON error
- [ ] Implement Auth middleware: extracts Bearer token from Authorization header, validates via JWKSValidator, stores Claims in context
- [ ] Run tests → pass
- [ ] Commit: "feat: add JWT authentication middleware"

---

### Task 9: User Repository

**Files:**
- Create: `backend/internal/iam/repository.go`
- Create: `backend/internal/iam/postgres.go`
- Create: `backend/internal/iam/postgres_test.go`

- [ ] Define UserRepository interface: `FindByIDPUserID(ctx, idpUserID) (*User, error)`, `FindByID(ctx, id) (*User, error)`, `Upsert(ctx, user) (*User, error)`, `UpdateLastLogin(ctx, id) error`
- [ ] Write integration tests against real PG (build tag `integration`)
- [ ] Implement PostgresUserRepository with pgxpool
- [ ] Run integration tests → pass
- [ ] Commit: "feat: add user repository with PostgreSQL implementation"

---

### Task 10: User Service

**Files:**
- Create: `backend/internal/iam/service.go`
- Create: `backend/internal/iam/service_test.go`

- [ ] Write tests: GetOrCreateUser finds existing user by idp_user_id; creates new user if not found; updates last_login
- [ ] Implement UserService wrapping UserRepository: `GetOrCreateUser(ctx, Claims) (*User, error)`, `GetCurrentUser(ctx) (*User, error)` (extracts claims from context, looks up user)
- [ ] Run tests → pass
- [ ] Commit: "feat: add user service"

---

### Task 11: RBAC Permission Resolver

**Files:**
- Create: `backend/internal/platform/middleware/rbac.go`
- Create: `backend/internal/platform/middleware/rbac_test.go`

- [ ] Write tests: user with correct permission for org → allowed; user without permission → 403; firm_admin inherits to child orgs via ltree; firm_admin inherits to managed HOAs
- [ ] Implement RBAC middleware: given a user ID and target org ID, resolve permissions by querying memberships → roles → role_permissions. For firm roles, expand upward via ltree to find inherited memberships. For firm→HOA, check organizations_management.
- [ ] Export `RequirePermission(permission string) func(http.Handler) http.Handler` middleware factory
- [ ] Run tests → pass
- [ ] Commit: "feat: add RBAC permission resolution middleware"

---

### Task 12: Tenant Context Middleware

**Files:**
- Create: `backend/internal/platform/middleware/tenant.go`
- Create: `backend/internal/platform/middleware/tenant_test.go`

- [ ] Write tests: extracts org_id from URL path parameter, sets `app.current_user_id` and `app.current_org_id` session vars on DB connection for RLS
- [ ] Implement TenantContext middleware: parses `{org_id}` from request URL, stores in context, provides helper `OrgIDFromContext(ctx) uuid.UUID`
- [ ] Run tests → pass
- [ ] Commit: "feat: add tenant context middleware for RLS"

---

### Task 13: Auth Handlers & Routes

**Files:**
- Create: `backend/internal/iam/handler.go`
- Create: `backend/internal/iam/handler_test.go`
- Create: `backend/internal/iam/routes.go`

- [ ] Write tests: `GET /api/v1/auth/me` returns current user profile; `PATCH /api/v1/auth/me` updates profile fields; `POST /api/v1/webhooks/zitadel` syncs user from webhook payload
- [ ] Implement handlers using UserService
- [ ] Implement `RegisterRoutes(mux, deps)` that registers all IAM routes
- [ ] Run tests → pass
- [ ] Commit: "feat: add auth handlers and IAM routes"

---

### Task 14: Wire IAM into API Server

**Files:**
- Modify: `backend/cmd/quorant-api/main.go`

- [ ] Add JWKSValidator initialization (from Zitadel config)
- [ ] Add Auth middleware to the middleware chain (after RequestID, before routes)
- [ ] Create UserService and IAM handlers
- [ ] Register IAM routes on the mux
- [ ] Health endpoint stays unauthenticated (excluded from auth middleware)
- [ ] Verify: start server, hit `/api/v1/health` (no auth needed), hit `/api/v1/auth/me` without token (401)
- [ ] Commit: "feat: wire IAM module into API server"

---

## Verification

After Phase 2 is complete:
1. `make docker-up` → all services start
2. Run all 4 migrations against Docker PG
3. `make build` → compiles
4. `make test` → all unit tests pass
5. `make test-integration` → integration tests pass
6. `GET /api/v1/health` → 200 (no auth required)
7. `GET /api/v1/auth/me` without token → 401 `UNAUTHENTICATED`
8. `GET /api/v1/auth/me` with valid Zitadel JWT → 200 with user profile
9. Roles and permissions seeded: 11 roles, ~80 permissions with correct mappings
