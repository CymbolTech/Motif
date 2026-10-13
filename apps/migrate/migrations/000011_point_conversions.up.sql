CREATE TABLE IF NOT EXISTS point_conversions (
    id text PRIMARY KEY,
    tenant_id text NOT NULL,
    environment_id text NOT NULL,
    external_user_id text NOT NULL,
    source_currency_id text NOT NULL REFERENCES currencies(id) ON DELETE RESTRICT,
    target_currency_id text NOT NULL REFERENCES currencies(id) ON DELETE RESTRICT,
    source_amount bigint NOT NULL CHECK (source_amount > 0),
    target_amount bigint NOT NULL CHECK (target_amount > 0),
    rate_numerator bigint NOT NULL CHECK (rate_numerator > 0),
    rate_denominator bigint NOT NULL CHECK (rate_denominator > 0),
    idempotency_key text NOT NULL,
    request_hash bytea NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (tenant_id, environment_id) REFERENCES environments(tenant_id, id) ON DELETE CASCADE,
    CHECK (source_currency_id <> target_currency_id),
    UNIQUE (tenant_id, environment_id, external_user_id, idempotency_key)
);

CREATE INDEX IF NOT EXISTS point_conversions_user_idx ON point_conversions (tenant_id, environment_id, external_user_id, created_at DESC);
