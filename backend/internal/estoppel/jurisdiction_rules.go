package estoppel

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// JurisdictionRulesRepository retrieves platform-managed statutory rules for a
// given jurisdiction (two-letter US state code). The underlying table has no
// RLS — it is not tenant-scoped.
type JurisdictionRulesRepository interface {
	GetEstoppelRules(ctx context.Context, jurisdiction string) (*EstoppelRules, error)
}

// PostgresJurisdictionRulesRepository is the production implementation backed
// by the jurisdiction_rules table.
type PostgresJurisdictionRulesRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresJurisdictionRulesRepository creates a new
// PostgresJurisdictionRulesRepository using the given connection pool.
func NewPostgresJurisdictionRulesRepository(pool *pgxpool.Pool) *PostgresJurisdictionRulesRepository {
	return &PostgresJurisdictionRulesRepository{pool: pool}
}

const getEstoppelRulesQuery = `
SELECT value
FROM jurisdiction_rules
WHERE jurisdiction = $1
  AND rule_category = 'estoppel'
  AND rule_key      = 'estoppel_rules'
  AND (expiration_date IS NULL OR expiration_date > now())
ORDER BY effective_date DESC
LIMIT 1
`

// GetEstoppelRules queries the jurisdiction_rules table for the most recent
// active estoppel rule set for the given jurisdiction. Returns nil, nil when no
// row exists (unknown or unseeded jurisdiction).
func (r *PostgresJurisdictionRulesRepository) GetEstoppelRules(ctx context.Context, jurisdiction string) (*EstoppelRules, error) {
	var raw json.RawMessage
	err := r.pool.QueryRow(ctx, getEstoppelRulesQuery, jurisdiction).Scan(&raw)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("querying estoppel rules for jurisdiction %q: %w", jurisdiction, err)
	}

	var rules EstoppelRules
	if err := json.Unmarshal(raw, &rules); err != nil {
		return nil, fmt.Errorf("deserializing estoppel rules for jurisdiction %q: %w", jurisdiction, err)
	}
	return &rules, nil
}

// ---------------------------------------------------------------------------
// MockJurisdictionRulesRepo — exported in-process test double
// ---------------------------------------------------------------------------

// MockJurisdictionRulesRepo is an exported test double for
// JurisdictionRulesRepository. It returns pre-loaded rules keyed by
// jurisdiction string. Exported so that test packages can reference the type
// directly for interface compliance checks.
type MockJurisdictionRulesRepo struct {
	rules map[string]*EstoppelRules
}

// NewMockJurisdictionRulesRepo creates a MockJurisdictionRulesRepo pre-loaded
// with the provided rules map (may be nil).
func NewMockJurisdictionRulesRepo(rules map[string]*EstoppelRules) *MockJurisdictionRulesRepo {
	if rules == nil {
		rules = make(map[string]*EstoppelRules)
	}
	return &MockJurisdictionRulesRepo{rules: rules}
}

// GetEstoppelRules returns the pre-loaded rules for the given jurisdiction, or
// nil, nil if no rules are configured.
func (m *MockJurisdictionRulesRepo) GetEstoppelRules(_ context.Context, jurisdiction string) (*EstoppelRules, error) {
	rules, ok := m.rules[jurisdiction]
	if !ok {
		return nil, nil
	}
	return rules, nil
}

// compile-time interface check
var _ JurisdictionRulesRepository = (*PostgresJurisdictionRulesRepository)(nil)
var _ JurisdictionRulesRepository = (*MockJurisdictionRulesRepo)(nil)
