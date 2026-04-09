package cfgstore_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"

	"github.com/quorant/quorant/internal/platform/cfgstore"
	"github.com/stretchr/testify/assert"
)

func TestScopeConstants(t *testing.T) {
	assert.Equal(t, cfgstore.Scope("platform"), cfgstore.ScopePlatform)
	assert.Equal(t, cfgstore.Scope("firm"), cfgstore.ScopeFirm)
	assert.Equal(t, cfgstore.Scope("org"), cfgstore.ScopeOrg)
}

func TestEntry_JSON(t *testing.T) {
	entry := cfgstore.Entry{
		Scope: cfgstore.ScopeOrg,
		Key:   "ai_config",
		Value: json.RawMessage(`{"auto_apply_threshold": 0.95}`),
	}

	data, err := json.Marshal(entry)
	assert.NoError(t, err)

	var decoded cfgstore.Entry
	assert.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, cfgstore.ScopeOrg, decoded.Scope)
	assert.Equal(t, "ai_config", decoded.Key)
}

func TestNewPostgresStore(t *testing.T) {
	store := cfgstore.NewPostgresStore(nil)
	assert.NotNil(t, store)
}

func TestValidation_PlatformScopeRejectsNonNilID(t *testing.T) {
	store := cfgstore.NewPostgresStore(nil)
	err := store.Set(context.Background(), cfgstore.ScopePlatform, ptrUUID(), "key", json.RawMessage(`"val"`))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "platform scope must have nil scope_id")
}

func TestValidation_OrgScopeRequiresID(t *testing.T) {
	store := cfgstore.NewPostgresStore(nil)
	err := store.Set(context.Background(), cfgstore.ScopeOrg, nil, "key", json.RawMessage(`"val"`))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "requires a scope_id")
}

func TestValidation_DeleteOrgScopeRequiresID(t *testing.T) {
	store := cfgstore.NewPostgresStore(nil)
	err := store.Delete(context.Background(), cfgstore.ScopeOrg, nil, "key")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "requires a scope_id")
}

func TestValidation_UnknownScope(t *testing.T) {
	store := cfgstore.NewPostgresStore(nil)
	err := store.Set(context.Background(), "invalid", nil, "key", json.RawMessage(`"val"`))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown scope")
}

func ptrUUID() *uuid.UUID {
	id := uuid.New()
	return &id
}

// Note: Integration tests for Get/GetAll/Set/Delete scope resolution
// require a running PostgreSQL with the config_entries table.
// Those tests belong in postgres_test.go with build tag: integration.
