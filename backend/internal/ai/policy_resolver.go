package ai

import (
	"context"

	"github.com/google/uuid"
)

// PostgresPolicyResolver implements PolicyResolver using PolicyService.
// GetPolicy performs a live database lookup. QueryPolicy is a placeholder
// that returns nil until full LLM inference is wired in a later phase.
type PostgresPolicyResolver struct {
	service *PolicyService
}

// NewPostgresPolicyResolver constructs a PostgresPolicyResolver backed by service.
func NewPostgresPolicyResolver(service *PolicyService) *PostgresPolicyResolver {
	return &PostgresPolicyResolver{service: service}
}

// GetPolicy implements PolicyResolver — looks up the active extraction for the policy key.
func (r *PostgresPolicyResolver) GetPolicy(ctx context.Context, orgID uuid.UUID, policyKey string) (*PolicyResult, error) {
	return r.service.GetActivePolicy(ctx, orgID, policyKey)
}

// QueryPolicy implements PolicyResolver — placeholder until LLM inference is built.
func (r *PostgresPolicyResolver) QueryPolicy(ctx context.Context, orgID uuid.UUID, query string, qctx QueryContext) (*ResolutionResult, error) {
	// Real AI inference will be added when LLM integration is built.
	return nil, nil
}
