package webhook

import (
	"net/url"
	"strings"

	"github.com/quorant/quorant/internal/platform/api"
)

// CreateSubscriptionRequest is the input for creating a new webhook subscription.
type CreateSubscriptionRequest struct {
	Name          string            `json:"name"`
	EventPatterns []string          `json:"event_patterns"`
	TargetURL     string            `json:"target_url"`
	Headers       map[string]string `json:"headers,omitempty"`
	RetryPolicy   *RetryPolicy      `json:"retry_policy,omitempty"`
}

// Validate ensures required fields are present and the target URL is valid.
func (r CreateSubscriptionRequest) Validate() error {
	if strings.TrimSpace(r.Name) == "" {
		return api.NewValidationError("name is required", "name")
	}
	if len(r.EventPatterns) == 0 {
		return api.NewValidationError("event_patterns must contain at least one pattern", "event_patterns")
	}
	if strings.TrimSpace(r.TargetURL) == "" {
		return api.NewValidationError("target_url is required", "target_url")
	}
	if err := validateTargetURL(r.TargetURL); err != nil {
		return err
	}
	return nil
}

// UpdateSubscriptionRequest is the input for updating an existing webhook subscription.
// At least one field must be provided.
type UpdateSubscriptionRequest struct {
	Name          *string            `json:"name,omitempty"`
	EventPatterns []string           `json:"event_patterns,omitempty"`
	TargetURL     *string            `json:"target_url,omitempty"`
	IsActive      *bool              `json:"is_active,omitempty"`
	Headers       map[string]string  `json:"headers,omitempty"`
}

// Validate ensures at least one field is provided and any supplied URL is valid.
func (r UpdateSubscriptionRequest) Validate() error {
	if r.Name == nil && len(r.EventPatterns) == 0 && r.TargetURL == nil && r.IsActive == nil && r.Headers == nil {
		return api.NewValidationError("at least one field must be provided", "")
	}
	if r.TargetURL != nil {
		if err := validateTargetURL(*r.TargetURL); err != nil {
			return err
		}
	}
	return nil
}

// TestEventRequest is the input for triggering a test delivery.
// It is intentionally empty — the service generates a synthetic payload.
type TestEventRequest struct{}

// validateTargetURL checks that the URL is well-formed and uses https (or http for dev).
func validateTargetURL(rawURL string) error {
	u, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return api.NewValidationError("target_url must be a valid URL", "target_url")
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return api.NewValidationError("target_url must use http or https scheme", "target_url")
	}
	if u.Host == "" {
		return api.NewValidationError("target_url must include a host", "target_url")
	}
	return nil
}
