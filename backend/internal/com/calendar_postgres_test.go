//go:build integration

package com_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/com"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Calendar Event Tests ─────────────────────────────────────────────────────

func TestCreateCalendarEvent(t *testing.T) {
	f := setupComFixture(t)
	repo := com.NewPostgresCalendarRepository(f.pool)
	ctx := context.Background()

	desc := "Annual general meeting"
	input := &com.CalendarEvent{
		OrgID:         f.orgID,
		Title:         "Annual Meeting",
		Description:   &desc,
		EventType:     "meeting",
		StartsAt:      time.Now().Add(24 * time.Hour).UTC().Truncate(time.Second),
		AudienceRoles: []string{"homeowner", "board"},
		RSVPEnabled:   true,
		CreatedBy:     f.userID,
	}

	got, err := repo.CreateEvent(ctx, input)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.NotEqual(t, uuid.Nil, got.ID)
	assert.Equal(t, "Annual Meeting", got.Title)
	assert.Equal(t, &desc, got.Description)
	assert.True(t, got.RSVPEnabled)
	assert.ElementsMatch(t, []string{"homeowner", "board"}, got.AudienceRoles)
	assert.False(t, got.CreatedAt.IsZero())
}

func TestFindCalendarEventByID(t *testing.T) {
	f := setupComFixture(t)
	repo := com.NewPostgresCalendarRepository(f.pool)
	ctx := context.Background()

	created, err := repo.CreateEvent(ctx, &com.CalendarEvent{
		OrgID:     f.orgID,
		Title:     "Find Event",
		EventType: "meeting",
		StartsAt:  time.Now().Add(time.Hour).UTC(),
		CreatedBy: f.userID,
	})
	require.NoError(t, err)

	got, err := repo.FindEventByID(ctx, created.ID)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, created.ID, got.ID)
	assert.Equal(t, "Find Event", got.Title)
}

func TestFindCalendarEventByID_NotFound(t *testing.T) {
	f := setupComFixture(t)
	repo := com.NewPostgresCalendarRepository(f.pool)
	ctx := context.Background()

	got, err := repo.FindEventByID(ctx, uuid.New())

	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestListCalendarEventsByOrg(t *testing.T) {
	f := setupComFixture(t)
	repo := com.NewPostgresCalendarRepository(f.pool)
	ctx := context.Background()

	for _, title := range []string{"Event Alpha", "Event Beta"} {
		_, err := repo.CreateEvent(ctx, &com.CalendarEvent{
			OrgID:     f.orgID,
			Title:     title,
			EventType: "community",
			StartsAt:  time.Now().Add(48 * time.Hour).UTC(),
			CreatedBy: f.userID,
		})
		require.NoError(t, err)
	}

	list, err := repo.ListEventsByOrg(ctx, f.orgID)

	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(list), 2)
	for _, e := range list {
		assert.Equal(t, f.orgID, e.OrgID)
	}
}

func TestListCalendarEventsByOrg_EmptyResult(t *testing.T) {
	f := setupComFixture(t)
	repo := com.NewPostgresCalendarRepository(f.pool)
	ctx := context.Background()

	list, err := repo.ListEventsByOrg(ctx, uuid.New())

	require.NoError(t, err)
	assert.NotNil(t, list)
	assert.Empty(t, list)
}

func TestSoftDeleteCalendarEvent(t *testing.T) {
	f := setupComFixture(t)
	repo := com.NewPostgresCalendarRepository(f.pool)
	ctx := context.Background()

	created, err := repo.CreateEvent(ctx, &com.CalendarEvent{
		OrgID:     f.orgID,
		Title:     "Delete Me Event",
		EventType: "meeting",
		StartsAt:  time.Now().Add(time.Hour).UTC(),
		CreatedBy: f.userID,
	})
	require.NoError(t, err)

	err = repo.SoftDeleteEvent(ctx, created.ID)
	require.NoError(t, err)

	got, err := repo.FindEventByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Nil(t, got, "soft-deleted event should not be returned by FindEventByID")
}

// ─── RSVP Tests ───────────────────────────────────────────────────────────────

func TestCreateRSVP(t *testing.T) {
	f := setupComFixture(t)
	repo := com.NewPostgresCalendarRepository(f.pool)
	ctx := context.Background()

	event, err := repo.CreateEvent(ctx, &com.CalendarEvent{
		OrgID:       f.orgID,
		Title:       "RSVP Event",
		EventType:   "meeting",
		StartsAt:    time.Now().Add(72 * time.Hour).UTC(),
		RSVPEnabled: true,
		CreatedBy:   f.userID,
	})
	require.NoError(t, err)

	rsvp, err := repo.CreateRSVP(ctx, &com.CalendarEventRSVP{
		EventID:    event.ID,
		UserID:     f.userID,
		Status:     "attending",
		GuestCount: 2,
	})

	require.NoError(t, err)
	require.NotNil(t, rsvp)
	assert.NotEqual(t, uuid.Nil, rsvp.ID)
	assert.Equal(t, "attending", rsvp.Status)
	assert.Equal(t, 2, rsvp.GuestCount)
}

func TestListRSVPsByEvent(t *testing.T) {
	f := setupComFixture(t)
	repo := com.NewPostgresCalendarRepository(f.pool)
	ctx := context.Background()

	event, err := repo.CreateEvent(ctx, &com.CalendarEvent{
		OrgID:       f.orgID,
		Title:       "List RSVPs Event",
		EventType:   "community",
		StartsAt:    time.Now().Add(48 * time.Hour).UTC(),
		RSVPEnabled: true,
		CreatedBy:   f.userID,
	})
	require.NoError(t, err)

	_, err = repo.CreateRSVP(ctx, &com.CalendarEventRSVP{
		EventID: event.ID,
		UserID:  f.userID,
		Status:  "attending",
	})
	require.NoError(t, err)

	list, err := repo.ListRSVPsByEvent(ctx, event.ID)

	require.NoError(t, err)
	assert.Len(t, list, 1)
	assert.Equal(t, f.userID, list[0].UserID)
}

func TestListRSVPsByEvent_Empty(t *testing.T) {
	f := setupComFixture(t)
	repo := com.NewPostgresCalendarRepository(f.pool)
	ctx := context.Background()

	list, err := repo.ListRSVPsByEvent(ctx, uuid.New())

	require.NoError(t, err)
	assert.NotNil(t, list)
	assert.Empty(t, list)
}

// ─── Template Tests ───────────────────────────────────────────────────────────

func TestCreateMessageTemplate(t *testing.T) {
	f := setupComFixture(t)
	repo := com.NewPostgresTemplateRepository(f.pool)
	ctx := context.Background()

	subject := "Welcome to {{ .OrgName }}"
	input := &com.MessageTemplate{
		OrgID:       &f.orgID,
		TemplateKey: "welcome",
		Channel:     "email",
		Subject:     &subject,
		Body:        "Hello {{ .Name }}, welcome!",
		IsActive:    true,
	}

	got, err := repo.Create(ctx, input)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.NotEqual(t, uuid.Nil, got.ID)
	assert.Equal(t, "welcome", got.TemplateKey)
	assert.Equal(t, "email", got.Channel)
	assert.Equal(t, &subject, got.Subject)
}

func TestListTemplatesByOrg_IncludesSystemDefaults(t *testing.T) {
	f := setupComFixture(t)
	repo := com.NewPostgresTemplateRepository(f.pool)
	ctx := context.Background()

	// Insert a system default (org_id = NULL).
	_, err := repo.Create(ctx, &com.MessageTemplate{
		OrgID:       nil,
		TemplateKey: "system-default-" + uuid.New().String(),
		Channel:     "email",
		Body:        "System default body",
		IsActive:    true,
	})
	require.NoError(t, err)

	// Insert an org-specific template.
	_, err = repo.Create(ctx, &com.MessageTemplate{
		OrgID:       &f.orgID,
		TemplateKey: "org-specific-" + uuid.New().String(),
		Channel:     "email",
		Body:        "Org specific body",
		IsActive:    true,
	})
	require.NoError(t, err)

	list, err := repo.ListByOrg(ctx, f.orgID)

	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(list), 2, "should include at least the org template and the system default")

	hasSystemDefault := false
	hasOrgTemplate := false
	for _, tmpl := range list {
		if tmpl.OrgID == nil {
			hasSystemDefault = true
		}
		if tmpl.OrgID != nil && *tmpl.OrgID == f.orgID {
			hasOrgTemplate = true
		}
	}
	assert.True(t, hasSystemDefault, "result should include system default templates (org_id IS NULL)")
	assert.True(t, hasOrgTemplate, "result should include org-specific templates")
}

// ─── Directory Preference Tests ───────────────────────────────────────────────

func TestUpsertDirectoryPreference(t *testing.T) {
	f := setupComFixture(t)
	repo := com.NewPostgresDirectoryRepository(f.pool)
	ctx := context.Background()

	input := &com.DirectoryPreference{
		UserID:    f.userID,
		OrgID:     f.orgID,
		OptIn:     true,
		ShowEmail: false,
		ShowPhone: false,
		ShowUnit:  true,
	}

	got, err := repo.Upsert(ctx, input)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.NotEqual(t, uuid.Nil, got.ID)
	assert.True(t, got.OptIn)
	assert.True(t, got.ShowUnit)
	assert.False(t, got.CreatedAt.IsZero())
}

func TestUpsertDirectoryPreference_UpdatesExisting(t *testing.T) {
	f := setupComFixture(t)
	repo := com.NewPostgresDirectoryRepository(f.pool)
	ctx := context.Background()

	pref := &com.DirectoryPreference{
		UserID:    f.userID,
		OrgID:     f.orgID,
		OptIn:     true,
		ShowEmail: false,
		ShowPhone: false,
		ShowUnit:  true,
	}
	first, err := repo.Upsert(ctx, pref)
	require.NoError(t, err)

	pref.ShowEmail = true
	pref.OptIn = false
	second, err := repo.Upsert(ctx, pref)

	require.NoError(t, err)
	assert.Equal(t, first.ID, second.ID, "should update existing row, not insert a new one")
	assert.False(t, second.OptIn)
	assert.True(t, second.ShowEmail)
}

func TestFindDirectoryPreferenceByUserAndOrg(t *testing.T) {
	f := setupComFixture(t)
	repo := com.NewPostgresDirectoryRepository(f.pool)
	ctx := context.Background()

	_, err := repo.Upsert(ctx, &com.DirectoryPreference{
		UserID: f.userID,
		OrgID:  f.orgID,
		OptIn:  true,
	})
	require.NoError(t, err)

	got, err := repo.FindByUserAndOrg(ctx, f.userID, f.orgID)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, f.userID, got.UserID)
	assert.Equal(t, f.orgID, got.OrgID)
}

func TestFindDirectoryPreferenceByUserAndOrg_NotFound(t *testing.T) {
	f := setupComFixture(t)
	repo := com.NewPostgresDirectoryRepository(f.pool)
	ctx := context.Background()

	got, err := repo.FindByUserAndOrg(ctx, uuid.New(), uuid.New())

	require.NoError(t, err)
	assert.Nil(t, got)
}

// ─── Communication Log Tests ──────────────────────────────────────────────────

func TestCreateCommunicationLog(t *testing.T) {
	f := setupComFixture(t)
	repo := com.NewPostgresCommLogRepository(f.pool)
	ctx := context.Background()

	subject := "Welcome email"
	input := &com.CommunicationLog{
		OrgID:     f.orgID,
		Direction: "outbound",
		Channel:   "email",
		UnitID:    &f.unitID,
		Subject:   &subject,
		Status:    "sent",
		Source:    "manual",
		Metadata:  map[string]any{"campaign": "welcome"},
	}

	got, err := repo.Create(ctx, input)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.NotEqual(t, uuid.Nil, got.ID)
	assert.Equal(t, "outbound", got.Direction)
	assert.Equal(t, "email", got.Channel)
	assert.Equal(t, "sent", got.Status)
	assert.Equal(t, map[string]any{"campaign": "welcome"}, got.Metadata)
	assert.NotNil(t, got.AttachmentIDs)
}

func TestFindCommunicationLogByID(t *testing.T) {
	f := setupComFixture(t)
	repo := com.NewPostgresCommLogRepository(f.pool)
	ctx := context.Background()

	created, err := repo.Create(ctx, &com.CommunicationLog{
		OrgID:     f.orgID,
		Direction: "outbound",
		Channel:   "sms",
		Status:    "sent",
		Source:    "automated",
		Metadata:  map[string]any{},
	})
	require.NoError(t, err)

	got, err := repo.FindByID(ctx, created.ID)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, created.ID, got.ID)
}

func TestFindCommunicationLogByID_NotFound(t *testing.T) {
	f := setupComFixture(t)
	repo := com.NewPostgresCommLogRepository(f.pool)
	ctx := context.Background()

	got, err := repo.FindByID(ctx, uuid.New())

	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestListCommunicationLogByOrg(t *testing.T) {
	f := setupComFixture(t)
	repo := com.NewPostgresCommLogRepository(f.pool)
	ctx := context.Background()

	for _, channel := range []string{"email", "sms"} {
		_, err := repo.Create(ctx, &com.CommunicationLog{
			OrgID:     f.orgID,
			Direction: "outbound",
			Channel:   channel,
			Status:    "sent",
			Source:    "manual",
			Metadata:  map[string]any{},
		})
		require.NoError(t, err)
	}

	list, err := repo.ListByOrg(ctx, f.orgID)

	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(list), 2)
	for _, entry := range list {
		assert.Equal(t, f.orgID, entry.OrgID)
	}
}

func TestListCommunicationLogByOrg_EmptyResult(t *testing.T) {
	f := setupComFixture(t)
	repo := com.NewPostgresCommLogRepository(f.pool)
	ctx := context.Background()

	list, err := repo.ListByOrg(ctx, uuid.New())

	require.NoError(t, err)
	assert.NotNil(t, list)
	assert.Empty(t, list)
}

func TestListCommunicationLogByUnit(t *testing.T) {
	f := setupComFixture(t)
	repo := com.NewPostgresCommLogRepository(f.pool)
	ctx := context.Background()

	// Create one entry linked to the unit and one without.
	_, err := repo.Create(ctx, &com.CommunicationLog{
		OrgID:     f.orgID,
		Direction: "outbound",
		Channel:   "email",
		UnitID:    &f.unitID,
		Status:    "sent",
		Source:    "manual",
		Metadata:  map[string]any{},
	})
	require.NoError(t, err)

	_, err = repo.Create(ctx, &com.CommunicationLog{
		OrgID:     f.orgID,
		Direction: "outbound",
		Channel:   "sms",
		Status:    "sent",
		Source:    "manual",
		Metadata:  map[string]any{},
	})
	require.NoError(t, err)

	list, err := repo.ListByUnit(ctx, f.unitID)

	require.NoError(t, err)
	assert.Len(t, list, 1)
	assert.Equal(t, &f.unitID, list[0].UnitID)
}
