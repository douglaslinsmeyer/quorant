package com

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/audit"
	"github.com/quorant/quorant/internal/platform/api"
	"github.com/quorant/quorant/internal/platform/queue"
)

// ComService orchestrates all 7 communication repositories.
type ComService struct {
	announcements AnnouncementRepository
	threads       ThreadRepository
	notifications NotificationRepository
	calendar      CalendarRepository
	templates     TemplateRepository
	directory     DirectoryRepository
	commLog       CommLogRepository
	auditor       audit.Auditor
	publisher     queue.Publisher
	logger        *slog.Logger
}

// NewComService constructs a ComService backed by the given repositories.
func NewComService(
	announcements AnnouncementRepository,
	threads ThreadRepository,
	notifications NotificationRepository,
	calendar CalendarRepository,
	templates TemplateRepository,
	directory DirectoryRepository,
	commLog CommLogRepository,
	auditor audit.Auditor,
	publisher queue.Publisher,
	logger *slog.Logger,
) *ComService {
	return &ComService{
		announcements: announcements,
		threads:       threads,
		notifications: notifications,
		calendar:      calendar,
		templates:     templates,
		directory:     directory,
		commLog:       commLog,
		auditor:       auditor,
		publisher:     publisher,
		logger:        logger,
	}
}

// ─── Announcements ────────────────────────────────────────────────────────────

// CreateAnnouncement validates the request and creates a new announcement.
// If no ScheduledFor is provided, PublishedAt is set to now.
func (s *ComService) CreateAnnouncement(ctx context.Context, orgID uuid.UUID, req CreateAnnouncementRequest, authorID uuid.UUID) (*Announcement, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	now := time.Now()
	a := &Announcement{
		OrgID:         orgID,
		AuthorID:      authorID,
		Title:         req.Title,
		Body:          req.Body,
		IsPinned:      req.IsPinned,
		AudienceRoles: req.AudienceRoles,
		ScheduledFor:  req.ScheduledFor,
	}

	if req.ScheduledFor == nil {
		a.PublishedAt = &now
	}

	created, err := s.announcements.Create(ctx, a)
	if err != nil {
		return nil, fmt.Errorf("com service: CreateAnnouncement: %w", err)
	}

	s.logger.InfoContext(ctx, "announcement created", "announcement_id", created.ID, "org_id", orgID)
	return created, nil
}

// ListAnnouncements returns announcements for an organization, supporting cursor-based pagination.
// limit controls the page size; afterID is the cursor from the previous page.
func (s *ComService) ListAnnouncements(ctx context.Context, orgID uuid.UUID, limit int, afterID *uuid.UUID) ([]Announcement, bool, error) {
	result, hasMore, err := s.announcements.ListByOrg(ctx, orgID, limit, afterID)
	if err != nil {
		return nil, false, fmt.Errorf("com service: ListAnnouncements: %w", err)
	}
	return result, hasMore, nil
}

// GetAnnouncement returns an announcement by ID, or a 404 if not found.
func (s *ComService) GetAnnouncement(ctx context.Context, id uuid.UUID) (*Announcement, error) {
	a, err := s.announcements.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("com service: GetAnnouncement: %w", err)
	}
	if a == nil {
		return nil, api.NewNotFoundError("resource.not_found", api.P("resource", "announcement"))
	}
	return a, nil
}

// UpdateAnnouncement persists changes to an existing announcement.
func (s *ComService) UpdateAnnouncement(ctx context.Context, id uuid.UUID, a *Announcement) (*Announcement, error) {
	a.ID = id
	updated, err := s.announcements.Update(ctx, a)
	if err != nil {
		return nil, fmt.Errorf("com service: UpdateAnnouncement: %w", err)
	}
	s.logger.InfoContext(ctx, "announcement updated", "announcement_id", id)
	return updated, nil
}

// DeleteAnnouncement soft-deletes an announcement by ID.
func (s *ComService) DeleteAnnouncement(ctx context.Context, id uuid.UUID) error {
	if err := s.announcements.SoftDelete(ctx, id); err != nil {
		return fmt.Errorf("com service: DeleteAnnouncement: %w", err)
	}
	s.logger.InfoContext(ctx, "announcement deleted", "announcement_id", id)
	return nil
}

// ─── Threads & Messages ───────────────────────────────────────────────────────

// CreateThread validates the request and creates a new message thread.
func (s *ComService) CreateThread(ctx context.Context, orgID uuid.UUID, req CreateThreadRequest, createdBy uuid.UUID) (*Thread, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	t := &Thread{
		OrgID:      orgID,
		Subject:    req.Subject,
		ThreadType: req.ThreadType,
		CreatedBy:  createdBy,
	}

	created, err := s.threads.CreateThread(ctx, t)
	if err != nil {
		return nil, fmt.Errorf("com service: CreateThread: %w", err)
	}

	s.logger.InfoContext(ctx, "thread created", "thread_id", created.ID, "org_id", orgID)
	return created, nil
}

// ListThreads returns all threads for an organization.
func (s *ComService) ListThreads(ctx context.Context, orgID uuid.UUID) ([]Thread, error) {
	result, err := s.threads.ListThreadsByOrg(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("com service: ListThreads: %w", err)
	}
	return result, nil
}

// GetThread returns a thread by ID, or a 404 if not found.
func (s *ComService) GetThread(ctx context.Context, id uuid.UUID) (*Thread, error) {
	t, err := s.threads.FindThreadByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("com service: GetThread: %w", err)
	}
	if t == nil {
		return nil, api.NewNotFoundError("resource.not_found", api.P("resource", "thread"))
	}
	return t, nil
}

// SendMessage validates the request and creates a new message in a thread.
func (s *ComService) SendMessage(ctx context.Context, threadID uuid.UUID, req SendMessageRequest, senderID uuid.UUID) (*Message, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	m := &Message{
		ThreadID: threadID,
		SenderID: senderID,
		Body:     req.Body,
	}

	created, err := s.threads.CreateMessage(ctx, m)
	if err != nil {
		return nil, fmt.Errorf("com service: SendMessage: %w", err)
	}

	s.logger.InfoContext(ctx, "message sent", "message_id", created.ID, "thread_id", threadID)
	return created, nil
}

// ListMessages returns all messages in a thread.
func (s *ComService) ListMessages(ctx context.Context, threadID uuid.UUID) ([]Message, error) {
	result, err := s.threads.ListMessagesByThread(ctx, threadID)
	if err != nil {
		return nil, fmt.Errorf("com service: ListMessages: %w", err)
	}
	return result, nil
}

// EditMessage updates the body of a message and sets EditedAt.
func (s *ComService) EditMessage(ctx context.Context, id uuid.UUID, body string) (*Message, error) {
	now := time.Now()
	m := &Message{
		ID:       id,
		Body:     body,
		EditedAt: &now,
	}

	updated, err := s.threads.UpdateMessage(ctx, m)
	if err != nil {
		return nil, fmt.Errorf("com service: EditMessage: %w", err)
	}

	s.logger.InfoContext(ctx, "message edited", "message_id", id)
	return updated, nil
}

// DeleteMessage soft-deletes a message by ID.
func (s *ComService) DeleteMessage(ctx context.Context, id uuid.UUID) error {
	if err := s.threads.SoftDeleteMessage(ctx, id); err != nil {
		return fmt.Errorf("com service: DeleteMessage: %w", err)
	}
	s.logger.InfoContext(ctx, "message deleted", "message_id", id)
	return nil
}

// ─── Notifications ────────────────────────────────────────────────────────────

// GetNotificationPreferences returns all notification preferences for a user in an org.
func (s *ComService) GetNotificationPreferences(ctx context.Context, userID, orgID uuid.UUID) ([]NotificationPreference, error) {
	result, err := s.notifications.ListPreferencesByUser(ctx, userID, orgID)
	if err != nil {
		return nil, fmt.Errorf("com service: GetNotificationPreferences: %w", err)
	}
	return result, nil
}

// UpdateNotificationPreferences upserts each preference in the slice.
func (s *ComService) UpdateNotificationPreferences(ctx context.Context, prefs []NotificationPreference) error {
	for i := range prefs {
		if _, err := s.notifications.UpsertPreference(ctx, &prefs[i]); err != nil {
			return fmt.Errorf("com service: UpdateNotificationPreferences: %w", err)
		}
	}
	return nil
}

// RegisterPushToken creates a new push token record.
func (s *ComService) RegisterPushToken(ctx context.Context, token *PushToken) (*PushToken, error) {
	created, err := s.notifications.CreatePushToken(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("com service: RegisterPushToken: %w", err)
	}
	s.logger.InfoContext(ctx, "push token registered", "token_id", created.ID, "user_id", created.UserID)
	return created, nil
}

// UnregisterPushToken removes a push token by ID.
func (s *ComService) UnregisterPushToken(ctx context.Context, id uuid.UUID) error {
	if err := s.notifications.DeletePushToken(ctx, id); err != nil {
		return fmt.Errorf("com service: UnregisterPushToken: %w", err)
	}
	s.logger.InfoContext(ctx, "push token unregistered", "token_id", id)
	return nil
}

// ─── Calendar ─────────────────────────────────────────────────────────────────

// CreateCalendarEvent validates the request and creates a new calendar event.
func (s *ComService) CreateCalendarEvent(ctx context.Context, orgID uuid.UUID, req CreateCalendarEventRequest, createdBy uuid.UUID) (*CalendarEvent, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	e := &CalendarEvent{
		OrgID:       orgID,
		Title:       req.Title,
		EventType:   req.EventType,
		StartsAt:    req.StartsAt,
		Description: req.Description,
		Location:    req.Location,
		EndsAt:      req.EndsAt,
		IsAllDay:    req.IsAllDay,
		RSVPEnabled: req.RSVPEnabled,
		CreatedBy:   createdBy,
	}

	created, err := s.calendar.CreateEvent(ctx, e)
	if err != nil {
		return nil, fmt.Errorf("com service: CreateCalendarEvent: %w", err)
	}

	s.logger.InfoContext(ctx, "calendar event created", "event_id", created.ID, "org_id", orgID)
	return created, nil
}

// ListCalendarEvents returns all calendar events for an organization.
func (s *ComService) ListCalendarEvents(ctx context.Context, orgID uuid.UUID) ([]CalendarEvent, error) {
	result, err := s.calendar.ListEventsByOrg(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("com service: ListCalendarEvents: %w", err)
	}
	return result, nil
}

// GetCalendarEvent returns a calendar event by ID, or a 404 if not found.
func (s *ComService) GetCalendarEvent(ctx context.Context, id uuid.UUID) (*CalendarEvent, error) {
	e, err := s.calendar.FindEventByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("com service: GetCalendarEvent: %w", err)
	}
	if e == nil {
		return nil, api.NewNotFoundError("resource.not_found", api.P("resource", "calendar event"))
	}
	return e, nil
}

// UpdateCalendarEvent persists changes to an existing calendar event.
func (s *ComService) UpdateCalendarEvent(ctx context.Context, id uuid.UUID, e *CalendarEvent) (*CalendarEvent, error) {
	e.ID = id
	updated, err := s.calendar.UpdateEvent(ctx, e)
	if err != nil {
		return nil, fmt.Errorf("com service: UpdateCalendarEvent: %w", err)
	}
	s.logger.InfoContext(ctx, "calendar event updated", "event_id", id)
	return updated, nil
}

// DeleteCalendarEvent soft-deletes a calendar event by ID.
func (s *ComService) DeleteCalendarEvent(ctx context.Context, id uuid.UUID) error {
	if err := s.calendar.SoftDeleteEvent(ctx, id); err != nil {
		return fmt.Errorf("com service: DeleteCalendarEvent: %w", err)
	}
	s.logger.InfoContext(ctx, "calendar event deleted", "event_id", id)
	return nil
}

// RSVPToEvent validates the request and records a user's RSVP for a calendar event.
func (s *ComService) RSVPToEvent(ctx context.Context, eventID uuid.UUID, req RSVPRequest, userID uuid.UUID) (*CalendarEventRSVP, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	rsvp := &CalendarEventRSVP{
		EventID:    eventID,
		UserID:     userID,
		Status:     req.Status,
		GuestCount: req.GuestCount,
	}

	created, err := s.calendar.CreateRSVP(ctx, rsvp)
	if err != nil {
		return nil, fmt.Errorf("com service: RSVPToEvent: %w", err)
	}

	s.logger.InfoContext(ctx, "rsvp recorded", "event_id", eventID, "user_id", userID, "status", req.Status)
	return created, nil
}

// ─── Templates ────────────────────────────────────────────────────────────────

// ListTemplates returns all templates for an organization (including system defaults).
func (s *ComService) ListTemplates(ctx context.Context, orgID uuid.UUID) ([]MessageTemplate, error) {
	result, err := s.templates.ListByOrg(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("com service: ListTemplates: %w", err)
	}
	return result, nil
}

// CreateTemplate validates the request and creates a new message template.
func (s *ComService) CreateTemplate(ctx context.Context, orgID uuid.UUID, req CreateTemplateRequest) (*MessageTemplate, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	t := &MessageTemplate{
		OrgID:       &orgID,
		TemplateKey: req.TemplateKey,
		Channel:     req.Channel,
		Locale:      req.Locale,
		Body:        req.Body,
		Subject:     req.Subject,
		IsActive:    true,
	}

	created, err := s.templates.Create(ctx, t)
	if err != nil {
		return nil, fmt.Errorf("com service: CreateTemplate: %w", err)
	}

	s.logger.InfoContext(ctx, "template created", "template_id", created.ID, "org_id", orgID)
	return created, nil
}

// UpdateTemplate persists changes to an existing message template.
func (s *ComService) UpdateTemplate(ctx context.Context, id uuid.UUID, t *MessageTemplate) (*MessageTemplate, error) {
	t.ID = id
	updated, err := s.templates.Update(ctx, t)
	if err != nil {
		return nil, fmt.Errorf("com service: UpdateTemplate: %w", err)
	}
	s.logger.InfoContext(ctx, "template updated", "template_id", id)
	return updated, nil
}

// DeleteTemplate hard-deletes a template by ID (falls back to system default).
func (s *ComService) DeleteTemplate(ctx context.Context, id uuid.UUID) error {
	if err := s.templates.Delete(ctx, id); err != nil {
		return fmt.Errorf("com service: DeleteTemplate: %w", err)
	}
	s.logger.InfoContext(ctx, "template deleted", "template_id", id)
	return nil
}

// ─── Directory ────────────────────────────────────────────────────────────────

// GetDirectoryPreferences returns the directory preferences for a user in an org.
// If no preferences exist, a default (opt-out) preference is returned.
func (s *ComService) GetDirectoryPreferences(ctx context.Context, userID, orgID uuid.UUID) (*DirectoryPreference, error) {
	pref, err := s.directory.FindByUserAndOrg(ctx, userID, orgID)
	if err != nil {
		return nil, fmt.Errorf("com service: GetDirectoryPreferences: %w", err)
	}
	if pref == nil {
		// Return default preferences (opt-out, nothing shown).
		return &DirectoryPreference{
			UserID:    userID,
			OrgID:     orgID,
			OptIn:     false,
			ShowEmail: false,
			ShowPhone: false,
			ShowUnit:  false,
		}, nil
	}
	return pref, nil
}

// UpdateDirectoryPreferences applies partial updates to a user's directory preferences.
func (s *ComService) UpdateDirectoryPreferences(ctx context.Context, userID, orgID uuid.UUID, req UpdateDirectoryPreferenceRequest) (*DirectoryPreference, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	// Fetch or default current preferences.
	pref, err := s.GetDirectoryPreferences(ctx, userID, orgID)
	if err != nil {
		return nil, err
	}

	// Ensure the IDs are set (in case it was a default).
	pref.UserID = userID
	pref.OrgID = orgID

	// Apply partial updates.
	if req.OptIn != nil {
		pref.OptIn = *req.OptIn
	}
	if req.ShowEmail != nil {
		pref.ShowEmail = *req.ShowEmail
	}
	if req.ShowPhone != nil {
		pref.ShowPhone = *req.ShowPhone
	}
	if req.ShowUnit != nil {
		pref.ShowUnit = *req.ShowUnit
	}

	saved, err := s.directory.Upsert(ctx, pref)
	if err != nil {
		return nil, fmt.Errorf("com service: UpdateDirectoryPreferences: %w", err)
	}

	s.logger.InfoContext(ctx, "directory preferences updated", "user_id", userID, "org_id", orgID)
	return saved, nil
}

// ─── Communication Log ────────────────────────────────────────────────────────

// LogCommunication validates the request and creates a new communication log entry.
func (s *ComService) LogCommunication(ctx context.Context, orgID uuid.UUID, req LogCommunicationRequest, initiatedBy uuid.UUID) (*CommunicationLog, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	entry := &CommunicationLog{
		OrgID:         orgID,
		Direction:     req.Direction,
		Channel:       req.Channel,
		ContactUserID: req.ContactUserID,
		ContactName:   req.ContactName,
		Subject:       req.Subject,
		Body:          req.Body,
		InitiatedBy:   &initiatedBy,
		Status:        "sent",
		Source:        "manual",
	}

	created, err := s.commLog.Create(ctx, entry)
	if err != nil {
		return nil, fmt.Errorf("com service: LogCommunication: %w", err)
	}

	s.logger.InfoContext(ctx, "communication logged", "comm_id", created.ID, "org_id", orgID)
	return created, nil
}

// ListCommunications returns all communication log entries for an organization.
func (s *ComService) ListCommunications(ctx context.Context, orgID uuid.UUID) ([]CommunicationLog, error) {
	result, err := s.commLog.ListByOrg(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("com service: ListCommunications: %w", err)
	}
	return result, nil
}

// GetCommunication returns a communication log entry by ID, or a 404 if not found.
func (s *ComService) GetCommunication(ctx context.Context, id uuid.UUID) (*CommunicationLog, error) {
	entry, err := s.commLog.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("com service: GetCommunication: %w", err)
	}
	if entry == nil {
		return nil, api.NewNotFoundError("resource.not_found", api.P("resource", "communication"))
	}
	return entry, nil
}

// UpdateCommunication persists changes to an existing communication log entry.
func (s *ComService) UpdateCommunication(ctx context.Context, id uuid.UUID, entry *CommunicationLog) (*CommunicationLog, error) {
	entry.ID = id
	updated, err := s.commLog.Update(ctx, entry)
	if err != nil {
		return nil, fmt.Errorf("com service: UpdateCommunication: %w", err)
	}
	s.logger.InfoContext(ctx, "communication updated", "comm_id", id)
	return updated, nil
}

// ListUnitCommunications returns all communication log entries for a unit.
func (s *ComService) ListUnitCommunications(ctx context.Context, unitID uuid.UUID) ([]CommunicationLog, error) {
	result, err := s.commLog.ListByUnit(ctx, unitID)
	if err != nil {
		return nil, fmt.Errorf("com service: ListUnitCommunications: %w", err)
	}
	return result, nil
}

// CalendarItem represents a unified calendar entry from any module.
type CalendarItem struct {
	Source   string    `json:"source"`    // "community_event", "meeting", "ballot_deadline", "assessment_due"
	SourceID uuid.UUID `json:"source_id"`
	Title    string    `json:"title"`
	StartsAt time.Time `json:"starts_at"`
	EndsAt   *time.Time `json:"ends_at,omitempty"`
	IsAllDay bool      `json:"is_all_day"`
	Location string    `json:"location,omitempty"`
}

// GetUnifiedCalendar returns a merged view of calendar events for the org.
// For now, this returns community events from the calendar module.
// Future: aggregate meetings, ballot deadlines, assessment due dates, reservations.
func (s *ComService) GetUnifiedCalendar(ctx context.Context, orgID uuid.UUID) ([]CalendarItem, error) {
	events, err := s.calendar.ListEventsByOrg(ctx, orgID)
	if err != nil {
		return nil, err
	}

	items := make([]CalendarItem, 0, len(events))
	for _, e := range events {
		items = append(items, CalendarItem{
			Source:   "community_event",
			SourceID: e.ID,
			Title:    e.Title,
			StartsAt: e.StartsAt,
			EndsAt:   e.EndsAt,
			IsAllDay: e.IsAllDay,
			Location: func() string { if e.Location != nil { return *e.Location }; return "" }(),
		})
	}

	return items, nil
}
