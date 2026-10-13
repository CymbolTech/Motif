CREATE TABLE IF NOT EXISTS level_definitions (
    id text PRIMARY KEY,
    tenant_id text NOT NULL,
    environment_id text NOT NULL,
    level integer NOT NULL CHECK (level > 0),
    name text NOT NULL,
    xp_threshold bigint NOT NULL CHECK (xp_threshold >= 0),
    active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (tenant_id, environment_id) REFERENCES environments(tenant_id, id) ON DELETE CASCADE,
    UNIQUE (tenant_id, environment_id, level),
    UNIQUE (tenant_id, environment_id, xp_threshold)
);

CREATE TABLE IF NOT EXISTS user_xp (
    tenant_id text NOT NULL,
    environment_id text NOT NULL,
    external_user_id text NOT NULL,
    total_xp bigint NOT NULL DEFAULT 0 CHECK (total_xp >= 0),
    updated_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (tenant_id, environment_id) REFERENCES environments(tenant_id, id) ON DELETE CASCADE,
    PRIMARY KEY (tenant_id, environment_id, external_user_id)
);

CREATE TABLE IF NOT EXISTS xp_events (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id text NOT NULL,
    environment_id text NOT NULL,
    external_user_id text NOT NULL,
    amount bigint NOT NULL CHECK (amount > 0),
    idempotency_key text NOT NULL,
    request_hash bytea NOT NULL,
    provenance jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (tenant_id, environment_id) REFERENCES environments(tenant_id, id) ON DELETE CASCADE,
    UNIQUE (tenant_id, environment_id, external_user_id, idempotency_key)
);

CREATE INDEX IF NOT EXISTS level_definitions_scope_idx ON level_definitions (tenant_id, environment_id, active, xp_threshold);
CREATE INDEX IF NOT EXISTS xp_events_user_idx ON xp_events (tenant_id, environment_id, external_user_id, created_at DESC);
