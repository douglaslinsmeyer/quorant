package cfgstore_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/platform/cfgstore"
	"github.com/stretchr/testify/assert"
)

// mockStore for unit testing the scope resolution logic.
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
	// Just verify it doesn't panic with nil pool (constructor only)
	store := cfgstore.NewPostgresStore(nil)
	assert.NotNil(t, store)

	// Set with nil pool will fail at query time, not construction
	err := store.Set(context.Background(), cfgstore.ScopePlatform, nil, "key", json.RawMessage(`"val"`))
	assert.Error(t, err) // platform config not allowed via DB

	id := uuid.New()
	err = store.Delete(context.Background(), cfgstore.ScopeOrg, nil, "key")
	assert.Error(t, err) // nil scope ID
	_ = id
}
