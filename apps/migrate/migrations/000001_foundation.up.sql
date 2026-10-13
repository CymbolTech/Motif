CREATE TABLE IF NOT EXISTS schema_migrations (
    version bigint PRIMARY KEY,
    applied_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS service_checks (
    name text PRIMARY KEY,
    observed_at timestamptz NOT NULL DEFAULT now()
);
