CREATE TABLE IF NOT EXISTS admin_users (
    id text PRIMARY KEY,
    email text NOT NULL UNIQUE,
    password_hash text NOT NULL,
    role text NOT NULL CHECK (role IN ('super_admin', 'tenant_admin')),
    active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS admin_memberships (
    admin_user_id text NOT NULL REFERENCES admin_users(id) ON DELETE CASCADE,
    tenant_id text NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (admin_user_id, tenant_id)
);

CREATE TABLE IF NOT EXISTS admin_sessions (
    id text PRIMARY KEY,
    admin_user_id text NOT NULL REFERENCES admin_users(id) ON DELETE CASCADE,
    token_hash bytea NOT NULL UNIQUE,
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    last_used_at timestamptz,
    user_agent text NOT NULL DEFAULT '',
    ip_address text NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS admin_sessions_active_idx
    ON admin_sessions (token_hash, expires_at) WHERE revoked_at IS NULL;
CREATE INDEX IF NOT EXISTS admin_memberships_tenant_idx
    ON admin_memberships (tenant_id, admin_user_id);
