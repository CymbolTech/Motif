ALTER TABLE event_types
    ADD COLUMN IF NOT EXISTS current_version integer NOT NULL DEFAULT 1 CHECK (current_version > 0);

CREATE TABLE IF NOT EXISTS event_type_versions (
    tenant_id text NOT NULL,
    environment_id text NOT NULL,
    name text NOT NULL,
    version integer NOT NULL CHECK (version > 0),
    schema jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, environment_id, name, version),
    FOREIGN KEY (tenant_id, environment_id, name) REFERENCES event_types(tenant_id, environment_id, name) ON DELETE CASCADE
);

INSERT INTO event_type_versions (tenant_id, environment_id, name, version, schema, created_at)
SELECT tenant_id, environment_id, name, 1, schema, created_at
FROM event_types
ON CONFLICT (tenant_id, environment_id, name, version) DO NOTHING;

ALTER TABLE events
    ADD COLUMN IF NOT EXISTS event_schema_version integer NOT NULL DEFAULT 1 CHECK (event_schema_version > 0);

CREATE INDEX IF NOT EXISTS event_type_versions_scope_idx
    ON event_type_versions (tenant_id, environment_id, name, version DESC);
