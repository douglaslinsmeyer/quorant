package license_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/license"
)

// ---------------------------------------------------------------------------
// Plan JSON serialization
// ---------------------------------------------------------------------------

func TestPlan_JSONSerialization_RequiredFieldsPresent(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	p := license.Plan{
		ID:         uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		Name:       "Starter",
		PlanType:   "hoa",
		PriceCents: 1999,
		IsActive:   true,
		Metadata:   map[string]any{},
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("json.Marshal(Plan) error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	for _, key := range []string{"id", "name", "plan_type", "price_cents", "is_active", "metadata", "created_at", "updated_at"} {
		if _, ok := result[key]; !ok {
			t.Errorf("expected JSON key %q to be present", key)
		}
	}
}

func TestPlan_JSONSerialization_OmitsNilDescription(t *testing.T) {
	now := time.Now().UTC()
	p := license.Plan{
		ID:        uuid.New(),
		Name:      "Basic",
		PlanType:  "firm",
		Metadata:  map[string]any{},
		CreatedAt: now,
		UpdatedAt: now,
	}

	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	if _, ok := result["description"]; ok {
		t.Error("expected 'description' to be omitted when nil")
	}
}

// ---------------------------------------------------------------------------
// Entitlement JSON serialization
// ---------------------------------------------------------------------------

func TestEntitlement_JSONSerialization_BooleanType(t *testing.T) {
	e := license.Entitlement{
		ID:         uuid.New(),
		PlanID:     uuid.New(),
		FeatureKey: "document_storage",
		LimitType:  "boolean",
		CreatedAt:  time.Now().UTC(),
	}

	data, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("json.Marshal(Entitlement) error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	if _, ok := result["limit_value"]; ok {
		t.Error("expected 'limit_value' to be omitted for boolean entitlement")
	}
	if result["limit_type"] != "boolean" {
		t.Errorf("limit_type: got %v, want boolean", result["limit_type"])
	}
}

func TestEntitlement_JSONSerialization_NumericType(t *testing.T) {
	val := int64(10)
	e := license.Entitlement{
		ID:         uuid.New(),
		PlanID:     uuid.New(),
		FeatureKey: "max_units",
		LimitType:  "numeric",
		LimitValue: &val,
		CreatedAt:  time.Now().UTC(),
	}

	data, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("json.Marshal(Entitlement) error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	if _, ok := result["limit_value"]; !ok {
		t.Error("expected 'limit_value' to be present for numeric entitlement")
	}
}

// ---------------------------------------------------------------------------
// OrgSubscription JSON serialization
// ---------------------------------------------------------------------------

func TestOrgSubscription_JSONSerialization_OmitsNilPointers(t *testing.T) {
	now := time.Now().UTC()
	s := license.OrgSubscription{
		ID:        uuid.New(),
		OrgID:     uuid.New(),
		PlanID:    uuid.New(),
		Status:    "active",
		StartsAt:  now,
		CreatedAt: now,
		UpdatedAt: now,
	}

	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("json.Marshal(OrgSubscription) error: %v", err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	for _, key := range []string{"ends_at", "trial_ends_at", "stripe_sub_id"} {
		if _, ok := result[key]; ok {
			t.Errorf("expected %q to be omitted when nil", key)
		}
	}
}

// ---------------------------------------------------------------------------
// EntitlementOverride JSON serialization
// ---------------------------------------------------------------------------

func TestEntitlementOverride_JSONSerialization_OmitsNilPointers(t *testing.T) {
	o := license.EntitlementOverride{
		ID:         uuid.New(),
		OrgID:      uuid.New(),
		FeatureKey: "special_feature",
		CreatedAt:  time.Now().UTC(),
	}

	data, err := json.Marshal(o)
	if err != nil {
		t.Fatalf("json.Marshal(EntitlementOverride) error: %v", err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	for _, key := range []string{"limit_value", "reason", "granted_by", "expires_at"} {
		if _, ok := result[key]; ok {
			t.Errorf("expected %q to be omitted when nil", key)
		}
	}
}

// ---------------------------------------------------------------------------
// CreatePlanRequest.Validate()
// ---------------------------------------------------------------------------

func TestCreatePlanRequest_Validate_ValidFirmPlan(t *testing.T) {
	req := license.CreatePlanRequest{Name: "Pro", PlanType: "firm"}
	if err := req.Validate(); err != nil {
		t.Errorf("expected nil error for valid firm plan, got: %v", err)
	}
}

func TestCreatePlanRequest_Validate_ValidHOAPlan(t *testing.T) {
	req := license.CreatePlanRequest{Name: "Basic", PlanType: "hoa"}
	if err := req.Validate(); err != nil {
		t.Errorf("expected nil error for valid hoa plan, got: %v", err)
	}
}

func TestCreatePlanRequest_Validate_ValidFirmBundle(t *testing.T) {
	req := license.CreatePlanRequest{Name: "Enterprise", PlanType: "firm_bundle"}
	if err := req.Validate(); err != nil {
		t.Errorf("expected nil error for valid firm_bundle plan, got: %v", err)
	}
}

func TestCreatePlanRequest_Validate_MissingNameReturnsError(t *testing.T) {
	req := license.CreatePlanRequest{PlanType: "hoa"}
	if err := req.Validate(); err == nil {
		t.Error("expected error when name is missing, got nil")
	}
}

func TestCreatePlanRequest_Validate_InvalidPlanTypeReturnsError(t *testing.T) {
	req := license.CreatePlanRequest{Name: "X", PlanType: "invalid"}
	if err := req.Validate(); err == nil {
		t.Error("expected error for invalid plan_type, got nil")
	}
}

func TestCreatePlanRequest_Validate_EmptyPlanTypeReturnsError(t *testing.T) {
	req := license.CreatePlanRequest{Name: "X"}
	if err := req.Validate(); err == nil {
		t.Error("expected error when plan_type is empty, got nil")
	}
}

// ---------------------------------------------------------------------------
// CreateSubscriptionRequest.Validate()
// ---------------------------------------------------------------------------

func TestCreateSubscriptionRequest_Validate_ValidRequest(t *testing.T) {
	req := license.CreateSubscriptionRequest{
		OrgID:  uuid.New(),
		PlanID: uuid.New(),
	}
	if err := req.Validate(); err != nil {
		t.Errorf("expected nil error for valid subscription request, got: %v", err)
	}
}

func TestCreateSubscriptionRequest_Validate_MissingOrgIDReturnsError(t *testing.T) {
	req := license.CreateSubscriptionRequest{PlanID: uuid.New()}
	if err := req.Validate(); err == nil {
		t.Error("expected error when org_id is zero, got nil")
	}
}

func TestCreateSubscriptionRequest_Validate_MissingPlanIDReturnsError(t *testing.T) {
	req := license.CreateSubscriptionRequest{OrgID: uuid.New()}
	if err := req.Validate(); err == nil {
		t.Error("expected error when plan_id is zero, got nil")
	}
}

// ---------------------------------------------------------------------------
// UpsertOverrideRequest.Validate()
// ---------------------------------------------------------------------------

func TestUpsertOverrideRequest_Validate_ValidRequest(t *testing.T) {
	req := license.UpsertOverrideRequest{
		OrgID:      uuid.New(),
		FeatureKey: "document_storage",
	}
	if err := req.Validate(); err != nil {
		t.Errorf("expected nil error for valid override request, got: %v", err)
	}
}

func TestUpsertOverrideRequest_Validate_MissingOrgIDReturnsError(t *testing.T) {
	req := license.UpsertOverrideRequest{FeatureKey: "document_storage"}
	if err := req.Validate(); err == nil {
		t.Error("expected error when org_id is zero, got nil")
	}
}

func TestUpsertOverrideRequest_Validate_MissingFeatureKeyReturnsError(t *testing.T) {
	req := license.UpsertOverrideRequest{OrgID: uuid.New()}
	if err := req.Validate(); err == nil {
		t.Error("expected error when feature_key is empty, got nil")
	}
}
