package billing

import (
	"github.com/quorant/quorant/internal/platform/api"
)

// CreateBillingAccountRequest is the input for creating a new BillingAccount.
type CreateBillingAccountRequest struct {
	BillingEmail string  `json:"billing_email"`
	BillingName  *string `json:"billing_name,omitempty"`
}

// Validate ensures required fields are present and valid.
func (r CreateBillingAccountRequest) Validate() error {
	if r.BillingEmail == "" {
		return api.NewValidationError("validation.required", "billing_email", api.P("field", "billing_email"))
	}
	return nil
}

// UpdateBillingAccountRequest is the input for updating an existing BillingAccount.
// At least one field must be provided.
type UpdateBillingAccountRequest struct {
	BillingEmail     *string `json:"billing_email,omitempty"`
	BillingName      *string `json:"billing_name,omitempty"`
	StripeCustomerID *string `json:"stripe_customer_id,omitempty"`
}

// Validate ensures at least one field is present.
func (r UpdateBillingAccountRequest) Validate() error {
	if r.BillingEmail == nil && r.BillingName == nil && r.StripeCustomerID == nil {
		return api.NewValidationError("validation.at_least_one", "")
	}
	return nil
}
