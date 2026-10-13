CREATE TABLE IF NOT EXISTS currencies (
    id text PRIMARY KEY,
    tenant_id text NOT NULL,
    environment_id text NOT NULL,
    code text NOT NULL,
    name text NOT NULL,
    unit_scale integer NOT NULL DEFAULT 0 CHECK (unit_scale BETWEEN 0 AND 6),
    policy_version integer NOT NULL DEFAULT 1 CHECK (policy_version > 0),
    active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (tenant_id, environment_id) REFERENCES environments(tenant_id, id) ON DELETE CASCADE,
    UNIQUE (tenant_id, environment_id, code)
);

CREATE TABLE IF NOT EXISTS currency_policy_versions (
    currency_id text NOT NULL REFERENCES currencies(id) ON DELETE CASCADE,
    version integer NOT NULL CHECK (version > 0),
    policy jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (currency_id, version)
);

CREATE INDEX IF NOT EXISTS currencies_scope_idx ON currencies (tenant_id, environment_id, active);
