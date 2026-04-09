package billing

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/platform/api"
)

// BillingService provides business logic for the billing domain.
type BillingService struct {
	repo   BillingRepository
	logger *slog.Logger
}

// NewBillingService constructs a BillingService backed by the given repository.
func NewBillingService(repo BillingRepository, logger *slog.Logger) *BillingService {
	return &BillingService{repo: repo, logger: logger}
}

// GetBillingAccount returns the billing account for the given org, or NotFoundError.
func (s *BillingService) GetBillingAccount(ctx context.Context, orgID uuid.UUID) (*BillingAccount, error) {
	acct, err := s.repo.FindAccountByOrg(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("billing service: GetBillingAccount: %w", err)
	}
	if acct == nil {
		return nil, api.NewNotFoundError(fmt.Sprintf("billing account for org %s not found", orgID))
	}
	return acct, nil
}

// UpdateBillingAccount applies the provided update request to the billing account.
func (s *BillingService) UpdateBillingAccount(ctx context.Context, orgID uuid.UUID, req UpdateBillingAccountRequest) (*BillingAccount, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	acct, err := s.GetBillingAccount(ctx, orgID)
	if err != nil {
		return nil, err
	}

	if req.BillingEmail != nil {
		acct.BillingEmail = *req.BillingEmail
	}
	if req.BillingName != nil {
		acct.BillingName = req.BillingName
	}
	if req.StripeCustomerID != nil {
		acct.StripeCustomerID = req.StripeCustomerID
	}

	updated, err := s.repo.UpdateAccount(ctx, acct)
	if err != nil {
		return nil, fmt.Errorf("billing service: UpdateBillingAccount: %w", err)
	}

	s.logger.InfoContext(ctx, "billing account updated", "org_id", orgID)
	return updated, nil
}

// ListInvoices returns all invoices for the given org.
func (s *BillingService) ListInvoices(ctx context.Context, orgID uuid.UUID) ([]Invoice, error) {
	invoices, err := s.repo.ListInvoicesByOrg(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("billing service: ListInvoices: %w", err)
	}
	return invoices, nil
}

// GetInvoice returns an invoice by ID, or NotFoundError if it does not exist.
func (s *BillingService) GetInvoice(ctx context.Context, id uuid.UUID) (*Invoice, error) {
	inv, err := s.repo.FindInvoiceByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("billing service: GetInvoice: %w", err)
	}
	if inv == nil {
		return nil, api.NewNotFoundError(fmt.Sprintf("invoice %s not found", id))
	}
	return inv, nil
}
