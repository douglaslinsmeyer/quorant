// Package api provides standard HTTP error types, response envelope helpers,
// and cursor-based pagination utilities used by every API endpoint.
package api

import (
	"fmt"
	"net/http"
)

// APIError is implemented by all API-level errors. It exposes the HTTP status
// code and a machine-readable code string alongside the standard error interface.
type APIError interface {
	error
	StatusCode() int   // HTTP status code to use in the response
	ErrorCode() string // Machine-readable code, e.g. "NOT_FOUND"
}

// ValidationError represents a 400 Bad Request caused by invalid input data.
type ValidationError struct {
	Message string
	Field   string // optional: the field that failed validation
}

func NewValidationError(message, field string) *ValidationError {
	return &ValidationError{Message: message, Field: field}
}

func (e *ValidationError) Error() string      { return fmt.Sprintf("validation error: %s", e.Message) }
func (e *ValidationError) StatusCode() int    { return http.StatusBadRequest }
func (e *ValidationError) ErrorCode() string  { return "VALIDATION_ERROR" }

// UnauthenticatedError represents a 401 Unauthorized — the caller is not identified.
type UnauthenticatedError struct {
	Message string
}

func NewUnauthenticatedError(message string) *UnauthenticatedError {
	return &UnauthenticatedError{Message: message}
}

func (e *UnauthenticatedError) Error() string      { return fmt.Sprintf("unauthenticated: %s", e.Message) }
func (e *UnauthenticatedError) StatusCode() int    { return http.StatusUnauthorized }
func (e *UnauthenticatedError) ErrorCode() string  { return "UNAUTHENTICATED" }

// ForbiddenError represents a 403 Forbidden — the caller is identified but lacks permission.
type ForbiddenError struct {
	Message string
}

func NewForbiddenError(message string) *ForbiddenError {
	return &ForbiddenError{Message: message}
}

func (e *ForbiddenError) Error() string      { return fmt.Sprintf("forbidden: %s", e.Message) }
func (e *ForbiddenError) StatusCode() int    { return http.StatusForbidden }
func (e *ForbiddenError) ErrorCode() string  { return "FORBIDDEN" }

// NotFoundError represents a 404 Not Found.
type NotFoundError struct {
	Message string
}

func NewNotFoundError(message string) *NotFoundError {
	return &NotFoundError{Message: message}
}

func (e *NotFoundError) Error() string      { return fmt.Sprintf("not found: %s", e.Message) }
func (e *NotFoundError) StatusCode() int    { return http.StatusNotFound }
func (e *NotFoundError) ErrorCode() string  { return "NOT_FOUND" }

// ConflictError represents a 409 Conflict — e.g. a duplicate resource.
type ConflictError struct {
	Message string
}

func NewConflictError(message string) *ConflictError {
	return &ConflictError{Message: message}
}

func (e *ConflictError) Error() string      { return fmt.Sprintf("conflict: %s", e.Message) }
func (e *ConflictError) StatusCode() int    { return http.StatusConflict }
func (e *ConflictError) ErrorCode() string  { return "CONFLICT" }

// UnprocessableError represents a 422 Unprocessable Entity — semantically invalid input.
type UnprocessableError struct {
	Message string
}

func NewUnprocessableError(message string) *UnprocessableError {
	return &UnprocessableError{Message: message}
}

func (e *UnprocessableError) Error() string      { return fmt.Sprintf("unprocessable: %s", e.Message) }
func (e *UnprocessableError) StatusCode() int    { return http.StatusUnprocessableEntity }
func (e *UnprocessableError) ErrorCode() string  { return "UNPROCESSABLE" }

// RateLimitedError represents a 429 Too Many Requests.
type RateLimitedError struct {
	Message string
}

func NewRateLimitedError(message string) *RateLimitedError {
	return &RateLimitedError{Message: message}
}

func (e *RateLimitedError) Error() string      { return fmt.Sprintf("rate limited: %s", e.Message) }
func (e *RateLimitedError) StatusCode() int    { return http.StatusTooManyRequests }
func (e *RateLimitedError) ErrorCode() string  { return "RATE_LIMITED" }

// InternalError represents a 500 Internal Server Error. It wraps an underlying
// error so that details remain available for server-side logging.
type InternalError struct {
	underlying error
}

func NewInternalError(err error) *InternalError {
	return &InternalError{underlying: err}
}

func (e *InternalError) Error() string      { return "internal server error" }
func (e *InternalError) StatusCode() int    { return http.StatusInternalServerError }
func (e *InternalError) ErrorCode() string  { return "INTERNAL_ERROR" }
func (e *InternalError) Unwrap() error      { return e.underlying }
