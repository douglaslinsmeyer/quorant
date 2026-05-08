-- Migration: 20260409000024_gl_seed_accounts.sql
-- Description: PL/pgSQL function to seed the default HOA chart of accounts
--              for a given organization. Idempotent — skips if accounts
--              already exist for the org.

CREATE OR REPLACE FUNCTION seed_gl_chart_of_accounts(
    p_org_id          UUID,
    p_operating_fund_id UUID,
    p_reserve_fund_id   UUID
) RETURNS VOID
LANGUAGE plpgsql
AS $$
DECLARE
    v_assets_id     UUID;
    v_liabilities_id UUID;
    v_equity_id     UUID;
    v_revenue_id    UUID;
    v_expenses_id   UUID;
BEGIN
    -- Idempotent guard: skip if accounts already exist for this org
    IF EXISTS (
        SELECT 1 FROM gl_accounts WHERE org_id = p_org_id AND deleted_at IS NULL
    ) THEN
        RETURN;
    END IF;

    -- ================================================================
    -- Header accounts (no parent, no fund)
    -- ================================================================

    -- Assets header (1000)
    v_assets_id := gen_random_uuid();
    INSERT INTO gl_accounts (id, org_id, parent_id, fund_id, account_number, name, account_type, is_header, is_system, description)
    VALUES (v_assets_id, p_org_id, NULL, NULL, 1000, 'Assets', 'asset', TRUE, TRUE, NULL);

    -- Liabilities header (2000)
    v_liabilities_id := gen_random_uuid();
    INSERT INTO gl_accounts (id, org_id, parent_id, fund_id, account_number, name, account_type, is_header, is_system, description)
    VALUES (v_liabilities_id, p_org_id, NULL, NULL, 2000, 'Liabilities', 'liability', TRUE, TRUE, NULL);

    -- Equity header (3000)
    v_equity_id := gen_random_uuid();
    INSERT INTO gl_accounts (id, org_id, parent_id, fund_id, account_number, name, account_type, is_header, is_system, description)
    VALUES (v_equity_id, p_org_id, NULL, NULL, 3000, 'Fund Balances', 'equity', TRUE, TRUE, NULL);

    -- Revenue header (4000)
    v_revenue_id := gen_random_uuid();
    INSERT INTO gl_accounts (id, org_id, parent_id, fund_id, account_number, name, account_type, is_header, is_system, description)
    VALUES (v_revenue_id, p_org_id, NULL, NULL, 4000, 'Revenue', 'revenue', TRUE, TRUE, NULL);

    -- Expenses header (5000)
    v_expenses_id := gen_random_uuid();
    INSERT INTO gl_accounts (id, org_id, parent_id, fund_id, account_number, name, account_type, is_header, is_system, description)
    VALUES (v_expenses_id, p_org_id, NULL, NULL, 5000, 'Operating Expenses', 'expense', TRUE, TRUE, NULL);

    -- ================================================================
    -- Asset accounts (1010-1200)
    -- ================================================================
    INSERT INTO gl_accounts (id, org_id, parent_id, fund_id, account_number, name, account_type, is_header, is_system, description)
    VALUES
        (gen_random_uuid(), p_org_id, v_assets_id, p_operating_fund_id, 1010, 'Cash - Operating',                       'asset', FALSE, TRUE,  NULL),
        (gen_random_uuid(), p_org_id, v_assets_id, p_reserve_fund_id,   1020, 'Cash - Reserve',                         'asset', FALSE, TRUE,  NULL),
        (gen_random_uuid(), p_org_id, v_assets_id, p_operating_fund_id, 1100, 'Accounts Receivable - Assessments',      'asset', FALSE, TRUE,  NULL),
        (gen_random_uuid(), p_org_id, v_assets_id, p_operating_fund_id, 1110, 'Accounts Receivable - Other',            'asset', FALSE, FALSE, NULL),
        (gen_random_uuid(), p_org_id, v_assets_id, p_operating_fund_id, 1200, 'Prepaid Expenses',                       'asset', FALSE, FALSE, NULL);

    -- ================================================================
    -- Liability accounts (2100-2200)
    -- ================================================================
    INSERT INTO gl_accounts (id, org_id, parent_id, fund_id, account_number, name, account_type, is_header, is_system, description)
    VALUES
        (gen_random_uuid(), p_org_id, v_liabilities_id, p_operating_fund_id, 2100, 'Accounts Payable',     'liability', FALSE, TRUE,  NULL),
        (gen_random_uuid(), p_org_id, v_liabilities_id, p_operating_fund_id, 2200, 'Prepaid Assessments',  'liability', FALSE, FALSE, NULL);

    -- ================================================================
    -- Equity accounts (3010-3110)
    -- ================================================================
    INSERT INTO gl_accounts (id, org_id, parent_id, fund_id, account_number, name, account_type, is_header, is_system, description)
    VALUES
        (gen_random_uuid(), p_org_id, v_equity_id, p_operating_fund_id, 3010, 'Operating Fund Balance',  'equity', FALSE, TRUE, NULL),
        (gen_random_uuid(), p_org_id, v_equity_id, p_reserve_fund_id,   3020, 'Reserve Fund Balance',    'equity', FALSE, TRUE, NULL),
        (gen_random_uuid(), p_org_id, v_equity_id, NULL,                3100, 'Interfund Transfer Out',  'equity', FALSE, TRUE, NULL),
        (gen_random_uuid(), p_org_id, v_equity_id, NULL,                3110, 'Interfund Transfer In',   'equity', FALSE, TRUE, NULL);

    -- ================================================================
    -- Revenue accounts (4010-4200)
    -- ================================================================
    INSERT INTO gl_accounts (id, org_id, parent_id, fund_id, account_number, name, account_type, is_header, is_system, description)
    VALUES
        (gen_random_uuid(), p_org_id, v_revenue_id, p_operating_fund_id, 4010, 'Assessment Revenue - Operating', 'revenue', FALSE, TRUE,  NULL),
        (gen_random_uuid(), p_org_id, v_revenue_id, p_reserve_fund_id,   4020, 'Assessment Revenue - Reserve',   'revenue', FALSE, TRUE,  NULL),
        (gen_random_uuid(), p_org_id, v_revenue_id, p_operating_fund_id, 4100, 'Late Fee Revenue',               'revenue', FALSE, TRUE,  NULL),
        (gen_random_uuid(), p_org_id, v_revenue_id, NULL,                4200, 'Interest Income',                'revenue', FALSE, FALSE, NULL);

    -- ================================================================
    -- Expense accounts (5010-5060)
    -- ================================================================
    INSERT INTO gl_accounts (id, org_id, parent_id, fund_id, account_number, name, account_type, is_header, is_system, description)
    VALUES
        (gen_random_uuid(), p_org_id, v_expenses_id, p_operating_fund_id, 5010, 'Management Fee',           'expense', FALSE, FALSE, NULL),
        (gen_random_uuid(), p_org_id, v_expenses_id, p_operating_fund_id, 5020, 'Insurance',                'expense', FALSE, FALSE, NULL),
        (gen_random_uuid(), p_org_id, v_expenses_id, p_operating_fund_id, 5030, 'Utilities',                'expense', FALSE, FALSE, NULL),
        (gen_random_uuid(), p_org_id, v_expenses_id, p_operating_fund_id, 5040, 'Landscaping',              'expense', FALSE, FALSE, NULL),
        (gen_random_uuid(), p_org_id, v_expenses_id, p_operating_fund_id, 5050, 'Maintenance and Repairs',  'expense', FALSE, FALSE, NULL),
        (gen_random_uuid(), p_org_id, v_expenses_id, p_operating_fund_id, 5060, 'Professional Services',    'expense', FALSE, FALSE, NULL);
END;
$$;
