package fin

// AssessmentStatus represents the lifecycle state of an assessment.
type AssessmentStatus string

const (
	AssessmentStatusPosted AssessmentStatus = "posted"
	AssessmentStatusVoid   AssessmentStatus = "void"
)

// IsValid returns true if the AssessmentStatus value is one of the defined constants.
func (s AssessmentStatus) IsValid() bool {
	switch s {
	case AssessmentStatusPosted, AssessmentStatusVoid:
		return true
	}
	return false
}

// BudgetStatus represents the lifecycle state of a budget.
type BudgetStatus string

const (
	BudgetStatusDraft    BudgetStatus = "draft"
	BudgetStatusProposed BudgetStatus = "proposed"
	BudgetStatusApproved BudgetStatus = "approved"
)

// IsValid returns true if the BudgetStatus value is one of the defined constants.
func (s BudgetStatus) IsValid() bool {
	switch s {
	case BudgetStatusDraft, BudgetStatusProposed, BudgetStatusApproved:
		return true
	}
	return false
}

// ExpenseStatus represents the lifecycle state of an expense.
type ExpenseStatus string

const (
	ExpenseStatusSubmitted ExpenseStatus = "submitted"
	ExpenseStatusApproved  ExpenseStatus = "approved"
	ExpenseStatusPaid      ExpenseStatus = "paid"
)

// IsValid returns true if the ExpenseStatus value is one of the defined constants.
func (s ExpenseStatus) IsValid() bool {
	switch s {
	case ExpenseStatusSubmitted, ExpenseStatusApproved, ExpenseStatusPaid:
		return true
	}
	return false
}

// PaymentStatus represents the lifecycle state of a payment.
type PaymentStatus string

const (
	PaymentStatusPending   PaymentStatus = "pending"
	PaymentStatusCompleted PaymentStatus = "completed"
	PaymentStatusFailed    PaymentStatus = "failed"
	PaymentStatusVoid      PaymentStatus = "void"
)

const (
	PaymentStatusPendingReview PaymentStatus = "pending_review"
	PaymentStatusReversed      PaymentStatus = "reversed"
	PaymentStatusNSF           PaymentStatus = "nsf"
)

// IsValid returns true if the PaymentStatus value is one of the defined constants.
func (s PaymentStatus) IsValid() bool {
	switch s {
	case PaymentStatusPending, PaymentStatusCompleted, PaymentStatusFailed,
		PaymentStatusPendingReview, PaymentStatusReversed, PaymentStatusNSF, PaymentStatusVoid:
		return true
	}
	return false
}

// CollectionCaseStatus represents the lifecycle state of a collection case.
type CollectionCaseStatus string

const (
	CollectionCaseStatusLate     CollectionCaseStatus = "late"
	CollectionCaseStatusClosed   CollectionCaseStatus = "closed"
	CollectionCaseStatusResolved CollectionCaseStatus = "resolved"
)

// IsValid returns true if the CollectionCaseStatus value is one of the defined constants.
func (s CollectionCaseStatus) IsValid() bool {
	switch s {
	case CollectionCaseStatusLate, CollectionCaseStatusClosed, CollectionCaseStatusResolved:
		return true
	}
	return false
}

// PaymentPlanStatus represents the lifecycle state of a payment plan.
type PaymentPlanStatus string

const (
	PaymentPlanStatusActive    PaymentPlanStatus = "active"
	PaymentPlanStatusDefaulted PaymentPlanStatus = "defaulted"
)

// IsValid returns true if the PaymentPlanStatus value is one of the defined constants.
func (s PaymentPlanStatus) IsValid() bool {
	switch s {
	case PaymentPlanStatusActive, PaymentPlanStatusDefaulted:
		return true
	}
	return false
}

// FundType classifies the purpose of a financial fund.
type FundType string

const (
	FundTypeOperating FundType = "operating"
	FundTypeReserve   FundType = "reserve"
	FundTypeCapital   FundType = "capital"
	FundTypeSpecial   FundType = "special"
)

// IsValid returns true if the FundType value is one of the defined constants.
func (s FundType) IsValid() bool {
	switch s {
	case FundTypeOperating, FundTypeReserve, FundTypeCapital, FundTypeSpecial:
		return true
	}
	return false
}

// FundTransactionType constants classify the type of fund transaction.
const (
	FundTxTypeTransferOut = "transfer_out"
	FundTxTypeTransferIn  = "transfer_in"
)

// FundTransactionRefType constants identify the source entity for a fund transaction.
const (
	FundTxRefTypeTransfer = "fund_transfer"
)

// LedgerEntryType classifies a ledger entry's financial effect.
type LedgerEntryType string

const (
	LedgerEntryTypeCharge     LedgerEntryType = "charge"
	LedgerEntryTypePayment    LedgerEntryType = "payment"
	LedgerEntryTypeCredit     LedgerEntryType = "credit"
	LedgerEntryTypeAdjustment LedgerEntryType = "adjustment"
	LedgerEntryTypeLateFee    LedgerEntryType = "late_fee"
	LedgerEntryTypeReversal   LedgerEntryType = "reversal"
)

// IsValid returns true if the LedgerEntryType value is one of the defined constants.
func (s LedgerEntryType) IsValid() bool {
	switch s {
	case LedgerEntryTypeCharge, LedgerEntryTypePayment, LedgerEntryTypeCredit,
		LedgerEntryTypeAdjustment, LedgerEntryTypeLateFee, LedgerEntryTypeReversal:
		return true
	}
	return false
}

// GLSourceType identifies the originating transaction type for a GL journal entry.
type GLSourceType string

const (
	GLSourceTypeAssessment GLSourceType = "assessment"
	GLSourceTypePayment    GLSourceType = "payment"
	GLSourceTypeTransfer   GLSourceType = "transfer"
	GLSourceTypeExpense    GLSourceType = "expense"
	GLSourceTypeManual     GLSourceType = "manual"
)

// IsValid returns true if the GLSourceType value is one of the defined constants.
func (s GLSourceType) IsValid() bool {
	switch s {
	case GLSourceTypeAssessment, GLSourceTypePayment, GLSourceTypeTransfer, GLSourceTypeExpense, GLSourceTypeManual:
		return true
	}
	return false
}

// LedgerReferenceType identifies the type of entity referenced by a ledger entry.
type LedgerReferenceType string

const (
	LedgerRefTypePayment   LedgerReferenceType = "payment"
	LedgerRefTypeReversal  LedgerReferenceType = "reversal"
)

// IsValid returns true if the LedgerReferenceType value is one of the defined constants.
func (s LedgerReferenceType) IsValid() bool {
	switch s {
	case LedgerRefTypePayment, LedgerRefTypeReversal:
		return true
	}
	return false
}

// PaymentMethodType identifies the kind of payment instrument.
type PaymentMethodType string

const (
	PaymentMethodTypeCard PaymentMethodType = "card"
	PaymentMethodTypeACH  PaymentMethodType = "ach"
)

// IsValid returns true if the PaymentMethodType value is one of the defined constants.
func (s PaymentMethodType) IsValid() bool {
	switch s {
	case PaymentMethodTypeCard, PaymentMethodTypeACH:
		return true
	}
	return false
}

// CollectionActionType classifies the type of action taken on a collection case.
type CollectionActionType string

const (
	CollectionActionTypeNoticeSent CollectionActionType = "notice_sent"
	CollectionActionTypeLienFiled  CollectionActionType = "lien_filed"
)

// IsValid returns true if the CollectionActionType value is one of the defined constants.
func (s CollectionActionType) IsValid() bool {
	switch s {
	case CollectionActionTypeNoticeSent, CollectionActionTypeLienFiled:
		return true
	}
	return false
}

// GLAccountType classifies an account within the chart of accounts.
type GLAccountType string

const (
	GLAccountTypeAsset     GLAccountType = "asset"
	GLAccountTypeLiability GLAccountType = "liability"
	GLAccountTypeEquity    GLAccountType = "equity"
	GLAccountTypeRevenue   GLAccountType = "revenue"
	GLAccountTypeExpense   GLAccountType = "expense"
)

// IsValid returns true if the GLAccountType value is one of the defined constants.
func (s GLAccountType) IsValid() bool {
	switch s {
	case GLAccountTypeAsset, GLAccountTypeLiability, GLAccountTypeEquity,
		GLAccountTypeRevenue, GLAccountTypeExpense:
		return true
	}
	return false
}

// AssessmentFrequency defines how often an assessment schedule recurs.
type AssessmentFrequency string

const (
	AssessmentFreqMonthly      AssessmentFrequency = "monthly"
	AssessmentFreqQuarterly    AssessmentFrequency = "quarterly"
	AssessmentFreqSemiAnnually AssessmentFrequency = "semi_annually"
	AssessmentFreqAnnually     AssessmentFrequency = "annually"
)

// IsValid returns true if the AssessmentFrequency value is one of the defined constants.
func (s AssessmentFrequency) IsValid() bool {
	switch s {
	case AssessmentFreqMonthly, AssessmentFreqQuarterly, AssessmentFreqSemiAnnually, AssessmentFreqAnnually:
		return true
	}
	return false
}

// PaymentPlanFrequency defines how often a payment plan installment recurs.
type PaymentPlanFrequency string

const (
	PaymentPlanFreqMonthly PaymentPlanFrequency = "monthly"
)

// IsValid returns true if the PaymentPlanFrequency value is one of the defined constants.
func (s PaymentPlanFrequency) IsValid() bool {
	switch s {
	case PaymentPlanFreqMonthly:
		return true
	}
	return false
}

// AmountStrategy determines how assessment amounts are calculated.
type AmountStrategy string

const (
	AmountStrategyFlat        AmountStrategy = "flat"
	AmountStrategyPerUnitType AmountStrategy = "per_unit_type"
	AmountStrategyPerSqft     AmountStrategy = "per_sqft"
	AmountStrategyCustom      AmountStrategy = "custom"
)

// IsValid returns true if the AmountStrategy value is one of the defined constants.
func (s AmountStrategy) IsValid() bool {
	switch s {
	case AmountStrategyFlat, AmountStrategyPerUnitType, AmountStrategyPerSqft, AmountStrategyCustom:
		return true
	}
	return false
}

// CustodianType identifies who holds custody of a fund's assets.
type CustodianType string

const (
	CustodianAssociationHeld       CustodianType = "association_held"
	CustodianManagementCompanyHeld CustodianType = "management_company_held"
)

// IsValid returns true if the CustodianType value is one of the defined constants.
func (s CustodianType) IsValid() bool {
	switch s {
	case CustodianAssociationHeld, CustodianManagementCompanyHeld:
		return true
	}
	return false
}

// BudgetCategoryType classifies a budget category.
type BudgetCategoryType string

const (
	BudgetCategoryTypeExpense BudgetCategoryType = "expense"
	BudgetCategoryTypeIncome  BudgetCategoryType = "income"
)

// IsValid returns true if the BudgetCategoryType value is one of the defined constants.
func (s BudgetCategoryType) IsValid() bool {
	switch s {
	case BudgetCategoryTypeExpense, BudgetCategoryTypeIncome:
		return true
	}
	return false
}

// ChargeType classifies a charge for payment allocation purposes.
type ChargeType string

const (
	ChargeTypeRegularAssessment ChargeType = "regular_assessment"
	ChargeTypeSpecialAssessment ChargeType = "special_assessment"
	ChargeTypeLateFee           ChargeType = "late_fee"
	ChargeTypeInterest          ChargeType = "interest"
	ChargeTypeCollectionCost    ChargeType = "collection_cost"
	ChargeTypeAttorneyFee       ChargeType = "attorney_fee"
	ChargeTypeFine              ChargeType = "fine"
)

func (s ChargeType) IsValid() bool {
	switch s {
	case ChargeTypeRegularAssessment, ChargeTypeSpecialAssessment,
		ChargeTypeLateFee, ChargeTypeInterest,
		ChargeTypeCollectionCost, ChargeTypeAttorneyFee, ChargeTypeFine:
		return true
	}
	return false
}

// TriggeredBy identifies whether an action was initiated by the system or a user.
type TriggeredBy string

const (
	TriggeredBySystem TriggeredBy = "system"
	TriggeredByUser   TriggeredBy = "user"
)

// IsValid returns true if the TriggeredBy value is one of the defined constants.
func (s TriggeredBy) IsValid() bool {
	switch s {
	case TriggeredBySystem, TriggeredByUser:
		return true
	}
	return false
}
