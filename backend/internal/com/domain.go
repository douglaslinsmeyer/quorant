// Package com provides domain types and request models for the Communications module.
package com

import (
	"time"

	"github.com/google/uuid"
)

// Announcement represents a pinnable, audience-scoped message from the board or management firm.
type Announcement struct {
	ID            uuid.UUID  `json:"id"`
	OrgID         uuid.UUID  `json:"org_id"`
	AuthorID      uuid.UUID  `json:"author_id"`
	Title         string     `json:"title"`
	Body          string     `json:"body"`
	IsPinned      bool       `json:"is_pinned"`
	AudienceRoles []string   `json:"audience_roles"`
	ScheduledFor  *time.Time `json:"scheduled_for,omitempty"`
	PublishedAt   *time.Time `json:"published_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	DeletedAt     *time.Time `json:"deleted_at,omitempty"`
}

// Thread represents a message conversation within an organization.
type Thread struct {
	ID         uuid.UUID  `json:"id"`
	OrgID      uuid.UUID  `json:"org_id"`
	Subject    string     `json:"subject"`
	ThreadType string     `json:"thread_type"`
	IsClosed   bool       `json:"is_closed"`
	CreatedBy  uuid.UUID  `json:"created_by"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	DeletedAt  *time.Time `json:"deleted_at,omitempty"`
}

// Message represents a single message within a thread.
type Message struct {
	ID            uuid.UUID   `json:"id"`
	ThreadID      uuid.UUID   `json:"thread_id"`
	SenderID      uuid.UUID   `json:"sender_id"`
	Body          string      `json:"body"`
	AttachmentIDs []uuid.UUID `json:"attachment_ids"`
	EditedAt      *time.Time  `json:"edited_at,omitempty"`
	CreatedAt     time.Time   `json:"created_at"`
	DeletedAt     *time.Time  `json:"deleted_at,omitempty"`
}

// NotificationPreference stores a user's channel/event-type notification opt-in for an org.
type NotificationPreference struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	OrgID     uuid.UUID `json:"org_id"`
	Channel   string    `json:"channel"`    // notification_channel enum: push, email, sms
	EventType string    `json:"event_type"` // e.g. "announcement.published"
	Enabled   bool      `json:"enabled"`
}

// UnitNotificationSubscription links a user to unit-level notification events.
type UnitNotificationSubscription struct {
	ID           uuid.UUID `json:"id"`
	UnitID       uuid.UUID `json:"unit_id"`
	UserID       uuid.UUID `json:"user_id"`
	OrgID        uuid.UUID `json:"org_id"`
	Channel      string    `json:"channel"`       // notification_channel enum
	EventPattern string    `json:"event_pattern"` // e.g. "unit.*"
	Enabled      bool      `json:"enabled"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// PushToken represents a device push notification token for a user.
type PushToken struct {
	ID         uuid.UUID  `json:"id"`
	UserID     uuid.UUID  `json:"user_id"`
	Token      string     `json:"token"`
	Platform   string     `json:"platform"` // e.g. "ios", "android", "web"
	DeviceName *string    `json:"device_name,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
}

// CalendarEvent represents a scheduled event for an HOA organization.
type CalendarEvent struct {
	ID             uuid.UUID  `json:"id"`
	OrgID          uuid.UUID  `json:"org_id"`
	Title          string     `json:"title"`
	Description    *string    `json:"description,omitempty"`
	EventType      string     `json:"event_type"`
	Location       *string    `json:"location,omitempty"`
	IsVirtual      bool       `json:"is_virtual"`
	VirtualLink    *string    `json:"virtual_link,omitempty"`
	StartsAt       time.Time  `json:"starts_at"`
	EndsAt         *time.Time `json:"ends_at,omitempty"`
	IsAllDay       bool       `json:"is_all_day"`
	RecurrenceRule *string    `json:"recurrence_rule,omitempty"`
	AudienceRoles  []string   `json:"audience_roles"`
	RSVPEnabled    bool       `json:"rsvp_enabled"`
	RSVPLimit      *int       `json:"rsvp_limit,omitempty"`
	CreatedBy      uuid.UUID  `json:"created_by"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	DeletedAt      *time.Time `json:"deleted_at,omitempty"`
}

// CalendarEventRSVP records a user's RSVP for a calendar event.
type CalendarEventRSVP struct {
	ID         uuid.UUID `json:"id"`
	EventID    uuid.UUID `json:"event_id"`
	UserID     uuid.UUID `json:"user_id"`
	Status     string    `json:"status"` // attending, maybe, declined
	GuestCount int       `json:"guest_count"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// MessageTemplate is a reusable notification template. OrgID is nil for system defaults.
type MessageTemplate struct {
	ID          uuid.UUID  `json:"id"`
	OrgID       *uuid.UUID `json:"org_id,omitempty"`
	TemplateKey string     `json:"template_key"`
	Channel     string     `json:"channel"` // notification_channel enum
	Locale      string     `json:"locale"`
	Subject     *string    `json:"subject,omitempty"`
	Body        string     `json:"body"`
	IsActive    bool       `json:"is_active"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// DirectoryPreference controls whether a user's contact info is visible in the HOA directory.
type DirectoryPreference struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	OrgID     uuid.UUID `json:"org_id"`
	OptIn     bool      `json:"opt_in"`
	ShowEmail bool      `json:"show_email"`
	ShowPhone bool      `json:"show_phone"`
	ShowUnit  bool      `json:"show_unit"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CommunicationLog is an immutable audit record of every communication touchpoint.
type CommunicationLog struct {
	ID              uuid.UUID      `json:"id"`
	OrgID           uuid.UUID      `json:"org_id"`
	Direction       string         `json:"direction"` // comm_direction enum: outbound, inbound
	Channel         string         `json:"channel"`   // comm_channel enum
	ContactUserID   *uuid.UUID     `json:"contact_user_id,omitempty"`
	ContactName     *string        `json:"contact_name,omitempty"`
	ContactInfo     *string        `json:"contact_info,omitempty"`
	InitiatedBy     *uuid.UUID     `json:"initiated_by,omitempty"`
	Subject         *string        `json:"subject,omitempty"`
	Body            *string        `json:"body,omitempty"`
	TemplateID      *uuid.UUID     `json:"template_id,omitempty"`
	AttachmentIDs   []uuid.UUID    `json:"attachment_ids"`
	UnitID          *uuid.UUID     `json:"unit_id,omitempty"`
	ResourceType    *string        `json:"resource_type,omitempty"`
	ResourceID      *uuid.UUID     `json:"resource_id,omitempty"`
	Status          string         `json:"status"` // comm_status enum
	SentAt          *time.Time     `json:"sent_at,omitempty"`
	DeliveredAt     *time.Time     `json:"delivered_at,omitempty"`
	OpenedAt        *time.Time     `json:"opened_at,omitempty"`
	BouncedAt       *time.Time     `json:"bounced_at,omitempty"`
	BounceReason    *string        `json:"bounce_reason,omitempty"`
	DurationMinutes *int           `json:"duration_minutes,omitempty"`
	Source          string         `json:"source"` // e.g. "manual", "automated", "system"
	ProviderRef     *string        `json:"provider_ref,omitempty"`
	Metadata        map[string]any `json:"metadata"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}
