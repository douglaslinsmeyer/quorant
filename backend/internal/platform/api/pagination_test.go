package api_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/quorant/quorant/internal/platform/api"
)

func TestParsePageRequest(t *testing.T) {
	t.Run("defaults limit to 25 when not specified", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/items", nil)
		page := api.ParsePageRequest(r)
		if page.Limit != 25 {
			t.Errorf("expected default limit 25, got %d", page.Limit)
		}
	})

	t.Run("parses limit from query param", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/items?limit=50", nil)
		page := api.ParsePageRequest(r)
		if page.Limit != 50 {
			t.Errorf("expected limit 50, got %d", page.Limit)
		}
	})

	t.Run("caps limit at 100", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/items?limit=200", nil)
		page := api.ParsePageRequest(r)
		if page.Limit != 100 {
			t.Errorf("expected limit capped at 100, got %d", page.Limit)
		}
	})

	t.Run("defaults limit to 25 when limit is invalid", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/items?limit=abc", nil)
		page := api.ParsePageRequest(r)
		if page.Limit != 25 {
			t.Errorf("expected default limit 25 for invalid value, got %d", page.Limit)
		}
	})

	t.Run("defaults limit to 25 when limit is zero", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/items?limit=0", nil)
		page := api.ParsePageRequest(r)
		if page.Limit != 25 {
			t.Errorf("expected default limit 25 for zero value, got %d", page.Limit)
		}
	})

	t.Run("defaults limit to 25 when limit is negative", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/items?limit=-5", nil)
		page := api.ParsePageRequest(r)
		if page.Limit != 25 {
			t.Errorf("expected default limit 25 for negative value, got %d", page.Limit)
		}
	})

	t.Run("parses cursor from query param", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/items?cursor=abc123", nil)
		page := api.ParsePageRequest(r)
		if page.Cursor != "abc123" {
			t.Errorf("expected cursor abc123, got %s", page.Cursor)
		}
	})

	t.Run("cursor is empty when not specified", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/items", nil)
		page := api.ParsePageRequest(r)
		if page.Cursor != "" {
			t.Errorf("expected empty cursor, got %s", page.Cursor)
		}
	})
}

func TestEncodeCursor(t *testing.T) {
	t.Run("returns a non-empty string for valid input", func(t *testing.T) {
		token := api.EncodeCursor(map[string]string{"id": "abc123"})
		if token == "" {
			t.Error("expected non-empty cursor token")
		}
	})

	t.Run("returns deterministic output for same input", func(t *testing.T) {
		vals := map[string]string{"id": "abc123", "created_at": "2024-01-01"}
		token1 := api.EncodeCursor(vals)
		token2 := api.EncodeCursor(vals)
		if token1 != token2 {
			t.Errorf("expected deterministic tokens, got %s and %s", token1, token2)
		}
	})

	t.Run("encodes multiple keys", func(t *testing.T) {
		vals := map[string]string{
			"id":         "xyz",
			"created_at": "2024-06-01",
		}
		token := api.EncodeCursor(vals)
		if token == "" {
			t.Error("expected non-empty cursor token for multiple keys")
		}
	})
}

func TestDecodeCursor(t *testing.T) {
	t.Run("decodes a cursor encoded by EncodeCursor", func(t *testing.T) {
		original := map[string]string{"id": "abc123"}
		token := api.EncodeCursor(original)

		decoded, err := api.DecodeCursor(token)
		if err != nil {
			t.Fatalf("unexpected error decoding cursor: %v", err)
		}
		if decoded["id"] != "abc123" {
			t.Errorf("expected id abc123, got %s", decoded["id"])
		}
	})

	t.Run("decodes cursor with multiple keys", func(t *testing.T) {
		original := map[string]string{
			"id":         "xyz",
			"created_at": "2024-06-01",
		}
		token := api.EncodeCursor(original)

		decoded, err := api.DecodeCursor(token)
		if err != nil {
			t.Fatalf("unexpected error decoding cursor: %v", err)
		}
		if decoded["id"] != "xyz" {
			t.Errorf("expected id xyz, got %s", decoded["id"])
		}
		if decoded["created_at"] != "2024-06-01" {
			t.Errorf("expected created_at 2024-06-01, got %s", decoded["created_at"])
		}
	})

	t.Run("returns error for invalid base64", func(t *testing.T) {
		_, err := api.DecodeCursor("!!!not-valid-base64!!!")
		if err == nil {
			t.Error("expected error for invalid base64 token, got nil")
		}
	})

	t.Run("returns error for valid base64 but invalid JSON", func(t *testing.T) {
		import64 := "bm90anNvbg==" // base64 of "notjson"
		_, err := api.DecodeCursor(import64)
		if err == nil {
			t.Error("expected error for valid base64 but invalid JSON, got nil")
		}
	})

	t.Run("roundtrip preserves all values", func(t *testing.T) {
		original := map[string]string{
			"tenant_id": "t-001",
			"unit":      "4B",
			"seq":       "9999",
		}
		token := api.EncodeCursor(original)
		decoded, err := api.DecodeCursor(token)
		if err != nil {
			t.Fatalf("roundtrip decode failed: %v", err)
		}
		for k, v := range original {
			if decoded[k] != v {
				t.Errorf("key %s: expected %s, got %s", k, v, decoded[k])
			}
		}
	})
}
