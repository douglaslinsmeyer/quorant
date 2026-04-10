-- i18n / l10n columns: locale, timezone, currency, country

-- 1. Organization locale, timezone, currency, country
ALTER TABLE organizations
  ADD COLUMN locale TEXT NOT NULL DEFAULT 'en_US',
  ADD COLUMN timezone TEXT NOT NULL DEFAULT 'UTC',
  ADD COLUMN currency_code TEXT NOT NULL DEFAULT 'USD',
  ADD COLUMN country TEXT NOT NULL DEFAULT 'US';

-- 2. Unit country
ALTER TABLE units
  ADD COLUMN country TEXT NOT NULL DEFAULT 'US';

-- 3. Financial tables: currency_code
ALTER TABLE assessment_schedules
  ADD COLUMN currency_code TEXT NOT NULL DEFAULT 'USD';

ALTER TABLE assessments
  ADD COLUMN currency_code TEXT NOT NULL DEFAULT 'USD';

ALTER TABLE ledger_entries
  ADD COLUMN currency_code TEXT NOT NULL DEFAULT 'USD';

ALTER TABLE payments
  ADD COLUMN currency_code TEXT NOT NULL DEFAULT 'USD';

ALTER TABLE expenses
  ADD COLUMN currency_code TEXT NOT NULL DEFAULT 'USD';

ALTER TABLE funds
  ADD COLUMN currency_code TEXT NOT NULL DEFAULT 'USD';

ALTER TABLE fund_transactions
  ADD COLUMN currency_code TEXT NOT NULL DEFAULT 'USD';

ALTER TABLE fund_transfers
  ADD COLUMN currency_code TEXT NOT NULL DEFAULT 'USD';

-- 4. Message templates: locale + updated unique constraint
ALTER TABLE message_templates
  ADD COLUMN locale TEXT NOT NULL DEFAULT 'en_US';

DROP INDEX IF EXISTS idx_message_templates_key;
CREATE UNIQUE INDEX idx_message_templates_key
  ON message_templates (COALESCE(org_id, '00000000-0000-0000-0000-000000000000'), template_key, channel, locale);
