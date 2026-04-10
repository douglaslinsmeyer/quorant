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
		return api.NewValidationError("name is required", "name")
	}
	switch r.Frequency {
	case "monthly", "quarterly", "annually", "semi_annually":
		// valid
	case "":
		return api.NewValidationError("frequency is required", "frequency")
	default:
		return api.NewValidationError("frequency must be one of: monthly, quarterly, annually, semi_annually", "frequency")
	}
	switch r.AmountStrategy {
	case "flat", "per_unit_type", "per_sqft", "custom":
		// valid
	case "":
		return api.NewValidationError("amount_strategy is required", "amount_strategy")
	default:
		return api.NewValidationError("amount_strategy must be one of: flat, per_unit_type, per_sqft, custom", "amount_strategy")
	}
	if r.BaseAmountCents <= 0 {
		return api.NewValidationError("base_amount_cents is required", "base_amount_cents")
	}
	if r.StartsAt.IsZero() {
		return api.NewValidationError("starts_at is required", "starts_at")
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
		return api.NewValidationError("unit_id is required", "unit_id")
	}
	if r.Description == "" {
		return api.NewValidationError("description is required", "description")
	}
	if r.AmountCents <= 0 {
		return api.NewValidationError("amount_cents is required", "amount_cents")
	}
	if r.DueDate.IsZero() {
		return api.NewValidationError("due_date is required", "due_date")
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
		return api.NewValidationError("unit_id is required", "unit_id")
	}
	if r.AmountCents <= 0 {
		return api.NewValidationError("amount_cents must be positive", "amount_cents")
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
		return api.NewValidationError("fiscal_year is required", "fiscal_year")
	}
	if r.Name == "" {
		return api.NewValidationError("name is required", "name")
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
		return api.NewValidationError("description is required", "description")
	}
	if r.AmountCents <= 0 {
		return api.NewValidationError("amount_cents is required", "amount_cents")
	}
	if r.ExpenseDate.IsZero() {
		return api.NewValidationError("expense_date is required", "expense_date")
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
		return api.NewValidationError("name is required", "name")
	}
	switch r.FundType {
	case "operating", "reserve", "capital", "special":
		// valid
	case "":
		return api.NewValidationError("fund_type is required", "fund_type")
	default:
		return api.NewValidationError("fund_type must be one of: operating, reserve, capital, special", "fund_type")
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
		return api.NewValidationError("from_fund_id is required", "from_fund_id")
	}
	if r.ToFundID == (uuid.UUID{}) {
		return api.NewValidationError("to_fund_id is required", "to_fund_id")
	}
	if r.AmountCents <= 0 {
		return api.NewValidationError("amount_cents must be positive", "amount_cents")
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
		return api.NewValidationError("action_type is required", "action_type")
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
		return api.NewValidationError("total_owed_cents is required", "total_owed_cents")
	}
	if r.InstallmentCents <= 0 {
		return api.NewValidationError("installment_cents is required", "installment_cents")
	}
	if r.InstallmentsTotal <= 0 {
		return api.NewValidationError("installments_total is required", "installments_total")
	}
	if r.NextDueDate.IsZero() {
		return api.NewValidationError("next_due_date is required", "next_due_date")
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
		return api.NewValidationError("name is required", "name")
	}
	if r.AccountNumber <= 0 {
		return api.NewValidationError("account_number must be greater than zero", "account_number")
	}
	switch r.AccountType {
	case "asset":
		if r.AccountNumber < 1000 || r.AccountNumber > 1999 {
			return api.NewValidationError("asset account_number must be between 1000 and 1999", "account_number")
		}
	case "liability":
		if r.AccountNumber < 2000 || r.AccountNumber > 2999 {
			return api.NewValidationError("liability account_number must be between 2000 and 2999", "account_number")
		}
	case "equity":
		if r.AccountNumber < 3000 || r.AccountNumber > 3999 {
			return api.NewValidationError("equity account_number must be between 3000 and 3999", "account_number")
		}
	case "revenue":
		if r.AccountNumber < 4000 || r.AccountNumber > 4999 {
			return api.NewValidationError("revenue account_number must be between 4000 and 4999", "account_number")
		}
	case "expense":
		if r.AccountNumber < 5000 || r.AccountNumber > 9999 {
			return api.NewValidationError("expense account_number must be between 5000 and 9999", "account_number")
		}
	case "":
		return api.NewValidationError("account_type is required", "account_type")
	default:
		return api.NewValidationError("account_type must be one of: asset, liability, equity, revenue, expense", "account_type")
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
		return api.NewValidationError("memo is required", "memo")
	}
	if r.EntryDate.IsZero() {
		return api.NewValidationError("entry_date is required", "entry_date")
	}
	if len(r.Lines) < 2 {
		return api.NewValidationError("at least 2 journal lines are required", "lines")
	}

	var totalDebits, totalCredits int64
	for i, line := range r.Lines {
		if line.AccountID == (uuid.UUID{}) {
			return api.NewValidationError("account_id is required on every line", "lines")
		}
		if line.DebitCents < 0 || line.CreditCents < 0 {
			return api.NewValidationError("debit_cents and credit_cents must not be negative", "lines")
		}
		// Exactly one of debit_cents or credit_cents must be > 0.
		hasDebit := line.DebitCents > 0
		hasCredit := line.CreditCents > 0
		if hasDebit == hasCredit {
			_ = i // suppress unused hint
			return api.NewValidationError("each line must have exactly one of debit_cents or credit_cents greater than zero", "lines")
		}
		totalDebits += line.DebitCents
		totalCredits += line.CreditCents
	}
	if totalDebits != totalCredits {
		return api.NewValidationError("total debits must equal total credits", "lines")
	}
	return nil
}
