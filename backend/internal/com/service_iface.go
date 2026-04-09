package com

import (
	"context"

	"github.com/google/uuid"
)

// Service defines the business operations for the communications module.
// Handlers depend on this interface rather than the concrete ComService struct.
type Service interface {
	// Announcements
	CreateAnnouncement(ctx context.Context, orgID uuid.UUID, req CreateAnnouncementRequest, authorID uuid.UUID) (*Announcement, error)
	ListAnnouncements(ctx context.Context, orgID uuid.UUID, limit int, afterID *uuid.UUID) ([]Announcement, bool, error)
	GetAnnouncement(ctx context.Context, id uuid.UUID) (*Announcement, error)
	UpdateAnnouncement(ctx context.Context, id uuid.UUID, a *Announcement) (*Announcement, error)
	DeleteAnnouncement(ctx context.Context, id uuid.UUID) error

	// Threads and Messages
	CreateThread(ctx context.Context, orgID uuid.UUID, req CreateThreadRequest, createdBy uuid.UUID) (*Thread, error)
	ListThreads(ctx context.Context, orgID uuid.UUID) ([]Thread, error)
	GetThread(ctx context.Context, id uuid.UUID) (*Thread, error)
	SendMessage(ctx context.Context, threadID uuid.UUID, req SendMessageRequest, senderID uuid.UUID) (*Message, error)
	ListMessages(ctx context.Context, threadID uuid.UUID) ([]Message, error)
	EditMessage(ctx context.Context, id uuid.UUID, body string) (*Message, error)
	DeleteMessage(ctx context.Context, id uuid.UUID) error

	// Notifications
	GetNotificationPreferences(ctx context.Context, userID, orgID uuid.UUID) ([]NotificationPreference, error)
	UpdateNotificationPreferences(ctx context.Context, prefs []NotificationPreference) error
	RegisterPushToken(ctx context.Context, token *PushToken) (*PushToken, error)
	UnregisterPushToken(ctx context.Context, id uuid.UUID) error

	// Calendar
	CreateCalendarEvent(ctx context.Context, orgID uuid.UUID, req CreateCalendarEventRequest, createdBy uuid.UUID) (*CalendarEvent, error)
	ListCalendarEvents(ctx context.Context, orgID uuid.UUID) ([]CalendarEvent, error)
	GetCalendarEvent(ctx context.Context, id uuid.UUID) (*CalendarEvent, error)
	UpdateCalendarEvent(ctx context.Context, id uuid.UUID, e *CalendarEvent) (*CalendarEvent, error)
	DeleteCalendarEvent(ctx context.Context, id uuid.UUID) error
	RSVPToEvent(ctx context.Context, eventID uuid.UUID, req RSVPRequest, userID uuid.UUID) (*CalendarEventRSVP, error)
	GetUnifiedCalendar(ctx context.Context, orgID uuid.UUID) ([]CalendarItem, error)

	// Templates
	ListTemplates(ctx context.Context, orgID uuid.UUID) ([]MessageTemplate, error)
	CreateTemplate(ctx context.Context, orgID uuid.UUID, req CreateTemplateRequest) (*MessageTemplate, error)
	UpdateTemplate(ctx context.Context, id uuid.UUID, t *MessageTemplate) (*MessageTemplate, error)
	DeleteTemplate(ctx context.Context, id uuid.UUID) error

	// Directory Preferences
	GetDirectoryPreferences(ctx context.Context, userID, orgID uuid.UUID) (*DirectoryPreference, error)
	UpdateDirectoryPreferences(ctx context.Context, userID, orgID uuid.UUID, req UpdateDirectoryPreferenceRequest) (*DirectoryPreference, error)

	// Communication Log
	LogCommunication(ctx context.Context, orgID uuid.UUID, req LogCommunicationRequest, initiatedBy uuid.UUID) (*CommunicationLog, error)
	ListCommunications(ctx context.Context, orgID uuid.UUID) ([]CommunicationLog, error)
	GetCommunication(ctx context.Context, id uuid.UUID) (*CommunicationLog, error)
	UpdateCommunication(ctx context.Context, id uuid.UUID, entry *CommunicationLog) (*CommunicationLog, error)
	ListUnitCommunications(ctx context.Context, unitID uuid.UUID) ([]CommunicationLog, error)
}
