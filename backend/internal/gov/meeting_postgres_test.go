//go:build integration

package gov_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/gov"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestMeeting builds a minimal Meeting for insertion.
func newTestMeeting(f govTestFixture) *gov.Meeting {
	return &gov.Meeting{
		OrgID:       f.orgID,
		Title:       "Annual General Meeting",
		MeetingType: "annual",
		Status:      "scheduled",
		ScheduledAt: time.Now().UTC().Add(7 * 24 * time.Hour),
		IsVirtual:   false,
		CreatedBy:   f.userID,
	}
}

// ─── TestCreateMeeting + FindByID + ListByOrg ─────────────────────────────────

func TestCreateMeeting(t *testing.T) {
	f := setupGovFixture(t)
	repo := gov.NewPostgresMeetingRepository(f.pool)
	ctx := context.Background()

	input := newTestMeeting(f)
	got, err := repo.Create(ctx, input)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.NotEqual(t, uuid.Nil, got.ID)
	assert.Equal(t, f.orgID, got.OrgID)
	assert.Equal(t, "Annual General Meeting", got.Title)
	assert.Equal(t, "annual", got.MeetingType)
	assert.Equal(t, "scheduled", got.Status)
	assert.False(t, got.IsVirtual)
	assert.Equal(t, f.userID, got.CreatedBy)
	assert.NotNil(t, got.Metadata)
	assert.False(t, got.CreatedAt.IsZero())
	assert.False(t, got.UpdatedAt.IsZero())
	assert.Nil(t, got.DeletedAt)
}

func TestFindMeetingByID_Found(t *testing.T) {
	f := setupGovFixture(t)
	repo := gov.NewPostgresMeetingRepository(f.pool)
	ctx := context.Background()

	created, err := repo.Create(ctx, newTestMeeting(f))
	require.NoError(t, err)

	got, err := repo.FindByID(ctx, created.ID)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, created.ID, got.ID)
	assert.Equal(t, "Annual General Meeting", got.Title)
}

func TestFindMeetingByID_NotFound(t *testing.T) {
	f := setupGovFixture(t)
	repo := gov.NewPostgresMeetingRepository(f.pool)
	ctx := context.Background()

	got, err := repo.FindByID(ctx, uuid.New())

	require.NoError(t, err)
	assert.Nil(t, got, "should return nil for unknown ID")
}

func TestListMeetingsByOrg(t *testing.T) {
	f := setupGovFixture(t)
	repo := gov.NewPostgresMeetingRepository(f.pool)
	ctx := context.Background()

	_, err := repo.Create(ctx, newTestMeeting(f))
	require.NoError(t, err)
	_, err = repo.Create(ctx, newTestMeeting(f))
	require.NoError(t, err)

	list, err := repo.ListByOrg(ctx, f.orgID)

	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(list), 2)
	for _, m := range list {
		assert.Equal(t, f.orgID, m.OrgID)
	}
}

func TestListMeetingsByOrg_EmptyForUnknown(t *testing.T) {
	f := setupGovFixture(t)
	repo := gov.NewPostgresMeetingRepository(f.pool)
	ctx := context.Background()

	list, err := repo.ListByOrg(ctx, uuid.New())

	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestSoftDeleteMeeting(t *testing.T) {
	f := setupGovFixture(t)
	repo := gov.NewPostgresMeetingRepository(f.pool)
	ctx := context.Background()

	created, err := repo.Create(ctx, newTestMeeting(f))
	require.NoError(t, err)

	err = repo.SoftDelete(ctx, created.ID)
	require.NoError(t, err)

	got, err := repo.FindByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Nil(t, got, "soft-deleted meeting should not be returned by FindByID")
}

// ─── TestAddAttendee + ListAttendees ──────────────────────────────────────────

func TestAddAttendee(t *testing.T) {
	f := setupGovFixture(t)
	repo := gov.NewPostgresMeetingRepository(f.pool)
	ctx := context.Background()

	meeting, err := repo.Create(ctx, newTestMeeting(f))
	require.NoError(t, err)

	rsvp := "accepted"
	attended := true
	attendee := &gov.MeetingAttendee{
		MeetingID:  meeting.ID,
		UserID:     f.userID,
		Role:       "board",
		RSVPStatus: &rsvp,
		Attended:   &attended,
	}

	got, err := repo.AddAttendee(ctx, attendee)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.NotEqual(t, uuid.Nil, got.ID)
	assert.Equal(t, meeting.ID, got.MeetingID)
	assert.Equal(t, f.userID, got.UserID)
	assert.Equal(t, "board", got.Role)
	assert.Equal(t, &rsvp, got.RSVPStatus)
	assert.Equal(t, &attended, got.Attended)
}

func TestListAttendeesByMeeting(t *testing.T) {
	f := setupGovFixture(t)
	repo := gov.NewPostgresMeetingRepository(f.pool)
	ctx := context.Background()

	meeting, err := repo.Create(ctx, newTestMeeting(f))
	require.NoError(t, err)

	// Create a second attendee.
	var user2ID uuid.UUID
	err = f.pool.QueryRow(ctx,
		`INSERT INTO users (idp_user_id, email, display_name)
		 VALUES ($1, $2, $3) RETURNING id`,
		"test-idp-attendee2-"+uuid.New().String(),
		"attendee2-"+uuid.New().String()+"@example.com",
		"Attendee Two",
	).Scan(&user2ID)
	require.NoError(t, err)

	_, err = repo.AddAttendee(ctx, &gov.MeetingAttendee{
		MeetingID: meeting.ID,
		UserID:    f.userID,
		Role:      "board",
	})
	require.NoError(t, err)

	_, err = repo.AddAttendee(ctx, &gov.MeetingAttendee{
		MeetingID: meeting.ID,
		UserID:    user2ID,
		Role:      "member",
	})
	require.NoError(t, err)

	attendees, err := repo.ListAttendeesByMeeting(ctx, meeting.ID)

	require.NoError(t, err)
	require.Len(t, attendees, 2)
	for _, a := range attendees {
		assert.Equal(t, meeting.ID, a.MeetingID)
	}
}

// ─── TestCreateMotion + ListMotions (ordered by motion_number) ────────────────

func TestCreateMotion(t *testing.T) {
	f := setupGovFixture(t)
	repo := gov.NewPostgresMeetingRepository(f.pool)
	ctx := context.Background()

	meeting, err := repo.Create(ctx, newTestMeeting(f))
	require.NoError(t, err)

	desc := "Approve the FY2026 operating budget as presented"
	motion := &gov.MeetingMotion{
		MeetingID:    meeting.ID,
		MotionNumber: 1,
		Title:        "Approve FY2026 Budget",
		Description:  &desc,
		MovedBy:      f.userID,
		Status:       "pending",
	}

	got, err := repo.CreateMotion(ctx, motion)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.NotEqual(t, uuid.Nil, got.ID)
	assert.Equal(t, meeting.ID, got.MeetingID)
	assert.Equal(t, int16(1), got.MotionNumber)
	assert.Equal(t, "Approve FY2026 Budget", got.Title)
	assert.Equal(t, &desc, got.Description)
	assert.Equal(t, f.userID, got.MovedBy)
	assert.Equal(t, "pending", got.Status)
	assert.False(t, got.CreatedAt.IsZero())
}

func TestListMotionsByMeeting_OrderedByMotionNumber(t *testing.T) {
	f := setupGovFixture(t)
	repo := gov.NewPostgresMeetingRepository(f.pool)
	ctx := context.Background()

	meeting, err := repo.Create(ctx, newTestMeeting(f))
	require.NoError(t, err)

	// Insert motions out of order to verify sort.
	_, err = repo.CreateMotion(ctx, &gov.MeetingMotion{
		MeetingID:    meeting.ID,
		MotionNumber: 3,
		Title:        "Motion Three",
		MovedBy:      f.userID,
		Status:       "pending",
	})
	require.NoError(t, err)

	_, err = repo.CreateMotion(ctx, &gov.MeetingMotion{
		MeetingID:    meeting.ID,
		MotionNumber: 1,
		Title:        "Motion One",
		MovedBy:      f.userID,
		Status:       "pending",
	})
	require.NoError(t, err)

	_, err = repo.CreateMotion(ctx, &gov.MeetingMotion{
		MeetingID:    meeting.ID,
		MotionNumber: 2,
		Title:        "Motion Two",
		MovedBy:      f.userID,
		Status:       "pending",
	})
	require.NoError(t, err)

	motions, err := repo.ListMotionsByMeeting(ctx, meeting.ID)

	require.NoError(t, err)
	require.Len(t, motions, 3)
	assert.Equal(t, int16(1), motions[0].MotionNumber)
	assert.Equal(t, int16(2), motions[1].MotionNumber)
	assert.Equal(t, int16(3), motions[2].MotionNumber)
}

// ─── TestUpdateMotion ─────────────────────────────────────────────────────────

func TestUpdateMotion_RecordVoteResults(t *testing.T) {
	f := setupGovFixture(t)
	repo := gov.NewPostgresMeetingRepository(f.pool)
	ctx := context.Background()

	meeting, err := repo.Create(ctx, newTestMeeting(f))
	require.NoError(t, err)

	motion, err := repo.CreateMotion(ctx, &gov.MeetingMotion{
		MeetingID:    meeting.ID,
		MotionNumber: 1,
		Title:        "Approve Annual Budget",
		MovedBy:      f.userID,
		Status:       "pending",
	})
	require.NoError(t, err)

	// Record vote results.
	votesFor := int16(5)
	votesAgainst := int16(1)
	votesAbstain := int16(0)
	resultNotes := "Motion passed with 5 in favor"
	motion.Status = "passed"
	motion.VotesFor = &votesFor
	motion.VotesAgainst = &votesAgainst
	motion.VotesAbstain = &votesAbstain
	motion.ResultNotes = &resultNotes

	updated, err := repo.UpdateMotion(ctx, motion)

	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, motion.ID, updated.ID)
	assert.Equal(t, "passed", updated.Status)
	assert.Equal(t, &votesFor, updated.VotesFor)
	assert.Equal(t, &votesAgainst, updated.VotesAgainst)
	assert.Equal(t, &votesAbstain, updated.VotesAbstain)
	assert.Equal(t, &resultNotes, updated.ResultNotes)
}

// ─── TestCreateHearingLink + FindByViolation ──────────────────────────────────

func TestCreateHearingLink(t *testing.T) {
	f := setupGovFixture(t)
	meetingRepo := gov.NewPostgresMeetingRepository(f.pool)
	violationRepo := gov.NewPostgresViolationRepository(f.pool)
	ctx := context.Background()

	meeting, err := meetingRepo.Create(ctx, newTestMeeting(f))
	require.NoError(t, err)

	violation, err := violationRepo.Create(ctx, newTestViolation(f))
	require.NoError(t, err)

	notifiedAt := time.Now().UTC()
	hearing := &gov.HearingLink{
		MeetingID:           meeting.ID,
		ViolationID:         violation.ID,
		HomeownerNotifiedAt: &notifiedAt,
	}

	got, err := meetingRepo.CreateHearingLink(ctx, hearing)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.NotEqual(t, uuid.Nil, got.ID)
	assert.Equal(t, meeting.ID, got.MeetingID)
	assert.Equal(t, violation.ID, got.ViolationID)
	assert.NotNil(t, got.HomeownerNotifiedAt)
	assert.False(t, got.CreatedAt.IsZero())
}

func TestFindHearingByViolation(t *testing.T) {
	f := setupGovFixture(t)
	meetingRepo := gov.NewPostgresMeetingRepository(f.pool)
	violationRepo := gov.NewPostgresViolationRepository(f.pool)
	ctx := context.Background()

	meeting, err := meetingRepo.Create(ctx, newTestMeeting(f))
	require.NoError(t, err)

	violation, err := violationRepo.Create(ctx, newTestViolation(f))
	require.NoError(t, err)

	created, err := meetingRepo.CreateHearingLink(ctx, &gov.HearingLink{
		MeetingID:   meeting.ID,
		ViolationID: violation.ID,
	})
	require.NoError(t, err)

	got, err := meetingRepo.FindHearingByViolation(ctx, violation.ID)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, created.ID, got.ID)
	assert.Equal(t, meeting.ID, got.MeetingID)
	assert.Equal(t, violation.ID, got.ViolationID)
}

func TestFindHearingByViolation_NotFound(t *testing.T) {
	f := setupGovFixture(t)
	repo := gov.NewPostgresMeetingRepository(f.pool)
	ctx := context.Background()

	got, err := repo.FindHearingByViolation(ctx, uuid.New())

	require.NoError(t, err)
	assert.Nil(t, got, "should return nil when no hearing link exists for violation")
}

func TestUpdateHearingLink(t *testing.T) {
	f := setupGovFixture(t)
	meetingRepo := gov.NewPostgresMeetingRepository(f.pool)
	violationRepo := gov.NewPostgresViolationRepository(f.pool)
	ctx := context.Background()

	meeting, err := meetingRepo.Create(ctx, newTestMeeting(f))
	require.NoError(t, err)

	violation, err := violationRepo.Create(ctx, newTestViolation(f))
	require.NoError(t, err)

	hearing, err := meetingRepo.CreateHearingLink(ctx, &gov.HearingLink{
		MeetingID:   meeting.ID,
		ViolationID: violation.ID,
	})
	require.NoError(t, err)

	// Record the board finding.
	attended := true
	boardFinding := "Violation confirmed, fine upheld"
	fineUpheld := true
	hearing.HomeownerAttended = &attended
	hearing.BoardFinding = &boardFinding
	hearing.FineUpheld = &fineUpheld

	updated, err := meetingRepo.UpdateHearingLink(ctx, hearing)

	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, hearing.ID, updated.ID)
	assert.Equal(t, &attended, updated.HomeownerAttended)
	assert.Equal(t, &boardFinding, updated.BoardFinding)
	assert.Equal(t, &fineUpheld, updated.FineUpheld)
}
