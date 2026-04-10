package admin

import "github.com/quorant/quorant/internal/platform/api"

// CreateFeatureFlagRequest is the body for POST /admin/feature-flags.
type CreateFeatureFlagRequest struct {
	Key         string  `json:"key"`
	Description *string `json:"description,omitempty"`
	Enabled     bool    `json:"enabled"`
}

// Validate checks that required fields are present.
func (r CreateFeatureFlagRequest) Validate() error {
	if r.Key == "" {
		return api.NewValidationError("validation.required", "key", api.P("field", "key"))
	}
	return nil
}

// UpdateFeatureFlagRequest is the body for PATCH /admin/feature-flags/{flag_id}.
// At least one field must be provided.
type UpdateFeatureFlagRequest struct {
	Description *string `json:"description,omitempty"`
	Enabled     *bool   `json:"enabled,omitempty"`
}

// Validate checks that at least one field is provided.
func (r UpdateFeatureFlagRequest) Validate() error {
	if r.Description == nil && r.Enabled == nil {
		return api.NewValidationError("validation.at_least_one", "")
	}
	return nil
}

// SetFlagOverrideRequest is the body for POST /admin/feature-flags/{flag_id}/overrides.
type SetFlagOverrideRequest struct {
	OrgID   string `json:"org_id"`
	Enabled bool   `json:"enabled"`
}

// Validate checks that org_id is provided.
func (r SetFlagOverrideRequest) Validate() error {
	if r.OrgID == "" {
		return api.NewValidationError("validation.required", "org_id", api.P("field", "org_id"))
	}
	return nil
}
