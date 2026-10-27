ALTER TABLE audit_log DROP CONSTRAINT IF EXISTS audit_log_actor_type_check;
UPDATE audit_log SET actor_type = 'access_token' WHERE actor_type = 'access_key';
ALTER TABLE audit_log ADD CONSTRAINT audit_log_actor_type_check
    CHECK (actor_type IN ('platform_admin', 'integration', 'worker', 'system', 'super_admin', 'tenant_admin', 'access_token'));

CREATE TABLE IF NOT EXISTS integration_credentials (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id text NOT NULL,
    environment_id text NOT NULL,
    key_hash bytea NOT NULL UNIQUE,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    label text NOT NULL DEFAULT 'default',
    last_used_at timestamptz,
    created_by text,
    FOREIGN KEY (tenant_id, environment_id) REFERENCES environments(tenant_id, id) ON DELETE CASCADE,
    UNIQUE (tenant_id, environment_id)
);

CREATE INDEX IF NOT EXISTS integration_credentials_scope_idx
    ON integration_credentials (tenant_id, environment_id, revoked_at);

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

-- The old schema holds one integration credential per environment: keep the
-- newest key that can write events, preferring keys that still work.
INSERT INTO integration_credentials (tenant_id, environment_id, key_hash, revoked_at, created_at, label, last_used_at)
SELECT DISTINCT ON (k.tenant_id, k.environment_id)
       k.tenant_id, k.environment_id, k.key_hash, k.revoked_at, k.created_at, k.name, k.last_used_at
FROM access_keys k
WHERE NOT 'mcp' = ANY (k.scopes) AND 'write' = ANY (k.scopes)
ORDER BY k.tenant_id, k.environment_id, (k.revoked_at IS NULL) DESC, k.created_at DESC;

INSERT INTO access_tokens (id, tenant_id, created_by, name, token_hash, token_hint, scopes, environment_ids, expires_at, last_used_at, revoked_at, created_at)
SELECT k.id, k.tenant_id, k.created_by, k.name, k.key_hash, k.key_hint,
       ARRAY(SELECT scope FROM unnest(ARRAY['read', 'write', 'publish']) AS scope
             WHERE scope = 'read'
                OR (scope = 'write' AND 'write' = ANY (k.scopes))
                OR (scope = 'publish' AND 'edit' = ANY (k.scopes))),
       ARRAY[k.environment_id], k.expires_at, k.last_used_at, k.revoked_at, k.created_at
FROM access_keys k
WHERE 'mcp' = ANY (k.scopes) AND k.created_by IS NOT NULL;

DROP TABLE IF EXISTS access_keys;
