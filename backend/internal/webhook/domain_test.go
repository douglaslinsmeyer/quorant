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
		TargetURL:     "http://localhost:8080/webhook",
	}
	assert.NoError(t, req.Validate())
}

// ─── UpdateSubscriptionRequest validation ────────────────────────────────────

func TestUpdateSubscriptionRequest_Validate_NoFieldsIsError(t *testing.T) {
	req := webhook.UpdateSubscriptionRequest{}
	err := req.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one field")
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
