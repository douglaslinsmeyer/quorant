package fin_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/fin"
)

// ---------------------------------------------------------------------------
// Assessment JSON serialization
// ---------------------------------------------------------------------------

func TestAssessment_JSONSerialization_RequiredFieldsPresent(t *testing.T) {
	id := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	orgID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	unitID := uuid.MustParse("00000000-0000-0000-0000-000000000003")
	now := time.Now().UTC().Truncate(time.Second)
	due := now.AddDate(0, 0, 30)

	a := fin.Assessment{
		ID:          id,
		OrgID:       orgID,
		UnitID:      unitID,
		Description: "Monthly HOA fee",
		AmountCents: 15000,
		DueDate:     due,
		IsRecurring: false,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	data, err := json.Marshal(a)
	if err != nil {
		t.Fatalf("json.Marshal(Assessment) error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	requiredKeys := []string{"id", "org_id", "unit_id", "description", "amount_cents", "due_date", "is_recurring", "created_at", "updated_at"}
	for _, key := range requiredKeys {
		if _, ok := result[key]; !ok {
			t.Errorf("expected JSON key %q to be present", key)
		}
	}
}

func TestAssessment_JSONSerialization_OmitsNilOptionalFields(t *testing.T) {
	now := time.Now().UTC()
	due := now.AddDate(0, 0, 30)

	a := fin.Assessment{
		ID:          uuid.New(),
		OrgID:       uuid.New(),
		UnitID:      uuid.New(),
		Description: "Monthly HOA fee",
		AmountCents: 15000,
		DueDate:     due,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	data, err := json.Marshal(a)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	omittedKeys := []string{"schedule_id", "late_fee_cents", "deleted_at"}
	for _, key := range omittedKeys {
		if _, ok := result[key]; ok {
			t.Errorf("expected JSON key %q to be omitted when nil", key)
		}
	}
}

func TestAssessment_JSONSerialization_OptionalFieldsIncludedWhenSet(t *testing.T) {
	now := time.Now().UTC()
	due := now.AddDate(0, 0, 30)
	scheduleID := uuid.New()
	lateFee := int64(2500)

	a := fin.Assessment{
		ID:           uuid.New(),
		OrgID:        uuid.New(),
		UnitID:       uuid.New(),
		ScheduleID:   &scheduleID,
		Description:  "Monthly HOA fee",
		AmountCents:  15000,
		DueDate:      due,
		LateFeeCents: &lateFee,
		IsRecurring:  true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	data, err := json.Marshal(a)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	presentKeys := []string{"schedule_id", "late_fee_cents"}
	for _, key := range presentKeys {
		if _, ok := result[key]; !ok {
			t.Errorf("expected JSON key %q to be present when set", key)
		}
	}

	if result["is_recurring"] != true {
		t.Errorf("is_recurring: got %v, want true", result["is_recurring"])
	}
}

// ---------------------------------------------------------------------------
// Payment JSON serialization
// ---------------------------------------------------------------------------

func TestPayment_JSONSerialization_RequiredFieldsPresent(t *testing.T) {
	id := uuid.MustParse("00000000-0000-0000-0000-000000000011")
	orgID := uuid.MustParse("00000000-0000-0000-0000-000000000012")
	unitID := uuid.MustParse("00000000-0000-0000-0000-000000000013")
	userID := uuid.MustParse("00000000-0000-0000-0000-000000000014")
	now := time.Now().UTC().Truncate(time.Second)

	p := fin.Payment{
		ID:          id,
		OrgID:       orgID,
		UnitID:      unitID,
		UserID:      userID,
		AmountCents: 15000,
		Status:      fin.PaymentStatusCompleted,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("json.Marshal(Payment) error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	requiredKeys := []string{"id", "org_id", "unit_id", "user_id", "amount_cents", "status", "created_at", "updated_at"}
	for _, key := range requiredKeys {
		if _, ok := result[key]; !ok {
			t.Errorf("expected JSON key %q to be present", key)
		}
	}
}

func TestPayment_JSONSerialization_OmitsNilOptionalFields(t *testing.T) {
	now := time.Now().UTC()

	p := fin.Payment{
		ID:          uuid.New(),
		OrgID:       uuid.New(),
		UnitID:      uuid.New(),
		UserID:      uuid.New(),
		AmountCents: 15000,
		Status:      fin.PaymentStatusPending,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	omittedKeys := []string{"payment_method_id", "provider_ref", "description", "paid_at"}
	for _, key := range omittedKeys {
		if _, ok := result[key]; ok {
			t.Errorf("expected JSON key %q to be omitted when nil", key)
		}
	}
}

// ---------------------------------------------------------------------------
// Fund JSON serialization
// ---------------------------------------------------------------------------

func TestFund_JSONSerialization_RequiredFieldsPresent(t *testing.T) {
	id := uuid.MustParse("00000000-0000-0000-0000-000000000021")
	orgID := uuid.MustParse("00000000-0000-0000-0000-000000000022")
	now := time.Now().UTC().Truncate(time.Second)

	f := fin.Fund{
		ID:           id,
		OrgID:        orgID,
		Name:         "Operating Fund",
		FundType:     fin.FundTypeOperating,
		BalanceCents: 500000,
		IsDefault:    true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	data, err := json.Marshal(f)
	if err != nil {
		t.Fatalf("json.Marshal(Fund) error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	requiredKeys := []string{"id", "org_id", "name", "fund_type", "balance_cents", "is_default", "created_at", "updated_at"}
	for _, key := range requiredKeys {
		if _, ok := result[key]; !ok {
			t.Errorf("expected JSON key %q to be present", key)
		}
	}
}

func TestFund_JSONSerialization_OmitsNilOptionalFields(t *testing.T) {
	now := time.Now().UTC()

	f := fin.Fund{
		ID:           uuid.New(),
		OrgID:        uuid.New(),
		Name:         "Reserve Fund",
		FundType:     fin.FundTypeReserve,
		BalanceCents: 100000,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	data, err := json.Marshal(f)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	omittedKeys := []string{"target_balance_cents", "deleted_at"}
	for _, key := range omittedKeys {
		if _, ok := result[key]; ok {
			t.Errorf("expected JSON key %q to be omitted when nil", key)
		}
	}
}

// ---------------------------------------------------------------------------
// CreateAssessmentScheduleRequest.Validate()
// ---------------------------------------------------------------------------

func TestCreateAssessmentScheduleRequest_Validate_ValidRequest(t *testing.T) {
	now := time.Now()
	req := fin.CreateAssessmentScheduleRequest{
		Name:             "Monthly HOA",
		Frequency:        "monthly",
		AmountStrategy:   "flat",
		BaseAmountCents:  15000,
		StartsAt:         now,
	}
	if err := req.Validate(); err != nil {
		t.Errorf("expected nil error for valid request, got: %v", err)
	}
}

func TestCreateAssessmentScheduleRequest_Validate_MissingNameReturnsError(t *testing.T) {
	now := time.Now()
	req := fin.CreateAssessmentScheduleRequest{
		Frequency:       "monthly",
		AmountStrategy:  "flat",
		BaseAmountCents: 15000,
		StartsAt:        now,
	}
	if err := req.Validate(); err == nil {
		t.Error("expected error when name is missing, got nil")
	}
}

func TestCreateAssessmentScheduleRequest_Validate_MissingFrequencyReturnsError(t *testing.T) {
	now := time.Now()
	req := fin.CreateAssessmentScheduleRequest{
		Name:            "Monthly HOA",
		AmountStrategy:  "flat",
		BaseAmountCents: 15000,
		StartsAt:        now,
	}
	if err := req.Validate(); err == nil {
		t.Error("expected error when frequency is missing, got nil")
	}
}

func TestCreateAssessmentScheduleRequest_Validate_InvalidFrequencyReturnsError(t *testing.T) {
	now := time.Now()
	req := fin.CreateAssessmentScheduleRequest{
		Name:            "Monthly HOA",
		Frequency:       "biweekly",
		AmountStrategy:  "flat",
		BaseAmountCents: 15000,
		StartsAt:        now,
	}
	if err := req.Validate(); err == nil {
		t.Error("expected error for invalid frequency, got nil")
	}
}

func TestCreateAssessmentScheduleRequest_Validate_AllValidFrequencies(t *testing.T) {
	now := time.Now()
	for _, freq := range []string{"monthly", "quarterly", "annually", "semi_annually"} {
		req := fin.CreateAssessmentScheduleRequest{
			Name:            "Test",
			Frequency:       freq,
			AmountStrategy:  "flat",
			BaseAmountCents: 1000,
			StartsAt:        now,
		}
		if err := req.Validate(); err != nil {
			t.Errorf("expected nil error for frequency %q, got: %v", freq, err)
		}
	}
}

func TestCreateAssessmentScheduleRequest_Validate_MissingAmountStrategyReturnsError(t *testing.T) {
	now := time.Now()
	req := fin.CreateAssessmentScheduleRequest{
		Name:            "Monthly HOA",
		Frequency:       "monthly",
		BaseAmountCents: 15000,
		StartsAt:        now,
	}
	if err := req.Validate(); err == nil {
		t.Error("expected error when amount_strategy is missing, got nil")
	}
}

func TestCreateAssessmentScheduleRequest_Validate_InvalidAmountStrategyReturnsError(t *testing.T) {
	now := time.Now()
	req := fin.CreateAssessmentScheduleRequest{
		Name:            "Monthly HOA",
		Frequency:       "monthly",
		AmountStrategy:  "unknown_strategy",
		BaseAmountCents: 15000,
		StartsAt:        now,
	}
	if err := req.Validate(); err == nil {
		t.Error("expected error for invalid amount_strategy, got nil")
	}
}

func TestCreateAssessmentScheduleRequest_Validate_AllValidAmountStrategies(t *testing.T) {
	now := time.Now()
	for _, strategy := range []string{"flat", "per_unit_type", "per_sqft", "custom"} {
		req := fin.CreateAssessmentScheduleRequest{
			Name:            "Test",
			Frequency:       "monthly",
			AmountStrategy:  strategy,
			BaseAmountCents: 1000,
			StartsAt:        now,
		}
		if err := req.Validate(); err != nil {
			t.Errorf("expected nil error for amount_strategy %q, got: %v", strategy, err)
		}
	}
}

func TestCreateAssessmentScheduleRequest_Validate_MissingStartsAtReturnsError(t *testing.T) {
	req := fin.CreateAssessmentScheduleRequest{
		Name:            "Monthly HOA",
		Frequency:       "monthly",
		AmountStrategy:  "flat",
		BaseAmountCents: 15000,
	}
	if err := req.Validate(); err == nil {
		t.Error("expected error when starts_at is zero, got nil")
	}
}

// ---------------------------------------------------------------------------
// CreateAssessmentRequest.Validate()
// ---------------------------------------------------------------------------

func TestCreateAssessmentRequest_Validate_ValidRequest(t *testing.T) {
	req := fin.CreateAssessmentRequest{
		UnitID:      uuid.New(),
		Description: "Monthly fee",
		AmountCents: 15000,
		DueDate:     time.Now().AddDate(0, 0, 30),
	}
	if err := req.Validate(); err != nil {
		t.Errorf("expected nil error for valid request, got: %v", err)
	}
}

func TestCreateAssessmentRequest_Validate_MissingUnitIDReturnsError(t *testing.T) {
	req := fin.CreateAssessmentRequest{
		Description: "Monthly fee",
		AmountCents: 15000,
		DueDate:     time.Now().AddDate(0, 0, 30),
	}
	if err := req.Validate(); err == nil {
		t.Error("expected error when unit_id is zero, got nil")
	}
}

func TestCreateAssessmentRequest_Validate_MissingDescriptionReturnsError(t *testing.T) {
	req := fin.CreateAssessmentRequest{
		UnitID:      uuid.New(),
		AmountCents: 15000,
		DueDate:     time.Now().AddDate(0, 0, 30),
	}
	if err := req.Validate(); err == nil {
		t.Error("expected error when description is missing, got nil")
	}
}

func TestCreateAssessmentRequest_Validate_MissingAmountCentsReturnsError(t *testing.T) {
	req := fin.CreateAssessmentRequest{
		UnitID:      uuid.New(),
		Description: "Monthly fee",
		DueDate:     time.Now().AddDate(0, 0, 30),
	}
	if err := req.Validate(); err == nil {
		t.Error("expected error when amount_cents is zero, got nil")
	}
}

func TestCreateAssessmentRequest_Validate_MissingDueDateReturnsError(t *testing.T) {
	req := fin.CreateAssessmentRequest{
		UnitID:      uuid.New(),
		Description: "Monthly fee",
		AmountCents: 15000,
	}
	if err := req.Validate(); err == nil {
		t.Error("expected error when due_date is zero, got nil")
	}
}

// ---------------------------------------------------------------------------
// CreatePaymentRequest.Validate()
// ---------------------------------------------------------------------------

func TestCreatePaymentRequest_Validate_ValidRequest(t *testing.T) {
	req := fin.CreatePaymentRequest{
		UnitID:      uuid.New(),
		AmountCents: 15000,
	}
	if err := req.Validate(); err != nil {
		t.Errorf("expected nil error for valid request, got: %v", err)
	}
}

func TestCreatePaymentRequest_Validate_MissingUnitIDReturnsError(t *testing.T) {
	req := fin.CreatePaymentRequest{
		AmountCents: 15000,
	}
	if err := req.Validate(); err == nil {
		t.Error("expected error when unit_id is zero, got nil")
	}
}

func TestCreatePaymentRequest_Validate_ZeroAmountCentsReturnsError(t *testing.T) {
	req := fin.CreatePaymentRequest{
		UnitID:      uuid.New(),
		AmountCents: 0,
	}
	if err := req.Validate(); err == nil {
		t.Error("expected error when amount_cents is zero, got nil")
	}
}

func TestCreatePaymentRequest_Validate_NegativeAmountCentsReturnsError(t *testing.T) {
	req := fin.CreatePaymentRequest{
		UnitID:      uuid.New(),
		AmountCents: -100,
	}
	if err := req.Validate(); err == nil {
		t.Error("expected error when amount_cents is negative, got nil")
	}
}

// ---------------------------------------------------------------------------
// CreateBudgetRequest.Validate()
// ---------------------------------------------------------------------------

func TestCreateBudgetRequest_Validate_ValidRequest(t *testing.T) {
	req := fin.CreateBudgetRequest{
		FiscalYear: 2025,
		Name:       "FY2025 Annual Budget",
	}
	if err := req.Validate(); err != nil {
		t.Errorf("expected nil error for valid request, got: %v", err)
	}
}

func TestCreateBudgetRequest_Validate_MissingFiscalYearReturnsError(t *testing.T) {
	req := fin.CreateBudgetRequest{
		Name: "FY2025 Annual Budget",
	}
	if err := req.Validate(); err == nil {
		t.Error("expected error when fiscal_year is zero, got nil")
	}
}

func TestCreateBudgetRequest_Validate_MissingNameReturnsError(t *testing.T) {
	req := fin.CreateBudgetRequest{
		FiscalYear: 2025,
	}
	if err := req.Validate(); err == nil {
		t.Error("expected error when name is missing, got nil")
	}
}

// ---------------------------------------------------------------------------
// CreateExpenseRequest.Validate()
// ---------------------------------------------------------------------------

func TestCreateExpenseRequest_Validate_ValidRequest(t *testing.T) {
	req := fin.CreateExpenseRequest{
		Description: "Landscaping service",
		AmountCents: 50000,
		ExpenseDate: time.Now(),
	}
	if err := req.Validate(); err != nil {
		t.Errorf("expected nil error for valid request, got: %v", err)
	}
}

func TestCreateExpenseRequest_Validate_MissingDescriptionReturnsError(t *testing.T) {
	req := fin.CreateExpenseRequest{
		AmountCents: 50000,
		ExpenseDate: time.Now(),
	}
	if err := req.Validate(); err == nil {
		t.Error("expected error when description is missing, got nil")
	}
}

func TestCreateExpenseRequest_Validate_MissingAmountCentsReturnsError(t *testing.T) {
	req := fin.CreateExpenseRequest{
		Description: "Landscaping service",
		ExpenseDate: time.Now(),
	}
	if err := req.Validate(); err == nil {
		t.Error("expected error when amount_cents is zero, got nil")
	}
}

func TestCreateExpenseRequest_Validate_MissingExpenseDateReturnsError(t *testing.T) {
	req := fin.CreateExpenseRequest{
		Description: "Landscaping service",
		AmountCents: 50000,
	}
	if err := req.Validate(); err == nil {
		t.Error("expected error when expense_date is zero, got nil")
	}
}

// ---------------------------------------------------------------------------
// CreateFundRequest.Validate()
// ---------------------------------------------------------------------------

func TestCreateFundRequest_Validate_ValidRequest(t *testing.T) {
	req := fin.CreateFundRequest{
		Name:     "Operating Fund",
		FundType: "operating",
	}
	if err := req.Validate(); err != nil {
		t.Errorf("expected nil error for valid request, got: %v", err)
	}
}

func TestCreateFundRequest_Validate_MissingNameReturnsError(t *testing.T) {
	req := fin.CreateFundRequest{
		FundType: "operating",
	}
	if err := req.Validate(); err == nil {
		t.Error("expected error when name is missing, got nil")
	}
}

func TestCreateFundRequest_Validate_MissingFundTypeReturnsError(t *testing.T) {
	req := fin.CreateFundRequest{
		Name: "Operating Fund",
	}
	if err := req.Validate(); err == nil {
		t.Error("expected error when fund_type is missing, got nil")
	}
}

func TestCreateFundRequest_Validate_InvalidFundTypeReturnsError(t *testing.T) {
	req := fin.CreateFundRequest{
		Name:     "Special Fund",
		FundType: "emergency",
	}
	if err := req.Validate(); err == nil {
		t.Error("expected error for invalid fund_type, got nil")
	}
}

func TestCreateFundRequest_Validate_AllValidFundTypes(t *testing.T) {
	for _, ft := range []string{"operating", "reserve", "capital", "special"} {
		req := fin.CreateFundRequest{
			Name:     "Test Fund",
			FundType: ft,
		}
		if err := req.Validate(); err != nil {
			t.Errorf("expected nil error for fund_type %q, got: %v", ft, err)
		}
	}
}

// ---------------------------------------------------------------------------
// CreateFundTransferRequest.Validate()
// ---------------------------------------------------------------------------

func TestCreateFundTransferRequest_Validate_ValidRequest(t *testing.T) {
	req := fin.CreateFundTransferRequest{
		FromFundID:  uuid.New(),
		ToFundID:    uuid.New(),
		AmountCents: 10000,
	}
	if err := req.Validate(); err != nil {
		t.Errorf("expected nil error for valid request, got: %v", err)
	}
}

func TestCreateFundTransferRequest_Validate_MissingFromFundIDReturnsError(t *testing.T) {
	req := fin.CreateFundTransferRequest{
		ToFundID:    uuid.New(),
		AmountCents: 10000,
	}
	if err := req.Validate(); err == nil {
		t.Error("expected error when from_fund_id is zero, got nil")
	}
}

func TestCreateFundTransferRequest_Validate_MissingToFundIDReturnsError(t *testing.T) {
	req := fin.CreateFundTransferRequest{
		FromFundID:  uuid.New(),
		AmountCents: 10000,
	}
	if err := req.Validate(); err == nil {
		t.Error("expected error when to_fund_id is zero, got nil")
	}
}

func TestCreateFundTransferRequest_Validate_ZeroAmountCentsReturnsError(t *testing.T) {
	req := fin.CreateFundTransferRequest{
		FromFundID:  uuid.New(),
		ToFundID:    uuid.New(),
		AmountCents: 0,
	}
	if err := req.Validate(); err == nil {
		t.Error("expected error when amount_cents is zero, got nil")
	}
}

func TestCreateFundTransferRequest_Validate_NegativeAmountCentsReturnsError(t *testing.T) {
	req := fin.CreateFundTransferRequest{
		FromFundID:  uuid.New(),
		ToFundID:    uuid.New(),
		AmountCents: -500,
	}
	if err := req.Validate(); err == nil {
		t.Error("expected error when amount_cents is negative, got nil")
	}
}

// ---------------------------------------------------------------------------
// CreateCollectionActionRequest.Validate()
// ---------------------------------------------------------------------------

func TestCreateCollectionActionRequest_Validate_ValidRequest(t *testing.T) {
	req := fin.CreateCollectionActionRequest{
		ActionType: "notice_sent",
	}
	if err := req.Validate(); err != nil {
		t.Errorf("expected nil error for valid request, got: %v", err)
	}
}

func TestCreateCollectionActionRequest_Validate_WithNotes(t *testing.T) {
	notes := "Sent certified mail"
	req := fin.CreateCollectionActionRequest{
		ActionType: "notice_sent",
		Notes:      &notes,
	}
	if err := req.Validate(); err != nil {
		t.Errorf("expected nil error for valid request with notes, got: %v", err)
	}
}

func TestCreateCollectionActionRequest_Validate_MissingActionTypeReturnsError(t *testing.T) {
	req := fin.CreateCollectionActionRequest{}
	if err := req.Validate(); err == nil {
		t.Error("expected error when action_type is missing, got nil")
	}
}

// ---------------------------------------------------------------------------
// CreatePaymentPlanRequest.Validate()
// ---------------------------------------------------------------------------

func TestCreatePaymentPlanRequest_Validate_ValidRequest(t *testing.T) {
	req := fin.CreatePaymentPlanRequest{
		TotalOwedCents:     300000,
		InstallmentCents:   50000,
		InstallmentsTotal:  6,
		NextDueDate:        time.Now().AddDate(0, 1, 0),
	}
	if err := req.Validate(); err != nil {
		t.Errorf("expected nil error for valid request, got: %v", err)
	}
}

func TestCreatePaymentPlanRequest_Validate_MissingTotalOwedCentsReturnsError(t *testing.T) {
	req := fin.CreatePaymentPlanRequest{
		InstallmentCents:  50000,
		InstallmentsTotal: 6,
		NextDueDate:       time.Now().AddDate(0, 1, 0),
	}
	if err := req.Validate(); err == nil {
		t.Error("expected error when total_owed_cents is zero, got nil")
	}
}

func TestCreatePaymentPlanRequest_Validate_MissingInstallmentCentsReturnsError(t *testing.T) {
	req := fin.CreatePaymentPlanRequest{
		TotalOwedCents:    300000,
		InstallmentsTotal: 6,
		NextDueDate:       time.Now().AddDate(0, 1, 0),
	}
	if err := req.Validate(); err == nil {
		t.Error("expected error when installment_cents is zero, got nil")
	}
}

func TestCreatePaymentPlanRequest_Validate_MissingInstallmentsTotalReturnsError(t *testing.T) {
	req := fin.CreatePaymentPlanRequest{
		TotalOwedCents:   300000,
		InstallmentCents: 50000,
		NextDueDate:      time.Now().AddDate(0, 1, 0),
	}
	if err := req.Validate(); err == nil {
		t.Error("expected error when installments_total is zero, got nil")
	}
}

func TestCreatePaymentPlanRequest_Validate_MissingNextDueDateReturnsError(t *testing.T) {
	req := fin.CreatePaymentPlanRequest{
		TotalOwedCents:    300000,
		InstallmentCents:  50000,
		InstallmentsTotal: 6,
	}
	if err := req.Validate(); err == nil {
		t.Error("expected error when next_due_date is zero, got nil")
	}
}

// ---------------------------------------------------------------------------
// GLAccount JSON serialization
// ---------------------------------------------------------------------------

func TestGLAccount_JSONSerialization_RequiredFieldsPresent(t *testing.T) {
	id := uuid.MustParse("00000000-0000-0000-0000-000000000031")
	orgID := uuid.MustParse("00000000-0000-0000-0000-000000000032")
	now := time.Now().UTC().Truncate(time.Second)

	a := fin.GLAccount{
		ID:            id,
		OrgID:         orgID,
		AccountNumber: 1000,
		Name:          "Cash",
		AccountType:   "asset",
		IsHeader:      false,
		IsSystem:      true,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	data, err := json.Marshal(a)
	if err != nil {
		t.Fatalf("json.Marshal(GLAccount) error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	requiredKeys := []string{"id", "org_id", "account_number", "name", "account_type", "is_header", "is_system", "created_at", "updated_at"}
	for _, key := range requiredKeys {
		if _, ok := result[key]; !ok {
			t.Errorf("expected JSON key %q to be present", key)
		}
	}
}

func TestGLAccount_JSONSerialization_OmitsNilOptionalFields(t *testing.T) {
	now := time.Now().UTC()

	a := fin.GLAccount{
		ID:            uuid.New(),
		OrgID:         uuid.New(),
		AccountNumber: 1000,
		Name:          "Cash",
		AccountType:   "asset",
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	data, err := json.Marshal(a)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	omittedKeys := []string{"parent_id", "fund_id", "description", "deleted_at"}
	for _, key := range omittedKeys {
		if _, ok := result[key]; ok {
			t.Errorf("expected JSON key %q to be omitted when nil", key)
		}
	}
}

func TestGLAccount_JSONSerialization_OptionalFieldsIncludedWhenSet(t *testing.T) {
	now := time.Now().UTC()
	parentID := uuid.New()
	fundID := uuid.New()
	desc := "Main cash account"

	a := fin.GLAccount{
		ID:            uuid.New(),
		OrgID:         uuid.New(),
		ParentID:      &parentID,
		FundID:        &fundID,
		AccountNumber: 1000,
		Name:          "Cash",
		AccountType:   "asset",
		IsHeader:      false,
		IsSystem:      false,
		Description:   &desc,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	data, err := json.Marshal(a)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	presentKeys := []string{"parent_id", "fund_id", "description"}
	for _, key := range presentKeys {
		if _, ok := result[key]; !ok {
			t.Errorf("expected JSON key %q to be present when set", key)
		}
	}
}

func TestGLAccount_JSONSerialization_RoundTrip(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	parentID := uuid.New()
	desc := "Operating cash"

	original := fin.GLAccount{
		ID:            uuid.New(),
		OrgID:         uuid.New(),
		ParentID:      &parentID,
		AccountNumber: 1010,
		Name:          "Operating Cash",
		AccountType:   "asset",
		IsHeader:      false,
		IsSystem:      true,
		Description:   &desc,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var decoded fin.GLAccount
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	if decoded.ID != original.ID {
		t.Errorf("ID: got %v, want %v", decoded.ID, original.ID)
	}
	if decoded.OrgID != original.OrgID {
		t.Errorf("OrgID: got %v, want %v", decoded.OrgID, original.OrgID)
	}
	if decoded.ParentID == nil || *decoded.ParentID != parentID {
		t.Errorf("ParentID: got %v, want %v", decoded.ParentID, parentID)
	}
	if decoded.AccountNumber != original.AccountNumber {
		t.Errorf("AccountNumber: got %d, want %d", decoded.AccountNumber, original.AccountNumber)
	}
	if decoded.Name != original.Name {
		t.Errorf("Name: got %q, want %q", decoded.Name, original.Name)
	}
	if decoded.AccountType != original.AccountType {
		t.Errorf("AccountType: got %q, want %q", decoded.AccountType, original.AccountType)
	}
	if decoded.IsSystem != original.IsSystem {
		t.Errorf("IsSystem: got %v, want %v", decoded.IsSystem, original.IsSystem)
	}
	if decoded.Description == nil || *decoded.Description != desc {
		t.Errorf("Description: got %v, want %q", decoded.Description, desc)
	}
	if !decoded.CreatedAt.Equal(original.CreatedAt) {
		t.Errorf("CreatedAt: got %v, want %v", decoded.CreatedAt, original.CreatedAt)
	}
}

// ---------------------------------------------------------------------------
// GLJournalEntry JSON serialization
// ---------------------------------------------------------------------------

func TestGLJournalEntry_JSONSerialization_RequiredFieldsPresent(t *testing.T) {
	id := uuid.MustParse("00000000-0000-0000-0000-000000000041")
	orgID := uuid.MustParse("00000000-0000-0000-0000-000000000042")
	postedBy := uuid.MustParse("00000000-0000-0000-0000-000000000043")
	now := time.Now().UTC().Truncate(time.Second)

	je := fin.GLJournalEntry{
		ID:          id,
		OrgID:       orgID,
		EntryNumber: 1,
		EntryDate:   now,
		Memo:        "Opening balances",
		PostedBy:    postedBy,
		IsReversal:  false,
		CreatedAt:   now,
	}

	data, err := json.Marshal(je)
	if err != nil {
		t.Fatalf("json.Marshal(GLJournalEntry) error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	requiredKeys := []string{"id", "org_id", "entry_number", "entry_date", "memo", "posted_by", "is_reversal", "created_at"}
	for _, key := range requiredKeys {
		if _, ok := result[key]; !ok {
			t.Errorf("expected JSON key %q to be present", key)
		}
	}
}

func TestGLJournalEntry_JSONSerialization_OmitsNilOptionalFields(t *testing.T) {
	now := time.Now().UTC()

	je := fin.GLJournalEntry{
		ID:          uuid.New(),
		OrgID:       uuid.New(),
		EntryNumber: 1,
		EntryDate:   now,
		Memo:        "Test entry",
		PostedBy:    uuid.New(),
		CreatedAt:   now,
	}

	data, err := json.Marshal(je)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	omittedKeys := []string{"source_type", "source_id", "unit_id", "reversed_by", "lines"}
	for _, key := range omittedKeys {
		if _, ok := result[key]; ok {
			t.Errorf("expected JSON key %q to be omitted when nil/empty", key)
		}
	}
}

func TestGLJournalEntry_JSONSerialization_OptionalFieldsIncludedWhenSet(t *testing.T) {
	now := time.Now().UTC()
	sourceType := fin.GLSourceTypeAssessment
	sourceID := uuid.New()
	unitID := uuid.New()

	je := fin.GLJournalEntry{
		ID:          uuid.New(),
		OrgID:       uuid.New(),
		EntryNumber: 5,
		EntryDate:   now,
		Memo:        "Assessment charge",
		SourceType:  &sourceType,
		SourceID:    &sourceID,
		UnitID:      &unitID,
		PostedBy:    uuid.New(),
		CreatedAt:   now,
		Lines: []fin.GLJournalLine{
			{
				ID:             uuid.New(),
				JournalEntryID: uuid.New(),
				AccountID:      uuid.New(),
				DebitCents:     10000,
				CreditCents:    0,
			},
		},
	}

	data, err := json.Marshal(je)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	presentKeys := []string{"source_type", "source_id", "unit_id", "lines"}
	for _, key := range presentKeys {
		if _, ok := result[key]; !ok {
			t.Errorf("expected JSON key %q to be present when set", key)
		}
	}
}

func TestGLJournalEntry_JSONSerialization_RoundTrip(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	sourceType := fin.GLSourceTypeManual
	sourceID := uuid.New()
	lineMemo := "Debit office supplies"

	lineID := uuid.New()
	entryID := uuid.New()
	accountID := uuid.New()

	original := fin.GLJournalEntry{
		ID:          entryID,
		OrgID:       uuid.New(),
		EntryNumber: 42,
		EntryDate:   now,
		Memo:        "Office supplies purchase",
		SourceType:  &sourceType,
		SourceID:    &sourceID,
		PostedBy:    uuid.New(),
		IsReversal:  false,
		CreatedAt:   now,
		Lines: []fin.GLJournalLine{
			{
				ID:             lineID,
				JournalEntryID: entryID,
				AccountID:      accountID,
				DebitCents:     5000,
				CreditCents:    0,
				Memo:           &lineMemo,
			},
			{
				ID:             uuid.New(),
				JournalEntryID: entryID,
				AccountID:      uuid.New(),
				DebitCents:     0,
				CreditCents:    5000,
			},
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var decoded fin.GLJournalEntry
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	if decoded.ID != original.ID {
		t.Errorf("ID: got %v, want %v", decoded.ID, original.ID)
	}
	if decoded.EntryNumber != original.EntryNumber {
		t.Errorf("EntryNumber: got %d, want %d", decoded.EntryNumber, original.EntryNumber)
	}
	if decoded.Memo != original.Memo {
		t.Errorf("Memo: got %q, want %q", decoded.Memo, original.Memo)
	}
	if decoded.SourceType == nil || *decoded.SourceType != sourceType {
		t.Errorf("SourceType: got %v, want %q", decoded.SourceType, sourceType)
	}
	if !decoded.EntryDate.Equal(original.EntryDate) {
		t.Errorf("EntryDate: got %v, want %v", decoded.EntryDate, original.EntryDate)
	}
	if len(decoded.Lines) != 2 {
		t.Fatalf("Lines: got %d, want 2", len(decoded.Lines))
	}
	if decoded.Lines[0].DebitCents != 5000 {
		t.Errorf("Lines[0].DebitCents: got %d, want 5000", decoded.Lines[0].DebitCents)
	}
	if decoded.Lines[0].Memo == nil || *decoded.Lines[0].Memo != lineMemo {
		t.Errorf("Lines[0].Memo: got %v, want %q", decoded.Lines[0].Memo, lineMemo)
	}
	if decoded.Lines[1].CreditCents != 5000 {
		t.Errorf("Lines[1].CreditCents: got %d, want 5000", decoded.Lines[1].CreditCents)
	}
}
