CREATE TABLE IF NOT EXISTS missions (
    id text PRIMARY KEY,
    tenant_id text NOT NULL,
    environment_id text NOT NULL,
    name text NOT NULL,
    event_type text NOT NULL,
    target_count integer NOT NULL CHECK (target_count > 0),
    active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (tenant_id, environment_id) REFERENCES environments(tenant_id, id) ON DELETE CASCADE,
    UNIQUE (tenant_id, environment_id, name)
);

CREATE TABLE IF NOT EXISTS user_mission_progress (
    mission_id text NOT NULL REFERENCES missions(id) ON DELETE CASCADE,
    external_user_id text NOT NULL,
    progress integer NOT NULL DEFAULT 0 CHECK (progress >= 0),
    completed_at timestamptz,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (mission_id, external_user_id)
);

CREATE TABLE IF NOT EXISTS mission_progress_events (
    mission_id text NOT NULL REFERENCES missions(id) ON DELETE CASCADE,
    event_id bigint NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    external_user_id text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (mission_id, event_id)
);

CREATE INDEX IF NOT EXISTS missions_scope_idx ON missions (tenant_id, environment_id, active, event_type);
CREATE INDEX IF NOT EXISTS mission_progress_user_idx ON user_mission_progress (external_user_id, updated_at DESC);
