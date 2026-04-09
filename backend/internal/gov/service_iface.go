package gov

import (
	"context"

	"github.com/google/uuid"
)

// Service defines the business operations for the governance module.
// Handlers depend on this interface rather than the concrete GovService struct.
type Service interface {
	// Violations
	ReportViolation(ctx context.Context, orgID uuid.UUID, req CreateViolationRequest, reportedBy uuid.UUID) (*Violation, error)
	ListViolations(ctx context.Context, orgID uuid.UUID, limit int, afterID *uuid.UUID) ([]Violation, bool, error)
	GetViolation(ctx context.Context, id uuid.UUID) (*Violation, error)
	UpdateViolation(ctx context.Context, id uuid.UUID, v *Violation) (*Violation, error)
	AddViolationAction(ctx context.Context, violationID uuid.UUID, req CreateViolationActionRequest, actorID uuid.UUID) (*ViolationAction, error)
	VerifyCure(ctx context.Context, violationID uuid.UUID, verifiedBy uuid.UUID) (*Violation, error)
	ScheduleHearing(ctx context.Context, violationID uuid.UUID, meetingID uuid.UUID) (*HearingLink, error)
	GetHearing(ctx context.Context, violationID uuid.UUID) (*HearingLink, error)
	UpdateHearing(ctx context.Context, hearingID uuid.UUID, h *HearingLink) (*HearingLink, error)

	// ARB
	SubmitARBRequest(ctx context.Context, orgID uuid.UUID, req CreateARBRequestRequest, submittedBy uuid.UUID) (*ARBRequest, error)
	ListARBRequests(ctx context.Context, orgID uuid.UUID) ([]ARBRequest, error)
	GetARBRequest(ctx context.Context, id uuid.UUID) (*ARBRequest, error)
	UpdateARBRequest(ctx context.Context, id uuid.UUID, r *ARBRequest) (*ARBRequest, error)
	CastARBVote(ctx context.Context, requestID uuid.UUID, req CastARBVoteRequest, voterID uuid.UUID) (*ARBVote, error)
	RequestRevision(ctx context.Context, requestID uuid.UUID) (*ARBRequest, error)

	// Ballots
	CreateBallot(ctx context.Context, orgID uuid.UUID, req CreateBallotRequest, createdBy uuid.UUID) (*Ballot, error)
	ListBallots(ctx context.Context, orgID uuid.UUID) ([]Ballot, error)
	GetBallot(ctx context.Context, id uuid.UUID) (*Ballot, error)
	UpdateBallot(ctx context.Context, id uuid.UUID, b *Ballot) (*Ballot, error)
	CastBallotVote(ctx context.Context, ballotID uuid.UUID, req CastBallotVoteRequest, voterID uuid.UUID) (*BallotVote, error)
	FileProxy(ctx context.Context, ballotID uuid.UUID, req FileProxyRequest, grantorID uuid.UUID) (*ProxyAuthorization, error)
	RevokeProxy(ctx context.Context, proxyID uuid.UUID) error

	// Meetings
	ScheduleMeeting(ctx context.Context, orgID uuid.UUID, req CreateMeetingRequest, createdBy uuid.UUID) (*Meeting, error)
	ListMeetings(ctx context.Context, orgID uuid.UUID) ([]Meeting, error)
	GetMeeting(ctx context.Context, id uuid.UUID) (*Meeting, error)
	UpdateMeeting(ctx context.Context, id uuid.UUID, m *Meeting) (*Meeting, error)
	AddAttendee(ctx context.Context, meetingID uuid.UUID, a *MeetingAttendee) (*MeetingAttendee, error)
	RecordMotion(ctx context.Context, meetingID uuid.UUID, req RecordMotionRequest) (*MeetingMotion, error)
	UpdateMotion(ctx context.Context, motionID uuid.UUID, m *MeetingMotion) (*MeetingMotion, error)
}
