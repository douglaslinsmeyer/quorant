ALTER TABLE funds ADD COLUMN IF NOT EXISTS custodian_type TEXT DEFAULT 'association_held'
    CHECK (custodian_type IN ('association_held', 'management_company_held'));
