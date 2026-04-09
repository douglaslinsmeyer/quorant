package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// Response is the standard JSON envelope for all API responses.
// Successful responses populate Data (and optionally Meta).
// Error responses populate Errors.
type Response struct {
	Data   any     `json:"data,omitempty"`
	Meta   *Meta   `json:"meta,omitempty"`
	Errors []Error `json:"errors,omitempty"`
}

// Meta carries cursor-based pagination metadata.
type Meta struct {
	Cursor  string `json:"cursor,omitempty"`
	HasMore bool   `json:"has_more"`
}

// Error is a single machine-readable error entry inside the errors array.
type Error struct {
	Code    string `json:"code"`
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
}

// WriteJSON writes a successful JSON response wrapped in the data envelope.
func WriteJSON(w http.ResponseWriter, status int, data any) {
	writeJSON(w, status, Response{Data: data})
}

// WriteJSONWithMeta writes a successful JSON response with pagination metadata.
func WriteJSONWithMeta(w http.ResponseWriter, status int, data any, meta *Meta) {
	writeJSON(w, status, Response{Data: data, Meta: meta})
}

// WriteError inspects err: if it implements APIError the status code and error
// code are taken from the error; otherwise a 500 InternalError is synthesised.
// The response body uses the errors envelope format.
func WriteError(w http.ResponseWriter, err error) {
	var apiErr APIError
	if !errors.As(err, &apiErr) {
		apiErr = NewInternalError(err)
	}

	entry := Error{
		Code:    apiErr.ErrorCode(),
		Message: apiErr.Error(),
	}

	// Expose the field if this is a ValidationError.
	var valErr *ValidationError
	if errors.As(err, &valErr) {
		entry.Field = valErr.Field
		entry.Message = valErr.Message
	}

	// For other typed errors use the message directly rather than the wrapped form.
	if entry.Message == "" {
		entry.Message = apiErr.Error()
	}

	// Use the human-readable message from the concrete error types where available.
	entry.Message = humanMessage(apiErr)

	writeJSON(w, apiErr.StatusCode(), Response{Errors: []Error{entry}})
}

// humanMessage returns the most user-friendly message from an APIError,
// preferring the struct's Message field over the Error() string.
func humanMessage(apiErr APIError) string {
	switch e := apiErr.(type) {
	case *ValidationError:
		return e.Message
	case *UnauthenticatedError:
		return e.Message
	case *ForbiddenError:
		return e.Message
	case *NotFoundError:
		return e.Message
	case *ConflictError:
		return e.Message
	case *UnprocessableError:
		return e.Message
	case *RateLimitedError:
		return e.Message
	case *InternalError:
		return "internal server error"
	default:
		return apiErr.Error()
	}
}

// ReadJSON decodes the JSON request body into dst. It returns a *ValidationError
// for malformed JSON, empty bodies, or unknown fields.
func ReadJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		return NewValidationError(fmt.Sprintf("invalid request body: %s", err.Error()), "")
	}
	return nil
}

// writeJSON is the internal helper that marshals the envelope and writes it.
func writeJSON(w http.ResponseWriter, status int, resp Response) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(resp)
}
