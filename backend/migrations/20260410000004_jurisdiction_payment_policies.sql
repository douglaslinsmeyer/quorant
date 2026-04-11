-- 20260410000004_jurisdiction_payment_policies.sql
-- Seed jurisdiction-level payment application rules with detailed allocation
-- ordering and jurisdiction-specific metadata (superlien, override flags, etc.).

INSERT INTO policy_records (scope, jurisdiction, category, key, value, priority_hint, statute_reference, effective_date, is_active)
VALUES
    ('jurisdiction', 'CA', 'payment_allocation_rules', 'application_order',
     '{"priority_order": ["regular_assessment", "special_assessment", "late_fee", "interest", "collection_cost", "attorney_fee"], "allow_owner_override": true}',
     'state', 'CA Civil Code 5655', '2025-01-01', true),

    ('jurisdiction', 'FL', 'payment_allocation_rules', 'application_order',
     '{"priority_order": [], "default_method": "oldest_first", "defer_to_declaration": true}',
     'state', 'FL FS 718.116 / 720.3085', '2025-01-01', true),

    ('jurisdiction', 'TX', 'payment_allocation_rules', 'application_order',
     '{"priority_order": ["regular_assessment", "special_assessment", "late_fee", "interest", "attorney_fee", "collection_cost"]}',
     'state', 'TX Property Code 209.0064', '2025-01-01', true),

    ('jurisdiction', 'CO', 'payment_allocation_rules', 'application_order',
     '{"priority_order": ["regular_assessment", "special_assessment", "late_fee", "interest", "collection_cost"]}',
     'state', 'CO CCIOA 38-33.3-316.3', '2025-01-01', true),

    ('jurisdiction', 'NV', 'payment_allocation_rules', 'application_order',
     '{"priority_order": ["regular_assessment", "special_assessment", "late_fee", "interest"], "superlien_months": 9}',
     'state', 'NV NRS 116.3115', '2025-01-01', true),

    ('jurisdiction', 'IL', 'payment_allocation_rules', 'application_order',
     '{"priority_order": ["regular_assessment", "special_assessment", "late_fee", "interest", "collection_cost"]}',
     'state', 'IL 765 ILCS 160/1-45', '2025-01-01', true),

    ('jurisdiction', 'VA', 'payment_allocation_rules', 'application_order',
     '{"priority_order": ["regular_assessment", "special_assessment", "late_fee", "interest", "attorney_fee", "collection_cost"]}',
     'state', 'VA 55.1-1964', '2025-01-01', true),

    ('jurisdiction', 'MD', 'payment_allocation_rules', 'application_order',
     '{"priority_order": ["regular_assessment", "special_assessment", "late_fee", "interest"], "collection_cost_restricted": true}',
     'state', 'MD Real Property 11B-117', '2025-01-01', true),

    ('jurisdiction', 'WA', 'payment_allocation_rules', 'application_order',
     '{"priority_order": ["regular_assessment", "special_assessment", "late_fee", "interest", "collection_cost"]}',
     'state', 'WA RCW 64.90.485', '2025-01-01', true);
