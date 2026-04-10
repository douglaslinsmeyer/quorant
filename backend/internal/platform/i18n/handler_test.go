package i18n

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()

	registry, err := NewRegistry()
	require.NoError(t, err)

	handler := NewHandler(registry)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/i18n/{locale}", handler.GetPack)
	mux.HandleFunc("GET /api/v1/i18n", handler.ListLocales)

	return httptest.NewServer(mux)
}

func TestGetPack(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	t.Run("200 with en_US pack and correct headers", func(t *testing.T) {
		resp, err := http.Get(srv.URL + "/api/v1/i18n/en_US")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
		assert.Equal(t, "public, max-age=3600", resp.Header.Get("Cache-Control"))
		assert.Equal(t, `"1.0.0"`, resp.Header.Get("ETag"))

		var pack Pack
		err = json.NewDecoder(resp.Body).Decode(&pack)
		require.NoError(t, err)
		assert.Equal(t, "en_US", pack.Locale)
		assert.NotEmpty(t, pack.Messages)
		assert.Equal(t, "{field} is required", pack.Messages["validation.required"])
	})

	t.Run("404 for unknown locale", func(t *testing.T) {
		resp, err := http.Get(srv.URL + "/api/v1/i18n/zz_ZZ")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}

func TestListLocales(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/v1/i18n")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	var body map[string][]string
	err = json.NewDecoder(resp.Body).Decode(&body)
	require.NoError(t, err)
	assert.Contains(t, body["locales"], "en_US")
}
