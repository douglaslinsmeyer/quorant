package org

import (
	"context"

	"github.com/google/uuid"
)

// AmenityRepository manages amenities and reservations.
type AmenityRepository interface {
	// Amenity CRUD
	CreateAmenity(ctx context.Context, a *Amenity) (*Amenity, error)
	FindAmenityByID(ctx context.Context, id uuid.UUID) (*Amenity, error)
	// ListAmenitiesByOrg returns amenities for the org, supporting cursor-based pagination.
	// afterID is an optional cursor (ID of the last item from the previous page).
	// hasMore is true when additional items exist beyond the returned page.
	ListAmenitiesByOrg(ctx context.Context, orgID uuid.UUID, limit int, afterID *uuid.UUID) ([]Amenity, bool, error)
	UpdateAmenity(ctx context.Context, a *Amenity) (*Amenity, error)
	SoftDeleteAmenity(ctx context.Context, id uuid.UUID) error

	// Reservations
	CreateReservation(ctx context.Context, r *AmenityReservation) (*AmenityReservation, error)
	ListReservationsByAmenity(ctx context.Context, amenityID uuid.UUID) ([]AmenityReservation, error)
	ListReservationsByUser(ctx context.Context, orgID, userID uuid.UUID) ([]AmenityReservation, error)
	FindReservationByID(ctx context.Context, id uuid.UUID) (*AmenityReservation, error)
	UpdateReservation(ctx context.Context, r *AmenityReservation) (*AmenityReservation, error)
}
