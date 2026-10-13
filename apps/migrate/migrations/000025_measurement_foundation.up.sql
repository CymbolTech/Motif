CREATE TABLE IF NOT EXISTS metric_definitions (
    tenant_id text NOT NULL,
    environment_id text NOT NULL,
    id text NOT NULL,
    name text NOT NULL,
    description text NOT NULL DEFAULT '',
    event_type text NOT NULL,
    aggregation text NOT NULL CHECK (aggregation IN ('count', 'unique_users')),
    active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, environment_id, id),
    FOREIGN KEY (tenant_id, environment_id) REFERENCES environments(tenant_id, id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS metric_definitions_scope_idx ON metric_definitions (tenant_id, environment_id, active, created_at DESC);
