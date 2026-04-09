package org

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/estoppel"
)

// EstoppelPropertyAdapter wraps OrgService to satisfy the
// estoppel.PropertyDataProvider interface.
type EstoppelPropertyAdapter struct {
	service *OrgService
}

// NewEstoppelPropertyAdapter returns a new EstoppelPropertyAdapter backed by
// the provided OrgService.
func NewEstoppelPropertyAdapter(service *OrgService) *EstoppelPropertyAdapter {
	return &EstoppelPropertyAdapter{service: service}
}

// GetPropertySnapshot builds a PropertySnapshot for the given unit by calling
// OrgService methods. Fields not available from existing service methods are
// left at their zero values.
func (a *EstoppelPropertyAdapter) GetPropertySnapshot(ctx context.Context, orgID, unitID uuid.UUID) (*estoppel.PropertySnapshot, error) {
	snap := &estoppel.PropertySnapshot{}

	// ── Unit ──────────────────────────────────────────────────────────────────
	unit, err := a.service.GetUnit(ctx, unitID)
	if err != nil {
		return nil, err
	}
	snap.UnitNumber = unit.Label
	if unit.UnitType != nil {
		snap.UnitType = *unit.UnitType
	}
	snap.Address = buildUnitAddress(unit)

	// ── Property details ──────────────────────────────────────────────────────
	prop, err := a.service.GetProperty(ctx, unitID)
	if err == nil && prop != nil {
		if prop.ParcelNumber != nil {
			snap.ParcelNumber = *prop.ParcelNumber
		}
		if prop.SquareFeet != nil {
			snap.SquareFootage = float64(*prop.SquareFeet)
		}
		if prop.Bedrooms != nil {
			snap.Bedrooms = *prop.Bedrooms
		}
		if prop.Bathrooms != nil {
			snap.Bathrooms = *prop.Bathrooms
		}
	}
	// If GetProperty returns a NotFoundError we proceed with zero values for
	// property-specific fields; this is intentional for units without a property record.

	// ── Unit memberships → owners ─────────────────────────────────────────────
	memberships, err := a.service.ListUnitMemberships(ctx, unitID)
	if err != nil {
		return nil, err
	}
	for _, m := range memberships {
		// Active memberships only (no ended_at).
		if m.EndedAt != nil {
			continue
		}
		owner := estoppel.OwnerInfo{
			IsOccupant: m.Relationship == "owner" || m.Relationship == "resident",
		}
		// UserID is available but we have no method to resolve user details
		// without importing the iam/user package. Leave name/email/phone empty.
		// The caller can enrich these fields if needed.
		_ = m.UserID
		snap.Owners = append(snap.Owners, owner)

		if m.Relationship == "tenant" {
			snap.IsRental = true
		}
	}

	// Fields not available from existing service methods and left as zero values:
	//   LegalDescription, ParkingSpaces, StorageUnits,
	//   PetRestrictions, LeaseOnFile,
	//   Owner Name/Email/Phone/MailingAddress (requires user lookup).

	return snap, nil
}

// buildUnitAddress constructs a single-line address string from a Unit's
// optional address fields.
func buildUnitAddress(unit *Unit) string {
	parts := make([]string, 0, 5)
	if unit.AddressLine1 != nil {
		parts = append(parts, *unit.AddressLine1)
	}
	if unit.AddressLine2 != nil {
		parts = append(parts, *unit.AddressLine2)
	}
	if unit.City != nil {
		parts = append(parts, *unit.City)
	}
	if unit.State != nil && unit.Zip != nil {
		parts = append(parts, fmt.Sprintf("%s %s", *unit.State, *unit.Zip))
	} else if unit.State != nil {
		parts = append(parts, *unit.State)
	} else if unit.Zip != nil {
		parts = append(parts, *unit.Zip)
	}
	return strings.Join(parts, ", ")
}
