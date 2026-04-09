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

func setupComplianceCheckTestDB(t *testing.T) *pgxpool.Pool {
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

// createTestOrgForCompliance inserts a test organization and returns its ID.
func createTestOrgForCompliance(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var orgID uuid.UUID
	err := pool.QueryRow(ctx,
		`INSERT INTO organizations (type, name, slug, path, settings)
		 VALUES ('hoa', $1, $2, $3, '{}') RETURNING id`,
		"Test HOA "+uuid.New().String(),
		"test-"+uuid.New().String(),
		"test_"+uuid.New().String(),
	).Scan(&orgID)
	require.NoError(t, err, "create test org")
	t.Cleanup(func() {
		pool.Exec(context.Background(), "DELETE FROM organizations WHERE id = $1", orgID)
	})
	return orgID
}

// createTestRuleForCompliance inserts a jurisdiction rule and returns its ID.
func createTestRuleForCompliance(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	ruleRepo := ai.NewPostgresJurisdictionRuleRepository(pool)
	ctx := context.Background()
	rule := &ai.JurisdictionRule{
		Jurisdiction:  "CA",
		RuleCategory:  "finance",
		RuleKey:       "test_rule_" + uuid.New().String(),
		ValueType:     "decimal",
		Value:         json.RawMessage(`0.10`),
		EffectiveDate: time.Now().UTC().AddDate(-1, 0, 0),
	}
	created, err := ruleRepo.Create(ctx, rule)
	require.NoError(t, err, "create test rule")
	return created.ID
}

func TestPostgresComplianceCheckRepository_Create(t *testing.T) {
	pool := setupComplianceCheckTestDB(t)
	checkRepo := ai.NewPostgresComplianceCheckRepository(pool)
	ctx := context.Background()

	orgID := createTestOrgForCompliance(t, pool)
	ruleID := createTestRuleForCompliance(t, pool)

	check := &ai.ComplianceCheck{
		OrgID:   orgID,
		RuleID:  ruleID,
		Status:  "compliant",
		Details: json.RawMessage(`{"note": "within limit"}`),
	}

	created, err := checkRepo.Create(ctx, check)
	require.NoError(t, err)
	require.NotNil(t, created)
	assert.NotEqual(t, uuid.Nil, created.ID)
	assert.Equal(t, orgID, created.OrgID)
	assert.Equal(t, ruleID, created.RuleID)
	assert.Equal(t, "compliant", created.Status)
	assert.JSONEq(t, `{"note": "within limit"}`, string(created.Details))
	assert.NotZero(t, created.CheckedAt)
	assert.Nil(t, created.ResolvedAt)
	assert.Empty(t, created.ResolutionNotes)
}

func TestPostgresComplianceCheckRepository_Resolve(t *testing.T) {
	pool := setupComplianceCheckTestDB(t)
	checkRepo := ai.NewPostgresComplianceCheckRepository(pool)
	ctx := context.Background()

	orgID := createTestOrgForCompliance(t, pool)
	ruleID := createTestRuleForCompliance(t, pool)

	check := &ai.ComplianceCheck{
		OrgID:  orgID,
		RuleID: ruleID,
		Status: "non_compliant",
	}

	created, err := checkRepo.Create(ctx, check)
	require.NoError(t, err)
	require.NotNil(t, created)
	assert.Nil(t, created.ResolvedAt)

	// Resolve the check
	resolved, err := checkRepo.Resolve(ctx, created.ID, "Corrected by board action on 2026-04-01")
	require.NoError(t, err)
	require.NotNil(t, resolved)
	assert.Equal(t, created.ID, resolved.ID)
	assert.NotNil(t, resolved.ResolvedAt, "resolved_at should be set after resolution")
	assert.Equal(t, "Corrected by board action on 2026-04-01", resolved.ResolutionNotes)
}
