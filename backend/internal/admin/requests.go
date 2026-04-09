package admin

import "errors"

// CreateFeatureFlagRequest is the body for POST /admin/feature-flags.
type CreateFeatureFlagRequest struct {
	Key         string  `json:"key"`
	Description *string `json:"description,omitempty"`
	Enabled     bool    `json:"enabled"`
}

// Validate checks that required fields are present.
func (r CreateFeatureFlagRequest) Validate() error {
	if r.Key == "" {
		return errors.New("key is required")
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
		return errors.New("at least one of description or enabled must be provided")
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
		return errors.New("org_id is required")
	}
	return nil
}
