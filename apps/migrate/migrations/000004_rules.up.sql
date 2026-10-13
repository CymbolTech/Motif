CREATE TABLE IF NOT EXISTS rules (
    id text PRIMARY KEY,
    tenant_id text NOT NULL,
    environment_id text NOT NULL,
    name text NOT NULL,
    description text NOT NULL DEFAULT '',
    event_type text NOT NULL,
    conditions jsonb NOT NULL DEFAULT '{}'::jsonb,
    action jsonb NOT NULL,
    status text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published', 'archived')),
    version integer NOT NULL DEFAULT 1 CHECK (version > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    published_at timestamptz,
    FOREIGN KEY (tenant_id, environment_id) REFERENCES environments(tenant_id, id) ON DELETE CASCADE,
    UNIQUE (tenant_id, environment_id, name)
);

CREATE INDEX IF NOT EXISTS rules_scope_idx ON rules (tenant_id, environment_id, status, event_type);
