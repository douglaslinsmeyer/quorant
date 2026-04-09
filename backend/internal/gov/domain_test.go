package gov_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/gov"
)

// ---------------------------------------------------------------------------
// Violation JSON serialization
// ---------------------------------------------------------------------------

func TestViolation_JSONSerialization_RequiredFieldsPresent(t *testing.T) {
	id := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	orgID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	unitID := uuid.MustParse("00000000-0000-0000-0000-000000000003")
	reportedBy := uuid.MustParse("00000000-0000-0000-0000-000000000004")
	now := time.Now().UTC().Truncate(time.Second)

	v := gov.Violation{
		ID:              id,
		OrgID:           orgID,
		UnitID:          unitID,
		ReportedBy:      reportedBy,
		Title:           "Parking violation",
		Description:     "Vehicle parked in fire lane",
		Category:        "parking",
		Status:          "open",
		Severity:        1,
		FineTotalCents:  0,
		EvidenceDocIDs:  []uuid.UUID{},
		Metadata:        map[string]any{},
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal(Violation) error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	requiredKeys := []string{
		"id", "org_id", "unit_id", "reported_by", "title", "description",
		"category", "status", "severity", "fine_total_cents", "evidence_doc_ids",
		"metadata", "created_at", "updated_at",
	}
	for _, key := range requiredKeys {
		if _, ok := result[key]; !ok {
			t.Errorf("expected JSON key %q to be present", key)
		}
	}
}

func TestViolation_JSONSerialization_OmitsNilOptionalFields(t *testing.T) {
	now := time.Now().UTC()

	v := gov.Violation{
		ID:             uuid.New(),
		OrgID:          uuid.New(),
		UnitID:         uuid.New(),
		ReportedBy:     uuid.New(),
		Title:          "Noise complaint",
		Description:    "Loud music after hours",
		Category:       "noise",
		Status:         "open",
		Severity:       1,
		EvidenceDocIDs: []uuid.UUID{},
		Metadata:       map[string]any{},
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	omittedKeys := []string{
		"assigned_to", "due_date", "governing_doc_id", "governing_section",
		"offense_number", "cure_deadline", "cure_verified_at", "cure_verified_by",
		"resolved_at", "deleted_at",
	}
	for _, key := range omittedKeys {
		if _, ok := result[key]; ok {
			t.Errorf("expected JSON key %q to be omitted when nil", key)
		}
	}
}

func TestViolation_JSONSerialization_OptionalFieldsIncludedWhenSet(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	assignedTo := uuid.New()
	govDocID := uuid.New()
	offenseNum := int16(2)
	resolvedAt := now.Add(24 * time.Hour)

	v := gov.Violation{
		ID:               uuid.New(),
		OrgID:            uuid.New(),
		UnitID:           uuid.New(),
		ReportedBy:       uuid.New(),
		AssignedTo:       &assignedTo,
		Title:            "Landscaping violation",
		Description:      "Uncut lawn",
		Category:         "landscaping",
		Status:           "resolved",
		Severity:         2,
		GoverningDocID:   &govDocID,
		OffenseNumber:    &offenseNum,
		ResolvedAt:       &resolvedAt,
		EvidenceDocIDs:   []uuid.UUID{uuid.New()},
		Metadata:         map[string]any{"note": "first warning"},
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	presentKeys := []string{"assigned_to", "governing_doc_id", "offense_number", "resolved_at"}
	for _, key := range presentKeys {
		if _, ok := result[key]; !ok {
			t.Errorf("expected JSON key %q to be present when set", key)
		}
	}
}

// ---------------------------------------------------------------------------
// Ballot JSON serialization
// ---------------------------------------------------------------------------

func TestBallot_JSONSerialization_RequiredFieldsPresent(t *testing.T) {
	id := uuid.MustParse("00000000-0000-0000-0000-000000000011")
	orgID := uuid.MustParse("00000000-0000-0000-0000-000000000012")
	createdBy := uuid.MustParse("00000000-0000-0000-0000-000000000013")
	now := time.Now().UTC().Truncate(time.Second)
	opens := now.Add(24 * time.Hour)
	closes := now.Add(7 * 24 * time.Hour)

	b := gov.Ballot{
		ID:           id,
		OrgID:        orgID,
		Title:        "Board Member Election",
		Description:  "Annual election for board seats",
		BallotType:   "election",
		Status:       "draft",
		Options:      json.RawMessage(`[]`),
		EligibleRole: "homeowner",
		OpensAt:      opens,
		ClosesAt:     closes,
		VotesCast:    0,
		WeightMethod: "equal",
		CreatedBy:    createdBy,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	data, err := json.Marshal(b)
	if err != nil {
		t.Fatalf("json.Marshal(Ballot) error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	requiredKeys := []string{
		"id", "org_id", "title", "description", "ballot_type", "status",
		"options", "eligible_role", "opens_at", "closes_at", "votes_cast",
		"weight_method", "created_by", "created_at", "updated_at",
	}
	for _, key := range requiredKeys {
		if _, ok := result[key]; !ok {
			t.Errorf("expected JSON key %q to be present", key)
		}
	}
}

func TestBallot_JSONSerialization_OmitsNilOptionalFields(t *testing.T) {
	now := time.Now().UTC()

	b := gov.Ballot{
		ID:           uuid.New(),
		OrgID:        uuid.New(),
		Title:        "Budget Approval",
		Description:  "Vote on annual budget",
		BallotType:   "approval",
		Status:       "draft",
		Options:      json.RawMessage(`[]`),
		EligibleRole: "homeowner",
		OpensAt:      now.Add(time.Hour),
		ClosesAt:     now.Add(48 * time.Hour),
		VotesCast:    0,
		WeightMethod: "equal",
		CreatedBy:    uuid.New(),
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	data, err := json.Marshal(b)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	omittedKeys := []string{"quorum_percent", "pass_percent", "eligible_units", "quorum_met", "results", "deleted_at"}
	for _, key := range omittedKeys {
		if _, ok := result[key]; ok {
			t.Errorf("expected JSON key %q to be omitted when nil", key)
		}
	}
}

// ---------------------------------------------------------------------------
// Meeting JSON serialization
// ---------------------------------------------------------------------------

func TestMeeting_JSONSerialization_RequiredFieldsPresent(t *testing.T) {
	id := uuid.MustParse("00000000-0000-0000-0000-000000000021")
	orgID := uuid.MustParse("00000000-0000-0000-0000-000000000022")
	createdBy := uuid.MustParse("00000000-0000-0000-0000-000000000023")
	now := time.Now().UTC().Truncate(time.Second)
	scheduledAt := now.Add(7 * 24 * time.Hour)

	m := gov.Meeting{
		ID:          id,
		OrgID:       orgID,
		Title:       "Annual General Meeting",
		MeetingType: "annual",
		Status:      "scheduled",
		ScheduledAt: scheduledAt,
		IsVirtual:   false,
		Metadata:    map[string]any{},
		CreatedBy:   createdBy,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	data, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("json.Marshal(Meeting) error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	requiredKeys := []string{
		"id", "org_id", "title", "meeting_type", "status", "scheduled_at",
		"is_virtual", "metadata", "created_by", "created_at", "updated_at",
	}
	for _, key := range requiredKeys {
		if _, ok := result[key]; !ok {
			t.Errorf("expected JSON key %q to be present", key)
		}
	}
}

func TestMeeting_JSONSerialization_OmitsNilOptionalFields(t *testing.T) {
	now := time.Now().UTC()

	m := gov.Meeting{
		ID:          uuid.New(),
		OrgID:       uuid.New(),
		Title:       "Board Meeting",
		MeetingType: "board",
		Status:      "scheduled",
		ScheduledAt: now.Add(48 * time.Hour),
		IsVirtual:   false,
		Metadata:    map[string]any{},
		CreatedBy:   uuid.New(),
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	data, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	omittedKeys := []string{
		"ended_at", "location", "virtual_link", "notice_required_days",
		"notice_sent_at", "quorum_required", "quorum_present", "quorum_met",
		"agenda_doc_id", "minutes_doc_id", "deleted_at",
	}
	for _, key := range omittedKeys {
		if _, ok := result[key]; ok {
			t.Errorf("expected JSON key %q to be omitted when nil", key)
		}
	}
}

// ---------------------------------------------------------------------------
// CreateViolationRequest.Validate()
// ---------------------------------------------------------------------------

func TestCreateViolationRequest_Validate_ValidRequest(t *testing.T) {
	req := gov.CreateViolationRequest{
		UnitID:      uuid.New(),
		Title:       "Parking violation",
		Description: "Vehicle parked in fire lane",
		Category:    "parking",
	}
	if err := req.Validate(); err != nil {
		t.Errorf("expected nil error for valid request, got: %v", err)
	}
}

func TestCreateViolationRequest_Validate_DefaultsSeverityToOne(t *testing.T) {
	req := gov.CreateViolationRequest{
		UnitID:      uuid.New(),
		Title:       "Parking violation",
		Description: "Vehicle parked in fire lane",
		Category:    "parking",
	}
	_ = req.Validate()
	if req.Severity != 1 {
		t.Errorf("expected default severity 1, got %d", req.Severity)
	}
}

func TestCreateViolationRequest_Validate_PreservesExplicitSeverity(t *testing.T) {
	req := gov.CreateViolationRequest{
		UnitID:      uuid.New(),
		Title:       "Repeat offense",
		Description: "Third parking violation",
		Category:    "parking",
		Severity:    3,
	}
	_ = req.Validate()
	if req.Severity != 3 {
		t.Errorf("expected severity 3, got %d", req.Severity)
	}
}

func TestCreateViolationRequest_Validate_MissingUnitIDReturnsError(t *testing.T) {
	req := gov.CreateViolationRequest{
		Title:       "Parking violation",
		Description: "Vehicle parked in fire lane",
		Category:    "parking",
	}
	if err := req.Validate(); err == nil {
		t.Error("expected error when unit_id is zero, got nil")
	}
}

func TestCreateViolationRequest_Validate_MissingTitleReturnsError(t *testing.T) {
	req := gov.CreateViolationRequest{
		UnitID:      uuid.New(),
		Description: "Vehicle parked in fire lane",
		Category:    "parking",
	}
	if err := req.Validate(); err == nil {
		t.Error("expected error when title is missing, got nil")
	}
}

func TestCreateViolationRequest_Validate_MissingDescriptionReturnsError(t *testing.T) {
	req := gov.CreateViolationRequest{
		UnitID:   uuid.New(),
		Title:    "Parking violation",
		Category: "parking",
	}
	if err := req.Validate(); err == nil {
		t.Error("expected error when description is missing, got nil")
	}
}

func TestCreateViolationRequest_Validate_MissingCategoryReturnsError(t *testing.T) {
	req := gov.CreateViolationRequest{
		UnitID:      uuid.New(),
		Title:       "Parking violation",
		Description: "Vehicle parked in fire lane",
	}
	if err := req.Validate(); err == nil {
		t.Error("expected error when category is missing, got nil")
	}
}

// ---------------------------------------------------------------------------
// CreateARBRequestRequest.Validate()
// ---------------------------------------------------------------------------

func TestCreateARBRequestRequest_Validate_ValidRequest(t *testing.T) {
	req := gov.CreateARBRequestRequest{
		UnitID:      uuid.New(),
		Title:       "Fence installation",
		Description: "Request to install 6-foot privacy fence",
		Category:    "exterior_modification",
	}
	if err := req.Validate(); err != nil {
		t.Errorf("expected nil error for valid request, got: %v", err)
	}
}

func TestCreateARBRequestRequest_Validate_MissingUnitIDReturnsError(t *testing.T) {
	req := gov.CreateARBRequestRequest{
		Title:       "Fence installation",
		Description: "Request to install 6-foot privacy fence",
		Category:    "exterior_modification",
	}
	if err := req.Validate(); err == nil {
		t.Error("expected error when unit_id is zero, got nil")
	}
}

func TestCreateARBRequestRequest_Validate_MissingTitleReturnsError(t *testing.T) {
	req := gov.CreateARBRequestRequest{
		UnitID:      uuid.New(),
		Description: "Request to install 6-foot privacy fence",
		Category:    "exterior_modification",
	}
	if err := req.Validate(); err == nil {
		t.Error("expected error when title is missing, got nil")
	}
}

func TestCreateARBRequestRequest_Validate_MissingDescriptionReturnsError(t *testing.T) {
	req := gov.CreateARBRequestRequest{
		UnitID:   uuid.New(),
		Title:    "Fence installation",
		Category: "exterior_modification",
	}
	if err := req.Validate(); err == nil {
		t.Error("expected error when description is missing, got nil")
	}
}

func TestCreateARBRequestRequest_Validate_MissingCategoryReturnsError(t *testing.T) {
	req := gov.CreateARBRequestRequest{
		UnitID:      uuid.New(),
		Title:       "Fence installation",
		Description: "Request to install 6-foot privacy fence",
	}
	if err := req.Validate(); err == nil {
		t.Error("expected error when category is missing, got nil")
	}
}

// ---------------------------------------------------------------------------
// CreateBallotRequest.Validate()
// ---------------------------------------------------------------------------

func TestCreateBallotRequest_Validate_ValidRequest(t *testing.T) {
	now := time.Now()
	req := gov.CreateBallotRequest{
		Title:       "Board Member Election",
		Description: "Annual election for board seats",
		BallotType:  "election",
		OpensAt:     now.Add(24 * time.Hour),
		ClosesAt:    now.Add(7 * 24 * time.Hour),
	}
	if err := req.Validate(); err != nil {
		t.Errorf("expected nil error for valid request, got: %v", err)
	}
}

func TestCreateBallotRequest_Validate_MissingTitleReturnsError(t *testing.T) {
	now := time.Now()
	req := gov.CreateBallotRequest{
		Description: "Annual election for board seats",
		BallotType:  "election",
		OpensAt:     now.Add(24 * time.Hour),
		ClosesAt:    now.Add(7 * 24 * time.Hour),
	}
	if err := req.Validate(); err == nil {
		t.Error("expected error when title is missing, got nil")
	}
}

func TestCreateBallotRequest_Validate_MissingDescriptionReturnsError(t *testing.T) {
	now := time.Now()
	req := gov.CreateBallotRequest{
		Title:      "Board Member Election",
		BallotType: "election",
		OpensAt:    now.Add(24 * time.Hour),
		ClosesAt:   now.Add(7 * 24 * time.Hour),
	}
	if err := req.Validate(); err == nil {
		t.Error("expected error when description is missing, got nil")
	}
}

func TestCreateBallotRequest_Validate_MissingBallotTypeReturnsError(t *testing.T) {
	now := time.Now()
	req := gov.CreateBallotRequest{
		Title:       "Board Member Election",
		Description: "Annual election for board seats",
		OpensAt:     now.Add(24 * time.Hour),
		ClosesAt:    now.Add(7 * 24 * time.Hour),
	}
	if err := req.Validate(); err == nil {
		t.Error("expected error when ballot_type is missing, got nil")
	}
}

func TestCreateBallotRequest_Validate_MissingOpensAtReturnsError(t *testing.T) {
	now := time.Now()
	req := gov.CreateBallotRequest{
		Title:       "Board Member Election",
		Description: "Annual election for board seats",
		BallotType:  "election",
		ClosesAt:    now.Add(7 * 24 * time.Hour),
	}
	if err := req.Validate(); err == nil {
		t.Error("expected error when opens_at is zero, got nil")
	}
}

func TestCreateBallotRequest_Validate_MissingClosesAtReturnsError(t *testing.T) {
	now := time.Now()
	req := gov.CreateBallotRequest{
		Title:       "Board Member Election",
		Description: "Annual election for board seats",
		BallotType:  "election",
		OpensAt:     now.Add(24 * time.Hour),
	}
	if err := req.Validate(); err == nil {
		t.Error("expected error when closes_at is zero, got nil")
	}
}

// ---------------------------------------------------------------------------
// CastBallotVoteRequest.Validate()
// ---------------------------------------------------------------------------

func TestCastBallotVoteRequest_Validate_ValidRequest(t *testing.T) {
	req := gov.CastBallotVoteRequest{
		UnitID:    uuid.New(),
		Selection: json.RawMessage(`{"choice":"yes"}`),
	}
	if err := req.Validate(); err != nil {
		t.Errorf("expected nil error for valid request, got: %v", err)
	}
}

func TestCastBallotVoteRequest_Validate_MissingUnitIDReturnsError(t *testing.T) {
	req := gov.CastBallotVoteRequest{
		Selection: json.RawMessage(`{"choice":"yes"}`),
	}
	if err := req.Validate(); err == nil {
		t.Error("expected error when unit_id is zero, got nil")
	}
}

func TestCastBallotVoteRequest_Validate_MissingSelectionReturnsError(t *testing.T) {
	req := gov.CastBallotVoteRequest{
		UnitID: uuid.New(),
	}
	if err := req.Validate(); err == nil {
		t.Error("expected error when selection is nil, got nil")
	}
}

// ---------------------------------------------------------------------------
// FileProxyRequest.Validate()
// ---------------------------------------------------------------------------

func TestFileProxyRequest_Validate_ValidRequest(t *testing.T) {
	req := gov.FileProxyRequest{
		UnitID:  uuid.New(),
		ProxyID: uuid.New(),
	}
	if err := req.Validate(); err != nil {
		t.Errorf("expected nil error for valid request, got: %v", err)
	}
}

func TestFileProxyRequest_Validate_MissingUnitIDReturnsError(t *testing.T) {
	req := gov.FileProxyRequest{
		ProxyID: uuid.New(),
	}
	if err := req.Validate(); err == nil {
		t.Error("expected error when unit_id is zero, got nil")
	}
}

func TestFileProxyRequest_Validate_MissingProxyIDReturnsError(t *testing.T) {
	req := gov.FileProxyRequest{
		UnitID: uuid.New(),
	}
	if err := req.Validate(); err == nil {
		t.Error("expected error when proxy_id is zero, got nil")
	}
}

// ---------------------------------------------------------------------------
// CreateMeetingRequest.Validate()
// ---------------------------------------------------------------------------

func TestCreateMeetingRequest_Validate_ValidRequest(t *testing.T) {
	req := gov.CreateMeetingRequest{
		Title:       "Annual General Meeting",
		MeetingType: "annual",
		ScheduledAt: time.Now().Add(7 * 24 * time.Hour),
	}
	if err := req.Validate(); err != nil {
		t.Errorf("expected nil error for valid request, got: %v", err)
	}
}

func TestCreateMeetingRequest_Validate_MissingTitleReturnsError(t *testing.T) {
	req := gov.CreateMeetingRequest{
		MeetingType: "annual",
		ScheduledAt: time.Now().Add(7 * 24 * time.Hour),
	}
	if err := req.Validate(); err == nil {
		t.Error("expected error when title is missing, got nil")
	}
}

func TestCreateMeetingRequest_Validate_MissingMeetingTypeReturnsError(t *testing.T) {
	req := gov.CreateMeetingRequest{
		Title:       "Annual General Meeting",
		ScheduledAt: time.Now().Add(7 * 24 * time.Hour),
	}
	if err := req.Validate(); err == nil {
		t.Error("expected error when meeting_type is missing, got nil")
	}
}

func TestCreateMeetingRequest_Validate_MissingScheduledAtReturnsError(t *testing.T) {
	req := gov.CreateMeetingRequest{
		Title:       "Annual General Meeting",
		MeetingType: "annual",
	}
	if err := req.Validate(); err == nil {
		t.Error("expected error when scheduled_at is zero, got nil")
	}
}

// ---------------------------------------------------------------------------
// RecordMotionRequest.Validate()
// ---------------------------------------------------------------------------

func TestRecordMotionRequest_Validate_ValidRequest(t *testing.T) {
	req := gov.RecordMotionRequest{
		Title:   "Approve FY2026 Budget",
		MovedBy: uuid.New(),
	}
	if err := req.Validate(); err != nil {
		t.Errorf("expected nil error for valid request, got: %v", err)
	}
}

func TestRecordMotionRequest_Validate_MissingTitleReturnsError(t *testing.T) {
	req := gov.RecordMotionRequest{
		MovedBy: uuid.New(),
	}
	if err := req.Validate(); err == nil {
		t.Error("expected error when title is missing, got nil")
	}
}

func TestRecordMotionRequest_Validate_MissingMovedByReturnsError(t *testing.T) {
	req := gov.RecordMotionRequest{
		Title: "Approve FY2026 Budget",
	}
	if err := req.Validate(); err == nil {
		t.Error("expected error when moved_by is zero, got nil")
	}
}

// ---------------------------------------------------------------------------
// CreateViolationActionRequest.Validate()
// ---------------------------------------------------------------------------

func TestCreateViolationActionRequest_Validate_ValidRequest(t *testing.T) {
	req := gov.CreateViolationActionRequest{
		ActionType: "notice_sent",
	}
	if err := req.Validate(); err != nil {
		t.Errorf("expected nil error for valid request, got: %v", err)
	}
}

func TestCreateViolationActionRequest_Validate_WithOptionalNotes(t *testing.T) {
	notes := "Certified mail sent"
	req := gov.CreateViolationActionRequest{
		ActionType: "notice_sent",
		Notes:      &notes,
	}
	if err := req.Validate(); err != nil {
		t.Errorf("expected nil error for valid request with notes, got: %v", err)
	}
}

func TestCreateViolationActionRequest_Validate_MissingActionTypeReturnsError(t *testing.T) {
	req := gov.CreateViolationActionRequest{}
	if err := req.Validate(); err == nil {
		t.Error("expected error when action_type is missing, got nil")
	}
}

// ---------------------------------------------------------------------------
// CastARBVoteRequest.Validate()
// ---------------------------------------------------------------------------

func TestCastARBVoteRequest_Validate_ValidApprove(t *testing.T) {
	req := gov.CastARBVoteRequest{Vote: "approve"}
	if err := req.Validate(); err != nil {
		t.Errorf("expected nil error for vote=approve, got: %v", err)
	}
}

func TestCastARBVoteRequest_Validate_ValidDeny(t *testing.T) {
	req := gov.CastARBVoteRequest{Vote: "deny"}
	if err := req.Validate(); err != nil {
		t.Errorf("expected nil error for vote=deny, got: %v", err)
	}
}

func TestCastARBVoteRequest_Validate_ValidAbstain(t *testing.T) {
	req := gov.CastARBVoteRequest{Vote: "abstain"}
	if err := req.Validate(); err != nil {
		t.Errorf("expected nil error for vote=abstain, got: %v", err)
	}
}

func TestCastARBVoteRequest_Validate_ValidConditionalApprove(t *testing.T) {
	req := gov.CastARBVoteRequest{Vote: "conditional_approve"}
	if err := req.Validate(); err != nil {
		t.Errorf("expected nil error for vote=conditional_approve, got: %v", err)
	}
}

func TestCastARBVoteRequest_Validate_MissingVoteReturnsError(t *testing.T) {
	req := gov.CastARBVoteRequest{}
	if err := req.Validate(); err == nil {
		t.Error("expected error when vote is missing, got nil")
	}
}

func TestCastARBVoteRequest_Validate_InvalidVoteReturnsError(t *testing.T) {
	req := gov.CastARBVoteRequest{Vote: "maybe"}
	if err := req.Validate(); err == nil {
		t.Error("expected error for invalid vote value, got nil")
	}
}

func TestCastARBVoteRequest_Validate_AllValidVotes(t *testing.T) {
	for _, vote := range []string{"approve", "deny", "abstain", "conditional_approve"} {
		req := gov.CastARBVoteRequest{Vote: vote}
		if err := req.Validate(); err != nil {
			t.Errorf("expected nil error for vote %q, got: %v", vote, err)
		}
	}
}
