package org

import (
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/platform/api"
)

// ─── Amenity requests ────────────────────────────────────────────────────────

// CreateAmenityRequest is the request body for POST /organizations/{org_id}/amenities.
type CreateAmenityRequest struct {
	Name             string         `json:"name"`         // required
	AmenityType      string         `json:"amenity_type"` // required
	Description      *string        `json:"description,omitempty"`
	Location         *string        `json:"location,omitempty"`
	Capacity         *int           `json:"capacity,omitempty"`
	IsReservable     bool           `json:"is_reservable"`
	ReservationRules map[string]any `json:"reservation_rules,omitempty"`
	FeeCents         *int64         `json:"fee_cents,omitempty"`
	Hours            map[string]any `json:"hours,omitempty"`
	Metadata         map[string]any `json:"metadata,omitempty"`
}

// Validate checks that required fields are present.
func (r CreateAmenityRequest) Validate() error {
	if r.Name == "" {
		return api.NewValidationError("validation.required", "name", api.P("field", "name"))
	}
	if r.AmenityType == "" {
		return api.NewValidationError("validation.required", "amenity_type", api.P("field", "amenity_type"))
	}
	return nil
}

// UpdateAmenityRequest is the request body for PATCH /organizations/{org_id}/amenities/{amenity_id}.
type UpdateAmenityRequest struct {
	Name             *string        `json:"name,omitempty"`
	AmenityType      *string        `json:"amenity_type,omitempty"`
	Description      *string        `json:"description,omitempty"`
	Location         *string        `json:"location,omitempty"`
	Capacity         *int           `json:"capacity,omitempty"`
	IsReservable     *bool          `json:"is_reservable,omitempty"`
	ReservationRules map[string]any `json:"reservation_rules,omitempty"`
	FeeCents         *int64         `json:"fee_cents,omitempty"`
	Hours            map[string]any `json:"hours,omitempty"`
	Status           *string        `json:"status,omitempty"`
	Metadata         map[string]any `json:"metadata,omitempty"`
}

// CreateReservationRequest is the request body for
// POST /organizations/{org_id}/amenities/{amenity_id}/reservations.
type CreateReservationRequest struct {
	UserID     uuid.UUID `json:"user_id"`  // required
	UnitID     uuid.UUID `json:"unit_id"`  // required
	StartsAt   time.Time `json:"starts_at"` // required
	EndsAt     time.Time `json:"ends_at"`   // required
	GuestCount *int      `json:"guest_count,omitempty"`
	Notes      *string   `json:"notes,omitempty"`
}

// Validate checks that required fields are present.
func (r CreateReservationRequest) Validate() error {
	if r.UserID == (uuid.UUID{}) {
		return api.NewValidationError("validation.required", "user_id", api.P("field", "user_id"))
	}
	if r.UnitID == (uuid.UUID{}) {
		return api.NewValidationError("validation.required", "unit_id", api.P("field", "unit_id"))
	}
	if r.StartsAt.IsZero() {
		return api.NewValidationError("validation.required", "starts_at", api.P("field", "starts_at"))
	}
	if r.EndsAt.IsZero() {
		return api.NewValidationError("validation.required", "ends_at", api.P("field", "ends_at"))
	}
	return nil
}

// UpdateReservationRequest is the request body for PATCH /organizations/{org_id}/reservations/{id}.
type UpdateReservationRequest struct {
	Status     *string    `json:"status,omitempty"`
	StartsAt   *time.Time `json:"starts_at,omitempty"`
	EndsAt     *time.Time `json:"ends_at,omitempty"`
	GuestCount *int       `json:"guest_count,omitempty"`
	Notes      *string    `json:"notes,omitempty"`
}

// ─── Vendor requests ──────────────────────────────────────────────────────────

// CreateVendorRequest is the request body for POST /api/v1/vendors.
type CreateVendorRequest struct {
	Name            string         `json:"name"` // required
	ContactEmail    *string        `json:"contact_email,omitempty"`
	ContactPhone    *string        `json:"contact_phone,omitempty"`
	ServiceTypes    []string       `json:"service_types,omitempty"`
	LicenseNumber   *string        `json:"license_number,omitempty"`
	InsuranceExpiry *time.Time     `json:"insurance_expiry,omitempty"`
	Metadata        map[string]any `json:"metadata,omitempty"`
}

// Validate checks that required fields are present.
func (r CreateVendorRequest) Validate() error {
	if r.Name == "" {
		return api.NewValidationError("validation.required", "name", api.P("field", "name"))
	}
	return nil
}

// UpdateVendorRequest is the request body for PATCH /api/v1/vendors/{vendor_id}.
type UpdateVendorRequest struct {
	Name            *string        `json:"name,omitempty"`
	ContactEmail    *string        `json:"contact_email,omitempty"`
	ContactPhone    *string        `json:"contact_phone,omitempty"`
	ServiceTypes    []string       `json:"service_types,omitempty"`
	LicenseNumber   *string        `json:"license_number,omitempty"`
	InsuranceExpiry *time.Time     `json:"insurance_expiry,omitempty"`
	Metadata        map[string]any `json:"metadata,omitempty"`
}

// CreateVendorAssignmentRequest is the request body for
// POST /organizations/{org_id}/vendor-assignments.
type CreateVendorAssignmentRequest struct {
	VendorID     uuid.UUID `json:"vendor_id"`    // required
	ServiceScope string    `json:"service_scope"` // required
	ContractRef  *string   `json:"contract_ref,omitempty"`
}

// Validate checks that required fields are present.
func (r CreateVendorAssignmentRequest) Validate() error {
	if r.VendorID == (uuid.UUID{}) {
		return api.NewValidationError("validation.required", "vendor_id", api.P("field", "vendor_id"))
	}
	if r.ServiceScope == "" {
		return api.NewValidationError("validation.required", "service_scope", api.P("field", "service_scope"))
	}
	return nil
}

// ─── Registration requests ────────────────────────────────────────────────────

// CreateRegistrationTypeRequest is the request body for
// POST /organizations/{org_id}/registration-types.
type CreateRegistrationTypeRequest struct {
	Name             string         `json:"name"` // required
	Slug             string         `json:"slug"` // required
	Schema           map[string]any `json:"schema,omitempty"`
	MaxPerUnit       *int           `json:"max_per_unit,omitempty"`
	RequiresApproval bool           `json:"requires_approval"`
}

// Validate checks that required fields are present.
func (r CreateRegistrationTypeRequest) Validate() error {
	if r.Name == "" {
		return api.NewValidationError("validation.required", "name", api.P("field", "name"))
	}
	if r.Slug == "" {
		return api.NewValidationError("validation.required", "slug", api.P("field", "slug"))
	}
	return nil
}

// UpdateRegistrationTypeRequest is the request body for
// PATCH /organizations/{org_id}/registration-types/{id}.
type UpdateRegistrationTypeRequest struct {
	Name             *string        `json:"name,omitempty"`
	Slug             *string        `json:"slug,omitempty"`
	Schema           map[string]any `json:"schema,omitempty"`
	MaxPerUnit       *int           `json:"max_per_unit,omitempty"`
	RequiresApproval *bool          `json:"requires_approval,omitempty"`
	IsActive         *bool          `json:"is_active,omitempty"`
}

// CreateRegistrationRequest is the request body for
// POST /organizations/{org_id}/units/{unit_id}/registrations.
type CreateRegistrationRequest struct {
	UserID             uuid.UUID      `json:"user_id"`              // required
	RegistrationTypeID uuid.UUID      `json:"registration_type_id"` // required
	Data               map[string]any `json:"data,omitempty"`
	ExpiresAt          *time.Time     `json:"expires_at,omitempty"`
}

// Validate checks that required fields are present.
func (r CreateRegistrationRequest) Validate() error {
	if r.UserID == (uuid.UUID{}) {
		return api.NewValidationError("validation.required", "user_id", api.P("field", "user_id"))
	}
	if r.RegistrationTypeID == (uuid.UUID{}) {
		return api.NewValidationError("validation.required", "registration_type_id", api.P("field", "registration_type_id"))
	}
	return nil
}

// UpdateRegistrationRequest is the request body for
// PATCH /organizations/{org_id}/registrations/{id}.
type UpdateRegistrationRequest struct {
	Data      map[string]any `json:"data,omitempty"`
	Status    *string        `json:"status,omitempty"`
	ExpiresAt *time.Time     `json:"expires_at,omitempty"`
}

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
	Locale       string         `json:"locale,omitempty"`
	Timezone     string         `json:"timezone,omitempty"`
	CurrencyCode string         `json:"currency_code,omitempty"`
	Country      string         `json:"country,omitempty"`
	Settings     map[string]any `json:"settings,omitempty"`
}

// Validate checks that Name and Type are present and Type is a valid value.
// It also applies defaults for Locale, Timezone, CurrencyCode, and Country.
func (r *CreateOrgRequest) Validate() error {
	if r.Name == "" {
		return api.NewValidationError("validation.required", "name", api.P("field", "name"))
	}
	if r.Type == "" {
		return api.NewValidationError("validation.required", "type", api.P("field", "type"))
	}
	if r.Type != "firm" && r.Type != "hoa" {
		return api.NewValidationError("validation.one_of", "type", api.P("field", "type"), api.P("values", "firm, hoa"))
	}

	// Apply defaults.
	if r.Locale == "" {
		r.Locale = "en_US"
	}
	if r.Timezone == "" {
		r.Timezone = "UTC"
	}
	if r.CurrencyCode == "" {
		r.CurrencyCode = "USD"
	}
	if r.Country == "" {
		r.Country = "US"
	}

	// Validate locale format.
	if !isValidLocaleFormat(r.Locale) {
		return api.NewValidationError("validation.constraint", "locale",
			api.P("field", "locale"), api.P("constraint", "a valid locale (e.g., en_US)"))
	}

	// Validate timezone.
	if _, err := time.LoadLocation(r.Timezone); err != nil {
		return api.NewValidationError("validation.constraint", "timezone",
			api.P("field", "timezone"), api.P("constraint", "a valid IANA timezone"))
	}

	// Validate currency code.
	if len(r.CurrencyCode) != 3 || !isUpperAlpha(r.CurrencyCode) {
		return api.NewValidationError("validation.constraint", "currency_code",
			api.P("field", "currency_code"), api.P("constraint", "a 3-letter ISO 4217 code"))
	}

	// Validate country code.
	if len(r.Country) != 2 || !isUpperAlpha(r.Country) {
		return api.NewValidationError("validation.constraint", "country",
			api.P("field", "country"), api.P("constraint", "a 2-letter ISO 3166-1 code"))
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
	Locale       *string        `json:"locale,omitempty"`
	Timezone     *string        `json:"timezone,omitempty"`
	CurrencyCode *string        `json:"currency_code,omitempty"`
	Country      *string        `json:"country,omitempty"`
	Settings     map[string]any `json:"settings,omitempty"`
}

// Validate checks that at least one field is set and validates field constraints.
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
		r.Locale == nil &&
		r.Timezone == nil &&
		r.CurrencyCode == nil &&
		r.Country == nil &&
		r.Settings == nil {
		return api.NewValidationError("validation.at_least_one", "")
	}
	if r.Locale != nil && !isValidLocaleFormat(*r.Locale) {
		return api.NewValidationError("validation.constraint", "locale",
			api.P("field", "locale"), api.P("constraint", "a valid locale (e.g., en_US)"))
	}
	if r.Timezone != nil {
		if _, err := time.LoadLocation(*r.Timezone); err != nil {
			return api.NewValidationError("validation.constraint", "timezone",
				api.P("field", "timezone"), api.P("constraint", "a valid IANA timezone"))
		}
	}
	if r.CurrencyCode != nil && (len(*r.CurrencyCode) != 3 || !isUpperAlpha(*r.CurrencyCode)) {
		return api.NewValidationError("validation.constraint", "currency_code",
			api.P("field", "currency_code"), api.P("constraint", "a 3-letter ISO 4217 code"))
	}
	if r.Country != nil && (len(*r.Country) != 2 || !isUpperAlpha(*r.Country)) {
		return api.NewValidationError("validation.constraint", "country",
			api.P("field", "country"), api.P("constraint", "a 2-letter ISO 3166-1 code"))
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
		return api.NewValidationError("validation.required", "user_id", api.P("field", "user_id"))
	}
	if r.RoleID == (uuid.UUID{}) {
		return api.NewValidationError("validation.required", "role_id", api.P("field", "role_id"))
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
	Country      string         `json:"country,omitempty"`
	LotSizeSqft  *int           `json:"lot_size_sqft,omitempty"`
	VotingWeight *float64       `json:"voting_weight,omitempty"` // default 1.00
	Metadata     map[string]any `json:"metadata,omitempty"`
}

// Validate checks that Label is present and applies defaults.
func (r *CreateUnitRequest) Validate() error {
	if r.Label == "" {
		return api.NewValidationError("validation.required", "label", api.P("field", "label"))
	}
	if r.Country == "" {
		r.Country = "US"
	}
	return nil
}

// UpdateUnitRequest is the request body for PATCH /api/v1/organizations/{org_id}/units/{unit_id}.
type UpdateUnitRequest struct {
	Label        *string        `json:"label,omitempty"`
	UnitType     *string        `json:"unit_type,omitempty"`
	AddressLine1 *string        `json:"address_line1,omitempty"`
	AddressLine2 *string        `json:"address_line2,omitempty"`
	City         *string        `json:"city,omitempty"`
	State        *string        `json:"state,omitempty"`
	Zip          *string        `json:"zip,omitempty"`
	LotSizeSqft  *int           `json:"lot_size_sqft,omitempty"`
	VotingWeight *float64       `json:"voting_weight,omitempty"`
	Status       *string        `json:"status,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

// SetPropertyRequest is the request body for PUT /api/v1/organizations/{org_id}/units/{unit_id}/property.
type SetPropertyRequest struct {
	ParcelNumber *string        `json:"parcel_number,omitempty"`
	SquareFeet   *int           `json:"square_feet,omitempty"`
	Bedrooms     *int           `json:"bedrooms,omitempty"`
	Bathrooms    *float64       `json:"bathrooms,omitempty"`
	YearBuilt    *int           `json:"year_built,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

// CreateUnitMembershipRequest is the request body for
// POST /api/v1/organizations/{org_id}/units/{unit_id}/memberships.
type CreateUnitMembershipRequest struct {
	UserID       uuid.UUID `json:"user_id"`
	Relationship string    `json:"relationship"` // owner, tenant, resident, emergency_contact
	IsVoter      bool      `json:"is_voter"`
	Notes        *string   `json:"notes,omitempty"`
}

// Validate checks that UserID is non-zero and Relationship is valid.
func (r CreateUnitMembershipRequest) Validate() error {
	if r.UserID == (uuid.UUID{}) {
		return api.NewValidationError("validation.required", "user_id", api.P("field", "user_id"))
	}
	switch r.Relationship {
	case "owner", "tenant", "resident", "emergency_contact":
		// valid
	default:
		return api.NewValidationError("validation.one_of", "relationship", api.P("field", "relationship"), api.P("values", "owner, tenant, resident, emergency_contact"))
	}
	return nil
}

// UpdateUnitMembershipRequest is the request body for
// PATCH /api/v1/organizations/{org_id}/units/{unit_id}/memberships/{id}.
type UpdateUnitMembershipRequest struct {
	Relationship *string `json:"relationship,omitempty"`
	IsVoter      *bool   `json:"is_voter,omitempty"`
	Notes        *string `json:"notes,omitempty"`
}

// TransferOwnershipRequest is the request body for
// POST /api/v1/organizations/{org_id}/units/{unit_id}/transfer.
type TransferOwnershipRequest struct {
	ToUserID                uuid.UUID  `json:"to_user_id"`    // required
	TransferType            string     `json:"transfer_type"` // required: sale|gift|inheritance|foreclosure|other
	TransferDate            time.Time  `json:"transfer_date"` // required
	FromUserID              *uuid.UUID `json:"from_user_id,omitempty"`
	SalePriceCents          *int64     `json:"sale_price_cents,omitempty"`
	OutstandingBalanceCents *int64     `json:"outstanding_balance_cents,omitempty"`
	BalanceSettled          bool       `json:"balance_settled"`
	RecordingRef            *string    `json:"recording_ref,omitempty"`
	Notes                   *string    `json:"notes,omitempty"`
}

// Validate checks that the required fields are present.
func (r TransferOwnershipRequest) Validate() error {
	if r.ToUserID == (uuid.UUID{}) {
		return api.NewValidationError("validation.required", "to_user_id", api.P("field", "to_user_id"))
	}
	if r.TransferType == "" {
		return api.NewValidationError("validation.required", "transfer_type", api.P("field", "transfer_type"))
	}
	if r.TransferDate.IsZero() {
		return api.NewValidationError("validation.required", "transfer_date", api.P("field", "transfer_date"))
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
		return api.NewValidationError("validation.required", "firm_org_id", api.P("field", "firm_org_id"))
	}
	return nil
}

// isValidLocaleFormat checks that the string matches the pattern ll_CC
// (2 lowercase letters + underscore + 2 uppercase letters), e.g. "en_US".
func isValidLocaleFormat(locale string) bool {
	if len(locale) != 5 || locale[2] != '_' {
		return false
	}
	for i := 0; i < 2; i++ {
		if locale[i] < 'a' || locale[i] > 'z' {
			return false
		}
	}
	for i := 3; i < 5; i++ {
		if locale[i] < 'A' || locale[i] > 'Z' {
			return false
		}
	}
	return true
}

// isUpperAlpha returns true when every character in s is an uppercase ASCII letter.
func isUpperAlpha(s string) bool {
	for _, c := range s {
		if c < 'A' || c > 'Z' {
			return false
		}
	}
	return true
}
