-- Migration: 20260409000023_gl_tables.sql
-- Description: Create general ledger tables for double-entry bookkeeping:
--              chart of accounts (gl_accounts), journal entries (gl_journal_entries),
--              and journal lines (gl_journal_lines).

-- ---------------------------------------------------------------------------
-- gl_accounts (Chart of Accounts)
-- ---------------------------------------------------------------------------
CREATE TABLE gl_accounts (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL REFERENCES organizations(id),
    parent_id       UUID REFERENCES gl_accounts(id),
    fund_id         UUID REFERENCES funds(id),
    account_number  INT NOT NULL,
    name            TEXT NOT NULL,
    account_type    TEXT NOT NULL,
    is_header       BOOLEAN NOT NULL DEFAULT FALSE,
    is_system       BOOLEAN NOT NULL DEFAULT FALSE,
    description     TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ,
    CONSTRAINT gl_accounts_type_check CHECK (
        account_type IN ('asset', 'liability', 'equity', 'revenue', 'expense')
    )
);

CREATE UNIQUE INDEX idx_gl_accounts_org_number
    ON gl_accounts (org_id, account_number)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_gl_accounts_org
    ON gl_accounts (org_id)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_gl_accounts_parent
    ON gl_accounts (parent_id)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_gl_accounts_fund
    ON gl_accounts (fund_id)
    WHERE deleted_at IS NULL;

-- ---------------------------------------------------------------------------
-- gl_journal_entries (Transaction Headers)
-- ---------------------------------------------------------------------------
CREATE TABLE gl_journal_entries (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL REFERENCES organizations(id),
    entry_number    INT NOT NULL,
    entry_date      DATE NOT NULL,
    memo            TEXT NOT NULL,
    source_type     TEXT,
    source_id       UUID,
    unit_id         UUID REFERENCES units(id),
    posted_by       UUID NOT NULL REFERENCES users(id),
    reversed_by     UUID,
    is_reversal     BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_gl_journal_entries_org_number
    ON gl_journal_entries (org_id, entry_number);

CREATE INDEX idx_gl_journal_entries_org_date
    ON gl_journal_entries (org_id, entry_date DESC);

CREATE INDEX idx_gl_journal_entries_source
    ON gl_journal_entries (source_type, source_id)
    WHERE source_id IS NOT NULL;

CREATE INDEX idx_gl_journal_entries_unit
    ON gl_journal_entries (unit_id)
    WHERE unit_id IS NOT NULL;

-- ---------------------------------------------------------------------------
-- gl_journal_lines (Debit/Credit Lines)
-- ---------------------------------------------------------------------------
CREATE TABLE gl_journal_lines (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    journal_entry_id    UUID NOT NULL REFERENCES gl_journal_entries(id) ON DELETE CASCADE,
    account_id          UUID NOT NULL REFERENCES gl_accounts(id),
    debit_cents         BIGINT NOT NULL DEFAULT 0,
    credit_cents        BIGINT NOT NULL DEFAULT 0,
    memo                TEXT,
    CONSTRAINT gl_journal_lines_one_side CHECK (
        (debit_cents > 0 AND credit_cents = 0) OR (debit_cents = 0 AND credit_cents > 0)
    )
);

CREATE INDEX idx_gl_journal_lines_entry ON gl_journal_lines (journal_entry_id);
CREATE INDEX idx_gl_journal_lines_account ON gl_journal_lines (account_id);

-- ---------------------------------------------------------------------------
-- Row-Level Security
-- gl_journal_lines does not need RLS — accessed via RLS-protected
-- gl_journal_entries through JOINs.
-- ---------------------------------------------------------------------------
ALTER TABLE gl_accounts ENABLE ROW LEVEL SECURITY;
ALTER TABLE gl_accounts FORCE ROW LEVEL SECURITY;
CREATE POLICY gl_accounts_tenant_isolation ON gl_accounts
    USING (org_id = current_setting('app.current_org_id', true)::uuid);

ALTER TABLE gl_journal_entries ENABLE ROW LEVEL SECURITY;
ALTER TABLE gl_journal_entries FORCE ROW LEVEL SECURITY;
CREATE POLICY gl_journal_entries_tenant_isolation ON gl_journal_entries
    USING (org_id = current_setting('app.current_org_id', true)::uuid);
