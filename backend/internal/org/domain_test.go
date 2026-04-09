package org_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/org"
)

// ---------------------------------------------------------------------------
// Organization JSON serialization
// ---------------------------------------------------------------------------

func TestOrganization_JSONSerialization_RequiredFieldsPresent(t *testing.T) {
	id := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	now := time.Now().UTC().Truncate(time.Second)

	o := org.Organization{
		ID:        id,
		Type:      "hoa",
		Name:      "Sunset Ridge HOA",
		Slug:      "sunset-ridge-hoa",
		Path:      "sunset_ridge_hoa",
		Settings:  map[string]any{},
		CreatedAt: now,
		UpdatedAt: now,
	}

	data, err := json.Marshal(o)
	if err != nil {
		t.Fatalf("json.Marshal(Organization) error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	requiredKeys := []string{"id", "type", "name", "slug", "path", "settings", "created_at", "updated_at"}
	for _, key := range requiredKeys {
		if _, ok := result[key]; !ok {
			t.Errorf("expected JSON key %q to be present", key)
		}
	}
}

func TestOrganization_JSONSerialization_OmitsNilOptionalFields(t *testing.T) {
	now := time.Now().UTC()
	o := org.Organization{
		ID:        uuid.New(),
		Type:      "firm",
		Name:      "Apex Management",
		Slug:      "apex-management",
		Path:      "apex_management",
		Settings:  map[string]any{},
		CreatedAt: now,
		UpdatedAt: now,
		// all optional pointer fields left nil
	}

	data, err := json.Marshal(o)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	omittedKeys := []string{
		"parent_id", "address_line1", "address_line2", "city", "state", "zip",
		"phone", "email", "website", "logo_url", "deleted_at",
	}
	for _, key := range omittedKeys {
		if _, ok := result[key]; ok {
			t.Errorf("expected JSON key %q to be omitted when nil", key)
		}
	}
}

func TestOrganization_JSONSerialization_OptionalFieldsIncludedWhenSet(t *testing.T) {
	parentID := uuid.New()
	addr := "123 Main St"
	city := "Springfield"
	state := "IL"
	zip := "62701"
	phone := "+12175551234"
	email := "info@apex.com"
	website := "https://apex.com"
	logo := "https://cdn.apex.com/logo.png"
	now := time.Now().UTC()

	o := org.Organization{
		ID:           uuid.New(),
		ParentID:     &parentID,
		Type:         "firm",
		Name:         "Apex Management",
		Slug:         "apex-management",
		Path:         "apex_management",
		AddressLine1: &addr,
		City:         &city,
		State:        &state,
		Zip:          &zip,
		Phone:        &phone,
		Email:        &email,
		Website:      &website,
		LogoURL:      &logo,
		Settings:     map[string]any{"key": "value"},
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	data, err := json.Marshal(o)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	presentKeys := []string{"parent_id", "address_line1", "city", "state", "zip", "phone", "email", "website", "logo_url"}
	for _, key := range presentKeys {
		if _, ok := result[key]; !ok {
			t.Errorf("expected JSON key %q to be present when set", key)
		}
	}

	if result["name"] != "Apex Management" {
		t.Errorf("name: got %v, want %v", result["name"], "Apex Management")
	}
}

// ---------------------------------------------------------------------------
// CreateOrgRequest.Validate()
// ---------------------------------------------------------------------------

func TestCreateOrgRequest_Validate_ValidFirm(t *testing.T) {
	req := org.CreateOrgRequest{
		Type: "firm",
		Name: "Apex Management",
	}
	if err := req.Validate(); err != nil {
		t.Errorf("expected nil error for valid firm request, got: %v", err)
	}
}

func TestCreateOrgRequest_Validate_ValidHOA(t *testing.T) {
	req := org.CreateOrgRequest{
		Type: "hoa",
		Name: "Sunset Ridge HOA",
	}
	if err := req.Validate(); err != nil {
		t.Errorf("expected nil error for valid hoa request, got: %v", err)
	}
}

func TestCreateOrgRequest_Validate_MissingNameReturnsError(t *testing.T) {
	req := org.CreateOrgRequest{
		Type: "firm",
	}
	err := req.Validate()
	if err == nil {
		t.Error("expected error when name is missing, got nil")
	}
}

func TestCreateOrgRequest_Validate_MissingTypeReturnsError(t *testing.T) {
	req := org.CreateOrgRequest{
		Name: "Apex Management",
	}
	err := req.Validate()
	if err == nil {
		t.Error("expected error when type is missing, got nil")
	}
}

func TestCreateOrgRequest_Validate_InvalidTypeReturnsError(t *testing.T) {
	req := org.CreateOrgRequest{
		Type: "invalid",
		Name: "Apex Management",
	}
	err := req.Validate()
	if err == nil {
		t.Error("expected error for invalid type, got nil")
	}
}

func TestCreateOrgRequest_Validate_ErrorMessageNonEmpty(t *testing.T) {
	req := org.CreateOrgRequest{}
	err := req.Validate()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() == "" {
		t.Error("expected non-empty error message")
	}
}

// ---------------------------------------------------------------------------
// UpdateOrgRequest.Validate()
// ---------------------------------------------------------------------------

func TestUpdateOrgRequest_Validate_AllNilReturnsError(t *testing.T) {
	req := org.UpdateOrgRequest{}
	err := req.Validate()
	if err == nil {
		t.Error("expected error when all fields are nil, got nil")
	}
}

func TestUpdateOrgRequest_Validate_NameSetReturnsNil(t *testing.T) {
	name := "New Name"
	req := org.UpdateOrgRequest{Name: &name}
	if err := req.Validate(); err != nil {
		t.Errorf("expected nil error when Name is set, got: %v", err)
	}
}

func TestUpdateOrgRequest_Validate_PhoneSetReturnsNil(t *testing.T) {
	phone := "+12175551234"
	req := org.UpdateOrgRequest{Phone: &phone}
	if err := req.Validate(); err != nil {
		t.Errorf("expected nil error when Phone is set, got: %v", err)
	}
}

func TestUpdateOrgRequest_Validate_EmailSetReturnsNil(t *testing.T) {
	email := "new@example.com"
	req := org.UpdateOrgRequest{Email: &email}
	if err := req.Validate(); err != nil {
		t.Errorf("expected nil error when Email is set, got: %v", err)
	}
}

func TestUpdateOrgRequest_Validate_WebsiteSetReturnsNil(t *testing.T) {
	website := "https://example.com"
	req := org.UpdateOrgRequest{Website: &website}
	if err := req.Validate(); err != nil {
		t.Errorf("expected nil error when Website is set, got: %v", err)
	}
}

func TestUpdateOrgRequest_Validate_SettingsSetReturnsNil(t *testing.T) {
	req := org.UpdateOrgRequest{Settings: map[string]any{"theme": "dark"}}
	if err := req.Validate(); err != nil {
		t.Errorf("expected nil error when Settings is set, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// CreateMembershipRequest.Validate()
// ---------------------------------------------------------------------------

func TestCreateMembershipRequest_Validate_ValidRequest(t *testing.T) {
	req := org.CreateMembershipRequest{
		UserID: uuid.New(),
		RoleID: uuid.New(),
	}
	if err := req.Validate(); err != nil {
		t.Errorf("expected nil error for valid request, got: %v", err)
	}
}

func TestCreateMembershipRequest_Validate_ZeroUserIDReturnsError(t *testing.T) {
	req := org.CreateMembershipRequest{
		UserID: uuid.UUID{}, // zero value
		RoleID: uuid.New(),
	}
	err := req.Validate()
	if err == nil {
		t.Error("expected error when user_id is zero, got nil")
	}
}

func TestCreateMembershipRequest_Validate_ZeroRoleIDReturnsError(t *testing.T) {
	req := org.CreateMembershipRequest{
		UserID: uuid.New(),
		RoleID: uuid.UUID{}, // zero value
	}
	err := req.Validate()
	if err == nil {
		t.Error("expected error when role_id is zero, got nil")
	}
}

func TestCreateMembershipRequest_Validate_ErrorMessageNonEmpty(t *testing.T) {
	req := org.CreateMembershipRequest{}
	err := req.Validate()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() == "" {
		t.Error("expected non-empty error message")
	}
}

// ---------------------------------------------------------------------------
// CreateUnitRequest.Validate()
// ---------------------------------------------------------------------------

func TestCreateUnitRequest_Validate_ValidRequest(t *testing.T) {
	req := org.CreateUnitRequest{
		Label: "Unit 101",
	}
	if err := req.Validate(); err != nil {
		t.Errorf("expected nil error for valid request, got: %v", err)
	}
}

func TestCreateUnitRequest_Validate_MissingLabelReturnsError(t *testing.T) {
	req := org.CreateUnitRequest{}
	err := req.Validate()
	if err == nil {
		t.Error("expected error when label is missing, got nil")
	}
}

func TestCreateUnitRequest_Validate_EmptyLabelReturnsError(t *testing.T) {
	req := org.CreateUnitRequest{Label: ""}
	err := req.Validate()
	if err == nil {
		t.Error("expected error when label is empty string, got nil")
	}
}

func TestCreateUnitRequest_Validate_WithOptionalFields(t *testing.T) {
	unitType := "condo"
	weight := 1.5
	req := org.CreateUnitRequest{
		Label:        "Unit 202",
		UnitType:     &unitType,
		VotingWeight: &weight,
		Metadata:     map[string]any{"floor": 2},
	}
	if err := req.Validate(); err != nil {
		t.Errorf("expected nil error with optional fields set, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// ConnectManagementRequest.Validate()
// ---------------------------------------------------------------------------

func TestConnectManagementRequest_Validate_ValidRequest(t *testing.T) {
	req := org.ConnectManagementRequest{
		FirmOrgID: uuid.New(),
	}
	if err := req.Validate(); err != nil {
		t.Errorf("expected nil error for valid request, got: %v", err)
	}
}

func TestConnectManagementRequest_Validate_ZeroFirmOrgIDReturnsError(t *testing.T) {
	req := org.ConnectManagementRequest{
		FirmOrgID: uuid.UUID{}, // zero value
	}
	err := req.Validate()
	if err == nil {
		t.Error("expected error when firm_org_id is zero, got nil")
	}
}
