package org

import (
	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/platform/api"
)

// CreateOrgRequest is the request body for POST /api/v1/organizations.
type CreateOrgRequest struct {
	ParentID     *uuid.UUID     `json:"parent_id,omitempty"`
	Type         string         `json:"type"`    // required: "firm" or "hoa"
	Name         string         `json:"name"`    // required
	AddressLine1 *string        `json:"address_line1,omitempty"`
	AddressLine2 *string        `json:"address_line2,omitempty"`
	City         *string        `json:"city,omitempty"`
	State        *string        `json:"state,omitempty"`
	Zip          *string        `json:"zip,omitempty"`
	Phone        *string        `json:"phone,omitempty"`
	Email        *string        `json:"email,omitempty"`
	Website      *string        `json:"website,omitempty"`
	Settings     map[string]any `json:"settings,omitempty"`
}

// Validate checks that Name and Type are present and Type is a valid value.
func (r CreateOrgRequest) Validate() error {
	if r.Name == "" {
		return api.NewValidationError("name is required", "name")
	}
	if r.Type == "" {
		return api.NewValidationError("type is required", "type")
	}
	if r.Type != "firm" && r.Type != "hoa" {
		return api.NewValidationError(`type must be "firm" or "hoa"`, "type")
	}
	return nil
}

// UpdateOrgRequest is the request body for PATCH /api/v1/organizations/{id}.
type UpdateOrgRequest struct {
	Name         *string        `json:"name,omitempty"`
	AddressLine1 *string        `json:"address_line1,omitempty"`
	AddressLine2 *string        `json:"address_line2,omitempty"`
	City         *string        `json:"city,omitempty"`
	State        *string        `json:"state,omitempty"`
	Zip          *string        `json:"zip,omitempty"`
	Phone        *string        `json:"phone,omitempty"`
	Email        *string        `json:"email,omitempty"`
	Website      *string        `json:"website,omitempty"`
	LogoURL      *string        `json:"logo_url,omitempty"`
	Settings     map[string]any `json:"settings,omitempty"`
}

// Validate checks that at least one field is set.
func (r UpdateOrgRequest) Validate() error {
	if r.Name == nil &&
		r.AddressLine1 == nil &&
		r.AddressLine2 == nil &&
		r.City == nil &&
		r.State == nil &&
		r.Zip == nil &&
		r.Phone == nil &&
		r.Email == nil &&
		r.Website == nil &&
		r.LogoURL == nil &&
		r.Settings == nil {
		return api.NewValidationError("at least one field must be provided", "")
	}
	return nil
}

// CreateMembershipRequest is the request body for
// POST /api/v1/organizations/{org_id}/memberships.
type CreateMembershipRequest struct {
	UserID uuid.UUID `json:"user_id"` // required — the user to add
	RoleID uuid.UUID `json:"role_id"` // required — which role to assign
}

// Validate checks that UserID and RoleID are non-zero.
func (r CreateMembershipRequest) Validate() error {
	if r.UserID == (uuid.UUID{}) {
		return api.NewValidationError("user_id is required", "user_id")
	}
	if r.RoleID == (uuid.UUID{}) {
		return api.NewValidationError("role_id is required", "role_id")
	}
	return nil
}

// UpdateMembershipRequest is the request body for
// PATCH /api/v1/organizations/{org_id}/memberships/{membership_id}.
type UpdateMembershipRequest struct {
	RoleID *uuid.UUID `json:"role_id,omitempty"`
	Status *string    `json:"status,omitempty"`
}

// CreateUnitRequest is the request body for
// POST /api/v1/organizations/{org_id}/units.
type CreateUnitRequest struct {
	Label        string         `json:"label"`    // required
	UnitType     *string        `json:"unit_type,omitempty"`
	AddressLine1 *string        `json:"address_line1,omitempty"`
	AddressLine2 *string        `json:"address_line2,omitempty"`
	City         *string        `json:"city,omitempty"`
	State        *string        `json:"state,omitempty"`
	Zip          *string        `json:"zip,omitempty"`
	LotSizeSqft  *int           `json:"lot_size_sqft,omitempty"`
	VotingWeight *float64       `json:"voting_weight,omitempty"` // default 1.00
	Metadata     map[string]any `json:"metadata,omitempty"`
}

// Validate checks that Label is present.
func (r CreateUnitRequest) Validate() error {
	if r.Label == "" {
		return api.NewValidationError("label is required", "label")
	}
	return nil
}

// ConnectManagementRequest is the request body for
// POST /api/v1/organizations/{org_id}/management.
type ConnectManagementRequest struct {
	FirmOrgID uuid.UUID `json:"firm_org_id"` // required
}

// Validate checks that FirmOrgID is non-zero.
func (r ConnectManagementRequest) Validate() error {
	if r.FirmOrgID == (uuid.UUID{}) {
		return api.NewValidationError("firm_org_id is required", "firm_org_id")
	}
	return nil
}
