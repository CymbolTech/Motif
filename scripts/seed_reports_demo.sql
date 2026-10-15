-- Local-only sample data for the Reports workspace.
-- Scope: safary.mr@gmail.com -> QuizHQ / Production.
-- Every identifier is prefixed with demo- so this dataset is easy to remove.

BEGIN;
SET LOCAL TIME ZONE 'UTC';

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM admin_users u
        JOIN admin_memberships m ON m.admin_user_id = u.id
        WHERE lower(u.email) = lower('safary.mr@gmail.com')
          AND m.tenant_id = 'quizhq'
    ) THEN
        RAISE EXCEPTION 'safary.mr@gmail.com is not a member of QuizHQ';
    END IF;
END $$;

-- Remove an earlier run without touching non-demo data.
DELETE FROM rule_executions WHERE rule_id LIKE 'demo-%';
DELETE FROM achievement_progress_events WHERE achievement_id LIKE 'demo-%';
DELETE FROM mission_progress_events WHERE mission_id LIKE 'demo-%';
DELETE FROM user_achievement_progress WHERE achievement_id LIKE 'demo-%';
DELETE FROM user_mission_progress WHERE mission_id LIKE 'demo-%';
DELETE FROM reward_delivery_attempts WHERE redemption_id LIKE 'demo-%';
DELETE FROM redemptions WHERE id LIKE 'demo-%';
DELETE FROM point_lot_allocations WHERE ledger_entry_id IN (SELECT id FROM point_ledger_entries WHERE idempotency_key LIKE 'demo-%');
DELETE FROM point_lots WHERE award_entry_id IN (SELECT id FROM point_ledger_entries WHERE idempotency_key LIKE 'demo-%');
DELETE FROM point_ledger_entries WHERE idempotency_key LIKE 'demo-%';
DELETE FROM point_accounts WHERE external_user_id LIKE 'demo-user-%';
DELETE FROM xp_events WHERE idempotency_key LIKE 'demo-%';
DELETE FROM user_xp WHERE external_user_id LIKE 'demo-user-%';
DELETE FROM events WHERE external_event_id LIKE 'demo-%';
DELETE FROM event_type_versions WHERE tenant_id = 'quizhq' AND environment_id = 'production' AND name LIKE 'demo.%';
DELETE FROM campaigns WHERE id LIKE 'demo-%';
DELETE FROM goals WHERE id LIKE 'demo-%';
DELETE FROM rules WHERE id LIKE 'demo-%';
DELETE FROM missions WHERE id LIKE 'demo-%';
DELETE FROM achievements WHERE id LIKE 'demo-%';
DELETE FROM rewards WHERE id LIKE 'demo-%';
DELETE FROM currencies WHERE id LIKE 'demo-%';
DELETE FROM event_types WHERE tenant_id = 'quizhq' AND environment_id = 'production' AND name LIKE 'demo.%';

-- Event catalog used by the sample activity.
INSERT INTO event_types (tenant_id, environment_id, name, schema, active)
VALUES
    ('quizhq', 'production', 'order.completed', '{"type":"object","properties":{"order_id":{"type":"string"},"total":{"type":"number"}}}'::jsonb, true),
    ('quizhq', 'production', 'product.viewed', '{"type":"object","properties":{"product_id":{"type":"string"}}}'::jsonb, true),
    ('quizhq', 'production', 'session.started', '{"type":"object"}'::jsonb, true),
    ('quizhq', 'production', 'review.submitted', '{"type":"object","properties":{"rating":{"type":"integer"}}}'::jsonb, true),
    ('quizhq', 'production', 'reward.redeemed', '{"type":"object","properties":{"reward_id":{"type":"string"}}}'::jsonb, true)
ON CONFLICT (tenant_id, environment_id, name) DO UPDATE
SET schema = EXCLUDED.schema, active = true, updated_at = now();

INSERT INTO event_type_versions (tenant_id, environment_id, name, version, schema)
SELECT tenant_id, environment_id, name, 1, schema
FROM event_types
WHERE tenant_id = 'quizhq' AND environment_id = 'production' AND name IN ('order.completed', 'product.viewed', 'session.started', 'review.submitted', 'reward.redeemed')
ON CONFLICT (tenant_id, environment_id, name, version) DO UPDATE SET schema = EXCLUDED.schema;

-- Currencies, progression, engagement, rewards, and automations.
INSERT INTO currencies (id, tenant_id, environment_id, code, name, unit_scale, active)
VALUES
    ('demo-coins', 'quizhq', 'production', 'COIN', 'Coins', 0, true),
    ('demo-gems', 'quizhq', 'production', 'GEM', 'Gems', 0, true)
ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, active = true, updated_at = now();

INSERT INTO achievements (id, tenant_id, environment_id, name, description, event_type, required_count, active)
VALUES
    ('demo-first-purchase', 'quizhq', 'production', 'First purchase', 'Complete a first order.', 'order.completed', 1, true),
    ('demo-loyalty-streak', 'quizhq', 'production', 'Loyalty streak', 'Return for seven sessions.', 'session.started', 7, true),
    ('demo-reviewer', 'quizhq', 'production', 'Helpful reviewer', 'Submit three product reviews.', 'review.submitted', 3, true)
ON CONFLICT (id) DO UPDATE SET description = EXCLUDED.description, event_type = EXCLUDED.event_type, required_count = EXCLUDED.required_count, active = true;

INSERT INTO missions (id, tenant_id, environment_id, name, event_type, target_count, active)
VALUES
    ('demo-weekly-buyer', 'quizhq', 'production', 'Weekly buyer', 'order.completed', 3, true),
    ('demo-product-explorer', 'quizhq', 'production', 'Product explorer', 'product.viewed', 8, true)
ON CONFLICT (id) DO UPDATE SET target_count = EXCLUDED.target_count, active = true;

INSERT INTO goals (id, tenant_id, environment_id, name, description, event_type, target_count, active)
VALUES
    ('demo-monthly-purchases', 'quizhq', 'production', 'Monthly purchases', 'Drive completed orders this month.', 'order.completed', 50, true),
    ('demo-review-goal', 'quizhq', 'production', 'Review momentum', 'Collect helpful product feedback.', 'review.submitted', 25, true)
ON CONFLICT (id) DO UPDATE SET description = EXCLUDED.description, target_count = EXCLUDED.target_count, active = true;

INSERT INTO campaigns (id, tenant_id, environment_id, goal_id, name, description, event_type, starts_at, ends_at, active)
VALUES
    ('demo-purchase-campaign', 'quizhq', 'production', 'demo-monthly-purchases', 'October purchase push', 'Sample campaign for completed orders.', 'order.completed', now() - interval '30 days', now() + interval '15 days', true),
    ('demo-review-campaign', 'quizhq', 'production', 'demo-review-goal', 'Review week', 'Sample campaign for review activity.', 'review.submitted', now() - interval '21 days', now() + interval '9 days', true)
ON CONFLICT (id) DO UPDATE SET goal_id = EXCLUDED.goal_id, starts_at = EXCLUDED.starts_at, ends_at = EXCLUDED.ends_at, active = true;

INSERT INTO rewards (id, tenant_id, environment_id, name, reward_type, description, cost_currency_id, cost_amount, payload, active)
VALUES
    ('demo-free-shipping', 'quizhq', 'production', 'Free shipping', 'digital_unlock', 'A free shipping code.', 'demo-coins', 100, '{"code":"DEMO-SHIP-FREE"}'::jsonb, true),
    ('demo-vip-badge', 'quizhq', 'production', 'VIP badge', 'digital_unlock', 'A digital VIP badge.', 'demo-gems', 20, '{"badge":"demo-vip"}'::jsonb, true),
    ('demo-surprise-box', 'quizhq', 'production', 'Surprise box', 'digital_unlock', 'A mystery sample reward.', 'demo-coins', 250, '{"box":"demo-surprise"}'::jsonb, true)
ON CONFLICT (id) DO UPDATE SET description = EXCLUDED.description, cost_currency_id = EXCLUDED.cost_currency_id, cost_amount = EXCLUDED.cost_amount, active = true, updated_at = now();

INSERT INTO rules (id, tenant_id, environment_id, name, description, event_type, conditions, action, status, version, published_at)
VALUES
    ('demo-rule-purchase-coins', 'quizhq', 'production', 'Reward completed purchases', 'Award coins after a completed order.', 'order.completed', '{}'::jsonb, '{"type":"award_points","currency":"COIN","amount":10}'::jsonb, 'published', 1, now() - interval '25 days'),
    ('demo-rule-review-xp', 'quizhq', 'production', 'Celebrate product reviews', 'Award XP after a review.', 'review.submitted', '{}'::jsonb, '{"type":"award_xp","amount":25}'::jsonb, 'published', 1, now() - interval '25 days')
ON CONFLICT (id) DO UPDATE SET action = EXCLUDED.action, status = 'published', version = 1, published_at = EXCLUDED.published_at, updated_at = now();

INSERT INTO rule_versions (id, rule_id, version, event_type, conditions, action, status, created_at, published_at)
SELECT r.id || ':v1', r.id, 1, r.event_type, r.conditions, r.action, 'published', r.created_at, r.published_at
FROM rules r
WHERE r.id LIKE 'demo-%'
ON CONFLICT (rule_id, version) DO UPDATE SET action = EXCLUDED.action, status = 'published', published_at = EXCLUDED.published_at;

-- 45 days of varied activity for 18 sample members.
DO $$
DECLARE
    d integer;
    u integer;
    event_id bigint;
    event_time timestamptz;
    event_name text;
    event_kind text;
    event_status text;
    event_payload jsonb;
BEGIN
    FOR d IN 0..44 LOOP
        FOR u IN 1..18 LOOP
            event_time := now() - (d::text || ' days')::interval - ((u % 6)::text || ' hours')::interval;

            IF (d + u) % 2 = 0 THEN
                event_name := format('demo-order-%s-%s', d, u);
                event_kind := 'order.completed';
                event_payload := jsonb_build_object('order_id', event_name, 'total', 25 + ((d + u) % 9) * 10, 'currency', 'USD');
                event_status := CASE WHEN (d + u) % 17 = 0 THEN 'failed' WHEN (d + u) % 13 = 0 THEN 'accepted' WHEN (d + u) % 29 = 0 THEN 'processing' ELSE 'processed' END;
                INSERT INTO events (tenant_id, environment_id, external_event_id, event_type, occurred_at, external_user_id, payload, payload_hash, status, received_at, processed_at)
                VALUES ('quizhq', 'production', event_name, event_kind, event_time, format('demo-user-%s', u), event_payload, decode(md5(event_payload::text), 'hex'), event_status, event_time + interval '2 minutes', CASE WHEN event_status = 'processed' THEN event_time + interval '3 minutes' ELSE NULL END)
                ON CONFLICT (tenant_id, environment_id, external_event_id) DO NOTHING
                RETURNING id INTO event_id;
            END IF;

            IF (d * 3 + u) % 4 = 0 THEN
                event_name := format('demo-view-%s-%s', d, u);
                event_kind := 'product.viewed';
                event_payload := jsonb_build_object('product_id', format('product-%s', (d + u) % 12 + 1));
                event_status := CASE WHEN (d + u) % 19 = 0 THEN 'failed' ELSE 'processed' END;
                INSERT INTO events (tenant_id, environment_id, external_event_id, event_type, occurred_at, external_user_id, payload, payload_hash, status, received_at, processed_at)
                VALUES ('quizhq', 'production', event_name, event_kind, event_time + interval '20 minutes', format('demo-user-%s', u), event_payload, decode(md5(event_payload::text), 'hex'), event_status, event_time + interval '22 minutes', CASE WHEN event_status = 'processed' THEN event_time + interval '23 minutes' ELSE NULL END)
                ON CONFLICT (tenant_id, environment_id, external_event_id) DO NOTHING
                RETURNING id INTO event_id;
            END IF;

            IF (d + u) % 5 = 0 THEN
                event_name := format('demo-session-%s-%s', d, u);
                event_kind := 'session.started';
                event_payload := jsonb_build_object('source', CASE WHEN u % 2 = 0 THEN 'mobile' ELSE 'web' END);
                INSERT INTO events (tenant_id, environment_id, external_event_id, event_type, occurred_at, external_user_id, payload, payload_hash, status, received_at, processed_at)
                VALUES ('quizhq', 'production', event_name, event_kind, event_time + interval '35 minutes', format('demo-user-%s', u), event_payload, decode(md5(event_payload::text), 'hex'), 'processed', event_time + interval '37 minutes', event_time + interval '38 minutes')
                ON CONFLICT (tenant_id, environment_id, external_event_id) DO NOTHING
                RETURNING id INTO event_id;
            END IF;
        END LOOP;

        IF d % 2 = 0 THEN
            event_name := format('demo-review-%s', d);
            event_payload := jsonb_build_object('rating', 4 + (d % 2), 'product_id', format('product-%s', d % 12 + 1));
            INSERT INTO events (tenant_id, environment_id, external_event_id, event_type, occurred_at, external_user_id, payload, payload_hash, status, received_at, processed_at)
            VALUES ('quizhq', 'production', event_name, 'review.submitted', now() - (d::text || ' days')::interval - interval '1 hour', format('demo-user-%s', d % 9 + 1), event_payload, decode(md5(event_payload::text), 'hex'), 'processed', now() - (d::text || ' days')::interval, now() - (d::text || ' days')::interval + interval '1 minute')
            ON CONFLICT (tenant_id, environment_id, external_event_id) DO NOTHING;
        END IF;
    END LOOP;
END $$;

-- Link activity to progression and expose meaningful completion/unlock series.
INSERT INTO achievement_progress_events (achievement_id, event_id, external_user_id, created_at)
SELECT a.id, e.id, e.external_user_id, COALESCE(e.occurred_at, e.received_at)
FROM achievements a
JOIN events e ON e.tenant_id = a.tenant_id AND e.environment_id = a.environment_id AND e.event_type = a.event_type
WHERE a.id LIKE 'demo-%' AND e.external_event_id LIKE 'demo-%' AND e.status <> 'failed'
ON CONFLICT DO NOTHING;

INSERT INTO user_achievement_progress (achievement_id, external_user_id, progress, unlocked_at, updated_at)
SELECT a.id, p.external_user_id, count(*)::integer, CASE WHEN count(*) >= a.required_count THEN max(p.created_at) ELSE NULL END, max(p.created_at)
FROM achievements a
JOIN achievement_progress_events p ON p.achievement_id = a.id
GROUP BY a.id, a.required_count, p.external_user_id
ON CONFLICT (achievement_id, external_user_id) DO UPDATE SET progress = EXCLUDED.progress, unlocked_at = EXCLUDED.unlocked_at, updated_at = EXCLUDED.updated_at;

INSERT INTO mission_progress_events (mission_id, event_id, external_user_id, created_at)
SELECT m.id, e.id, e.external_user_id, COALESCE(e.occurred_at, e.received_at)
FROM missions m
JOIN events e ON e.tenant_id = m.tenant_id AND e.environment_id = m.environment_id AND e.event_type = m.event_type
WHERE m.id LIKE 'demo-%' AND e.external_event_id LIKE 'demo-%' AND e.status <> 'failed'
ON CONFLICT DO NOTHING;

INSERT INTO user_mission_progress (mission_id, external_user_id, progress, completed_at, updated_at)
SELECT m.id, p.external_user_id, count(*)::integer, CASE WHEN count(*) >= m.target_count THEN max(p.created_at) ELSE NULL END, max(p.created_at)
FROM missions m
JOIN mission_progress_events p ON p.mission_id = m.id
GROUP BY m.id, m.target_count, p.external_user_id
ON CONFLICT (mission_id, external_user_id) DO UPDATE SET progress = EXCLUDED.progress, completed_at = EXCLUDED.completed_at, updated_at = EXCLUDED.updated_at;

-- XP activity across the same sample members.
INSERT INTO xp_events (tenant_id, environment_id, external_user_id, amount, idempotency_key, request_hash, provenance, created_at)
SELECT 'quizhq', 'production', e.external_user_id,
       CASE WHEN e.event_type = 'review.submitted' THEN 25 ELSE 10 END,
       'demo-xp-' || e.id, decode(md5('demo-xp-' || e.id), 'hex'), '{"source":"demo-seed"}'::jsonb, COALESCE(e.occurred_at, e.received_at)
FROM events e
WHERE e.external_event_id LIKE 'demo-%' AND e.event_type IN ('order.completed', 'review.submitted')
ON CONFLICT DO NOTHING;

INSERT INTO user_xp (tenant_id, environment_id, external_user_id, total_xp, updated_at)
SELECT 'quizhq', 'production', external_user_id, sum(amount), max(created_at)
FROM xp_events
WHERE idempotency_key LIKE 'demo-%'
GROUP BY external_user_id
ON CONFLICT (tenant_id, environment_id, external_user_id) DO UPDATE SET total_xp = EXCLUDED.total_xp, updated_at = EXCLUDED.updated_at;

-- Point accounts and ledger history: awards, spends, and expirations.
INSERT INTO point_accounts (tenant_id, environment_id, currency_id, external_user_id, available_balance)
SELECT 'quizhq', 'production', c.id, format('demo-user-%s', u), 500 + (u * 15)
FROM currencies c CROSS JOIN generate_series(1, 18) AS users(u)
WHERE c.id IN ('demo-coins', 'demo-gems')
ON CONFLICT (tenant_id, environment_id, currency_id, external_user_id) DO UPDATE SET available_balance = EXCLUDED.available_balance, updated_at = now();

INSERT INTO point_ledger_entries (account_id, entry_type, amount, balance_after, idempotency_key, request_hash, provenance, created_at)
SELECT pa.id, 'award', 10 + (e.id % 5), 500, 'demo-award-' || e.id, decode(md5('demo-award-' || e.id), 'hex'), '{"source":"demo-seed","reason":"completed_order"}'::jsonb, COALESCE(e.occurred_at, e.received_at)
FROM events e
JOIN point_accounts pa ON pa.tenant_id = 'quizhq' AND pa.environment_id = 'production' AND pa.currency_id = 'demo-coins' AND pa.external_user_id = e.external_user_id
WHERE e.external_event_id LIKE 'demo-%' AND e.event_type = 'order.completed' AND e.status <> 'failed'
ON CONFLICT DO NOTHING;

INSERT INTO point_ledger_entries (account_id, entry_type, amount, balance_after, idempotency_key, request_hash, provenance, created_at)
SELECT pa.id, CASE WHEN e.id % 7 = 0 THEN 'expiration' ELSE 'spend' END, -5 - (e.id % 4), 300, 'demo-spend-' || e.id, decode(md5('demo-spend-' || e.id), 'hex'), '{"source":"demo-seed","reason":"sample_redemption"}'::jsonb, COALESCE(e.occurred_at, e.received_at) + interval '2 hours'
FROM events e
JOIN point_accounts pa ON pa.tenant_id = 'quizhq' AND pa.environment_id = 'production' AND pa.currency_id = 'demo-coins' AND pa.external_user_id = e.external_user_id
WHERE e.external_event_id LIKE 'demo-%' AND e.event_type = 'order.completed' AND e.status <> 'failed' AND e.id % 3 = 0
ON CONFLICT DO NOTHING;

-- Reward redemption history with fulfilled and failed deliveries visible in Reports.
DO $$
DECLARE
    i integer;
    redemption_status text;
    reward_id text;
    redemption_time timestamptz;
BEGIN
    FOR i IN 1..48 LOOP
        redemption_status := CASE WHEN i % 11 = 0 THEN 'failed' WHEN i % 7 = 0 THEN 'delivering' WHEN i % 5 = 0 THEN 'reserved' ELSE 'fulfilled' END;
        reward_id := CASE WHEN i % 3 = 0 THEN 'demo-vip-badge' WHEN i % 2 = 0 THEN 'demo-surprise-box' ELSE 'demo-free-shipping' END;
        redemption_time := now() - ((i * 2 % 44)::text || ' days')::interval - ((i % 8)::text || ' hours')::interval;
        INSERT INTO redemptions (id, reward_id, tenant_id, environment_id, external_user_id, status, idempotency_key, request_hash, provider_reference, failure_reason, created_at, updated_at)
        VALUES (format('demo-redemption-%s', i), reward_id, 'quizhq', 'production', format('demo-user-%s', (i % 18) + 1), redemption_status, format('demo-redemption-key-%s', i), decode(md5(format('demo-redemption-key-%s', i)), 'hex'), CASE WHEN redemption_status IN ('fulfilled', 'delivering') THEN format('demo-provider-%s', i) ELSE NULL END, CASE WHEN redemption_status = 'failed' THEN 'Sample provider failure' ELSE NULL END, redemption_time, redemption_time + interval '1 hour')
        ON CONFLICT (id) DO UPDATE SET status = EXCLUDED.status, updated_at = EXCLUDED.updated_at;

        INSERT INTO reward_delivery_attempts (redemption_id, status, provider_reference, error_message, created_at, kind, provider_state, completed_at)
        VALUES (format('demo-redemption-%s', i), CASE WHEN redemption_status = 'failed' THEN 'failed' WHEN redemption_status = 'fulfilled' THEN 'fulfilled' ELSE 'pending' END, CASE WHEN redemption_status IN ('fulfilled', 'delivering') THEN format('demo-provider-%s', i) ELSE NULL END, CASE WHEN redemption_status = 'failed' THEN 'Sample provider failure' ELSE NULL END, redemption_time + interval '30 minutes', 'delivery', redemption_status, CASE WHEN redemption_status = 'fulfilled' THEN redemption_time + interval '1 hour' ELSE NULL END);
    END LOOP;
END $$;

-- Automation execution history over the same event stream.
INSERT INTO rule_executions (rule_id, event_id, action_type, action, status, created_at, rule_version_id)
SELECT 'demo-rule-purchase-coins', e.id, 'award_points', '{"currency":"COIN","amount":10}'::jsonb, CASE WHEN e.id % 19 = 0 THEN 'failed' ELSE 'recorded' END, COALESCE(e.occurred_at, e.received_at) + interval '4 minutes', 'demo-rule-purchase-coins:v1'
FROM events e
WHERE e.external_event_id LIKE 'demo-%' AND e.event_type = 'order.completed'
ON CONFLICT (rule_id, event_id) DO NOTHING;

INSERT INTO rule_executions (rule_id, event_id, action_type, action, status, created_at, rule_version_id)
SELECT 'demo-rule-review-xp', e.id, 'award_xp', '{"amount":25}'::jsonb, 'recorded', COALESCE(e.occurred_at, e.received_at) + interval '4 minutes', 'demo-rule-review-xp:v1'
FROM events e
WHERE e.external_event_id LIKE 'demo-%' AND e.event_type = 'review.submitted'
ON CONFLICT (rule_id, event_id) DO NOTHING;

COMMIT;
