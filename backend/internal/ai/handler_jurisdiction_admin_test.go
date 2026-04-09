package ai_test

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/quorant/quorant/internal/ai"
	"github.com/quorant/quorant/internal/platform/queue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupAdminTestServer(t *testing.T) (*httptest.Server, *mockJurisdictionRuleRepo) {
	t.Helper()
	repo := &mockJurisdictionRuleRepo{}
	handler := ai.NewJurisdictionAdminHandler(repo, queue.NewInMemoryPublisher(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/admin/jurisdiction-rules", handler.CreateRule)
	mux.HandleFunc("GET /api/v1/admin/jurisdiction-rules/{id}", handler.GetRule)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts, repo
}

func TestAdminCreateRule_Valid(t *testing.T) {
	// Given a valid CreateJurisdictionRuleRequest,
	// when POST /api/v1/admin/jurisdiction-rules is called,
	// then the response is 201 and the rule is returned.
	ts, _ := setupAdminTestServer(t)

	body, err := json.Marshal(map[string]any{
		"jurisdiction":      "FL",
		"rule_category":     "meeting_notice",
		"rule_key":          "min_days_notice",
		"value_type":        "integer",
		"value":             14,
		"statute_reference": "FL Stat. § 720.303(2)",
		"effective_date":    "2024-01-01",
	})
	require.NoError(t, err)

	resp, err := http.Post(ts.URL+"/api/v1/admin/jurisdiction-rules", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var result map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))

	data, ok := result["data"].(map[string]any)
	require.True(t, ok, "response should have a data field")
	assert.Equal(t, "FL", data["jurisdiction"])
	assert.Equal(t, "meeting_notice", data["rule_category"])
	assert.Equal(t, "min_days_notice", data["rule_key"])
}

func TestAdminCreateRule_InvalidCategory(t *testing.T) {
	// Given a request with an invalid rule_category,
	// when POST /api/v1/admin/jurisdiction-rules is called,
	// then the response is 400.
	ts, _ := setupAdminTestServer(t)

	body, err := json.Marshal(map[string]any{
		"jurisdiction":      "FL",
		"rule_category":     "not_a_real_category",
		"rule_key":          "some_key",
		"value_type":        "integer",
		"value":             42,
		"statute_reference": "FL Stat. § 720.303(2)",
		"effective_date":    "2024-01-01",
	})
	require.NoError(t, err)

	resp, err := http.Post(ts.URL+"/api/v1/admin/jurisdiction-rules", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}
