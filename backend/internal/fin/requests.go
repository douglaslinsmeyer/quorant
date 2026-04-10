package fin

import (
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/platform/api"
)

// CreateAssessmentScheduleRequest is the request body for creating a recurring
// assessment schedule.
type CreateAssessmentScheduleRequest struct {
	Name            string         `json:"name"`             // required
	Description     *string        `json:"description,omitempty"`
	Frequency       string         `json:"frequency"`        // required: monthly|quarterly|annually|semi_annually
	AmountStrategy  string         `json:"amount_strategy"`  // required: flat|per_unit_type|per_sqft|custom
	BaseAmountCents int64          `json:"base_amount_cents"` // required
	AmountRules     map[string]any `json:"amount_rules,omitempty"`
	DayOfMonth      *int           `json:"day_of_month,omitempty"`
	GraceDays       *int           `json:"grace_days,omitempty"`
	StartsAt        time.Time      `json:"starts_at"` // required
	EndsAt          *time.Time     `json:"ends_at,omitempty"`
}

// Validate checks that all required fields are present and have valid values.
func (r CreateAssessmentScheduleRequest) Validate() error {
	if r.Name == "" {
		return api.NewValidationError("validation.required", "name", api.P("field", "name"))
	}
	switch r.Frequency {
	case "monthly", "quarterly", "annually", "semi_annually":
		// valid
	case "":
		return api.NewValidationError("validation.required", "frequency", api.P("field", "frequency"))
	default:
		return api.NewValidationError("validation.one_of", "frequency", api.P("field", "frequency"), api.P("values", "monthly, quarterly, annually, semi_annually"))
	}
	switch r.AmountStrategy {
	case "flat", "per_unit_type", "per_sqft", "custom":
		// valid
	case "":
		return api.NewValidationError("validation.required", "amount_strategy", api.P("field", "amount_strategy"))
	default:
		return api.NewValidationError("validation.one_of", "amount_strategy", api.P("field", "amount_strategy"), api.P("values", "flat, per_unit_type, per_sqft, custom"))
	}
	if r.BaseAmountCents <= 0 {
		return api.NewValidationError("validation.required", "base_amount_cents", api.P("field", "base_amount_cents"))
	}
	if r.StartsAt.IsZero() {
		return api.NewValidationError("validation.required", "starts_at", api.P("field", "starts_at"))
	}
	return nil
}

// CreateAssessmentRequest is the request body for creating a single assessment
// for a unit.
type CreateAssessmentRequest struct {
	UnitID      uuid.UUID `json:"unit_id"`      // required
	Description string    `json:"description"`  // required
	AmountCents int64     `json:"amount_cents"` // required
	DueDate     time.Time `json:"due_date"`     // required
	GraceDays   *int      `json:"grace_days,omitempty"`
}

// Validate checks that all required fields are present.
func (r CreateAssessmentRequest) Validate() error {
	if r.UnitID == (uuid.UUID{}) {
		return api.NewValidationError("validation.required", "unit_id", api.P("field", "unit_id"))
	}
	if r.Description == "" {
		return api.NewValidationError("validation.required", "description", api.P("field", "description"))
	}
	if r.AmountCents <= 0 {
		return api.NewValidationError("validation.required", "amount_cents", api.P("field", "amount_cents"))
	}
	if r.DueDate.IsZero() {
		return api.NewValidationError("validation.required", "due_date", api.P("field", "due_date"))
	}
	return nil
}

// CreatePaymentRequest is the request body for recording a payment.
type CreatePaymentRequest struct {
	UnitID          uuid.UUID  `json:"unit_id"`            // required
	AmountCents     int64      `json:"amount_cents"`       // required, positive
	PaymentMethodID *uuid.UUID `json:"payment_method_id,omitempty"`
	Description     *string    `json:"description,omitempty"`
}

// Validate checks that unit_id is set and amount_cents is positive.
func (r CreatePaymentRequest) Validate() error {
	if r.UnitID == (uuid.UUID{}) {
		return api.NewValidationError("validation.required", "unit_id", api.P("field", "unit_id"))
	}
	if r.AmountCents <= 0 {
		return api.NewValidationError("validation.constraint", "amount_cents", api.P("field", "amount_cents"), api.P("constraint", "positive"))
	}
	return nil
}

// CreateBudgetRequest is the request body for creating a new budget.
type CreateBudgetRequest struct {
	FiscalYear int    `json:"fiscal_year"` // required
	Name       string `json:"name"`        // required
	Notes      *string `json:"notes,omitempty"`
}

// Validate checks that fiscal_year and name are set.
func (r CreateBudgetRequest) Validate() error {
	if r.FiscalYear == 0 {
		return api.NewValidationError("validation.required", "fiscal_year", api.P("field", "fiscal_year"))
	}
	if r.Name == "" {
		return api.NewValidationError("validation.required", "name", api.P("field", "name"))
	}
	return nil
}

// CreateExpenseRequest is the request body for recording an expense.
type CreateExpenseRequest struct {
	Description string     `json:"description"`   // required
	AmountCents int64      `json:"amount_cents"`  // required
	ExpenseDate time.Time  `json:"expense_date"`  // required
	FundType    *string    `json:"fund_type,omitempty"`
	VendorID    *uuid.UUID `json:"vendor_id,omitempty"`
	CategoryID  *uuid.UUID `json:"category_id,omitempty"`
	BudgetID    *uuid.UUID `json:"budget_id,omitempty"`
	DueDate     *time.Time `json:"due_date,omitempty"`
}

// Validate checks that description, amount_cents, and expense_date are set.
func (r CreateExpenseRequest) Validate() error {
	if r.Description == "" {
		return api.NewValidationError("validation.required", "description", api.P("field", "description"))
	}
	if r.AmountCents <= 0 {
		return api.NewValidationError("validation.required", "amount_cents", api.P("field", "amount_cents"))
	}
	if r.ExpenseDate.IsZero() {
		return api.NewValidationError("validation.required", "expense_date", api.P("field", "expense_date"))
	}
	return nil
}

// CreateFundRequest is the request body for creating a new fund.
type CreateFundRequest struct {
	Name               string  `json:"name"`      // required
	FundType           string  `json:"fund_type"` // required: operating|reserve|capital|special
	TargetBalanceCents *int64  `json:"target_balance_cents,omitempty"`
}

// Validate checks that name and fund_type are present and fund_type is valid.
func (r CreateFundRequest) Validate() error {
	if r.Name == "" {
		return api.NewValidationError("validation.required", "name", api.P("field", "name"))
	}
	switch r.FundType {
	case "operating", "reserve", "capital", "special":
		// valid
	case "":
		return api.NewValidationError("validation.required", "fund_type", api.P("field", "fund_type"))
	default:
		return api.NewValidationError("validation.one_of", "fund_type", api.P("field", "fund_type"), api.P("values", "operating, reserve, capital, special"))
	}
	return nil
}

// CreateFundTransferRequest is the request body for transferring money between
// funds.
type CreateFundTransferRequest struct {
	FromFundID  uuid.UUID `json:"from_fund_id"` // required
	ToFundID    uuid.UUID `json:"to_fund_id"`   // required
	AmountCents int64     `json:"amount_cents"` // required, positive
	Description *string   `json:"description,omitempty"`
}

// Validate checks that both fund IDs and a positive amount are set.
func (r CreateFundTransferRequest) Validate() error {
	if r.FromFundID == (uuid.UUID{}) {
		return api.NewValidationError("validation.required", "from_fund_id", api.P("field", "from_fund_id"))
	}
	if r.ToFundID == (uuid.UUID{}) {
		return api.NewValidationError("validation.required", "to_fund_id", api.P("field", "to_fund_id"))
	}
	if r.AmountCents <= 0 {
		return api.NewValidationError("validation.constraint", "amount_cents", api.P("field", "amount_cents"), api.P("constraint", "positive"))
	}
	return nil
}

// UpdateBudgetRequest holds the fields that can be patched on an existing budget.
type UpdateBudgetRequest struct {
	Name  *string `json:"name,omitempty"`
	Notes *string `json:"notes,omitempty"`
}

// UpdateCollectionRequest holds the fields that can be patched on an existing
// collection case.
type UpdateCollectionRequest struct {
	Status           *string    `json:"status,omitempty"`
	EscalationPaused *bool      `json:"escalation_paused,omitempty"`
	PauseReason      *string    `json:"pause_reason,omitempty"`
	AssignedTo       *uuid.UUID `json:"assigned_to,omitempty"`
	ClosedReason     *string    `json:"closed_reason,omitempty"`
}

// CreateCollectionActionRequest is the request body for adding an action to a
// collection case.
type CreateCollectionActionRequest struct {
	ActionType   string     `json:"action_type"` // required
	Notes        *string    `json:"notes,omitempty"`
	DocumentID   *uuid.UUID `json:"document_id,omitempty"`
	ScheduledFor *time.Time `json:"scheduled_for,omitempty"`
}

// Validate checks that action_type is set.
func (r CreateCollectionActionRequest) Validate() error {
	if r.ActionType == "" {
		return api.NewValidationError("validation.required", "action_type", api.P("field", "action_type"))
	}
	return nil
}

// CreatePaymentPlanRequest is the request body for creating a payment plan on
// a collection case.
type CreatePaymentPlanRequest struct {
	TotalOwedCents    int64     `json:"total_owed_cents"`    // required
	InstallmentCents  int64     `json:"installment_cents"`   // required
	InstallmentsTotal int       `json:"installments_total"`  // required
	NextDueDate       time.Time `json:"next_due_date"`       // required
	Frequency         string    `json:"frequency,omitempty"`
}

// Validate checks that all required fields are present.
func (r CreatePaymentPlanRequest) Validate() error {
	if r.TotalOwedCents <= 0 {
		return api.NewValidationError("validation.required", "total_owed_cents", api.P("field", "total_owed_cents"))
	}
	if r.InstallmentCents <= 0 {
		return api.NewValidationError("validation.required", "installment_cents", api.P("field", "installment_cents"))
	}
	if r.InstallmentsTotal <= 0 {
		return api.NewValidationError("validation.required", "installments_total", api.P("field", "installments_total"))
	}
	if r.NextDueDate.IsZero() {
		return api.NewValidationError("validation.required", "next_due_date", api.P("field", "next_due_date"))
	}
	return nil
}

// ---------------------------------------------------------------------------
// GL Requests
// ---------------------------------------------------------------------------

// CreateGLAccountRequest is the request body for creating a new GL account in
// the chart of accounts.
type CreateGLAccountRequest struct {
	ParentID      *uuid.UUID `json:"parent_id,omitempty"`
	FundID        *uuid.UUID `json:"fund_id,omitempty"`
	AccountNumber int        `json:"account_number"` // required
	Name          string     `json:"name"`           // required
	AccountType   string     `json:"account_type"`   // required: asset|liability|equity|revenue|expense
	IsHeader      bool       `json:"is_header"`
	Description   *string    `json:"description,omitempty"`
}

// Validate checks that all required fields are present and account_number
// falls within the correct range for the given account_type.
func (r CreateGLAccountRequest) Validate() error {
	if r.Name == "" {
		return api.NewValidationError("validation.required", "name", api.P("field", "name"))
	}
	if r.AccountNumber <= 0 {
		return api.NewValidationError("validation.constraint", "account_number", api.P("field", "account_number"), api.P("constraint", "greater than zero"))
	}
	switch r.AccountType {
	case "asset":
		if r.AccountNumber < 1000 || r.AccountNumber > 1999 {
			return api.NewValidationError("validation.constraint", "account_number", api.P("field", "account_number"), api.P("constraint", "between 1000 and 1999 for asset accounts"))
		}
	case "liability":
		if r.AccountNumber < 2000 || r.AccountNumber > 2999 {
			return api.NewValidationError("validation.constraint", "account_number", api.P("field", "account_number"), api.P("constraint", "between 2000 and 2999 for liability accounts"))
		}
	case "equity":
		if r.AccountNumber < 3000 || r.AccountNumber > 3999 {
			return api.NewValidationError("validation.constraint", "account_number", api.P("field", "account_number"), api.P("constraint", "between 3000 and 3999 for equity accounts"))
		}
	case "revenue":
		if r.AccountNumber < 4000 || r.AccountNumber > 4999 {
			return api.NewValidationError("validation.constraint", "account_number", api.P("field", "account_number"), api.P("constraint", "between 4000 and 4999 for revenue accounts"))
		}
	case "expense":
		if r.AccountNumber < 5000 || r.AccountNumber > 9999 {
			return api.NewValidationError("validation.constraint", "account_number", api.P("field", "account_number"), api.P("constraint", "between 5000 and 9999 for expense accounts"))
		}
	case "":
		return api.NewValidationError("validation.required", "account_type", api.P("field", "account_type"))
	default:
		return api.NewValidationError("validation.one_of", "account_type", api.P("field", "account_type"), api.P("values", "asset, liability, equity, revenue, expense"))
	}
	return nil
}

// UpdateGLAccountRequest holds the fields that can be patched on an existing
// GL account.
type UpdateGLAccountRequest struct {
	Name        *string    `json:"name,omitempty"`
	Description *string    `json:"description,omitempty"`
	FundID      *uuid.UUID `json:"fund_id,omitempty"`
}

// JournalEntryLineInput is a single line within a CreateJournalEntryRequest.
type JournalEntryLineInput struct {
	AccountID   uuid.UUID `json:"account_id"`   // required
	DebitCents  int64     `json:"debit_cents"`
	CreditCents int64     `json:"credit_cents"`
	Memo        *string   `json:"memo,omitempty"`
}

// CreateJournalEntryRequest is the request body for posting a new journal
// entry with balanced debit/credit lines.
type CreateJournalEntryRequest struct {
	EntryDate time.Time              `json:"entry_date"` // required
	Memo      string                 `json:"memo"`       // required
	Lines     []JournalEntryLineInput `json:"lines"`     // required, min 2
}

// Validate checks that all required fields are present, lines are balanced,
// and each line has exactly one of debit_cents or credit_cents set.
func (r CreateJournalEntryRequest) Validate() error {
	if r.Memo == "" {
		return api.NewValidationError("validation.required", "memo", api.P("field", "memo"))
	}
	if r.EntryDate.IsZero() {
		return api.NewValidationError("validation.required", "entry_date", api.P("field", "entry_date"))
	}
	if len(r.Lines) < 2 {
		return api.NewValidationError("validation.constraint", "lines", api.P("field", "lines"), api.P("constraint", "at least 2 entries"))
	}

	var totalDebits, totalCredits int64
	for i, line := range r.Lines {
		if line.AccountID == (uuid.UUID{}) {
			return api.NewValidationError("validation.required", "lines.account_id", api.P("field", "lines.account_id"))
		}
		if line.DebitCents < 0 || line.CreditCents < 0 {
			return api.NewValidationError("validation.constraint", "lines", api.P("field", "lines.debit_cents, lines.credit_cents"), api.P("constraint", "must not be negative"))
		}
		// Exactly one of debit_cents or credit_cents must be > 0.
		hasDebit := line.DebitCents > 0
		hasCredit := line.CreditCents > 0
		if hasDebit == hasCredit {
			_ = i // suppress unused hint
			return api.NewValidationError("validation.constraint", "lines", api.P("field", "lines.debit_cents, lines.credit_cents"), api.P("constraint", "exactly one of debit_cents or credit_cents must be greater than zero"))
		}
		totalDebits += line.DebitCents
		totalCredits += line.CreditCents
	}
	if totalDebits != totalCredits {
		return api.NewValidationError("validation.constraint", "lines", api.P("field", "lines"), api.P("constraint", "total debits must equal total credits"))
	}
	return nil
}
