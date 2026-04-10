package api

import (
	"encoding/json"
	"errors"
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
		Message: apiErr.MsgKey(),
	}

	// Expose the field if this is a ValidationError.
	var valErr *ValidationError
	if errors.As(err, &valErr) {
		entry.Field = valErr.Field
	}

	writeJSON(w, apiErr.StatusCode(), Response{Errors: []Error{entry}})
}

// ReadJSON decodes the JSON request body into dst. It returns a *ValidationError
// for malformed JSON, empty bodies, or unknown fields.
func ReadJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		return NewValidationError("validation.invalid_request_body", "")
	}
	return nil
}

// writeJSON is the internal helper that marshals the envelope and writes it.
func writeJSON(w http.ResponseWriter, status int, resp Response) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(resp)
}
