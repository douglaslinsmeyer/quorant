package com_test

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/audit"
	"github.com/quorant/quorant/internal/com"
	"github.com/quorant/quorant/internal/platform/api"
	"github.com/quorant/quorant/internal/platform/queue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Mock repositories ────────────────────────────────────────────────────────

// mockAnnouncementRepo is an in-memory AnnouncementRepository.
type mockAnnouncementRepo struct {
	items     map[uuid.UUID]*com.Announcement
	createErr error
	findErr   error
	updateErr error
	deleteErr error
}

func newMockAnnouncementRepo() *mockAnnouncementRepo {
	return &mockAnnouncementRepo{items: make(map[uuid.UUID]*com.Announcement)}
}

func (m *mockAnnouncementRepo) Create(_ context.Context, a *com.Announcement) (*com.Announcement, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	a.CreatedAt = time.Now()
	a.UpdatedAt = time.Now()
	cp := *a
	m.items[a.ID] = &cp
	return &cp, nil
}

func (m *mockAnnouncementRepo) FindByID(_ context.Context, id uuid.UUID) (*com.Announcement, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	a, ok := m.items[id]
	if !ok {
		return nil, nil
	}
	cp := *a
	return &cp, nil
}

func (m *mockAnnouncementRepo) ListByOrg(_ context.Context, orgID uuid.UUID) ([]com.Announcement, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	var result []com.Announcement
	for _, a := range m.items {
		if a.OrgID == orgID {
			result = append(result, *a)
		}
	}
	return result, nil
}

func (m *mockAnnouncementRepo) Update(_ context.Context, a *com.Announcement) (*com.Announcement, error) {
	if m.updateErr != nil {
		return nil, m.updateErr
	}
	a.UpdatedAt = time.Now()
	cp := *a
	m.items[a.ID] = &cp
	return &cp, nil
}

func (m *mockAnnouncementRepo) SoftDelete(_ context.Context, id uuid.UUID) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	delete(m.items, id)
	return nil
}

// mockThreadRepo is an in-memory ThreadRepository.
type mockThreadRepo struct {
	threads   map[uuid.UUID]*com.Thread
	messages  map[uuid.UUID]*com.Message
	createErr error
	findErr   error
}

func newMockThreadRepo() *mockThreadRepo {
	return &mockThreadRepo{
		threads:  make(map[uuid.UUID]*com.Thread),
		messages: make(map[uuid.UUID]*com.Message),
	}
}

func (m *mockThreadRepo) CreateThread(_ context.Context, t *com.Thread) (*com.Thread, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	t.CreatedAt = time.Now()
	t.UpdatedAt = time.Now()
	cp := *t
	m.threads[t.ID] = &cp
	return &cp, nil
}

func (m *mockThreadRepo) FindThreadByID(_ context.Context, id uuid.UUID) (*com.Thread, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	t, ok := m.threads[id]
	if !ok {
		return nil, nil
	}
	cp := *t
	return &cp, nil
}

func (m *mockThreadRepo) ListThreadsByOrg(_ context.Context, orgID uuid.UUID) ([]com.Thread, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	var result []com.Thread
	for _, t := range m.threads {
		if t.OrgID == orgID {
			result = append(result, *t)
		}
	}
	return result, nil
}

func (m *mockThreadRepo) CreateMessage(_ context.Context, msg *com.Message) (*com.Message, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	if msg.ID == uuid.Nil {
		msg.ID = uuid.New()
	}
	msg.CreatedAt = time.Now()
	cp := *msg
	m.messages[msg.ID] = &cp
	return &cp, nil
}

func (m *mockThreadRepo) ListMessagesByThread(_ context.Context, threadID uuid.UUID) ([]com.Message, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	var result []com.Message
	for _, msg := range m.messages {
		if msg.ThreadID == threadID {
			result = append(result, *msg)
		}
	}
	return result, nil
}

func (m *mockThreadRepo) UpdateMessage(_ context.Context, msg *com.Message) (*com.Message, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	cp := *msg
	m.messages[msg.ID] = &cp
	return &cp, nil
}

func (m *mockThreadRepo) SoftDeleteMessage(_ context.Context, id uuid.UUID) error {
	if m.createErr != nil {
		return m.createErr
	}
	delete(m.messages, id)
	return nil
}

// mockNotificationRepo is an in-memory NotificationRepository.
type mockNotificationRepo struct {
	prefs      map[string]*com.NotificationPreference
	tokens     map[uuid.UUID]*com.PushToken
	upsertErr  error
	createErr  error
	deleteErr  error
}

func newMockNotificationRepo() *mockNotificationRepo {
	return &mockNotificationRepo{
		prefs:  make(map[string]*com.NotificationPreference),
		tokens: make(map[uuid.UUID]*com.PushToken),
	}
}

func (m *mockNotificationRepo) UpsertPreference(_ context.Context, p *com.NotificationPreference) (*com.NotificationPreference, error) {
	if m.upsertErr != nil {
		return nil, m.upsertErr
	}
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	key := p.UserID.String() + ":" + p.OrgID.String() + ":" + p.Channel + ":" + p.EventType
	cp := *p
	m.prefs[key] = &cp
	return &cp, nil
}

func (m *mockNotificationRepo) ListPreferencesByUser(_ context.Context, userID uuid.UUID, orgID uuid.UUID) ([]com.NotificationPreference, error) {
	var result []com.NotificationPreference
	for _, p := range m.prefs {
		if p.UserID == userID && p.OrgID == orgID {
			result = append(result, *p)
		}
	}
	return result, nil
}

func (m *mockNotificationRepo) CreatePushToken(_ context.Context, t *com.PushToken) (*com.PushToken, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	t.CreatedAt = time.Now()
	cp := *t
	m.tokens[t.ID] = &cp
	return &cp, nil
}

func (m *mockNotificationRepo) DeletePushToken(_ context.Context, id uuid.UUID) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	delete(m.tokens, id)
	return nil
}

func (m *mockNotificationRepo) ListPushTokensByUser(_ context.Context, userID uuid.UUID) ([]com.PushToken, error) {
	var result []com.PushToken
	for _, t := range m.tokens {
		if t.UserID == userID {
			result = append(result, *t)
		}
	}
	return result, nil
}

// mockCalendarRepo is an in-memory CalendarRepository.
type mockCalendarRepo struct {
	events    map[uuid.UUID]*com.CalendarEvent
	rsvps     map[uuid.UUID]*com.CalendarEventRSVP
	createErr error
	findErr   error
	updateErr error
	deleteErr error
}

func newMockCalendarRepo() *mockCalendarRepo {
	return &mockCalendarRepo{
		events: make(map[uuid.UUID]*com.CalendarEvent),
		rsvps:  make(map[uuid.UUID]*com.CalendarEventRSVP),
	}
}

func (m *mockCalendarRepo) CreateEvent(_ context.Context, e *com.CalendarEvent) (*com.CalendarEvent, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	e.CreatedAt = time.Now()
	e.UpdatedAt = time.Now()
	cp := *e
	m.events[e.ID] = &cp
	return &cp, nil
}

func (m *mockCalendarRepo) FindEventByID(_ context.Context, id uuid.UUID) (*com.CalendarEvent, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	e, ok := m.events[id]
	if !ok {
		return nil, nil
	}
	cp := *e
	return &cp, nil
}

func (m *mockCalendarRepo) ListEventsByOrg(_ context.Context, orgID uuid.UUID) ([]com.CalendarEvent, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	var result []com.CalendarEvent
	for _, e := range m.events {
		if e.OrgID == orgID {
			result = append(result, *e)
		}
	}
	return result, nil
}

func (m *mockCalendarRepo) UpdateEvent(_ context.Context, e *com.CalendarEvent) (*com.CalendarEvent, error) {
	if m.updateErr != nil {
		return nil, m.updateErr
	}
	e.UpdatedAt = time.Now()
	cp := *e
	m.events[e.ID] = &cp
	return &cp, nil
}

func (m *mockCalendarRepo) SoftDeleteEvent(_ context.Context, id uuid.UUID) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	delete(m.events, id)
	return nil
}

func (m *mockCalendarRepo) CreateRSVP(_ context.Context, r *com.CalendarEventRSVP) (*com.CalendarEventRSVP, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	r.CreatedAt = time.Now()
	r.UpdatedAt = time.Now()
	cp := *r
	m.rsvps[r.ID] = &cp
	return &cp, nil
}

func (m *mockCalendarRepo) ListRSVPsByEvent(_ context.Context, eventID uuid.UUID) ([]com.CalendarEventRSVP, error) {
	var result []com.CalendarEventRSVP
	for _, r := range m.rsvps {
		if r.EventID == eventID {
			result = append(result, *r)
		}
	}
	return result, nil
}

// mockTemplateRepo is an in-memory TemplateRepository.
type mockTemplateRepo struct {
	items     map[uuid.UUID]*com.MessageTemplate
	createErr error
	updateErr error
	deleteErr error
}

func newMockTemplateRepo() *mockTemplateRepo {
	return &mockTemplateRepo{items: make(map[uuid.UUID]*com.MessageTemplate)}
}

func (m *mockTemplateRepo) Create(_ context.Context, t *com.MessageTemplate) (*com.MessageTemplate, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	t.CreatedAt = time.Now()
	t.UpdatedAt = time.Now()
	cp := *t
	m.items[t.ID] = &cp
	return &cp, nil
}

func (m *mockTemplateRepo) ListByOrg(_ context.Context, orgID uuid.UUID) ([]com.MessageTemplate, error) {
	var result []com.MessageTemplate
	for _, t := range m.items {
		if t.OrgID != nil && *t.OrgID == orgID {
			result = append(result, *t)
		}
	}
	return result, nil
}

func (m *mockTemplateRepo) Update(_ context.Context, t *com.MessageTemplate) (*com.MessageTemplate, error) {
	if m.updateErr != nil {
		return nil, m.updateErr
	}
	t.UpdatedAt = time.Now()
	cp := *t
	m.items[t.ID] = &cp
	return &cp, nil
}

func (m *mockTemplateRepo) Delete(_ context.Context, id uuid.UUID) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	delete(m.items, id)
	return nil
}

// mockDirectoryRepo is an in-memory DirectoryRepository.
type mockDirectoryRepo struct {
	items     map[string]*com.DirectoryPreference
	upsertErr error
	findErr   error
}

func newMockDirectoryRepo() *mockDirectoryRepo {
	return &mockDirectoryRepo{items: make(map[string]*com.DirectoryPreference)}
}

func (m *mockDirectoryRepo) Upsert(_ context.Context, p *com.DirectoryPreference) (*com.DirectoryPreference, error) {
	if m.upsertErr != nil {
		return nil, m.upsertErr
	}
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	p.CreatedAt = time.Now()
	p.UpdatedAt = time.Now()
	key := p.UserID.String() + ":" + p.OrgID.String()
	cp := *p
	m.items[key] = &cp
	return &cp, nil
}

func (m *mockDirectoryRepo) FindByUserAndOrg(_ context.Context, userID, orgID uuid.UUID) (*com.DirectoryPreference, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	key := userID.String() + ":" + orgID.String()
	p, ok := m.items[key]
	if !ok {
		return nil, nil
	}
	cp := *p
	return &cp, nil
}

// mockCommLogRepo is an in-memory CommLogRepository.
type mockCommLogRepo struct {
	items     map[uuid.UUID]*com.CommunicationLog
	createErr error
	findErr   error
	updateErr error
}

func newMockCommLogRepo() *mockCommLogRepo {
	return &mockCommLogRepo{items: make(map[uuid.UUID]*com.CommunicationLog)}
}

func (m *mockCommLogRepo) Create(_ context.Context, entry *com.CommunicationLog) (*com.CommunicationLog, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	if entry.ID == uuid.Nil {
		entry.ID = uuid.New()
	}
	entry.CreatedAt = time.Now()
	entry.UpdatedAt = time.Now()
	cp := *entry
	m.items[entry.ID] = &cp
	return &cp, nil
}

func (m *mockCommLogRepo) FindByID(_ context.Context, id uuid.UUID) (*com.CommunicationLog, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	entry, ok := m.items[id]
	if !ok {
		return nil, nil
	}
	cp := *entry
	return &cp, nil
}

func (m *mockCommLogRepo) ListByOrg(_ context.Context, orgID uuid.UUID) ([]com.CommunicationLog, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	var result []com.CommunicationLog
	for _, e := range m.items {
		if e.OrgID == orgID {
			result = append(result, *e)
		}
	}
	return result, nil
}

func (m *mockCommLogRepo) ListByUnit(_ context.Context, unitID uuid.UUID) ([]com.CommunicationLog, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	var result []com.CommunicationLog
	for _, e := range m.items {
		if e.UnitID != nil && *e.UnitID == unitID {
			result = append(result, *e)
		}
	}
	return result, nil
}

func (m *mockCommLogRepo) Update(_ context.Context, entry *com.CommunicationLog) (*com.CommunicationLog, error) {
	if m.updateErr != nil {
		return nil, m.updateErr
	}
	entry.UpdatedAt = time.Now()
	cp := *entry
	m.items[entry.ID] = &cp
	return &cp, nil
}

// ─── Helper ───────────────────────────────────────────────────────────────────

func newTestService(
	announcements *mockAnnouncementRepo,
	threads *mockThreadRepo,
	notifications *mockNotificationRepo,
	calendar *mockCalendarRepo,
	templates *mockTemplateRepo,
	directory *mockDirectoryRepo,
	commLog *mockCommLogRepo,
) *com.ComService {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	return com.NewComService(
		announcements,
		threads,
		notifications,
		calendar,
		templates,
		directory,
		commLog,
		audit.NewNoopAuditor(),
		queue.NewInMemoryPublisher(),
		logger,
	)
}

// ─── Announcement tests ───────────────────────────────────────────────────────

func TestCreateAnnouncement_SetsPublishedAt(t *testing.T) {
	// Given: no scheduled_for is provided
	svc := newTestService(
		newMockAnnouncementRepo(),
		newMockThreadRepo(),
		newMockNotificationRepo(),
		newMockCalendarRepo(),
		newMockTemplateRepo(),
		newMockDirectoryRepo(),
		newMockCommLogRepo(),
	)
	orgID := uuid.New()
	authorID := uuid.New()
	before := time.Now()

	req := com.CreateAnnouncementRequest{
		Title: "Test Announcement",
		Body:  "Hello residents.",
	}

	// When: CreateAnnouncement is called
	result, err := svc.CreateAnnouncement(context.Background(), orgID, req, authorID)

	// Then: published_at should be set to now, scheduled_for is nil
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEqual(t, uuid.Nil, result.ID)
	assert.Equal(t, orgID, result.OrgID)
	assert.Equal(t, authorID, result.AuthorID)
	assert.Equal(t, "Test Announcement", result.Title)
	assert.Nil(t, result.ScheduledFor)
	require.NotNil(t, result.PublishedAt)
	assert.False(t, result.PublishedAt.Before(before), "published_at should not be before test start")
}

func TestCreateAnnouncement_ScheduledForFuture(t *testing.T) {
	// Given: a scheduled_for time is provided
	svc := newTestService(
		newMockAnnouncementRepo(),
		newMockThreadRepo(),
		newMockNotificationRepo(),
		newMockCalendarRepo(),
		newMockTemplateRepo(),
		newMockDirectoryRepo(),
		newMockCommLogRepo(),
	)
	orgID := uuid.New()
	authorID := uuid.New()
	future := time.Now().Add(24 * time.Hour)

	req := com.CreateAnnouncementRequest{
		Title:        "Scheduled Announcement",
		Body:         "Coming soon.",
		ScheduledFor: &future,
	}

	// When: CreateAnnouncement is called
	result, err := svc.CreateAnnouncement(context.Background(), orgID, req, authorID)

	// Then: published_at should be nil, scheduled_for should be set
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Nil(t, result.PublishedAt, "published_at should be nil for scheduled announcements")
	require.NotNil(t, result.ScheduledFor)
	assert.Equal(t, future.Unix(), result.ScheduledFor.Unix())
}

func TestCreateAnnouncement_ValidationError(t *testing.T) {
	// Given: an invalid request with missing title
	svc := newTestService(
		newMockAnnouncementRepo(),
		newMockThreadRepo(),
		newMockNotificationRepo(),
		newMockCalendarRepo(),
		newMockTemplateRepo(),
		newMockDirectoryRepo(),
		newMockCommLogRepo(),
	)

	req := com.CreateAnnouncementRequest{Body: "body only, no title"}

	// When: CreateAnnouncement is called
	result, err := svc.CreateAnnouncement(context.Background(), uuid.New(), req, uuid.New())

	// Then: a validation error should be returned
	require.Error(t, err)
	assert.Nil(t, result)
	var valErr *api.ValidationError
	assert.ErrorAs(t, err, &valErr)
}

func TestGetAnnouncement_NotFound(t *testing.T) {
	// Given: no announcements exist
	svc := newTestService(
		newMockAnnouncementRepo(),
		newMockThreadRepo(),
		newMockNotificationRepo(),
		newMockCalendarRepo(),
		newMockTemplateRepo(),
		newMockDirectoryRepo(),
		newMockCommLogRepo(),
	)

	// When: GetAnnouncement is called with an unknown ID
	result, err := svc.GetAnnouncement(context.Background(), uuid.New())

	// Then: a 404 NotFoundError should be returned
	require.Error(t, err)
	assert.Nil(t, result)
	var notFoundErr *api.NotFoundError
	assert.ErrorAs(t, err, &notFoundErr)
}

// ─── Thread & Message tests ───────────────────────────────────────────────────

func TestSendMessage_Success(t *testing.T) {
	// Given: a thread repo and a valid thread
	threadRepo := newMockThreadRepo()
	svc := newTestService(
		newMockAnnouncementRepo(),
		threadRepo,
		newMockNotificationRepo(),
		newMockCalendarRepo(),
		newMockTemplateRepo(),
		newMockDirectoryRepo(),
		newMockCommLogRepo(),
	)
	ctx := context.Background()
	orgID := uuid.New()
	createdBy := uuid.New()

	// Create a thread first
	thread, err := svc.CreateThread(ctx, orgID, com.CreateThreadRequest{Subject: "Test thread"}, createdBy)
	require.NoError(t, err)

	senderID := uuid.New()
	req := com.SendMessageRequest{Body: "Hello, world!"}

	// When: SendMessage is called
	msg, err := svc.SendMessage(ctx, thread.ID, req, senderID)

	// Then: a message should be created with correct fields
	require.NoError(t, err)
	require.NotNil(t, msg)
	assert.NotEqual(t, uuid.Nil, msg.ID)
	assert.Equal(t, thread.ID, msg.ThreadID)
	assert.Equal(t, senderID, msg.SenderID)
	assert.Equal(t, "Hello, world!", msg.Body)
	assert.Nil(t, msg.EditedAt)
}

func TestSendMessage_ValidationError(t *testing.T) {
	// Given: an empty body
	svc := newTestService(
		newMockAnnouncementRepo(),
		newMockThreadRepo(),
		newMockNotificationRepo(),
		newMockCalendarRepo(),
		newMockTemplateRepo(),
		newMockDirectoryRepo(),
		newMockCommLogRepo(),
	)

	req := com.SendMessageRequest{Body: ""}

	// When: SendMessage is called
	result, err := svc.SendMessage(context.Background(), uuid.New(), req, uuid.New())

	// Then: a validation error should be returned
	require.Error(t, err)
	assert.Nil(t, result)
	var valErr *api.ValidationError
	assert.ErrorAs(t, err, &valErr)
}

// ─── Calendar tests ───────────────────────────────────────────────────────────

func TestCreateCalendarEvent_Success(t *testing.T) {
	// Given: a valid calendar event request
	svc := newTestService(
		newMockAnnouncementRepo(),
		newMockThreadRepo(),
		newMockNotificationRepo(),
		newMockCalendarRepo(),
		newMockTemplateRepo(),
		newMockDirectoryRepo(),
		newMockCommLogRepo(),
	)
	orgID := uuid.New()
	createdBy := uuid.New()
	startsAt := time.Now().Add(48 * time.Hour)

	req := com.CreateCalendarEventRequest{
		Title:     "Annual HOA Meeting",
		EventType: "meeting",
		StartsAt:  startsAt,
	}

	// When: CreateCalendarEvent is called
	event, err := svc.CreateCalendarEvent(context.Background(), orgID, req, createdBy)

	// Then: the event should be created with correct fields
	require.NoError(t, err)
	require.NotNil(t, event)
	assert.NotEqual(t, uuid.Nil, event.ID)
	assert.Equal(t, orgID, event.OrgID)
	assert.Equal(t, createdBy, event.CreatedBy)
	assert.Equal(t, "Annual HOA Meeting", event.Title)
	assert.Equal(t, "meeting", event.EventType)
	assert.Equal(t, startsAt.Unix(), event.StartsAt.Unix())
}

func TestCreateCalendarEvent_ValidationError(t *testing.T) {
	// Given: a request with missing required fields
	svc := newTestService(
		newMockAnnouncementRepo(),
		newMockThreadRepo(),
		newMockNotificationRepo(),
		newMockCalendarRepo(),
		newMockTemplateRepo(),
		newMockDirectoryRepo(),
		newMockCommLogRepo(),
	)

	req := com.CreateCalendarEventRequest{
		// Missing title, event_type, starts_at
	}

	// When: CreateCalendarEvent is called
	result, err := svc.CreateCalendarEvent(context.Background(), uuid.New(), req, uuid.New())

	// Then: a validation error is returned
	require.Error(t, err)
	assert.Nil(t, result)
	var valErr *api.ValidationError
	assert.ErrorAs(t, err, &valErr)
}

func TestRSVPToEvent_Success(t *testing.T) {
	// Given: an event exists
	calendarRepo := newMockCalendarRepo()
	svc := newTestService(
		newMockAnnouncementRepo(),
		newMockThreadRepo(),
		newMockNotificationRepo(),
		calendarRepo,
		newMockTemplateRepo(),
		newMockDirectoryRepo(),
		newMockCommLogRepo(),
	)
	ctx := context.Background()
	orgID := uuid.New()
	createdBy := uuid.New()

	event, err := svc.CreateCalendarEvent(ctx, orgID, com.CreateCalendarEventRequest{
		Title:       "Board Meeting",
		EventType:   "meeting",
		StartsAt:    time.Now().Add(24 * time.Hour),
		RSVPEnabled: true,
	}, createdBy)
	require.NoError(t, err)

	userID := uuid.New()
	req := com.RSVPRequest{Status: "attending", GuestCount: 1}

	// When: RSVPToEvent is called
	rsvp, err := svc.RSVPToEvent(ctx, event.ID, req, userID)

	// Then: the RSVP should be recorded
	require.NoError(t, err)
	require.NotNil(t, rsvp)
	assert.NotEqual(t, uuid.Nil, rsvp.ID)
	assert.Equal(t, event.ID, rsvp.EventID)
	assert.Equal(t, userID, rsvp.UserID)
	assert.Equal(t, "attending", rsvp.Status)
	assert.Equal(t, 1, rsvp.GuestCount)
}

func TestRSVPToEvent_InvalidStatus(t *testing.T) {
	// Given: an invalid RSVP status
	svc := newTestService(
		newMockAnnouncementRepo(),
		newMockThreadRepo(),
		newMockNotificationRepo(),
		newMockCalendarRepo(),
		newMockTemplateRepo(),
		newMockDirectoryRepo(),
		newMockCommLogRepo(),
	)

	req := com.RSVPRequest{Status: "invalid-status"}

	// When: RSVPToEvent is called
	result, err := svc.RSVPToEvent(context.Background(), uuid.New(), req, uuid.New())

	// Then: a validation error is returned
	require.Error(t, err)
	assert.Nil(t, result)
	var valErr *api.ValidationError
	assert.ErrorAs(t, err, &valErr)
}

// ─── Communication Log tests ──────────────────────────────────────────────────

func TestLogCommunication_Success(t *testing.T) {
	// Given: a valid log request
	svc := newTestService(
		newMockAnnouncementRepo(),
		newMockThreadRepo(),
		newMockNotificationRepo(),
		newMockCalendarRepo(),
		newMockTemplateRepo(),
		newMockDirectoryRepo(),
		newMockCommLogRepo(),
	)
	orgID := uuid.New()
	initiatedBy := uuid.New()
	contactName := "Jane Smith"

	req := com.LogCommunicationRequest{
		Direction:   "outbound",
		Channel:     "email",
		ContactName: &contactName,
	}

	// When: LogCommunication is called
	entry, err := svc.LogCommunication(context.Background(), orgID, req, initiatedBy)

	// Then: the log entry should be created with correct fields
	require.NoError(t, err)
	require.NotNil(t, entry)
	assert.NotEqual(t, uuid.Nil, entry.ID)
	assert.Equal(t, orgID, entry.OrgID)
	assert.Equal(t, "outbound", entry.Direction)
	assert.Equal(t, "email", entry.Channel)
	assert.Equal(t, "sent", entry.Status)
	assert.Equal(t, "manual", entry.Source)
	require.NotNil(t, entry.InitiatedBy)
	assert.Equal(t, initiatedBy, *entry.InitiatedBy)
}

func TestLogCommunication_ValidationError(t *testing.T) {
	// Given: a request with no contact identifier
	svc := newTestService(
		newMockAnnouncementRepo(),
		newMockThreadRepo(),
		newMockNotificationRepo(),
		newMockCalendarRepo(),
		newMockTemplateRepo(),
		newMockDirectoryRepo(),
		newMockCommLogRepo(),
	)

	req := com.LogCommunicationRequest{
		Direction: "outbound",
		Channel:   "email",
		// No ContactName or ContactUserID
	}

	// When: LogCommunication is called
	result, err := svc.LogCommunication(context.Background(), uuid.New(), req, uuid.New())

	// Then: a validation error is returned
	require.Error(t, err)
	assert.Nil(t, result)
	var valErr *api.ValidationError
	assert.ErrorAs(t, err, &valErr)
}

// ─── Directory tests ──────────────────────────────────────────────────────────

func TestGetDirectoryPreferences_NotFound(t *testing.T) {
	// Given: no preferences exist for the user
	svc := newTestService(
		newMockAnnouncementRepo(),
		newMockThreadRepo(),
		newMockNotificationRepo(),
		newMockCalendarRepo(),
		newMockTemplateRepo(),
		newMockDirectoryRepo(),
		newMockCommLogRepo(),
	)
	userID := uuid.New()
	orgID := uuid.New()

	// When: GetDirectoryPreferences is called
	pref, err := svc.GetDirectoryPreferences(context.Background(), userID, orgID)

	// Then: default (opt-out) preferences are returned, not an error
	require.NoError(t, err)
	require.NotNil(t, pref)
	assert.Equal(t, userID, pref.UserID)
	assert.Equal(t, orgID, pref.OrgID)
	assert.False(t, pref.OptIn, "default should be opted out")
	assert.False(t, pref.ShowEmail)
	assert.False(t, pref.ShowPhone)
	assert.False(t, pref.ShowUnit)
}

func TestGetDirectoryPreferences_ExistingPreferences(t *testing.T) {
	// Given: preferences have been set
	directoryRepo := newMockDirectoryRepo()
	svc := newTestService(
		newMockAnnouncementRepo(),
		newMockThreadRepo(),
		newMockNotificationRepo(),
		newMockCalendarRepo(),
		newMockTemplateRepo(),
		directoryRepo,
		newMockCommLogRepo(),
	)
	ctx := context.Background()
	userID := uuid.New()
	orgID := uuid.New()
	trueVal := true

	// Set up preferences
	_, err := svc.UpdateDirectoryPreferences(ctx, userID, orgID, com.UpdateDirectoryPreferenceRequest{
		OptIn: &trueVal,
	})
	require.NoError(t, err)

	// When: GetDirectoryPreferences is called
	pref, err := svc.GetDirectoryPreferences(ctx, userID, orgID)

	// Then: the stored preferences are returned
	require.NoError(t, err)
	require.NotNil(t, pref)
	assert.True(t, pref.OptIn)
}
