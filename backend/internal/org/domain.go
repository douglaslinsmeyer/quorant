package org

import (
	"time"

	"github.com/google/uuid"
)

// Organization represents a firm or HOA.
type Organization struct {
	ID           uuid.UUID      `json:"id"`
	ParentID     *uuid.UUID     `json:"parent_id,omitempty"`
	Type         string         `json:"type"`     // "firm" or "hoa"
	Name         string         `json:"name"`
	Slug         string         `json:"slug"`
	Path         string         `json:"path"` // ltree path
	AddressLine1 *string        `json:"address_line1,omitempty"`
	AddressLine2 *string        `json:"address_line2,omitempty"`
	City         *string        `json:"city,omitempty"`
	State        *string        `json:"state,omitempty"`
	Zip          *string        `json:"zip,omitempty"`
	Phone        *string        `json:"phone,omitempty"`
	Email        *string        `json:"email,omitempty"`
	Website      *string        `json:"website,omitempty"`
	LogoURL      *string        `json:"logo_url,omitempty"`
	Settings     map[string]any `json:"settings"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    *time.Time     `json:"deleted_at,omitempty"`
}

// OrgManagement represents a firm-HOA management relationship.
type OrgManagement struct {
	ID        uuid.UUID  `json:"id"`
	FirmOrgID uuid.UUID  `json:"firm_org_id"`
	HOAOrgID  uuid.UUID  `json:"hoa_org_id"`
	StartedAt time.Time  `json:"started_at"`
	EndedAt   *time.Time `json:"ended_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// Unit represents a lot, condo unit, or townhouse within an HOA.
type Unit struct {
	ID           uuid.UUID      `json:"id"`
	OrgID        uuid.UUID      `json:"org_id"`
	Label        string         `json:"label"`
	UnitType     *string        `json:"unit_type,omitempty"`
	AddressLine1 *string        `json:"address_line1,omitempty"`
	AddressLine2 *string        `json:"address_line2,omitempty"`
	City         *string        `json:"city,omitempty"`
	State        *string        `json:"state,omitempty"`
	Zip          *string        `json:"zip,omitempty"`
	Status       string         `json:"status"`
	LotSizeSqft  *int           `json:"lot_size_sqft,omitempty"`
	VotingWeight float64        `json:"voting_weight"`
	Metadata     map[string]any `json:"metadata"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    *time.Time     `json:"deleted_at,omitempty"`
}

// Property represents physical property details for a unit.
type Property struct {
	ID           uuid.UUID      `json:"id"`
	UnitID       uuid.UUID      `json:"unit_id"`
	ParcelNumber *string        `json:"parcel_number,omitempty"`
	SquareFeet   *int           `json:"square_feet,omitempty"`
	Bedrooms     *int           `json:"bedrooms,omitempty"`
	Bathrooms    *float64       `json:"bathrooms,omitempty"`
	YearBuilt    *int           `json:"year_built,omitempty"`
	Metadata     map[string]any `json:"metadata"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

// UnitMembership links a user to a unit (owner, tenant, resident).
type UnitMembership struct {
	ID           uuid.UUID  `json:"id"`
	UnitID       uuid.UUID  `json:"unit_id"`
	UserID       uuid.UUID  `json:"user_id"`
	Relationship string     `json:"relationship"` // 'owner', 'tenant', 'resident', 'emergency_contact'
	IsVoter      bool       `json:"is_voter"`
	StartedAt    time.Time  `json:"started_at"`
	EndedAt      *time.Time `json:"ended_at,omitempty"`
	Notes        *string    `json:"notes,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// UnitOwnershipHistory records a property transfer event for a unit.
type UnitOwnershipHistory struct {
	ID                     uuid.UUID  `json:"id"`
	UnitID                 uuid.UUID  `json:"unit_id"`
	OrgID                  uuid.UUID  `json:"org_id"`
	FromUserID             *uuid.UUID `json:"from_user_id,omitempty"`
	ToUserID               uuid.UUID  `json:"to_user_id"`
	TransferType           string     `json:"transfer_type"`
	TransferDate           time.Time  `json:"transfer_date"`
	SalePriceCents         *int64     `json:"sale_price_cents,omitempty"`
	OutstandingBalanceCents *int64    `json:"outstanding_balance_cents,omitempty"`
	BalanceSettled         bool       `json:"balance_settled"`
	RecordingRef           *string    `json:"recording_ref,omitempty"`
	Notes                  *string    `json:"notes,omitempty"`
	RecordedBy             uuid.UUID  `json:"recorded_by"`
	CreatedAt              time.Time  `json:"created_at"`
}

// Vendor represents an external service provider.
type Vendor struct {
	ID              uuid.UUID      `json:"id"`
	Name            string         `json:"name"`
	ContactEmail    *string        `json:"contact_email,omitempty"`
	ContactPhone    *string        `json:"contact_phone,omitempty"`
	ServiceTypes    []string       `json:"service_types"`
	LicenseNumber   *string        `json:"license_number,omitempty"`
	InsuranceExpiry *time.Time     `json:"insurance_expiry,omitempty"`
	Metadata        map[string]any `json:"metadata"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       *time.Time     `json:"deleted_at,omitempty"`
}

// Amenity represents a community amenity (pool, clubhouse, etc.).
type Amenity struct {
	ID               uuid.UUID      `json:"id"`
	OrgID            uuid.UUID      `json:"org_id"`
	Name             string         `json:"name"`
	AmenityType      string         `json:"amenity_type"`
	Description      *string        `json:"description,omitempty"`
	Location         *string        `json:"location,omitempty"`
	Capacity         *int           `json:"capacity,omitempty"`
	IsReservable     bool           `json:"is_reservable"`
	ReservationRules map[string]any `json:"reservation_rules"`
	FeeCents         *int64         `json:"fee_cents,omitempty"`
	Hours            map[string]any `json:"hours"`
	Status           string         `json:"status"`
	Metadata         map[string]any `json:"metadata"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	DeletedAt        *time.Time     `json:"deleted_at,omitempty"`
}
