package api_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/quorant/quorant/internal/platform/api"
)

func TestValidationError(t *testing.T) {
	t.Run("implements APIError interface", func(t *testing.T) {
		var _ api.APIError = api.NewValidationError("invalid email", "email")
	})

	t.Run("returns correct HTTP status", func(t *testing.T) {
		err := api.NewValidationError("invalid email", "email")
		if err.StatusCode() != http.StatusBadRequest {
			t.Errorf("expected StatusCode %d, got %d", http.StatusBadRequest, err.StatusCode())
		}
	})

	t.Run("returns correct error code", func(t *testing.T) {
		err := api.NewValidationError("invalid email", "email")
		if err.ErrorCode() != "VALIDATION_ERROR" {
			t.Errorf("expected ErrorCode VALIDATION_ERROR, got %s", err.ErrorCode())
		}
	})

	t.Run("error message includes message text", func(t *testing.T) {
		err := api.NewValidationError("invalid email", "email")
		if err.Error() == "" {
			t.Error("expected non-empty error message")
		}
	})

	t.Run("stores field name", func(t *testing.T) {
		err := api.NewValidationError("invalid email", "email")
		if err.Field != "email" {
			t.Errorf("expected Field email, got %s", err.Field)
		}
	})

	t.Run("stores message", func(t *testing.T) {
		err := api.NewValidationError("invalid email", "email")
		if err.Message != "invalid email" {
			t.Errorf("expected Message 'invalid email', got %s", err.Message)
		}
	})
}

func TestUnauthenticatedError(t *testing.T) {
	t.Run("implements APIError interface", func(t *testing.T) {
		var _ api.APIError = api.NewUnauthenticatedError("not authenticated")
	})

	t.Run("returns correct HTTP status", func(t *testing.T) {
		err := api.NewUnauthenticatedError("not authenticated")
		if err.StatusCode() != http.StatusUnauthorized {
			t.Errorf("expected StatusCode %d, got %d", http.StatusUnauthorized, err.StatusCode())
		}
	})

	t.Run("returns correct error code", func(t *testing.T) {
		err := api.NewUnauthenticatedError("not authenticated")
		if err.ErrorCode() != "UNAUTHENTICATED" {
			t.Errorf("expected ErrorCode UNAUTHENTICATED, got %s", err.ErrorCode())
		}
	})

	t.Run("stores message", func(t *testing.T) {
		err := api.NewUnauthenticatedError("not authenticated")
		if err.Message != "not authenticated" {
			t.Errorf("expected Message 'not authenticated', got %s", err.Message)
		}
	})
}

func TestForbiddenError(t *testing.T) {
	t.Run("implements APIError interface", func(t *testing.T) {
		var _ api.APIError = api.NewForbiddenError("access denied")
	})

	t.Run("returns correct HTTP status", func(t *testing.T) {
		err := api.NewForbiddenError("access denied")
		if err.StatusCode() != http.StatusForbidden {
			t.Errorf("expected StatusCode %d, got %d", http.StatusForbidden, err.StatusCode())
		}
	})

	t.Run("returns correct error code", func(t *testing.T) {
		err := api.NewForbiddenError("access denied")
		if err.ErrorCode() != "FORBIDDEN" {
			t.Errorf("expected ErrorCode FORBIDDEN, got %s", err.ErrorCode())
		}
	})

	t.Run("stores message", func(t *testing.T) {
		err := api.NewForbiddenError("access denied")
		if err.Message != "access denied" {
			t.Errorf("expected Message 'access denied', got %s", err.Message)
		}
	})
}

func TestNotFoundError(t *testing.T) {
	t.Run("implements APIError interface", func(t *testing.T) {
		var _ api.APIError = api.NewNotFoundError("resource not found")
	})

	t.Run("returns correct HTTP status", func(t *testing.T) {
		err := api.NewNotFoundError("resource not found")
		if err.StatusCode() != http.StatusNotFound {
			t.Errorf("expected StatusCode %d, got %d", http.StatusNotFound, err.StatusCode())
		}
	})

	t.Run("returns correct error code", func(t *testing.T) {
		err := api.NewNotFoundError("resource not found")
		if err.ErrorCode() != "NOT_FOUND" {
			t.Errorf("expected ErrorCode NOT_FOUND, got %s", err.ErrorCode())
		}
	})

	t.Run("stores message", func(t *testing.T) {
		err := api.NewNotFoundError("resource not found")
		if err.Message != "resource not found" {
			t.Errorf("expected Message 'resource not found', got %s", err.Message)
		}
	})
}

func TestConflictError(t *testing.T) {
	t.Run("implements APIError interface", func(t *testing.T) {
		var _ api.APIError = api.NewConflictError("already exists")
	})

	t.Run("returns correct HTTP status", func(t *testing.T) {
		err := api.NewConflictError("already exists")
		if err.StatusCode() != http.StatusConflict {
			t.Errorf("expected StatusCode %d, got %d", http.StatusConflict, err.StatusCode())
		}
	})

	t.Run("returns correct error code", func(t *testing.T) {
		err := api.NewConflictError("already exists")
		if err.ErrorCode() != "CONFLICT" {
			t.Errorf("expected ErrorCode CONFLICT, got %s", err.ErrorCode())
		}
	})

	t.Run("stores message", func(t *testing.T) {
		err := api.NewConflictError("already exists")
		if err.Message != "already exists" {
			t.Errorf("expected Message 'already exists', got %s", err.Message)
		}
	})
}

func TestUnprocessableError(t *testing.T) {
	t.Run("implements APIError interface", func(t *testing.T) {
		var _ api.APIError = api.NewUnprocessableError("cannot process")
	})

	t.Run("returns correct HTTP status", func(t *testing.T) {
		err := api.NewUnprocessableError("cannot process")
		if err.StatusCode() != http.StatusUnprocessableEntity {
			t.Errorf("expected StatusCode %d, got %d", http.StatusUnprocessableEntity, err.StatusCode())
		}
	})

	t.Run("returns correct error code", func(t *testing.T) {
		err := api.NewUnprocessableError("cannot process")
		if err.ErrorCode() != "UNPROCESSABLE" {
			t.Errorf("expected ErrorCode UNPROCESSABLE, got %s", err.ErrorCode())
		}
	})

	t.Run("stores message", func(t *testing.T) {
		err := api.NewUnprocessableError("cannot process")
		if err.Message != "cannot process" {
			t.Errorf("expected Message 'cannot process', got %s", err.Message)
		}
	})
}

func TestRateLimitedError(t *testing.T) {
	t.Run("implements APIError interface", func(t *testing.T) {
		var _ api.APIError = api.NewRateLimitedError("too many requests")
	})

	t.Run("returns correct HTTP status", func(t *testing.T) {
		err := api.NewRateLimitedError("too many requests")
		if err.StatusCode() != http.StatusTooManyRequests {
			t.Errorf("expected StatusCode %d, got %d", http.StatusTooManyRequests, err.StatusCode())
		}
	})

	t.Run("returns correct error code", func(t *testing.T) {
		err := api.NewRateLimitedError("too many requests")
		if err.ErrorCode() != "RATE_LIMITED" {
			t.Errorf("expected ErrorCode RATE_LIMITED, got %s", err.ErrorCode())
		}
	})

	t.Run("stores message", func(t *testing.T) {
		err := api.NewRateLimitedError("too many requests")
		if err.Message != "too many requests" {
			t.Errorf("expected Message 'too many requests', got %s", err.Message)
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

	t.Run("wraps underlying error", func(t *testing.T) {
		underlying := errors.New("db connection failed")
		err := api.NewInternalError(underlying)
		if !errors.Is(err.Unwrap(), underlying) {
			t.Error("expected InternalError to wrap the underlying error")
		}
	})

	t.Run("error message is not empty", func(t *testing.T) {
		err := api.NewInternalError(errors.New("db connection failed"))
		if err.Error() == "" {
			t.Error("expected non-empty error message")
		}
	})
}
