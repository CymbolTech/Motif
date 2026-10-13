CREATE TABLE IF NOT EXISTS reward_provider_configs (
    tenant_id text NOT NULL,
    environment_id text NOT NULL,
    provider text NOT NULL CHECK (provider IN ('mock', 'http')),
    delivery_url text NOT NULL DEFAULT '',
    status_url text,
    token_ciphertext bytea,
    timeout_seconds integer NOT NULL DEFAULT 8 CHECK (timeout_seconds BETWEEN 1 AND 60),
    active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, environment_id),
    FOREIGN KEY (tenant_id, environment_id) REFERENCES environments(tenant_id, id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS reward_provider_configs_active_idx
    ON reward_provider_configs (active, updated_at DESC);
