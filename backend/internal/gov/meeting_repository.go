package gov

import (
	"context"

	"github.com/google/uuid"
)

// MeetingRepository persists and retrieves meetings, attendees, motions, and hearing links.
type MeetingRepository interface {
	Create(ctx context.Context, m *Meeting) (*Meeting, error)
	FindByID(ctx context.Context, id uuid.UUID) (*Meeting, error)
	ListByOrg(ctx context.Context, orgID uuid.UUID) ([]Meeting, error)
	Update(ctx context.Context, m *Meeting) (*Meeting, error)
	SoftDelete(ctx context.Context, id uuid.UUID) error

	AddAttendee(ctx context.Context, a *MeetingAttendee) (*MeetingAttendee, error)
	ListAttendeesByMeeting(ctx context.Context, meetingID uuid.UUID) ([]MeetingAttendee, error)

	CreateMotion(ctx context.Context, m *MeetingMotion) (*MeetingMotion, error)
	UpdateMotion(ctx context.Context, m *MeetingMotion) (*MeetingMotion, error)
	ListMotionsByMeeting(ctx context.Context, meetingID uuid.UUID) ([]MeetingMotion, error)

	CreateHearingLink(ctx context.Context, h *HearingLink) (*HearingLink, error)
	FindHearingByViolation(ctx context.Context, violationID uuid.UUID) (*HearingLink, error)
	UpdateHearingLink(ctx context.Context, h *HearingLink) (*HearingLink, error)
}
