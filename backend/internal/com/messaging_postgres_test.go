//go:build integration

package com_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/quorant/quorant/internal/com"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Test Database Setup ──────────────────────────────────────────────────────

func setupComTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, "postgres://quorant:quorant@localhost:5432/quorant_dev?sslmode=disable")
	require.NoError(t, err, "connecting to test database")

	t.Cleanup(func() {
		cleanCtx := context.Background()
		pool.Exec(cleanCtx, "DELETE FROM communication_log")
		pool.Exec(cleanCtx, "DELETE FROM directory_preferences")
		pool.Exec(cleanCtx, "DELETE FROM calendar_event_rsvps")
		pool.Exec(cleanCtx, "DELETE FROM calendar_events")
		pool.Exec(cleanCtx, "DELETE FROM message_templates WHERE org_id IS NOT NULL")
		pool.Exec(cleanCtx, "DELETE FROM push_tokens")
		pool.Exec(cleanCtx, "DELETE FROM notification_preferences")
		pool.Exec(cleanCtx, "DELETE FROM messages")
		pool.Exec(cleanCtx, "DELETE FROM threads")
		pool.Exec(cleanCtx, "DELETE FROM announcements")
		pool.Exec(cleanCtx, "DELETE FROM memberships")
		pool.Exec(cleanCtx, "DELETE FROM units")
		pool.Exec(cleanCtx, "DELETE FROM organizations WHERE parent_id IS NOT NULL")
		pool.Exec(cleanCtx, "DELETE FROM organizations")
		pool.Exec(cleanCtx, "DELETE FROM users")
		pool.Close()
	})

	return pool
}

// comTestFixture holds shared test resources.
type comTestFixture struct {
	pool   *pgxpool.Pool
	orgID  uuid.UUID
	userID uuid.UUID
	unitID uuid.UUID
}

// setupComFixture creates a pool with a test org, user, and unit.
func setupComFixture(t *testing.T) comTestFixture {
	t.Helper()
	pool := setupComTestDB(t)
	ctx := context.Background()

	// Create a test org.
	var orgID uuid.UUID
	err := pool.QueryRow(ctx,
		`INSERT INTO organizations (type, name, slug, path, settings)
		 VALUES ('hoa', $1, $2, $3, '{}')
		 RETURNING id`,
		"Test HOA "+uuid.New().String(),
		"test-hoa-"+uuid.New().String(),
		"test_hoa_"+uuid.New().String(),
	).Scan(&orgID)
	require.NoError(t, err, "create test org")

	// Create a test user.
	var userID uuid.UUID
	err = pool.QueryRow(ctx,
		`INSERT INTO users (idp_user_id, email, display_name)
		 VALUES ($1, $2, 'Test User')
		 RETURNING id`,
		"test-idp-"+uuid.New().String(),
		"test-"+uuid.New().String()+"@example.com",
	).Scan(&userID)
	require.NoError(t, err, "create test user")

	// Create a test unit.
	var unitID uuid.UUID
	err = pool.QueryRow(ctx,
		`INSERT INTO units (org_id, label, status)
		 VALUES ($1, 'Unit 101', 'occupied')
		 RETURNING id`,
		orgID,
	).Scan(&unitID)
	require.NoError(t, err, "create test unit")

	return comTestFixture{
		pool:   pool,
		orgID:  orgID,
		userID: userID,
		unitID: unitID,
	}
}

// ─── Announcement Tests ───────────────────────────────────────────────────────

func TestCreateAnnouncement(t *testing.T) {
	f := setupComFixture(t)
	repo := com.NewPostgresAnnouncementRepository(f.pool)
	ctx := context.Background()

	input := &com.Announcement{
		OrgID:         f.orgID,
		AuthorID:      f.userID,
		Title:         "Welcome to the community",
		Body:          "This is the body of the announcement.",
		IsPinned:      true,
		AudienceRoles: []string{"homeowner", "board"},
	}

	got, err := repo.Create(ctx, input)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.NotEqual(t, uuid.Nil, got.ID)
	assert.Equal(t, f.orgID, got.OrgID)
	assert.Equal(t, "Welcome to the community", got.Title)
	assert.True(t, got.IsPinned)
	assert.ElementsMatch(t, []string{"homeowner", "board"}, got.AudienceRoles)
	assert.False(t, got.CreatedAt.IsZero())
}

func TestFindAnnouncementByID(t *testing.T) {
	f := setupComFixture(t)
	repo := com.NewPostgresAnnouncementRepository(f.pool)
	ctx := context.Background()

	created, err := repo.Create(ctx, &com.Announcement{
		OrgID:    f.orgID,
		AuthorID: f.userID,
		Title:    "Find Me",
		Body:     "body",
	})
	require.NoError(t, err)

	got, err := repo.FindByID(ctx, created.ID)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, created.ID, got.ID)
	assert.Equal(t, "Find Me", got.Title)
}

func TestFindAnnouncementByID_NotFound(t *testing.T) {
	f := setupComFixture(t)
	repo := com.NewPostgresAnnouncementRepository(f.pool)
	ctx := context.Background()

	got, err := repo.FindByID(ctx, uuid.New())

	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestListAnnouncementsByOrg(t *testing.T) {
	f := setupComFixture(t)
	repo := com.NewPostgresAnnouncementRepository(f.pool)
	ctx := context.Background()

	for _, title := range []string{"Announcement Alpha", "Announcement Beta"} {
		_, err := repo.Create(ctx, &com.Announcement{
			OrgID:    f.orgID,
			AuthorID: f.userID,
			Title:    title,
			Body:     "body",
		})
		require.NoError(t, err)
	}

	list, err := repo.ListByOrg(ctx, f.orgID)

	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(list), 2)
	for _, a := range list {
		assert.Equal(t, f.orgID, a.OrgID)
	}
}

func TestListAnnouncementsByOrg_EmptyResult(t *testing.T) {
	f := setupComFixture(t)
	repo := com.NewPostgresAnnouncementRepository(f.pool)
	ctx := context.Background()

	list, err := repo.ListByOrg(ctx, uuid.New())

	require.NoError(t, err)
	assert.NotNil(t, list)
	assert.Empty(t, list)
}

func TestSoftDeleteAnnouncement(t *testing.T) {
	f := setupComFixture(t)
	repo := com.NewPostgresAnnouncementRepository(f.pool)
	ctx := context.Background()

	created, err := repo.Create(ctx, &com.Announcement{
		OrgID:    f.orgID,
		AuthorID: f.userID,
		Title:    "Delete Me",
		Body:     "body",
	})
	require.NoError(t, err)

	err = repo.SoftDelete(ctx, created.ID)
	require.NoError(t, err)

	got, err := repo.FindByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Nil(t, got, "soft-deleted announcement should not be returned by FindByID")
}

// ─── Thread and Message Tests ─────────────────────────────────────────────────

func TestCreateThread(t *testing.T) {
	f := setupComFixture(t)
	repo := com.NewPostgresThreadRepository(f.pool)
	ctx := context.Background()

	input := &com.Thread{
		OrgID:      f.orgID,
		Subject:    "Important Discussion",
		ThreadType: "general",
		CreatedBy:  f.userID,
	}

	got, err := repo.CreateThread(ctx, input)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.NotEqual(t, uuid.Nil, got.ID)
	assert.Equal(t, "Important Discussion", got.Subject)
	assert.Equal(t, "general", got.ThreadType)
	assert.False(t, got.IsClosed)
}

func TestFindThreadByID(t *testing.T) {
	f := setupComFixture(t)
	repo := com.NewPostgresThreadRepository(f.pool)
	ctx := context.Background()

	created, err := repo.CreateThread(ctx, &com.Thread{
		OrgID:      f.orgID,
		Subject:    "Find Thread",
		ThreadType: "general",
		CreatedBy:  f.userID,
	})
	require.NoError(t, err)

	got, err := repo.FindThreadByID(ctx, created.ID)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, created.ID, got.ID)
}

func TestFindThreadByID_NotFound(t *testing.T) {
	f := setupComFixture(t)
	repo := com.NewPostgresThreadRepository(f.pool)
	ctx := context.Background()

	got, err := repo.FindThreadByID(ctx, uuid.New())

	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestListThreadsByOrg(t *testing.T) {
	f := setupComFixture(t)
	repo := com.NewPostgresThreadRepository(f.pool)
	ctx := context.Background()

	for _, subject := range []string{"Thread Alpha", "Thread Beta"} {
		_, err := repo.CreateThread(ctx, &com.Thread{
			OrgID:      f.orgID,
			Subject:    subject,
			ThreadType: "general",
			CreatedBy:  f.userID,
		})
		require.NoError(t, err)
	}

	list, err := repo.ListThreadsByOrg(ctx, f.orgID)

	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(list), 2)
}

func TestCreateAndListMessages(t *testing.T) {
	f := setupComFixture(t)
	repo := com.NewPostgresThreadRepository(f.pool)
	ctx := context.Background()

	thread, err := repo.CreateThread(ctx, &com.Thread{
		OrgID:      f.orgID,
		Subject:    "Message Thread",
		ThreadType: "general",
		CreatedBy:  f.userID,
	})
	require.NoError(t, err)

	msg1, err := repo.CreateMessage(ctx, &com.Message{
		ThreadID: thread.ID,
		SenderID: f.userID,
		Body:     "Hello world",
	})
	require.NoError(t, err)
	require.NotNil(t, msg1)
	assert.NotEqual(t, uuid.Nil, msg1.ID)
	assert.Equal(t, "Hello world", msg1.Body)
	assert.NotNil(t, msg1.AttachmentIDs)

	_, err = repo.CreateMessage(ctx, &com.Message{
		ThreadID: thread.ID,
		SenderID: f.userID,
		Body:     "Second message",
	})
	require.NoError(t, err)

	messages, err := repo.ListMessagesByThread(ctx, thread.ID)

	require.NoError(t, err)
	assert.Len(t, messages, 2)
	assert.Equal(t, "Hello world", messages[0].Body)
}

func TestListMessagesByThread_Empty(t *testing.T) {
	f := setupComFixture(t)
	repo := com.NewPostgresThreadRepository(f.pool)
	ctx := context.Background()

	thread, err := repo.CreateThread(ctx, &com.Thread{
		OrgID:      f.orgID,
		Subject:    "Empty Thread",
		ThreadType: "general",
		CreatedBy:  f.userID,
	})
	require.NoError(t, err)

	messages, err := repo.ListMessagesByThread(ctx, thread.ID)

	require.NoError(t, err)
	assert.NotNil(t, messages)
	assert.Empty(t, messages)
}

func TestSoftDeleteMessage(t *testing.T) {
	f := setupComFixture(t)
	repo := com.NewPostgresThreadRepository(f.pool)
	ctx := context.Background()

	thread, err := repo.CreateThread(ctx, &com.Thread{
		OrgID:      f.orgID,
		Subject:    "Delete Message Thread",
		ThreadType: "general",
		CreatedBy:  f.userID,
	})
	require.NoError(t, err)

	msg, err := repo.CreateMessage(ctx, &com.Message{
		ThreadID: thread.ID,
		SenderID: f.userID,
		Body:     "Delete me",
	})
	require.NoError(t, err)

	err = repo.SoftDeleteMessage(ctx, msg.ID)
	require.NoError(t, err)

	messages, err := repo.ListMessagesByThread(ctx, thread.ID)
	require.NoError(t, err)
	for _, m := range messages {
		assert.NotEqual(t, msg.ID, m.ID, "soft-deleted message should not appear in list")
	}
}

// ─── Notification Preference Tests ───────────────────────────────────────────

func TestUpsertNotificationPreference(t *testing.T) {
	f := setupComFixture(t)
	repo := com.NewPostgresNotificationRepository(f.pool)
	ctx := context.Background()

	input := &com.NotificationPreference{
		UserID:    f.userID,
		OrgID:     f.orgID,
		Channel:   "email",
		EventType: "announcement.published",
		Enabled:   true,
	}

	got, err := repo.UpsertPreference(ctx, input)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.NotEqual(t, uuid.Nil, got.ID)
	assert.Equal(t, "email", got.Channel)
	assert.True(t, got.Enabled)
}

func TestUpsertNotificationPreference_UpdatesExisting(t *testing.T) {
	f := setupComFixture(t)
	repo := com.NewPostgresNotificationRepository(f.pool)
	ctx := context.Background()

	pref := &com.NotificationPreference{
		UserID:    f.userID,
		OrgID:     f.orgID,
		Channel:   "push",
		EventType: "thread.message.created",
		Enabled:   true,
	}
	first, err := repo.UpsertPreference(ctx, pref)
	require.NoError(t, err)

	pref.Enabled = false
	second, err := repo.UpsertPreference(ctx, pref)

	require.NoError(t, err)
	assert.Equal(t, first.ID, second.ID, "should update existing row, not insert a new one")
	assert.False(t, second.Enabled)
}

func TestListNotificationPreferencesByUser(t *testing.T) {
	f := setupComFixture(t)
	repo := com.NewPostgresNotificationRepository(f.pool)
	ctx := context.Background()

	prefs := []*com.NotificationPreference{
		{UserID: f.userID, OrgID: f.orgID, Channel: "email", EventType: "announcement.published", Enabled: true},
		{UserID: f.userID, OrgID: f.orgID, Channel: "push", EventType: "thread.message.created", Enabled: false},
	}
	for _, p := range prefs {
		_, err := repo.UpsertPreference(ctx, p)
		require.NoError(t, err)
	}

	list, err := repo.ListPreferencesByUser(ctx, f.userID, f.orgID)

	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(list), 2)
	for _, p := range list {
		assert.Equal(t, f.userID, p.UserID)
		assert.Equal(t, f.orgID, p.OrgID)
	}
}

// ─── Push Token Tests ─────────────────────────────────────────────────────────

func TestCreatePushToken(t *testing.T) {
	f := setupComFixture(t)
	repo := com.NewPostgresNotificationRepository(f.pool)
	ctx := context.Background()

	deviceName := "iPhone 15"
	input := &com.PushToken{
		UserID:     f.userID,
		Token:      "test-device-token-" + uuid.New().String(),
		Platform:   "ios",
		DeviceName: &deviceName,
	}

	got, err := repo.CreatePushToken(ctx, input)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.NotEqual(t, uuid.Nil, got.ID)
	assert.Equal(t, "ios", got.Platform)
	assert.Equal(t, &deviceName, got.DeviceName)
}

func TestListPushTokensByUser(t *testing.T) {
	f := setupComFixture(t)
	repo := com.NewPostgresNotificationRepository(f.pool)
	ctx := context.Background()

	for _, platform := range []string{"ios", "android"} {
		_, err := repo.CreatePushToken(ctx, &com.PushToken{
			UserID:   f.userID,
			Token:    "token-" + platform + "-" + uuid.New().String(),
			Platform: platform,
		})
		require.NoError(t, err)
	}

	list, err := repo.ListPushTokensByUser(ctx, f.userID)

	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(list), 2)
	for _, tok := range list {
		assert.Equal(t, f.userID, tok.UserID)
	}
}

func TestDeletePushToken(t *testing.T) {
	f := setupComFixture(t)
	repo := com.NewPostgresNotificationRepository(f.pool)
	ctx := context.Background()

	created, err := repo.CreatePushToken(ctx, &com.PushToken{
		UserID:   f.userID,
		Token:    "delete-me-" + uuid.New().String(),
		Platform: "web",
	})
	require.NoError(t, err)

	err = repo.DeletePushToken(ctx, created.ID)
	require.NoError(t, err)

	list, err := repo.ListPushTokensByUser(ctx, f.userID)
	require.NoError(t, err)
	for _, tok := range list {
		assert.NotEqual(t, created.ID, tok.ID, "deleted token should not appear in list")
	}
}
