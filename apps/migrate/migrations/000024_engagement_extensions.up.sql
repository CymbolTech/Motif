CREATE TABLE IF NOT EXISTS goals (
    id text PRIMARY KEY,
    tenant_id text NOT NULL,
    environment_id text NOT NULL,
    name text NOT NULL,
    description text NOT NULL DEFAULT '',
    event_type text NOT NULL,
    target_count bigint NOT NULL CHECK (target_count > 0),
    active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (tenant_id, environment_id) REFERENCES environments(tenant_id, id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS campaigns (
    id text PRIMARY KEY,
    tenant_id text NOT NULL,
    environment_id text NOT NULL,
    goal_id text REFERENCES goals(id) ON DELETE SET NULL,
    name text NOT NULL,
    description text NOT NULL DEFAULT '',
    event_type text NOT NULL,
    starts_at timestamptz,
    ends_at timestamptz,
    active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (tenant_id, environment_id) REFERENCES environments(tenant_id, id) ON DELETE CASCADE,
    CHECK (ends_at IS NULL OR starts_at IS NULL OR ends_at > starts_at)
);

CREATE INDEX IF NOT EXISTS goals_scope_idx ON goals (tenant_id, environment_id, active, created_at DESC);
CREATE INDEX IF NOT EXISTS campaigns_scope_idx ON campaigns (tenant_id, environment_id, active, starts_at, ends_at);
