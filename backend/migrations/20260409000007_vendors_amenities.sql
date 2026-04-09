-- Migration: 20260409000007_vendors_amenities.sql
-- Description: Create vendor, amenity, and unit registration tables

-- vendors
CREATE TABLE vendors (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            TEXT NOT NULL,
    contact_email   TEXT,
    contact_phone   TEXT,
    service_types   TEXT[] NOT NULL DEFAULT '{}',
    license_number  TEXT,
    insurance_expiry TIMESTAMPTZ,
    metadata        JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ
);

-- vendor_assignments
CREATE TABLE vendor_assignments (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    vendor_id       UUID NOT NULL REFERENCES vendors(id),
    org_id          UUID NOT NULL REFERENCES organizations(id),
    service_scope   TEXT NOT NULL,
    contract_ref    TEXT,
    started_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    ended_at        TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_vendor_assignments_org ON vendor_assignments (org_id) WHERE ended_at IS NULL;
CREATE INDEX idx_vendor_assignments_vendor ON vendor_assignments (vendor_id) WHERE ended_at IS NULL;

-- amenities
CREATE TABLE amenities (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL REFERENCES organizations(id),
    name            TEXT NOT NULL,
    amenity_type    TEXT NOT NULL,
    description     TEXT,
    location        TEXT,
    capacity        INTEGER,
    is_reservable   BOOLEAN NOT NULL DEFAULT FALSE,
    reservation_rules JSONB NOT NULL DEFAULT '{}',
    fee_cents       BIGINT,
    hours           JSONB NOT NULL DEFAULT '{}',
    status          TEXT NOT NULL DEFAULT 'open',
    metadata        JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ
);

CREATE INDEX idx_amenities_org ON amenities (org_id) WHERE deleted_at IS NULL;

-- amenity_reservations
CREATE TABLE amenity_reservations (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    amenity_id      UUID NOT NULL REFERENCES amenities(id),
    org_id          UUID NOT NULL REFERENCES organizations(id),
    user_id         UUID NOT NULL REFERENCES users(id),
    unit_id         UUID NOT NULL REFERENCES units(id),
    status          reservation_status NOT NULL DEFAULT 'pending',
    starts_at       TIMESTAMPTZ NOT NULL,
    ends_at         TIMESTAMPTZ NOT NULL,
    guest_count     INTEGER,
    fee_cents       BIGINT,
    deposit_cents   BIGINT,
    deposit_refunded BOOLEAN DEFAULT FALSE,
    notes           TEXT,
    cancelled_at    TIMESTAMPTZ,
    cancelled_by    UUID REFERENCES users(id),
    cancellation_reason TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_reservations_amenity_time
    ON amenity_reservations (amenity_id, starts_at, ends_at)
    WHERE status IN ('pending', 'confirmed');

CREATE INDEX idx_reservations_user ON amenity_reservations (user_id, starts_at DESC);
CREATE INDEX idx_reservations_org ON amenity_reservations (org_id, starts_at DESC);

-- unit_registration_types
CREATE TABLE unit_registration_types (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL REFERENCES organizations(id),
    name            TEXT NOT NULL,
    slug            TEXT NOT NULL,
    schema          JSONB NOT NULL DEFAULT '{}',
    max_per_unit    INTEGER,
    requires_approval BOOLEAN NOT NULL DEFAULT FALSE,
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (org_id, slug)
);

-- unit_registrations
CREATE TABLE unit_registrations (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL REFERENCES organizations(id),
    unit_id         UUID NOT NULL REFERENCES units(id),
    user_id         UUID NOT NULL REFERENCES users(id),
    registration_type_id UUID NOT NULL REFERENCES unit_registration_types(id),
    data            JSONB NOT NULL,
    status          TEXT NOT NULL DEFAULT 'active',
    approved_by     UUID REFERENCES users(id),
    approved_at     TIMESTAMPTZ,
    expires_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ
);

CREATE INDEX idx_unit_registrations_unit ON unit_registrations (unit_id, registration_type_id)
    WHERE deleted_at IS NULL;
CREATE INDEX idx_unit_registrations_org ON unit_registrations (org_id, registration_type_id)
    WHERE deleted_at IS NULL;
