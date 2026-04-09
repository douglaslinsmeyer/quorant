package api_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/quorant/quorant/internal/platform/api"
)

func TestWriteJSON(t *testing.T) {
	t.Run("writes correct status code", func(t *testing.T) {
		w := httptest.NewRecorder()
		api.WriteJSON(w, http.StatusOK, map[string]string{"id": "123"})
		if w.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
		}
	})

	t.Run("sets content-type to application/json", func(t *testing.T) {
		w := httptest.NewRecorder()
		api.WriteJSON(w, http.StatusOK, map[string]string{"id": "123"})
		ct := w.Header().Get("Content-Type")
		if ct != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", ct)
		}
	})

	t.Run("wraps data in response envelope", func(t *testing.T) {
		w := httptest.NewRecorder()
		api.WriteJSON(w, http.StatusOK, map[string]string{"id": "123"})

		var resp api.Response
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Data == nil {
			t.Error("expected data field in response envelope")
		}
		if resp.Errors != nil {
			t.Error("expected no errors in data response")
		}
	})

	t.Run("writes 201 Created for created resources", func(t *testing.T) {
		w := httptest.NewRecorder()
		api.WriteJSON(w, http.StatusCreated, map[string]string{"id": "abc"})
		if w.Code != http.StatusCreated {
			t.Errorf("expected status %d, got %d", http.StatusCreated, w.Code)
		}
	})
}

func TestWriteJSONWithMeta(t *testing.T) {
	t.Run("includes meta in response envelope", func(t *testing.T) {
		w := httptest.NewRecorder()
		meta := &api.Meta{Cursor: "abc123", HasMore: true}
		api.WriteJSONWithMeta(w, http.StatusOK, []string{"item1", "item2"}, meta)

		var resp api.Response
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Meta == nil {
			t.Fatal("expected meta field in response")
		}
		if resp.Meta.Cursor != "abc123" {
			t.Errorf("expected cursor abc123, got %s", resp.Meta.Cursor)
		}
		if !resp.Meta.HasMore {
			t.Error("expected has_more to be true")
		}
	})

	t.Run("has_more false is present when no more pages", func(t *testing.T) {
		w := httptest.NewRecorder()
		meta := &api.Meta{HasMore: false}
		api.WriteJSONWithMeta(w, http.StatusOK, []string{}, meta)

		var resp api.Response
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Meta == nil {
			t.Fatal("expected meta field in response")
		}
		if resp.Meta.HasMore {
			t.Error("expected has_more to be false")
		}
	})

	t.Run("sets correct status code", func(t *testing.T) {
		w := httptest.NewRecorder()
		meta := &api.Meta{HasMore: false}
		api.WriteJSONWithMeta(w, http.StatusOK, []string{}, meta)
		if w.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
		}
	})
}

func TestWriteError(t *testing.T) {
	t.Run("uses APIError status and code when err implements APIError", func(t *testing.T) {
		w := httptest.NewRecorder()
		apiErr := api.NewNotFoundError("user not found")
		api.WriteError(w, apiErr)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
		}

		var resp api.Response
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if len(resp.Errors) == 0 {
			t.Fatal("expected errors in response")
		}
		if resp.Errors[0].Code != "NOT_FOUND" {
			t.Errorf("expected error code NOT_FOUND, got %s", resp.Errors[0].Code)
		}
		if resp.Errors[0].Message != "user not found" {
			t.Errorf("expected message 'user not found', got %s", resp.Errors[0].Message)
		}
	})

	t.Run("returns 500 InternalError for non-APIError", func(t *testing.T) {
		w := httptest.NewRecorder()
		api.WriteError(w, errors.New("unexpected db failure"))

		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
		}

		var resp api.Response
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if len(resp.Errors) == 0 {
			t.Fatal("expected errors in response")
		}
		if resp.Errors[0].Code != "INTERNAL_ERROR" {
			t.Errorf("expected error code INTERNAL_ERROR, got %s", resp.Errors[0].Code)
		}
	})

	t.Run("includes field in error when ValidationError has field", func(t *testing.T) {
		w := httptest.NewRecorder()
		apiErr := api.NewValidationError("email is required", "email")
		api.WriteError(w, apiErr)

		var resp api.Response
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if len(resp.Errors) == 0 {
			t.Fatal("expected errors in response")
		}
		if resp.Errors[0].Field != "email" {
			t.Errorf("expected field 'email', got %s", resp.Errors[0].Field)
		}
	})

	t.Run("response has no data field on error", func(t *testing.T) {
		w := httptest.NewRecorder()
		api.WriteError(w, api.NewForbiddenError("not allowed"))

		var resp api.Response
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Data != nil {
			t.Error("expected no data field in error response")
		}
	})
}

func TestReadJSON(t *testing.T) {
	t.Run("decodes valid JSON body into destination struct", func(t *testing.T) {
		type payload struct {
			Name string `json:"name"`
		}
		body := `{"name":"Alice"}`
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")

		var dst payload
		err := api.ReadJSON(r, &dst)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if dst.Name != "Alice" {
			t.Errorf("expected Name Alice, got %s", dst.Name)
		}
	})

	t.Run("returns ValidationError for malformed JSON", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{bad json`))
		r.Header.Set("Content-Type", "application/json")

		var dst map[string]any
		err := api.ReadJSON(r, &dst)
		if err == nil {
			t.Fatal("expected error for malformed JSON, got nil")
		}

		var valErr *api.ValidationError
		if !errors.As(err, &valErr) {
			t.Errorf("expected *api.ValidationError, got %T", err)
		}
	})

	t.Run("returns ValidationError for empty body", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(""))
		r.Header.Set("Content-Type", "application/json")

		var dst map[string]any
		err := api.ReadJSON(r, &dst)
		if err == nil {
			t.Fatal("expected error for empty body, got nil")
		}

		var valErr *api.ValidationError
		if !errors.As(err, &valErr) {
			t.Errorf("expected *api.ValidationError, got %T", err)
		}
	})

	t.Run("returns ValidationError for unknown fields", func(t *testing.T) {
		type strictPayload struct {
			Name string `json:"name"`
		}
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"Bob","unknown":"val"}`))
		r.Header.Set("Content-Type", "application/json")

		var dst strictPayload
		// ReadJSON should disallow unknown fields
		err := api.ReadJSON(r, &dst)
		if err == nil {
			t.Fatal("expected error for unknown fields, got nil")
		}

		var valErr *api.ValidationError
		if !errors.As(err, &valErr) {
			t.Errorf("expected *api.ValidationError, got %T", err)
		}
	})
}
