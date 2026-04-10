package billing

import (
	"context"

	"github.com/google/uuid"
)

// Service defines the business operations for the billing module.
// Handlers depend on this interface rather than the concrete BillingService struct.
type Service interface {
	// Billing Accounts
	GetBillingAccount(ctx context.Context, orgID uuid.UUID) (*BillingAccount, error)
	UpdateBillingAccount(ctx context.Context, orgID uuid.UUID, req UpdateBillingAccountRequest) (*BillingAccount, error)

	// Invoices
	ListInvoices(ctx context.Context, orgID uuid.UUID) ([]Invoice, error)
	GetInvoice(ctx context.Context, id uuid.UUID) (*Invoice, error)
}
