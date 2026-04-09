package gov

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/platform/api"
)

// CreateViolationRequest is the request body for reporting a new violation.
type CreateViolationRequest struct {
	UnitID      uuid.UUID `json:"unit_id"`     // required
	Title       string    `json:"title"`       // required
	Description string    `json:"description"` // required
	Category    string    `json:"category"`    // required
	Severity    int16     `json:"severity"`    // optional, defaults to 1
}

// Validate checks that all required fields are present and applies defaults.
func (r *CreateViolationRequest) Validate() error {
	if r.UnitID == (uuid.UUID{}) {
		return api.NewValidationError("unit_id is required", "unit_id")
	}
	if r.Title == "" {
		return api.NewValidationError("title is required", "title")
	}
	if r.Description == "" {
		return api.NewValidationError("description is required", "description")
	}
	if r.Category == "" {
		return api.NewValidationError("category is required", "category")
	}
	if r.Severity == 0 {
		r.Severity = 1
	}
	return nil
}

// CreateARBRequestRequest is the request body for submitting an ARB request.
type CreateARBRequestRequest struct {
	UnitID      uuid.UUID `json:"unit_id"`     // required
	Title       string    `json:"title"`       // required
	Description string    `json:"description"` // required
	Category    string    `json:"category"`    // required
}

// Validate checks that all required fields are present.
func (r CreateARBRequestRequest) Validate() error {
	if r.UnitID == (uuid.UUID{}) {
		return api.NewValidationError("unit_id is required", "unit_id")
	}
	if r.Title == "" {
		return api.NewValidationError("title is required", "title")
	}
	if r.Description == "" {
		return api.NewValidationError("description is required", "description")
	}
	if r.Category == "" {
		return api.NewValidationError("category is required", "category")
	}
	return nil
}

// CreateBallotRequest is the request body for creating a new ballot.
type CreateBallotRequest struct {
	Title       string    `json:"title"`       // required
	Description string    `json:"description"` // required
	BallotType  string    `json:"ballot_type"` // required
	OpensAt     time.Time `json:"opens_at"`    // required
	ClosesAt    time.Time `json:"closes_at"`   // required
}

// Validate checks that all required fields are present.
func (r CreateBallotRequest) Validate() error {
	if r.Title == "" {
		return api.NewValidationError("title is required", "title")
	}
	if r.Description == "" {
		return api.NewValidationError("description is required", "description")
	}
	if r.BallotType == "" {
		return api.NewValidationError("ballot_type is required", "ballot_type")
	}
	if r.OpensAt.IsZero() {
		return api.NewValidationError("opens_at is required", "opens_at")
	}
	if r.ClosesAt.IsZero() {
		return api.NewValidationError("closes_at is required", "closes_at")
	}
	return nil
}

// CastBallotVoteRequest is the request body for casting a vote on a ballot.
type CastBallotVoteRequest struct {
	UnitID    uuid.UUID       `json:"unit_id"`   // required
	Selection json.RawMessage `json:"selection"` // required
}

// Validate checks that unit_id and selection are present.
func (r CastBallotVoteRequest) Validate() error {
	if r.UnitID == (uuid.UUID{}) {
		return api.NewValidationError("unit_id is required", "unit_id")
	}
	if len(r.Selection) == 0 {
		return api.NewValidationError("selection is required", "selection")
	}
	return nil
}

// FileProxyRequest is the request body for filing a proxy authorization.
type FileProxyRequest struct {
	UnitID  uuid.UUID `json:"unit_id"`  // required
	ProxyID uuid.UUID `json:"proxy_id"` // required
}

// Validate checks that both unit_id and proxy_id are present.
func (r FileProxyRequest) Validate() error {
	if r.UnitID == (uuid.UUID{}) {
		return api.NewValidationError("unit_id is required", "unit_id")
	}
	if r.ProxyID == (uuid.UUID{}) {
		return api.NewValidationError("proxy_id is required", "proxy_id")
	}
	return nil
}

// CreateMeetingRequest is the request body for scheduling a new meeting.
type CreateMeetingRequest struct {
	Title       string    `json:"title"`        // required
	MeetingType string    `json:"meeting_type"` // required
	ScheduledAt time.Time `json:"scheduled_at"` // required
}

// Validate checks that title, meeting_type, and scheduled_at are present.
func (r CreateMeetingRequest) Validate() error {
	if r.Title == "" {
		return api.NewValidationError("title is required", "title")
	}
	if r.MeetingType == "" {
		return api.NewValidationError("meeting_type is required", "meeting_type")
	}
	if r.ScheduledAt.IsZero() {
		return api.NewValidationError("scheduled_at is required", "scheduled_at")
	}
	return nil
}

// RecordMotionRequest is the request body for recording a motion in a meeting.
type RecordMotionRequest struct {
	Title   string    `json:"title"`    // required
	MovedBy uuid.UUID `json:"moved_by"` // required
}

// Validate checks that title and moved_by are present.
func (r RecordMotionRequest) Validate() error {
	if r.Title == "" {
		return api.NewValidationError("title is required", "title")
	}
	if r.MovedBy == (uuid.UUID{}) {
		return api.NewValidationError("moved_by is required", "moved_by")
	}
	return nil
}

// CreateViolationActionRequest is the request body for adding an action to a violation.
type CreateViolationActionRequest struct {
	ActionType string  `json:"action_type"` // required
	Notes      *string `json:"notes,omitempty"`
}

// Validate checks that action_type is present.
func (r CreateViolationActionRequest) Validate() error {
	if r.ActionType == "" {
		return api.NewValidationError("action_type is required", "action_type")
	}
	return nil
}

// CastARBVoteRequest is the request body for casting a vote on an ARB request.
type CastARBVoteRequest struct {
	Vote string `json:"vote"` // required: approve|deny|abstain|conditional_approve
}

// Validate checks that vote is present and one of the allowed values.
func (r CastARBVoteRequest) Validate() error {
	switch r.Vote {
	case "approve", "deny", "abstain", "conditional_approve":
		// valid
	case "":
		return api.NewValidationError("vote is required", "vote")
	default:
		return api.NewValidationError("vote must be one of: approve, deny, abstain, conditional_approve", "vote")
	}
	return nil
}
