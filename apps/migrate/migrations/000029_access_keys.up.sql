-- Access keys replace both the per-environment integration credential and the
-- workspace access token. A workspace can hold any number of keys per
-- environment; each key carries its own scopes and browser origins.
CREATE TABLE IF NOT EXISTS access_keys (
    id text PRIMARY KEY,
    tenant_id text NOT NULL,
    environment_id text NOT NULL,
    name text NOT NULL,
    key_hash bytea NOT NULL UNIQUE,
    key_hint text NOT NULL DEFAULT '',
    scopes text[] NOT NULL CHECK (
        cardinality(scopes) > 0
        AND scopes <@ ARRAY['read', 'write', 'edit', 'delete', 'mcp']::text[]
        -- The MCP server reads the workspace before it can do anything else.
        AND (NOT 'mcp' = ANY (scopes) OR 'read' = ANY (scopes))
    ),
    -- Browser origins allowed to use the key. Empty allows every origin.
    allowed_origins text[] NOT NULL DEFAULT '{}',
    created_by text REFERENCES admin_users(id) ON DELETE SET NULL,
    expires_at timestamptz,
    last_used_at timestamptz,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (tenant_id, environment_id) REFERENCES environments(tenant_id, id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS access_keys_scope_idx ON access_keys (tenant_id, environment_id, created_at DESC);

-- Integration credentials keep working with the scopes they had: reading and
-- writing through the REST API. Only their hashes exist, so they have no hint.
INSERT INTO access_keys (id, tenant_id, environment_id, name, key_hash, scopes, last_used_at, revoked_at, created_at, updated_at)
SELECT 'key_' || substr(md5(c.tenant_id || '/' || c.environment_id || '/' || c.id::text || '/' || clock_timestamp()::text), 1, 16),
       c.tenant_id, c.environment_id,
       CASE WHEN c.label = 'default' THEN 'Server key' ELSE c.label END,
       c.key_hash, ARRAY['read', 'write'], c.last_used_at, c.revoked_at, c.created_at, c.created_at
FROM integration_credentials c
ON CONFLICT (key_hash) DO NOTHING;

-- Access tokens become MCP keys. Write covered creating, changing and deleting,
-- and publish becomes edit. A key belongs to one environment, so a token moves
-- only when it reaches exactly one; any other token is dropped.
INSERT INTO access_keys (id, tenant_id, environment_id, name, key_hash, key_hint, scopes, created_by, expires_at, last_used_at, revoked_at, created_at, updated_at)
SELECT t.id, t.tenant_id, target.environment_id, t.name, t.token_hash, t.token_hint,
       ARRAY(SELECT scope FROM unnest(ARRAY['read', 'write', 'edit', 'delete', 'mcp']) AS scope
             WHERE scope IN ('read', 'mcp')
                OR (scope IN ('write', 'delete') AND 'write' = ANY (t.scopes))
                OR (scope = 'edit' AND ('write' = ANY (t.scopes) OR 'publish' = ANY (t.scopes)))),
       t.created_by, t.expires_at, t.last_used_at, t.revoked_at, t.created_at, t.created_at
FROM access_tokens t
JOIN LATERAL (
    SELECT min(e.id) AS environment_id, count(*) AS reachable
    FROM environments e
    WHERE e.tenant_id = t.tenant_id AND (cardinality(t.environment_ids) = 0 OR e.id = ANY (t.environment_ids))
) target ON target.reachable = 1
ON CONFLICT (key_hash) DO NOTHING;

DROP TABLE IF EXISTS access_tokens;
DROP TABLE IF EXISTS integration_credentials;

ALTER TABLE audit_log DROP CONSTRAINT IF EXISTS audit_log_actor_type_check;
UPDATE audit_log SET actor_type = 'access_key' WHERE actor_type = 'access_token';
ALTER TABLE audit_log ADD CONSTRAINT audit_log_actor_type_check
    CHECK (actor_type IN ('platform_admin', 'integration', 'worker', 'system', 'super_admin', 'tenant_admin', 'access_key'));
