package fin

import "testing"

func TestBudgetStatus_IsValid(t *testing.T) {
	valid := []BudgetStatus{BudgetStatusDraft, BudgetStatusProposed, BudgetStatusApproved}
	for _, v := range valid {
		if !v.IsValid() {
			t.Errorf("expected %q to be valid", v)
		}
	}
	invalid := []BudgetStatus{"", "unknown", "DRAFT"}
	for _, v := range invalid {
		if v.IsValid() {
			t.Errorf("expected %q to be invalid", v)
		}
	}
}

func TestExpenseStatus_IsValid(t *testing.T) {
	valid := []ExpenseStatus{ExpenseStatusSubmitted, ExpenseStatusApproved, ExpenseStatusPaid}
	for _, v := range valid {
		if !v.IsValid() {
			t.Errorf("expected %q to be valid", v)
		}
	}
	invalid := []ExpenseStatus{"", "unknown", "Pending"}
	for _, v := range invalid {
		if v.IsValid() {
			t.Errorf("expected %q to be invalid", v)
		}
	}
}

func TestPaymentStatus_IsValid(t *testing.T) {
	valid := []PaymentStatus{PaymentStatusPending, PaymentStatusCompleted, PaymentStatusFailed}
	for _, v := range valid {
		if !v.IsValid() {
			t.Errorf("expected %q to be valid", v)
		}
	}
	invalid := []PaymentStatus{"", "unknown", "Completed"}
	for _, v := range invalid {
		if v.IsValid() {
			t.Errorf("expected %q to be invalid", v)
		}
	}
}

func TestCollectionCaseStatus_IsValid(t *testing.T) {
	valid := []CollectionCaseStatus{CollectionCaseStatusLate, CollectionCaseStatusClosed, CollectionCaseStatusResolved}
	for _, v := range valid {
		if !v.IsValid() {
			t.Errorf("expected %q to be valid", v)
		}
	}
	invalid := []CollectionCaseStatus{"", "unknown", "Late"}
	for _, v := range invalid {
		if v.IsValid() {
			t.Errorf("expected %q to be invalid", v)
		}
	}
}

func TestPaymentPlanStatus_IsValid(t *testing.T) {
	valid := []PaymentPlanStatus{PaymentPlanStatusActive, PaymentPlanStatusDefaulted}
	for _, v := range valid {
		if !v.IsValid() {
			t.Errorf("expected %q to be valid", v)
		}
	}
	invalid := []PaymentPlanStatus{"", "unknown", "Active"}
	for _, v := range invalid {
		if v.IsValid() {
			t.Errorf("expected %q to be invalid", v)
		}
	}
}

func TestFundType_IsValid(t *testing.T) {
	valid := []FundType{FundTypeOperating, FundTypeReserve, FundTypeCapital, FundTypeSpecial}
	for _, v := range valid {
		if !v.IsValid() {
			t.Errorf("expected %q to be valid", v)
		}
	}
	invalid := []FundType{"", "unknown", "Operating"}
	for _, v := range invalid {
		if v.IsValid() {
			t.Errorf("expected %q to be invalid", v)
		}
	}
}

func TestLedgerEntryType_IsValid(t *testing.T) {
	valid := []LedgerEntryType{
		LedgerEntryTypeCharge, LedgerEntryTypePayment, LedgerEntryTypeCredit,
		LedgerEntryTypeAdjustment, LedgerEntryTypeLateFee,
	}
	for _, v := range valid {
		if !v.IsValid() {
			t.Errorf("expected %q to be valid", v)
		}
	}
	invalid := []LedgerEntryType{"", "unknown", "Charge"}
	for _, v := range invalid {
		if v.IsValid() {
			t.Errorf("expected %q to be invalid", v)
		}
	}
}

func TestGLSourceType_IsValid(t *testing.T) {
	valid := []GLSourceType{
		GLSourceTypeAssessment, GLSourceTypePayment, GLSourceTypeTransfer, GLSourceTypeManual,
	}
	for _, v := range valid {
		if !v.IsValid() {
			t.Errorf("expected %q to be valid", v)
		}
	}
	invalid := []GLSourceType{"", "unknown", "Assessment"}
	for _, v := range invalid {
		if v.IsValid() {
			t.Errorf("expected %q to be invalid", v)
		}
	}
}

func TestLedgerReferenceType_IsValid(t *testing.T) {
	valid := []LedgerReferenceType{LedgerRefTypePayment}
	for _, v := range valid {
		if !v.IsValid() {
			t.Errorf("expected %q to be valid", v)
		}
	}
	invalid := []LedgerReferenceType{"", "unknown", "Payment"}
	for _, v := range invalid {
		if v.IsValid() {
			t.Errorf("expected %q to be invalid", v)
		}
	}
}

func TestPaymentMethodType_IsValid(t *testing.T) {
	valid := []PaymentMethodType{PaymentMethodTypeCard, PaymentMethodTypeACH}
	for _, v := range valid {
		if !v.IsValid() {
			t.Errorf("expected %q to be valid", v)
		}
	}
	invalid := []PaymentMethodType{"", "unknown", "Card"}
	for _, v := range invalid {
		if v.IsValid() {
			t.Errorf("expected %q to be invalid", v)
		}
	}
}

func TestCollectionActionType_IsValid(t *testing.T) {
	valid := []CollectionActionType{CollectionActionTypeNoticeSent, CollectionActionTypeLienFiled}
	for _, v := range valid {
		if !v.IsValid() {
			t.Errorf("expected %q to be valid", v)
		}
	}
	invalid := []CollectionActionType{"", "unknown", "NoticeSent"}
	for _, v := range invalid {
		if v.IsValid() {
			t.Errorf("expected %q to be invalid", v)
		}
	}
}

func TestGLAccountType_IsValid(t *testing.T) {
	valid := []GLAccountType{
		GLAccountTypeAsset, GLAccountTypeLiability, GLAccountTypeEquity,
		GLAccountTypeRevenue, GLAccountTypeExpense,
	}
	for _, v := range valid {
		if !v.IsValid() {
			t.Errorf("expected %q to be valid", v)
		}
	}
	invalid := []GLAccountType{"", "unknown", "Asset"}
	for _, v := range invalid {
		if v.IsValid() {
			t.Errorf("expected %q to be invalid", v)
		}
	}
}

func TestAssessmentFrequency_IsValid(t *testing.T) {
	valid := []AssessmentFrequency{
		AssessmentFreqMonthly, AssessmentFreqQuarterly, AssessmentFreqSemiAnnually, AssessmentFreqAnnually,
	}
	for _, v := range valid {
		if !v.IsValid() {
			t.Errorf("expected %q to be valid", v)
		}
	}
	invalid := []AssessmentFrequency{"", "unknown", "Monthly"}
	for _, v := range invalid {
		if v.IsValid() {
			t.Errorf("expected %q to be invalid", v)
		}
	}
}

func TestPaymentPlanFrequency_IsValid(t *testing.T) {
	valid := []PaymentPlanFrequency{PaymentPlanFreqMonthly}
	for _, v := range valid {
		if !v.IsValid() {
			t.Errorf("expected %q to be valid", v)
		}
	}
	invalid := []PaymentPlanFrequency{"", "unknown", "Monthly"}
	for _, v := range invalid {
		if v.IsValid() {
			t.Errorf("expected %q to be invalid", v)
		}
	}
}

func TestAmountStrategy_IsValid(t *testing.T) {
	valid := []AmountStrategy{
		AmountStrategyFlat, AmountStrategyPerUnitType, AmountStrategyPerSqft, AmountStrategyCustom,
	}
	for _, v := range valid {
		if !v.IsValid() {
			t.Errorf("expected %q to be valid", v)
		}
	}
	invalid := []AmountStrategy{"", "unknown", "Flat"}
	for _, v := range invalid {
		if v.IsValid() {
			t.Errorf("expected %q to be invalid", v)
		}
	}
}

func TestBudgetCategoryType_IsValid(t *testing.T) {
	valid := []BudgetCategoryType{BudgetCategoryTypeExpense}
	for _, v := range valid {
		if !v.IsValid() {
			t.Errorf("expected %q to be valid", v)
		}
	}
	invalid := []BudgetCategoryType{"", "unknown", "Expense"}
	for _, v := range invalid {
		if v.IsValid() {
			t.Errorf("expected %q to be invalid", v)
		}
	}
}

func TestTriggeredBy_IsValid(t *testing.T) {
	valid := []TriggeredBy{TriggeredBySystem, TriggeredByUser}
	for _, v := range valid {
		if !v.IsValid() {
			t.Errorf("expected %q to be valid", v)
		}
	}
	invalid := []TriggeredBy{"", "unknown", "System"}
	for _, v := range invalid {
		if v.IsValid() {
			t.Errorf("expected %q to be invalid", v)
		}
	}
}
