// Package gov provides domain types, request models, and interfaces for
// the Governance module: violations, ARB requests, ballots, meetings, and hearings.
package gov

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Violation represents an HOA rule violation against a unit.
type Violation struct {
	ID               uuid.UUID      `json:"id"`
	OrgID            uuid.UUID      `json:"org_id"`
	UnitID           uuid.UUID      `json:"unit_id"`
	ReportedBy       uuid.UUID      `json:"reported_by"`
	AssignedTo       *uuid.UUID     `json:"assigned_to,omitempty"`
	Title            string         `json:"title"`
	Description      string         `json:"description"`
	Category         string         `json:"category"`
	Status           string         `json:"status"`           // open|notice_sent|hearing_scheduled|cured|fined|resolved|appealed
	Severity         int16          `json:"severity"`         // 1–5
	DueDate          *time.Time     `json:"due_date,omitempty"`
	GoverningDocID   *uuid.UUID     `json:"governing_doc_id,omitempty"`
	GoverningSection *string        `json:"governing_section,omitempty"`
	OffenseNumber    *int16         `json:"offense_number,omitempty"`
	CureDeadline     *time.Time     `json:"cure_deadline,omitempty"`
	CureVerifiedAt   *time.Time     `json:"cure_verified_at,omitempty"`
	CureVerifiedBy   *uuid.UUID     `json:"cure_verified_by,omitempty"`
	FineTotalCents   int64          `json:"fine_total_cents"`
	ResolvedAt       *time.Time     `json:"resolved_at,omitempty"`
	EvidenceDocIDs   []uuid.UUID    `json:"evidence_doc_ids"`
	Metadata         map[string]any `json:"metadata"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	DeletedAt        *time.Time     `json:"deleted_at,omitempty"`
}

// ViolationAction records a lifecycle event on a violation.
type ViolationAction struct {
	ID          uuid.UUID      `json:"id"`
	ViolationID uuid.UUID      `json:"violation_id"`
	ActorID     uuid.UUID      `json:"actor_id"`
	ActionType  string         `json:"action_type"`
	Notes       *string        `json:"notes,omitempty"`
	Metadata    map[string]any `json:"metadata"`
	CreatedAt   time.Time      `json:"created_at"`
}

// ARBRequest is an Architectural Review Board request submitted by a homeowner.
type ARBRequest struct {
	ID                uuid.UUID       `json:"id"`
	OrgID             uuid.UUID       `json:"org_id"`
	UnitID            uuid.UUID       `json:"unit_id"`
	SubmittedBy       uuid.UUID       `json:"submitted_by"`
	Title             string          `json:"title"`
	Description       string          `json:"description"`
	Category          string          `json:"category"`
	Status            string          `json:"status"`    // submitted|under_review|approved|denied|conditional_approve|withdrawn
	ReviewedBy        *uuid.UUID      `json:"reviewed_by,omitempty"`
	DecisionNotes     *string         `json:"decision_notes,omitempty"`
	DecidedAt         *time.Time      `json:"decided_at,omitempty"`
	SupportingDocIDs  []uuid.UUID     `json:"supporting_doc_ids"`
	GoverningDocID    *uuid.UUID      `json:"governing_doc_id,omitempty"`
	GoverningSection  *string         `json:"governing_section,omitempty"`
	ReviewDeadline    *time.Time      `json:"review_deadline,omitempty"`
	AutoApproved      bool            `json:"auto_approved"`
	Conditions        json.RawMessage `json:"conditions"`
	RevisionCount     int16           `json:"revision_count"`
	Metadata          map[string]any  `json:"metadata"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
	DeletedAt         *time.Time      `json:"deleted_at,omitempty"`
}

// ARBVote is a single board member's vote on an ARB request.
type ARBVote struct {
	ID           uuid.UUID `json:"id"`
	ARBRequestID uuid.UUID `json:"arb_request_id"`
	VoterID      uuid.UUID `json:"voter_id"`
	Vote         string    `json:"vote"` // approve|deny|abstain|conditional_approve
	Notes        *string   `json:"notes,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// Ballot represents a community vote or election.
type Ballot struct {
	ID            uuid.UUID       `json:"id"`
	OrgID         uuid.UUID       `json:"org_id"`
	Title         string          `json:"title"`
	Description   string          `json:"description"`
	BallotType    string          `json:"ballot_type"` // election|approval|survey|special_assessment
	Status        string          `json:"status"`      // draft|open|closed|cancelled
	Options       json.RawMessage `json:"options"`
	EligibleRole  string          `json:"eligible_role"`
	OpensAt       time.Time       `json:"opens_at"`
	ClosesAt      time.Time       `json:"closes_at"`
	QuorumPercent *float64        `json:"quorum_percent,omitempty"`
	PassPercent   *float64        `json:"pass_percent,omitempty"`
	EligibleUnits *int            `json:"eligible_units,omitempty"`
	VotesCast     int             `json:"votes_cast"`
	QuorumMet     *bool           `json:"quorum_met,omitempty"`
	WeightMethod  string          `json:"weight_method"` // equal|per_unit|per_sqft
	Results       json.RawMessage `json:"results,omitempty"`
	CreatedBy     uuid.UUID       `json:"created_by"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
	DeletedAt     *time.Time      `json:"deleted_at,omitempty"`
}

// BallotVote is a unit's cast vote on a ballot.
type BallotVote struct {
	ID         uuid.UUID       `json:"id"`
	BallotID   uuid.UUID       `json:"ballot_id"`
	VoterID    uuid.UUID       `json:"voter_id"`
	UnitID     uuid.UUID       `json:"unit_id"`
	Selection  json.RawMessage `json:"selection"`
	VoteWeight float64         `json:"vote_weight"`
	CreatedAt  time.Time       `json:"created_at"`
}

// ProxyAuthorization grants a proxy the right to vote on a ballot on behalf of a unit.
type ProxyAuthorization struct {
	ID         uuid.UUID  `json:"id"`
	BallotID   uuid.UUID  `json:"ballot_id"`
	UnitID     uuid.UUID  `json:"unit_id"`
	GrantorID  uuid.UUID  `json:"grantor_id"`
	ProxyID    uuid.UUID  `json:"proxy_id"`
	FiledAt    time.Time  `json:"filed_at"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
	DocumentID *uuid.UUID `json:"document_id,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

// Meeting represents a community or board meeting.
type Meeting struct {
	ID                  uuid.UUID      `json:"id"`
	OrgID               uuid.UUID      `json:"org_id"`
	Title               string         `json:"title"`
	MeetingType         string         `json:"meeting_type"` // annual|board|special|emergency
	Status              string         `json:"status"`       // scheduled|in_progress|completed|cancelled
	ScheduledAt         time.Time      `json:"scheduled_at"`
	EndedAt             *time.Time     `json:"ended_at,omitempty"`
	Location            *string        `json:"location,omitempty"`
	IsVirtual           bool           `json:"is_virtual"`
	VirtualLink         *string        `json:"virtual_link,omitempty"`
	NoticeRequiredDays  *int16         `json:"notice_required_days,omitempty"`
	NoticeSentAt        *time.Time     `json:"notice_sent_at,omitempty"`
	QuorumRequired      *int16         `json:"quorum_required,omitempty"`
	QuorumPresent       *int16         `json:"quorum_present,omitempty"`
	QuorumMet           *bool          `json:"quorum_met,omitempty"`
	AgendaDocID         *uuid.UUID     `json:"agenda_doc_id,omitempty"`
	MinutesDocID        *uuid.UUID     `json:"minutes_doc_id,omitempty"`
	Metadata            map[string]any `json:"metadata"`
	CreatedBy           uuid.UUID      `json:"created_by"`
	CreatedAt           time.Time      `json:"created_at"`
	UpdatedAt           time.Time      `json:"updated_at"`
	DeletedAt           *time.Time     `json:"deleted_at,omitempty"`
}

// MeetingAttendee records a user's attendance at a meeting.
type MeetingAttendee struct {
	ID         uuid.UUID  `json:"id"`
	MeetingID  uuid.UUID  `json:"meeting_id"`
	UserID     uuid.UUID  `json:"user_id"`
	Role       string     `json:"role"`        // member|board|manager|guest
	RSVPStatus *string    `json:"rsvp_status,omitempty"` // accepted|declined|tentative
	Attended   *bool      `json:"attended,omitempty"`
	ArrivedAt  *time.Time `json:"arrived_at,omitempty"`
	LeftAt     *time.Time `json:"left_at,omitempty"`
}

// MeetingMotion records a formal motion made during a meeting.
type MeetingMotion struct {
	ID           uuid.UUID  `json:"id"`
	MeetingID    uuid.UUID  `json:"meeting_id"`
	MotionNumber int16      `json:"motion_number"`
	Title        string     `json:"title"`
	Description  *string    `json:"description,omitempty"`
	MovedBy      uuid.UUID  `json:"moved_by"`
	SecondedBy   *uuid.UUID `json:"seconded_by,omitempty"`
	Status       string     `json:"status"` // pending|passed|failed|tabled|withdrawn
	VotesFor     *int16     `json:"votes_for,omitempty"`
	VotesAgainst *int16     `json:"votes_against,omitempty"`
	VotesAbstain *int16     `json:"votes_abstain,omitempty"`
	ResultNotes  *string    `json:"result_notes,omitempty"`
	ResourceType *string    `json:"resource_type,omitempty"`
	ResourceID   *uuid.UUID `json:"resource_id,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

// HearingLink connects a meeting to a violation for formal hearing purposes.
type HearingLink struct {
	ID                    uuid.UUID  `json:"id"`
	MeetingID             uuid.UUID  `json:"meeting_id"`
	ViolationID           uuid.UUID  `json:"violation_id"`
	HomeownerNotifiedAt   *time.Time `json:"homeowner_notified_at,omitempty"`
	NoticeDocID           *uuid.UUID `json:"notice_doc_id,omitempty"`
	HomeownerAttended     *bool      `json:"homeowner_attended,omitempty"`
	HomeownerStatement    *string    `json:"homeowner_statement,omitempty"`
	BoardFinding          *string    `json:"board_finding,omitempty"`
	FineUpheld            *bool      `json:"fine_upheld,omitempty"`
	FineModifiedCents     *int64     `json:"fine_modified_cents,omitempty"`
	CreatedAt             time.Time  `json:"created_at"`
}
