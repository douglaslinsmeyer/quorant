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
		Jurisdiction:     "CA",
		RuleCategory:     "finance",
		RuleKey:          "max_late_fee_pct",
		ValueType:        "decimal",
		Value:            json.RawMessage(`10.0`),
		StatuteReference: "Cal. Corp. Code § 7341",
		EffectiveDate:    time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		Notes:            "Maximum late fee percentage",
	}

	created, err := repo.Create(ctx, rule)
	require.NoError(t, err)
	require.NotNil(t, created)
	assert.NotEqual(t, uuid.Nil, created.ID)
	assert.Equal(t, "CA", created.Jurisdiction)
	assert.Equal(t, "finance", created.RuleCategory)
	assert.Equal(t, "max_late_fee_pct", created.RuleKey)
	assert.Equal(t, "decimal", created.ValueType)
	assert.JSONEq(t, `10.0`, string(created.Value))
	assert.Equal(t, "Cal. Corp. Code § 7341", created.StatuteReference)
	assert.Equal(t, "Maximum late fee percentage", created.Notes)
	assert.Nil(t, created.ExpirationDate)
	assert.Nil(t, created.SourceDocID)
	assert.Nil(t, created.CreatedBy)
	assert.NotZero(t, created.CreatedAt)
	assert.NotZero(t, created.UpdatedAt)
}

func TestPostgresJurisdictionRuleRepository_GetActiveRule(t *testing.T) {
	pool := setupJurisdictionRuleTestDB(t)
	repo := ai.NewPostgresJurisdictionRuleRepository(pool)
	ctx := context.Background()

	// Insert an active rule (effective today - 1 year)
	activeRule := &ai.JurisdictionRule{
		Jurisdiction:     "CA",
		RuleCategory:     "quorum",
		RuleKey:          "min_quorum_pct",
		ValueType:        "decimal",
		Value:            json.RawMessage(`0.25`),
		StatuteReference: "Cal. Corp. Code § 7512",
		EffectiveDate:    time.Now().UTC().AddDate(-1, 0, 0),
	}
	_, err := repo.Create(ctx, activeRule)
	require.NoError(t, err)

	// Insert a future rule (effective 30 days from now — not yet active)
	futureRule := &ai.JurisdictionRule{
		Jurisdiction:     "CA",
		RuleCategory:     "quorum",
		RuleKey:          "min_quorum_pct",
		ValueType:        "decimal",
		Value:            json.RawMessage(`0.30`),
		StatuteReference: "Cal. Corp. Code § 7512 (amended)",
		EffectiveDate:    time.Now().UTC().AddDate(0, 0, 30),
	}
	_, err = repo.Create(ctx, futureRule)
	require.NoError(t, err)

	// Only the active rule should be returned
	found, err := repo.GetActiveRule(ctx, "CA", "quorum", "min_quorum_pct")
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.JSONEq(t, `0.25`, string(found.Value))

	// Non-existent rule should return nil without error
	notFound, err := repo.GetActiveRule(ctx, "CA", "quorum", "nonexistent_key")
	require.NoError(t, err)
	assert.Nil(t, notFound)
}

func TestPostgresJurisdictionRuleRepository_ListActiveRules(t *testing.T) {
	pool := setupJurisdictionRuleTestDB(t)
	repo := ai.NewPostgresJurisdictionRuleRepository(pool)
	ctx := context.Background()

	pastDate := time.Now().UTC().AddDate(-1, 0, 0)

	rule1 := &ai.JurisdictionRule{
		Jurisdiction:  "TX",
		RuleCategory:  "assessment",
		RuleKey:       "max_increase_pct",
		ValueType:     "decimal",
		Value:         json.RawMessage(`0.15`),
		EffectiveDate: pastDate,
	}
	rule2 := &ai.JurisdictionRule{
		Jurisdiction:  "TX",
		RuleCategory:  "assessment",
		RuleKey:       "notice_days",
		ValueType:     "integer",
		Value:         json.RawMessage(`30`),
		EffectiveDate: pastDate,
	}
	// Rule in different category — should not be returned
	rule3 := &ai.JurisdictionRule{
		Jurisdiction:  "TX",
		RuleCategory:  "quorum",
		RuleKey:       "min_quorum_pct",
		ValueType:     "decimal",
		Value:         json.RawMessage(`0.25`),
		EffectiveDate: pastDate,
	}

	_, err := repo.Create(ctx, rule1)
	require.NoError(t, err)
	_, err = repo.Create(ctx, rule2)
	require.NoError(t, err)
	_, err = repo.Create(ctx, rule3)
	require.NoError(t, err)

	results, err := repo.ListActiveRules(ctx, "TX", "assessment")
	require.NoError(t, err)
	assert.Len(t, results, 2)

	keys := make([]string, len(results))
	for i, r := range results {
		keys[i] = r.RuleKey
	}
	assert.Contains(t, keys, "max_increase_pct")
	assert.Contains(t, keys, "notice_days")
}

func TestPostgresJurisdictionRuleRepository_ListUpcomingRules(t *testing.T) {
	pool := setupJurisdictionRuleTestDB(t)
	repo := ai.NewPostgresJurisdictionRuleRepository(pool)
	ctx := context.Background()

	// Rule effective 10 days from now — within a 30-day window
	upcoming := &ai.JurisdictionRule{
		Jurisdiction:  "FL",
		RuleCategory:  "finance",
		RuleKey:       "reserve_fund_pct",
		ValueType:     "decimal",
		Value:         json.RawMessage(`0.10`),
		EffectiveDate: time.Now().UTC().AddDate(0, 0, 10),
	}
	created, err := repo.Create(ctx, upcoming)
	require.NoError(t, err)

	// Rule effective 60 days from now — outside the 30-day window
	farFuture := &ai.JurisdictionRule{
		Jurisdiction:  "FL",
		RuleCategory:  "finance",
		RuleKey:       "late_fee_cap",
		ValueType:     "decimal",
		Value:         json.RawMessage(`0.05`),
		EffectiveDate: time.Now().UTC().AddDate(0, 0, 60),
	}
	_, err = repo.Create(ctx, farFuture)
	require.NoError(t, err)

	results, err := repo.ListUpcomingRules(ctx, 30)
	require.NoError(t, err)
	require.NotEmpty(t, results)

	ids := make([]uuid.UUID, len(results))
	for i, r := range results {
		ids[i] = r.ID
	}
	assert.Contains(t, ids, created.ID, "rule effective in 10 days should be found within 30-day window")
}

func TestPostgresJurisdictionRuleRepository_UniqueConstraint(t *testing.T) {
	pool := setupJurisdictionRuleTestDB(t)
	repo := ai.NewPostgresJurisdictionRuleRepository(pool)
	ctx := context.Background()

	rule := &ai.JurisdictionRule{
		Jurisdiction:  "NY",
		RuleCategory:  "governance",
		RuleKey:       "board_term_years",
		ValueType:     "integer",
		Value:         json.RawMessage(`2`),
		EffectiveDate: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	_, err := repo.Create(ctx, rule)
	require.NoError(t, err)

	// Duplicate insert with same jurisdiction/category/key/effective_date should fail
	duplicate := &ai.JurisdictionRule{
		Jurisdiction:  "NY",
		RuleCategory:  "governance",
		RuleKey:       "board_term_years",
		ValueType:     "integer",
		Value:         json.RawMessage(`3`),
		EffectiveDate: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	_, err = repo.Create(ctx, duplicate)
	assert.Error(t, err, "duplicate jurisdiction/category/key/effective_date should fail")
}
