-- Migration: 20260409000011_gov_tables.sql
-- Description: Create governance tables: violations, ARB, ballots, meetings, hearings

-- violations
CREATE TABLE violations (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL REFERENCES organizations(id),
    unit_id         UUID NOT NULL REFERENCES units(id),
    reported_by     UUID NOT NULL REFERENCES users(id),
    assigned_to     UUID REFERENCES users(id),
    title           TEXT NOT NULL,
    description     TEXT NOT NULL,
    category        TEXT NOT NULL,
    status          violation_status NOT NULL DEFAULT 'open',
    severity        SMALLINT NOT NULL DEFAULT 1,
    due_date        DATE,
    governing_doc_id UUID,  -- FK to governing_documents added in AI phase
    governing_section TEXT,
    offense_number  SMALLINT,
    cure_deadline   DATE,
    cure_verified_at TIMESTAMPTZ,
    cure_verified_by UUID REFERENCES users(id),
    fine_total_cents BIGINT NOT NULL DEFAULT 0,
    resolved_at     TIMESTAMPTZ,
    evidence_doc_ids UUID[] DEFAULT '{}',
    metadata        JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ
);

CREATE INDEX idx_violations_org ON violations (org_id, status) WHERE deleted_at IS NULL;
CREATE INDEX idx_violations_unit ON violations (unit_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_violations_offense ON violations (unit_id, category, created_at DESC) WHERE deleted_at IS NULL;

-- violation_actions
CREATE TABLE violation_actions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    violation_id    UUID NOT NULL REFERENCES violations(id),
    actor_id        UUID NOT NULL REFERENCES users(id),
    action_type     TEXT NOT NULL,
    notes           TEXT,
    metadata        JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_violation_actions_violation ON violation_actions (violation_id, created_at);

-- arb_requests
CREATE TABLE arb_requests (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL REFERENCES organizations(id),
    unit_id         UUID NOT NULL REFERENCES units(id),
    submitted_by    UUID NOT NULL REFERENCES users(id),
    title           TEXT NOT NULL,
    description     TEXT NOT NULL,
    category        TEXT NOT NULL,
    status          arb_status NOT NULL DEFAULT 'submitted',
    reviewed_by     UUID REFERENCES users(id),
    decision_notes  TEXT,
    decided_at      TIMESTAMPTZ,
    supporting_doc_ids UUID[] DEFAULT '{}',
    governing_doc_id UUID,  -- FK to governing_documents added later
    governing_section TEXT,
    review_deadline TIMESTAMPTZ,
    auto_approved   BOOLEAN NOT NULL DEFAULT FALSE,
    conditions      JSONB NOT NULL DEFAULT '[]',
    revision_count  SMALLINT NOT NULL DEFAULT 0,
    metadata        JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ
);

CREATE INDEX idx_arb_requests_org ON arb_requests (org_id, status) WHERE deleted_at IS NULL;
CREATE INDEX idx_arb_requests_deadline ON arb_requests (review_deadline)
    WHERE status IN ('submitted', 'under_review') AND auto_approved = FALSE;

-- arb_votes
CREATE TABLE arb_votes (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    arb_request_id  UUID NOT NULL REFERENCES arb_requests(id),
    voter_id        UUID NOT NULL REFERENCES users(id),
    vote            TEXT NOT NULL,
    notes           TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (arb_request_id, voter_id)
);

-- ballots
CREATE TABLE ballots (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL REFERENCES organizations(id),
    title           TEXT NOT NULL,
    description     TEXT NOT NULL,
    ballot_type     TEXT NOT NULL,
    status          ballot_status NOT NULL DEFAULT 'draft',
    options         JSONB NOT NULL DEFAULT '[]',
    eligible_role   TEXT NOT NULL DEFAULT 'homeowner',
    opens_at        TIMESTAMPTZ NOT NULL,
    closes_at       TIMESTAMPTZ NOT NULL,
    quorum_percent  NUMERIC(5,2),
    pass_percent    NUMERIC(5,2),
    eligible_units  INTEGER,
    votes_cast      INTEGER NOT NULL DEFAULT 0,
    quorum_met      BOOLEAN,
    weight_method   TEXT NOT NULL DEFAULT 'equal',
    results         JSONB,
    created_by      UUID NOT NULL REFERENCES users(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ
);

CREATE INDEX idx_ballots_org ON ballots (org_id, status) WHERE deleted_at IS NULL;

-- ballot_votes
CREATE TABLE ballot_votes (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ballot_id       UUID NOT NULL REFERENCES ballots(id),
    voter_id        UUID NOT NULL REFERENCES users(id),
    unit_id         UUID NOT NULL REFERENCES units(id),
    selection       JSONB NOT NULL,
    vote_weight     NUMERIC(5,2) NOT NULL DEFAULT 1.00,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (ballot_id, unit_id)
);

-- proxy_authorizations
CREATE TABLE proxy_authorizations (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ballot_id       UUID NOT NULL REFERENCES ballots(id),
    unit_id         UUID NOT NULL REFERENCES units(id),
    grantor_id      UUID NOT NULL REFERENCES users(id),
    proxy_id        UUID NOT NULL REFERENCES users(id),
    filed_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at      TIMESTAMPTZ,
    document_id     UUID,  -- FK to documents added later
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (ballot_id, unit_id)
);

CREATE INDEX idx_proxy_auth_ballot ON proxy_authorizations (ballot_id) WHERE revoked_at IS NULL;

-- meetings
CREATE TABLE meetings (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL REFERENCES organizations(id),
    title           TEXT NOT NULL,
    meeting_type    TEXT NOT NULL,
    status          meeting_status NOT NULL DEFAULT 'scheduled',
    scheduled_at    TIMESTAMPTZ NOT NULL,
    ended_at        TIMESTAMPTZ,
    location        TEXT,
    is_virtual      BOOLEAN NOT NULL DEFAULT FALSE,
    virtual_link    TEXT,
    notice_required_days SMALLINT,
    notice_sent_at  TIMESTAMPTZ,
    quorum_required SMALLINT,
    quorum_present  SMALLINT,
    quorum_met      BOOLEAN,
    agenda_doc_id   UUID,  -- FK to documents added later
    minutes_doc_id  UUID,  -- FK to documents added later
    created_by      UUID NOT NULL REFERENCES users(id),
    metadata        JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ
);

CREATE INDEX idx_meetings_org ON meetings (org_id, scheduled_at DESC) WHERE deleted_at IS NULL;

-- meeting_attendees
CREATE TABLE meeting_attendees (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    meeting_id      UUID NOT NULL REFERENCES meetings(id) ON DELETE CASCADE,
    user_id         UUID NOT NULL REFERENCES users(id),
    role            TEXT NOT NULL DEFAULT 'member',
    rsvp_status     TEXT,
    attended        BOOLEAN,
    arrived_at      TIMESTAMPTZ,
    left_at         TIMESTAMPTZ,
    UNIQUE (meeting_id, user_id)
);

CREATE INDEX idx_meeting_attendees_meeting ON meeting_attendees (meeting_id);

-- meeting_motions
CREATE TABLE meeting_motions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    meeting_id      UUID NOT NULL REFERENCES meetings(id) ON DELETE CASCADE,
    motion_number   SMALLINT NOT NULL,
    title           TEXT NOT NULL,
    description     TEXT,
    moved_by        UUID NOT NULL REFERENCES users(id),
    seconded_by     UUID REFERENCES users(id),
    status          TEXT NOT NULL DEFAULT 'pending',
    votes_for       SMALLINT,
    votes_against   SMALLINT,
    votes_abstain   SMALLINT,
    result_notes    TEXT,
    resource_type   TEXT,
    resource_id     UUID,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_meeting_motions_meeting ON meeting_motions (meeting_id, motion_number);

-- hearing_links
CREATE TABLE hearing_links (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    meeting_id      UUID NOT NULL REFERENCES meetings(id),
    violation_id    UUID NOT NULL REFERENCES violations(id),
    homeowner_notified_at TIMESTAMPTZ,
    notice_doc_id   UUID,  -- FK to documents added later
    homeowner_attended BOOLEAN,
    homeowner_statement TEXT,
    board_finding   TEXT,
    fine_upheld     BOOLEAN,
    fine_modified_cents BIGINT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_hearing_links_violation ON hearing_links (violation_id);
CREATE INDEX idx_hearing_links_meeting ON hearing_links (meeting_id);
