package webhook

import (
	"net"
	"net/url"
	"strings"

	"github.com/quorant/quorant/internal/platform/api"
)

const maxRetries = 10

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
	if r.RetryPolicy != nil {
		if err := validateRetryPolicy(r.RetryPolicy); err != nil {
			return err
		}
	}
	return nil
}

// UpdateSubscriptionRequest is the input for updating an existing webhook subscription.
// At least one field must be provided.
type UpdateSubscriptionRequest struct {
	Name          *string           `json:"name,omitempty"`
	EventPatterns []string          `json:"event_patterns,omitempty"`
	TargetURL     *string           `json:"target_url,omitempty"`
	IsActive      *bool             `json:"is_active,omitempty"`
	Headers       map[string]string `json:"headers,omitempty"`
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

// validateTargetURL checks that the URL is well-formed, uses http/https, and
// does not target private/reserved IP ranges (SSRF protection).
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
	// Strip port before IP check.
	hostname := u.Hostname()
	if isPrivateHost(hostname) {
		return api.NewValidationError("target_url must not point to a private or reserved address", "target_url")
	}
	return nil
}

// privateIPNets lists IP ranges that must not be reachable via webhooks.
var privateIPNets = func() []*net.IPNet {
	cidrs := []string{
		"127.0.0.0/8",    // loopback
		"10.0.0.0/8",     // RFC1918
		"172.16.0.0/12",  // RFC1918
		"192.168.0.0/16", // RFC1918
		"169.254.0.0/16", // link-local / AWS metadata
		"::1/128",        // IPv6 loopback
		"fc00::/7",       // IPv6 unique local
	}
	nets := make([]*net.IPNet, 0, len(cidrs))
	for _, c := range cidrs {
		_, ipNet, _ := net.ParseCIDR(c)
		nets = append(nets, ipNet)
	}
	return nets
}()

// isPrivateHost returns true if hostname is a literal IP in a private/reserved range.
// Non-IP hostnames (DNS names) pass; DNS resolution is deferred to a future hardening step.
func isPrivateHost(hostname string) bool {
	// Block the "localhost" keyword regardless of resolution.
	if strings.EqualFold(hostname, "localhost") {
		return true
	}
	ip := net.ParseIP(hostname)
	if ip == nil {
		return false // hostname; allow (DNS resolution hardening is a separate step)
	}
	for _, ipNet := range privateIPNets {
		if ipNet.Contains(ip) {
			return true
		}
	}
	return false
}

// validateRetryPolicy checks MaxRetries bounds and that BackoffSeconds covers all retries.
func validateRetryPolicy(p *RetryPolicy) error {
	if p.MaxRetries < 0 {
		return api.NewValidationError("retry_policy.max_retries must be non-negative", "retry_policy.max_retries")
	}
	if p.MaxRetries > maxRetries {
		return api.NewValidationError("retry_policy.max_retries must not exceed 10", "retry_policy.max_retries")
	}
	if len(p.BackoffSeconds) < p.MaxRetries {
		return api.NewValidationError("retry_policy.backoff_seconds must have at least max_retries entries", "retry_policy.backoff_seconds")
	}
	return nil
}
