package estoppel_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/audit"
	"github.com/quorant/quorant/internal/estoppel"
	"github.com/quorant/quorant/internal/platform/queue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Handler test setup
// ---------------------------------------------------------------------------

func newTestHandler() (*estoppel.Handler, *mockRepo) {
	repo := newMockRepo()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	svc := estoppel.NewEstoppelService(
		repo,
		&mockFinancialProvider{snapshot: &estoppel.FinancialSnapshot{}},
		&mockComplianceProvider{snapshot: &estoppel.ComplianceSnapshot{}},
		&mockPropertyProvider{snapshot: &estoppel.PropertySnapshot{UnitNumber: "1A"}},
		estoppel.NewNoopNarrativeGenerator(),
		&mockCertificateGenerator{},
		audit.NewNoopAuditor(),
		queue.NewInMemoryPublisher(),
		logger,
	)
	return estoppel.NewHandler(svc, logger), repo
}

// newHandlerMux builds a minimal ServeMux wiring the handler routes directly
// (no auth/entitlement middleware) so we can test handler logic in isolation.
func newHandlerMux(h *estoppel.Handler) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /organizations/{org_id}/estoppel/requests", h.CreateRequest)
	mux.HandleFunc("GET /organizations/{org_id}/estoppel/requests", h.ListRequests)
	mux.HandleFunc("GET /organizations/{org_id}/estoppel/requests/{id}", h.GetRequest)
	mux.HandleFunc("POST /organizations/{org_id}/estoppel/requests/{id}/approve", h.ApproveRequest)
	mux.HandleFunc("POST /organizations/{org_id}/estoppel/requests/{id}/reject", h.RejectRequest)
	return mux
}

// ---------------------------------------------------------------------------
// Helper: decode response envelope
// ---------------------------------------------------------------------------

type apiResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"errors"`
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestHandler_CreateRequest_Success(t *testing.T) {
	h, _ := newTestHandler()
	mux := newHandlerMux(h)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	orgID := uuid.New()
	body := estoppel.CreateEstoppelRequestDTO{
		UnitID:          uuid.New(),
		RequestType:     "estoppel_certificate",
		RequestorType:   "title_company",
		RequestorName:   "Jane Doe",
		RequestorEmail:  "jane@title.com",
		RequestorPhone:  "555-0200",
		RequestorCompany: "Best Title",
		PropertyAddress: "456 Elm St",
		OwnerName:       "Sam Green",
	}

	b, err := json.Marshal(body)
	require.NoError(t, err)

	url := srv.URL + "/organizations/" + orgID.String() + "/estoppel/requests"
	resp, err := http.Post(url, "application/json", bytes.NewReader(b)) //nolint:noctx
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var env apiResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&env))
	assert.Nil(t, env.Errors)
	assert.NotEmpty(t, env.Data)
}

func TestHandler_CreateRequest_InvalidBody(t *testing.T) {
	h, _ := newTestHandler()
	mux := newHandlerMux(h)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	orgID := uuid.New()
	// Send empty JSON object — all required fields missing.
	url := srv.URL + "/organizations/" + orgID.String() + "/estoppel/requests"
	resp, err := http.Post(url, "application/json", bytes.NewReader([]byte(`{}`))) //nolint:noctx
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var env apiResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&env))
	assert.NotEmpty(t, env.Errors)
}

func TestHandler_ListRequests_Empty(t *testing.T) {
	h, _ := newTestHandler()
	mux := newHandlerMux(h)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	orgID := uuid.New()
	url := srv.URL + "/organizations/" + orgID.String() + "/estoppel/requests"
	resp, err := http.Get(url) //nolint:noctx
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var env apiResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&env))
	assert.Empty(t, env.Errors)
}

func TestHandler_GetRequest_NotFound(t *testing.T) {
	h, _ := newTestHandler()
	mux := newHandlerMux(h)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	orgID := uuid.New()
	unknownID := uuid.New()
	url := srv.URL + "/organizations/" + orgID.String() + "/estoppel/requests/" + unknownID.String()

	req, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestHandler_CreateRequest_MissingOrgID(t *testing.T) {
	h, _ := newTestHandler()
	mux := newHandlerMux(h)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// Route without {org_id} won't match the registered pattern, so we set up
	// a minimal check using httptest directly to verify the handler responds
	// correctly to a bad UUID in the path.
	orgID := "not-a-uuid"
	body := estoppel.CreateEstoppelRequestDTO{
		UnitID:        uuid.New(),
		RequestType:   "estoppel_certificate",
		RequestorType: "homeowner",
	}
	b, _ := json.Marshal(body)

	url := srv.URL + "/organizations/" + orgID + "/estoppel/requests"
	resp, err := http.Post(url, "application/json", bytes.NewReader(b)) //nolint:noctx
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}
