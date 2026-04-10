package com

import (
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/platform/api"
)

// CreateAnnouncementRequest is the request body for POST /api/v1/orgs/{org_id}/announcements.
type CreateAnnouncementRequest struct {
	Title         string     `json:"title"`          // required
	Body          string     `json:"body"`           // required
	IsPinned      bool       `json:"is_pinned"`
	AudienceRoles []string   `json:"audience_roles,omitempty"`
	ScheduledFor  *time.Time `json:"scheduled_for,omitempty"`
}

// Validate checks that Title and Body are present.
func (r CreateAnnouncementRequest) Validate() error {
	if r.Title == "" {
		return api.NewValidationError("validation.required", "title", api.P("field", "title"))
	}
	if r.Body == "" {
		return api.NewValidationError("validation.required", "body", api.P("field", "body"))
	}
	return nil
}

// CreateThreadRequest is the request body for POST /api/v1/orgs/{org_id}/threads.
type CreateThreadRequest struct {
	Subject    string `json:"subject"`     // required
	ThreadType string `json:"thread_type,omitempty"`
}

// Validate checks that Subject is present.
func (r CreateThreadRequest) Validate() error {
	if r.Subject == "" {
		return api.NewValidationError("validation.required", "subject", api.P("field", "subject"))
	}
	return nil
}

// SendMessageRequest is the request body for POST /api/v1/orgs/{org_id}/threads/{thread_id}/messages.
type SendMessageRequest struct {
	Body string `json:"body"` // required
}

// Validate checks that Body is present.
func (r SendMessageRequest) Validate() error {
	if r.Body == "" {
		return api.NewValidationError("validation.required", "body", api.P("field", "body"))
	}
	return nil
}

// CreateCalendarEventRequest is the request body for POST /api/v1/orgs/{org_id}/calendar.
type CreateCalendarEventRequest struct {
	Title       string     `json:"title"`      // required
	EventType   string     `json:"event_type"` // required
	StartsAt    time.Time  `json:"starts_at"`  // required (zero value means unset)
	Description *string    `json:"description,omitempty"`
	Location    *string    `json:"location,omitempty"`
	EndsAt      *time.Time `json:"ends_at,omitempty"`
	IsAllDay    bool       `json:"is_all_day"`
	RSVPEnabled bool       `json:"rsvp_enabled"`
}

// Validate checks that Title, EventType, and StartsAt are present.
func (r CreateCalendarEventRequest) Validate() error {
	if r.Title == "" {
		return api.NewValidationError("validation.required", "title", api.P("field", "title"))
	}
	if r.EventType == "" {
		return api.NewValidationError("validation.required", "event_type", api.P("field", "event_type"))
	}
	if r.StartsAt.IsZero() {
		return api.NewValidationError("validation.required", "starts_at", api.P("field", "starts_at"))
	}
	return nil
}

// RSVPRequest is the request body for PUT /api/v1/orgs/{org_id}/calendar/{event_id}/rsvp.
type RSVPRequest struct {
	Status     string `json:"status"`      // required: attending, maybe, declined
	GuestCount int    `json:"guest_count"`
}

// Validate checks that Status is one of the allowed values.
func (r RSVPRequest) Validate() error {
	switch r.Status {
	case "attending", "maybe", "declined":
		// valid
	default:
		return api.NewValidationError("validation.one_of", "status", api.P("field", "status"), api.P("values", "attending, maybe, declined"))
	}
	return nil
}

// CreateTemplateRequest is the request body for POST /api/v1/orgs/{org_id}/templates.
type CreateTemplateRequest struct {
	TemplateKey string  `json:"template_key"` // required
	Channel     string  `json:"channel"`      // required
	Body        string  `json:"body"`         // required
	Subject     *string `json:"subject,omitempty"`
	Locale      string  `json:"locale,omitempty"` // defaults to "en_US"
}

// Validate checks that TemplateKey, Channel, and Body are present.
// If Locale is empty it defaults to "en_US".
func (r *CreateTemplateRequest) Validate() error {
	if r.TemplateKey == "" {
		return api.NewValidationError("validation.required", "template_key", api.P("field", "template_key"))
	}
	if r.Channel == "" {
		return api.NewValidationError("validation.required", "channel", api.P("field", "channel"))
	}
	if r.Body == "" {
		return api.NewValidationError("validation.required", "body", api.P("field", "body"))
	}
	if r.Locale == "" {
		r.Locale = "en_US"
	}
	return nil
}

// UpdateDirectoryPreferenceRequest is the request body for
// PUT /api/v1/orgs/{org_id}/directory/preferences.
type UpdateDirectoryPreferenceRequest struct {
	OptIn     *bool `json:"opt_in,omitempty"`
	ShowEmail *bool `json:"show_email,omitempty"`
	ShowPhone *bool `json:"show_phone,omitempty"`
	ShowUnit  *bool `json:"show_unit,omitempty"`
}

// Validate checks that at least one field is provided.
func (r UpdateDirectoryPreferenceRequest) Validate() error {
	if r.OptIn == nil && r.ShowEmail == nil && r.ShowPhone == nil && r.ShowUnit == nil {
		return api.NewValidationError("validation.at_least_one", "")
	}
	return nil
}

// LogCommunicationRequest is the request body for POST /api/v1/orgs/{org_id}/communications.
type LogCommunicationRequest struct {
	Direction     string     `json:"direction"` // required: outbound, inbound
	Channel       string     `json:"channel"`   // required
	ContactUserID *uuid.UUID `json:"contact_user_id,omitempty"`
	ContactName   *string    `json:"contact_name,omitempty"`
	Subject       *string    `json:"subject,omitempty"`
	Body          *string    `json:"body,omitempty"`
}

// Validate checks that Direction, Channel, and at least one contact identifier are present.
func (r LogCommunicationRequest) Validate() error {
	if r.Direction == "" {
		return api.NewValidationError("validation.required", "direction", api.P("field", "direction"))
	}
	if r.Channel == "" {
		return api.NewValidationError("validation.required", "channel", api.P("field", "channel"))
	}
	if r.ContactUserID == nil && r.ContactName == nil {
		return api.NewValidationError("validation.required", "contact_name", api.P("field", "contact_name or contact_user_id"))
	}
	return nil
}
