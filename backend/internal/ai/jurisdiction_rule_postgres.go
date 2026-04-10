package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const jurisdictionRuleCols = "id, jurisdiction, rule_category, rule_key, value_type, value, statute_reference, effective_date, expiration_date, notes, source_doc_id, created_at, updated_at, created_by"

// PostgresJurisdictionRuleRepository implements JurisdictionRuleRepository using pgxpool.
type PostgresJurisdictionRuleRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresJurisdictionRuleRepository constructs a PostgresJurisdictionRuleRepository backed by pool.
func NewPostgresJurisdictionRuleRepository(pool *pgxpool.Pool) *PostgresJurisdictionRuleRepository {
	return &PostgresJurisdictionRuleRepository{pool: pool}
}

// Create inserts a new jurisdiction rule and returns the persisted record.
func (r *PostgresJurisdictionRuleRepository) Create(ctx context.Context, rule *JurisdictionRule) (*JurisdictionRule, error) {
	const q = `
		INSERT INTO jurisdiction_rules (
			jurisdiction, rule_category, rule_key, value_type, value,
			statute_reference, effective_date, expiration_date,
			notes, source_doc_id, created_by
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8,
			$9, $10, $11
		)
		RETURNING ` + jurisdictionRuleCols

	row := r.pool.QueryRow(ctx, q,
		rule.Jurisdiction,
		rule.RuleCategory,
		rule.RuleKey,
		rule.ValueType,
		marshalRawOrNull(rule.Value),
		rule.StatuteReference,
		utcMidnight(rule.EffectiveDate),
		rule.ExpirationDate,
		rule.Notes,
		rule.SourceDocID,
		rule.CreatedBy,
	)
	result, err := scanJurisdictionRule(row)
	if err != nil {
		return nil, fmt.Errorf("ai: Create: %w", err)
	}
	return result, nil
}

// Update persists changes to an existing jurisdiction rule and returns the updated row.
// Only updates mutable fields: value, statute_reference, expiration_date, notes, source_doc_id, updated_at.
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
		marshalRawOrNull(rule.Value),
		rule.StatuteReference,
		rule.ExpirationDate,
		rule.Notes,
		rule.SourceDocID,
		rule.ID,
	)
	result, err := scanJurisdictionRule(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("ai: Update: %s not found", rule.ID)
	}
	if err != nil {
		return nil, fmt.Errorf("ai: Update: %w", err)
	}
	return result, nil
}

// FindByID returns the jurisdiction rule with the given ID, or nil if not found.
func (r *PostgresJurisdictionRuleRepository) FindByID(ctx context.Context, id uuid.UUID) (*JurisdictionRule, error) {
	const q = `
		SELECT ` + jurisdictionRuleCols + `
		FROM jurisdiction_rules
		WHERE id = $1`

	row := r.pool.QueryRow(ctx, q, id)
	result, err := scanJurisdictionRule(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("ai: FindByID: %w", err)
	}
	return result, nil
}

// GetActiveRule returns the most recently effective active rule for the given jurisdiction/category/key,
// or nil if none exists.
func (r *PostgresJurisdictionRuleRepository) GetActiveRule(ctx context.Context, jurisdiction, category, key string) (*JurisdictionRule, error) {
	const q = `
		SELECT ` + jurisdictionRuleCols + `
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
		return nil, fmt.Errorf("ai: GetActiveRule: %w", err)
	}
	return result, nil
}

// ListActiveRules returns all currently active rules for the given jurisdiction and category.
func (r *PostgresJurisdictionRuleRepository) ListActiveRules(ctx context.Context, jurisdiction, category string) ([]JurisdictionRule, error) {
	const q = `
		SELECT ` + jurisdictionRuleCols + `
		FROM jurisdiction_rules
		WHERE jurisdiction = $1
		  AND rule_category = $2
		  AND effective_date <= CURRENT_DATE
		  AND (expiration_date IS NULL OR expiration_date > CURRENT_DATE)
		ORDER BY rule_key, effective_date DESC`

	rows, err := r.pool.Query(ctx, q, jurisdiction, category)
	if err != nil {
		return nil, fmt.Errorf("ai: ListActiveRules: %w", err)
	}
	defer rows.Close()

	return collectJurisdictionRules(rows, "ListActiveRules")
}

// ListActiveRulesByJurisdiction returns all currently active rules for the given jurisdiction.
func (r *PostgresJurisdictionRuleRepository) ListActiveRulesByJurisdiction(ctx context.Context, jurisdiction string) ([]JurisdictionRule, error) {
	const q = `
		SELECT ` + jurisdictionRuleCols + `
		FROM jurisdiction_rules
		WHERE jurisdiction = $1
		  AND effective_date <= CURRENT_DATE
		  AND (expiration_date IS NULL OR expiration_date > CURRENT_DATE)
		ORDER BY rule_category, rule_key, effective_date DESC`

	rows, err := r.pool.Query(ctx, q, jurisdiction)
	if err != nil {
		return nil, fmt.Errorf("ai: ListActiveRulesByJurisdiction: %w", err)
	}
	defer rows.Close()

	return collectJurisdictionRules(rows, "ListActiveRulesByJurisdiction")
}

// ListAllRules returns a cursor-paginated list of all rules for the given jurisdiction.
// afterID is the last ID from the previous page (exclusive). hasMore is true if there are more rows.
func (r *PostgresJurisdictionRuleRepository) ListAllRules(ctx context.Context, jurisdiction string, limit int, afterID *uuid.UUID) ([]JurisdictionRule, bool, error) {
	const q = `
		SELECT ` + jurisdictionRuleCols + `
		FROM jurisdiction_rules
		WHERE jurisdiction = $1
		  AND ($3::uuid IS NULL OR id < $3)
		ORDER BY id DESC
		LIMIT $2`

	rows, err := r.pool.Query(ctx, q, jurisdiction, limit+1, afterID)
	if err != nil {
		return nil, false, fmt.Errorf("ai: ListAllRules: %w", err)
	}
	defer rows.Close()

	results, err := collectJurisdictionRules(rows, "ListAllRules")
	if err != nil {
		return nil, false, err
	}

	hasMore := len(results) > limit
	if hasMore {
		results = results[:limit]
	}
	return results, hasMore, nil
}

// ListUpcomingRules returns rules that become effective within the next withinDays days.
func (r *PostgresJurisdictionRuleRepository) ListUpcomingRules(ctx context.Context, withinDays int) ([]JurisdictionRule, error) {
	const q = `
		SELECT ` + jurisdictionRuleCols + `
		FROM jurisdiction_rules
		WHERE effective_date > CURRENT_DATE
		  AND effective_date <= CURRENT_DATE + $1 * INTERVAL '1 day'
		ORDER BY effective_date ASC`

	rows, err := r.pool.Query(ctx, q, withinDays)
	if err != nil {
		return nil, fmt.Errorf("ai: ListUpcomingRules: %w", err)
	}
	defer rows.Close()

	return collectJurisdictionRules(rows, "ListUpcomingRules")
}

// ListRulesEffectiveToday returns all rules whose effective_date is today and that have not yet expired.
func (r *PostgresJurisdictionRuleRepository) ListRulesEffectiveToday(ctx context.Context) ([]JurisdictionRule, error) {
	q := `SELECT ` + jurisdictionRuleCols + `
		FROM jurisdiction_rules
		WHERE effective_date = CURRENT_DATE
		  AND (expiration_date IS NULL OR expiration_date > CURRENT_DATE)
		ORDER BY jurisdiction, rule_category`

	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("ai: ListRulesEffectiveToday: %w", err)
	}
	defer rows.Close()
	return collectJurisdictionRules(rows, "ListRulesEffectiveToday")
}

// ─── Scan helpers ─────────────────────────────────────────────────────────────

func scanJurisdictionRule(row pgx.Row) (*JurisdictionRule, error) {
	var rule JurisdictionRule
	var valueRaw []byte
	err := row.Scan(
		&rule.ID,
		&rule.Jurisdiction,
		&rule.RuleCategory,
		&rule.RuleKey,
		&rule.ValueType,
		&valueRaw,
		&rule.StatuteReference,
		&rule.EffectiveDate,
		&rule.ExpirationDate,
		&rule.Notes,
		&rule.SourceDocID,
		&rule.CreatedAt,
		&rule.UpdatedAt,
		&rule.CreatedBy,
	)
	if err != nil {
		return nil, err
	}
	if len(valueRaw) > 0 {
		rule.Value = json.RawMessage(valueRaw)
	}
	return &rule, nil
}

func collectJurisdictionRules(rows pgx.Rows, op string) ([]JurisdictionRule, error) {
	results := []JurisdictionRule{}
	for rows.Next() {
		var rule JurisdictionRule
		var valueRaw []byte
		if err := rows.Scan(
			&rule.ID,
			&rule.Jurisdiction,
			&rule.RuleCategory,
			&rule.RuleKey,
			&rule.ValueType,
			&valueRaw,
			&rule.StatuteReference,
			&rule.EffectiveDate,
			&rule.ExpirationDate,
			&rule.Notes,
			&rule.SourceDocID,
			&rule.CreatedAt,
			&rule.UpdatedAt,
			&rule.CreatedBy,
		); err != nil {
			return nil, fmt.Errorf("ai: %s scan: %w", op, err)
		}
		if len(valueRaw) > 0 {
			rule.Value = json.RawMessage(valueRaw)
		}
		results = append(results, rule)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ai: %s rows: %w", op, err)
	}
	return results, nil
}
