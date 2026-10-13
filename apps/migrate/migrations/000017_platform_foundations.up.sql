ALTER TABLE integration_credentials
    ADD COLUMN IF NOT EXISTS label text NOT NULL DEFAULT 'default',
    ADD COLUMN IF NOT EXISTS last_used_at timestamptz,
    ADD COLUMN IF NOT EXISTS created_by text;

CREATE INDEX IF NOT EXISTS integration_credentials_scope_idx
    ON integration_credentials (tenant_id, environment_id, revoked_at);

CREATE TABLE IF NOT EXISTS plans (
    id text PRIMARY KEY,
    name text NOT NULL,
    entitlements jsonb NOT NULL DEFAULT '{}'::jsonb,
    active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS tenant_plans (
    tenant_id text PRIMARY KEY REFERENCES tenants(id) ON DELETE CASCADE,
    plan_id text NOT NULL REFERENCES plans(id),
    overrides jsonb NOT NULL DEFAULT '{}'::jsonb,
    assigned_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS audit_log (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id text,
    environment_id text,
    actor_type text NOT NULL CHECK (actor_type IN ('platform_admin', 'integration', 'worker', 'system')),
    actor_id text,
    action text NOT NULL,
    resource_type text NOT NULL,
    resource_id text,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (tenant_id, environment_id) REFERENCES environments(tenant_id, id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS audit_log_scope_created_idx
    ON audit_log (tenant_id, environment_id, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS audit_log_action_idx
    ON audit_log (action, created_at DESC);

INSERT INTO plans (id, name, entitlements)
VALUES ('starter', 'Starter', '{"max_event_payload_bytes":262144,"max_rules":100,"max_currencies":20,"max_rewards":100}'::jsonb)
ON CONFLICT (id) DO NOTHING;

INSERT INTO tenant_plans (tenant_id, plan_id)
SELECT id, 'starter' FROM tenants
ON CONFLICT (tenant_id) DO NOTHING;
