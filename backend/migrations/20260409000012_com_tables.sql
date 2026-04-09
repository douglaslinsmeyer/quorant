-- Migration: 20260409000012_com_tables.sql
-- Description: Create all communications tables for the Quorant platform

-- announcements
CREATE TABLE announcements (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL REFERENCES organizations(id),
    author_id       UUID NOT NULL REFERENCES users(id),
    title           TEXT NOT NULL,
    body            TEXT NOT NULL,
    is_pinned       BOOLEAN NOT NULL DEFAULT FALSE,
    audience_roles  TEXT[] NOT NULL DEFAULT '{}',
    scheduled_for   TIMESTAMPTZ,
    published_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ
);

CREATE INDEX idx_announcements_org ON announcements (org_id, published_at DESC) WHERE deleted_at IS NULL;

-- threads
CREATE TABLE threads (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL REFERENCES organizations(id),
    subject         TEXT NOT NULL,
    thread_type     TEXT NOT NULL DEFAULT 'general',
    is_closed       BOOLEAN NOT NULL DEFAULT FALSE,
    created_by      UUID NOT NULL REFERENCES users(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ
);

CREATE INDEX idx_threads_org ON threads (org_id, updated_at DESC) WHERE deleted_at IS NULL;

-- messages
CREATE TABLE messages (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    thread_id       UUID NOT NULL REFERENCES threads(id),
    sender_id       UUID NOT NULL REFERENCES users(id),
    body            TEXT NOT NULL,
    attachment_ids  UUID[] DEFAULT '{}',
    edited_at       TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ
);

CREATE INDEX idx_messages_thread ON messages (thread_id, created_at) WHERE deleted_at IS NULL;

-- notification_preferences
CREATE TABLE notification_preferences (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id),
    org_id          UUID NOT NULL REFERENCES organizations(id),
    channel         notification_channel NOT NULL,
    event_type      TEXT NOT NULL,
    enabled         BOOLEAN NOT NULL DEFAULT TRUE,
    UNIQUE (user_id, org_id, channel, event_type)
);

-- unit_notification_subscriptions
CREATE TABLE unit_notification_subscriptions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    unit_id         UUID NOT NULL REFERENCES units(id),
    user_id         UUID NOT NULL REFERENCES users(id),
    org_id          UUID NOT NULL REFERENCES organizations(id),
    channel         notification_channel NOT NULL,
    event_pattern   TEXT NOT NULL,
    enabled         BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (unit_id, user_id, channel, event_pattern)
);

CREATE INDEX idx_unit_notif_subs_unit ON unit_notification_subscriptions (unit_id) WHERE enabled = TRUE;
CREATE INDEX idx_unit_notif_subs_user ON unit_notification_subscriptions (user_id) WHERE enabled = TRUE;

-- push_tokens
CREATE TABLE push_tokens (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id),
    token           TEXT NOT NULL UNIQUE,
    platform        TEXT NOT NULL,
    device_name     TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_used_at    TIMESTAMPTZ
);

CREATE INDEX idx_push_tokens_user ON push_tokens (user_id);

-- calendar_events
CREATE TABLE calendar_events (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL REFERENCES organizations(id),
    title           TEXT NOT NULL,
    description     TEXT,
    event_type      TEXT NOT NULL,
    location        TEXT,
    is_virtual      BOOLEAN NOT NULL DEFAULT FALSE,
    virtual_link    TEXT,
    starts_at       TIMESTAMPTZ NOT NULL,
    ends_at         TIMESTAMPTZ,
    is_all_day      BOOLEAN NOT NULL DEFAULT FALSE,
    recurrence_rule TEXT,
    audience_roles  TEXT[] NOT NULL DEFAULT '{}',
    rsvp_enabled    BOOLEAN NOT NULL DEFAULT FALSE,
    rsvp_limit      INTEGER,
    created_by      UUID NOT NULL REFERENCES users(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ
);

CREATE INDEX idx_calendar_events_org ON calendar_events (org_id, starts_at) WHERE deleted_at IS NULL;

-- calendar_event_rsvps
CREATE TABLE calendar_event_rsvps (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id        UUID NOT NULL REFERENCES calendar_events(id) ON DELETE CASCADE,
    user_id         UUID NOT NULL REFERENCES users(id),
    status          TEXT NOT NULL DEFAULT 'attending',
    guest_count     SMALLINT NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (event_id, user_id)
);

CREATE INDEX idx_calendar_event_rsvps_event ON calendar_event_rsvps (event_id);

-- message_templates
-- org_id is nullable: NULL means a system-default template
CREATE TABLE message_templates (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID REFERENCES organizations(id),
    template_key    TEXT NOT NULL,
    channel         notification_channel NOT NULL,
    subject         TEXT,
    body            TEXT NOT NULL,
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_message_templates_key
    ON message_templates (COALESCE(org_id, '00000000-0000-0000-0000-000000000000'), template_key, channel);

-- directory_preferences
CREATE TABLE directory_preferences (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id),
    org_id          UUID NOT NULL REFERENCES organizations(id),
    opt_in          BOOLEAN NOT NULL DEFAULT TRUE,
    show_email      BOOLEAN NOT NULL DEFAULT FALSE,
    show_phone      BOOLEAN NOT NULL DEFAULT FALSE,
    show_unit       BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, org_id)
);

-- communication_log
CREATE TABLE communication_log (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL REFERENCES organizations(id),
    direction       comm_direction NOT NULL,
    channel         comm_channel NOT NULL,
    contact_user_id UUID REFERENCES users(id),
    contact_name    TEXT,
    contact_info    TEXT,
    initiated_by    UUID REFERENCES users(id),
    subject         TEXT,
    body            TEXT,
    template_id     UUID REFERENCES message_templates(id),
    attachment_ids  UUID[] DEFAULT '{}',
    unit_id         UUID REFERENCES units(id),
    resource_type   TEXT,
    resource_id     UUID,
    status          comm_status NOT NULL DEFAULT 'sent',
    sent_at         TIMESTAMPTZ,
    delivered_at    TIMESTAMPTZ,
    opened_at       TIMESTAMPTZ,
    bounced_at      TIMESTAMPTZ,
    bounce_reason   TEXT,
    duration_minutes INTEGER,
    source          TEXT NOT NULL DEFAULT 'manual',
    provider_ref    TEXT,
    metadata        JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_comm_log_unit ON communication_log (unit_id, created_at DESC) WHERE unit_id IS NOT NULL;
CREATE INDEX idx_comm_log_resource ON communication_log (resource_type, resource_id, created_at DESC) WHERE resource_type IS NOT NULL;
CREATE INDEX idx_comm_log_org ON communication_log (org_id, created_at DESC);
CREATE INDEX idx_comm_log_contact ON communication_log (contact_user_id, created_at DESC) WHERE contact_user_id IS NOT NULL;
CREATE INDEX idx_comm_log_failures ON communication_log (org_id, status) WHERE status IN ('bounced', 'failed', 'returned_to_sender');
