CREATE TABLE IF NOT EXISTS tenants (
    id text PRIMARY KEY,
    name text NOT NULL,
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'suspended')),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS environments (
    tenant_id text NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    id text NOT NULL,
    name text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, id)
);

CREATE TABLE IF NOT EXISTS integration_credentials (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id text NOT NULL,
    environment_id text NOT NULL,
    key_hash bytea NOT NULL UNIQUE,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (tenant_id, environment_id) REFERENCES environments(tenant_id, id) ON DELETE CASCADE,
    UNIQUE (tenant_id, environment_id)
);

CREATE TABLE IF NOT EXISTS events (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id text NOT NULL,
    environment_id text NOT NULL,
    external_event_id text NOT NULL,
    event_type text NOT NULL,
    occurred_at timestamptz,
    external_user_id text,
    payload jsonb NOT NULL,
    payload_hash bytea NOT NULL,
    status text NOT NULL DEFAULT 'accepted' CHECK (status IN ('accepted', 'processing', 'processed', 'failed')),
    received_at timestamptz NOT NULL DEFAULT now(),
    processed_at timestamptz,
    FOREIGN KEY (tenant_id, environment_id) REFERENCES environments(tenant_id, id) ON DELETE CASCADE,
    UNIQUE (tenant_id, environment_id, external_event_id)
);

CREATE TABLE IF NOT EXISTS event_outbox (
    event_id bigint PRIMARY KEY REFERENCES events(id) ON DELETE CASCADE,
    attempts integer NOT NULL DEFAULT 0,
    published_at timestamptz,
    last_error text,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS events_status_idx ON events (status, received_at);
CREATE INDEX IF NOT EXISTS event_outbox_pending_idx ON event_outbox (published_at, created_at) WHERE published_at IS NULL;
