package gov

import (
	"net/http"

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
) {
	orgMw := func(h http.HandlerFunc) http.Handler {
		return middleware.Auth(validator, middleware.TenantContext(http.HandlerFunc(h)))
	}

	// Violations (9 routes)
	mux.Handle("POST /api/v1/organizations/{org_id}/violations", orgMw(violationHandler.ReportViolation))
	mux.Handle("GET /api/v1/organizations/{org_id}/violations", orgMw(violationHandler.ListViolations))
	mux.Handle("GET /api/v1/organizations/{org_id}/violations/{violation_id}", orgMw(violationHandler.GetViolation))
	mux.Handle("PATCH /api/v1/organizations/{org_id}/violations/{violation_id}", orgMw(violationHandler.UpdateViolation))
	mux.Handle("POST /api/v1/organizations/{org_id}/violations/{violation_id}/actions", orgMw(violationHandler.AddAction))
	mux.Handle("POST /api/v1/organizations/{org_id}/violations/{violation_id}/verify-cure", orgMw(violationHandler.VerifyCure))
	mux.Handle("POST /api/v1/organizations/{org_id}/violations/{violation_id}/hearing", orgMw(violationHandler.ScheduleHearing))
	mux.Handle("GET /api/v1/organizations/{org_id}/violations/{violation_id}/hearing", orgMw(violationHandler.GetHearing))
	mux.Handle("PATCH /api/v1/organizations/{org_id}/violations/{violation_id}/hearing/{hearing_id}", orgMw(violationHandler.UpdateHearing))

	// ARB Requests (6 routes)
	mux.Handle("POST /api/v1/organizations/{org_id}/arb-requests", orgMw(arbHandler.SubmitARBRequest))
	mux.Handle("GET /api/v1/organizations/{org_id}/arb-requests", orgMw(arbHandler.ListARBRequests))
	mux.Handle("GET /api/v1/organizations/{org_id}/arb-requests/{request_id}", orgMw(arbHandler.GetARBRequest))
	mux.Handle("PATCH /api/v1/organizations/{org_id}/arb-requests/{request_id}", orgMw(arbHandler.UpdateARBRequest))
	mux.Handle("POST /api/v1/organizations/{org_id}/arb-requests/{request_id}/votes", orgMw(arbHandler.CastARBVote))
	mux.Handle("POST /api/v1/organizations/{org_id}/arb-requests/{request_id}/request-revision", orgMw(arbHandler.RequestRevision))

	// Ballots (7 routes)
	mux.Handle("POST /api/v1/organizations/{org_id}/ballots", orgMw(ballotHandler.CreateBallot))
	mux.Handle("GET /api/v1/organizations/{org_id}/ballots", orgMw(ballotHandler.ListBallots))
	mux.Handle("GET /api/v1/organizations/{org_id}/ballots/{ballot_id}", orgMw(ballotHandler.GetBallot))
	mux.Handle("PATCH /api/v1/organizations/{org_id}/ballots/{ballot_id}", orgMw(ballotHandler.UpdateBallot))
	mux.Handle("POST /api/v1/organizations/{org_id}/ballots/{ballot_id}/votes", orgMw(ballotHandler.CastVote))
	mux.Handle("POST /api/v1/organizations/{org_id}/ballots/{ballot_id}/proxy", orgMw(ballotHandler.FileProxy))
	mux.Handle("DELETE /api/v1/organizations/{org_id}/ballots/{ballot_id}/proxy/{proxy_id}", orgMw(ballotHandler.RevokeProxy))

	// Meetings (7 routes)
	mux.Handle("POST /api/v1/organizations/{org_id}/meetings", orgMw(meetingHandler.ScheduleMeeting))
	mux.Handle("GET /api/v1/organizations/{org_id}/meetings", orgMw(meetingHandler.ListMeetings))
	mux.Handle("GET /api/v1/organizations/{org_id}/meetings/{meeting_id}", orgMw(meetingHandler.GetMeeting))
	mux.Handle("PATCH /api/v1/organizations/{org_id}/meetings/{meeting_id}", orgMw(meetingHandler.UpdateMeeting))
	mux.Handle("POST /api/v1/organizations/{org_id}/meetings/{meeting_id}/attendees", orgMw(meetingHandler.AddAttendee))
	mux.Handle("POST /api/v1/organizations/{org_id}/meetings/{meeting_id}/motions", orgMw(meetingHandler.RecordMotion))
	mux.Handle("PATCH /api/v1/organizations/{org_id}/meetings/{meeting_id}/motions/{motion_id}", orgMw(meetingHandler.UpdateMotion))
}
