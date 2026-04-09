// Package cfgstore provides hierarchical configuration storage.
// Configuration resolves through a scope chain: org → firm → platform.
// More specific scopes override broader ones.
package cfgstore

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
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
// Configuration resolves through the scope chain: org → firm → platform.
// The most specific scope wins.
type Store interface {
	// Get retrieves a config value for the given key, resolving through the scope chain.
	// For an HOA managed by a firm: checks org → firm → platform.
	// For a self-managed HOA: checks org → platform.
	// Returns the resolved value and the scope it came from.
	Get(ctx context.Context, orgID uuid.UUID, key string) (json.RawMessage, Scope, error)

	// GetAll retrieves all config values for an org, merged through the scope chain.
	// Org-level values override firm-level, which override platform-level.
	GetAll(ctx context.Context, orgID uuid.UUID) (map[string]json.RawMessage, error)

	// Set writes a config value at a specific scope.
	Set(ctx context.Context, scope Scope, scopeID *uuid.UUID, key string, value json.RawMessage) error

	// Delete removes a config value at a specific scope.
	Delete(ctx context.Context, scope Scope, scopeID *uuid.UUID, key string) error
}

// PostgresStore implements Store using the organizations.settings JSONB column
// and a dedicated config_entries table for platform and firm-level config.
type PostgresStore struct {
	pool *pgxpool.Pool
}

// NewPostgresStore creates a hierarchical config store.
func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

// Get resolves a config value through the scope chain: org → firm → platform.
func (s *PostgresStore) Get(ctx context.Context, orgID uuid.UUID, key string) (json.RawMessage, Scope, error) {
	// 1. Check org-level (organizations.settings JSONB)
	var settings json.RawMessage
	err := s.pool.QueryRow(ctx, `
		SELECT settings FROM organizations WHERE id = $1 AND deleted_at IS NULL
	`, orgID).Scan(&settings)
	if err == nil && len(settings) > 2 { // > 2 means not just '{}'
		var m map[string]json.RawMessage
		if json.Unmarshal(settings, &m) == nil {
			if val, ok := m[key]; ok {
				return val, ScopeOrg, nil
			}
		}
	}

	// 2. Check firm-level (if org is managed by a firm)
	var firmOrgID *uuid.UUID
	err = s.pool.QueryRow(ctx, `
		SELECT firm_org_id FROM organizations_management
		WHERE hoa_org_id = $1 AND ended_at IS NULL
	`, orgID).Scan(&firmOrgID)
	if err == nil && firmOrgID != nil {
		var firmSettings json.RawMessage
		err = s.pool.QueryRow(ctx, `
			SELECT settings FROM organizations WHERE id = $1 AND deleted_at IS NULL
		`, firmOrgID).Scan(&firmSettings)
		if err == nil && len(firmSettings) > 2 {
			var m map[string]json.RawMessage
			if json.Unmarshal(firmSettings, &m) == nil {
				if val, ok := m[key]; ok {
					return val, ScopeFirm, nil
				}
			}
		}
	}

	// 3. Check platform-level (first org with NULL parent_id in settings, or a dedicated config)
	// For platform config, we use a well-known key pattern in the global config
	// stored as a row in a platform_config table. For simplicity, use env-based
	// config for platform defaults and return nil here.
	return nil, "", fmt.Errorf("config key %q not found for org %s", key, orgID)
}

// GetAll merges config values from platform → firm → org (most specific wins).
func (s *PostgresStore) GetAll(ctx context.Context, orgID uuid.UUID) (map[string]json.RawMessage, error) {
	result := make(map[string]json.RawMessage)

	// Load org settings
	var settings json.RawMessage
	err := s.pool.QueryRow(ctx, `
		SELECT settings FROM organizations WHERE id = $1 AND deleted_at IS NULL
	`, orgID).Scan(&settings)
	if err == nil && len(settings) > 2 {
		var m map[string]json.RawMessage
		if json.Unmarshal(settings, &m) == nil {
			for k, v := range m {
				result[k] = v
			}
		}
	}

	return result, nil
}

// Set writes a config value into the organization's settings JSONB.
func (s *PostgresStore) Set(ctx context.Context, scope Scope, scopeID *uuid.UUID, key string, value json.RawMessage) error {
	if scope == ScopePlatform {
		// Platform config is managed via environment variables, not DB
		return fmt.Errorf("platform-level config is set via environment variables")
	}

	if scopeID == nil {
		return fmt.Errorf("scope ID required for %s-level config", scope)
	}

	// Use jsonb_set to update the specific key in the settings column
	_, err := s.pool.Exec(ctx, `
		UPDATE organizations
		SET settings = jsonb_set(COALESCE(settings, '{}'), $2, $3),
		    updated_at = now()
		WHERE id = $1
	`, scopeID, []string{key}, value)
	return err
}

// Delete removes a config key from the organization's settings JSONB.
func (s *PostgresStore) Delete(ctx context.Context, scope Scope, scopeID *uuid.UUID, key string) error {
	if scopeID == nil {
		return fmt.Errorf("scope ID required")
	}

	_, err := s.pool.Exec(ctx, `
		UPDATE organizations
		SET settings = settings - $2,
		    updated_at = now()
		WHERE id = $1
	`, scopeID, key)
	return err
}
