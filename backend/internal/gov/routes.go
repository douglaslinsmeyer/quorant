package gov

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/platform/auth"
	"github.com/quorant/quorant/internal/platform/middleware"
)

func RegisterRoutes(
	mux *http.ServeMux,
	violationHandler *ViolationHandler,
	arbHandler *ARBHandler,
	ballotHandler *BallotHandler,
	meetingHandler *MeetingHandler,
	validator auth.TokenValidator,
	checker middleware.PermissionChecker,
	resolveUserID func(context.Context) (uuid.UUID, error),
) {
	permMw := func(perm string, h http.HandlerFunc) http.Handler {
		return middleware.Auth(validator,
			middleware.TenantContext(
				middleware.RequirePermission(checker, perm, resolveUserID)(
					http.HandlerFunc(h))))
	}

	// Violations (9 routes)
	mux.Handle("POST /api/v1/organizations/{org_id}/violations", permMw("gov.violation.create", violationHandler.ReportViolation))
	mux.Handle("GET /api/v1/organizations/{org_id}/violations", permMw("gov.violation.read", violationHandler.ListViolations))
	mux.Handle("GET /api/v1/organizations/{org_id}/violations/{violation_id}", permMw("gov.violation.read", violationHandler.GetViolation))
	mux.Handle("PATCH /api/v1/organizations/{org_id}/violations/{violation_id}", permMw("gov.violation.manage", violationHandler.UpdateViolation))
	mux.Handle("POST /api/v1/organizations/{org_id}/violations/{violation_id}/actions", permMw("gov.violation.manage", violationHandler.AddAction))
	mux.Handle("POST /api/v1/organizations/{org_id}/violations/{violation_id}/verify-cure", permMw("gov.violation.manage", violationHandler.VerifyCure))
	mux.Handle("POST /api/v1/organizations/{org_id}/violations/{violation_id}/hearing", permMw("gov.hearing.manage", violationHandler.ScheduleHearing))
	mux.Handle("GET /api/v1/organizations/{org_id}/violations/{violation_id}/hearing", permMw("gov.violation.read", violationHandler.GetHearing))
	mux.Handle("PATCH /api/v1/organizations/{org_id}/violations/{violation_id}/hearing/{hearing_id}", permMw("gov.hearing.manage", violationHandler.UpdateHearing))

	// ARB Requests (6 routes)
	mux.Handle("POST /api/v1/organizations/{org_id}/arb-requests", permMw("gov.arb.submit", arbHandler.SubmitARBRequest))
	mux.Handle("GET /api/v1/organizations/{org_id}/arb-requests", permMw("gov.arb.read", arbHandler.ListARBRequests))
	mux.Handle("GET /api/v1/organizations/{org_id}/arb-requests/{request_id}", permMw("gov.arb.read", arbHandler.GetARBRequest))
	mux.Handle("PATCH /api/v1/organizations/{org_id}/arb-requests/{request_id}", permMw("gov.arb.review", arbHandler.UpdateARBRequest))
	mux.Handle("POST /api/v1/organizations/{org_id}/arb-requests/{request_id}/votes", permMw("gov.arb.review", arbHandler.CastARBVote))
	mux.Handle("POST /api/v1/organizations/{org_id}/arb-requests/{request_id}/request-revision", permMw("gov.arb.review", arbHandler.RequestRevision))

	// Ballots (7 routes)
	mux.Handle("POST /api/v1/organizations/{org_id}/ballots", permMw("gov.ballot.create", ballotHandler.CreateBallot))
	mux.Handle("GET /api/v1/organizations/{org_id}/ballots", permMw("gov.ballot.read", ballotHandler.ListBallots))
	mux.Handle("GET /api/v1/organizations/{org_id}/ballots/{ballot_id}", permMw("gov.ballot.read", ballotHandler.GetBallot))
	mux.Handle("PATCH /api/v1/organizations/{org_id}/ballots/{ballot_id}", permMw("gov.ballot.create", ballotHandler.UpdateBallot))
	mux.Handle("POST /api/v1/organizations/{org_id}/ballots/{ballot_id}/votes", permMw("gov.ballot.vote", ballotHandler.CastVote))
	mux.Handle("POST /api/v1/organizations/{org_id}/ballots/{ballot_id}/proxy", permMw("gov.proxy.file", ballotHandler.FileProxy))
	mux.Handle("DELETE /api/v1/organizations/{org_id}/ballots/{ballot_id}/proxy/{proxy_id}", permMw("gov.proxy.file", ballotHandler.RevokeProxy))

	// Meetings (7 routes)
	mux.Handle("POST /api/v1/organizations/{org_id}/meetings", permMw("gov.meeting.create", meetingHandler.ScheduleMeeting))
	mux.Handle("GET /api/v1/organizations/{org_id}/meetings", permMw("gov.meeting.read", meetingHandler.ListMeetings))
	mux.Handle("GET /api/v1/organizations/{org_id}/meetings/{meeting_id}", permMw("gov.meeting.read", meetingHandler.GetMeeting))
	mux.Handle("PATCH /api/v1/organizations/{org_id}/meetings/{meeting_id}", permMw("gov.meeting.create", meetingHandler.UpdateMeeting))
	mux.Handle("POST /api/v1/organizations/{org_id}/meetings/{meeting_id}/attendees", permMw("gov.meeting.create", meetingHandler.AddAttendee))
	mux.Handle("POST /api/v1/organizations/{org_id}/meetings/{meeting_id}/motions", permMw("gov.motion.create", meetingHandler.RecordMotion))
	mux.Handle("PATCH /api/v1/organizations/{org_id}/meetings/{meeting_id}/motions/{motion_id}", permMw("gov.motion.create", meetingHandler.UpdateMotion))
}
