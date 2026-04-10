-- 20260409000031_payment_allocation_seed.sql
-- Seed jurisdiction-level payment allocation priority orders.

INSERT INTO policy_records (scope, jurisdiction, category, key, value, priority_hint, statute_reference, effective_date, is_active)
VALUES
    ('jurisdiction', 'CA', 'payment_allocation', 'priority_order',
     '{"order":["regular_assessment","special_assessment","collection_cost","attorney_fee","late_fee","interest"]}',
     'state', 'CA Civil Code 5655(a)', '2025-01-01', true),

    ('jurisdiction', 'TX', 'payment_allocation', 'priority_order',
     '{"order":["regular_assessment","special_assessment","attorney_fee","fine"]}',
     'state', 'TX Property Code 209.0063(a)', '2025-01-01', true),

    ('jurisdiction', 'FL', 'payment_allocation', 'priority_order',
     '{"order":["interest","late_fee","collection_cost","attorney_fee","regular_assessment","special_assessment"]}',
     'state', 'FL Statute 720.3085(3)(b)', '2025-01-01', true),

    ('jurisdiction', 'NV', 'payment_allocation', 'priority_order',
     '{"order":["regular_assessment","special_assessment","late_fee","interest","collection_cost","attorney_fee"],"super_lien_months":9}',
     'state', 'NRS 116.3116', '2025-01-01', true),

    ('jurisdiction', 'CO', 'payment_allocation', 'priority_order',
     '{"order":["regular_assessment","special_assessment","late_fee","interest","collection_cost","attorney_fee"],"super_lien_months":6}',
     'state', 'CRS 38-33.3-316(2)', '2025-01-01', true),

    ('jurisdiction', 'CT', 'payment_allocation', 'priority_order',
     '{"order":["regular_assessment","special_assessment","late_fee","interest","collection_cost","attorney_fee"],"super_lien_months":6}',
     'state', 'CGS 47-258(m)', '2025-01-01', true),

    ('jurisdiction', 'DEFAULT', 'payment_allocation', 'priority_order',
     '{"order":["regular_assessment","special_assessment","late_fee","interest","collection_cost","attorney_fee","fine"]}',
     'state', NULL, '2025-01-01', true);
