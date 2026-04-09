// Package cfgstore provides hierarchical configuration storage.
// Configuration resolves through a scope chain: org → firm → platform.
// More specific scopes override broader ones.
//
// Storage uses the config_entries table (not organizations.settings).
package cfgstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Scope represents the level at which configuration is defined.
type Scope string

const (
	ScopePlatform Scope = "platform" // global defaults
	ScopeFirm     Scope = "firm"     // management firm overrides
	ScopeOrg      Scope = "org"      // individual org (HOA) overrides
)

// Entry is a single configuration value with its scope.
type Entry struct {
	Scope Scope           `json:"scope"`
	Key   string          `json:"key"`
	Value json.RawMessage `json:"value"`
}

// Store reads and writes hierarchical configuration.
type Store interface {
	// Get retrieves a config value, resolving: org → firm → platform.
	Get(ctx context.Context, orgID uuid.UUID, key string) (json.RawMessage, Scope, error)

	// GetAll retrieves all config values merged: platform → firm → org (most specific wins).
	GetAll(ctx context.Context, orgID uuid.UUID) (map[string]json.RawMessage, error)

	// GetPlatform retrieves a platform-level config value directly.
	GetPlatform(ctx context.Context, key string) (json.RawMessage, error)

	// Set writes a config value at a specific scope using UPSERT.
	Set(ctx context.Context, scope Scope, scopeID *uuid.UUID, key string, value json.RawMessage) error

	// Delete removes a config value at a specific scope.
	Delete(ctx context.Context, scope Scope, scopeID *uuid.UUID, key string) error
}

// PostgresStore implements Store using the config_entries table.
type PostgresStore struct {
	pool *pgxpool.Pool
}

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

func (s *PostgresStore) Get(ctx context.Context, orgID uuid.UUID, key string) (json.RawMessage, Scope, error) {
	// 1. Check org-level
	val, err := s.getEntry(ctx, ScopeOrg, &orgID, key)
	if err != nil {
		return nil, "", fmt.Errorf("cfgstore: org scope: %w", err)
	}
	if val != nil {
		return val, ScopeOrg, nil
	}

	// 2. Check firm-level (if managed)
	var firmOrgID uuid.UUID
	err = s.pool.QueryRow(ctx, `
		SELECT firm_org_id FROM organizations_management
		WHERE hoa_org_id = $1 AND ended_at IS NULL
	`, orgID).Scan(&firmOrgID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, "", fmt.Errorf("cfgstore: management link: %w", err)
	}
	if err == nil {
		val, err = s.getEntry(ctx, ScopeFirm, &firmOrgID, key)
		if err != nil {
			return nil, "", fmt.Errorf("cfgstore: firm scope: %w", err)
		}
		if val != nil {
			return val, ScopeFirm, nil
		}
	}

	// 3. Check platform-level
	val, err = s.getEntry(ctx, ScopePlatform, nil, key)
	if err != nil {
		return nil, "", fmt.Errorf("cfgstore: platform scope: %w", err)
	}
	if val != nil {
		return val, ScopePlatform, nil
	}

	return nil, "", fmt.Errorf("cfgstore: key %q not found (checked org, firm, platform)", key)
}

func (s *PostgresStore) GetAll(ctx context.Context, orgID uuid.UUID) (map[string]json.RawMessage, error) {
	result := make(map[string]json.RawMessage)

	// 1. Platform defaults
	platform, err := s.listEntries(ctx, ScopePlatform, nil)
	if err != nil {
		return nil, fmt.Errorf("cfgstore: platform: %w", err)
	}
	for k, v := range platform {
		result[k] = v
	}

	// 2. Firm overrides
	var firmOrgID uuid.UUID
	err = s.pool.QueryRow(ctx, `
		SELECT firm_org_id FROM organizations_management
		WHERE hoa_org_id = $1 AND ended_at IS NULL
	`, orgID).Scan(&firmOrgID)
	if err == nil {
		firm, err := s.listEntries(ctx, ScopeFirm, &firmOrgID)
		if err != nil {
			return nil, fmt.Errorf("cfgstore: firm: %w", err)
		}
		for k, v := range firm {
			result[k] = v
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("cfgstore: management link: %w", err)
	}

	// 3. Org overrides (most specific wins)
	org, err := s.listEntries(ctx, ScopeOrg, &orgID)
	if err != nil {
		return nil, fmt.Errorf("cfgstore: org: %w", err)
	}
	for k, v := range org {
		result[k] = v
	}

	return result, nil
}

func (s *PostgresStore) GetPlatform(ctx context.Context, key string) (json.RawMessage, error) {
	val, err := s.getEntry(ctx, ScopePlatform, nil, key)
	if err != nil {
		return nil, fmt.Errorf("cfgstore: platform key %q: %w", key, err)
	}
	if val == nil {
		return nil, fmt.Errorf("cfgstore: platform key %q not found", key)
	}
	return val, nil
}

func (s *PostgresStore) Set(ctx context.Context, scope Scope, scopeID *uuid.UUID, key string, value json.RawMessage) error {
	if err := validateScopeID(scope, scopeID); err != nil {
		return err
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO config_entries (scope, scope_id, key, value)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (scope, COALESCE(scope_id, '00000000-0000-0000-0000-000000000000'), key)
		DO UPDATE SET value = EXCLUDED.value, updated_at = now()
	`, string(scope), scopeID, key, value)
	return err
}

func (s *PostgresStore) Delete(ctx context.Context, scope Scope, scopeID *uuid.UUID, key string) error {
	if err := validateScopeID(scope, scopeID); err != nil {
		return err
	}
	if scopeID == nil {
		_, err := s.pool.Exec(ctx, `
			DELETE FROM config_entries WHERE scope = $1 AND scope_id IS NULL AND key = $2
		`, string(scope), key)
		return err
	}
	_, err := s.pool.Exec(ctx, `
		DELETE FROM config_entries WHERE scope = $1 AND scope_id = $2 AND key = $3
	`, string(scope), scopeID, key)
	return err
}

// --- internal helpers ---

func (s *PostgresStore) getEntry(ctx context.Context, scope Scope, scopeID *uuid.UUID, key string) (json.RawMessage, error) {
	var value json.RawMessage
	var err error
	if scopeID == nil {
		err = s.pool.QueryRow(ctx, `
			SELECT value FROM config_entries WHERE scope = $1 AND scope_id IS NULL AND key = $2
		`, string(scope), key).Scan(&value)
	} else {
		err = s.pool.QueryRow(ctx, `
			SELECT value FROM config_entries WHERE scope = $1 AND scope_id = $2 AND key = $3
		`, string(scope), scopeID, key).Scan(&value)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return value, err
}

func (s *PostgresStore) listEntries(ctx context.Context, scope Scope, scopeID *uuid.UUID) (map[string]json.RawMessage, error) {
	var rows pgx.Rows
	var err error
	if scopeID == nil {
		rows, err = s.pool.Query(ctx, `SELECT key, value FROM config_entries WHERE scope = $1 AND scope_id IS NULL`, string(scope))
	} else {
		rows, err = s.pool.Query(ctx, `SELECT key, value FROM config_entries WHERE scope = $1 AND scope_id = $2`, string(scope), scopeID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]json.RawMessage)
	for rows.Next() {
		var key string
		var val json.RawMessage
		if err := rows.Scan(&key, &val); err != nil {
			return nil, err
		}
		result[key] = val
	}
	return result, rows.Err()
}

func validateScopeID(scope Scope, scopeID *uuid.UUID) error {
	switch scope {
	case ScopePlatform:
		if scopeID != nil {
			return fmt.Errorf("cfgstore: platform scope must have nil scope_id")
		}
	case ScopeFirm, ScopeOrg:
		if scopeID == nil {
			return fmt.Errorf("cfgstore: %s scope requires a scope_id", scope)
		}
	default:
		return fmt.Errorf("cfgstore: unknown scope %q", scope)
	}
	return nil
}
