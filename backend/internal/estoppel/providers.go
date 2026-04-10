package estoppel

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Data provider interfaces
// ---------------------------------------------------------------------------

// FinancialDataProvider retrieves financial snapshot data for a unit from the
// fin module.
type FinancialDataProvider interface {
	GetUnitFinancialSnapshot(ctx context.Context, orgID, unitID uuid.UUID) (*FinancialSnapshot, error)
}

// ComplianceDataProvider retrieves compliance snapshot data for a unit from
// the gov module.
type ComplianceDataProvider interface {
	GetUnitComplianceSnapshot(ctx context.Context, orgID, unitID uuid.UUID) (*ComplianceSnapshot, error)
}

// PropertyDataProvider retrieves property snapshot data for a unit from the
// org/com module.
type PropertyDataProvider interface {
	GetPropertySnapshot(ctx context.Context, orgID, unitID uuid.UUID) (*PropertySnapshot, error)
}

// ---------------------------------------------------------------------------
// Aggregate
// ---------------------------------------------------------------------------

// AggregatedData holds the full set of data gathered from all provider
// modules for a single estoppel certificate generation run.
type AggregatedData struct {
	Property   PropertySnapshot   `json:"property"`
	Financial  FinancialSnapshot  `json:"financial"`
	Compliance ComplianceSnapshot `json:"compliance"`
	Narratives *NarrativeSections `json:"narratives,omitempty"`
	AsOfTime   time.Time          `json:"as_of_time"`
}

// ---------------------------------------------------------------------------
// FinancialSnapshot and supporting types
// ---------------------------------------------------------------------------

// SpecialAssessment describes a one-time or limited-term assessment levied
// against a unit in addition to regular dues.
type SpecialAssessment struct {
	Description      string    `json:"description"`
	TotalAmountCents int64     `json:"total_amount_cents"`
	PerInstallmentCents int64  `json:"per_installment_cents"`
	Frequency        string    `json:"frequency"`
	StartDate        time.Time `json:"start_date"`
	EndDate          *time.Time `json:"end_date,omitempty"`
	BalanceRemainingCents int64 `json:"balance_remaining_cents"`
	Paid             bool      `json:"paid"`
}

// PastDueItem describes a single overdue charge on a unit's account.
type PastDueItem struct {
	Description    string    `json:"description"`
	AmountCents    int64     `json:"amount_cents"`
	DueDate        time.Time `json:"due_date"`
	DaysOverdue    int       `json:"days_overdue"`
	ChargeType     string    `json:"charge_type"` // assessment|late_fee|interest|other
}

// PaymentPlanInfo holds details of an active payment plan for a delinquent
// unit.
type PaymentPlanInfo struct {
	TotalOwedCents      int64     `json:"total_owed_cents"`
	InstallmentCents    int64     `json:"installment_cents"`
	Frequency           string    `json:"frequency"`
	InstallmentsTotal   int       `json:"installments_total"`
	InstallmentsPaid    int       `json:"installments_paid"`
	NextDueDate         time.Time `json:"next_due_date"`
	Status              string    `json:"status"`
}

// FinancialSnapshot captures the full financial position of a unit at a point
// in time, as needed for estoppel certificate generation.
type FinancialSnapshot struct {
	// Current assessment charges
	RegularAssessmentCents      int64               `json:"regular_assessment_cents"`
	AssessmentFrequency         string              `json:"assessment_frequency"`
	PaidThroughDate             time.Time           `json:"paid_through_date"`
	NextInstallmentDueDate      time.Time           `json:"next_installment_due_date"`
	SpecialAssessments          []SpecialAssessment `json:"special_assessments"`

	// Transfer and contribution fees
	CapitalContributionCents    int64               `json:"capital_contribution_cents"`
	TransferFeeCents            int64               `json:"transfer_fee_cents"`
	ReserveFundFeeCents         int64               `json:"reserve_fund_fee_cents"`

	// Current balance and past-due items
	CurrentBalanceCents         int64               `json:"current_balance_cents"`
	PastDueItems                []PastDueItem       `json:"past_due_items"`

	// Delinquency breakdown
	LateFeesCents               int64               `json:"late_fees_cents"`
	InterestCents               int64               `json:"interest_cents"`
	CollectionCostsCents        int64               `json:"collection_costs_cents"`
	TotalDelinquentCents        int64               `json:"total_delinquent_cents"`

	// Collection status
	HasActiveCollection         bool                `json:"has_active_collection"`
	CollectionStatus            string              `json:"collection_status"`
	AttorneyName                string              `json:"attorney_name"`
	AttorneyContact             string              `json:"attorney_contact"`
	LienStatus                  string              `json:"lien_status"`

	// Payment plan
	HasPaymentPlan              bool                `json:"has_payment_plan"`
	PaymentPlanDetails          *PaymentPlanInfo    `json:"payment_plan_details,omitempty"`

	// Association-level financials
	DelinquencyRate60Days       float64             `json:"delinquency_rate_60_days"`
	ReserveBalanceCents         int64               `json:"reserve_balance_cents"`
	ReserveTargetCents          int64               `json:"reserve_target_cents"`
	BudgetStatus                string              `json:"budget_status"`
	TotalUnits                  int                 `json:"total_units"`
	OwnerOccupiedCount          int                 `json:"owner_occupied_count"`
	RentalCount                 int                 `json:"rental_count"`
}

// ---------------------------------------------------------------------------
// ComplianceSnapshot and supporting types
// ---------------------------------------------------------------------------

// ViolationSummary describes open or recent compliance violations for a unit.
type ViolationSummary struct {
	OpenCount        int      `json:"open_count"`
	ResolvedCount    int      `json:"resolved_count"`
	Categories       []string `json:"categories"`
	HasHearingPending bool    `json:"has_hearing_pending"`
	HasActiveFine    bool     `json:"has_active_fine"`
	TotalFinesCents  int64    `json:"total_fines_cents"`
}

// LitigationSummary describes any pending or active litigation involving the
// unit or association.
type LitigationSummary struct {
	HasActiveLitigation  bool   `json:"has_active_litigation"`
	CaseDescription      string `json:"case_description"`
	InvolvesUnit         bool   `json:"involves_unit"`
	InvolvesAssociation  bool   `json:"involves_association"`
}

// InsuranceSummary describes the insurance coverage status for the
// association.
type InsuranceSummary struct {
	HasMasterPolicy      bool      `json:"has_master_policy"`
	PolicyNumber         string    `json:"policy_number"`
	Carrier              string    `json:"carrier"`
	ExpiresAt            *time.Time `json:"expires_at,omitempty"`
	CoverageAmountCents  int64     `json:"coverage_amount_cents"`
	FloodCoverage        bool      `json:"flood_coverage"`
	EarthquakeCoverage   bool      `json:"earthquake_coverage"`
}

// ComplianceSnapshot captures the compliance and governance state of a unit
// and the broader association at a point in time.
type ComplianceSnapshot struct {
	Violations  ViolationSummary  `json:"violations"`
	Litigation  LitigationSummary `json:"litigation"`
	Insurance   InsuranceSummary  `json:"insurance"`
	// Governing document versions in effect
	CCRVersion  string `json:"ccr_version"`
	ByLawVersion string `json:"by_law_version"`
	RulesVersion string `json:"rules_version"`
	// Pending rule changes that affect the unit
	PendingAmendments []string `json:"pending_amendments"`
}

// ---------------------------------------------------------------------------
// PropertySnapshot and supporting types
// ---------------------------------------------------------------------------

// OwnerInfo describes the current owner(s) of record for a unit.
type OwnerInfo struct {
	Name          string `json:"name"`
	Email         string `json:"email"`
	Phone         string `json:"phone"`
	MailingAddress string `json:"mailing_address"`
	IsOccupant    bool   `json:"is_occupant"`
}

// PropertySnapshot captures physical and ownership details for a unit.
type PropertySnapshot struct {
	UnitNumber      string      `json:"unit_number"`
	Address         string      `json:"address"`
	LegalDescription string     `json:"legal_description"`
	ParcelNumber    string      `json:"parcel_number"`
	SquareFootage   float64     `json:"square_footage"`
	UnitType        string      `json:"unit_type"`
	Bedrooms        int         `json:"bedrooms"`
	Bathrooms       float64     `json:"bathrooms"`
	ParkingSpaces   []string    `json:"parking_spaces"`
	StorageUnits    []string    `json:"storage_units"`
	Owners          []OwnerInfo `json:"owners"`
	IsRental        bool        `json:"is_rental"`
	PetRestrictions string      `json:"pet_restrictions"`
	LeaseOnFile     bool        `json:"lease_on_file"`
	// OrgState is the two-letter US state code for the HOA's jurisdiction,
	// derived from the Organization.State field. Used to populate Jurisdiction
	// on the generated EstoppelCertificate.
	OrgState        string      `json:"org_state,omitempty"`
}

// ---------------------------------------------------------------------------
// Narrative types
// ---------------------------------------------------------------------------

// Citation describes a reference to a governing document or statute used in
// a narrative section.
type Citation struct {
	Document string `json:"document"`
	Section  string `json:"section"`
	Text     string `json:"text"`
}

// NarrativeField is a single human-readable field within a narrative section,
// optionally backed by citations.
type NarrativeField struct {
	Label     string     `json:"label"`
	Value     string     `json:"value"`
	Citations []Citation `json:"citations,omitempty"`
}

// NarrativeSections holds the AI-generated or template-driven narrative
// content for each section of an estoppel certificate.
type NarrativeSections struct {
	AssessmentSummary    []NarrativeField `json:"assessment_summary"`
	DelinquencySummary   []NarrativeField `json:"delinquency_summary"`
	ComplianceSummary    []NarrativeField `json:"compliance_summary"`
	InsuranceSummary     []NarrativeField `json:"insurance_summary"`
	LitigationSummary    []NarrativeField `json:"litigation_summary"`
	TransferFees         []NarrativeField `json:"transfer_fees"`
	AdditionalDisclosures []NarrativeField `json:"additional_disclosures"`
}
