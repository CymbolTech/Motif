CREATE TABLE IF NOT EXISTS access_tokens (
    id text PRIMARY KEY,
    tenant_id text NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    created_by text NOT NULL REFERENCES admin_users(id) ON DELETE CASCADE,
    name text NOT NULL,
    token_hash bytea NOT NULL UNIQUE,
    token_hint text NOT NULL,
    scopes text[] NOT NULL CHECK (cardinality(scopes) > 0 AND scopes <@ ARRAY['read', 'write', 'publish']::text[]),
    environment_ids text[] NOT NULL DEFAULT '{}',
    expires_at timestamptz,
    last_used_at timestamptz,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS access_tokens_tenant_idx ON access_tokens (tenant_id, created_at DESC);

-- Administrator sign-in, sign-up and sign-out already record their own role as
-- the actor type, and access tokens record themselves.
ALTER TABLE audit_log DROP CONSTRAINT IF EXISTS audit_log_actor_type_check;
ALTER TABLE audit_log ADD CONSTRAINT audit_log_actor_type_check
    CHECK (actor_type IN ('platform_admin', 'integration', 'worker', 'system', 'super_admin', 'tenant_admin', 'access_token'));
