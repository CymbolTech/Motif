CREATE TABLE IF NOT EXISTS achievements (
    id text PRIMARY KEY,
    tenant_id text NOT NULL,
    environment_id text NOT NULL,
    name text NOT NULL,
    description text NOT NULL DEFAULT '',
    event_type text NOT NULL,
    required_count integer NOT NULL CHECK (required_count > 0),
    active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (tenant_id, environment_id) REFERENCES environments(tenant_id, id) ON DELETE CASCADE,
    UNIQUE (tenant_id, environment_id, name)
);

CREATE TABLE IF NOT EXISTS user_achievement_progress (
    achievement_id text NOT NULL REFERENCES achievements(id) ON DELETE CASCADE,
    external_user_id text NOT NULL,
    progress integer NOT NULL DEFAULT 0 CHECK (progress >= 0),
    unlocked_at timestamptz,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (achievement_id, external_user_id)
);

CREATE TABLE IF NOT EXISTS achievement_progress_events (
    achievement_id text NOT NULL REFERENCES achievements(id) ON DELETE CASCADE,
    event_id bigint NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    external_user_id text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (achievement_id, event_id)
);

CREATE INDEX IF NOT EXISTS achievements_scope_idx ON achievements (tenant_id, environment_id, active, event_type);
CREATE INDEX IF NOT EXISTS achievement_progress_user_idx ON user_achievement_progress (external_user_id, updated_at DESC);
