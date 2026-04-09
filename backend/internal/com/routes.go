package com

import (
	"net/http"

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
) {
	orgMw := func(h http.HandlerFunc) http.Handler {
		return middleware.Auth(validator, middleware.TenantContext(http.HandlerFunc(h)))
	}
	authMw := func(h http.HandlerFunc) http.Handler {
		return middleware.Auth(validator, http.HandlerFunc(h))
	}

	// Announcements (5 routes)
	mux.Handle("POST /api/v1/organizations/{org_id}/announcements", orgMw(announcementHandler.Create))
	mux.Handle("GET /api/v1/organizations/{org_id}/announcements", orgMw(announcementHandler.List))
	mux.Handle("GET /api/v1/organizations/{org_id}/announcements/{announcement_id}", orgMw(announcementHandler.Get))
	mux.Handle("PATCH /api/v1/organizations/{org_id}/announcements/{announcement_id}", orgMw(announcementHandler.Update))
	mux.Handle("DELETE /api/v1/organizations/{org_id}/announcements/{announcement_id}", orgMw(announcementHandler.Delete))

	// Threads & Messages (6 routes)
	mux.Handle("POST /api/v1/organizations/{org_id}/threads", orgMw(threadHandler.CreateThread))
	mux.Handle("GET /api/v1/organizations/{org_id}/threads", orgMw(threadHandler.ListThreads))
	mux.Handle("GET /api/v1/organizations/{org_id}/threads/{thread_id}", orgMw(threadHandler.GetThread))
	mux.Handle("POST /api/v1/organizations/{org_id}/threads/{thread_id}/messages", orgMw(threadHandler.SendMessage))
	mux.Handle("PATCH /api/v1/organizations/{org_id}/threads/{thread_id}/messages/{message_id}", orgMw(threadHandler.EditMessage))
	mux.Handle("DELETE /api/v1/organizations/{org_id}/threads/{thread_id}/messages/{message_id}", orgMw(threadHandler.DeleteMessage))

	// Calendar Events (6 routes)
	mux.Handle("POST /api/v1/organizations/{org_id}/calendar-events", orgMw(calendarHandler.Create))
	mux.Handle("GET /api/v1/organizations/{org_id}/calendar-events", orgMw(calendarHandler.List))
	mux.Handle("GET /api/v1/organizations/{org_id}/calendar-events/{event_id}", orgMw(calendarHandler.Get))
	mux.Handle("PATCH /api/v1/organizations/{org_id}/calendar-events/{event_id}", orgMw(calendarHandler.Update))
	mux.Handle("DELETE /api/v1/organizations/{org_id}/calendar-events/{event_id}", orgMw(calendarHandler.Delete))
	mux.Handle("POST /api/v1/organizations/{org_id}/calendar-events/{event_id}/rsvp", orgMw(calendarHandler.RSVP))

	// Templates (4 routes)
	mux.Handle("GET /api/v1/organizations/{org_id}/message-templates", orgMw(calendarHandler.ListTemplates))
	mux.Handle("POST /api/v1/organizations/{org_id}/message-templates", orgMw(calendarHandler.CreateTemplate))
	mux.Handle("PATCH /api/v1/organizations/{org_id}/message-templates/{template_id}", orgMw(calendarHandler.UpdateTemplate))
	mux.Handle("DELETE /api/v1/organizations/{org_id}/message-templates/{template_id}", orgMw(calendarHandler.DeleteTemplate))

	// Directory (2 routes)
	mux.Handle("GET /api/v1/organizations/{org_id}/directory/preferences", orgMw(calendarHandler.GetDirectoryPrefs))
	mux.Handle("PUT /api/v1/organizations/{org_id}/directory/preferences", orgMw(calendarHandler.UpdateDirectoryPrefs))

	// Notification Preferences (2 routes — user-scoped, no org context)
	mux.Handle("GET /api/v1/notification-preferences", authMw(notificationHandler.GetPrefs))
	mux.Handle("PUT /api/v1/notification-preferences", authMw(notificationHandler.UpdatePrefs))

	// Push Tokens (2 routes — user-scoped)
	mux.Handle("POST /api/v1/push-tokens", authMw(notificationHandler.RegisterToken))
	mux.Handle("DELETE /api/v1/push-tokens/{token_id}", authMw(notificationHandler.UnregisterToken))

	// Communication Log (5 routes)
	mux.Handle("POST /api/v1/organizations/{org_id}/communications", orgMw(commLogHandler.Log))
	mux.Handle("GET /api/v1/organizations/{org_id}/communications", orgMw(commLogHandler.List))
	mux.Handle("GET /api/v1/organizations/{org_id}/communications/{comm_id}", orgMw(commLogHandler.Get))
	mux.Handle("PATCH /api/v1/organizations/{org_id}/communications/{comm_id}", orgMw(commLogHandler.Update))
	mux.Handle("GET /api/v1/organizations/{org_id}/units/{unit_id}/communications", orgMw(commLogHandler.ListByUnit))
}
