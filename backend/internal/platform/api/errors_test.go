package api_test

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/quorant/quorant/internal/platform/api"
)

func TestValidationError(t *testing.T) {
	t.Run("implements APIError interface", func(t *testing.T) {
		var _ api.APIError = api.NewValidationError("validation.required", "email")
	})

	t.Run("returns correct HTTP status", func(t *testing.T) {
		err := api.NewValidationError("validation.required", "email")
		if err.StatusCode() != http.StatusBadRequest {
			t.Errorf("expected StatusCode %d, got %d", http.StatusBadRequest, err.StatusCode())
		}
	})

	t.Run("returns correct error code", func(t *testing.T) {
		err := api.NewValidationError("validation.required", "email")
		if err.ErrorCode() != "VALIDATION_ERROR" {
			t.Errorf("expected ErrorCode VALIDATION_ERROR, got %s", err.ErrorCode())
		}
	})

	t.Run("stores messageKey via MsgKey", func(t *testing.T) {
		err := api.NewValidationError("validation.required", "email")
		if err.MsgKey() != "validation.required" {
			t.Errorf("expected MsgKey 'validation.required', got %s", err.MsgKey())
		}
	})

	t.Run("stores params via MsgParams", func(t *testing.T) {
		err := api.NewValidationError("validation.min_length", "password", api.P("min", 8))
		params := err.MsgParams()
		if params == nil {
			t.Fatal("expected non-nil params")
		}
		if params["min"] != 8 {
			t.Errorf("expected param min=8, got %v", params["min"])
		}
	})

	t.Run("returns empty params when none provided", func(t *testing.T) {
		err := api.NewValidationError("validation.required", "email")
		params := err.MsgParams()
		if len(params) != 0 {
			t.Errorf("expected empty params, got %v", params)
		}
	})

	t.Run("stores field name", func(t *testing.T) {
		err := api.NewValidationError("validation.required", "email")
		if err.Field != "email" {
			t.Errorf("expected Field email, got %s", err.Field)
		}
	})

	t.Run("Error returns log-friendly string with field", func(t *testing.T) {
		err := api.NewValidationError("validation.required", "email")
		got := err.Error()
		if !strings.Contains(got, "validation.required") {
			t.Errorf("expected Error() to contain message key, got: %s", got)
		}
		if !strings.Contains(got, "email") {
			t.Errorf("expected Error() to contain field name, got: %s", got)
		}
	})
}

func TestNotFoundError(t *testing.T) {
	t.Run("implements APIError interface", func(t *testing.T) {
		var _ api.APIError = api.NewNotFoundError("resource.not_found")
	})

	t.Run("returns correct HTTP status", func(t *testing.T) {
		err := api.NewNotFoundError("resource.not_found")
		if err.StatusCode() != http.StatusNotFound {
			t.Errorf("expected StatusCode %d, got %d", http.StatusNotFound, err.StatusCode())
		}
	})

	t.Run("returns correct error code", func(t *testing.T) {
		err := api.NewNotFoundError("resource.not_found")
		if err.ErrorCode() != "NOT_FOUND" {
			t.Errorf("expected ErrorCode NOT_FOUND, got %s", err.ErrorCode())
		}
	})

	t.Run("stores messageKey via MsgKey", func(t *testing.T) {
		err := api.NewNotFoundError("resource.not_found")
		if err.MsgKey() != "resource.not_found" {
			t.Errorf("expected MsgKey 'resource.not_found', got %s", err.MsgKey())
		}
	})

	t.Run("stores params via MsgParams", func(t *testing.T) {
		err := api.NewNotFoundError("resource.not_found", api.P("id", "abc-123"))
		params := err.MsgParams()
		if params["id"] != "abc-123" {
			t.Errorf("expected param id=abc-123, got %v", params["id"])
		}
	})

	t.Run("returns empty params when none provided", func(t *testing.T) {
		err := api.NewNotFoundError("resource.not_found")
		params := err.MsgParams()
		if len(params) != 0 {
			t.Errorf("expected empty params, got %v", params)
		}
	})

	t.Run("Error returns log-friendly string", func(t *testing.T) {
		err := api.NewNotFoundError("resource.not_found")
		got := err.Error()
		if !strings.Contains(got, "resource.not_found") {
			t.Errorf("expected Error() to contain message key, got: %s", got)
		}
	})
}

func TestUnauthenticatedError(t *testing.T) {
	t.Run("implements APIError interface", func(t *testing.T) {
		var _ api.APIError = api.NewUnauthenticatedError("auth.token_expired")
	})

	t.Run("returns correct HTTP status", func(t *testing.T) {
		err := api.NewUnauthenticatedError("auth.token_expired")
		if err.StatusCode() != http.StatusUnauthorized {
			t.Errorf("expected StatusCode %d, got %d", http.StatusUnauthorized, err.StatusCode())
		}
	})

	t.Run("returns correct error code", func(t *testing.T) {
		err := api.NewUnauthenticatedError("auth.token_expired")
		if err.ErrorCode() != "UNAUTHENTICATED" {
			t.Errorf("expected ErrorCode UNAUTHENTICATED, got %s", err.ErrorCode())
		}
	})

	t.Run("stores messageKey via MsgKey", func(t *testing.T) {
		err := api.NewUnauthenticatedError("auth.token_expired")
		if err.MsgKey() != "auth.token_expired" {
			t.Errorf("expected MsgKey 'auth.token_expired', got %s", err.MsgKey())
		}
	})

	t.Run("stores params via MsgParams", func(t *testing.T) {
		err := api.NewUnauthenticatedError("auth.token_expired", api.P("expiry", "2h"))
		params := err.MsgParams()
		if params["expiry"] != "2h" {
			t.Errorf("expected param expiry=2h, got %v", params["expiry"])
		}
	})

	t.Run("returns empty params when none provided", func(t *testing.T) {
		err := api.NewUnauthenticatedError("auth.token_expired")
		params := err.MsgParams()
		if len(params) != 0 {
			t.Errorf("expected empty params, got %v", params)
		}
	})

	t.Run("Error returns log-friendly string", func(t *testing.T) {
		err := api.NewUnauthenticatedError("auth.token_expired")
		got := err.Error()
		if !strings.Contains(got, "auth.token_expired") {
			t.Errorf("expected Error() to contain message key, got: %s", got)
		}
	})
}

func TestForbiddenError(t *testing.T) {
	t.Run("implements APIError interface", func(t *testing.T) {
		var _ api.APIError = api.NewForbiddenError("auth.insufficient_permissions")
	})

	t.Run("returns correct HTTP status", func(t *testing.T) {
		err := api.NewForbiddenError("auth.insufficient_permissions")
		if err.StatusCode() != http.StatusForbidden {
			t.Errorf("expected StatusCode %d, got %d", http.StatusForbidden, err.StatusCode())
		}
	})

	t.Run("returns correct error code", func(t *testing.T) {
		err := api.NewForbiddenError("auth.insufficient_permissions")
		if err.ErrorCode() != "FORBIDDEN" {
			t.Errorf("expected ErrorCode FORBIDDEN, got %s", err.ErrorCode())
		}
	})

	t.Run("stores messageKey via MsgKey", func(t *testing.T) {
		err := api.NewForbiddenError("auth.insufficient_permissions")
		if err.MsgKey() != "auth.insufficient_permissions" {
			t.Errorf("expected MsgKey 'auth.insufficient_permissions', got %s", err.MsgKey())
		}
	})

	t.Run("stores params via MsgParams", func(t *testing.T) {
		err := api.NewForbiddenError("auth.insufficient_permissions", api.P("role", "admin"))
		params := err.MsgParams()
		if params["role"] != "admin" {
			t.Errorf("expected param role=admin, got %v", params["role"])
		}
	})

	t.Run("returns empty params when none provided", func(t *testing.T) {
		err := api.NewForbiddenError("auth.insufficient_permissions")
		params := err.MsgParams()
		if len(params) != 0 {
			t.Errorf("expected empty params, got %v", params)
		}
	})

	t.Run("Error returns log-friendly string", func(t *testing.T) {
		err := api.NewForbiddenError("auth.insufficient_permissions")
		got := err.Error()
		if !strings.Contains(got, "auth.insufficient_permissions") {
			t.Errorf("expected Error() to contain message key, got: %s", got)
		}
	})
}

func TestConflictError(t *testing.T) {
	t.Run("implements APIError interface", func(t *testing.T) {
		var _ api.APIError = api.NewConflictError("resource.already_exists")
	})

	t.Run("returns correct HTTP status", func(t *testing.T) {
		err := api.NewConflictError("resource.already_exists")
		if err.StatusCode() != http.StatusConflict {
			t.Errorf("expected StatusCode %d, got %d", http.StatusConflict, err.StatusCode())
		}
	})

	t.Run("returns correct error code", func(t *testing.T) {
		err := api.NewConflictError("resource.already_exists")
		if err.ErrorCode() != "CONFLICT" {
			t.Errorf("expected ErrorCode CONFLICT, got %s", err.ErrorCode())
		}
	})

	t.Run("stores messageKey via MsgKey", func(t *testing.T) {
		err := api.NewConflictError("resource.already_exists")
		if err.MsgKey() != "resource.already_exists" {
			t.Errorf("expected MsgKey 'resource.already_exists', got %s", err.MsgKey())
		}
	})

	t.Run("stores params via MsgParams", func(t *testing.T) {
		err := api.NewConflictError("resource.already_exists", api.P("name", "unit-101"))
		params := err.MsgParams()
		if params["name"] != "unit-101" {
			t.Errorf("expected param name=unit-101, got %v", params["name"])
		}
	})

	t.Run("returns empty params when none provided", func(t *testing.T) {
		err := api.NewConflictError("resource.already_exists")
		params := err.MsgParams()
		if len(params) != 0 {
			t.Errorf("expected empty params, got %v", params)
		}
	})

	t.Run("Error returns log-friendly string", func(t *testing.T) {
		err := api.NewConflictError("resource.already_exists")
		got := err.Error()
		if !strings.Contains(got, "resource.already_exists") {
			t.Errorf("expected Error() to contain message key, got: %s", got)
		}
	})
}

func TestUnprocessableError(t *testing.T) {
	t.Run("implements APIError interface", func(t *testing.T) {
		var _ api.APIError = api.NewUnprocessableError("entity.invalid_state")
	})

	t.Run("returns correct HTTP status", func(t *testing.T) {
		err := api.NewUnprocessableError("entity.invalid_state")
		if err.StatusCode() != http.StatusUnprocessableEntity {
			t.Errorf("expected StatusCode %d, got %d", http.StatusUnprocessableEntity, err.StatusCode())
		}
	})

	t.Run("returns correct error code", func(t *testing.T) {
		err := api.NewUnprocessableError("entity.invalid_state")
		if err.ErrorCode() != "UNPROCESSABLE" {
			t.Errorf("expected ErrorCode UNPROCESSABLE, got %s", err.ErrorCode())
		}
	})

	t.Run("stores messageKey via MsgKey", func(t *testing.T) {
		err := api.NewUnprocessableError("entity.invalid_state")
		if err.MsgKey() != "entity.invalid_state" {
			t.Errorf("expected MsgKey 'entity.invalid_state', got %s", err.MsgKey())
		}
	})

	t.Run("stores params via MsgParams", func(t *testing.T) {
		err := api.NewUnprocessableError("entity.invalid_state", api.P("current", "draft"))
		params := err.MsgParams()
		if params["current"] != "draft" {
			t.Errorf("expected param current=draft, got %v", params["current"])
		}
	})

	t.Run("returns empty params when none provided", func(t *testing.T) {
		err := api.NewUnprocessableError("entity.invalid_state")
		params := err.MsgParams()
		if len(params) != 0 {
			t.Errorf("expected empty params, got %v", params)
		}
	})

	t.Run("Error returns log-friendly string", func(t *testing.T) {
		err := api.NewUnprocessableError("entity.invalid_state")
		got := err.Error()
		if !strings.Contains(got, "entity.invalid_state") {
			t.Errorf("expected Error() to contain message key, got: %s", got)
		}
	})
}

func TestRateLimitedError(t *testing.T) {
	t.Run("implements APIError interface", func(t *testing.T) {
		var _ api.APIError = api.NewRateLimitedError("server.rate_limited")
	})

	t.Run("returns correct HTTP status", func(t *testing.T) {
		err := api.NewRateLimitedError("server.rate_limited")
		if err.StatusCode() != http.StatusTooManyRequests {
			t.Errorf("expected StatusCode %d, got %d", http.StatusTooManyRequests, err.StatusCode())
		}
	})

	t.Run("returns correct error code", func(t *testing.T) {
		err := api.NewRateLimitedError("server.rate_limited")
		if err.ErrorCode() != "RATE_LIMITED" {
			t.Errorf("expected ErrorCode RATE_LIMITED, got %s", err.ErrorCode())
		}
	})

	t.Run("stores messageKey via MsgKey", func(t *testing.T) {
		err := api.NewRateLimitedError("server.rate_limited")
		if err.MsgKey() != "server.rate_limited" {
			t.Errorf("expected MsgKey 'server.rate_limited', got %s", err.MsgKey())
		}
	})

	t.Run("stores params via MsgParams", func(t *testing.T) {
		err := api.NewRateLimitedError("server.rate_limited", api.P("retry_after", 30))
		params := err.MsgParams()
		if params["retry_after"] != 30 {
			t.Errorf("expected param retry_after=30, got %v", params["retry_after"])
		}
	})

	t.Run("returns empty params when none provided", func(t *testing.T) {
		err := api.NewRateLimitedError("server.rate_limited")
		params := err.MsgParams()
		if len(params) != 0 {
			t.Errorf("expected empty params, got %v", params)
		}
	})

	t.Run("Error returns log-friendly string", func(t *testing.T) {
		err := api.NewRateLimitedError("server.rate_limited")
		got := err.Error()
		if !strings.Contains(got, "server.rate_limited") {
			t.Errorf("expected Error() to contain message key, got: %s", got)
		}
	})
}

func TestInternalError(t *testing.T) {
	t.Run("implements APIError interface", func(t *testing.T) {
		var _ api.APIError = api.NewInternalError(errors.New("db connection failed"))
	})

	t.Run("returns correct HTTP status", func(t *testing.T) {
		err := api.NewInternalError(errors.New("db connection failed"))
		if err.StatusCode() != http.StatusInternalServerError {
			t.Errorf("expected StatusCode %d, got %d", http.StatusInternalServerError, err.StatusCode())
		}
	})

	t.Run("returns correct error code", func(t *testing.T) {
		err := api.NewInternalError(errors.New("db connection failed"))
		if err.ErrorCode() != "INTERNAL_ERROR" {
			t.Errorf("expected ErrorCode INTERNAL_ERROR, got %s", err.ErrorCode())
		}
	})

	t.Run("MsgKey returns server.internal_error", func(t *testing.T) {
		err := api.NewInternalError(errors.New("db connection failed"))
		if err.MsgKey() != "server.internal_error" {
			t.Errorf("expected MsgKey 'server.internal_error', got %s", err.MsgKey())
		}
	})

	t.Run("returns nil params", func(t *testing.T) {
		err := api.NewInternalError(errors.New("db connection failed"))
		params := err.MsgParams()
		if params != nil {
			t.Errorf("expected nil params, got %v", params)
		}
	})

	t.Run("wraps underlying error", func(t *testing.T) {
		underlying := errors.New("db connection failed")
		err := api.NewInternalError(underlying)
		if !errors.Is(err.Unwrap(), underlying) {
			t.Error("expected InternalError to wrap the underlying error")
		}
	})

	t.Run("Error returns log-friendly string", func(t *testing.T) {
		err := api.NewInternalError(errors.New("db connection failed"))
		got := err.Error()
		if got == "" {
			t.Error("expected non-empty error message")
		}
	})
}

// TestParamConstructor verifies the P() helper builds Param values correctly.
func TestParamConstructor(t *testing.T) {
	p := api.P("key", "value")
	if p.Key != "key" {
		t.Errorf("expected Key 'key', got %s", p.Key)
	}
	if p.Value != "value" {
		t.Errorf("expected Value 'value', got %v", p.Value)
	}
}

// TestMultipleParams verifies that multiple params are stored correctly.
func TestMultipleParams(t *testing.T) {
	err := api.NewNotFoundError("resource.not_found", api.P("type", "unit"), api.P("id", "abc-123"))
	params := err.MsgParams()
	if len(params) != 2 {
		t.Fatalf("expected 2 params, got %d", len(params))
	}
	if params["type"] != "unit" {
		t.Errorf("expected param type=unit, got %v", params["type"])
	}
	if params["id"] != "abc-123" {
		t.Errorf("expected param id=abc-123, got %v", params["id"])
	}
}
