package gov_test

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/ai"
	"github.com/quorant/quorant/internal/audit"
	"github.com/quorant/quorant/internal/gov"
	"github.com/quorant/quorant/internal/platform/api"
	"github.com/quorant/quorant/internal/platform/queue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// In-memory mock repositories
// ---------------------------------------------------------------------------

// mockViolationRepo is an in-memory ViolationRepository.
type mockViolationRepo struct {
	violations    map[uuid.UUID]*gov.Violation
	actions       map[uuid.UUID][]*gov.ViolationAction
	offenseCounts map[string]int // key: unitID+":"+category
}

func newMockViolationRepo() *mockViolationRepo {
	return &mockViolationRepo{
		violations:    make(map[uuid.UUID]*gov.Violation),
		actions:       make(map[uuid.UUID][]*gov.ViolationAction),
		offenseCounts: make(map[string]int),
	}
}

func (r *mockViolationRepo) setOffenseCount(unitID uuid.UUID, category string, count int) {
	r.offenseCounts[unitID.String()+":"+category] = count
}

func (r *mockViolationRepo) Create(ctx context.Context, v *gov.Violation) (*gov.Violation, error) {
	r.violations[v.ID] = v
	return v, nil
}

func (r *mockViolationRepo) FindByID(ctx context.Context, id uuid.UUID) (*gov.Violation, error) {
	v, ok := r.violations[id]
	if !ok {
		return nil, nil
	}
	return v, nil
}

func (r *mockViolationRepo) ListByOrg(ctx context.Context, orgID uuid.UUID, limit int, afterID *uuid.UUID) ([]gov.Violation, bool, error) {
	var out []gov.Violation
	for _, v := range r.violations {
		if v.OrgID == orgID {
			out = append(out, *v)
		}
	}
	hasMore := limit > 0 && len(out) > limit
	if hasMore {
		out = out[:limit]
	}
	return out, hasMore, nil
}

func (r *mockViolationRepo) ListByUnit(ctx context.Context, unitID uuid.UUID) ([]gov.Violation, error) {
	var out []gov.Violation
	for _, v := range r.violations {
		if v.UnitID == unitID {
			out = append(out, *v)
		}
	}
	return out, nil
}

func (r *mockViolationRepo) Update(ctx context.Context, v *gov.Violation) (*gov.Violation, error) {
	r.violations[v.ID] = v
	return v, nil
}

func (r *mockViolationRepo) SoftDelete(ctx context.Context, id uuid.UUID) error {
	delete(r.violations, id)
	return nil
}

func (r *mockViolationRepo) CreateAction(ctx context.Context, a *gov.ViolationAction) (*gov.ViolationAction, error) {
	r.actions[a.ViolationID] = append(r.actions[a.ViolationID], a)
	return a, nil
}

func (r *mockViolationRepo) ListActionsByViolation(ctx context.Context, violationID uuid.UUID) ([]gov.ViolationAction, error) {
	var out []gov.ViolationAction
	for _, a := range r.actions[violationID] {
		out = append(out, *a)
	}
	return out, nil
}

func (r *mockViolationRepo) GetOffenseCount(ctx context.Context, unitID uuid.UUID, category string) (int, error) {
	return r.offenseCounts[unitID.String()+":"+category], nil
}

// mockARBRepo is an in-memory ARBRepository.
type mockARBRepo struct {
	requests map[uuid.UUID]*gov.ARBRequest
	votes    map[uuid.UUID][]*gov.ARBVote
}

func newMockARBRepo() *mockARBRepo {
	return &mockARBRepo{
		requests: make(map[uuid.UUID]*gov.ARBRequest),
		votes:    make(map[uuid.UUID][]*gov.ARBVote),
	}
}

func (r *mockARBRepo) Create(ctx context.Context, req *gov.ARBRequest) (*gov.ARBRequest, error) {
	r.requests[req.ID] = req
	return req, nil
}

func (r *mockARBRepo) FindByID(ctx context.Context, id uuid.UUID) (*gov.ARBRequest, error) {
	req, ok := r.requests[id]
	if !ok {
		return nil, nil
	}
	return req, nil
}

func (r *mockARBRepo) ListByOrg(ctx context.Context, orgID uuid.UUID) ([]gov.ARBRequest, error) {
	var out []gov.ARBRequest
	for _, req := range r.requests {
		if req.OrgID == orgID {
			out = append(out, *req)
		}
	}
	return out, nil
}

func (r *mockARBRepo) Update(ctx context.Context, req *gov.ARBRequest) (*gov.ARBRequest, error) {
	r.requests[req.ID] = req
	return req, nil
}

func (r *mockARBRepo) SoftDelete(ctx context.Context, id uuid.UUID) error {
	delete(r.requests, id)
	return nil
}

func (r *mockARBRepo) CreateVote(ctx context.Context, v *gov.ARBVote) (*gov.ARBVote, error) {
	r.votes[v.ARBRequestID] = append(r.votes[v.ARBRequestID], v)
	return v, nil
}

func (r *mockARBRepo) ListVotesByRequest(ctx context.Context, requestID uuid.UUID) ([]gov.ARBVote, error) {
	var out []gov.ARBVote
	for _, v := range r.votes[requestID] {
		out = append(out, *v)
	}
	return out, nil
}

// mockBallotRepo is an in-memory BallotRepository.
type mockBallotRepo struct {
	ballots  map[uuid.UUID]*gov.Ballot
	votes    map[uuid.UUID][]*gov.BallotVote
	proxies  map[uuid.UUID]*gov.ProxyAuthorization
}

func newMockBallotRepo() *mockBallotRepo {
	return &mockBallotRepo{
		ballots: make(map[uuid.UUID]*gov.Ballot),
		votes:   make(map[uuid.UUID][]*gov.BallotVote),
		proxies: make(map[uuid.UUID]*gov.ProxyAuthorization),
	}
}

func (r *mockBallotRepo) Create(ctx context.Context, b *gov.Ballot) (*gov.Ballot, error) {
	r.ballots[b.ID] = b
	return b, nil
}

func (r *mockBallotRepo) FindByID(ctx context.Context, id uuid.UUID) (*gov.Ballot, error) {
	b, ok := r.ballots[id]
	if !ok {
		return nil, nil
	}
	return b, nil
}

func (r *mockBallotRepo) ListByOrg(ctx context.Context, orgID uuid.UUID) ([]gov.Ballot, error) {
	var out []gov.Ballot
	for _, b := range r.ballots {
		if b.OrgID == orgID {
			out = append(out, *b)
		}
	}
	return out, nil
}

func (r *mockBallotRepo) Update(ctx context.Context, b *gov.Ballot) (*gov.Ballot, error) {
	r.ballots[b.ID] = b
	return b, nil
}

func (r *mockBallotRepo) SoftDelete(ctx context.Context, id uuid.UUID) error {
	delete(r.ballots, id)
	return nil
}

func (r *mockBallotRepo) CastVote(ctx context.Context, v *gov.BallotVote) (*gov.BallotVote, error) {
	r.votes[v.BallotID] = append(r.votes[v.BallotID], v)
	return v, nil
}

func (r *mockBallotRepo) ListVotesByBallot(ctx context.Context, ballotID uuid.UUID) ([]gov.BallotVote, error) {
	var out []gov.BallotVote
	for _, v := range r.votes[ballotID] {
		out = append(out, *v)
	}
	return out, nil
}

func (r *mockBallotRepo) FileProxy(ctx context.Context, p *gov.ProxyAuthorization) (*gov.ProxyAuthorization, error) {
	r.proxies[p.ID] = p
	return p, nil
}

func (r *mockBallotRepo) RevokeProxy(ctx context.Context, id uuid.UUID) error {
	delete(r.proxies, id)
	return nil
}

func (r *mockBallotRepo) ListProxiesByBallot(ctx context.Context, ballotID uuid.UUID) ([]gov.ProxyAuthorization, error) {
	var out []gov.ProxyAuthorization
	for _, p := range r.proxies {
		if p.BallotID == ballotID {
			out = append(out, *p)
		}
	}
	return out, nil
}

// mockMeetingRepo is an in-memory MeetingRepository.
type mockMeetingRepo struct {
	meetings  map[uuid.UUID]*gov.Meeting
	attendees map[uuid.UUID][]*gov.MeetingAttendee
	motions   map[uuid.UUID][]*gov.MeetingMotion
	hearings  map[uuid.UUID]*gov.HearingLink // keyed by violationID
	hearingsByID map[uuid.UUID]*gov.HearingLink
}

func newMockMeetingRepo() *mockMeetingRepo {
	return &mockMeetingRepo{
		meetings:     make(map[uuid.UUID]*gov.Meeting),
		attendees:    make(map[uuid.UUID][]*gov.MeetingAttendee),
		motions:      make(map[uuid.UUID][]*gov.MeetingMotion),
		hearings:     make(map[uuid.UUID]*gov.HearingLink),
		hearingsByID: make(map[uuid.UUID]*gov.HearingLink),
	}
}

func (r *mockMeetingRepo) Create(ctx context.Context, m *gov.Meeting) (*gov.Meeting, error) {
	r.meetings[m.ID] = m
	return m, nil
}

func (r *mockMeetingRepo) FindByID(ctx context.Context, id uuid.UUID) (*gov.Meeting, error) {
	m, ok := r.meetings[id]
	if !ok {
		return nil, nil
	}
	return m, nil
}

func (r *mockMeetingRepo) ListByOrg(ctx context.Context, orgID uuid.UUID) ([]gov.Meeting, error) {
	var out []gov.Meeting
	for _, m := range r.meetings {
		if m.OrgID == orgID {
			out = append(out, *m)
		}
	}
	return out, nil
}

func (r *mockMeetingRepo) Update(ctx context.Context, m *gov.Meeting) (*gov.Meeting, error) {
	r.meetings[m.ID] = m
	return m, nil
}

func (r *mockMeetingRepo) SoftDelete(ctx context.Context, id uuid.UUID) error {
	delete(r.meetings, id)
	return nil
}

func (r *mockMeetingRepo) AddAttendee(ctx context.Context, a *gov.MeetingAttendee) (*gov.MeetingAttendee, error) {
	r.attendees[a.MeetingID] = append(r.attendees[a.MeetingID], a)
	return a, nil
}

func (r *mockMeetingRepo) ListAttendeesByMeeting(ctx context.Context, meetingID uuid.UUID) ([]gov.MeetingAttendee, error) {
	var out []gov.MeetingAttendee
	for _, a := range r.attendees[meetingID] {
		out = append(out, *a)
	}
	return out, nil
}

func (r *mockMeetingRepo) CreateMotion(ctx context.Context, m *gov.MeetingMotion) (*gov.MeetingMotion, error) {
	r.motions[m.MeetingID] = append(r.motions[m.MeetingID], m)
	return m, nil
}

func (r *mockMeetingRepo) UpdateMotion(ctx context.Context, m *gov.MeetingMotion) (*gov.MeetingMotion, error) {
	for i, existing := range r.motions[m.MeetingID] {
		if existing.ID == m.ID {
			r.motions[m.MeetingID][i] = m
			return m, nil
		}
	}
	return m, nil
}

func (r *mockMeetingRepo) ListMotionsByMeeting(ctx context.Context, meetingID uuid.UUID) ([]gov.MeetingMotion, error) {
	var out []gov.MeetingMotion
	for _, m := range r.motions[meetingID] {
		out = append(out, *m)
	}
	return out, nil
}

func (r *mockMeetingRepo) CreateHearingLink(ctx context.Context, h *gov.HearingLink) (*gov.HearingLink, error) {
	r.hearings[h.ViolationID] = h
	r.hearingsByID[h.ID] = h
	return h, nil
}

func (r *mockMeetingRepo) FindHearingByViolation(ctx context.Context, violationID uuid.UUID) (*gov.HearingLink, error) {
	h, ok := r.hearings[violationID]
	if !ok {
		return nil, nil
	}
	return h, nil
}

func (r *mockMeetingRepo) UpdateHearingLink(ctx context.Context, h *gov.HearingLink) (*gov.HearingLink, error) {
	r.hearingsByID[h.ID] = h
	r.hearings[h.ViolationID] = h
	return h, nil
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func newTestService() (*gov.GovService, *mockViolationRepo, *mockARBRepo, *mockBallotRepo, *mockMeetingRepo) {
	vr := newMockViolationRepo()
	ar := newMockARBRepo()
	br := newMockBallotRepo()
	mr := newMockMeetingRepo()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	svc := gov.NewGovService(vr, ar, br, mr, audit.NewNoopAuditor(), queue.NewInMemoryPublisher(), ai.NewNoopPolicyResolver(), ai.NewNoopComplianceResolver(), logger)
	return svc, vr, ar, br, mr
}

// ---------------------------------------------------------------------------
// Violation tests
// ---------------------------------------------------------------------------

func TestReportViolation_SetsOffenseNumber(t *testing.T) {
	svc, vr, _, _, _ := newTestService()
	ctx := context.Background()

	orgID := uuid.New()
	unitID := uuid.New()
	reportedBy := uuid.New()

	// Simulate 2 prior offenses for this unit+category.
	vr.setOffenseCount(unitID, "landscaping", 2)

	req := gov.CreateViolationRequest{
		UnitID:      unitID,
		Title:       "Overgrown lawn",
		Description: "Lawn exceeds 6 inches",
		Category:    "landscaping",
		Severity:    2,
	}

	v, err := svc.ReportViolation(ctx, orgID, req, reportedBy)
	require.NoError(t, err)
	require.NotNil(t, v)
	require.NotNil(t, v.OffenseNumber)
	assert.Equal(t, int16(3), *v.OffenseNumber)
}

func TestReportViolation_DefaultSeverity(t *testing.T) {
	svc, _, _, _, _ := newTestService()
	ctx := context.Background()

	req := gov.CreateViolationRequest{
		UnitID:      uuid.New(),
		Title:       "Noise complaint",
		Description: "Loud music after 10pm",
		Category:    "noise",
		Severity:    0, // not set — should default to 1
	}

	v, err := svc.ReportViolation(ctx, uuid.New(), req, uuid.New())
	require.NoError(t, err)
	require.NotNil(t, v)
	assert.Equal(t, int16(1), v.Severity)
}

func TestReportViolation_ValidationError_MissingTitle(t *testing.T) {
	svc, _, _, _, _ := newTestService()
	ctx := context.Background()

	req := gov.CreateViolationRequest{
		UnitID:      uuid.New(),
		Description: "desc",
		Category:    "noise",
	}

	_, err := svc.ReportViolation(ctx, uuid.New(), req, uuid.New())
	require.Error(t, err)
	var ve *api.ValidationError
	require.ErrorAs(t, err, &ve)
	assert.Equal(t, "title", ve.Field)
}

func TestGetViolation_NotFound(t *testing.T) {
	svc, _, _, _, _ := newTestService()
	ctx := context.Background()

	_, err := svc.GetViolation(ctx, uuid.New())
	require.Error(t, err)
	var nfe *api.NotFoundError
	require.ErrorAs(t, err, &nfe)
}

func TestVerifyCure_SetsStatusResolved(t *testing.T) {
	svc, vr, _, _, _ := newTestService()
	ctx := context.Background()

	// Pre-seed a violation.
	offNum := int16(1)
	v := &gov.Violation{
		ID:            uuid.New(),
		OrgID:         uuid.New(),
		UnitID:        uuid.New(),
		ReportedBy:    uuid.New(),
		Title:         "Fence violation",
		Description:   "Fence too tall",
		Category:      "structure",
		Status:        "open",
		Severity:      1,
		OffenseNumber: &offNum,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	vr.violations[v.ID] = v

	verifiedBy := uuid.New()
	updated, err := svc.VerifyCure(ctx, v.ID, verifiedBy)
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, "resolved", updated.Status)
	assert.NotNil(t, updated.CureVerifiedAt)
	assert.Equal(t, verifiedBy, *updated.CureVerifiedBy)
	assert.NotNil(t, updated.ResolvedAt)
}

func TestAddViolationAction_Success(t *testing.T) {
	svc, vr, _, _, _ := newTestService()
	ctx := context.Background()

	offNum := int16(1)
	v := &gov.Violation{
		ID: uuid.New(), OrgID: uuid.New(), UnitID: uuid.New(),
		ReportedBy: uuid.New(), Title: "t", Description: "d",
		Category: "c", Status: "open", Severity: 1,
		OffenseNumber: &offNum, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	vr.violations[v.ID] = v

	notes := "notice mailed"
	req := gov.CreateViolationActionRequest{
		ActionType: "notice_sent",
		Notes:      &notes,
	}
	action, err := svc.AddViolationAction(ctx, v.ID, req, uuid.New())
	require.NoError(t, err)
	require.NotNil(t, action)
	assert.Equal(t, "notice_sent", action.ActionType)
	assert.Equal(t, v.ID, action.ViolationID)
}

// ---------------------------------------------------------------------------
// ARB tests
// ---------------------------------------------------------------------------

func TestSubmitARBRequest_Success(t *testing.T) {
	svc, _, _, _, _ := newTestService()
	ctx := context.Background()

	orgID := uuid.New()
	submittedBy := uuid.New()
	req := gov.CreateARBRequestRequest{
		UnitID:      uuid.New(),
		Title:       "New deck",
		Description: "Adding a 10x12 cedar deck",
		Category:    "construction",
	}

	r, err := svc.SubmitARBRequest(ctx, orgID, req, submittedBy)
	require.NoError(t, err)
	require.NotNil(t, r)
	assert.Equal(t, "submitted", r.Status)
	assert.Equal(t, orgID, r.OrgID)
	assert.Equal(t, submittedBy, r.SubmittedBy)
}

func TestRequestRevision_IncrementsCount(t *testing.T) {
	svc, _, ar, _, _ := newTestService()
	ctx := context.Background()

	arb := &gov.ARBRequest{
		ID:            uuid.New(),
		OrgID:         uuid.New(),
		UnitID:        uuid.New(),
		SubmittedBy:   uuid.New(),
		Title:         "Pool fence",
		Description:   "Installing pool fence",
		Category:      "safety",
		Status:        "under_review",
		RevisionCount: 0,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	ar.requests[arb.ID] = arb

	updated, err := svc.RequestRevision(ctx, arb.ID)
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, "revision_requested", updated.Status)
	assert.Equal(t, int16(1), updated.RevisionCount)
}

func TestCastARBVote_Validation_InvalidVoteValue(t *testing.T) {
	svc, _, _, _, _ := newTestService()
	ctx := context.Background()

	req := gov.CastARBVoteRequest{Vote: "maybe"}
	_, err := svc.CastARBVote(ctx, uuid.New(), req, uuid.New())
	require.Error(t, err)
	var ve *api.ValidationError
	require.ErrorAs(t, err, &ve)
}

func TestCastARBVote_ValidValues(t *testing.T) {
	validVotes := []string{"approve", "deny", "abstain", "conditional_approve"}
	for _, vote := range validVotes {
		t.Run(vote, func(t *testing.T) {
			svc, _, _, _, _ := newTestService()
			ctx := context.Background()

			req := gov.CastARBVoteRequest{Vote: vote}
			v, err := svc.CastARBVote(ctx, uuid.New(), req, uuid.New())
			require.NoError(t, err)
			require.NotNil(t, v)
			assert.Equal(t, vote, v.Vote)
		})
	}
}

// ---------------------------------------------------------------------------
// Ballot tests
// ---------------------------------------------------------------------------

func TestCreateBallot_Success(t *testing.T) {
	svc, _, _, _, _ := newTestService()
	ctx := context.Background()

	orgID := uuid.New()
	createdBy := uuid.New()
	req := gov.CreateBallotRequest{
		Title:       "Board Election 2026",
		Description: "Annual board member election",
		BallotType:  "election",
		OpensAt:     time.Now().Add(24 * time.Hour),
		ClosesAt:    time.Now().Add(7 * 24 * time.Hour),
	}

	b, err := svc.CreateBallot(ctx, orgID, req, createdBy)
	require.NoError(t, err)
	require.NotNil(t, b)
	assert.Equal(t, "draft", b.Status)
	assert.Equal(t, orgID, b.OrgID)
	assert.Equal(t, createdBy, b.CreatedBy)
	assert.Equal(t, "election", b.BallotType)
}

func TestCastBallotVote_Success(t *testing.T) {
	svc, _, _, br, _ := newTestService()
	ctx := context.Background()

	ballot := &gov.Ballot{
		ID:        uuid.New(),
		OrgID:     uuid.New(),
		Title:     "t",
		BallotType: "approval",
		Status:    "open",
		OpensAt:   time.Now().Add(-time.Hour),
		ClosesAt:  time.Now().Add(time.Hour),
		CreatedBy: uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	br.ballots[ballot.ID] = ballot

	voterID := uuid.New()
	req := gov.CastBallotVoteRequest{
		UnitID:    uuid.New(),
		Selection: []byte(`{"option":"yes"}`),
	}
	v, err := svc.CastBallotVote(ctx, ballot.ID, req, voterID)
	require.NoError(t, err)
	require.NotNil(t, v)
	assert.Equal(t, ballot.ID, v.BallotID)
	assert.Equal(t, voterID, v.VoterID)
}

func TestFileProxy_Success(t *testing.T) {
	svc, _, _, _, _ := newTestService()
	ctx := context.Background()

	ballotID := uuid.New()
	grantorID := uuid.New()
	req := gov.FileProxyRequest{
		UnitID:  uuid.New(),
		ProxyID: uuid.New(),
	}

	p, err := svc.FileProxy(ctx, ballotID, req, grantorID)
	require.NoError(t, err)
	require.NotNil(t, p)
	assert.Equal(t, ballotID, p.BallotID)
	assert.Equal(t, grantorID, p.GrantorID)
}

// ---------------------------------------------------------------------------
// Meeting tests
// ---------------------------------------------------------------------------

func TestScheduleMeeting_Success(t *testing.T) {
	svc, _, _, _, _ := newTestService()
	ctx := context.Background()

	orgID := uuid.New()
	createdBy := uuid.New()
	req := gov.CreateMeetingRequest{
		Title:       "Annual HOA Meeting",
		MeetingType: "annual",
		ScheduledAt: time.Now().Add(30 * 24 * time.Hour),
	}

	m, err := svc.ScheduleMeeting(ctx, orgID, req, createdBy)
	require.NoError(t, err)
	require.NotNil(t, m)
	assert.Equal(t, "scheduled", m.Status)
	assert.Equal(t, orgID, m.OrgID)
	assert.Equal(t, createdBy, m.CreatedBy)
	assert.Equal(t, "annual", m.MeetingType)
}

func TestScheduleHearing_Success(t *testing.T) {
	svc, vr, _, _, _ := newTestService()
	ctx := context.Background()

	offNum := int16(1)
	v := &gov.Violation{
		ID: uuid.New(), OrgID: uuid.New(), UnitID: uuid.New(),
		ReportedBy: uuid.New(), Title: "t", Description: "d",
		Category: "c", Status: "open", Severity: 1,
		OffenseNumber: &offNum, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	vr.violations[v.ID] = v

	meetingID := uuid.New()
	h, err := svc.ScheduleHearing(ctx, v.ID, meetingID)
	require.NoError(t, err)
	require.NotNil(t, h)
	assert.Equal(t, v.ID, h.ViolationID)
	assert.Equal(t, meetingID, h.MeetingID)
}

func TestGetHearing_NotFound(t *testing.T) {
	svc, _, _, _, _ := newTestService()
	ctx := context.Background()

	_, err := svc.GetHearing(ctx, uuid.New())
	require.Error(t, err)
	var nfe *api.NotFoundError
	require.ErrorAs(t, err, &nfe)
}

func TestRecordMotion_SetsMotionNumber(t *testing.T) {
	svc, _, _, _, mr := newTestService()
	ctx := context.Background()

	meetingID := uuid.New()
	meeting := &gov.Meeting{
		ID:          meetingID,
		OrgID:       uuid.New(),
		Title:       "Board Meeting",
		MeetingType: "board",
		Status:      "in_progress",
		ScheduledAt: time.Now(),
		CreatedBy:   uuid.New(),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	mr.meetings[meeting.ID] = meeting

	req := gov.RecordMotionRequest{
		Title:   "Approve budget",
		MovedBy: uuid.New(),
	}

	m, err := svc.RecordMotion(ctx, meetingID, req)
	require.NoError(t, err)
	require.NotNil(t, m)
	assert.Equal(t, int16(1), m.MotionNumber)
	assert.Equal(t, "pending", m.Status)

	// Second motion gets number 2.
	req2 := gov.RecordMotionRequest{
		Title:   "Approve minutes",
		MovedBy: uuid.New(),
	}
	m2, err := svc.RecordMotion(ctx, meetingID, req2)
	require.NoError(t, err)
	assert.Equal(t, int16(2), m2.MotionNumber)
}
