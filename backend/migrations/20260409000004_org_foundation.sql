-- ORG: organizations, management links, memberships

CREATE TABLE organizations (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    parent_id       UUID REFERENCES organizations(id),
    type            org_type NOT NULL,
    name            TEXT NOT NULL,
    slug            TEXT NOT NULL UNIQUE,
    path            LTREE NOT NULL,
    address_line1   TEXT,
    address_line2   TEXT,
    city            TEXT,
    state           TEXT,
    zip             TEXT,
    phone           TEXT,
    email           TEXT,
    website         TEXT,
    logo_url        TEXT,
    settings        JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ
);

CREATE INDEX idx_organizations_parent ON organizations (parent_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_organizations_path ON organizations USING GIST (path);
CREATE INDEX idx_organizations_type ON organizations (type) WHERE deleted_at IS NULL;
CREATE INDEX idx_organizations_slug ON organizations (slug) WHERE deleted_at IS NULL;

CREATE TABLE organizations_management (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    firm_org_id     UUID NOT NULL REFERENCES organizations(id),
    hoa_org_id      UUID NOT NULL REFERENCES organizations(id),
    started_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    ended_at        TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Only one active management firm per HOA at a time
CREATE UNIQUE INDEX idx_org_mgmt_active
    ON organizations_management (hoa_org_id)
    WHERE ended_at IS NULL;

CREATE INDEX idx_org_mgmt_firm ON organizations_management (firm_org_id) WHERE ended_at IS NULL;

CREATE TABLE memberships (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id),
    org_id          UUID NOT NULL REFERENCES organizations(id),
    role_id         UUID NOT NULL REFERENCES roles(id),
    status          membership_status NOT NULL DEFAULT 'active',
    invited_by      UUID REFERENCES users(id),
    joined_at       TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ
);

-- A user can have only one active role per org
CREATE UNIQUE INDEX idx_memberships_user_org_role
    ON memberships (user_id, org_id, role_id)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_memberships_org ON memberships (org_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_memberships_user ON memberships (user_id) WHERE deleted_at IS NULL;
