package admin_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/admin"
	"github.com/quorant/quorant/internal/audit"
	"github.com/quorant/quorant/internal/platform/auth"
	"github.com/quorant/quorant/internal/platform/middleware"
	"github.com/quorant/quorant/internal/platform/queue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// allowAllChecker is a PermissionChecker that always grants access (for use in tests).
type allowAllChecker struct{}

func (allowAllChecker) HasPermission(_ context.Context, _, _ uuid.UUID, _ string) (bool, error) {
	return true, nil
}

// fixedUserIDResolver returns a resolveUserID func that always returns the given UUID.
func fixedUserIDResolver(id uuid.UUID) func(context.Context) (uuid.UUID, error) {
	return func(_ context.Context) (uuid.UUID, error) {
		return id, nil
	}
}

// noopChecker satisfies middleware.PermissionChecker without importing extra packages.
var _ middleware.PermissionChecker = allowAllChecker{}

// newAdminTestServer builds a real http.ServeMux with admin routes registered,
// backed by the mockAdminRepository defined in service_test.go.
func newAdminTestServer(t *testing.T, repo *mockAdminRepository, validator auth.TokenValidator) *httptest.Server {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	svc := admin.NewAdminService(repo, audit.NewNoopAuditor(), queue.NewInMemoryPublisher(), logger)
	handler := admin.NewAdminHandler(svc, logger)
	mux := http.NewServeMux()
	admin.RegisterRoutes(mux, handler, validator, allowAllChecker{}, fixedUserIDResolver(uuid.New()))
	return httptest.NewServer(mux)
}

// adminValidator returns a StaticValidator with a test subject.
func adminValidator() auth.TokenValidator {
	return auth.NewStaticValidator(&auth.Claims{
		Subject: "test-admin-subject",
		Email:   "admin@example.com",
		Name:    "Test Admin",
	})
}

// responseData parses the "data" envelope from the response body.
func adminResponseData(t *testing.T, body []byte, dst any) {
	t.Helper()
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &envelope))
	require.NoError(t, json.Unmarshal(envelope.Data, dst))
}

// readBody reads and returns the full response body bytes.
func readBody(t *testing.T, resp *http.Response) []byte {
	t.Helper()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(resp.Body)
	return buf.Bytes()
}

// ─── TestCreateFlag_Handler ───────────────────────────────────────────────────

func TestCreateFlag_Handler(t *testing.T) {
	repo := newMockAdminRepo()
	srv := newAdminTestServer(t, repo, adminValidator())
	defer srv.Close()

	body := bytes.NewBufferString(`{"key":"test_feature","enabled":true}`)
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/admin/feature-flags", body)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer fake-token")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var flag admin.FeatureFlag
	adminResponseData(t, readBody(t, resp), &flag)
	assert.Equal(t, "test_feature", flag.Key)
	assert.True(t, flag.Enabled)
	assert.NotEqual(t, uuid.Nil, flag.ID)
}

func TestCreateFlag_Handler_MissingKey(t *testing.T) {
	repo := newMockAdminRepo()
	srv := newAdminTestServer(t, repo, adminValidator())
	defer srv.Close()

	body := bytes.NewBufferString(`{"enabled":true}`)
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/admin/feature-flags", body)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer fake-token")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// ─── TestListFlags_Handler ────────────────────────────────────────────────────

func TestListFlags_Handler(t *testing.T) {
	repo := newMockAdminRepo()
	// Pre-populate some flags.
	flagID1 := uuid.New()
	flagID2 := uuid.New()
	now := time.Now()
	repo.flags[flagID1] = &admin.FeatureFlag{ID: flagID1, Key: "flag_alpha", Enabled: true, CreatedAt: now, UpdatedAt: now}
	repo.flags[flagID2] = &admin.FeatureFlag{ID: flagID2, Key: "flag_beta", Enabled: false, CreatedAt: now, UpdatedAt: now}

	srv := newAdminTestServer(t, repo, adminValidator())
	defer srv.Close()

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/admin/feature-flags", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer fake-token")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var flags []admin.FeatureFlag
	adminResponseData(t, readBody(t, resp), &flags)
	assert.Len(t, flags, 2)
}

// ─── TestSetOverride_Handler ──────────────────────────────────────────────────

func TestSetOverride_Handler(t *testing.T) {
	repo := newMockAdminRepo()
	flagID := uuid.New()
	now := time.Now()
	repo.flags[flagID] = &admin.FeatureFlag{ID: flagID, Key: "override_flag", Enabled: false, CreatedAt: now, UpdatedAt: now}

	srv := newAdminTestServer(t, repo, adminValidator())
	defer srv.Close()

	orgID := uuid.New()
	body := bytes.NewBufferString(`{"org_id":"` + orgID.String() + `","enabled":true}`)
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/admin/feature-flags/"+flagID.String()+"/overrides", body)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer fake-token")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var override admin.FeatureFlagOverride
	adminResponseData(t, readBody(t, resp), &override)
	assert.Equal(t, flagID, override.FlagID)
	assert.Equal(t, orgID, override.OrgID)
	assert.True(t, override.Enabled)
}

// ─── TestListTenants_Handler ──────────────────────────────────────────────────

func TestListTenants_Handler(t *testing.T) {
	repo := newMockAdminRepo()
	repo.tenants = []map[string]any{
		{"id": uuid.New().String(), "name": "HOA Alpha", "type": "hoa"},
		{"id": uuid.New().String(), "name": "HOA Beta", "type": "hoa"},
	}

	srv := newAdminTestServer(t, repo, adminValidator())
	defer srv.Close()

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/admin/tenants", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer fake-token")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var tenants []map[string]any
	adminResponseData(t, readBody(t, resp), &tenants)
	assert.Len(t, tenants, 2)
}

// ─── TestSuspendTenant_Handler ────────────────────────────────────────────────

func TestSuspendTenant_Handler(t *testing.T) {
	repo := newMockAdminRepo()
	srv := newAdminTestServer(t, repo, adminValidator())
	defer srv.Close()

	orgID := uuid.New()
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/admin/tenants/"+orgID.String()+"/suspend", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer fake-token")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any
	adminResponseData(t, readBody(t, resp), &result)
	assert.Equal(t, "ok", result["status"])
	assert.Equal(t, "suspended", result["action"])
}

// ─── Auth checks ──────────────────────────────────────────────────────────────

func TestListFlags_Handler_Unauthenticated(t *testing.T) {
	repo := newMockAdminRepo()
	// Use a validator that always rejects.
	validator := &auth.StaticValidator{Err: auth.ErrStaticValidatorErr}
	srv := newAdminTestServer(t, repo, validator)
	defer srv.Close()

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/admin/feature-flags", nil)
	require.NoError(t, err)
	// No Authorization header.

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
