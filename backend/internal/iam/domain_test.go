package iam_test

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/iam"
)

func TestUser_JSONSerialization(t *testing.T) {
	id := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	phone := "+15551234567"
	avatarURL := "https://example.com/avatar.png"
	now := time.Now().UTC().Truncate(time.Second)

	u := iam.User{
		ID:          id,
		IDPUserID:   "idp-abc123",
		Email:       "alice@example.com",
		DisplayName: "Alice",
		Phone:       &phone,
		AvatarURL:   &avatarURL,
		IsActive:    true,
		LastLoginAt: &now,
		CreatedAt:   now,
		UpdatedAt:   now,
		DeletedAt:   nil,
	}

	data, err := json.Marshal(u)
	if err != nil {
		t.Fatalf("json.Marshal(User) error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	requiredKeys := []string{
		"id", "idp_user_id", "email", "display_name",
		"phone", "avatar_url", "is_active", "last_login_at",
		"created_at", "updated_at",
	}
	for _, key := range requiredKeys {
		if _, ok := result[key]; !ok {
			t.Errorf("expected JSON key %q to be present", key)
		}
	}

	// deleted_at should be omitted when nil
	if _, ok := result["deleted_at"]; ok {
		t.Errorf("expected JSON key %q to be omitted when nil", "deleted_at")
	}

	if result["email"] != "alice@example.com" {
		t.Errorf("email: got %v, want %v", result["email"], "alice@example.com")
	}
	if result["display_name"] != "Alice" {
		t.Errorf("display_name: got %v, want %v", result["display_name"], "Alice")
	}
	if result["is_active"] != true {
		t.Errorf("is_active: got %v, want true", result["is_active"])
	}
}

func TestUser_OmitemptyOptionalFields(t *testing.T) {
	u := iam.User{
		ID:          uuid.New(),
		IDPUserID:   "idp-xyz",
		Email:       "bob@example.com",
		DisplayName: "Bob",
		IsActive:    false,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	data, err := json.Marshal(u)
	if err != nil {
		t.Fatalf("json.Marshal(User) error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	omittedKeys := []string{"phone", "avatar_url", "last_login_at", "deleted_at"}
	for _, key := range omittedKeys {
		if _, ok := result[key]; ok {
			t.Errorf("expected JSON key %q to be omitted when nil", key)
		}
	}
}

func TestUserProfile_IncludesMembershipsArray(t *testing.T) {
	userID := uuid.New()
	orgID := uuid.New()
	roleID := uuid.New()
	now := time.Now()

	membership := iam.Membership{
		ID:        uuid.New(),
		UserID:    userID,
		OrgID:     orgID,
		RoleID:    roleID,
		RoleName:  "admin",
		Status:    "active",
		CreatedAt: now,
		UpdatedAt: now,
	}

	profile := iam.UserProfile{
		ID:          userID,
		Email:       "carol@example.com",
		DisplayName: "Carol",
		Memberships: []iam.Membership{membership},
	}

	data, err := json.Marshal(profile)
	if err != nil {
		t.Fatalf("json.Marshal(UserProfile) error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	memberships, ok := result["memberships"]
	if !ok {
		t.Fatal("expected JSON key \"memberships\" to be present")
	}

	arr, ok := memberships.([]interface{})
	if !ok {
		t.Fatalf("expected memberships to be an array, got %T", memberships)
	}

	if len(arr) != 1 {
		t.Errorf("memberships length: got %d, want 1", len(arr))
	}
}

func TestUserProfile_EmptyMembershipsArray(t *testing.T) {
	profile := iam.UserProfile{
		ID:          uuid.New(),
		Email:       "dave@example.com",
		DisplayName: "Dave",
		Memberships: []iam.Membership{},
	}

	data, err := json.Marshal(profile)
	if err != nil {
		t.Fatalf("json.Marshal(UserProfile) error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	memberships, ok := result["memberships"]
	if !ok {
		t.Fatal("expected JSON key \"memberships\" to be present even when empty")
	}

	arr, ok := memberships.([]interface{})
	if !ok {
		t.Fatalf("expected memberships to be an array, got %T", memberships)
	}

	if len(arr) != 0 {
		t.Errorf("memberships length: got %d, want 0", len(arr))
	}
}

func TestUpdateProfileRequest_Validate_AllNilReturnsError(t *testing.T) {
	req := iam.UpdateProfileRequest{}

	err := req.Validate()
	if err == nil {
		t.Error("expected error when all fields are nil, got nil")
	}
}

func TestUpdateProfileRequest_Validate_DisplayNameSetReturnsNil(t *testing.T) {
	name := "New Name"
	req := iam.UpdateProfileRequest{
		DisplayName: &name,
	}

	err := req.Validate()
	if err != nil {
		t.Errorf("expected nil error when DisplayName is set, got: %v", err)
	}
}

func TestUpdateProfileRequest_Validate_PhoneSetReturnsNil(t *testing.T) {
	phone := "+15559876543"
	req := iam.UpdateProfileRequest{
		Phone: &phone,
	}

	err := req.Validate()
	if err != nil {
		t.Errorf("expected nil error when Phone is set, got: %v", err)
	}
}

func TestUpdateProfileRequest_Validate_AvatarURLSetReturnsNil(t *testing.T) {
	url := "https://example.com/new-avatar.png"
	req := iam.UpdateProfileRequest{
		AvatarURL: &url,
	}

	err := req.Validate()
	if err != nil {
		t.Errorf("expected nil error when AvatarURL is set, got: %v", err)
	}
}

func TestUpdateProfileRequest_Validate_ErrorMessage(t *testing.T) {
	req := iam.UpdateProfileRequest{}

	err := req.Validate()
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Confirm it is a real error (not wrapped sentinel for now)
	if errors.Unwrap(err) != nil && !errors.Is(err, errors.New("")) {
		// just check it is non-nil and has a message
	}

	if err.Error() == "" {
		t.Error("expected non-empty error message")
	}
}
