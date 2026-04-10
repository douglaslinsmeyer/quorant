package com

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/platform/auth"
	"github.com/quorant/quorant/internal/platform/middleware"
)

func RegisterRoutes(
	mux *http.ServeMux,
	announcementHandler *AnnouncementHandler,
	threadHandler *ThreadHandler,
	calendarHandler *CalendarHandler,
	notificationHandler *NotificationHandler,
	commLogHandler *CommLogHandler,
	validator auth.TokenValidator,
	checker middleware.PermissionChecker,
	resolveUserID func(context.Context) (uuid.UUID, error),
) {
	// auth only — no org context (user-scoped); resolves and stores user ID in context
	authMw := func(h http.HandlerFunc) http.Handler {
		return middleware.Auth(validator, middleware.ResolveUserID(resolveUserID)(http.HandlerFunc(h)))
	}

	// auth + tenant context + permission check
	permMw := func(perm string, h http.HandlerFunc) http.Handler {
		return middleware.Auth(validator,
			middleware.TenantContext(
				middleware.RequirePermission(checker, perm, resolveUserID)(
					http.HandlerFunc(h))))
	}

	// Announcements (5 routes)
	mux.Handle("POST /api/v1/organizations/{org_id}/announcements", permMw("com.announcement.create", announcementHandler.Create))
	mux.Handle("GET /api/v1/organizations/{org_id}/announcements", permMw("com.announcement.read", announcementHandler.List))
	mux.Handle("GET /api/v1/organizations/{org_id}/announcements/{announcement_id}", permMw("com.announcement.read", announcementHandler.Get))
	mux.Handle("PATCH /api/v1/organizations/{org_id}/announcements/{announcement_id}", permMw("com.announcement.create", announcementHandler.Update))
	mux.Handle("DELETE /api/v1/organizations/{org_id}/announcements/{announcement_id}", permMw("com.announcement.create", announcementHandler.Delete))

	// Threads & Messages (6 routes)
	mux.Handle("POST /api/v1/organizations/{org_id}/threads", permMw("com.thread.create", threadHandler.CreateThread))
	mux.Handle("GET /api/v1/organizations/{org_id}/threads", permMw("com.thread.read", threadHandler.ListThreads))
	mux.Handle("GET /api/v1/organizations/{org_id}/threads/{thread_id}", permMw("com.thread.read", threadHandler.GetThread))
	mux.Handle("POST /api/v1/organizations/{org_id}/threads/{thread_id}/messages", permMw("com.message.send", threadHandler.SendMessage))
	mux.Handle("PATCH /api/v1/organizations/{org_id}/threads/{thread_id}/messages/{message_id}", permMw("com.message.send", threadHandler.EditMessage))
	mux.Handle("DELETE /api/v1/organizations/{org_id}/threads/{thread_id}/messages/{message_id}", permMw("com.message.send", threadHandler.DeleteMessage))

	// Unified Calendar
	mux.Handle("GET /api/v1/organizations/{org_id}/calendar", permMw("com.calendar.read", calendarHandler.GetUnifiedCalendar))

	// Calendar Events (6 routes)
	mux.Handle("POST /api/v1/organizations/{org_id}/calendar-events", permMw("com.calendar_event.create", calendarHandler.Create))
	mux.Handle("GET /api/v1/organizations/{org_id}/calendar-events", permMw("com.calendar_event.read", calendarHandler.List))
	mux.Handle("GET /api/v1/organizations/{org_id}/calendar-events/{event_id}", permMw("com.calendar_event.read", calendarHandler.Get))
	mux.Handle("PATCH /api/v1/organizations/{org_id}/calendar-events/{event_id}", permMw("com.calendar_event.create", calendarHandler.Update))
	mux.Handle("DELETE /api/v1/organizations/{org_id}/calendar-events/{event_id}", permMw("com.calendar_event.create", calendarHandler.Delete))
	mux.Handle("POST /api/v1/organizations/{org_id}/calendar-events/{event_id}/rsvp", permMw("com.calendar_event.read", calendarHandler.RSVP))

	// Templates (4 routes)
	mux.Handle("GET /api/v1/organizations/{org_id}/message-templates", permMw("com.template.manage", calendarHandler.ListTemplates))
	mux.Handle("POST /api/v1/organizations/{org_id}/message-templates", permMw("com.template.manage", calendarHandler.CreateTemplate))
	mux.Handle("PATCH /api/v1/organizations/{org_id}/message-templates/{template_id}", permMw("com.template.manage", calendarHandler.UpdateTemplate))
	mux.Handle("DELETE /api/v1/organizations/{org_id}/message-templates/{template_id}", permMw("com.template.manage", calendarHandler.DeleteTemplate))

	// Directory (2 routes)
	mux.Handle("GET /api/v1/organizations/{org_id}/directory/preferences", permMw("com.directory.read", calendarHandler.GetDirectoryPrefs))
	mux.Handle("PUT /api/v1/organizations/{org_id}/directory/preferences", permMw("com.directory.read", calendarHandler.UpdateDirectoryPrefs))

	// Notification Preferences (2 routes — user-scoped, no org context)
	mux.Handle("GET /api/v1/notification-preferences", authMw(notificationHandler.GetPrefs))
	mux.Handle("PUT /api/v1/notification-preferences", authMw(notificationHandler.UpdatePrefs))

	// Push Tokens (2 routes — user-scoped)
	mux.Handle("POST /api/v1/push-tokens", authMw(notificationHandler.RegisterToken))
	mux.Handle("DELETE /api/v1/push-tokens/{token_id}", authMw(notificationHandler.UnregisterToken))

	// Communication Log (5 routes)
	mux.Handle("POST /api/v1/organizations/{org_id}/communications", permMw("com.comm_log.create", commLogHandler.Log))
	mux.Handle("GET /api/v1/organizations/{org_id}/communications", permMw("com.comm_log.read", commLogHandler.List))
	mux.Handle("GET /api/v1/organizations/{org_id}/communications/{comm_id}", permMw("com.comm_log.read", commLogHandler.Get))
	mux.Handle("PATCH /api/v1/organizations/{org_id}/communications/{comm_id}", permMw("com.comm_log.create", commLogHandler.Update))
	mux.Handle("GET /api/v1/organizations/{org_id}/units/{unit_id}/communications", permMw("com.comm_log.read", commLogHandler.ListByUnit))
	mux.Handle("GET /api/v1/organizations/{org_id}/communications/summary", permMw("com.comm_log.read", commLogHandler.GetCommunicationsSummary))

	// Directory
	mux.Handle("GET /api/v1/organizations/{org_id}/directory", permMw("com.directory.read", calendarHandler.GetDirectory))
}
