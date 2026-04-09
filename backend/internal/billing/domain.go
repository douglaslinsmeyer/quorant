package billing

import (
	"time"

	"github.com/google/uuid"
)

// BillingAccount holds the SaaS billing details for an organization.
type BillingAccount struct {
	ID               uuid.UUID `json:"id"`
	OrgID            uuid.UUID `json:"org_id"`
	StripeCustomerID *string   `json:"stripe_customer_id,omitempty"`
	BillingEmail     string    `json:"billing_email"`
	BillingName      *string   `json:"billing_name,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// Invoice represents a billing invoice issued to an organization.
type Invoice struct {
	ID               uuid.UUID  `json:"id"`
	BillingAccountID uuid.UUID  `json:"billing_account_id"`
	OrgID            uuid.UUID  `json:"org_id"`
	StripeInvoiceID  *string    `json:"stripe_invoice_id,omitempty"`
	Status           string     `json:"status"` // draft, issued, paid, overdue, void
	SubtotalCents    int64      `json:"subtotal_cents"`
	TaxCents         int64      `json:"tax_cents"`
	TotalCents       int64      `json:"total_cents"`
	PeriodStart      time.Time  `json:"period_start"`
	PeriodEnd        time.Time  `json:"period_end"`
	DueDate          *time.Time `json:"due_date,omitempty"`
	PaidAt           *time.Time `json:"paid_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// InvoiceLineItem represents a single line on an invoice.
type InvoiceLineItem struct {
	ID             uuid.UUID      `json:"id"`
	InvoiceID      uuid.UUID      `json:"invoice_id"`
	Description    string         `json:"description"`
	Quantity       int            `json:"quantity"`
	UnitPriceCents int64          `json:"unit_price_cents"`
	TotalCents     int64          `json:"total_cents"`
	LineType       string         `json:"line_type"` // subscription, overage, credit
	Metadata       map[string]any `json:"metadata"`
	CreatedAt      time.Time      `json:"created_at"`
}
