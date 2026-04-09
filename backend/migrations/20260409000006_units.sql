-- Migration: 20260409000006_units.sql
-- Description: Create unit, property, and unit membership tables

-- units table
CREATE TABLE units (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL REFERENCES organizations(id),
    label           TEXT NOT NULL,
    unit_type       TEXT,
    address_line1   TEXT,
    address_line2   TEXT,
    city            TEXT,
    state           TEXT,
    zip             TEXT,
    status          TEXT NOT NULL DEFAULT 'occupied',
    lot_size_sqft   INTEGER,
    voting_weight   NUMERIC(5,2) NOT NULL DEFAULT 1.00,
    metadata        JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ
);

CREATE INDEX idx_units_org ON units (org_id) WHERE deleted_at IS NULL;

-- properties table
CREATE TABLE properties (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    unit_id         UUID NOT NULL REFERENCES units(id),
    parcel_number   TEXT,
    square_feet     INTEGER,
    bedrooms        SMALLINT,
    bathrooms       NUMERIC(3,1),
    year_built      SMALLINT,
    metadata        JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_properties_unit ON properties (unit_id);

-- unit_memberships table
CREATE TABLE unit_memberships (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    unit_id         UUID NOT NULL REFERENCES units(id),
    user_id         UUID NOT NULL REFERENCES users(id),
    relationship    unit_relationship NOT NULL,
    is_voter        BOOLEAN NOT NULL DEFAULT FALSE,
    started_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    ended_at        TIMESTAMPTZ,
    notes           TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_vote_requires_owner
        CHECK (is_voter = FALSE OR relationship = 'owner')
);

CREATE UNIQUE INDEX idx_unit_memberships_voter
    ON unit_memberships (unit_id)
    WHERE is_voter = TRUE AND ended_at IS NULL;

CREATE INDEX idx_unit_memberships_unit
    ON unit_memberships (unit_id)
    WHERE ended_at IS NULL;

CREATE INDEX idx_unit_memberships_user
    ON unit_memberships (user_id)
    WHERE ended_at IS NULL;

CREATE UNIQUE INDEX idx_unit_memberships_unique_active
    ON unit_memberships (unit_id, user_id, relationship)
    WHERE ended_at IS NULL;

-- unit_ownership_history table
CREATE TABLE unit_ownership_history (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    unit_id         UUID NOT NULL REFERENCES units(id),
    org_id          UUID NOT NULL REFERENCES organizations(id),
    from_user_id    UUID REFERENCES users(id),
    to_user_id      UUID NOT NULL REFERENCES users(id),
    transfer_type   TEXT NOT NULL,
    transfer_date   DATE NOT NULL,
    sale_price_cents BIGINT,
    outstanding_balance_cents BIGINT,
    balance_settled BOOLEAN NOT NULL DEFAULT FALSE,
    recording_ref   TEXT,
    notes           TEXT,
    recorded_by     UUID NOT NULL REFERENCES users(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_ownership_history_unit ON unit_ownership_history (unit_id, transfer_date DESC);
CREATE INDEX idx_ownership_history_org ON unit_ownership_history (org_id, transfer_date DESC);
