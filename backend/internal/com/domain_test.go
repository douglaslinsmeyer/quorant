package com_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/com"
)

// ---------------------------------------------------------------------------
// Announcement JSON serialization
// ---------------------------------------------------------------------------

func TestAnnouncement_JSONSerialization_RequiredFieldsPresent(t *testing.T) {
	id := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	orgID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	authorID := uuid.MustParse("00000000-0000-0000-0000-000000000003")
	now := time.Now().UTC().Truncate(time.Second)

	a := com.Announcement{
		ID:            id,
		OrgID:         orgID,
		AuthorID:      authorID,
		Title:         "Pool Closure Notice",
		Body:          "The pool will be closed for maintenance.",
		IsPinned:      false,
		AudienceRoles: []string{},
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	data, err := json.Marshal(a)
	if err != nil {
		t.Fatalf("json.Marshal(Announcement) error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	requiredKeys := []string{"id", "org_id", "author_id", "title", "body", "is_pinned", "audience_roles", "created_at", "updated_at"}
	for _, key := range requiredKeys {
		if _, ok := result[key]; !ok {
			t.Errorf("expected JSON key %q to be present", key)
		}
	}
}

func TestAnnouncement_JSONSerialization_OmitsNilOptionalFields(t *testing.T) {
	now := time.Now().UTC()
	a := com.Announcement{
		ID:            uuid.New(),
		OrgID:         uuid.New(),
		AuthorID:      uuid.New(),
		Title:         "Test",
		Body:          "Body text",
		IsPinned:      false,
		AudienceRoles: []string{},
		CreatedAt:     now,
		UpdatedAt:     now,
		// optional pointer fields left nil
	}

	data, err := json.Marshal(a)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	omittedKeys := []string{"scheduled_for", "published_at", "deleted_at"}
	for _, key := range omittedKeys {
		if _, ok := result[key]; ok {
			t.Errorf("expected JSON key %q to be omitted when nil", key)
		}
	}
}

func TestAnnouncement_JSONSerialization_OptionalFieldsIncludedWhenSet(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	scheduledFor := now.Add(24 * time.Hour)
	publishedAt := now

	a := com.Announcement{
		ID:            uuid.New(),
		OrgID:         uuid.New(),
		AuthorID:      uuid.New(),
		Title:         "Board Meeting",
		Body:          "Annual board meeting next week.",
		IsPinned:      true,
		AudienceRoles: []string{"board", "homeowner"},
		ScheduledFor:  &scheduledFor,
		PublishedAt:   &publishedAt,
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

	if result["is_pinned"] != true {
		t.Errorf("is_pinned: got %v, want true", result["is_pinned"])
	}

	roles, ok := result["audience_roles"].([]interface{})
	if !ok {
		t.Fatalf("audience_roles: expected []interface{}, got %T", result["audience_roles"])
	}
	if len(roles) != 2 {
		t.Errorf("audience_roles: got %d items, want 2", len(roles))
	}

	presentKeys := []string{"scheduled_for", "published_at"}
	for _, key := range presentKeys {
		if _, ok := result[key]; !ok {
			t.Errorf("expected JSON key %q to be present when set", key)
		}
	}
}

// ---------------------------------------------------------------------------
// CalendarEvent JSON serialization
// ---------------------------------------------------------------------------

func TestCalendarEvent_JSONSerialization_RequiredFieldsPresent(t *testing.T) {
	id := uuid.MustParse("00000000-0000-0000-0000-000000000010")
	orgID := uuid.MustParse("00000000-0000-0000-0000-000000000011")
	createdBy := uuid.MustParse("00000000-0000-0000-0000-000000000012")
	now := time.Now().UTC().Truncate(time.Second)
	startsAt := now.Add(72 * time.Hour)

	e := com.CalendarEvent{
		ID:            id,
		OrgID:         orgID,
		Title:         "Annual HOA Meeting",
		EventType:     "board_meeting",
		IsVirtual:     false,
		StartsAt:      startsAt,
		IsAllDay:      false,
		AudienceRoles: []string{},
		RSVPEnabled:   false,
		CreatedBy:     createdBy,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	data, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("json.Marshal(CalendarEvent) error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	requiredKeys := []string{
		"id", "org_id", "title", "event_type", "is_virtual",
		"starts_at", "is_all_day", "audience_roles", "rsvp_enabled",
		"created_by", "created_at", "updated_at",
	}
	for _, key := range requiredKeys {
		if _, ok := result[key]; !ok {
			t.Errorf("expected JSON key %q to be present", key)
		}
	}
}

func TestCalendarEvent_JSONSerialization_OmitsNilOptionalFields(t *testing.T) {
	now := time.Now().UTC()
	e := com.CalendarEvent{
		ID:            uuid.New(),
		OrgID:         uuid.New(),
		Title:         "Community Event",
		EventType:     "community",
		StartsAt:      now.Add(24 * time.Hour),
		AudienceRoles: []string{},
		CreatedBy:     uuid.New(),
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	data, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	omittedKeys := []string{"description", "location", "virtual_link", "ends_at", "recurrence_rule", "rsvp_limit", "deleted_at"}
	for _, key := range omittedKeys {
		if _, ok := result[key]; ok {
			t.Errorf("expected JSON key %q to be omitted when nil", key)
		}
	}
}

func TestCalendarEvent_JSONSerialization_OptionalFieldsIncludedWhenSet(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	desc := "Year-end review"
	location := "Clubhouse"
	virtualLink := "https://zoom.us/meeting/123"
	endsAt := now.Add(3 * time.Hour)
	recRule := "FREQ=MONTHLY;COUNT=12"
	rsvpLimit := 50

	e := com.CalendarEvent{
		ID:             uuid.New(),
		OrgID:          uuid.New(),
		Title:          "Monthly Meeting",
		Description:    &desc,
		EventType:      "board_meeting",
		Location:       &location,
		IsVirtual:      true,
		VirtualLink:    &virtualLink,
		StartsAt:       now.Add(24 * time.Hour),
		EndsAt:         &endsAt,
		RecurrenceRule: &recRule,
		AudienceRoles:  []string{"board"},
		RSVPEnabled:    true,
		RSVPLimit:      &rsvpLimit,
		CreatedBy:      uuid.New(),
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	data, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	presentKeys := []string{"description", "location", "virtual_link", "ends_at", "recurrence_rule", "rsvp_limit"}
	for _, key := range presentKeys {
		if _, ok := result[key]; !ok {
			t.Errorf("expected JSON key %q to be present when set", key)
		}
	}

	if result["is_virtual"] != true {
		t.Errorf("is_virtual: got %v, want true", result["is_virtual"])
	}
}

// ---------------------------------------------------------------------------
// CommunicationLog JSON serialization
// ---------------------------------------------------------------------------

func TestCommunicationLog_JSONSerialization_RequiredFieldsPresent(t *testing.T) {
	id := uuid.MustParse("00000000-0000-0000-0000-000000000020")
	orgID := uuid.MustParse("00000000-0000-0000-0000-000000000021")
	now := time.Now().UTC().Truncate(time.Second)

	log := com.CommunicationLog{
		ID:            id,
		OrgID:         orgID,
		Direction:     "outbound",
		Channel:       "email",
		Status:        "sent",
		Source:        "manual",
		AttachmentIDs: []uuid.UUID{},
		Metadata:      map[string]any{},
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	data, err := json.Marshal(log)
	if err != nil {
		t.Fatalf("json.Marshal(CommunicationLog) error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	requiredKeys := []string{
		"id", "org_id", "direction", "channel", "status",
		"source", "attachment_ids", "metadata", "created_at", "updated_at",
	}
	for _, key := range requiredKeys {
		if _, ok := result[key]; !ok {
			t.Errorf("expected JSON key %q to be present", key)
		}
	}
}

func TestCommunicationLog_JSONSerialization_OmitsNilOptionalFields(t *testing.T) {
	now := time.Now().UTC()
	log := com.CommunicationLog{
		ID:            uuid.New(),
		OrgID:         uuid.New(),
		Direction:     "outbound",
		Channel:       "sms",
		Status:        "delivered",
		Source:        "system",
		AttachmentIDs: []uuid.UUID{},
		Metadata:      map[string]any{},
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	data, err := json.Marshal(log)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	omittedKeys := []string{
		"contact_user_id", "contact_name", "contact_info", "initiated_by",
		"subject", "body", "template_id", "unit_id", "resource_type", "resource_id",
		"sent_at", "delivered_at", "opened_at", "bounced_at", "bounce_reason",
		"duration_minutes", "provider_ref",
	}
	for _, key := range omittedKeys {
		if _, ok := result[key]; ok {
			t.Errorf("expected JSON key %q to be omitted when nil", key)
		}
	}
}

func TestCommunicationLog_JSONSerialization_FullRecordSerializes(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	contactUserID := uuid.New()
	initiatedBy := uuid.New()
	templateID := uuid.New()
	unitID := uuid.New()
	resourceID := uuid.New()
	contactName := "Jane Doe"
	contactInfo := "jane@example.com"
	subject := "Payment Due Reminder"
	body := "Your payment is due."
	resourceType := "assessment"
	bounceReason := "mailbox full"
	providerRef := "sg-msg-001"
	duration := 5

	log := com.CommunicationLog{
		ID:              uuid.New(),
		OrgID:           uuid.New(),
		Direction:       "outbound",
		Channel:         "email",
		ContactUserID:   &contactUserID,
		ContactName:     &contactName,
		ContactInfo:     &contactInfo,
		InitiatedBy:     &initiatedBy,
		Subject:         &subject,
		Body:            &body,
		TemplateID:      &templateID,
		AttachmentIDs:   []uuid.UUID{uuid.New()},
		UnitID:          &unitID,
		ResourceType:    &resourceType,
		ResourceID:      &resourceID,
		Status:          "bounced",
		SentAt:          &now,
		DeliveredAt:     nil,
		OpenedAt:        nil,
		BouncedAt:       &now,
		BounceReason:    &bounceReason,
		DurationMinutes: &duration,
		Source:          "automated",
		ProviderRef:     &providerRef,
		Metadata:        map[string]any{"campaign": "monthly-billing"},
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	data, err := json.Marshal(log)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	presentKeys := []string{
		"contact_user_id", "contact_name", "contact_info", "initiated_by",
		"subject", "body", "template_id", "unit_id", "resource_type", "resource_id",
		"sent_at", "bounced_at", "bounce_reason", "duration_minutes", "provider_ref",
	}
	for _, key := range presentKeys {
		if _, ok := result[key]; !ok {
			t.Errorf("expected JSON key %q to be present when set", key)
		}
	}

	if result["direction"] != "outbound" {
		t.Errorf("direction: got %v, want outbound", result["direction"])
	}
	if result["status"] != "bounced" {
		t.Errorf("status: got %v, want bounced", result["status"])
	}
}

// ---------------------------------------------------------------------------
// CreateAnnouncementRequest.Validate()
// ---------------------------------------------------------------------------

func TestCreateAnnouncementRequest_Validate_ValidRequest(t *testing.T) {
	req := com.CreateAnnouncementRequest{
		Title: "Pool Closure",
		Body:  "The pool will be closed.",
	}
	if err := req.Validate(); err != nil {
		t.Errorf("expected nil error for valid request, got: %v", err)
	}
}

func TestCreateAnnouncementRequest_Validate_MissingTitleReturnsError(t *testing.T) {
	req := com.CreateAnnouncementRequest{
		Body: "No title here.",
	}
	if err := req.Validate(); err == nil {
		t.Error("expected error when title is missing, got nil")
	}
}

func TestCreateAnnouncementRequest_Validate_MissingBodyReturnsError(t *testing.T) {
	req := com.CreateAnnouncementRequest{
		Title: "Has title",
	}
	if err := req.Validate(); err == nil {
		t.Error("expected error when body is missing, got nil")
	}
}

func TestCreateAnnouncementRequest_Validate_ErrorIsValidationError(t *testing.T) {
	req := com.CreateAnnouncementRequest{}
	err := req.Validate()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() == "" {
		t.Error("expected non-empty error message")
	}
}

func TestCreateAnnouncementRequest_Validate_WithOptionalFields(t *testing.T) {
	future := time.Now().Add(24 * time.Hour)
	req := com.CreateAnnouncementRequest{
		Title:         "Pinned Notice",
		Body:          "Important notice content.",
		IsPinned:      true,
		AudienceRoles: []string{"board", "homeowner"},
		ScheduledFor:  &future,
	}
	if err := req.Validate(); err != nil {
		t.Errorf("expected nil error with optional fields set, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// CreateThreadRequest.Validate()
// ---------------------------------------------------------------------------

func TestCreateThreadRequest_Validate_ValidRequest(t *testing.T) {
	req := com.CreateThreadRequest{
		Subject: "Noise complaint on Oak Street",
	}
	if err := req.Validate(); err != nil {
		t.Errorf("expected nil error for valid request, got: %v", err)
	}
}

func TestCreateThreadRequest_Validate_MissingSubjectReturnsError(t *testing.T) {
	req := com.CreateThreadRequest{}
	if err := req.Validate(); err == nil {
		t.Error("expected error when subject is missing, got nil")
	}
}

func TestCreateThreadRequest_Validate_WithThreadType(t *testing.T) {
	req := com.CreateThreadRequest{
		Subject:    "Maintenance Request",
		ThreadType: "maintenance",
	}
	if err := req.Validate(); err != nil {
		t.Errorf("expected nil error with thread_type set, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// SendMessageRequest.Validate()
// ---------------------------------------------------------------------------

func TestSendMessageRequest_Validate_ValidRequest(t *testing.T) {
	req := com.SendMessageRequest{
		Body: "Hello everyone!",
	}
	if err := req.Validate(); err != nil {
		t.Errorf("expected nil error for valid request, got: %v", err)
	}
}

func TestSendMessageRequest_Validate_MissingBodyReturnsError(t *testing.T) {
	req := com.SendMessageRequest{}
	if err := req.Validate(); err == nil {
		t.Error("expected error when body is missing, got nil")
	}
}

func TestSendMessageRequest_Validate_EmptyBodyReturnsError(t *testing.T) {
	req := com.SendMessageRequest{Body: ""}
	if err := req.Validate(); err == nil {
		t.Error("expected error when body is empty string, got nil")
	}
}

// ---------------------------------------------------------------------------
// CreateCalendarEventRequest.Validate()
// ---------------------------------------------------------------------------

func TestCreateCalendarEventRequest_Validate_ValidRequest(t *testing.T) {
	req := com.CreateCalendarEventRequest{
		Title:     "Annual Meeting",
		EventType: "board_meeting",
		StartsAt:  time.Now().Add(72 * time.Hour),
	}
	if err := req.Validate(); err != nil {
		t.Errorf("expected nil error for valid request, got: %v", err)
	}
}

func TestCreateCalendarEventRequest_Validate_MissingTitleReturnsError(t *testing.T) {
	req := com.CreateCalendarEventRequest{
		EventType: "board_meeting",
		StartsAt:  time.Now().Add(72 * time.Hour),
	}
	if err := req.Validate(); err == nil {
		t.Error("expected error when title is missing, got nil")
	}
}

func TestCreateCalendarEventRequest_Validate_MissingEventTypeReturnsError(t *testing.T) {
	req := com.CreateCalendarEventRequest{
		Title:    "Annual Meeting",
		StartsAt: time.Now().Add(72 * time.Hour),
	}
	if err := req.Validate(); err == nil {
		t.Error("expected error when event_type is missing, got nil")
	}
}

func TestCreateCalendarEventRequest_Validate_ZeroStartsAtReturnsError(t *testing.T) {
	req := com.CreateCalendarEventRequest{
		Title:     "Annual Meeting",
		EventType: "board_meeting",
		// StartsAt left as zero value
	}
	if err := req.Validate(); err == nil {
		t.Error("expected error when starts_at is zero, got nil")
	}
}

func TestCreateCalendarEventRequest_Validate_WithOptionalFields(t *testing.T) {
	desc := "Year-end review"
	location := "Clubhouse"
	endsAt := time.Now().Add(75 * time.Hour)
	req := com.CreateCalendarEventRequest{
		Title:       "Annual Meeting",
		EventType:   "board_meeting",
		StartsAt:    time.Now().Add(72 * time.Hour),
		Description: &desc,
		Location:    &location,
		EndsAt:      &endsAt,
		IsAllDay:    false,
		RSVPEnabled: true,
	}
	if err := req.Validate(); err != nil {
		t.Errorf("expected nil error with optional fields set, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// RSVPRequest.Validate()
// ---------------------------------------------------------------------------

func TestRSVPRequest_Validate_AttendingIsValid(t *testing.T) {
	req := com.RSVPRequest{Status: "attending"}
	if err := req.Validate(); err != nil {
		t.Errorf("expected nil for attending, got: %v", err)
	}
}

func TestRSVPRequest_Validate_MaybeIsValid(t *testing.T) {
	req := com.RSVPRequest{Status: "maybe"}
	if err := req.Validate(); err != nil {
		t.Errorf("expected nil for maybe, got: %v", err)
	}
}

func TestRSVPRequest_Validate_DeclinedIsValid(t *testing.T) {
	req := com.RSVPRequest{Status: "declined"}
	if err := req.Validate(); err != nil {
		t.Errorf("expected nil for declined, got: %v", err)
	}
}

func TestRSVPRequest_Validate_MissingStatusReturnsError(t *testing.T) {
	req := com.RSVPRequest{}
	if err := req.Validate(); err == nil {
		t.Error("expected error when status is missing, got nil")
	}
}

func TestRSVPRequest_Validate_InvalidStatusReturnsError(t *testing.T) {
	req := com.RSVPRequest{Status: "yes_please"}
	if err := req.Validate(); err == nil {
		t.Error("expected error for invalid status, got nil")
	}
}

// ---------------------------------------------------------------------------
// CreateTemplateRequest.Validate()
// ---------------------------------------------------------------------------

func TestCreateTemplateRequest_Validate_ValidRequest(t *testing.T) {
	req := com.CreateTemplateRequest{
		TemplateKey: "welcome_email",
		Channel:     "email",
		Body:        "Welcome to {{.OrgName}}!",
	}
	if err := req.Validate(); err != nil {
		t.Errorf("expected nil error for valid request, got: %v", err)
	}
}

func TestCreateTemplateRequest_Validate_MissingTemplateKeyReturnsError(t *testing.T) {
	req := com.CreateTemplateRequest{
		Channel: "email",
		Body:    "Welcome!",
	}
	if err := req.Validate(); err == nil {
		t.Error("expected error when template_key is missing, got nil")
	}
}

func TestCreateTemplateRequest_Validate_MissingChannelReturnsError(t *testing.T) {
	req := com.CreateTemplateRequest{
		TemplateKey: "welcome_email",
		Body:        "Welcome!",
	}
	if err := req.Validate(); err == nil {
		t.Error("expected error when channel is missing, got nil")
	}
}

func TestCreateTemplateRequest_Validate_MissingBodyReturnsError(t *testing.T) {
	req := com.CreateTemplateRequest{
		TemplateKey: "welcome_email",
		Channel:     "email",
	}
	if err := req.Validate(); err == nil {
		t.Error("expected error when body is missing, got nil")
	}
}

func TestCreateTemplateRequest_Validate_WithSubject(t *testing.T) {
	subject := "Welcome to the HOA"
	req := com.CreateTemplateRequest{
		TemplateKey: "welcome_email",
		Channel:     "email",
		Body:        "Welcome to {{.OrgName}}!",
		Subject:     &subject,
	}
	if err := req.Validate(); err != nil {
		t.Errorf("expected nil error with subject set, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// UpdateDirectoryPreferenceRequest.Validate()
// ---------------------------------------------------------------------------

func TestUpdateDirectoryPreferenceRequest_Validate_AllNilReturnsError(t *testing.T) {
	req := com.UpdateDirectoryPreferenceRequest{}
	if err := req.Validate(); err == nil {
		t.Error("expected error when all fields are nil, got nil")
	}
}

func TestUpdateDirectoryPreferenceRequest_Validate_OptInSetReturnsNil(t *testing.T) {
	v := true
	req := com.UpdateDirectoryPreferenceRequest{OptIn: &v}
	if err := req.Validate(); err != nil {
		t.Errorf("expected nil error when opt_in is set, got: %v", err)
	}
}

func TestUpdateDirectoryPreferenceRequest_Validate_ShowEmailSetReturnsNil(t *testing.T) {
	v := false
	req := com.UpdateDirectoryPreferenceRequest{ShowEmail: &v}
	if err := req.Validate(); err != nil {
		t.Errorf("expected nil error when show_email is set, got: %v", err)
	}
}

func TestUpdateDirectoryPreferenceRequest_Validate_ShowPhoneSetReturnsNil(t *testing.T) {
	v := true
	req := com.UpdateDirectoryPreferenceRequest{ShowPhone: &v}
	if err := req.Validate(); err != nil {
		t.Errorf("expected nil error when show_phone is set, got: %v", err)
	}
}

func TestUpdateDirectoryPreferenceRequest_Validate_ShowUnitSetReturnsNil(t *testing.T) {
	v := false
	req := com.UpdateDirectoryPreferenceRequest{ShowUnit: &v}
	if err := req.Validate(); err != nil {
		t.Errorf("expected nil error when show_unit is set, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// LogCommunicationRequest.Validate()
// ---------------------------------------------------------------------------

func TestLogCommunicationRequest_Validate_ValidWithContactName(t *testing.T) {
	contactName := "John Smith"
	req := com.LogCommunicationRequest{
		Direction:   "outbound",
		Channel:     "email",
		ContactName: &contactName,
	}
	if err := req.Validate(); err != nil {
		t.Errorf("expected nil error for valid request with contact_name, got: %v", err)
	}
}

func TestLogCommunicationRequest_Validate_ValidWithContactUserID(t *testing.T) {
	contactUserID := uuid.New()
	req := com.LogCommunicationRequest{
		Direction:     "inbound",
		Channel:       "phone",
		ContactUserID: &contactUserID,
	}
	if err := req.Validate(); err != nil {
		t.Errorf("expected nil error for valid request with contact_user_id, got: %v", err)
	}
}

func TestLogCommunicationRequest_Validate_MissingDirectionReturnsError(t *testing.T) {
	contactName := "John Smith"
	req := com.LogCommunicationRequest{
		Channel:     "email",
		ContactName: &contactName,
	}
	if err := req.Validate(); err == nil {
		t.Error("expected error when direction is missing, got nil")
	}
}

func TestLogCommunicationRequest_Validate_MissingChannelReturnsError(t *testing.T) {
	contactName := "John Smith"
	req := com.LogCommunicationRequest{
		Direction:   "outbound",
		ContactName: &contactName,
	}
	if err := req.Validate(); err == nil {
		t.Error("expected error when channel is missing, got nil")
	}
}

func TestLogCommunicationRequest_Validate_MissingContactReturnsError(t *testing.T) {
	req := com.LogCommunicationRequest{
		Direction: "outbound",
		Channel:   "email",
		// no ContactName or ContactUserID
	}
	if err := req.Validate(); err == nil {
		t.Error("expected error when neither contact_name nor contact_user_id is set, got nil")
	}
}

func TestLogCommunicationRequest_Validate_WithSubjectAndBody(t *testing.T) {
	contactName := "Resident A"
	subject := "Welcome"
	body := "Welcome to the community!"
	req := com.LogCommunicationRequest{
		Direction:   "outbound",
		Channel:     "email",
		ContactName: &contactName,
		Subject:     &subject,
		Body:        &body,
	}
	if err := req.Validate(); err != nil {
		t.Errorf("expected nil error with subject and body, got: %v", err)
	}
}
