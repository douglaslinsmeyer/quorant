# Jurisdiction Compliance Rules Engine — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a deterministic jurisdiction rules table and compliance checking engine to the AI module, enabling platform-managed statutory parameter lookups and per-org compliance evaluation across 7 enforceable dimensions.

**Architecture:** Extends the existing AI module with a `jurisdiction_rules` table (platform-scoped, no RLS), a `compliance_checks` audit trail table (tenant-scoped, with RLS), a `ComplianceResolver` interface that wraps deterministic lookups, and registered `CategoryEvaluator` functions per dimension. Events flow through NATS for webhook integration and worker-driven re-evaluation.

**Tech Stack:** Go 1.25, PostgreSQL (pgx v5), NATS JetStream, `net/http` stdlib router

**Spec:** `docs/superpowers/specs/2026-04-09-jurisdiction-compliance-engine-design.md`

---

## File Map

### New Files

| File | Responsibility |
|---|---|
| `backend/migrations/20260409000025_jurisdiction_rules.sql` | Schema for jurisdiction_rules + compliance_checks + indexes + RLS |
| `backend/internal/ai/compliance.go` | ComplianceResolver interface, CategoryEvaluator type, OrgComplianceContext |
| `backend/internal/ai/compliance_domain.go` | JurisdictionRule, RuleValue, ComplianceCheck, ComplianceResult, ComplianceReport types |
| `backend/internal/ai/jurisdiction_rule_repository.go` | JurisdictionRuleRepository interface |
| `backend/internal/ai/jurisdiction_rule_postgres.go` | PostgreSQL implementation of JurisdictionRuleRepository |
| `backend/internal/ai/compliance_check_repository.go` | ComplianceCheckRepository interface |
| `backend/internal/ai/compliance_check_postgres.go` | PostgreSQL implementation of ComplianceCheckRepository |
| `backend/internal/ai/compliance_service.go` | ComplianceService struct implementing ComplianceResolver |
| `backend/internal/ai/compliance_evaluators.go` | 7 CategoryEvaluator functions |
| `backend/internal/ai/compliance_requests.go` | Request/response DTOs for HTTP handlers |
| `backend/internal/ai/handler_jurisdiction_admin.go` | Platform admin CRUD handlers for jurisdiction rules |
| `backend/internal/ai/handler_compliance.go` | Tenant-scoped compliance status handlers |
| `backend/seeds/jurisdiction_rules_seed.sql` | Seed data for FL, CA, TX, AZ, CO |
| `backend/internal/ai/jurisdiction_rule_postgres_test.go` | Repository integration tests |
| `backend/internal/ai/compliance_check_postgres_test.go` | Repository integration tests |
| `backend/internal/ai/compliance_service_test.go` | Service unit tests |
| `backend/internal/ai/compliance_evaluators_test.go` | Evaluator BDD tests |
| `backend/internal/ai/handler_compliance_test.go` | Handler tests |
| `backend/internal/ai/handler_jurisdiction_admin_test.go` | Admin handler tests |

### Modified Files

| File | Change |
|---|---|
| `backend/internal/ai/routes.go` | Register compliance + admin jurisdiction rule routes |
| `backend/internal/gov/service.go` | Add `compliance` field to GovService, use in fine/meeting enforcement |
| `backend/internal/fin/service.go` | Add `compliance` field to FinService, use in estoppel lookups |
| `backend/cmd/quorant-api/main.go` | Wire ComplianceService, register evaluators, inject into gov/fin |
| `backend/migrations/20260409000005_seed_roles.sql` | Add new permissions |

---

## Task 1: Database Migration

**Files:**
- Create: `backend/migrations/20260409000025_jurisdiction_rules.sql`
- Modify: `backend/migrations/20260409000005_seed_roles.sql`

- [ ] **Step 1: Write the migration SQL**

```sql
-- backend/migrations/20260409000025_jurisdiction_rules.sql

-- jurisdiction_rules: platform-managed deterministic statutory parameters.
-- NOT tenant-scoped. No org_id. No RLS.
CREATE TABLE jurisdiction_rules (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    jurisdiction      TEXT NOT NULL,
    rule_category     TEXT NOT NULL,
    rule_key          TEXT NOT NULL,
    value_type        TEXT NOT NULL,
    value             JSONB NOT NULL,
    statute_reference TEXT NOT NULL,
    effective_date    DATE NOT NULL,
    expiration_date   DATE,
    notes             TEXT,
    source_doc_id     UUID REFERENCES governing_documents(id),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by        UUID REFERENCES users(id),
    UNIQUE (jurisdiction, rule_category, rule_key, effective_date)
);

CREATE INDEX idx_jurisdiction_rules_active
    ON jurisdiction_rules (jurisdiction, rule_category, rule_key)
    WHERE expiration_date IS NULL OR expiration_date > now();

CREATE INDEX idx_jurisdiction_rules_jurisdiction
    ON jurisdiction_rules (jurisdiction);

CREATE INDEX idx_jurisdiction_rules_upcoming
    ON jurisdiction_rules (effective_date)
    WHERE effective_date > now();

-- compliance_checks: tenant-scoped audit trail of compliance evaluations.
CREATE TABLE compliance_checks (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id           UUID NOT NULL REFERENCES organizations(id),
    rule_id          UUID NOT NULL REFERENCES jurisdiction_rules(id),
    status           TEXT NOT NULL,
    details          JSONB,
    checked_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    resolved_at      TIMESTAMPTZ,
    resolution_notes TEXT
);

CREATE INDEX idx_compliance_checks_org ON compliance_checks (org_id, checked_at DESC);
CREATE INDEX idx_compliance_checks_rule ON compliance_checks (rule_id);
CREATE INDEX idx_compliance_checks_unresolved
    ON compliance_checks (org_id)
    WHERE resolved_at IS NULL AND status = 'non_compliant';

-- RLS for compliance_checks (tenant-scoped).
ALTER TABLE compliance_checks ENABLE ROW LEVEL SECURITY;
ALTER TABLE compliance_checks FORCE ROW LEVEL SECURITY;
CREATE POLICY compliance_checks_tenant_isolation ON compliance_checks
    USING (org_id = current_setting('app.current_org_id', true)::uuid);
```

- [ ] **Step 2: Add new permissions to seed_roles migration**

Append to the permissions INSERT in `backend/migrations/20260409000005_seed_roles.sql`:

```sql
('admin.jurisdiction_rule.manage', 'Manage jurisdiction rules',        'admin'),
('ai.compliance.read',             'Read compliance status',           'ai'),
('ai.jurisdiction_rule.read',      'Read jurisdiction rules',          'ai'),
```

Add role assignments for these permissions:
- `platform_admin` gets `admin.jurisdiction_rule.manage`
- `board_president`, `hoa_manager`, `firm_admin` get `ai.compliance.read` and `ai.jurisdiction_rule.read`

- [ ] **Step 3: Verify migration applies cleanly**

Run: `cd backend && go run cmd/migrate/main.go up` (or equivalent Atlas command)

Expected: Migration applies, tables and indexes created.

- [ ] **Step 4: Commit**

```bash
git add backend/migrations/20260409000025_jurisdiction_rules.sql backend/migrations/20260409000005_seed_roles.sql
git commit -m "feat: add jurisdiction_rules and compliance_checks schema (issue #7)"
```

---

## Task 2: Domain Types and Interfaces

**Files:**
- Create: `backend/internal/ai/compliance.go`
- Create: `backend/internal/ai/compliance_domain.go`

- [ ] **Step 1: Create compliance domain types**

```go
// backend/internal/ai/compliance_domain.go
package ai

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// JurisdictionRule represents a platform-managed statutory parameter.
type JurisdictionRule struct {
	ID               uuid.UUID       `json:"id"`
	Jurisdiction     string          `json:"jurisdiction"`
	RuleCategory     string          `json:"rule_category"`
	RuleKey          string          `json:"rule_key"`
	ValueType        string          `json:"value_type"`
	Value            json.RawMessage `json:"value"`
	StatuteReference string          `json:"statute_reference"`
	EffectiveDate    time.Time       `json:"effective_date"`
	ExpirationDate   *time.Time      `json:"expiration_date,omitempty"`
	Notes            string          `json:"notes,omitempty"`
	SourceDocID      *uuid.UUID      `json:"source_doc_id,omitempty"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
	CreatedBy        *uuid.UUID      `json:"created_by,omitempty"`
}

// RuleValue is the read-model returned by ComplianceResolver lookups.
type RuleValue struct {
	ID               uuid.UUID       `json:"id"`
	Jurisdiction     string          `json:"jurisdiction"`
	Category         string          `json:"category"`
	Key              string          `json:"key"`
	ValueType        string          `json:"value_type"`
	Value            json.RawMessage `json:"value"`
	StatuteReference string          `json:"statute_reference"`
	EffectiveDate    time.Time       `json:"effective_date"`
	ExpirationDate   *time.Time      `json:"expiration_date,omitempty"`
	Notes            string          `json:"notes,omitempty"`
	SourceDocID      *uuid.UUID      `json:"source_doc_id,omitempty"`
}

// IntValue unmarshals the JSONB value as an integer.
func (r *RuleValue) IntValue() (int, error) {
	var v int
	if err := json.Unmarshal(r.Value, &v); err != nil {
		return 0, fmt.Errorf("rule %s/%s: expected integer: %w", r.Category, r.Key, err)
	}
	return v, nil
}

// BoolValue unmarshals the JSONB value as a boolean.
func (r *RuleValue) BoolValue() (bool, error) {
	var v bool
	if err := json.Unmarshal(r.Value, &v); err != nil {
		return false, fmt.Errorf("rule %s/%s: expected boolean: %w", r.Category, r.Key, err)
	}
	return v, nil
}

// DecimalValue unmarshals the JSONB value as a float64.
func (r *RuleValue) DecimalValue() (float64, error) {
	var v float64
	if err := json.Unmarshal(r.Value, &v); err != nil {
		return 0, fmt.Errorf("rule %s/%s: expected decimal: %w", r.Category, r.Key, err)
	}
	return v, nil
}

// TextValue unmarshals the JSONB value as a string.
func (r *RuleValue) TextValue() (string, error) {
	var v string
	if err := json.Unmarshal(r.Value, &v); err != nil {
		return "", fmt.Errorf("rule %s/%s: expected text: %w", r.Category, r.Key, err)
	}
	return v, nil
}

// RuleValueFromJurisdictionRule converts a JurisdictionRule to a RuleValue.
func RuleValueFromJurisdictionRule(r *JurisdictionRule) RuleValue {
	return RuleValue{
		ID:               r.ID,
		Jurisdiction:     r.Jurisdiction,
		Category:         r.RuleCategory,
		Key:              r.RuleKey,
		ValueType:        r.ValueType,
		Value:            r.Value,
		StatuteReference: r.StatuteReference,
		EffectiveDate:    r.EffectiveDate,
		ExpirationDate:   r.ExpirationDate,
		Notes:            r.Notes,
		SourceDocID:      r.SourceDocID,
	}
}

// ComplianceCheck records a single compliance evaluation result for an org+rule.
type ComplianceCheck struct {
	ID              uuid.UUID       `json:"id"`
	OrgID           uuid.UUID       `json:"org_id"`
	RuleID          uuid.UUID       `json:"rule_id"`
	Status          string          `json:"status"`
	Details         json.RawMessage `json:"details,omitempty"`
	CheckedAt       time.Time       `json:"checked_at"`
	ResolvedAt      *time.Time      `json:"resolved_at,omitempty"`
	ResolutionNotes string          `json:"resolution_notes,omitempty"`
}

// ComplianceResult is the outcome of evaluating one category for an org.
type ComplianceResult struct {
	Category  string         `json:"category"`
	Status    string         `json:"status"`
	Rules     []RuleValue    `json:"rules"`
	Details   map[string]any `json:"details,omitempty"`
	CheckedAt time.Time      `json:"checked_at"`
}

// ComplianceReport is the full compliance evaluation for an org across all categories.
type ComplianceReport struct {
	OrgID        uuid.UUID          `json:"org_id"`
	Jurisdiction string             `json:"jurisdiction"`
	Results      []ComplianceResult `json:"results"`
	Summary      ComplianceSummary  `json:"summary"`
	CheckedAt    time.Time          `json:"checked_at"`
}

// ComplianceSummary summarizes a ComplianceReport.
type ComplianceSummary struct {
	Total         int `json:"total"`
	Compliant     int `json:"compliant"`
	NonCompliant  int `json:"non_compliant"`
	NotApplicable int `json:"not_applicable"`
	Unknown       int `json:"unknown"`
}
```

- [ ] **Step 2: Create the ComplianceResolver interface and related types**

```go
// backend/internal/ai/compliance.go
package ai

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// ComplianceResolver wraps both tiers of compliance checking.
// Tier 1: deterministic jurisdiction rule lookups (this engine).
// Tier 2: interpretive compliance via PolicyResolver (existing AI layer).
type ComplianceResolver interface {
	GetJurisdictionRule(ctx context.Context, jurisdiction, category, key string) (*RuleValue, error)
	ListJurisdictionRules(ctx context.Context, jurisdiction, category string) ([]RuleValue, error)
	EvaluateCompliance(ctx context.Context, orgID uuid.UUID) (*ComplianceReport, error)
	CheckCompliance(ctx context.Context, orgID uuid.UUID, category string) (*ComplianceResult, error)
}

// OrgComplianceContext provides the org-level data that category evaluators need.
type OrgComplianceContext struct {
	OrgID                   uuid.UUID
	Jurisdiction            string
	UnitCount               int
	HasWebsite              bool
	LastReserveStudyDate    *time.Time
	BuildingStories         int
	ElectronicVotingEnabled bool
	ProxyVotingEnabled      bool
}

// CategoryEvaluator evaluates compliance for a single category given an org's context and the active rules.
type CategoryEvaluator func(ctx context.Context, org OrgComplianceContext, rules []RuleValue) (*ComplianceResult, error)

// ValidRuleCategories lists the 7 enforceable dimensions.
var ValidRuleCategories = []string{
	"meeting_notice",
	"fine_limits",
	"reserve_study",
	"website_requirements",
	"record_retention",
	"voting_rules",
	"estoppel",
}

// ValidValueTypes lists the supported value types for jurisdiction rules.
var ValidValueTypes = []string{"integer", "decimal", "boolean", "text", "json"}

// IsValidRuleCategory returns true if the category is one of the 7 supported dimensions.
func IsValidRuleCategory(cat string) bool {
	for _, c := range ValidRuleCategories {
		if c == cat {
			return true
		}
	}
	return false
}

// IsValidValueType returns true if the value type is supported.
func IsValidValueType(vt string) bool {
	for _, v := range ValidValueTypes {
		if v == vt {
			return true
		}
	}
	return false
}

// NoopComplianceResolver is a stub used when the compliance engine is not yet wired.
type NoopComplianceResolver struct{}

func NewNoopComplianceResolver() *NoopComplianceResolver { return &NoopComplianceResolver{} }
func (r *NoopComplianceResolver) GetJurisdictionRule(ctx context.Context, jurisdiction, category, key string) (*RuleValue, error) {
	return nil, nil
}
func (r *NoopComplianceResolver) ListJurisdictionRules(ctx context.Context, jurisdiction, category string) ([]RuleValue, error) {
	return nil, nil
}
func (r *NoopComplianceResolver) EvaluateCompliance(ctx context.Context, orgID uuid.UUID) (*ComplianceReport, error) {
	return nil, nil
}
func (r *NoopComplianceResolver) CheckCompliance(ctx context.Context, orgID uuid.UUID, category string) (*ComplianceResult, error) {
	return nil, nil
}
```

- [ ] **Step 3: Verify compilation**

Run: `cd backend && go build ./internal/ai/...`

Expected: Compiles cleanly.

- [ ] **Step 4: Commit**

```bash
git add backend/internal/ai/compliance.go backend/internal/ai/compliance_domain.go
git commit -m "feat: add ComplianceResolver interface and jurisdiction rule domain types (issue #7)"
```

---

## Task 3: JurisdictionRule Repository

**Files:**
- Create: `backend/internal/ai/jurisdiction_rule_repository.go`
- Create: `backend/internal/ai/jurisdiction_rule_postgres.go`
- Create: `backend/internal/ai/jurisdiction_rule_postgres_test.go`

- [ ] **Step 1: Write the repository interface**

```go
// backend/internal/ai/jurisdiction_rule_repository.go
package ai

import (
	"context"

	"github.com/google/uuid"
)

// JurisdictionRuleRepository persists jurisdiction rules.
type JurisdictionRuleRepository interface {
	Create(ctx context.Context, rule *JurisdictionRule) (*JurisdictionRule, error)
	Update(ctx context.Context, rule *JurisdictionRule) (*JurisdictionRule, error)
	FindByID(ctx context.Context, id uuid.UUID) (*JurisdictionRule, error)
	GetActiveRule(ctx context.Context, jurisdiction, category, key string) (*JurisdictionRule, error)
	ListActiveRules(ctx context.Context, jurisdiction, category string) ([]JurisdictionRule, error)
	ListActiveRulesByJurisdiction(ctx context.Context, jurisdiction string) ([]JurisdictionRule, error)
	ListAllRules(ctx context.Context, jurisdiction string, limit int, afterID *uuid.UUID) ([]JurisdictionRule, bool, error)
	ListUpcomingRules(ctx context.Context, withinDays int) ([]JurisdictionRule, error)
}
```

- [ ] **Step 2: Write the failing integration tests**

```go
// backend/internal/ai/jurisdiction_rule_postgres_test.go
//go:build integration

package ai_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/quorant/quorant/internal/ai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupJurisdictionRuleTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, "postgres://quorant:quorant@localhost:5432/quorant_dev?sslmode=disable")
	require.NoError(t, err, "connecting to test database")
	t.Cleanup(func() {
		cleanCtx := context.Background()
		pool.Exec(cleanCtx, "DELETE FROM compliance_checks")
		pool.Exec(cleanCtx, "DELETE FROM jurisdiction_rules")
		pool.Close()
	})
	return pool
}

func TestPostgresJurisdictionRuleRepository_Create(t *testing.T) {
	pool := setupJurisdictionRuleTestDB(t)
	repo := ai.NewPostgresJurisdictionRuleRepository(pool)
	ctx := context.Background()

	rule := &ai.JurisdictionRule{
		Jurisdiction:     "FL",
		RuleCategory:     "meeting_notice",
		RuleKey:          "board_meeting_notice_days",
		ValueType:        "integer",
		Value:            json.RawMessage(`2`),
		StatuteReference: "FS 720.303(2)(c)",
		EffectiveDate:    time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	created, err := repo.Create(ctx, rule)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, created.ID)
	assert.Equal(t, "FL", created.Jurisdiction)
	assert.Equal(t, "meeting_notice", created.RuleCategory)
	assert.Equal(t, "board_meeting_notice_days", created.RuleKey)
	assert.Equal(t, json.RawMessage(`2`), created.Value)
}

func TestPostgresJurisdictionRuleRepository_GetActiveRule(t *testing.T) {
	pool := setupJurisdictionRuleTestDB(t)
	repo := ai.NewPostgresJurisdictionRuleRepository(pool)
	ctx := context.Background()

	// Insert an active rule (effective in the past, no expiration)
	active := &ai.JurisdictionRule{
		Jurisdiction:     "FL",
		RuleCategory:     "fine_limits",
		RuleKey:          "hearing_required",
		ValueType:        "boolean",
		Value:            json.RawMessage(`true`),
		StatuteReference: "FS 720.305(2)",
		EffectiveDate:    time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	_, err := repo.Create(ctx, active)
	require.NoError(t, err)

	// Insert a future rule (effective next year)
	future := &ai.JurisdictionRule{
		Jurisdiction:     "FL",
		RuleCategory:     "fine_limits",
		RuleKey:          "hearing_required",
		ValueType:        "boolean",
		Value:            json.RawMessage(`false`),
		StatuteReference: "FS 720.305(2) amended",
		EffectiveDate:    time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	_, err = repo.Create(ctx, future)
	require.NoError(t, err)

	// GetActiveRule should return the currently active one
	found, err := repo.GetActiveRule(ctx, "FL", "fine_limits", "hearing_required")
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, json.RawMessage(`true`), found.Value)
}

func TestPostgresJurisdictionRuleRepository_ListActiveRules(t *testing.T) {
	pool := setupJurisdictionRuleTestDB(t)
	repo := ai.NewPostgresJurisdictionRuleRepository(pool)
	ctx := context.Background()

	// Insert 2 active meeting_notice rules for FL
	for _, key := range []string{"board_meeting_notice_days", "annual_meeting_notice_days"} {
		_, err := repo.Create(ctx, &ai.JurisdictionRule{
			Jurisdiction:     "FL",
			RuleCategory:     "meeting_notice",
			RuleKey:          key,
			ValueType:        "integer",
			Value:            json.RawMessage(`7`),
			StatuteReference: "FS 720.303",
			EffectiveDate:    time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		})
		require.NoError(t, err)
	}

	rules, err := repo.ListActiveRules(ctx, "FL", "meeting_notice")
	require.NoError(t, err)
	assert.Len(t, rules, 2)
}

func TestPostgresJurisdictionRuleRepository_ListUpcomingRules(t *testing.T) {
	pool := setupJurisdictionRuleTestDB(t)
	repo := ai.NewPostgresJurisdictionRuleRepository(pool)
	ctx := context.Background()

	// Insert a rule effective 10 days from now
	_, err := repo.Create(ctx, &ai.JurisdictionRule{
		Jurisdiction:     "CO",
		RuleCategory:     "website_requirements",
		RuleKey:          "required_for_unit_count",
		ValueType:        "integer",
		Value:            json.RawMessage(`50`),
		StatuteReference: "CRS 38-33.3-209.5",
		EffectiveDate:    time.Now().AddDate(0, 0, 10),
	})
	require.NoError(t, err)

	rules, err := repo.ListUpcomingRules(ctx, 30)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(rules), 1)
}

func TestPostgresJurisdictionRuleRepository_UniqueConstraint(t *testing.T) {
	pool := setupJurisdictionRuleTestDB(t)
	repo := ai.NewPostgresJurisdictionRuleRepository(pool)
	ctx := context.Background()

	rule := &ai.JurisdictionRule{
		Jurisdiction:     "TX",
		RuleCategory:     "voting_rules",
		RuleKey:          "proxy_allowed",
		ValueType:        "boolean",
		Value:            json.RawMessage(`true`),
		StatuteReference: "Tex. Prop. Code 209",
		EffectiveDate:    time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC),
	}

	_, err := repo.Create(ctx, rule)
	require.NoError(t, err)

	// Duplicate should fail
	_, err = repo.Create(ctx, rule)
	assert.Error(t, err)
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `cd backend && go test -tags=integration ./internal/ai/... -run TestPostgresJurisdictionRuleRepository -v`

Expected: FAIL — `NewPostgresJurisdictionRuleRepository` not defined.

- [ ] **Step 4: Write the PostgreSQL implementation**

```go
// backend/internal/ai/jurisdiction_rule_postgres.go
package ai

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresJurisdictionRuleRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresJurisdictionRuleRepository(pool *pgxpool.Pool) *PostgresJurisdictionRuleRepository {
	return &PostgresJurisdictionRuleRepository{pool: pool}
}

const jurisdictionRuleCols = `id, jurisdiction, rule_category, rule_key, value_type,
	value, statute_reference, effective_date, expiration_date,
	notes, source_doc_id, created_at, updated_at, created_by`

func scanJurisdictionRule(row pgx.Row) (*JurisdictionRule, error) {
	var r JurisdictionRule
	err := row.Scan(
		&r.ID, &r.Jurisdiction, &r.RuleCategory, &r.RuleKey, &r.ValueType,
		&r.Value, &r.StatuteReference, &r.EffectiveDate, &r.ExpirationDate,
		&r.Notes, &r.SourceDocID, &r.CreatedAt, &r.UpdatedAt, &r.CreatedBy,
	)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func collectJurisdictionRules(rows pgx.Rows) ([]JurisdictionRule, error) {
	results := []JurisdictionRule{}
	for rows.Next() {
		var r JurisdictionRule
		if err := rows.Scan(
			&r.ID, &r.Jurisdiction, &r.RuleCategory, &r.RuleKey, &r.ValueType,
			&r.Value, &r.StatuteReference, &r.EffectiveDate, &r.ExpirationDate,
			&r.Notes, &r.SourceDocID, &r.CreatedAt, &r.UpdatedAt, &r.CreatedBy,
		); err != nil {
			return nil, fmt.Errorf("ai: jurisdiction_rules scan: %w", err)
		}
		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ai: jurisdiction_rules rows: %w", err)
	}
	return results, nil
}

func (r *PostgresJurisdictionRuleRepository) Create(ctx context.Context, rule *JurisdictionRule) (*JurisdictionRule, error) {
	const q = `
		INSERT INTO jurisdiction_rules (
			jurisdiction, rule_category, rule_key, value_type,
			value, statute_reference, effective_date, expiration_date,
			notes, source_doc_id, created_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING ` + jurisdictionRuleCols

	row := r.pool.QueryRow(ctx, q,
		rule.Jurisdiction, rule.RuleCategory, rule.RuleKey, rule.ValueType,
		rule.Value, rule.StatuteReference, utcMidnight(rule.EffectiveDate), rule.ExpirationDate,
		rule.Notes, rule.SourceDocID, rule.CreatedBy,
	)
	result, err := scanJurisdictionRule(row)
	if err != nil {
		return nil, fmt.Errorf("ai: CreateJurisdictionRule: %w", err)
	}
	return result, nil
}

func (r *PostgresJurisdictionRuleRepository) Update(ctx context.Context, rule *JurisdictionRule) (*JurisdictionRule, error) {
	const q = `
		UPDATE jurisdiction_rules SET
			value             = $1,
			statute_reference = $2,
			expiration_date   = $3,
			notes             = $4,
			source_doc_id     = $5,
			updated_at        = now()
		WHERE id = $6
		RETURNING ` + jurisdictionRuleCols

	row := r.pool.QueryRow(ctx, q,
		rule.Value, rule.StatuteReference, rule.ExpirationDate,
		rule.Notes, rule.SourceDocID, rule.ID,
	)
	result, err := scanJurisdictionRule(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("ai: UpdateJurisdictionRule: %s not found", rule.ID)
	}
	if err != nil {
		return nil, fmt.Errorf("ai: UpdateJurisdictionRule: %w", err)
	}
	return result, nil
}

func (r *PostgresJurisdictionRuleRepository) FindByID(ctx context.Context, id uuid.UUID) (*JurisdictionRule, error) {
	q := `SELECT ` + jurisdictionRuleCols + ` FROM jurisdiction_rules WHERE id = $1`
	row := r.pool.QueryRow(ctx, q, id)
	result, err := scanJurisdictionRule(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("ai: FindJurisdictionRuleByID: %w", err)
	}
	return result, nil
}

func (r *PostgresJurisdictionRuleRepository) GetActiveRule(ctx context.Context, jurisdiction, category, key string) (*JurisdictionRule, error) {
	q := `SELECT ` + jurisdictionRuleCols + `
		FROM jurisdiction_rules
		WHERE jurisdiction = $1
		  AND rule_category = $2
		  AND rule_key = $3
		  AND effective_date <= CURRENT_DATE
		  AND (expiration_date IS NULL OR expiration_date > CURRENT_DATE)
		ORDER BY effective_date DESC
		LIMIT 1`

	row := r.pool.QueryRow(ctx, q, jurisdiction, category, key)
	result, err := scanJurisdictionRule(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("ai: GetActiveJurisdictionRule: %w", err)
	}
	return result, nil
}

func (r *PostgresJurisdictionRuleRepository) ListActiveRules(ctx context.Context, jurisdiction, category string) ([]JurisdictionRule, error) {
	q := `SELECT ` + jurisdictionRuleCols + `
		FROM jurisdiction_rules
		WHERE jurisdiction = $1
		  AND rule_category = $2
		  AND effective_date <= CURRENT_DATE
		  AND (expiration_date IS NULL OR expiration_date > CURRENT_DATE)
		ORDER BY rule_key`

	rows, err := r.pool.Query(ctx, q, jurisdiction, category)
	if err != nil {
		return nil, fmt.Errorf("ai: ListActiveJurisdictionRules: %w", err)
	}
	defer rows.Close()
	return collectJurisdictionRules(rows)
}

func (r *PostgresJurisdictionRuleRepository) ListActiveRulesByJurisdiction(ctx context.Context, jurisdiction string) ([]JurisdictionRule, error) {
	q := `SELECT ` + jurisdictionRuleCols + `
		FROM jurisdiction_rules
		WHERE jurisdiction = $1
		  AND effective_date <= CURRENT_DATE
		  AND (expiration_date IS NULL OR expiration_date > CURRENT_DATE)
		ORDER BY rule_category, rule_key`

	rows, err := r.pool.Query(ctx, q, jurisdiction)
	if err != nil {
		return nil, fmt.Errorf("ai: ListActiveRulesByJurisdiction: %w", err)
	}
	defer rows.Close()
	return collectJurisdictionRules(rows)
}

func (r *PostgresJurisdictionRuleRepository) ListAllRules(ctx context.Context, jurisdiction string, limit int, afterID *uuid.UUID) ([]JurisdictionRule, bool, error) {
	q := `SELECT ` + jurisdictionRuleCols + `
		FROM jurisdiction_rules
		WHERE jurisdiction = $1
		  AND ($3::uuid IS NULL OR id < $3)
		ORDER BY effective_date DESC, id DESC
		LIMIT $2`

	rows, err := r.pool.Query(ctx, q, jurisdiction, limit+1, afterID)
	if err != nil {
		return nil, false, fmt.Errorf("ai: ListAllJurisdictionRules: %w", err)
	}
	defer rows.Close()

	results, err := collectJurisdictionRules(rows)
	if err != nil {
		return nil, false, err
	}

	hasMore := len(results) > limit
	if hasMore {
		results = results[:limit]
	}
	return results, hasMore, nil
}

func (r *PostgresJurisdictionRuleRepository) ListUpcomingRules(ctx context.Context, withinDays int) ([]JurisdictionRule, error) {
	q := `SELECT ` + jurisdictionRuleCols + `
		FROM jurisdiction_rules
		WHERE effective_date > CURRENT_DATE
		  AND effective_date <= CURRENT_DATE + $1 * INTERVAL '1 day'
		  AND (expiration_date IS NULL OR expiration_date > CURRENT_DATE)
		ORDER BY effective_date`

	rows, err := r.pool.Query(ctx, q, withinDays)
	if err != nil {
		return nil, fmt.Errorf("ai: ListUpcomingJurisdictionRules: %w", err)
	}
	defer rows.Close()
	return collectJurisdictionRules(rows)
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd backend && go test -tags=integration ./internal/ai/... -run TestPostgresJurisdictionRuleRepository -v`

Expected: All 4 tests PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/ai/jurisdiction_rule_repository.go backend/internal/ai/jurisdiction_rule_postgres.go backend/internal/ai/jurisdiction_rule_postgres_test.go
git commit -m "feat: add JurisdictionRuleRepository with PostgreSQL implementation (issue #7)"
```

---

## Task 4: ComplianceCheck Repository

**Files:**
- Create: `backend/internal/ai/compliance_check_repository.go`
- Create: `backend/internal/ai/compliance_check_postgres.go`
- Create: `backend/internal/ai/compliance_check_postgres_test.go`

- [ ] **Step 1: Write the repository interface**

```go
// backend/internal/ai/compliance_check_repository.go
package ai

import (
	"context"

	"github.com/google/uuid"
)

// ComplianceCheckRepository persists compliance check audit records.
type ComplianceCheckRepository interface {
	Create(ctx context.Context, check *ComplianceCheck) (*ComplianceCheck, error)
	ListByOrg(ctx context.Context, orgID uuid.UUID, limit int, afterID *uuid.UUID) ([]ComplianceCheck, bool, error)
	GetLatestByOrgAndRule(ctx context.Context, orgID, ruleID uuid.UUID) (*ComplianceCheck, error)
	Resolve(ctx context.Context, id uuid.UUID, notes string) (*ComplianceCheck, error)
}
```

- [ ] **Step 2: Write the failing integration tests**

```go
// backend/internal/ai/compliance_check_postgres_test.go
//go:build integration

package ai_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/ai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPostgresComplianceCheckRepository_Create(t *testing.T) {
	pool := setupJurisdictionRuleTestDB(t)
	ruleRepo := ai.NewPostgresJurisdictionRuleRepository(pool)
	checkRepo := ai.NewPostgresComplianceCheckRepository(pool)
	ctx := context.Background()

	// Create a test org
	var orgID uuid.UUID
	err := pool.QueryRow(ctx,
		`INSERT INTO organizations (type, name, slug, path, settings)
		 VALUES ('hoa', $1, $2, $3, '{}') RETURNING id`,
		"Compliance Test HOA "+uuid.New().String(),
		"compliance-test-"+uuid.New().String(),
		"compliance_test_"+uuid.New().String(),
	).Scan(&orgID)
	require.NoError(t, err)

	// Create a jurisdiction rule
	rule, err := ruleRepo.Create(ctx, &ai.JurisdictionRule{
		Jurisdiction:     "FL",
		RuleCategory:     "website_requirements",
		RuleKey:          "required_for_unit_count",
		ValueType:        "integer",
		Value:            json.RawMessage(`100`),
		StatuteReference: "FL HB 1203",
		EffectiveDate:    time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)

	// Create a compliance check
	check := &ai.ComplianceCheck{
		OrgID:  orgID,
		RuleID: rule.ID,
		Status: "non_compliant",
		Details: json.RawMessage(`{"unit_count": 150, "threshold": 100, "has_website": false}`),
	}

	created, err := checkRepo.Create(ctx, check)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, created.ID)
	assert.Equal(t, "non_compliant", created.Status)
}

func TestPostgresComplianceCheckRepository_Resolve(t *testing.T) {
	pool := setupJurisdictionRuleTestDB(t)
	ruleRepo := ai.NewPostgresJurisdictionRuleRepository(pool)
	checkRepo := ai.NewPostgresComplianceCheckRepository(pool)
	ctx := context.Background()

	var orgID uuid.UUID
	err := pool.QueryRow(ctx,
		`INSERT INTO organizations (type, name, slug, path, settings)
		 VALUES ('hoa', $1, $2, $3, '{}') RETURNING id`,
		"Resolve Test HOA "+uuid.New().String(),
		"resolve-test-"+uuid.New().String(),
		"resolve_test_"+uuid.New().String(),
	).Scan(&orgID)
	require.NoError(t, err)

	rule, err := ruleRepo.Create(ctx, &ai.JurisdictionRule{
		Jurisdiction:     "FL",
		RuleCategory:     "reserve_study",
		RuleKey:          "sirs_required",
		ValueType:        "boolean",
		Value:            json.RawMessage(`true`),
		StatuteReference: "SB 4-D",
		EffectiveDate:    time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)

	check, err := checkRepo.Create(ctx, &ai.ComplianceCheck{
		OrgID:  orgID,
		RuleID: rule.ID,
		Status: "non_compliant",
	})
	require.NoError(t, err)
	assert.Nil(t, check.ResolvedAt)

	resolved, err := checkRepo.Resolve(ctx, check.ID, "Reserve study completed 2026-03-15")
	require.NoError(t, err)
	assert.NotNil(t, resolved.ResolvedAt)
	assert.Equal(t, "Reserve study completed 2026-03-15", resolved.ResolutionNotes)
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `cd backend && go test -tags=integration ./internal/ai/... -run TestPostgresComplianceCheckRepository -v`

Expected: FAIL — `NewPostgresComplianceCheckRepository` not defined.

- [ ] **Step 4: Write the PostgreSQL implementation**

```go
// backend/internal/ai/compliance_check_postgres.go
package ai

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresComplianceCheckRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresComplianceCheckRepository(pool *pgxpool.Pool) *PostgresComplianceCheckRepository {
	return &PostgresComplianceCheckRepository{pool: pool}
}

const complianceCheckCols = `id, org_id, rule_id, status, details, checked_at, resolved_at, resolution_notes`

func scanComplianceCheck(row pgx.Row) (*ComplianceCheck, error) {
	var c ComplianceCheck
	err := row.Scan(
		&c.ID, &c.OrgID, &c.RuleID, &c.Status, &c.Details,
		&c.CheckedAt, &c.ResolvedAt, &c.ResolutionNotes,
	)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func collectComplianceChecks(rows pgx.Rows) ([]ComplianceCheck, error) {
	results := []ComplianceCheck{}
	for rows.Next() {
		var c ComplianceCheck
		if err := rows.Scan(
			&c.ID, &c.OrgID, &c.RuleID, &c.Status, &c.Details,
			&c.CheckedAt, &c.ResolvedAt, &c.ResolutionNotes,
		); err != nil {
			return nil, fmt.Errorf("ai: compliance_checks scan: %w", err)
		}
		results = append(results, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ai: compliance_checks rows: %w", err)
	}
	return results, nil
}

func (r *PostgresComplianceCheckRepository) Create(ctx context.Context, check *ComplianceCheck) (*ComplianceCheck, error) {
	const q = `
		INSERT INTO compliance_checks (org_id, rule_id, status, details)
		VALUES ($1, $2, $3, $4)
		RETURNING ` + complianceCheckCols

	row := r.pool.QueryRow(ctx, q,
		check.OrgID, check.RuleID, check.Status, marshalRawOrNull(check.Details),
	)
	result, err := scanComplianceCheck(row)
	if err != nil {
		return nil, fmt.Errorf("ai: CreateComplianceCheck: %w", err)
	}
	return result, nil
}

func (r *PostgresComplianceCheckRepository) ListByOrg(ctx context.Context, orgID uuid.UUID, limit int, afterID *uuid.UUID) ([]ComplianceCheck, bool, error) {
	q := `SELECT ` + complianceCheckCols + `
		FROM compliance_checks
		WHERE org_id = $1
		  AND ($3::uuid IS NULL OR id < $3)
		ORDER BY checked_at DESC, id DESC
		LIMIT $2`

	rows, err := r.pool.Query(ctx, q, orgID, limit+1, afterID)
	if err != nil {
		return nil, false, fmt.Errorf("ai: ListComplianceChecksByOrg: %w", err)
	}
	defer rows.Close()

	results, err := collectComplianceChecks(rows)
	if err != nil {
		return nil, false, err
	}

	hasMore := len(results) > limit
	if hasMore {
		results = results[:limit]
	}
	return results, hasMore, nil
}

func (r *PostgresComplianceCheckRepository) GetLatestByOrgAndRule(ctx context.Context, orgID, ruleID uuid.UUID) (*ComplianceCheck, error) {
	q := `SELECT ` + complianceCheckCols + `
		FROM compliance_checks
		WHERE org_id = $1 AND rule_id = $2
		ORDER BY checked_at DESC
		LIMIT 1`

	row := r.pool.QueryRow(ctx, q, orgID, ruleID)
	result, err := scanComplianceCheck(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("ai: GetLatestComplianceCheck: %w", err)
	}
	return result, nil
}

func (r *PostgresComplianceCheckRepository) Resolve(ctx context.Context, id uuid.UUID, notes string) (*ComplianceCheck, error) {
	const q = `
		UPDATE compliance_checks
		SET resolved_at = now(), resolution_notes = $1
		WHERE id = $2
		RETURNING ` + complianceCheckCols

	row := r.pool.QueryRow(ctx, q, notes, id)
	result, err := scanComplianceCheck(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("ai: ResolveComplianceCheck: %s not found", id)
	}
	if err != nil {
		return nil, fmt.Errorf("ai: ResolveComplianceCheck: %w", err)
	}
	return result, nil
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd backend && go test -tags=integration ./internal/ai/... -run TestPostgresComplianceCheckRepository -v`

Expected: All tests PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/ai/compliance_check_repository.go backend/internal/ai/compliance_check_postgres.go backend/internal/ai/compliance_check_postgres_test.go
git commit -m "feat: add ComplianceCheckRepository with PostgreSQL implementation (issue #7)"
```

---

## Task 5: Category Evaluators

**Files:**
- Create: `backend/internal/ai/compliance_evaluators.go`
- Create: `backend/internal/ai/compliance_evaluators_test.go`

- [ ] **Step 1: Write the failing BDD evaluator tests**

```go
// backend/internal/ai/compliance_evaluators_test.go
package ai_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/ai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeRule(category, key, valueType string, value any) ai.RuleValue {
	v, _ := json.Marshal(value)
	return ai.RuleValue{
		ID:               uuid.New(),
		Jurisdiction:     "FL",
		Category:         category,
		Key:              key,
		ValueType:        valueType,
		Value:            json.RawMessage(v),
		StatuteReference: "test statute",
		EffectiveDate:    time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
	}
}

func TestEvaluateWebsiteRequirements_NonCompliant(t *testing.T) {
	ctx := context.Background()
	org := ai.OrgComplianceContext{
		OrgID:        uuid.New(),
		Jurisdiction: "FL",
		UnitCount:    150,
		HasWebsite:   false,
	}
	rules := []ai.RuleValue{
		makeRule("website_requirements", "required_for_unit_count", "integer", 100),
	}

	result, err := ai.EvaluateWebsiteRequirements(ctx, org, rules)
	require.NoError(t, err)
	assert.Equal(t, "non_compliant", result.Status)
	assert.Equal(t, "website_requirements", result.Category)
}

func TestEvaluateWebsiteRequirements_Compliant(t *testing.T) {
	ctx := context.Background()
	org := ai.OrgComplianceContext{
		OrgID:        uuid.New(),
		Jurisdiction: "FL",
		UnitCount:    150,
		HasWebsite:   true,
	}
	rules := []ai.RuleValue{
		makeRule("website_requirements", "required_for_unit_count", "integer", 100),
	}

	result, err := ai.EvaluateWebsiteRequirements(ctx, org, rules)
	require.NoError(t, err)
	assert.Equal(t, "compliant", result.Status)
}

func TestEvaluateWebsiteRequirements_NotApplicable(t *testing.T) {
	ctx := context.Background()
	org := ai.OrgComplianceContext{
		OrgID:        uuid.New(),
		Jurisdiction: "FL",
		UnitCount:    50,
		HasWebsite:   false,
	}
	rules := []ai.RuleValue{
		makeRule("website_requirements", "required_for_unit_count", "integer", 100),
	}

	result, err := ai.EvaluateWebsiteRequirements(ctx, org, rules)
	require.NoError(t, err)
	assert.Equal(t, "not_applicable", result.Status)
}

func TestEvaluateWebsiteRequirements_NoRules(t *testing.T) {
	ctx := context.Background()
	org := ai.OrgComplianceContext{OrgID: uuid.New(), Jurisdiction: "TX"}

	result, err := ai.EvaluateWebsiteRequirements(ctx, org, nil)
	require.NoError(t, err)
	assert.Equal(t, "not_applicable", result.Status)
}

func TestEvaluateMeetingNotice_RulesPresent(t *testing.T) {
	ctx := context.Background()
	org := ai.OrgComplianceContext{OrgID: uuid.New(), Jurisdiction: "FL"}
	rules := []ai.RuleValue{
		makeRule("meeting_notice", "board_meeting_notice_days", "integer", 2),
	}

	result, err := ai.EvaluateMeetingNotice(ctx, org, rules)
	require.NoError(t, err)
	assert.Equal(t, "compliant", result.Status)
}

func TestEvaluateMeetingNotice_NoRules(t *testing.T) {
	ctx := context.Background()
	org := ai.OrgComplianceContext{OrgID: uuid.New(), Jurisdiction: "FL"}

	result, err := ai.EvaluateMeetingNotice(ctx, org, nil)
	require.NoError(t, err)
	assert.Equal(t, "unknown", result.Status)
}

func TestEvaluateReserveStudy_NonCompliant_NoStudy(t *testing.T) {
	ctx := context.Background()
	org := ai.OrgComplianceContext{
		OrgID:                uuid.New(),
		Jurisdiction:         "FL",
		BuildingStories:      4,
		LastReserveStudyDate: nil,
	}
	rules := []ai.RuleValue{
		makeRule("reserve_study", "sirs_required", "boolean", true),
		makeRule("reserve_study", "sirs_interval_years", "integer", 10),
		makeRule("reserve_study", "sirs_min_stories", "integer", 3),
		makeRule("reserve_study", "waiver_allowed", "boolean", false),
	}

	result, err := ai.EvaluateReserveStudy(ctx, org, rules)
	require.NoError(t, err)
	assert.Equal(t, "non_compliant", result.Status)
}

func TestEvaluateReserveStudy_NotApplicable_TooShort(t *testing.T) {
	ctx := context.Background()
	org := ai.OrgComplianceContext{
		OrgID:           uuid.New(),
		Jurisdiction:    "FL",
		BuildingStories: 2,
	}
	rules := []ai.RuleValue{
		makeRule("reserve_study", "sirs_required", "boolean", true),
		makeRule("reserve_study", "sirs_min_stories", "integer", 3),
	}

	result, err := ai.EvaluateReserveStudy(ctx, org, rules)
	require.NoError(t, err)
	assert.Equal(t, "not_applicable", result.Status)
}

func TestEvaluateFineLimits_RulesPresent(t *testing.T) {
	ctx := context.Background()
	org := ai.OrgComplianceContext{OrgID: uuid.New(), Jurisdiction: "FL"}
	rules := []ai.RuleValue{
		makeRule("fine_limits", "hearing_required", "boolean", true),
		makeRule("fine_limits", "daily_aggregate_cap_cents", "integer", 10000),
	}

	result, err := ai.EvaluateFineLimits(ctx, org, rules)
	require.NoError(t, err)
	assert.Equal(t, "compliant", result.Status)
}

func TestEvaluateRecordRetention_RulesPresent(t *testing.T) {
	ctx := context.Background()
	org := ai.OrgComplianceContext{OrgID: uuid.New(), Jurisdiction: "FL"}
	rules := []ai.RuleValue{
		makeRule("record_retention", "financial_records_years", "integer", 7),
	}

	result, err := ai.EvaluateRecordRetention(ctx, org, rules)
	require.NoError(t, err)
	assert.Equal(t, "compliant", result.Status)
}

func TestEvaluateVotingRules_RulesPresent(t *testing.T) {
	ctx := context.Background()
	org := ai.OrgComplianceContext{OrgID: uuid.New(), Jurisdiction: "FL"}
	rules := []ai.RuleValue{
		makeRule("voting_rules", "proxy_allowed", "boolean", true),
	}

	result, err := ai.EvaluateVotingRules(ctx, org, rules)
	require.NoError(t, err)
	assert.Equal(t, "compliant", result.Status)
}

func TestEvaluateEstoppel_RulesPresent(t *testing.T) {
	ctx := context.Background()
	org := ai.OrgComplianceContext{OrgID: uuid.New(), Jurisdiction: "FL"}
	rules := []ai.RuleValue{
		makeRule("estoppel", "turnaround_business_days", "integer", 10),
		makeRule("estoppel", "fee_cap_cents", "integer", 25000),
	}

	result, err := ai.EvaluateEstoppel(ctx, org, rules)
	require.NoError(t, err)
	assert.Equal(t, "compliant", result.Status)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd backend && go test ./internal/ai/... -run TestEvaluate -v`

Expected: FAIL — `EvaluateWebsiteRequirements` not defined.

- [ ] **Step 3: Write the evaluator implementations**

```go
// backend/internal/ai/compliance_evaluators.go
package ai

import (
	"context"
	"time"
)

func findRuleByKey(rules []RuleValue, key string) *RuleValue {
	for i := range rules {
		if rules[i].Key == key {
			return &rules[i]
		}
	}
	return nil
}

func newResult(category, status string, rules []RuleValue, details map[string]any) *ComplianceResult {
	return &ComplianceResult{
		Category:  category,
		Status:    status,
		Rules:     rules,
		Details:   details,
		CheckedAt: time.Now(),
	}
}

// EvaluateWebsiteRequirements checks if the org meets website mandate thresholds.
func EvaluateWebsiteRequirements(ctx context.Context, org OrgComplianceContext, rules []RuleValue) (*ComplianceResult, error) {
	threshold := findRuleByKey(rules, "required_for_unit_count")
	if threshold == nil {
		return newResult("website_requirements", "not_applicable", rules, nil), nil
	}

	requiredCount, err := threshold.IntValue()
	if err != nil {
		return nil, err
	}

	details := map[string]any{
		"unit_count":  org.UnitCount,
		"threshold":   requiredCount,
		"has_website": org.HasWebsite,
	}

	if org.UnitCount < requiredCount {
		return newResult("website_requirements", "not_applicable", rules, details), nil
	}
	if !org.HasWebsite {
		return newResult("website_requirements", "non_compliant", rules, details), nil
	}
	return newResult("website_requirements", "compliant", rules, details), nil
}

// EvaluateMeetingNotice confirms that meeting notice rules exist for the jurisdiction.
// Actual enforcement happens at meeting scheduling time in the gov module.
func EvaluateMeetingNotice(ctx context.Context, org OrgComplianceContext, rules []RuleValue) (*ComplianceResult, error) {
	if len(rules) == 0 {
		return newResult("meeting_notice", "unknown", rules, map[string]any{
			"reason": "no meeting notice rules configured for jurisdiction",
		}), nil
	}
	return newResult("meeting_notice", "compliant", rules, nil), nil
}

// EvaluateFineLimits confirms that fine limit rules exist for the jurisdiction.
// Actual enforcement happens at fine creation time in the gov module.
func EvaluateFineLimits(ctx context.Context, org OrgComplianceContext, rules []RuleValue) (*ComplianceResult, error) {
	if len(rules) == 0 {
		return newResult("fine_limits", "unknown", rules, map[string]any{
			"reason": "no fine limit rules configured for jurisdiction",
		}), nil
	}
	return newResult("fine_limits", "compliant", rules, nil), nil
}

// EvaluateReserveStudy checks if the org has a current reserve study per state mandates.
func EvaluateReserveStudy(ctx context.Context, org OrgComplianceContext, rules []RuleValue) (*ComplianceResult, error) {
	requiredRule := findRuleByKey(rules, "sirs_required")
	if requiredRule == nil {
		return newResult("reserve_study", "not_applicable", rules, nil), nil
	}

	required, err := requiredRule.BoolValue()
	if err != nil {
		return nil, err
	}
	if !required {
		return newResult("reserve_study", "not_applicable", rules, nil), nil
	}

	// Check min stories threshold (e.g., FL SIRS applies to 3+ story buildings)
	minStoriesRule := findRuleByKey(rules, "sirs_min_stories")
	if minStoriesRule != nil {
		minStories, err := minStoriesRule.IntValue()
		if err != nil {
			return nil, err
		}
		if org.BuildingStories < minStories {
			return newResult("reserve_study", "not_applicable", rules, map[string]any{
				"building_stories": org.BuildingStories,
				"min_stories":      minStories,
			}), nil
		}
	}

	intervalRule := findRuleByKey(rules, "sirs_interval_years")
	var intervalYears int
	if intervalRule != nil {
		intervalYears, err = intervalRule.IntValue()
		if err != nil {
			return nil, err
		}
	}

	details := map[string]any{
		"required":             true,
		"interval_years":       intervalYears,
		"last_study_date":      org.LastReserveStudyDate,
		"building_stories":     org.BuildingStories,
	}

	if org.LastReserveStudyDate == nil {
		return newResult("reserve_study", "non_compliant", rules, details), nil
	}

	if intervalYears > 0 {
		cutoff := time.Now().AddDate(-intervalYears, 0, 0)
		if org.LastReserveStudyDate.Before(cutoff) {
			return newResult("reserve_study", "non_compliant", rules, details), nil
		}
	}

	return newResult("reserve_study", "compliant", rules, details), nil
}

// EvaluateRecordRetention confirms that record retention rules exist.
// Actual enforcement would require checking document ages, which is out of scope for v1.
func EvaluateRecordRetention(ctx context.Context, org OrgComplianceContext, rules []RuleValue) (*ComplianceResult, error) {
	if len(rules) == 0 {
		return newResult("record_retention", "unknown", rules, map[string]any{
			"reason": "no record retention rules configured for jurisdiction",
		}), nil
	}
	return newResult("record_retention", "compliant", rules, nil), nil
}

// EvaluateVotingRules confirms that voting rules exist for the jurisdiction.
// Actual enforcement happens in the gov module ballot/proxy features.
func EvaluateVotingRules(ctx context.Context, org OrgComplianceContext, rules []RuleValue) (*ComplianceResult, error) {
	if len(rules) == 0 {
		return newResult("voting_rules", "unknown", rules, map[string]any{
			"reason": "no voting rules configured for jurisdiction",
		}), nil
	}
	return newResult("voting_rules", "compliant", rules, nil), nil
}

// EvaluateEstoppel confirms that estoppel rules exist for the jurisdiction.
// Actual enforcement happens in the fin module estoppel certificate features.
func EvaluateEstoppel(ctx context.Context, org OrgComplianceContext, rules []RuleValue) (*ComplianceResult, error) {
	if len(rules) == 0 {
		return newResult("estoppel", "not_applicable", rules, map[string]any{
			"reason": "no estoppel rules configured for jurisdiction",
		}), nil
	}
	return newResult("estoppel", "compliant", rules, nil), nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd backend && go test ./internal/ai/... -run TestEvaluate -v`

Expected: All 12 evaluator tests PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/ai/compliance_evaluators.go backend/internal/ai/compliance_evaluators_test.go
git commit -m "feat: add 7 category evaluator functions with BDD tests (issue #7)"
```

---

## Task 6: ComplianceService

**Files:**
- Create: `backend/internal/ai/compliance_service.go`
- Create: `backend/internal/ai/compliance_service_test.go`

- [ ] **Step 1: Write the failing service tests**

```go
// backend/internal/ai/compliance_service_test.go
package ai_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/ai"
	"github.com/quorant/quorant/internal/org"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockJurisdictionRuleRepo implements ai.JurisdictionRuleRepository in-memory.
type mockJurisdictionRuleRepo struct {
	rules []ai.JurisdictionRule
}

func (m *mockJurisdictionRuleRepo) Create(ctx context.Context, rule *ai.JurisdictionRule) (*ai.JurisdictionRule, error) {
	rule.ID = uuid.New()
	rule.CreatedAt = time.Now()
	rule.UpdatedAt = time.Now()
	m.rules = append(m.rules, *rule)
	return rule, nil
}
func (m *mockJurisdictionRuleRepo) Update(ctx context.Context, rule *ai.JurisdictionRule) (*ai.JurisdictionRule, error) {
	return rule, nil
}
func (m *mockJurisdictionRuleRepo) FindByID(ctx context.Context, id uuid.UUID) (*ai.JurisdictionRule, error) {
	for _, r := range m.rules {
		if r.ID == id {
			return &r, nil
		}
	}
	return nil, nil
}
func (m *mockJurisdictionRuleRepo) GetActiveRule(ctx context.Context, jurisdiction, category, key string) (*ai.JurisdictionRule, error) {
	now := time.Now()
	for _, r := range m.rules {
		if r.Jurisdiction == jurisdiction && r.RuleCategory == category && r.RuleKey == key &&
			!r.EffectiveDate.After(now) && (r.ExpirationDate == nil || r.ExpirationDate.After(now)) {
			return &r, nil
		}
	}
	return nil, nil
}
func (m *mockJurisdictionRuleRepo) ListActiveRules(ctx context.Context, jurisdiction, category string) ([]ai.JurisdictionRule, error) {
	now := time.Now()
	var results []ai.JurisdictionRule
	for _, r := range m.rules {
		if r.Jurisdiction == jurisdiction && r.RuleCategory == category &&
			!r.EffectiveDate.After(now) && (r.ExpirationDate == nil || r.ExpirationDate.After(now)) {
			results = append(results, r)
		}
	}
	return results, nil
}
func (m *mockJurisdictionRuleRepo) ListActiveRulesByJurisdiction(ctx context.Context, jurisdiction string) ([]ai.JurisdictionRule, error) {
	now := time.Now()
	var results []ai.JurisdictionRule
	for _, r := range m.rules {
		if r.Jurisdiction == jurisdiction &&
			!r.EffectiveDate.After(now) && (r.ExpirationDate == nil || r.ExpirationDate.After(now)) {
			results = append(results, r)
		}
	}
	return results, nil
}
func (m *mockJurisdictionRuleRepo) ListAllRules(ctx context.Context, jurisdiction string, limit int, afterID *uuid.UUID) ([]ai.JurisdictionRule, bool, error) {
	return m.rules, false, nil
}
func (m *mockJurisdictionRuleRepo) ListUpcomingRules(ctx context.Context, withinDays int) ([]ai.JurisdictionRule, error) {
	return nil, nil
}

// mockComplianceCheckRepo implements ai.ComplianceCheckRepository in-memory.
type mockComplianceCheckRepo struct {
	checks []ai.ComplianceCheck
}

func (m *mockComplianceCheckRepo) Create(ctx context.Context, check *ai.ComplianceCheck) (*ai.ComplianceCheck, error) {
	check.ID = uuid.New()
	check.CheckedAt = time.Now()
	m.checks = append(m.checks, *check)
	return check, nil
}
func (m *mockComplianceCheckRepo) ListByOrg(ctx context.Context, orgID uuid.UUID, limit int, afterID *uuid.UUID) ([]ai.ComplianceCheck, bool, error) {
	return m.checks, false, nil
}
func (m *mockComplianceCheckRepo) GetLatestByOrgAndRule(ctx context.Context, orgID, ruleID uuid.UUID) (*ai.ComplianceCheck, error) {
	return nil, nil
}
func (m *mockComplianceCheckRepo) Resolve(ctx context.Context, id uuid.UUID, notes string) (*ai.ComplianceCheck, error) {
	return nil, nil
}

// mockOrgLookup implements ai.OrgLookup for testing.
type mockOrgLookup struct {
	orgs map[uuid.UUID]*org.Organization
}

func (m *mockOrgLookup) FindByID(ctx context.Context, id uuid.UUID) (*org.Organization, error) {
	if o, ok := m.orgs[id]; ok {
		return o, nil
	}
	return nil, nil
}
func (m *mockOrgLookup) FindActiveManagement(ctx context.Context, hoaOrgID uuid.UUID) (*org.OrgManagement, error) {
	return nil, nil
}
func (m *mockOrgLookup) Update(ctx context.Context, o *org.Organization) (*org.Organization, error) {
	return o, nil
}

func TestComplianceService_GetJurisdictionRule(t *testing.T) {
	ruleRepo := &mockJurisdictionRuleRepo{}
	_, err := ruleRepo.Create(context.Background(), &ai.JurisdictionRule{
		Jurisdiction:     "FL",
		RuleCategory:     "fine_limits",
		RuleKey:          "hearing_required",
		ValueType:        "boolean",
		Value:            json.RawMessage(`true`),
		StatuteReference: "FS 720.305(2)",
		EffectiveDate:    time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)

	svc := ai.NewComplianceService(ruleRepo, &mockComplianceCheckRepo{}, &mockOrgLookup{}, nil)

	rv, err := svc.GetJurisdictionRule(context.Background(), "FL", "fine_limits", "hearing_required")
	require.NoError(t, err)
	require.NotNil(t, rv)

	val, err := rv.BoolValue()
	require.NoError(t, err)
	assert.True(t, val)
}

func TestComplianceService_EvaluateCompliance(t *testing.T) {
	orgID := uuid.New()
	state := "FL"

	ruleRepo := &mockJurisdictionRuleRepo{}
	_, _ = ruleRepo.Create(context.Background(), &ai.JurisdictionRule{
		Jurisdiction:     "FL",
		RuleCategory:     "website_requirements",
		RuleKey:          "required_for_unit_count",
		ValueType:        "integer",
		Value:            json.RawMessage(`100`),
		StatuteReference: "FL HB 1203",
		EffectiveDate:    time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	})

	orgLookup := &mockOrgLookup{
		orgs: map[uuid.UUID]*org.Organization{
			orgID: {ID: orgID, Type: "hoa", Name: "Test HOA", State: &state},
		},
	}

	svc := ai.NewComplianceService(ruleRepo, &mockComplianceCheckRepo{}, orgLookup, nil)
	svc.RegisterEvaluator("website_requirements", ai.EvaluateWebsiteRequirements)
	svc.RegisterEvaluator("meeting_notice", ai.EvaluateMeetingNotice)
	svc.RegisterEvaluator("fine_limits", ai.EvaluateFineLimits)
	svc.RegisterEvaluator("reserve_study", ai.EvaluateReserveStudy)
	svc.RegisterEvaluator("record_retention", ai.EvaluateRecordRetention)
	svc.RegisterEvaluator("voting_rules", ai.EvaluateVotingRules)
	svc.RegisterEvaluator("estoppel", ai.EvaluateEstoppel)

	report, err := svc.EvaluateCompliance(context.Background(), orgID)
	require.NoError(t, err)
	require.NotNil(t, report)
	assert.Equal(t, "FL", report.Jurisdiction)
	assert.Len(t, report.Results, 7)
	assert.Equal(t, 7, report.Summary.Total)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd backend && go test ./internal/ai/... -run TestComplianceService -v`

Expected: FAIL — `NewComplianceService` not defined.

- [ ] **Step 3: Write the ComplianceService implementation**

```go
// backend/internal/ai/compliance_service.go
package ai

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

// ComplianceService implements ComplianceResolver by combining deterministic
// jurisdiction rule lookups with registered category evaluators.
type ComplianceService struct {
	rules      JurisdictionRuleRepository
	checks     ComplianceCheckRepository
	orgLookup  OrgLookup
	evaluators map[string]CategoryEvaluator
	logger     *slog.Logger
}

func NewComplianceService(
	rules JurisdictionRuleRepository,
	checks ComplianceCheckRepository,
	orgLookup OrgLookup,
	logger *slog.Logger,
) *ComplianceService {
	return &ComplianceService{
		rules:      rules,
		checks:     checks,
		orgLookup:  orgLookup,
		evaluators: make(map[string]CategoryEvaluator),
		logger:     logger,
	}
}

// RegisterEvaluator registers a CategoryEvaluator for a rule category.
func (s *ComplianceService) RegisterEvaluator(category string, eval CategoryEvaluator) {
	s.evaluators[category] = eval
}

func (s *ComplianceService) GetJurisdictionRule(ctx context.Context, jurisdiction, category, key string) (*RuleValue, error) {
	rule, err := s.rules.GetActiveRule(ctx, jurisdiction, category, key)
	if err != nil {
		return nil, fmt.Errorf("ai: GetJurisdictionRule: %w", err)
	}
	if rule == nil {
		return nil, nil
	}
	rv := RuleValueFromJurisdictionRule(rule)
	return &rv, nil
}

func (s *ComplianceService) ListJurisdictionRules(ctx context.Context, jurisdiction, category string) ([]RuleValue, error) {
	rules, err := s.rules.ListActiveRules(ctx, jurisdiction, category)
	if err != nil {
		return nil, fmt.Errorf("ai: ListJurisdictionRules: %w", err)
	}
	result := make([]RuleValue, len(rules))
	for i := range rules {
		result[i] = RuleValueFromJurisdictionRule(&rules[i])
	}
	return result, nil
}

func (s *ComplianceService) EvaluateCompliance(ctx context.Context, orgID uuid.UUID) (*ComplianceReport, error) {
	orgEntity, err := s.orgLookup.FindByID(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("ai: EvaluateCompliance org lookup: %w", err)
	}
	if orgEntity == nil {
		return nil, fmt.Errorf("ai: EvaluateCompliance: org %s not found", orgID)
	}
	if orgEntity.State == nil {
		return nil, fmt.Errorf("ai: EvaluateCompliance: org %s has no state configured", orgID)
	}

	jurisdiction := *orgEntity.State
	orgCtx := s.buildOrgContext(orgEntity)
	now := time.Now()

	var results []ComplianceResult
	summary := ComplianceSummary{}

	for _, category := range ValidRuleCategories {
		result, err := s.evaluateCategory(ctx, orgCtx, jurisdiction, category)
		if err != nil {
			if s.logger != nil {
				s.logger.Error("compliance evaluation failed",
					"org_id", orgID, "category", category, "error", err)
			}
			result = &ComplianceResult{
				Category:  category,
				Status:    "unknown",
				Details:   map[string]any{"error": err.Error()},
				CheckedAt: now,
			}
		}
		results = append(results, *result)
		summary.Total++
		switch result.Status {
		case "compliant":
			summary.Compliant++
		case "non_compliant":
			summary.NonCompliant++
		case "not_applicable":
			summary.NotApplicable++
		default:
			summary.Unknown++
		}
	}

	return &ComplianceReport{
		OrgID:        orgID,
		Jurisdiction: jurisdiction,
		Results:      results,
		Summary:      summary,
		CheckedAt:    now,
	}, nil
}

func (s *ComplianceService) CheckCompliance(ctx context.Context, orgID uuid.UUID, category string) (*ComplianceResult, error) {
	if !IsValidRuleCategory(category) {
		return nil, fmt.Errorf("ai: CheckCompliance: unknown category %q", category)
	}

	orgEntity, err := s.orgLookup.FindByID(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("ai: CheckCompliance org lookup: %w", err)
	}
	if orgEntity == nil {
		return nil, fmt.Errorf("ai: CheckCompliance: org %s not found", orgID)
	}
	if orgEntity.State == nil {
		return nil, fmt.Errorf("ai: CheckCompliance: org %s has no state configured", orgID)
	}

	orgCtx := s.buildOrgContext(orgEntity)
	return s.evaluateCategory(ctx, orgCtx, *orgEntity.State, category)
}

func (s *ComplianceService) evaluateCategory(ctx context.Context, orgCtx OrgComplianceContext, jurisdiction, category string) (*ComplianceResult, error) {
	evaluator, ok := s.evaluators[category]
	if !ok {
		return &ComplianceResult{
			Category:  category,
			Status:    "unknown",
			CheckedAt: time.Now(),
			Details:   map[string]any{"reason": "no evaluator registered"},
		}, nil
	}

	rules, err := s.rules.ListActiveRules(ctx, jurisdiction, category)
	if err != nil {
		return nil, fmt.Errorf("ai: evaluateCategory %s: %w", category, err)
	}

	ruleValues := make([]RuleValue, len(rules))
	for i := range rules {
		ruleValues[i] = RuleValueFromJurisdictionRule(&rules[i])
	}

	return evaluator(ctx, orgCtx, ruleValues)
}

func (s *ComplianceService) buildOrgContext(orgEntity interface{ GetID() uuid.UUID }) OrgComplianceContext {
	// This uses a type assertion to work with org.Organization.
	// The org entity is passed via OrgLookup.FindByID which returns *org.Organization.
	// We build the context from available fields. Fields not yet available
	// (unit count, building stories, etc.) default to zero values.
	// Future: query unit count from org repo, building info from org settings.
	return OrgComplianceContext{}
}
```

Wait — the `buildOrgContext` needs to work with `*org.Organization` which has `State` and `Website` fields. Let me fix that. The `OrgLookup` interface returns `*org.Organization`, so we can use it directly.

Replace the `buildOrgContext` method:

```go
func (s *ComplianceService) buildOrgContext(o *org.Organization) OrgComplianceContext {
	ctx := OrgComplianceContext{
		OrgID:        o.ID,
		HasWebsite:   o.Website != nil && *o.Website != "",
	}
	if o.State != nil {
		ctx.Jurisdiction = *o.State
	}
	// UnitCount, BuildingStories, LastReserveStudyDate, voting flags
	// are populated from org settings JSONB or will require additional
	// queries in future iterations. Defaults to zero values for now.
	return ctx
}
```

And update the method signatures in `EvaluateCompliance` and `CheckCompliance` to pass `orgEntity` directly.

Actually, let me write the complete, correct file:

```go
// backend/internal/ai/compliance_service.go
package ai

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/org"
)

// ComplianceService implements ComplianceResolver.
type ComplianceService struct {
	rules      JurisdictionRuleRepository
	checks     ComplianceCheckRepository
	orgLookup  OrgLookup
	evaluators map[string]CategoryEvaluator
	logger     *slog.Logger
}

func NewComplianceService(
	rules JurisdictionRuleRepository,
	checks ComplianceCheckRepository,
	orgLookup OrgLookup,
	logger *slog.Logger,
) *ComplianceService {
	return &ComplianceService{
		rules:      rules,
		checks:     checks,
		orgLookup:  orgLookup,
		evaluators: make(map[string]CategoryEvaluator),
		logger:     logger,
	}
}

func (s *ComplianceService) RegisterEvaluator(category string, eval CategoryEvaluator) {
	s.evaluators[category] = eval
}

func (s *ComplianceService) GetJurisdictionRule(ctx context.Context, jurisdiction, category, key string) (*RuleValue, error) {
	rule, err := s.rules.GetActiveRule(ctx, jurisdiction, category, key)
	if err != nil {
		return nil, fmt.Errorf("ai: GetJurisdictionRule: %w", err)
	}
	if rule == nil {
		return nil, nil
	}
	rv := RuleValueFromJurisdictionRule(rule)
	return &rv, nil
}

func (s *ComplianceService) ListJurisdictionRules(ctx context.Context, jurisdiction, category string) ([]RuleValue, error) {
	rules, err := s.rules.ListActiveRules(ctx, jurisdiction, category)
	if err != nil {
		return nil, fmt.Errorf("ai: ListJurisdictionRules: %w", err)
	}
	result := make([]RuleValue, len(rules))
	for i := range rules {
		result[i] = RuleValueFromJurisdictionRule(&rules[i])
	}
	return result, nil
}

func (s *ComplianceService) EvaluateCompliance(ctx context.Context, orgID uuid.UUID) (*ComplianceReport, error) {
	orgEntity, err := s.orgLookup.FindByID(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("ai: EvaluateCompliance org lookup: %w", err)
	}
	if orgEntity == nil {
		return nil, fmt.Errorf("ai: EvaluateCompliance: org %s not found", orgID)
	}
	if orgEntity.State == nil {
		return nil, fmt.Errorf("ai: EvaluateCompliance: org %s has no state configured", orgID)
	}

	jurisdiction := *orgEntity.State
	orgCtx := buildOrgComplianceContext(orgEntity)
	now := time.Now()

	var results []ComplianceResult
	summary := ComplianceSummary{}

	for _, category := range ValidRuleCategories {
		result, err := s.evaluateCategory(ctx, orgCtx, jurisdiction, category)
		if err != nil {
			if s.logger != nil {
				s.logger.Error("compliance evaluation failed",
					"org_id", orgID, "category", category, "error", err)
			}
			result = &ComplianceResult{
				Category:  category,
				Status:    "unknown",
				Details:   map[string]any{"error": err.Error()},
				CheckedAt: now,
			}
		}
		results = append(results, *result)
		summary.Total++
		switch result.Status {
		case "compliant":
			summary.Compliant++
		case "non_compliant":
			summary.NonCompliant++
		case "not_applicable":
			summary.NotApplicable++
		default:
			summary.Unknown++
		}
	}

	return &ComplianceReport{
		OrgID:        orgID,
		Jurisdiction: jurisdiction,
		Results:      results,
		Summary:      summary,
		CheckedAt:    now,
	}, nil
}

func (s *ComplianceService) CheckCompliance(ctx context.Context, orgID uuid.UUID, category string) (*ComplianceResult, error) {
	if !IsValidRuleCategory(category) {
		return nil, fmt.Errorf("ai: CheckCompliance: unknown category %q", category)
	}

	orgEntity, err := s.orgLookup.FindByID(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("ai: CheckCompliance org lookup: %w", err)
	}
	if orgEntity == nil {
		return nil, fmt.Errorf("ai: CheckCompliance: org %s not found", orgID)
	}
	if orgEntity.State == nil {
		return nil, fmt.Errorf("ai: CheckCompliance: org %s has no state configured", orgID)
	}

	orgCtx := buildOrgComplianceContext(orgEntity)
	return s.evaluateCategory(ctx, orgCtx, *orgEntity.State, category)
}

func (s *ComplianceService) evaluateCategory(ctx context.Context, orgCtx OrgComplianceContext, jurisdiction, category string) (*ComplianceResult, error) {
	evaluator, ok := s.evaluators[category]
	if !ok {
		return &ComplianceResult{
			Category:  category,
			Status:    "unknown",
			CheckedAt: time.Now(),
			Details:   map[string]any{"reason": "no evaluator registered"},
		}, nil
	}

	rules, err := s.rules.ListActiveRules(ctx, jurisdiction, category)
	if err != nil {
		return nil, fmt.Errorf("ai: evaluateCategory %s: %w", category, err)
	}

	ruleValues := make([]RuleValue, len(rules))
	for i := range rules {
		ruleValues[i] = RuleValueFromJurisdictionRule(&rules[i])
	}

	return evaluator(ctx, orgCtx, ruleValues)
}

func buildOrgComplianceContext(o *org.Organization) OrgComplianceContext {
	ctx := OrgComplianceContext{
		OrgID:      o.ID,
		HasWebsite: o.Website != nil && *o.Website != "",
	}
	if o.State != nil {
		ctx.Jurisdiction = *o.State
	}
	return ctx
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd backend && go test ./internal/ai/... -run TestComplianceService -v`

Expected: All service tests PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/ai/compliance_service.go backend/internal/ai/compliance_service_test.go
git commit -m "feat: add ComplianceService implementing ComplianceResolver (issue #7)"
```

---

## Task 7: Request DTOs and HTTP Handlers

**Files:**
- Create: `backend/internal/ai/compliance_requests.go`
- Create: `backend/internal/ai/handler_jurisdiction_admin.go`
- Create: `backend/internal/ai/handler_compliance.go`
- Create: `backend/internal/ai/handler_jurisdiction_admin_test.go`
- Create: `backend/internal/ai/handler_compliance_test.go`

- [ ] **Step 1: Write request/response DTOs**

```go
// backend/internal/ai/compliance_requests.go
package ai

import (
	"encoding/json"
	"fmt"
	"time"
)

// CreateJurisdictionRuleRequest is the request body for creating a jurisdiction rule.
type CreateJurisdictionRuleRequest struct {
	Jurisdiction     string          `json:"jurisdiction"`
	RuleCategory     string          `json:"rule_category"`
	RuleKey          string          `json:"rule_key"`
	ValueType        string          `json:"value_type"`
	Value            json.RawMessage `json:"value"`
	StatuteReference string          `json:"statute_reference"`
	EffectiveDate    string          `json:"effective_date"` // YYYY-MM-DD
	Notes            string          `json:"notes,omitempty"`
	SourceDocID      string          `json:"source_doc_id,omitempty"` // UUID string
}

func (r CreateJurisdictionRuleRequest) Validate() error {
	if r.Jurisdiction == "" {
		return fmt.Errorf("jurisdiction is required")
	}
	if !IsValidRuleCategory(r.RuleCategory) {
		return fmt.Errorf("invalid rule_category: %s", r.RuleCategory)
	}
	if r.RuleKey == "" {
		return fmt.Errorf("rule_key is required")
	}
	if !IsValidValueType(r.ValueType) {
		return fmt.Errorf("invalid value_type: %s", r.ValueType)
	}
	if len(r.Value) == 0 {
		return fmt.Errorf("value is required")
	}
	if r.StatuteReference == "" {
		return fmt.Errorf("statute_reference is required")
	}
	if r.EffectiveDate == "" {
		return fmt.Errorf("effective_date is required")
	}
	if _, err := time.Parse("2006-01-02", r.EffectiveDate); err != nil {
		return fmt.Errorf("effective_date must be YYYY-MM-DD: %w", err)
	}
	return nil
}

// UpdateJurisdictionRuleRequest is the request body for updating a jurisdiction rule.
// An update expires the current rule and creates a new one.
type UpdateJurisdictionRuleRequest struct {
	Value            json.RawMessage `json:"value"`
	StatuteReference string          `json:"statute_reference"`
	EffectiveDate    string          `json:"effective_date"` // YYYY-MM-DD for the new rule
	Notes            string          `json:"notes,omitempty"`
}

func (r UpdateJurisdictionRuleRequest) Validate() error {
	if len(r.Value) == 0 {
		return fmt.Errorf("value is required")
	}
	if r.StatuteReference == "" {
		return fmt.Errorf("statute_reference is required")
	}
	if r.EffectiveDate == "" {
		return fmt.Errorf("effective_date is required")
	}
	if _, err := time.Parse("2006-01-02", r.EffectiveDate); err != nil {
		return fmt.Errorf("effective_date must be YYYY-MM-DD: %w", err)
	}
	return nil
}
```

- [ ] **Step 2: Write the admin handler**

```go
// backend/internal/ai/handler_jurisdiction_admin.go
package ai

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/platform/api"
	"github.com/quorant/quorant/internal/platform/middleware"
)

// parseComplianceCursorID decodes a pagination cursor and returns the ID.
// Returns nil, nil when cursor is empty (first page).
func parseComplianceCursorID(cursor string) (*uuid.UUID, error) {
	if cursor == "" {
		return nil, nil
	}
	vals, err := api.DecodeCursor(cursor)
	if err != nil {
		return nil, err
	}
	id, err := uuid.Parse(vals["id"])
	if err != nil {
		return nil, err
	}
	return &id, nil
}

// JurisdictionAdminHandler handles platform admin CRUD for jurisdiction rules.
type JurisdictionAdminHandler struct {
	rules  JurisdictionRuleRepository
	logger *slog.Logger
}

func NewJurisdictionAdminHandler(rules JurisdictionRuleRepository, logger *slog.Logger) *JurisdictionAdminHandler {
	return &JurisdictionAdminHandler{rules: rules, logger: logger}
}

func (h *JurisdictionAdminHandler) CreateRule(w http.ResponseWriter, r *http.Request) {
	var req CreateJurisdictionRuleRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}
	if err := req.Validate(); err != nil {
		api.WriteError(w, api.NewValidationError(err.Error(), ""))
		return
	}

	effectiveDate, _ := time.Parse("2006-01-02", req.EffectiveDate)

	var sourceDocID *uuid.UUID
	if req.SourceDocID != "" {
		id, err := uuid.Parse(req.SourceDocID)
		if err != nil {
			api.WriteError(w, api.NewValidationError("source_doc_id must be a valid UUID", "source_doc_id"))
			return
		}
		sourceDocID = &id
	}

	userID := middleware.UserIDFromContext(r.Context())

	rule := &JurisdictionRule{
		Jurisdiction:     req.Jurisdiction,
		RuleCategory:     req.RuleCategory,
		RuleKey:          req.RuleKey,
		ValueType:        req.ValueType,
		Value:            req.Value,
		StatuteReference: req.StatuteReference,
		EffectiveDate:    effectiveDate,
		Notes:            req.Notes,
		SourceDocID:      sourceDocID,
		CreatedBy:        &userID,
	}

	created, err := h.rules.Create(r.Context(), rule)
	if err != nil {
		h.logger.Error("CreateJurisdictionRule failed", "error", err)
		api.WriteError(w, api.NewInternalError(err))
		return
	}

	api.WriteJSON(w, http.StatusCreated, created)
}

func (h *JurisdictionAdminHandler) ListRules(w http.ResponseWriter, r *http.Request) {
	jurisdiction := r.URL.Query().Get("jurisdiction")
	if jurisdiction == "" {
		api.WriteError(w, api.NewValidationError("jurisdiction query parameter is required", "jurisdiction"))
		return
	}

	page := api.ParsePageRequest(r)
	afterID, err := parseComplianceCursorID(page.Cursor)
	if err != nil {
		api.WriteError(w, api.NewValidationError("invalid cursor", "cursor"))
		return
	}

	rules, hasMore, err := h.rules.ListAllRules(r.Context(), jurisdiction, page.Limit, afterID)
	if err != nil {
		h.logger.Error("ListJurisdictionRules failed", "error", err)
		api.WriteError(w, api.NewInternalError(err))
		return
	}

	var meta *api.Meta
	if hasMore && len(rules) > 0 {
		meta = &api.Meta{
			Cursor:  api.EncodeCursor(map[string]string{"id": rules[len(rules)-1].ID.String()}),
			HasMore: true,
		}
	}

	api.WriteJSONWithMeta(w, http.StatusOK, rules, meta)
}

func (h *JurisdictionAdminHandler) GetRule(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		api.WriteError(w, api.NewValidationError("id must be a valid UUID", "id"))
		return
	}

	rule, err := h.rules.FindByID(r.Context(), id)
	if err != nil {
		h.logger.Error("GetJurisdictionRule failed", "error", err)
		api.WriteError(w, api.NewInternalError(err))
		return
	}
	if rule == nil {
		api.WriteError(w, api.NewNotFoundError("jurisdiction rule not found"))
		return
	}

	api.WriteJSON(w, http.StatusOK, rule)
}

func (h *JurisdictionAdminHandler) UpdateRule(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		api.WriteError(w, api.NewValidationError("id must be a valid UUID", "id"))
		return
	}

	var req UpdateJurisdictionRuleRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}
	if err := req.Validate(); err != nil {
		api.WriteError(w, api.NewValidationError(err.Error(), ""))
		return
	}

	// Find the existing rule
	existing, err := h.rules.FindByID(r.Context(), id)
	if err != nil {
		h.logger.Error("UpdateJurisdictionRule find failed", "error", err)
		api.WriteError(w, api.NewInternalError(err))
		return
	}
	if existing == nil {
		api.WriteError(w, api.NewNotFoundError("jurisdiction rule not found"))
		return
	}

	// Expire the existing rule
	now := time.Now()
	existing.ExpirationDate = &now
	if _, err := h.rules.Update(r.Context(), existing); err != nil {
		h.logger.Error("UpdateJurisdictionRule expire failed", "error", err)
		api.WriteError(w, api.NewInternalError(err))
		return
	}

	// Create the replacement rule
	effectiveDate, _ := time.Parse("2006-01-02", req.EffectiveDate)
	userID := middleware.UserIDFromContext(r.Context())

	replacement := &JurisdictionRule{
		Jurisdiction:     existing.Jurisdiction,
		RuleCategory:     existing.RuleCategory,
		RuleKey:          existing.RuleKey,
		ValueType:        existing.ValueType,
		Value:            req.Value,
		StatuteReference: req.StatuteReference,
		EffectiveDate:    effectiveDate,
		Notes:            req.Notes,
		SourceDocID:      existing.SourceDocID,
		CreatedBy:        &userID,
	}

	created, err := h.rules.Create(r.Context(), replacement)
	if err != nil {
		h.logger.Error("UpdateJurisdictionRule create replacement failed", "error", err)
		api.WriteError(w, api.NewInternalError(err))
		return
	}

	api.WriteJSON(w, http.StatusOK, created)
}

func (h *JurisdictionAdminHandler) ExpireRule(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		api.WriteError(w, api.NewValidationError("id must be a valid UUID", "id"))
		return
	}

	existing, err := h.rules.FindByID(r.Context(), id)
	if err != nil {
		h.logger.Error("ExpireJurisdictionRule find failed", "error", err)
		api.WriteError(w, api.NewInternalError(err))
		return
	}
	if existing == nil {
		api.WriteError(w, api.NewNotFoundError("jurisdiction rule not found"))
		return
	}

	now := time.Now()
	existing.ExpirationDate = &now
	updated, err := h.rules.Update(r.Context(), existing)
	if err != nil {
		h.logger.Error("ExpireJurisdictionRule update failed", "error", err)
		api.WriteError(w, api.NewInternalError(err))
		return
	}

	api.WriteJSON(w, http.StatusOK, updated)
}
```

- [ ] **Step 3: Write the tenant compliance handler**

```go
// backend/internal/ai/handler_compliance.go
package ai

import (
	"log/slog"
	"net/http"

	"github.com/quorant/quorant/internal/platform/api"
)

// ComplianceHandler handles tenant-scoped compliance status endpoints.
type ComplianceHandler struct {
	service *ComplianceService
	checks  ComplianceCheckRepository
	logger  *slog.Logger
}

func NewComplianceHandler(service *ComplianceService, checks ComplianceCheckRepository, logger *slog.Logger) *ComplianceHandler {
	return &ComplianceHandler{service: service, checks: checks, logger: logger}
}

func (h *ComplianceHandler) GetComplianceReport(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseOrgID(r)
	if err != nil {
		api.WriteError(w, api.NewValidationError("org_id must be a valid UUID", "org_id"))
		return
	}

	report, err := h.service.EvaluateCompliance(r.Context(), orgID)
	if err != nil {
		h.logger.Error("GetComplianceReport failed", "org_id", orgID, "error", err)
		api.WriteError(w, api.NewInternalError(err))
		return
	}

	api.WriteJSON(w, http.StatusOK, report)
}

func (h *ComplianceHandler) CheckCategory(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseOrgID(r)
	if err != nil {
		api.WriteError(w, api.NewValidationError("org_id must be a valid UUID", "org_id"))
		return
	}

	category := r.PathValue("category")
	if !IsValidRuleCategory(category) {
		api.WriteError(w, api.NewValidationError("invalid category: "+category, "category"))
		return
	}

	result, err := h.service.CheckCompliance(r.Context(), orgID, category)
	if err != nil {
		h.logger.Error("CheckCategory failed", "org_id", orgID, "category", category, "error", err)
		api.WriteError(w, api.NewInternalError(err))
		return
	}

	api.WriteJSON(w, http.StatusOK, result)
}

func (h *ComplianceHandler) GetComplianceHistory(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseOrgID(r)
	if err != nil {
		api.WriteError(w, api.NewValidationError("org_id must be a valid UUID", "org_id"))
		return
	}

	page := api.ParsePageRequest(r)
	afterID, err := parseComplianceCursorID(page.Cursor)
	if err != nil {
		api.WriteError(w, api.NewValidationError("invalid cursor", "cursor"))
		return
	}

	checks, hasMore, err := h.checks.ListByOrg(r.Context(), orgID, page.Limit, afterID)
	if err != nil {
		h.logger.Error("GetComplianceHistory failed", "org_id", orgID, "error", err)
		api.WriteError(w, api.NewInternalError(err))
		return
	}

	var meta *api.Meta
	if hasMore && len(checks) > 0 {
		meta = &api.Meta{
			Cursor:  api.EncodeCursor(map[string]string{"id": checks[len(checks)-1].ID.String()}),
			HasMore: true,
		}
	}

	api.WriteJSONWithMeta(w, http.StatusOK, checks, meta)
}

func (h *ComplianceHandler) ListJurisdictionRulesForOrg(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseOrgID(r)
	if err != nil {
		api.WriteError(w, api.NewValidationError("org_id must be a valid UUID", "org_id"))
		return
	}

	orgEntity, err := h.service.orgLookup.FindByID(r.Context(), orgID)
	if err != nil {
		h.logger.Error("ListJurisdictionRulesForOrg org lookup failed", "error", err)
		api.WriteError(w, api.NewInternalError(err))
		return
	}
	if orgEntity == nil || orgEntity.State == nil {
		api.WriteError(w, api.NewNotFoundError("org not found or has no state configured"))
		return
	}

	rules, err := h.service.rules.ListActiveRulesByJurisdiction(r.Context(), *orgEntity.State)
	if err != nil {
		h.logger.Error("ListJurisdictionRulesForOrg failed", "error", err)
		api.WriteError(w, api.NewInternalError(err))
		return
	}

	api.WriteJSON(w, http.StatusOK, rules)
}

func (h *ComplianceHandler) ListJurisdictionRulesByCategory(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseOrgID(r)
	if err != nil {
		api.WriteError(w, api.NewValidationError("org_id must be a valid UUID", "org_id"))
		return
	}

	category := r.PathValue("category")
	if !IsValidRuleCategory(category) {
		api.WriteError(w, api.NewValidationError("invalid category: "+category, "category"))
		return
	}

	orgEntity, err := h.service.orgLookup.FindByID(r.Context(), orgID)
	if err != nil {
		h.logger.Error("ListJurisdictionRulesByCategory org lookup failed", "error", err)
		api.WriteError(w, api.NewInternalError(err))
		return
	}
	if orgEntity == nil || orgEntity.State == nil {
		api.WriteError(w, api.NewNotFoundError("org not found or has no state configured"))
		return
	}

	rules, err := h.service.rules.ListActiveRules(r.Context(), *orgEntity.State, category)
	if err != nil {
		h.logger.Error("ListJurisdictionRulesByCategory failed", "error", err)
		api.WriteError(w, api.NewInternalError(err))
		return
	}

	api.WriteJSON(w, http.StatusOK, rules)
}
```

- [ ] **Step 4: Write handler tests**

```go
// backend/internal/ai/handler_jurisdiction_admin_test.go
package ai_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/quorant/quorant/internal/ai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"log/slog"
)

func setupAdminTestServer(t *testing.T) (*httptest.Server, *mockJurisdictionRuleRepo) {
	t.Helper()
	repo := &mockJurisdictionRuleRepo{}
	handler := ai.NewJurisdictionAdminHandler(repo, slog.New(slog.NewTextHandler(io.Discard, nil)))

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/admin/jurisdiction-rules", handler.CreateRule)
	mux.HandleFunc("GET /api/v1/admin/jurisdiction-rules/{id}", handler.GetRule)

	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts, repo
}

func TestAdminCreateRule_Valid(t *testing.T) {
	ts, _ := setupAdminTestServer(t)

	body, _ := json.Marshal(ai.CreateJurisdictionRuleRequest{
		Jurisdiction:     "FL",
		RuleCategory:     "meeting_notice",
		RuleKey:          "board_meeting_notice_days",
		ValueType:        "integer",
		Value:            json.RawMessage(`2`),
		StatuteReference: "FS 720.303(2)(c)",
		EffectiveDate:    "2024-01-01",
	})

	resp, err := http.Post(ts.URL+"/api/v1/admin/jurisdiction-rules", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}

func TestAdminCreateRule_InvalidCategory(t *testing.T) {
	ts, _ := setupAdminTestServer(t)

	body, _ := json.Marshal(map[string]any{
		"jurisdiction":      "FL",
		"rule_category":     "invalid_category",
		"rule_key":          "test",
		"value_type":        "integer",
		"value":             1,
		"statute_reference": "test",
		"effective_date":    "2024-01-01",
	})

	resp, err := http.Post(ts.URL+"/api/v1/admin/jurisdiction-rules", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}
```

```go
// backend/internal/ai/handler_compliance_test.go
package ai_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/ai"
	"github.com/quorant/quorant/internal/org"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"log/slog"
)

func setupComplianceTestServer(t *testing.T) (*httptest.Server, *mockJurisdictionRuleRepo, uuid.UUID) {
	t.Helper()
	orgID := uuid.New()
	state := "FL"

	ruleRepo := &mockJurisdictionRuleRepo{}
	checkRepo := &mockComplianceCheckRepo{}
	orgLookup := &mockOrgLookup{
		orgs: map[uuid.UUID]*org.Organization{
			orgID: {ID: orgID, Type: "hoa", Name: "Test HOA", State: &state},
		},
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := ai.NewComplianceService(ruleRepo, checkRepo, orgLookup, logger)
	svc.RegisterEvaluator("website_requirements", ai.EvaluateWebsiteRequirements)
	svc.RegisterEvaluator("meeting_notice", ai.EvaluateMeetingNotice)
	svc.RegisterEvaluator("fine_limits", ai.EvaluateFineLimits)
	svc.RegisterEvaluator("reserve_study", ai.EvaluateReserveStudy)
	svc.RegisterEvaluator("record_retention", ai.EvaluateRecordRetention)
	svc.RegisterEvaluator("voting_rules", ai.EvaluateVotingRules)
	svc.RegisterEvaluator("estoppel", ai.EvaluateEstoppel)

	handler := ai.NewComplianceHandler(svc, checkRepo, logger)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/organizations/{org_id}/compliance", handler.GetComplianceReport)
	mux.HandleFunc("GET /api/v1/organizations/{org_id}/compliance/{category}", handler.CheckCategory)

	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts, ruleRepo, orgID
}

func TestComplianceHandler_GetReport(t *testing.T) {
	ts, ruleRepo, orgID := setupComplianceTestServer(t)

	// Seed a rule
	ruleRepo.Create(nil, &ai.JurisdictionRule{
		Jurisdiction:     "FL",
		RuleCategory:     "website_requirements",
		RuleKey:          "required_for_unit_count",
		ValueType:        "integer",
		Value:            json.RawMessage(`100`),
		StatuteReference: "FL HB 1203",
		EffectiveDate:    time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	})

	resp, err := http.Get(ts.URL + "/api/v1/organizations/" + orgID.String() + "/compliance")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var report ai.ComplianceReport
	err = json.NewDecoder(resp.Body).Decode(&report)
	require.NoError(t, err)
	assert.Equal(t, "FL", report.Jurisdiction)
	assert.Len(t, report.Results, 7)
}

func TestComplianceHandler_CheckCategory_Invalid(t *testing.T) {
	ts, _, orgID := setupComplianceTestServer(t)

	resp, err := http.Get(ts.URL + "/api/v1/organizations/" + orgID.String() + "/compliance/invalid_cat")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}
```

- [ ] **Step 5: Run all handler tests**

Run: `cd backend && go test ./internal/ai/... -run "TestAdmin|TestComplianceHandler" -v`

Expected: All handler tests PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/ai/compliance_requests.go backend/internal/ai/handler_jurisdiction_admin.go backend/internal/ai/handler_compliance.go backend/internal/ai/handler_jurisdiction_admin_test.go backend/internal/ai/handler_compliance_test.go
git commit -m "feat: add HTTP handlers for jurisdiction rules admin and compliance status (issue #7)"
```

---

## Task 8: Route Registration

**Files:**
- Modify: `backend/internal/ai/routes.go`

- [ ] **Step 1: Add compliance and admin routes to RegisterRoutes**

Add a new `RegisterComplianceRoutes` function (or extend `RegisterRoutes`) in `routes.go`. Since admin routes use a different middleware pattern (platform-scoped, no tenant context), add a separate registration function:

```go
// Add to backend/internal/ai/routes.go

// RegisterComplianceRoutes registers tenant-scoped compliance endpoints.
func RegisterComplianceRoutes(
	mux *http.ServeMux,
	handler *ComplianceHandler,
	validator auth.TokenValidator,
	checker middleware.PermissionChecker,
	resolveUserID func(context.Context) (uuid.UUID, error),
	entChecker middleware.EntitlementChecker,
) {
	permMw := func(perm string, h http.HandlerFunc) http.Handler {
		return middleware.Auth(validator,
			middleware.TenantContext(
				middleware.RequireEntitlement(entChecker, "ai.context_lake")(
					middleware.RequirePermission(checker, perm, resolveUserID)(
						http.HandlerFunc(h)))))
	}

	// Compliance status
	mux.Handle("GET /api/v1/organizations/{org_id}/compliance", permMw("ai.compliance.read", handler.GetComplianceReport))
	mux.Handle("GET /api/v1/organizations/{org_id}/compliance/history", permMw("ai.compliance.read", handler.GetComplianceHistory))
	mux.Handle("GET /api/v1/organizations/{org_id}/compliance/{category}", permMw("ai.compliance.read", handler.CheckCategory))

	// Jurisdiction rules reference (read-only)
	mux.Handle("GET /api/v1/organizations/{org_id}/jurisdiction-rules", permMw("ai.jurisdiction_rule.read", handler.ListJurisdictionRulesForOrg))
	mux.Handle("GET /api/v1/organizations/{org_id}/jurisdiction-rules/{category}", permMw("ai.jurisdiction_rule.read", handler.ListJurisdictionRulesByCategory))
}

// RegisterJurisdictionAdminRoutes registers platform admin jurisdiction rule endpoints.
func RegisterJurisdictionAdminRoutes(
	mux *http.ServeMux,
	handler *JurisdictionAdminHandler,
	validator auth.TokenValidator,
	checker middleware.PermissionChecker,
	resolveUserID func(context.Context) (uuid.UUID, error),
) {
	platformPermMw := func(perm string, h http.HandlerFunc) http.Handler {
		return middleware.Auth(validator, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, err := resolveUserID(r.Context())
			if err != nil {
				api.WriteError(w, api.NewUnauthenticatedError("could not resolve user"))
				return
			}
			allowed, err := checker.HasPermission(r.Context(), userID, uuid.Nil, perm)
			if err != nil {
				api.WriteError(w, api.NewInternalError(err))
				return
			}
			if !allowed {
				api.WriteError(w, api.NewForbiddenError("insufficient permissions"))
				return
			}
			h(w, r)
		}))
	}

	mux.Handle("POST /api/v1/admin/jurisdiction-rules", platformPermMw("admin.jurisdiction_rule.manage", handler.CreateRule))
	mux.Handle("GET /api/v1/admin/jurisdiction-rules", platformPermMw("admin.jurisdiction_rule.manage", handler.ListRules))
	mux.Handle("GET /api/v1/admin/jurisdiction-rules/{id}", platformPermMw("admin.jurisdiction_rule.manage", handler.GetRule))
	mux.Handle("PUT /api/v1/admin/jurisdiction-rules/{id}", platformPermMw("admin.jurisdiction_rule.manage", handler.UpdateRule))
	mux.Handle("DELETE /api/v1/admin/jurisdiction-rules/{id}", platformPermMw("admin.jurisdiction_rule.manage", handler.ExpireRule))
}
```

Ensure the necessary imports are added: `api`, `auth`, `middleware`, `uuid`.

- [ ] **Step 2: Verify compilation**

Run: `cd backend && go build ./internal/ai/...`

Expected: Compiles cleanly.

- [ ] **Step 3: Commit**

```bash
git add backend/internal/ai/routes.go
git commit -m "feat: register compliance and jurisdiction admin routes (issue #7)"
```

---

## Task 9: Wiring in main.go and Module Integration

**Files:**
- Modify: `backend/cmd/quorant-api/main.go`
- Modify: `backend/internal/gov/service.go`
- Modify: `backend/internal/fin/service.go`

- [ ] **Step 1: Add ComplianceResolver field to GovService**

In `backend/internal/gov/service.go`, add a `compliance` field:

```go
type GovService struct {
	violations ViolationRepository
	arb        ARBRepository
	ballots    BallotRepository
	meetings   MeetingRepository
	auditor    audit.Auditor
	publisher  queue.Publisher
	policy     ai.PolicyResolver
	compliance ai.ComplianceResolver
	logger     *slog.Logger
}

func NewGovService(
	violations ViolationRepository,
	arb ARBRepository,
	ballots BallotRepository,
	meetings MeetingRepository,
	auditor audit.Auditor,
	publisher queue.Publisher,
	policy ai.PolicyResolver,
	compliance ai.ComplianceResolver,
	logger *slog.Logger,
) *GovService {
	return &GovService{
		violations: violations,
		arb:        arb,
		ballots:    ballots,
		meetings:   meetings,
		auditor:    auditor,
		publisher:  publisher,
		policy:     policy,
		compliance: compliance,
		logger:     logger,
	}
}
```

- [ ] **Step 2: Add ComplianceResolver field to FinService**

In `backend/internal/fin/service.go`, add a `compliance` field:

```go
type FinService struct {
	assessments AssessmentRepository
	payments    PaymentRepository
	budgets     BudgetRepository
	funds       FundRepository
	collections CollectionRepository
	auditor     audit.Auditor
	publisher   queue.Publisher
	policy      ai.PolicyResolver
	compliance  ai.ComplianceResolver
	logger      *slog.Logger
}

func NewFinService(
	assessments AssessmentRepository,
	payments PaymentRepository,
	budgets BudgetRepository,
	funds FundRepository,
	collections CollectionRepository,
	auditor audit.Auditor,
	publisher queue.Publisher,
	policy ai.PolicyResolver,
	compliance ai.ComplianceResolver,
	logger *slog.Logger,
) *FinService {
	return &FinService{
		assessments: assessments,
		payments:    payments,
		budgets:     budgets,
		funds:       funds,
		collections: collections,
		auditor:     auditor,
		publisher:   publisher,
		policy:      policy,
		compliance:  compliance,
		logger:      logger,
	}
}
```

- [ ] **Step 3: Wire compliance engine in main.go**

Add after the existing AI module wiring block in `backend/cmd/quorant-api/main.go`:

```go
// Compliance engine
jurisdictionRuleRepo := ai.NewPostgresJurisdictionRuleRepository(pool)
complianceCheckRepo := ai.NewPostgresComplianceCheckRepository(pool)
complianceService := ai.NewComplianceService(jurisdictionRuleRepo, complianceCheckRepo, orgRepo, logger)

// Register category evaluators
complianceService.RegisterEvaluator("meeting_notice", ai.EvaluateMeetingNotice)
complianceService.RegisterEvaluator("fine_limits", ai.EvaluateFineLimits)
complianceService.RegisterEvaluator("reserve_study", ai.EvaluateReserveStudy)
complianceService.RegisterEvaluator("website_requirements", ai.EvaluateWebsiteRequirements)
complianceService.RegisterEvaluator("record_retention", ai.EvaluateRecordRetention)
complianceService.RegisterEvaluator("voting_rules", ai.EvaluateVotingRules)
complianceService.RegisterEvaluator("estoppel", ai.EvaluateEstoppel)

// Register compliance routes
complianceHandler := ai.NewComplianceHandler(complianceService, complianceCheckRepo, logger)
ai.RegisterComplianceRoutes(mux, complianceHandler, tokenValidator, permChecker, resolveUserID, entitlementChecker)

jurisdictionAdminHandler := ai.NewJurisdictionAdminHandler(jurisdictionRuleRepo, logger)
ai.RegisterJurisdictionAdminRoutes(mux, jurisdictionAdminHandler, tokenValidator, permChecker, resolveUserID)
```

Update the `govService` and `finService` construction to include `complianceService`:

```go
govService := gov.NewGovService(violationRepo, arbRepo, ballotRepo, meetingRepo, auditor, outboxPublisher, policyResolver, complianceService, logger)
finService := fin.NewFinService(assessmentRepo, paymentRepo, budgetRepo, fundRepo, collectionRepo, auditor, outboxPublisher, policyResolver, complianceService, logger)
```

- [ ] **Step 4: Verify compilation**

Run: `cd backend && go build ./cmd/quorant-api/...`

Expected: Compiles cleanly. All tests still pass.

- [ ] **Step 5: Run full test suite**

Run: `cd backend && go test ./... -v -count=1`

Expected: All existing + new tests pass.

- [ ] **Step 6: Commit**

```bash
git add backend/cmd/quorant-api/main.go backend/internal/gov/service.go backend/internal/fin/service.go
git commit -m "feat: wire ComplianceResolver into gov/fin modules and main.go (issue #7)"
```

---

## Task 10: Seed Data

**Files:**
- Create: `backend/seeds/jurisdiction_rules_seed.sql`

- [ ] **Step 1: Write the seed data SQL**

```sql
-- backend/seeds/jurisdiction_rules_seed.sql
-- Jurisdiction rules seed data for FL, CA, TX, AZ, CO.
-- Source: current state statutes as of 2025.

-- Clear existing seed data (idempotent)
DELETE FROM jurisdiction_rules WHERE created_by IS NULL;

-- ============================================================
-- FLORIDA
-- ============================================================

-- Meeting Notice
INSERT INTO jurisdiction_rules (jurisdiction, rule_category, rule_key, value_type, value, statute_reference, effective_date, notes)
VALUES
('FL', 'meeting_notice', 'board_meeting_notice_days', 'integer', '2', 'FS 720.303(2)(c)', '2024-01-01', '48 hours notice required'),
('FL', 'meeting_notice', 'annual_meeting_notice_days', 'integer', '14', 'FS 720.306(5)', '2024-01-01', '14 days mailed or posted notice'),
('FL', 'meeting_notice', 'special_meeting_notice_days', 'integer', '7', 'FS 720.306(5)', '2024-01-01', NULL),
('FL', 'meeting_notice', 'emergency_meeting_notice_days', 'integer', '0', 'FS 720.303(2)(c)', '2024-01-01', 'Reasonable notice; no minimum for emergencies');

-- Fine Limits
INSERT INTO jurisdiction_rules (jurisdiction, rule_category, rule_key, value_type, value, statute_reference, effective_date, notes)
VALUES
('FL', 'fine_limits', 'hearing_required', 'boolean', 'true', 'FS 720.305(2)', '2024-01-01', 'Committee hearing required before fines'),
('FL', 'fine_limits', 'daily_aggregate_cap_cents', 'integer', '10000', 'FS 720.305(2)(b)', '2024-01-01', '$100/day aggregate cap'),
('FL', 'fine_limits', 'per_violation_cap_cents', 'text', '"none"', 'FS 720.305(2)', '2024-01-01', 'No statutory per-violation cap for HOAs');

-- Reserve Study
INSERT INTO jurisdiction_rules (jurisdiction, rule_category, rule_key, value_type, value, statute_reference, effective_date, notes)
VALUES
('FL', 'reserve_study', 'sirs_required', 'boolean', 'true', 'SB 4-D (2022)', '2025-01-01', 'Structural Integrity Reserve Study required'),
('FL', 'reserve_study', 'sirs_interval_years', 'integer', '10', 'SB 4-D (2022)', '2025-01-01', NULL),
('FL', 'reserve_study', 'sirs_min_stories', 'integer', '3', 'SB 4-D (2022)', '2025-01-01', 'Applies to buildings 3+ stories'),
('FL', 'reserve_study', 'waiver_allowed', 'boolean', 'false', 'SB 4-D (2022)', '2025-01-01', 'Reserve waivers eliminated for SIRS components');

-- Website Requirements
INSERT INTO jurisdiction_rules (jurisdiction, rule_category, rule_key, value_type, value, statute_reference, effective_date, notes)
VALUES
('FL', 'website_requirements', 'required_for_unit_count', 'integer', '100', 'FL HB 1203 (2025)', '2025-01-01', 'Website required for 100+ unit communities');

-- Record Retention
INSERT INTO jurisdiction_rules (jurisdiction, rule_category, rule_key, value_type, value, statute_reference, effective_date, notes)
VALUES
('FL', 'record_retention', 'financial_records_years', 'integer', '7', 'FS 720.303(4)', '2024-01-01', NULL),
('FL', 'record_retention', 'governing_docs_retention', 'text', '"permanent"', 'FS 720.303(4)', '2024-01-01', NULL),
('FL', 'record_retention', 'meeting_minutes_years', 'integer', '7', 'FS 720.303(4)', '2024-01-01', NULL);

-- Voting Rules
INSERT INTO jurisdiction_rules (jurisdiction, rule_category, rule_key, value_type, value, statute_reference, effective_date, notes)
VALUES
('FL', 'voting_rules', 'proxy_allowed', 'boolean', 'true', 'FS 720.306(8)', '2024-01-01', NULL),
('FL', 'voting_rules', 'electronic_voting_allowed', 'boolean', 'true', 'FS 720.317', '2024-01-01', 'With proper authorization');

-- Estoppel
INSERT INTO jurisdiction_rules (jurisdiction, rule_category, rule_key, value_type, value, statute_reference, effective_date, notes)
VALUES
('FL', 'estoppel', 'turnaround_business_days', 'integer', '10', 'FS 720.30851', '2024-01-01', NULL),
('FL', 'estoppel', 'fee_cap_cents', 'integer', '25000', 'FS 720.30851', '2024-01-01', '$250 statutory cap');

-- ============================================================
-- CALIFORNIA
-- ============================================================

INSERT INTO jurisdiction_rules (jurisdiction, rule_category, rule_key, value_type, value, statute_reference, effective_date, notes)
VALUES
('CA', 'meeting_notice', 'board_meeting_notice_days', 'integer', '4', 'Civil Code 4920(a)', '2024-01-01', NULL),
('CA', 'meeting_notice', 'annual_meeting_notice_days', 'integer', '30', 'Civil Code 5115(b)', '2024-01-01', '10-90 day range; 30 typical'),
('CA', 'meeting_notice', 'special_meeting_notice_days', 'integer', '10', 'Civil Code 5115(b)', '2024-01-01', NULL),
('CA', 'meeting_notice', 'emergency_meeting_notice_days', 'integer', '0', 'Civil Code 4923', '2024-01-01', NULL),
('CA', 'fine_limits', 'hearing_required', 'boolean', 'true', 'Civil Code 5855', '2024-01-01', 'IDR/ADR process before fine enforcement'),
('CA', 'fine_limits', 'per_violation_cap_cents', 'text', '"reasonable"', 'Civil Code 5850', '2024-01-01', 'Must be reasonable; no specific cap'),
('CA', 'reserve_study', 'sirs_required', 'boolean', 'true', 'Civil Code 5550', '2024-01-01', 'Reserve study every 3 years'),
('CA', 'reserve_study', 'sirs_interval_years', 'integer', '3', 'Civil Code 5550', '2024-01-01', 'With annual review update'),
('CA', 'record_retention', 'financial_records_years', 'integer', '4', 'Civil Code 5200', '2024-01-01', NULL),
('CA', 'record_retention', 'governing_docs_retention', 'text', '"permanent"', 'Civil Code 5200', '2024-01-01', NULL),
('CA', 'voting_rules', 'proxy_allowed', 'boolean', 'true', 'Civil Code 5130', '2024-01-01', NULL),
('CA', 'estoppel', 'turnaround_business_days', 'integer', '10', 'Civil Code 4530', '2024-01-01', NULL);

-- ============================================================
-- TEXAS
-- ============================================================

INSERT INTO jurisdiction_rules (jurisdiction, rule_category, rule_key, value_type, value, statute_reference, effective_date, notes)
VALUES
('TX', 'meeting_notice', 'board_meeting_notice_days', 'integer', '3', 'Tex. Prop. Code 209.0051', '2024-01-01', '72 hours notice'),
('TX', 'meeting_notice', 'annual_meeting_notice_days', 'integer', '10', 'Tex. Prop. Code 209.0051', '2024-01-01', '10-60 day range'),
('TX', 'fine_limits', 'hearing_required', 'boolean', 'true', 'Tex. Prop. Code 209.007', '2024-01-01', 'Notice and hearing required'),
('TX', 'record_retention', 'financial_records_years', 'integer', '4', 'Tex. Prop. Code 209.005', '2024-01-01', NULL),
('TX', 'record_retention', 'election_records_years', 'integer', '7', 'Tex. Prop. Code 209.0058', '2024-01-01', NULL),
('TX', 'record_retention', 'governing_docs_retention', 'text', '"permanent"', 'Tex. Prop. Code 209.005', '2024-01-01', NULL),
('TX', 'voting_rules', 'proxy_allowed', 'boolean', 'true', 'Tex. Prop. Code 209.00592', '2024-01-01', NULL);

-- ============================================================
-- ARIZONA
-- ============================================================

INSERT INTO jurisdiction_rules (jurisdiction, rule_category, rule_key, value_type, value, statute_reference, effective_date, notes)
VALUES
('AZ', 'meeting_notice', 'board_meeting_notice_days', 'integer', '2', 'ARS 33-1804', '2024-01-01', '48 hours notice'),
('AZ', 'meeting_notice', 'annual_meeting_notice_days', 'integer', '10', 'ARS 33-1804', '2024-01-01', '10-50 day range'),
('AZ', 'fine_limits', 'per_violation_cap_cents', 'integer', '5000', 'ARS 33-1803', '2024-01-01', '$50 per violation cap'),
('AZ', 'fine_limits', 'hearing_required', 'boolean', 'false', 'ARS 33-1803', '2024-01-01', 'Written notice required; no committee hearing mandate'),
('AZ', 'record_retention', 'financial_records_years', 'integer', '7', 'ARS 33-1805', '2024-01-01', NULL),
('AZ', 'record_retention', 'governing_docs_retention', 'text', '"permanent"', 'ARS 33-1805', '2024-01-01', NULL),
('AZ', 'voting_rules', 'proxy_allowed', 'boolean', 'true', 'ARS 33-1812', '2024-01-01', NULL),
('AZ', 'voting_rules', 'quorum_percent', 'decimal', '25.0', 'ARS 33-1803', '2024-01-01', '25% quorum for member meetings'),
('AZ', 'estoppel', 'turnaround_business_days', 'integer', '10', 'ARS 33-1806', '2024-01-01', NULL),
('AZ', 'estoppel', 'fee_cap_cents', 'integer', '40000', 'ARS 33-1806', '2024-01-01', '$400 cap');

-- ============================================================
-- COLORADO
-- ============================================================

INSERT INTO jurisdiction_rules (jurisdiction, rule_category, rule_key, value_type, value, statute_reference, effective_date, notes)
VALUES
('CO', 'meeting_notice', 'board_meeting_notice_days', 'integer', '1', 'CRS 38-33.3-308', '2024-01-01', '24 hours notice'),
('CO', 'meeting_notice', 'annual_meeting_notice_days', 'integer', '10', 'CRS 38-33.3-308', '2024-01-01', '10-50 day range'),
('CO', 'fine_limits', 'per_violation_cap_cents', 'integer', '50000', 'CCIOA 38-33.3-315', '2024-01-01', '$500 per violation typical cap'),
('CO', 'fine_limits', 'hearing_required', 'boolean', 'true', 'CCIOA 38-33.3-315', '2024-01-01', 'Written notice and hearing required'),
('CO', 'record_retention', 'financial_records_years', 'integer', '7', 'CRS 38-33.3-317', '2024-01-01', NULL),
('CO', 'record_retention', 'governing_docs_retention', 'text', '"permanent"', 'CRS 38-33.3-317', '2024-01-01', NULL),
('CO', 'voting_rules', 'proxy_allowed', 'boolean', 'true', 'CRS 38-33.3-310', '2024-01-01', NULL);
```

- [ ] **Step 2: Verify seed data loads**

Run: `cd backend && psql -U quorant -d quorant_dev -f seeds/jurisdiction_rules_seed.sql`

Expected: INSERT statements succeed. Verify with:
`SELECT jurisdiction, COUNT(*) FROM jurisdiction_rules GROUP BY jurisdiction;`

Expected output: FL ~20, CA ~12, TX ~7, AZ ~10, CO ~7.

- [ ] **Step 3: Commit**

```bash
git add backend/seeds/jurisdiction_rules_seed.sql
git commit -m "feat: add jurisdiction rules seed data for FL, CA, TX, AZ, CO (issue #7)"
```

---

## Task 11: Event Publishing from Admin Handlers

**Files:**
- Modify: `backend/internal/ai/handler_jurisdiction_admin.go`

- [ ] **Step 1: Add publisher dependency to JurisdictionAdminHandler**

Update the handler struct and constructor to accept a `queue.Publisher`:

```go
type JurisdictionAdminHandler struct {
	rules     JurisdictionRuleRepository
	publisher queue.Publisher
	logger    *slog.Logger
}

func NewJurisdictionAdminHandler(rules JurisdictionRuleRepository, publisher queue.Publisher, logger *slog.Logger) *JurisdictionAdminHandler {
	return &JurisdictionAdminHandler{rules: rules, publisher: publisher, logger: logger}
}
```

- [ ] **Step 2: Publish events after Create and Update**

At the end of `CreateRule`, after successful creation:

```go
payload, _ := json.Marshal(map[string]any{
	"rule_id":       created.ID,
	"jurisdiction":  created.Jurisdiction,
	"rule_category": created.RuleCategory,
	"rule_key":      created.RuleKey,
	"effective_date": created.EffectiveDate.Format("2006-01-02"),
	"changed_by":    userID,
})
event := queue.NewBaseEvent("quorant.ai.JurisdictionRuleCreated", "jurisdiction_rule", created.ID, uuid.Nil, payload)
if err := h.publisher.Publish(r.Context(), event); err != nil {
	h.logger.Error("failed to publish JurisdictionRuleCreated", "error", err)
}
```

At the end of `UpdateRule`, after creating the replacement:

```go
payload, _ := json.Marshal(map[string]any{
	"rule_id":       created.ID,
	"jurisdiction":  created.Jurisdiction,
	"rule_category": created.RuleCategory,
	"rule_key":      created.RuleKey,
	"effective_date": created.EffectiveDate.Format("2006-01-02"),
	"changed_by":    userID,
})
event := queue.NewBaseEvent("quorant.ai.JurisdictionRuleUpdated", "jurisdiction_rule", created.ID, uuid.Nil, payload)
if err := h.publisher.Publish(r.Context(), event); err != nil {
	h.logger.Error("failed to publish JurisdictionRuleUpdated", "error", err)
}
```

- [ ] **Step 3: Update main.go wiring to pass publisher**

In `main.go`, update the admin handler construction:

```go
jurisdictionAdminHandler := ai.NewJurisdictionAdminHandler(jurisdictionRuleRepo, outboxPublisher, logger)
```

- [ ] **Step 4: Update admin handler test to include mock publisher**

Update `setupAdminTestServer` to include `queue.NewInMemoryPublisher()`.

- [ ] **Step 5: Verify compilation and tests pass**

Run: `cd backend && go test ./internal/ai/... -run TestAdmin -v`

Expected: All admin handler tests PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/ai/handler_jurisdiction_admin.go backend/cmd/quorant-api/main.go
git commit -m "feat: publish NATS events on jurisdiction rule create/update (issue #7)"
```

---

## Task 12 (Follow-up): Compliance Worker

> **Note:** The compliance worker (rule change re-evaluation + daily cron proactive alerts) runs in `quorant-worker` and is a follow-up task. It subscribes to `JurisdictionRuleCreated` / `JurisdictionRuleUpdated` events, re-evaluates affected orgs, and creates tasks + publishes `ComplianceAlertRaised` events. This should be implemented after the core engine is verified working.

---

## Task 13: Final Verification

- [ ] **Step 1: Run full test suite**

Run: `cd backend && go test ./... -count=1 -v 2>&1 | tail -50`

Expected: All tests pass.

- [ ] **Step 2: Run integration tests**

Run: `cd backend && go test -tags=integration ./internal/ai/... -v`

Expected: All repository integration tests pass.

- [ ] **Step 3: Build both binaries**

Run: `cd backend && go build ./cmd/quorant-api/... && go build ./cmd/quorant-worker/...`

Expected: Both compile cleanly.

- [ ] **Step 4: Commit all remaining changes**

```bash
git add -A
git commit -m "feat: complete jurisdiction compliance rules engine (closes #7)"
```
