package i18n

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRegistry(t *testing.T) {
	registry, err := NewRegistry()
	require.NoError(t, err)
	require.NotNil(t, registry)
}

func TestRegistry_Get(t *testing.T) {
	registry, err := NewRegistry()
	require.NoError(t, err)

	t.Run("returns en_US pack with correct fields", func(t *testing.T) {
		pack, ok := registry.Get("en_US")
		require.True(t, ok)
		assert.Equal(t, "en_US", pack.Locale)
		assert.Equal(t, "1.0.0", pack.Version)
		assert.NotEmpty(t, pack.Messages)
		assert.Equal(t, "{field} is required", pack.Messages["validation.required"])
		assert.Equal(t, "an internal error occurred", pack.Messages["server.internal_error"])
	})

	t.Run("returns false for unknown locale", func(t *testing.T) {
		pack, ok := registry.Get("zz_ZZ")
		assert.False(t, ok)
		assert.Nil(t, pack)
	})
}

func TestRegistry_Locales(t *testing.T) {
	registry, err := NewRegistry()
	require.NoError(t, err)

	locales := registry.Locales()
	assert.Contains(t, locales, "en_US")

	// Verify sorted order
	for i := 1; i < len(locales); i++ {
		assert.True(t, locales[i-1] <= locales[i], "locales should be sorted")
	}
}
