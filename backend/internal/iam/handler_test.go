package iam_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/iam"
	"github.com/quorant/quorant/internal/platform/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"log/slog"
	"os"
)

// newTestServer builds a real http.ServeMux with IAM routes registered, backed
// by the mockUserRepository defined in service_test.go.
func newTestServer(t *testing.T, repo *mockUserRepository, validator auth.TokenValidator) *httptest.Server {
	t.Helper()
	svc := iam.NewUserService(repo)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	handler := iam.NewHandler(svc, logger)
	mux := http.NewServeMux()
	iam.RegisterRoutes(mux, handler, validator)
	return httptest.NewServer(mux)
}

// responseData is a helper to unmarshal the "data" field of a JSON envelope.
func responseData(t *testing.T, body []byte, dst any) {
	t.Helper()
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &envelope))
	require.NoError(t, json.Unmarshal(envelope.Data, dst))
}

// responseErrors returns the first error entry from the errors envelope.
func responseErrors(t *testing.T, body []byte) []struct {
	Code    string `json:"code"`
	Message string `json:"message"`
} {
	t.Helper()
	var envelope struct {
		Errors []struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"errors"`
	}
	require.NoError(t, json.Unmarshal(body, &envelope))
	return envelope.Errors
}

// --- GET /api/v1/auth/me ---

func TestGetMe_ReturnsUserProfile(t *testing.T) {
	repo := newMockRepo()
	userID := uuid.New()
	orgID := uuid.New()
	roleID := uuid.New()
	repo.users["idp-test-user"] = &iam.User{
		ID:          userID,
		IDPUserID:   "idp-test-user",
		Email:       "alice@example.com",
		DisplayName: "Alice",
		IsActive:    true,
	}
	repo.memberships[userID] = []iam.Membership{
		{
			ID:       uuid.New(),
			UserID:   userID,
			OrgID:    orgID,
			RoleID:   roleID,
			RoleName: "admin",
			Status:   "active",
		},
	}

	validator := auth.NewStaticValidator(&auth.Claims{
		Subject: "idp-test-user",
		Email:   "alice@example.com",
		Name:    "Alice",
	})
	srv := newTestServer(t, repo, validator)
	defer srv.Close()

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/auth/me", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer fake-token")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(resp.Body)

	var profile iam.UserProfile
	responseData(t, buf.Bytes(), &profile)

	assert.Equal(t, userID, profile.ID)
	assert.Equal(t, "alice@example.com", profile.Email)
	assert.Equal(t, "Alice", profile.DisplayName)
	require.Len(t, profile.Memberships, 1)
	assert.Equal(t, "admin", profile.Memberships[0].RoleName)
}

func TestGetMe_Unauthenticated(t *testing.T) {
	repo := newMockRepo()
	// Use a validator that returns an error to simulate missing/invalid auth.
	validator := &auth.StaticValidator{Err: auth.ErrStaticValidatorErr}
	srv := newTestServer(t, repo, validator)
	defer srv.Close()

	// No Authorization header.
	req, err := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/auth/me", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

// --- PATCH /api/v1/auth/me ---

func TestUpdateMe_UpdatesDisplayName(t *testing.T) {
	repo := newMockRepo()
	userID := uuid.New()
	repo.users["idp-dave"] = &iam.User{
		ID:          userID,
		IDPUserID:   "idp-dave",
		Email:       "dave@example.com",
		DisplayName: "Dave",
		IsActive:    true,
	}

	validator := auth.NewStaticValidator(&auth.Claims{
		Subject: "idp-dave",
		Email:   "dave@example.com",
		Name:    "Dave",
	})
	srv := newTestServer(t, repo, validator)
	defer srv.Close()

	body := bytes.NewBufferString(`{"display_name":"David"}`)
	req, err := http.NewRequest(http.MethodPatch, srv.URL+"/api/v1/auth/me", body)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer fake-token")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(resp.Body)

	var profile iam.UserProfile
	responseData(t, buf.Bytes(), &profile)

	assert.Equal(t, "David", profile.DisplayName)
	assert.Equal(t, "dave@example.com", profile.Email)
}

func TestUpdateMe_EmptyBody(t *testing.T) {
	repo := newMockRepo()
	userID := uuid.New()
	repo.users["idp-empty"] = &iam.User{
		ID:          userID,
		IDPUserID:   "idp-empty",
		Email:       "empty@example.com",
		DisplayName: "Empty",
		IsActive:    true,
	}

	validator := auth.NewStaticValidator(&auth.Claims{
		Subject: "idp-empty",
		Email:   "empty@example.com",
		Name:    "Empty",
	})
	srv := newTestServer(t, repo, validator)
	defer srv.Close()

	// Empty JSON object — no fields set means validation should fail.
	body := bytes.NewBufferString(`{}`)
	req, err := http.NewRequest(http.MethodPatch, srv.URL+"/api/v1/auth/me", body)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer fake-token")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(resp.Body)
	errs := responseErrors(t, buf.Bytes())
	require.NotEmpty(t, errs)
	assert.Equal(t, "VALIDATION_ERROR", errs[0].Code)
}

func TestUpdateMe_InvalidJSON(t *testing.T) {
	repo := newMockRepo()
	userID := uuid.New()
	repo.users["idp-bad-json"] = &iam.User{
		ID:          userID,
		IDPUserID:   "idp-bad-json",
		Email:       "bad@example.com",
		DisplayName: "Bad",
		IsActive:    true,
	}

	validator := auth.NewStaticValidator(&auth.Claims{
		Subject: "idp-bad-json",
		Email:   "bad@example.com",
		Name:    "Bad",
	})
	srv := newTestServer(t, repo, validator)
	defer srv.Close()

	body := bytes.NewBufferString(`{not valid json}`)
	req, err := http.NewRequest(http.MethodPatch, srv.URL+"/api/v1/auth/me", body)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer fake-token")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(resp.Body)
	errs := responseErrors(t, buf.Bytes())
	require.NotEmpty(t, errs)
	assert.Equal(t, "VALIDATION_ERROR", errs[0].Code)
}

// --- POST /api/v1/webhooks/zitadel ---

func TestZitadelWebhook_CreatesUser(t *testing.T) {
	repo := newMockRepo()
	// Validator is not used for the webhook endpoint (no auth middleware).
	validator := auth.NewStaticValidator(&auth.Claims{})
	srv := newTestServer(t, repo, validator)
	defer srv.Close()

	payload := `{"user_id":"zitadel-new-1","email":"webhook@example.com","name":"Webhook User","event":"user.created"}`
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/webhooks/zitadel", bytes.NewBufferString(payload))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(resp.Body)

	var data map[string]string
	responseData(t, buf.Bytes(), &data)
	assert.Equal(t, "ok", data["status"])

	// Verify the user was actually created in the repository.
	_, exists := repo.users["zitadel-new-1"]
	assert.True(t, exists, "user should have been persisted via GetOrCreateUser")
}

func TestZitadelWebhook_MissingFields(t *testing.T) {
	repo := newMockRepo()
	validator := auth.NewStaticValidator(&auth.Claims{})
	srv := newTestServer(t, repo, validator)
	defer srv.Close()

	// Missing user_id — should result in 400.
	payload := `{"email":"webhook@example.com","name":"Webhook User","event":"user.created"}`
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/webhooks/zitadel", bytes.NewBufferString(payload))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(resp.Body)
	errs := responseErrors(t, buf.Bytes())
	require.NotEmpty(t, errs)
	assert.Equal(t, "VALIDATION_ERROR", errs[0].Code)
}
