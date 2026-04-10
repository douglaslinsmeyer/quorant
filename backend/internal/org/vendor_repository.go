package org

import (
	"context"

	"github.com/google/uuid"
)

// VendorRepository manages vendors and their org assignments.
type VendorRepository interface {
	// Vendor CRUD
	CreateVendor(ctx context.Context, v *Vendor) (*Vendor, error)
	FindVendorByID(ctx context.Context, id uuid.UUID) (*Vendor, error)
	// ListVendors returns vendors, supporting cursor-based pagination.
	// afterID is an optional cursor (ID of the last item from the previous page).
	// hasMore is true when additional items exist beyond the returned page.
	ListVendors(ctx context.Context, limit int, afterID *uuid.UUID) ([]Vendor, bool, error)
	UpdateVendor(ctx context.Context, v *Vendor) (*Vendor, error)
	SoftDeleteVendor(ctx context.Context, id uuid.UUID) error

	// Vendor Assignments
	CreateAssignment(ctx context.Context, a *VendorAssignment) (*VendorAssignment, error)
	ListAssignmentsByOrg(ctx context.Context, orgID uuid.UUID) ([]VendorAssignment, error)
	DeleteAssignment(ctx context.Context, id uuid.UUID) error
}
