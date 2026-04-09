package gov

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/ai"
	"github.com/quorant/quorant/internal/audit"
	"github.com/quorant/quorant/internal/platform/api"
	"github.com/quorant/quorant/internal/platform/queue"
)

// GovService orchestrates violations, ARB requests, ballots, and meetings.
type GovService struct {
	violations ViolationRepository
	arb        ARBRepository
	ballots    BallotRepository
	meetings   MeetingRepository
	auditor    audit.Auditor
	publisher  queue.Publisher
	policy     ai.PolicyResolver
	compliance ai.ComplianceResolver
	logger     *slog.Logger
}

// NewGovService constructs a GovService with all required repositories.
func NewGovService(
	violations ViolationRepository,
	arb ARBRepository,
	ballots BallotRepository,
	meetings MeetingRepository,
	auditor audit.Auditor,
	publisher queue.Publisher,
	policy ai.PolicyResolver,
	compliance ai.ComplianceResolver,
	logger *slog.Logger,
) *GovService {
	return &GovService{
		violations: violations,
		arb:        arb,
		ballots:    ballots,
		meetings:   meetings,
		auditor:    auditor,
		publisher:  publisher,
		policy:     policy,
		compliance: compliance,
		logger:     logger,
	}
}

// ---------------------------------------------------------------------------
// Violations
// ---------------------------------------------------------------------------

// ReportViolation validates the request, sets the offense number, and persists
// a new Violation record.
func (s *GovService) ReportViolation(ctx context.Context, orgID uuid.UUID, req CreateViolationRequest, reportedBy uuid.UUID) (*Violation, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	count, err := s.violations.GetOffenseCount(ctx, req.UnitID, req.Category)
	if err != nil {
		return nil, err
	}
	offenseNum := int16(count + 1)

	now := time.Now().UTC()
	v := &Violation{
		ID:            uuid.New(),
		OrgID:         orgID,
		UnitID:        req.UnitID,
		ReportedBy:    reportedBy,
		Title:         req.Title,
		Description:   req.Description,
		Category:      req.Category,
		Status:        "open",
		Severity:      req.Severity,
		OffenseNumber: &offenseNum,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	// Optional: look up fine schedule policy to set cure deadline.
	if s.policy != nil {
		result, err := s.policy.GetPolicy(ctx, orgID, "fine_schedule")
		if err == nil && result != nil {
			var cfg struct {
				CureDays int `json:"cure_days"`
			}
			if jsonErr := json.Unmarshal(result.Config, &cfg); jsonErr == nil && cfg.CureDays > 0 {
				deadline := now.AddDate(0, 0, cfg.CureDays)
				v.CureDeadline = &deadline
			}
		}
	}

	return s.violations.Create(ctx, v)
}

// ListViolations returns violations for an org, supporting cursor-based pagination.
// limit controls the page size; afterID is the cursor from the previous page.
func (s *GovService) ListViolations(ctx context.Context, orgID uuid.UUID, limit int, afterID *uuid.UUID) ([]Violation, bool, error) {
	return s.violations.ListByOrg(ctx, orgID, limit, afterID)
}

// GetViolation returns a single violation by ID or a 404 error if not found.
func (s *GovService) GetViolation(ctx context.Context, id uuid.UUID) (*Violation, error) {
	v, err := s.violations.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if v == nil {
		return nil, api.NewNotFoundError("violation not found")
	}
	return v, nil
}

// UpdateViolation persists changes to an existing violation.
func (s *GovService) UpdateViolation(ctx context.Context, id uuid.UUID, v *Violation) (*Violation, error) {
	v.ID = id
	v.UpdatedAt = time.Now().UTC()
	return s.violations.Update(ctx, v)
}

// AddViolationAction appends a lifecycle action to a violation.
func (s *GovService) AddViolationAction(ctx context.Context, violationID uuid.UUID, req CreateViolationActionRequest, actorID uuid.UUID) (*ViolationAction, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	a := &ViolationAction{
		ID:          uuid.New(),
		ViolationID: violationID,
		ActorID:     actorID,
		ActionType:  req.ActionType,
		Notes:       req.Notes,
		CreatedAt:   time.Now().UTC(),
	}

	return s.violations.CreateAction(ctx, a)
}

// VerifyCure marks a violation's cure as verified and sets its status to resolved.
func (s *GovService) VerifyCure(ctx context.Context, violationID uuid.UUID, verifiedBy uuid.UUID) (*Violation, error) {
	v, err := s.GetViolation(ctx, violationID)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	v.CureVerifiedAt = &now
	v.CureVerifiedBy = &verifiedBy
	v.Status = "resolved"
	v.ResolvedAt = &now
	v.UpdatedAt = now

	return s.violations.Update(ctx, v)
}

// ScheduleHearing creates a HearingLink connecting a violation to a meeting.
func (s *GovService) ScheduleHearing(ctx context.Context, violationID uuid.UUID, meetingID uuid.UUID) (*HearingLink, error) {
	h := &HearingLink{
		ID:          uuid.New(),
		MeetingID:   meetingID,
		ViolationID: violationID,
		CreatedAt:   time.Now().UTC(),
	}
	return s.meetings.CreateHearingLink(ctx, h)
}

// GetHearing returns the hearing link for a violation.
func (s *GovService) GetHearing(ctx context.Context, violationID uuid.UUID) (*HearingLink, error) {
	h, err := s.meetings.FindHearingByViolation(ctx, violationID)
	if err != nil {
		return nil, err
	}
	if h == nil {
		return nil, api.NewNotFoundError("hearing not found")
	}
	return h, nil
}

// UpdateHearing persists changes to an existing hearing link.
func (s *GovService) UpdateHearing(ctx context.Context, hearingID uuid.UUID, h *HearingLink) (*HearingLink, error) {
	h.ID = hearingID
	return s.meetings.UpdateHearingLink(ctx, h)
}

// ---------------------------------------------------------------------------
// ARB
// ---------------------------------------------------------------------------

// SubmitARBRequest validates and persists a new ARB request.
func (s *GovService) SubmitARBRequest(ctx context.Context, orgID uuid.UUID, req CreateARBRequestRequest, submittedBy uuid.UUID) (*ARBRequest, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	r := &ARBRequest{
		ID:          uuid.New(),
		OrgID:       orgID,
		UnitID:      req.UnitID,
		SubmittedBy: submittedBy,
		Title:       req.Title,
		Description: req.Description,
		Category:    req.Category,
		Status:      "submitted",
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	return s.arb.Create(ctx, r)
}

// ListARBRequests returns all ARB requests for an org.
func (s *GovService) ListARBRequests(ctx context.Context, orgID uuid.UUID) ([]ARBRequest, error) {
	return s.arb.ListByOrg(ctx, orgID)
}

// GetARBRequest returns a single ARB request by ID or a 404 error if not found.
func (s *GovService) GetARBRequest(ctx context.Context, id uuid.UUID) (*ARBRequest, error) {
	r, err := s.arb.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, api.NewNotFoundError("ARB request not found")
	}
	return r, nil
}

// UpdateARBRequest persists changes to an existing ARB request.
func (s *GovService) UpdateARBRequest(ctx context.Context, id uuid.UUID, r *ARBRequest) (*ARBRequest, error) {
	r.ID = id
	r.UpdatedAt = time.Now().UTC()
	return s.arb.Update(ctx, r)
}

// CastARBVote validates and records a board member's vote on an ARB request.
func (s *GovService) CastARBVote(ctx context.Context, requestID uuid.UUID, req CastARBVoteRequest, voterID uuid.UUID) (*ARBVote, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	vote := &ARBVote{
		ID:           uuid.New(),
		ARBRequestID: requestID,
		VoterID:      voterID,
		Vote:         req.Vote,
		CreatedAt:    time.Now().UTC(),
	}

	return s.arb.CreateVote(ctx, vote)
}

// RequestRevision sets an ARB request's status to revision_requested and increments its revision count.
func (s *GovService) RequestRevision(ctx context.Context, requestID uuid.UUID) (*ARBRequest, error) {
	r, err := s.GetARBRequest(ctx, requestID)
	if err != nil {
		return nil, err
	}

	r.Status = "revision_requested"
	r.RevisionCount++
	r.UpdatedAt = time.Now().UTC()

	return s.arb.Update(ctx, r)
}

// ---------------------------------------------------------------------------
// Ballots
// ---------------------------------------------------------------------------

// CreateBallot validates and persists a new ballot.
func (s *GovService) CreateBallot(ctx context.Context, orgID uuid.UUID, req CreateBallotRequest, createdBy uuid.UUID) (*Ballot, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	b := &Ballot{
		ID:          uuid.New(),
		OrgID:       orgID,
		Title:       req.Title,
		Description: req.Description,
		BallotType:  req.BallotType,
		Status:      "draft",
		OpensAt:     req.OpensAt,
		ClosesAt:    req.ClosesAt,
		CreatedBy:   createdBy,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	return s.ballots.Create(ctx, b)
}

// ListBallots returns all ballots for an org.
func (s *GovService) ListBallots(ctx context.Context, orgID uuid.UUID) ([]Ballot, error) {
	return s.ballots.ListByOrg(ctx, orgID)
}

// GetBallot returns a single ballot by ID or a 404 error if not found.
func (s *GovService) GetBallot(ctx context.Context, id uuid.UUID) (*Ballot, error) {
	b, err := s.ballots.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if b == nil {
		return nil, api.NewNotFoundError("ballot not found")
	}
	return b, nil
}

// UpdateBallot persists changes to an existing ballot.
func (s *GovService) UpdateBallot(ctx context.Context, id uuid.UUID, b *Ballot) (*Ballot, error) {
	b.ID = id
	b.UpdatedAt = time.Now().UTC()
	return s.ballots.Update(ctx, b)
}

// CastBallotVote validates and records a unit's vote on a ballot.
func (s *GovService) CastBallotVote(ctx context.Context, ballotID uuid.UUID, req CastBallotVoteRequest, voterID uuid.UUID) (*BallotVote, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	vote := &BallotVote{
		ID:        uuid.New(),
		BallotID:  ballotID,
		VoterID:   voterID,
		UnitID:    req.UnitID,
		Selection: req.Selection,
		CreatedAt: time.Now().UTC(),
	}

	return s.ballots.CastVote(ctx, vote)
}

// FileProxy validates and records a proxy authorization for a ballot.
func (s *GovService) FileProxy(ctx context.Context, ballotID uuid.UUID, req FileProxyRequest, grantorID uuid.UUID) (*ProxyAuthorization, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	p := &ProxyAuthorization{
		ID:        uuid.New(),
		BallotID:  ballotID,
		UnitID:    req.UnitID,
		GrantorID: grantorID,
		ProxyID:   req.ProxyID,
		FiledAt:   now,
		CreatedAt: now,
	}

	return s.ballots.FileProxy(ctx, p)
}

// RevokeProxy revokes an existing proxy authorization.
func (s *GovService) RevokeProxy(ctx context.Context, proxyID uuid.UUID) error {
	return s.ballots.RevokeProxy(ctx, proxyID)
}

// ---------------------------------------------------------------------------
// Meetings
// ---------------------------------------------------------------------------

// ScheduleMeeting validates and persists a new meeting.
func (s *GovService) ScheduleMeeting(ctx context.Context, orgID uuid.UUID, req CreateMeetingRequest, createdBy uuid.UUID) (*Meeting, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	m := &Meeting{
		ID:          uuid.New(),
		OrgID:       orgID,
		Title:       req.Title,
		MeetingType: req.MeetingType,
		Status:      "scheduled",
		ScheduledAt: req.ScheduledAt,
		CreatedBy:   createdBy,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	return s.meetings.Create(ctx, m)
}

// ListMeetings returns all meetings for an org.
func (s *GovService) ListMeetings(ctx context.Context, orgID uuid.UUID) ([]Meeting, error) {
	return s.meetings.ListByOrg(ctx, orgID)
}

// GetMeeting returns a single meeting by ID or a 404 error if not found.
func (s *GovService) GetMeeting(ctx context.Context, id uuid.UUID) (*Meeting, error) {
	m, err := s.meetings.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, api.NewNotFoundError("meeting not found")
	}
	return m, nil
}

// UpdateMeeting persists changes to an existing meeting.
func (s *GovService) UpdateMeeting(ctx context.Context, id uuid.UUID, m *Meeting) (*Meeting, error) {
	m.ID = id
	m.UpdatedAt = time.Now().UTC()
	return s.meetings.Update(ctx, m)
}

// AddAttendee records an attendee on a meeting.
func (s *GovService) AddAttendee(ctx context.Context, meetingID uuid.UUID, a *MeetingAttendee) (*MeetingAttendee, error) {
	a.MeetingID = meetingID
	if a.ID == (uuid.UUID{}) {
		a.ID = uuid.New()
	}
	return s.meetings.AddAttendee(ctx, a)
}

// RecordMotion validates and records a formal motion during a meeting.
func (s *GovService) RecordMotion(ctx context.Context, meetingID uuid.UUID, req RecordMotionRequest) (*MeetingMotion, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	// Determine next motion number.
	existing, err := s.meetings.ListMotionsByMeeting(ctx, meetingID)
	if err != nil {
		return nil, err
	}

	m := &MeetingMotion{
		ID:           uuid.New(),
		MeetingID:    meetingID,
		MotionNumber: int16(len(existing) + 1),
		Title:        req.Title,
		MovedBy:      req.MovedBy,
		Status:       "pending",
		CreatedAt:    time.Now().UTC(),
	}

	return s.meetings.CreateMotion(ctx, m)
}

// UpdateMotion persists changes to an existing meeting motion.
func (s *GovService) UpdateMotion(ctx context.Context, motionID uuid.UUID, m *MeetingMotion) (*MeetingMotion, error) {
	m.ID = motionID
	return s.meetings.UpdateMotion(ctx, m)
}
