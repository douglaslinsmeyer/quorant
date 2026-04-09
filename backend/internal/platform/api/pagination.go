package api

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
)

const (
	defaultPageLimit = 25
	maxPageLimit     = 100
)

// PageRequest holds the parsed pagination parameters from a request.
type PageRequest struct {
	Limit  int    // number of items to return (default 25, max 100)
	Cursor string // opaque cursor token from the previous page
}

// ParsePageRequest extracts cursor-based pagination parameters from the query string.
// The limit defaults to 25, is capped at 100, and is reset to 25 for invalid values.
func ParsePageRequest(r *http.Request) PageRequest {
	q := r.URL.Query()

	limit := defaultPageLimit
	if raw := q.Get("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > maxPageLimit {
		limit = maxPageLimit
	}

	return PageRequest{
		Limit:  limit,
		Cursor: q.Get("cursor"),
	}
}

// EncodeCursor creates an opaque base64-encoded cursor token from a map of sort
// key values. The map keys are sorted before encoding to produce deterministic
// output regardless of insertion order.
func EncodeCursor(vals map[string]string) string {
	// Sort keys for deterministic output.
	keys := make([]string, 0, len(vals))
	for k := range vals {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	ordered := make([]struct {
		K string
		V string
	}, len(keys))
	for i, k := range keys {
		ordered[i].K = k
		ordered[i].V = vals[k]
	}

	data, _ := json.Marshal(ordered)
	return base64.StdEncoding.EncodeToString(data)
}

// DecodeCursor decodes an opaque cursor token back into its sort key values.
// Returns an error if the token is not valid base64 or does not contain valid JSON.
func DecodeCursor(token string) (map[string]string, error) {
	data, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return nil, err
	}

	var ordered []struct {
		K string
		V string
	}
	if err := json.Unmarshal(data, &ordered); err != nil {
		return nil, err
	}

	result := make(map[string]string, len(ordered))
	for _, entry := range ordered {
		result[entry.K] = entry.V
	}
	return result, nil
}
