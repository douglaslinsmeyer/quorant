package webhook_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/webhook"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Subscription JSON serialization ─────────────────────────────────────────

func TestSubscription_SecretOmittedFromJSON(t *testing.T) {
	sub := webhook.Subscription{
		ID:            uuid.New(),
		OrgID:         uuid.New(),
		Name:          "My Webhook",
		EventPatterns: []string{"quorant.gov.*"},
		TargetURL:     "https://example.com/hook",
		Secret:        "super-secret-value",
		Headers:       map[string]string{"X-Custom": "header"},
		IsActive:      true,
		RetryPolicy:   webhook.DefaultRetryPolicy(),
		CreatedBy:     uuid.New(),
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	data, err := json.Marshal(sub)
	require.NoError(t, err)

	var out map[string]any
	require.NoError(t, json.Unmarshal(data, &out))

	_, hasSecret := out["secret"]
	assert.False(t, hasSecret, "secret field must be omitted from JSON output")

	assert.Equal(t, "My Webhook", out["name"])
	assert.Equal(t, "https://example.com/hook", out["target_url"])
	assert.True(t, out["is_active"].(bool))
}

func TestSubscription_DeletedAtOmittedWhenNil(t *testing.T) {
	sub := webhook.Subscription{
		ID:            uuid.New(),
		OrgID:         uuid.New(),
		Name:          "Active Sub",
		EventPatterns: []string{"quorant.fin.*"},
		TargetURL:     "https://example.com/hook",
		IsActive:      true,
		RetryPolicy:   webhook.DefaultRetryPolicy(),
		CreatedBy:     uuid.New(),
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	data, err := json.Marshal(sub)
	require.NoError(t, err)

	var out map[string]any
	require.NoError(t, json.Unmarshal(data, &out))

	_, hasDeletedAt := out["deleted_at"]
	assert.False(t, hasDeletedAt, "deleted_at should be omitted when nil")
}

func TestSubscription_DeletedAtPresentWhenSet(t *testing.T) {
	now := time.Now()
	sub := webhook.Subscription{
		ID:            uuid.New(),
		OrgID:         uuid.New(),
		Name:          "Deleted Sub",
		EventPatterns: []string{"quorant.fin.*"},
		TargetURL:     "https://example.com/hook",
		IsActive:      false,
		RetryPolicy:   webhook.DefaultRetryPolicy(),
		CreatedBy:     uuid.New(),
		CreatedAt:     now,
		UpdatedAt:     now,
		DeletedAt:     &now,
	}

	data, err := json.Marshal(sub)
	require.NoError(t, err)

	var out map[string]any
	require.NoError(t, json.Unmarshal(data, &out))

	_, hasDeletedAt := out["deleted_at"]
	assert.True(t, hasDeletedAt, "deleted_at should be present when set")
}

// ─── DefaultRetryPolicy ───────────────────────────────────────────────────────

func TestDefaultRetryPolicy_HasExpectedValues(t *testing.T) {
	policy := webhook.DefaultRetryPolicy()
	assert.Equal(t, 3, policy.MaxRetries)
	assert.Equal(t, []int{5, 30, 300}, policy.BackoffSeconds)
}

// ─── AvailableEventTypes ──────────────────────────────────────────────────────

func TestAvailableEventTypes_NotEmpty(t *testing.T) {
	types := webhook.AvailableEventTypes()
	assert.NotEmpty(t, types)
}

func TestAvailableEventTypes_ContainsExpectedEntries(t *testing.T) {
	types := webhook.AvailableEventTypes()
	typeSet := make(map[string]bool, len(types))
	for _, t := range types {
		typeSet[t] = true
	}

	expected := []string{
		"quorant.gov.ViolationCreated",
		"quorant.fin.PaymentReceived",
		"quorant.org.MembershipChanged",
		"quorant.billing.PaymentFailed",
		"quorant.ai.PolicyExtractionComplete",
	}
	for _, e := range expected {
		assert.True(t, typeSet[e], "expected event type %q to be in AvailableEventTypes", e)
	}
}

// ─── CreateSubscriptionRequest validation ────────────────────────────────────

func TestCreateSubscriptionRequest_Validate_Success(t *testing.T) {
	req := webhook.CreateSubscriptionRequest{
		Name:          "My Hook",
		EventPatterns: []string{"quorant.gov.*"},
		TargetURL:     "https://example.com/webhook",
	}
	assert.NoError(t, req.Validate())
}

func TestCreateSubscriptionRequest_Validate_MissingName(t *testing.T) {
	req := webhook.CreateSubscriptionRequest{
		EventPatterns: []string{"quorant.gov.*"},
		TargetURL:     "https://example.com/webhook",
	}
	err := req.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name")
}

func TestCreateSubscriptionRequest_Validate_MissingPatterns(t *testing.T) {
	req := webhook.CreateSubscriptionRequest{
		Name:      "My Hook",
		TargetURL: "https://example.com/webhook",
	}
	err := req.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "event_patterns")
}

func TestCreateSubscriptionRequest_Validate_MissingTargetURL(t *testing.T) {
	req := webhook.CreateSubscriptionRequest{
		Name:          "My Hook",
		EventPatterns: []string{"quorant.gov.*"},
	}
	err := req.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "target_url")
}

func TestCreateSubscriptionRequest_Validate_InvalidURL(t *testing.T) {
	req := webhook.CreateSubscriptionRequest{
		Name:          "My Hook",
		EventPatterns: []string{"quorant.gov.*"},
		TargetURL:     "not-a-url",
	}
	err := req.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "target_url")
}

func TestCreateSubscriptionRequest_Validate_HTTPAllowed(t *testing.T) {
	req := webhook.CreateSubscriptionRequest{
		Name:          "My Hook",
		EventPatterns: []string{"quorant.gov.*"},
		TargetURL:     "http://external.example.com/webhook",
	}
	assert.NoError(t, req.Validate())
}

// ─── SSRF protection ──────────────────────────────────────────────────────────

func TestCreateSubscriptionRequest_Validate_BlocksLocalhost(t *testing.T) {
	for _, rawURL := range []string{
		"http://localhost/hook",
		"http://localhost:8080/hook",
		"https://localhost/hook",
	} {
		req := webhook.CreateSubscriptionRequest{
			Name:          "Hook",
			EventPatterns: []string{"quorant.gov.*"},
			TargetURL:     rawURL,
		}
		err := req.Validate()
		require.Errorf(t, err, "expected SSRF block for %s", rawURL)
		assert.Contains(t, err.Error(), "target_url")
	}
}

func TestCreateSubscriptionRequest_Validate_BlocksPrivateIPv4(t *testing.T) {
	privateURLs := []string{
		"http://127.0.0.1/hook",
		"http://10.0.0.1/hook",
		"http://172.16.0.1/hook",
		"http://172.31.255.255/hook",
		"http://192.168.1.1/hook",
		"http://169.254.169.254/hook", // AWS metadata
	}
	for _, rawURL := range privateURLs {
		req := webhook.CreateSubscriptionRequest{
			Name:          "Hook",
			EventPatterns: []string{"quorant.gov.*"},
			TargetURL:     rawURL,
		}
		err := req.Validate()
		require.Errorf(t, err, "expected SSRF block for %s", rawURL)
		assert.Contains(t, err.Error(), "target_url")
	}
}

func TestCreateSubscriptionRequest_Validate_BlocksPrivateIPv6(t *testing.T) {
	for _, rawURL := range []string{
		"http://[::1]/hook",
		"http://[fc00::1]/hook",
	} {
		req := webhook.CreateSubscriptionRequest{
			Name:          "Hook",
			EventPatterns: []string{"quorant.gov.*"},
			TargetURL:     rawURL,
		}
		err := req.Validate()
		require.Errorf(t, err, "expected SSRF block for %s", rawURL)
		assert.Contains(t, err.Error(), "target_url")
	}
}

func TestCreateSubscriptionRequest_Validate_AllowsPublicIP(t *testing.T) {
	req := webhook.CreateSubscriptionRequest{
		Name:          "Hook",
		EventPatterns: []string{"quorant.gov.*"},
		TargetURL:     "https://8.8.8.8/hook",
	}
	assert.NoError(t, req.Validate())
}

// ─── RetryPolicy validation ───────────────────────────────────────────────────

func TestCreateSubscriptionRequest_Validate_RetryPolicyTooManyRetries(t *testing.T) {
	req := webhook.CreateSubscriptionRequest{
		Name:          "Hook",
		EventPatterns: []string{"quorant.gov.*"},
		TargetURL:     "https://example.com/hook",
		RetryPolicy: &webhook.RetryPolicy{
			MaxRetries:     11,
			BackoffSeconds: []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11},
		},
	}
	err := req.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "max_retries")
}

func TestCreateSubscriptionRequest_Validate_RetryPolicyInsufficientBackoff(t *testing.T) {
	req := webhook.CreateSubscriptionRequest{
		Name:          "Hook",
		EventPatterns: []string{"quorant.gov.*"},
		TargetURL:     "https://example.com/hook",
		RetryPolicy: &webhook.RetryPolicy{
			MaxRetries:     3,
			BackoffSeconds: []int{5, 30}, // only 2 entries for 3 retries
		},
	}
	err := req.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "backoff_seconds")
}

func TestCreateSubscriptionRequest_Validate_RetryPolicyValid(t *testing.T) {
	req := webhook.CreateSubscriptionRequest{
		Name:          "Hook",
		EventPatterns: []string{"quorant.gov.*"},
		TargetURL:     "https://example.com/hook",
		RetryPolicy: &webhook.RetryPolicy{
			MaxRetries:     3,
			BackoffSeconds: []int{5, 30, 300},
		},
	}
	assert.NoError(t, req.Validate())
}

func TestCreateSubscriptionRequest_Validate_RetryPolicyZeroRetries(t *testing.T) {
	req := webhook.CreateSubscriptionRequest{
		Name:          "Hook",
		EventPatterns: []string{"quorant.gov.*"},
		TargetURL:     "https://example.com/hook",
		RetryPolicy: &webhook.RetryPolicy{
			MaxRetries:     0,
			BackoffSeconds: []int{},
		},
	}
	assert.NoError(t, req.Validate())
}

// ─── UpdateSubscriptionRequest validation ────────────────────────────────────

func TestUpdateSubscriptionRequest_Validate_NoFieldsIsError(t *testing.T) {
	req := webhook.UpdateSubscriptionRequest{}
	err := req.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "validation.at_least_one")
}

func TestUpdateSubscriptionRequest_Validate_NameOnly(t *testing.T) {
	name := "New Name"
	req := webhook.UpdateSubscriptionRequest{Name: &name}
	assert.NoError(t, req.Validate())
}

func TestUpdateSubscriptionRequest_Validate_InvalidURL(t *testing.T) {
	badURL := "ftp://invalid"
	req := webhook.UpdateSubscriptionRequest{TargetURL: &badURL}
	err := req.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "target_url")
}
