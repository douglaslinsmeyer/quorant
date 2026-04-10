// Package fin provides domain types, request models, and interfaces for
// the Finance module: assessments, ledger, payments, budgets, expenses,
// funds, and collections.
package fin

import (
	"time"

	"github.com/google/uuid"
)

// AssessmentSchedule defines recurring assessment rules for an HOA.
type AssessmentSchedule struct {
	ID               uuid.UUID      `json:"id"`
	OrgID            uuid.UUID      `json:"org_id"`
	CurrencyCode     string         `json:"currency_code"`
	Name             string         `json:"name"`
	Description      *string        `json:"description,omitempty"`
	Frequency        AssessmentFrequency `json:"frequency"`
	AmountStrategy   AmountStrategy      `json:"amount_strategy"`
	BaseAmountCents  int64          `json:"base_amount_cents"`
	AmountRules      map[string]any `json:"amount_rules"`
	DayOfMonth       *int           `json:"day_of_month,omitempty"`
	GraceDays        *int           `json:"grace_days,omitempty"`
	StartsAt         time.Time      `json:"starts_at"`
	EndsAt           *time.Time     `json:"ends_at,omitempty"`
	IsActive         bool           `json:"is_active"`
	ApprovedBy       *uuid.UUID     `json:"approved_by,omitempty"`
	ApprovedAt       *time.Time     `json:"approved_at,omitempty"`
	CreatedBy        uuid.UUID      `json:"created_by"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	DeletedAt        *time.Time     `json:"deleted_at,omitempty"`
}

// Assessment is a single charge generated for a unit.
type Assessment struct {
	ID           uuid.UUID  `json:"id"`
	OrgID        uuid.UUID  `json:"org_id"`
	CurrencyCode string    `json:"currency_code"`
	UnitID       uuid.UUID  `json:"unit_id"`
	ScheduleID   *uuid.UUID `json:"schedule_id,omitempty"`
	Description  string     `json:"description"`
	AmountCents  int64      `json:"amount_cents"`
	DueDate      time.Time  `json:"due_date"`
	GraceDays    *int       `json:"grace_days,omitempty"`
	LateFeeCents *int64     `json:"late_fee_cents,omitempty"`
	IsRecurring  bool             `json:"is_recurring"`
	Status       AssessmentStatus `json:"status"`
	VoidedBy     *uuid.UUID       `json:"voided_by,omitempty"`
	VoidedAt     *time.Time       `json:"voided_at,omitempty"`
	CreatedBy    *uuid.UUID       `json:"created_by,omitempty"`
	CreatedAt    time.Time        `json:"created_at"`
	UpdatedAt    time.Time        `json:"updated_at"`
	DeletedAt    *time.Time       `json:"deleted_at,omitempty"`
}

// LedgerEntry records a financial event against a unit's balance.
type LedgerEntry struct {
	ID            uuid.UUID  `json:"id"`
	OrgID         uuid.UUID  `json:"org_id"`
	CurrencyCode  string     `json:"currency_code"`
	UnitID        uuid.UUID  `json:"unit_id"`
	AssessmentID  *uuid.UUID `json:"assessment_id,omitempty"`
	EntryType     LedgerEntryType      `json:"entry_type"`
	AmountCents   int64                `json:"amount_cents"`
	BalanceCents  int64                `json:"balance_cents"`
	Description   *string              `json:"description,omitempty"`
	ReferenceType      *LedgerReferenceType `json:"reference_type,omitempty"`
	ReferenceID        *uuid.UUID           `json:"reference_id,omitempty"`
	ReversedByEntryID  *uuid.UUID           `json:"reversed_by_entry_id,omitempty"`
	EffectiveDate      time.Time            `json:"effective_date"`
	CreatedAt          time.Time            `json:"created_at"`
}

// PaymentMethod stores a user's saved payment instrument.
type PaymentMethod struct {
	ID          uuid.UUID  `json:"id"`
	OrgID       uuid.UUID  `json:"org_id"`
	UserID      uuid.UUID  `json:"user_id"`
	MethodType  PaymentMethodType `json:"method_type"`
	ProviderRef *string    `json:"provider_ref,omitempty"`
	LastFour    *string    `json:"last_four,omitempty"`
	IsDefault   bool       `json:"is_default"`
	CreatedAt   time.Time  `json:"created_at"`
	DeletedAt   *time.Time `json:"deleted_at,omitempty"`
}

// Payment records a payment transaction.
type Payment struct {
	ID              uuid.UUID  `json:"id"`
	OrgID           uuid.UUID  `json:"org_id"`
	CurrencyCode    string     `json:"currency_code"`
	UnitID          uuid.UUID  `json:"unit_id"`
	UserID          uuid.UUID  `json:"user_id"`
	PaymentMethodID *uuid.UUID `json:"payment_method_id,omitempty"`
	AmountCents     int64         `json:"amount_cents"`
	Status          PaymentStatus `json:"status"`
	ProviderRef     *string       `json:"provider_ref,omitempty"`
	Description     *string    `json:"description,omitempty"`
	PaidAt          *time.Time `json:"paid_at,omitempty"`
	VoidedBy        *uuid.UUID `json:"voided_by,omitempty"`
	VoidedAt        *time.Time `json:"voided_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// PaymentAllocation records how a specific portion of a payment was applied
// to a specific charge. Links payments to assessments, late fees, etc.
type PaymentAllocation struct {
	ID             uuid.UUID  `json:"id"`
	PaymentID      uuid.UUID  `json:"payment_id"`
	ChargeType     string     `json:"charge_type"`
	ChargeID       uuid.UUID  `json:"charge_id"`
	AllocatedCents int64      `json:"allocated_cents"`
	ResolutionID   uuid.UUID  `json:"resolution_id"`
	EstoppelID     *uuid.UUID `json:"estoppel_id,omitempty"`
	ReversedAt     *time.Time `json:"reversed_at,omitempty"`
	ReversedByID   *uuid.UUID `json:"reversed_by_id,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

// BudgetCategory organizes budget line items.
type BudgetCategory struct {
	ID           uuid.UUID  `json:"id"`
	OrgID        uuid.UUID  `json:"org_id"`
	Name         string             `json:"name"`
	CategoryType BudgetCategoryType `json:"category_type"`
	ParentID     *uuid.UUID `json:"parent_id,omitempty"`
	SortOrder    int        `json:"sort_order"`
	IsReserve    bool       `json:"is_reserve"`
	CreatedAt    time.Time  `json:"created_at"`
}

// Budget represents an annual financial plan for an HOA.
type Budget struct {
	ID                 uuid.UUID  `json:"id"`
	OrgID              uuid.UUID  `json:"org_id"`
	FiscalYear         int          `json:"fiscal_year"`
	Name               string       `json:"name"`
	Status             BudgetStatus `json:"status"`
	TotalIncomeCents   int64      `json:"total_income_cents"`
	TotalExpenseCents  int64      `json:"total_expense_cents"`
	NetCents           int64      `json:"net_cents"`
	Notes              *string    `json:"notes,omitempty"`
	ProposedAt         *time.Time `json:"proposed_at,omitempty"`
	ProposedBy         *uuid.UUID `json:"proposed_by,omitempty"`
	ApprovedAt         *time.Time `json:"approved_at,omitempty"`
	ApprovedBy         *uuid.UUID `json:"approved_by,omitempty"`
	DocumentID         *uuid.UUID `json:"document_id,omitempty"`
	CreatedBy          uuid.UUID  `json:"created_by"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
	DeletedAt          *time.Time `json:"deleted_at,omitempty"`
}

// BudgetLineItem is a single line within a budget.
type BudgetLineItem struct {
	ID           uuid.UUID  `json:"id"`
	BudgetID     uuid.UUID  `json:"budget_id"`
	CategoryID   uuid.UUID  `json:"category_id"`
	Description  *string    `json:"description,omitempty"`
	PlannedCents int64      `json:"planned_cents"`
	ActualCents  int64      `json:"actual_cents"`
	Notes        *string    `json:"notes,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// Expense records a vendor or operational expense.
type Expense struct {
	ID            uuid.UUID      `json:"id"`
	OrgID         uuid.UUID      `json:"org_id"`
	CurrencyCode  string         `json:"currency_code"`
	VendorID      *uuid.UUID     `json:"vendor_id,omitempty"`
	CategoryID    *uuid.UUID     `json:"category_id,omitempty"`
	BudgetID      *uuid.UUID     `json:"budget_id,omitempty"`
	FundType      *FundType      `json:"fund_type,omitempty"`
	Description   string         `json:"description"`
	AmountCents   int64          `json:"amount_cents"`
	TaxCents      int64          `json:"tax_cents"`
	TotalCents    int64          `json:"total_cents"`
	Status        ExpenseStatus  `json:"status"`
	ExpenseDate   time.Time      `json:"expense_date"`
	DueDate       *time.Time     `json:"due_date,omitempty"`
	PaidDate      *time.Time     `json:"paid_date,omitempty"`
	PaymentMethod *string        `json:"payment_method,omitempty"`
	PaymentRef    *string        `json:"payment_ref,omitempty"`
	InvoiceNumber *string        `json:"invoice_number,omitempty"`
	ReceiptDocID  *uuid.UUID     `json:"receipt_doc_id,omitempty"`
	SubmittedBy   uuid.UUID      `json:"submitted_by"`
	ApprovedBy    *uuid.UUID     `json:"approved_by,omitempty"`
	ApprovedAt    *time.Time     `json:"approved_at,omitempty"`
	ApprovalNotes *string        `json:"approval_notes,omitempty"`
	Metadata      map[string]any `json:"metadata"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     *time.Time     `json:"deleted_at,omitempty"`
}

// Fund represents a financial pool (operating, reserve, capital, special).
type Fund struct {
	ID                  uuid.UUID  `json:"id"`
	OrgID               uuid.UUID  `json:"org_id"`
	CurrencyCode        string     `json:"currency_code"`
	Name                string   `json:"name"`
	FundType            FundType `json:"fund_type"`
	BalanceCents        int64      `json:"balance_cents"`
	TargetBalanceCents  *int64     `json:"target_balance_cents,omitempty"`
	IsDefault           bool       `json:"is_default"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
	DeletedAt           *time.Time `json:"deleted_at,omitempty"`
}

// FundTransaction records a single debit or credit to a fund.
type FundTransaction struct {
	ID               uuid.UUID  `json:"id"`
	FundID           uuid.UUID  `json:"fund_id"`
	OrgID            uuid.UUID  `json:"org_id"`
	CurrencyCode     string     `json:"currency_code"`
	TransactionType  string     `json:"transaction_type"`
	AmountCents      int64      `json:"amount_cents"`
	BalanceAfterCents int64     `json:"balance_after_cents"`
	Description      *string    `json:"description,omitempty"`
	ReferenceType    *string    `json:"reference_type,omitempty"`
	ReferenceID      *uuid.UUID `json:"reference_id,omitempty"`
	EffectiveDate    time.Time  `json:"effective_date"`
	CreatedAt        time.Time  `json:"created_at"`
}

// FundTransfer moves money between two funds.
type FundTransfer struct {
	ID           uuid.UUID  `json:"id"`
	OrgID        uuid.UUID  `json:"org_id"`
	CurrencyCode string     `json:"currency_code"`
	FromFundID   uuid.UUID  `json:"from_fund_id"`
	ToFundID     uuid.UUID  `json:"to_fund_id"`
	AmountCents  int64      `json:"amount_cents"`
	Description  *string    `json:"description,omitempty"`
	ApprovedBy   *uuid.UUID `json:"approved_by,omitempty"`
	ApprovedAt   *time.Time `json:"approved_at,omitempty"`
	EffectiveDate time.Time `json:"effective_date"`
	CreatedAt    time.Time  `json:"created_at"`
}

// BudgetReport combines a Budget with its line items for reporting.
type BudgetReport struct {
	Budget    *Budget          `json:"budget"`
	LineItems []BudgetLineItem `json:"line_items"`
}

// CollectionCase tracks outstanding debt collection for a unit.
type CollectionCase struct {
	ID                uuid.UUID      `json:"id"`
	OrgID             uuid.UUID      `json:"org_id"`
	UnitID            uuid.UUID            `json:"unit_id"`
	Status            CollectionCaseStatus `json:"status"`
	TotalOwedCents    int64          `json:"total_owed_cents"`
	CurrentOwedCents  int64          `json:"current_owed_cents"`
	EscalationPaused  bool           `json:"escalation_paused"`
	PauseReason       *string        `json:"pause_reason,omitempty"`
	OpenedAt          time.Time      `json:"opened_at"`
	ClosedAt          *time.Time     `json:"closed_at,omitempty"`
	ClosedReason      *string        `json:"closed_reason,omitempty"`
	AssignedTo        *uuid.UUID     `json:"assigned_to,omitempty"`
	Metadata          map[string]any `json:"metadata"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
}

// CollectionAction records a step taken within a collection case.
type CollectionAction struct {
	ID            uuid.UUID      `json:"id"`
	CaseID        uuid.UUID            `json:"case_id"`
	ActionType    CollectionActionType `json:"action_type"`
	Notes         *string              `json:"notes,omitempty"`
	DocumentID    *uuid.UUID           `json:"document_id,omitempty"`
	TriggeredBy   *TriggeredBy         `json:"triggered_by,omitempty"`
	PerformedBy   *uuid.UUID     `json:"performed_by,omitempty"`
	ScheduledFor  *time.Time     `json:"scheduled_for,omitempty"`
	CompletedAt   *time.Time     `json:"completed_at,omitempty"`
	Metadata      map[string]any `json:"metadata"`
	CreatedAt     time.Time      `json:"created_at"`
}

// PaymentPlan is a structured repayment agreement for a collection case.
type PaymentPlan struct {
	ID                 uuid.UUID  `json:"id"`
	CaseID             uuid.UUID  `json:"case_id"`
	OrgID              uuid.UUID  `json:"org_id"`
	UnitID             uuid.UUID  `json:"unit_id"`
	TotalOwedCents     int64      `json:"total_owed_cents"`
	InstallmentCents   int64      `json:"installment_cents"`
	Frequency          PaymentPlanFrequency `json:"frequency"`
	InstallmentsTotal  int                  `json:"installments_total"`
	InstallmentsPaid   int                  `json:"installments_paid"`
	NextDueDate        time.Time            `json:"next_due_date"`
	Status             PaymentPlanStatus    `json:"status"`
	ApprovedBy         *uuid.UUID `json:"approved_by,omitempty"`
	ApprovedAt         *time.Time `json:"approved_at,omitempty"`
	DefaultedAt        *time.Time `json:"defaulted_at,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

// GLAccount represents a single account in the chart of accounts.
type GLAccount struct {
	ID            uuid.UUID  `json:"id"`
	OrgID         uuid.UUID  `json:"org_id"`
	ParentID      *uuid.UUID `json:"parent_id,omitempty"`
	FundID        *uuid.UUID `json:"fund_id,omitempty"`
	AccountNumber int           `json:"account_number"`
	Name          string        `json:"name"`
	AccountType   GLAccountType `json:"account_type"`
	IsHeader      bool       `json:"is_header"`
	IsSystem      bool       `json:"is_system"`
	Description   *string    `json:"description,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	DeletedAt     *time.Time `json:"deleted_at,omitempty"`
}

// GLJournalEntry represents a double-entry journal entry header.
type GLJournalEntry struct {
	ID          uuid.UUID       `json:"id"`
	OrgID       uuid.UUID       `json:"org_id"`
	EntryNumber int             `json:"entry_number"`
	EntryDate   time.Time       `json:"entry_date"`
	Memo        string        `json:"memo"`
	SourceType  *GLSourceType `json:"source_type,omitempty"`
	SourceID    *uuid.UUID      `json:"source_id,omitempty"`
	UnitID      *uuid.UUID      `json:"unit_id,omitempty"`
	PostedBy    uuid.UUID       `json:"posted_by"`
	ReversedBy  *uuid.UUID      `json:"reversed_by,omitempty"`
	IsReversal  bool            `json:"is_reversal"`
	CreatedAt   time.Time       `json:"created_at"`
	Lines       []GLJournalLine `json:"lines,omitempty"`
}

// GLJournalLine is a single debit or credit line within a journal entry.
type GLJournalLine struct {
	ID             uuid.UUID `json:"id"`
	JournalEntryID uuid.UUID `json:"journal_entry_id"`
	AccountID      uuid.UUID `json:"account_id"`
	DebitCents     int64     `json:"debit_cents"`
	CreditCents    int64     `json:"credit_cents"`
	Memo           *string   `json:"memo,omitempty"`
}

// TrialBalanceRow is a single row in a trial balance report.
type TrialBalanceRow struct {
	AccountID     uuid.UUID     `json:"account_id"`
	AccountNumber int           `json:"account_number"`
	AccountName   string        `json:"account_name"`
	AccountType   GLAccountType `json:"account_type"`
	DebitCents    int64         `json:"debit_cents"`
	CreditCents   int64         `json:"credit_cents"`
}

// AccountBalance holds the net balance for a single GL account.
type AccountBalance struct {
	AccountID     uuid.UUID     `json:"account_id"`
	AccountNumber int           `json:"account_number"`
	AccountName   string        `json:"account_name"`
	AccountType   GLAccountType `json:"account_type"`
	BalanceCents  int64         `json:"balance_cents"`
}
