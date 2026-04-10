// Package api provides standard HTTP error types, response envelope helpers,
// and cursor-based pagination utilities used by every API endpoint.
package api

import (
	"fmt"
	"net/http"
)

// Param holds a single key-value pair used for i18n message interpolation.
type Param struct {
	Key   string
	Value any
}

// P is a convenience constructor for Param.
func P(key string, value any) Param {
	return Param{Key: key, Value: value}
}

// paramsToMap converts a slice of Param into a map for easy consumption by
// i18n message renderers.
func paramsToMap(params []Param) map[string]any {
	m := make(map[string]any, len(params))
	for _, p := range params {
		m[p.Key] = p.Value
	}
	return m
}

// APIError is implemented by all API-level errors. It exposes the HTTP status
// code, a machine-readable code string, and i18n message key/params alongside
// the standard error interface.
type APIError interface {
	error
	StatusCode() int            // HTTP status code to use in the response
	ErrorCode() string          // Machine-readable code, e.g. "NOT_FOUND"
	MsgKey() string             // i18n message key, e.g. "validation.required"
	MsgParams() map[string]any  // interpolation params for the message key
}

// ---------------------------------------------------------------------------
// ValidationError — 400 Bad Request
// ---------------------------------------------------------------------------

// ValidationError represents a 400 Bad Request caused by invalid input data.
type ValidationError struct {
	Field      string // the field that failed validation
	messageKey string
	params     map[string]any
}

func NewValidationError(messageKey, field string, params ...Param) *ValidationError {
	return &ValidationError{
		Field:      field,
		messageKey: messageKey,
		params:     paramsToMap(params),
	}
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error: %s (field=%s)", e.messageKey, e.Field)
}
func (e *ValidationError) StatusCode() int           { return http.StatusBadRequest }
func (e *ValidationError) ErrorCode() string         { return "VALIDATION_ERROR" }
func (e *ValidationError) MsgKey() string            { return e.messageKey }
func (e *ValidationError) MsgParams() map[string]any { return e.params }

// ---------------------------------------------------------------------------
// NotFoundError — 404 Not Found
// ---------------------------------------------------------------------------

// NotFoundError represents a 404 Not Found.
type NotFoundError struct {
	messageKey string
	params     map[string]any
}

func NewNotFoundError(messageKey string, params ...Param) *NotFoundError {
	return &NotFoundError{
		messageKey: messageKey,
		params:     paramsToMap(params),
	}
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("not found: %s", e.messageKey)
}
func (e *NotFoundError) StatusCode() int           { return http.StatusNotFound }
func (e *NotFoundError) ErrorCode() string         { return "NOT_FOUND" }
func (e *NotFoundError) MsgKey() string            { return e.messageKey }
func (e *NotFoundError) MsgParams() map[string]any { return e.params }

// ---------------------------------------------------------------------------
// UnauthenticatedError — 401 Unauthorized
// ---------------------------------------------------------------------------

// UnauthenticatedError represents a 401 Unauthorized — the caller is not identified.
type UnauthenticatedError struct {
	messageKey string
	params     map[string]any
}

func NewUnauthenticatedError(messageKey string, params ...Param) *UnauthenticatedError {
	return &UnauthenticatedError{
		messageKey: messageKey,
		params:     paramsToMap(params),
	}
}

func (e *UnauthenticatedError) Error() string {
	return fmt.Sprintf("unauthenticated: %s", e.messageKey)
}
func (e *UnauthenticatedError) StatusCode() int           { return http.StatusUnauthorized }
func (e *UnauthenticatedError) ErrorCode() string         { return "UNAUTHENTICATED" }
func (e *UnauthenticatedError) MsgKey() string            { return e.messageKey }
func (e *UnauthenticatedError) MsgParams() map[string]any { return e.params }

// ---------------------------------------------------------------------------
// ForbiddenError — 403 Forbidden
// ---------------------------------------------------------------------------

// ForbiddenError represents a 403 Forbidden — the caller is identified but lacks permission.
type ForbiddenError struct {
	messageKey string
	params     map[string]any
}

func NewForbiddenError(messageKey string, params ...Param) *ForbiddenError {
	return &ForbiddenError{
		messageKey: messageKey,
		params:     paramsToMap(params),
	}
}

func (e *ForbiddenError) Error() string {
	return fmt.Sprintf("forbidden: %s", e.messageKey)
}
func (e *ForbiddenError) StatusCode() int           { return http.StatusForbidden }
func (e *ForbiddenError) ErrorCode() string         { return "FORBIDDEN" }
func (e *ForbiddenError) MsgKey() string            { return e.messageKey }
func (e *ForbiddenError) MsgParams() map[string]any { return e.params }

// ---------------------------------------------------------------------------
// ConflictError — 409 Conflict
// ---------------------------------------------------------------------------

// ConflictError represents a 409 Conflict — e.g. a duplicate resource.
type ConflictError struct {
	messageKey string
	params     map[string]any
}

func NewConflictError(messageKey string, params ...Param) *ConflictError {
	return &ConflictError{
		messageKey: messageKey,
		params:     paramsToMap(params),
	}
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("conflict: %s", e.messageKey)
}
func (e *ConflictError) StatusCode() int           { return http.StatusConflict }
func (e *ConflictError) ErrorCode() string         { return "CONFLICT" }
func (e *ConflictError) MsgKey() string            { return e.messageKey }
func (e *ConflictError) MsgParams() map[string]any { return e.params }

// ---------------------------------------------------------------------------
// UnprocessableError — 422 Unprocessable Entity
// ---------------------------------------------------------------------------

// UnprocessableError represents a 422 Unprocessable Entity — semantically invalid input.
type UnprocessableError struct {
	messageKey string
	params     map[string]any
}

func NewUnprocessableError(messageKey string, params ...Param) *UnprocessableError {
	return &UnprocessableError{
		messageKey: messageKey,
		params:     paramsToMap(params),
	}
}

func (e *UnprocessableError) Error() string {
	return fmt.Sprintf("unprocessable: %s", e.messageKey)
}
func (e *UnprocessableError) StatusCode() int           { return http.StatusUnprocessableEntity }
func (e *UnprocessableError) ErrorCode() string         { return "UNPROCESSABLE" }
func (e *UnprocessableError) MsgKey() string            { return e.messageKey }
func (e *UnprocessableError) MsgParams() map[string]any { return e.params }

// ---------------------------------------------------------------------------
// RateLimitedError — 429 Too Many Requests
// ---------------------------------------------------------------------------

// RateLimitedError represents a 429 Too Many Requests.
type RateLimitedError struct {
	messageKey string
	params     map[string]any
}

func NewRateLimitedError(messageKey string, params ...Param) *RateLimitedError {
	return &RateLimitedError{
		messageKey: messageKey,
		params:     paramsToMap(params),
	}
}

func (e *RateLimitedError) Error() string {
	return fmt.Sprintf("rate limited: %s", e.messageKey)
}
func (e *RateLimitedError) StatusCode() int           { return http.StatusTooManyRequests }
func (e *RateLimitedError) ErrorCode() string         { return "RATE_LIMITED" }
func (e *RateLimitedError) MsgKey() string            { return e.messageKey }
func (e *RateLimitedError) MsgParams() map[string]any { return e.params }

// ---------------------------------------------------------------------------
// InternalError — 500 Internal Server Error
// ---------------------------------------------------------------------------

// InternalError represents a 500 Internal Server Error. It wraps an underlying
// error so that details remain available for server-side logging.
type InternalError struct {
	underlying error
}

func NewInternalError(err error) *InternalError {
	return &InternalError{underlying: err}
}

func (e *InternalError) Error() string              { return "internal server error" }
func (e *InternalError) StatusCode() int            { return http.StatusInternalServerError }
func (e *InternalError) ErrorCode() string          { return "INTERNAL_ERROR" }
func (e *InternalError) MsgKey() string             { return "server.internal_error" }
func (e *InternalError) MsgParams() map[string]any  { return map[string]any{} }
func (e *InternalError) Unwrap() error              { return e.underlying }
