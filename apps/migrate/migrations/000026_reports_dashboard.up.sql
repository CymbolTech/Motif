CREATE TABLE IF NOT EXISTS report_dashboards (
    tenant_id text NOT NULL,
    environment_id text NOT NULL,
    widgets jsonb NOT NULL DEFAULT '[]'::jsonb,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, environment_id),
    FOREIGN KEY (tenant_id, environment_id) REFERENCES environments(tenant_id, id) ON DELETE CASCADE
);
