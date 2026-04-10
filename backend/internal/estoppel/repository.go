package estoppel

import (
	"context"

	"github.com/google/uuid"
)

// EstoppelRepository defines persistence operations for estoppel requests and
// certificates.
type EstoppelRepository interface {
	// ─── Requests ─────────────────────────────────────────────────────────────

	// CreateRequest inserts a new estoppel request and returns the
	// fully-populated row.
	CreateRequest(ctx context.Context, r *EstoppelRequest) (*EstoppelRequest, error)

	// FindRequestByID returns the request with the given id, or nil, nil if
	// not found or soft-deleted.
	FindRequestByID(ctx context.Context, id uuid.UUID) (*EstoppelRequest, error)

	// ListRequestsByOrg returns non-deleted requests for the given org,
	// optionally filtered by status. Pagination is cursor-based ordered by
	// created_at DESC. afterID is the cursor from the previous page; passing nil
	// returns the first page. hasMore is true when additional items exist.
	ListRequestsByOrg(ctx context.Context, orgID uuid.UUID, status *string, limit int, afterID *uuid.UUID) ([]EstoppelRequest, bool, error)

	// UpdateRequestStatus updates the status field and updated_at for the given
	// request. Returns the updated row, or an error if not found.
	UpdateRequestStatus(ctx context.Context, id uuid.UUID, status string) (*EstoppelRequest, error)

	// UpdateRequestNarratives stores the generated narrative sections (as JSON)
	// against the request's metadata field. Returns the updated row.
	UpdateRequestNarratives(ctx context.Context, id uuid.UUID, narrativeSections []byte) (*EstoppelRequest, error)

	// ─── Certificates ─────────────────────────────────────────────────────────

	// CreateCertificate inserts a new estoppel certificate and returns the
	// fully-populated row.
	CreateCertificate(ctx context.Context, c *EstoppelCertificate) (*EstoppelCertificate, error)

	// FindCertificateByID returns the certificate with the given id, or
	// nil, nil if not found.
	FindCertificateByID(ctx context.Context, id uuid.UUID) (*EstoppelCertificate, error)

	// FindCertificateByRequestID returns the most recent certificate for the
	// given request, or nil, nil if none exists.
	FindCertificateByRequestID(ctx context.Context, requestID uuid.UUID) (*EstoppelCertificate, error)

	// ListCertificatesByOrg returns all certificates for the given org ordered
	// by effective_date DESC. Returns an empty (non-nil) slice when none exist.
	ListCertificatesByOrg(ctx context.Context, orgID uuid.UUID) ([]EstoppelCertificate, error)
}
