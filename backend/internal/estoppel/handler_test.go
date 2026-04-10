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
	"github.com/quorant/quorant/internal/platform/middleware"
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

// withTestUserID is a test middleware that injects a fixed user UUID into the
// request context so that handlers requiring authentication succeed.
func withTestUserID(userID uuid.UUID, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := middleware.WithUserID(r.Context(), userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// newHandlerMux builds a minimal ServeMux wiring the handler routes with a
// stub auth middleware that injects a test user ID into the request context.
func newHandlerMux(h *estoppel.Handler) *http.ServeMux {
	testUserID := uuid.New()
	mux := http.NewServeMux()
	mux.Handle("POST /organizations/{org_id}/estoppel/requests",
		withTestUserID(testUserID, http.HandlerFunc(h.CreateRequest)))
	mux.HandleFunc("GET /organizations/{org_id}/estoppel/requests", h.ListRequests)
	mux.HandleFunc("GET /organizations/{org_id}/estoppel/requests/{id}", h.GetRequest)
	mux.Handle("POST /organizations/{org_id}/estoppel/requests/{id}/approve",
		withTestUserID(testUserID, http.HandlerFunc(h.ApproveRequest)))
	mux.Handle("POST /organizations/{org_id}/estoppel/requests/{id}/reject",
		withTestUserID(testUserID, http.HandlerFunc(h.RejectRequest)))
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

func TestHandler_CreateRequest_Unauthenticated(t *testing.T) {
	h, _ := newTestHandler()
	// Build a mux WITHOUT the auth middleware so no user ID is injected.
	mux := http.NewServeMux()
	mux.HandleFunc("POST /organizations/{org_id}/estoppel/requests", h.CreateRequest)
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

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	var env apiResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&env))
	require.Len(t, env.Errors, 1)
	assert.Equal(t, "UNAUTHENTICATED", env.Errors[0].Code)
}
