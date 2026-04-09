package billing

import (
	"context"

	"github.com/google/uuid"
)

// BillingRepository defines persistence operations for the billing domain.
type BillingRepository interface {
	// Accounts
	CreateAccount(ctx context.Context, a *BillingAccount) (*BillingAccount, error)
	FindAccountByOrg(ctx context.Context, orgID uuid.UUID) (*BillingAccount, error)
	UpdateAccount(ctx context.Context, a *BillingAccount) (*BillingAccount, error)

	// Invoices
	CreateInvoice(ctx context.Context, inv *Invoice) (*Invoice, error)
	FindInvoiceByID(ctx context.Context, id uuid.UUID) (*Invoice, error)
	ListInvoicesByOrg(ctx context.Context, orgID uuid.UUID) ([]Invoice, error)
	UpdateInvoice(ctx context.Context, inv *Invoice) (*Invoice, error)

	// Line Items
	CreateLineItem(ctx context.Context, item *InvoiceLineItem) (*InvoiceLineItem, error)
	ListLineItemsByInvoice(ctx context.Context, invoiceID uuid.UUID) ([]InvoiceLineItem, error)
}
