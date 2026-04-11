ALTER TABLE gl_journal_lines ADD COLUMN IF NOT EXISTS reconciled BOOLEAN NOT NULL DEFAULT false;

CREATE TABLE bank_transactions (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id            UUID NOT NULL REFERENCES organizations(id),
    account_id        UUID NOT NULL REFERENCES gl_accounts(id),
    transaction_date  DATE NOT NULL,
    amount_cents      BIGINT NOT NULL,
    description       TEXT,
    reference         TEXT,
    matched_line_id   UUID REFERENCES gl_journal_lines(id),
    status            TEXT NOT NULL DEFAULT 'unmatched' CHECK (status IN ('unmatched', 'matched', 'excluded')),
    import_batch_id   UUID,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_bank_transactions_unmatched ON bank_transactions (org_id, account_id) WHERE status = 'unmatched';
