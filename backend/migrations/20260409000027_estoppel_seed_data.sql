-- Migration: 20260409000027_estoppel_seed_data.sql
-- Description: Seed jurisdiction_rules with state-level estoppel statutory
-- parameters for FL, CA, TX, NV, VA and seed task_types with the
-- estoppel_request workflow type.
--
-- Uses idempotent INSERT ... WHERE NOT EXISTS patterns so the migration is
-- safe to re-run.

BEGIN;

-- ─── Jurisdiction Rules: estoppel statutory parameters ────────────────────────
-- Stored in jurisdiction_rules (platform-managed, no org_id, no RLS) rather
-- than policy_extractions, which requires org_id NOT NULL and source_doc_id NOT NULL.

-- Florida — §720.30851 / §718.116(8)
INSERT INTO jurisdiction_rules (
    jurisdiction,
    rule_category,
    rule_key,
    value_type,
    value,
    statute_reference,
    effective_date
)
SELECT
    'FL',
    'estoppel',
    'estoppel_rules',
    'json',
    '{"standard_turnaround_business_days":10,"standard_fee_cents":29900,"rush_turnaround_business_days":3,"rush_fee_cents":11900,"delinquent_surcharge_cents":17900,"effective_period_days":30,"electronic_delivery_required":true,"statutory_form_required":true,"statutory_form_id":"fl_720_30851","free_amendment_on_error":true,"statutory_questions":19,"statute_ref":"§720.30851/§718.116(8)","jurisdiction":"FL"}'::jsonb,
    '§720.30851/§718.116(8)',
    '2024-01-01'::date
WHERE NOT EXISTS (
    SELECT 1
    FROM jurisdiction_rules
    WHERE jurisdiction = 'FL'
      AND rule_category = 'estoppel'
      AND rule_key = 'estoppel_rules'
      AND effective_date = '2024-01-01'
);

-- California — Civil Code §4525–4530
INSERT INTO jurisdiction_rules (
    jurisdiction,
    rule_category,
    rule_key,
    value_type,
    value,
    statute_reference,
    effective_date
)
SELECT
    'CA',
    'estoppel',
    'estoppel_rules',
    'json',
    '{"standard_turnaround_business_days":10,"standard_fee_cents":0,"fee_cap_type":"reasonable","statutory_form_required":true,"statutory_form_id":"ca_4528","required_attachments":["governing_docs","ccrs","bylaws","rules","budget","reserve_study_summary"],"statute_ref":"Civil Code §4525–4530","jurisdiction":"CA"}'::jsonb,
    'Civil Code §4525–4530',
    '2024-01-01'::date
WHERE NOT EXISTS (
    SELECT 1
    FROM jurisdiction_rules
    WHERE jurisdiction = 'CA'
      AND rule_category = 'estoppel'
      AND rule_key = 'estoppel_rules'
      AND effective_date = '2024-01-01'
);

-- Texas — Property Code §207.003
INSERT INTO jurisdiction_rules (
    jurisdiction,
    rule_category,
    rule_key,
    value_type,
    value,
    statute_reference,
    effective_date
)
SELECT
    'TX',
    'estoppel',
    'estoppel_rules',
    'json',
    '{"standard_turnaround_business_days":10,"standard_fee_cents":37500,"update_fee_cents":7500,"effective_period_days":60,"noncompliance_damages_cents":500000,"statute_ref":"Property Code §207.003","jurisdiction":"TX"}'::jsonb,
    'Property Code §207.003',
    '2024-01-01'::date
WHERE NOT EXISTS (
    SELECT 1
    FROM jurisdiction_rules
    WHERE jurisdiction = 'TX'
      AND rule_category = 'estoppel'
      AND rule_key = 'estoppel_rules'
      AND effective_date = '2024-01-01'
);

-- Nevada — NRS 116.4109
INSERT INTO jurisdiction_rules (
    jurisdiction,
    rule_category,
    rule_key,
    value_type,
    value,
    statute_reference,
    effective_date
)
SELECT
    'NV',
    'estoppel',
    'estoppel_rules',
    'json',
    '{"standard_turnaround_business_days":10,"standard_fee_cents":18500,"fee_cpi_adjusted":true,"cpi_cap_percent":3,"rush_fee_cents":10000,"effective_period_days":90,"electronic_delivery_required":true,"statute_ref":"NRS 116.4109","jurisdiction":"NV"}'::jsonb,
    'NRS 116.4109',
    '2024-01-01'::date
WHERE NOT EXISTS (
    SELECT 1
    FROM jurisdiction_rules
    WHERE jurisdiction = 'NV'
      AND rule_category = 'estoppel'
      AND rule_key = 'estoppel_rules'
      AND effective_date = '2024-01-01'
);

-- Virginia — §55.1-1808
INSERT INTO jurisdiction_rules (
    jurisdiction,
    rule_category,
    rule_key,
    value_type,
    value,
    statute_reference,
    effective_date
)
SELECT
    'VA',
    'estoppel',
    'estoppel_rules',
    'json',
    '{"standard_turnaround_business_days":14,"fee_cpi_adjusted":true,"electronic_delivery_required":true,"buyer_rescission_days":3,"cic_board_registration_required":true,"statute_ref":"§55.1-1808","jurisdiction":"VA"}'::jsonb,
    '§55.1-1808',
    '2024-01-01'::date
WHERE NOT EXISTS (
    SELECT 1
    FROM jurisdiction_rules
    WHERE jurisdiction = 'VA'
      AND rule_category = 'estoppel'
      AND rule_key = 'estoppel_rules'
      AND effective_date = '2024-01-01'
);

-- ─── Task Type: estoppel_request ──────────────────────────────────────────────
-- org_id = NULL means system-defined (available to all tenants).

INSERT INTO task_types (
    org_id,
    key,
    name,
    description,
    default_priority,
    sla_hours,
    workflow_stages,
    checklist_template,
    auto_assign_role,
    source_module,
    is_active
)
SELECT
    NULL,
    'estoppel_request',
    'Estoppel Certificate Request',
    'Workflow for processing estoppel certificate and lender questionnaire requests from homeowners, title companies, and closing agents.',
    'high'::task_priority,
    80,
    '["submitted","data_aggregation","manager_review","approved","generating","delivered"]'::jsonb,
    '[
        {"id":"verify_requestor","label":"Verify requestor identity and authority","required":true},
        {"id":"check_delinquency","label":"Check unit delinquency and assessment status","required":true},
        {"id":"aggregate_data","label":"Aggregate financial, compliance, and property data","required":true},
        {"id":"manager_review","label":"Manager review and approval of data snapshot","required":true},
        {"id":"generate_pdf","label":"Generate and review estoppel certificate PDF","required":true},
        {"id":"deliver_certificate","label":"Deliver certificate to requestor","required":true}
    ]'::jsonb,
    'hoa_manager',
    'estoppel',
    TRUE
WHERE NOT EXISTS (
    SELECT 1
    FROM task_types
    WHERE org_id IS NULL
      AND key = 'estoppel_request'
);

COMMIT;
