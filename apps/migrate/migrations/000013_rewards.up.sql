CREATE TABLE IF NOT EXISTS rewards (
    id text PRIMARY KEY,
    tenant_id text NOT NULL,
    environment_id text NOT NULL,
    name text NOT NULL,
    reward_type text NOT NULL CHECK (reward_type IN ('digital_unlock')),
    description text NOT NULL DEFAULT '',
    cost_currency_id text REFERENCES currencies(id) ON DELETE RESTRICT,
    cost_amount bigint CHECK (cost_amount IS NULL OR cost_amount > 0),
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (tenant_id, environment_id) REFERENCES environments(tenant_id, id) ON DELETE CASCADE,
    UNIQUE (tenant_id, environment_id, name),
    CHECK ((cost_currency_id IS NULL AND cost_amount IS NULL) OR (cost_currency_id IS NOT NULL AND cost_amount IS NOT NULL))
);

CREATE TABLE IF NOT EXISTS redemptions (
    id text PRIMARY KEY,
    reward_id text NOT NULL REFERENCES rewards(id) ON DELETE RESTRICT,
    tenant_id text NOT NULL,
    environment_id text NOT NULL,
    external_user_id text NOT NULL,
    status text NOT NULL CHECK (status IN ('requested', 'reserved', 'delivering', 'fulfilled', 'failed', 'cancelled')),
    idempotency_key text NOT NULL,
    request_hash bytea NOT NULL,
    provider_reference text,
    failure_reason text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (tenant_id, environment_id) REFERENCES environments(tenant_id, id) ON DELETE CASCADE,
    UNIQUE (tenant_id, environment_id, external_user_id, idempotency_key)
);

CREATE TABLE IF NOT EXISTS reward_delivery_attempts (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    redemption_id text NOT NULL REFERENCES redemptions(id) ON DELETE CASCADE,
    status text NOT NULL CHECK (status IN ('started', 'fulfilled', 'failed')),
    provider_reference text,
    error_message text,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS rewards_scope_idx ON rewards (tenant_id, environment_id, active);
CREATE INDEX IF NOT EXISTS redemptions_user_idx ON redemptions (tenant_id, environment_id, external_user_id, created_at DESC);
