ALTER TABLE rules
    ADD COLUMN IF NOT EXISTS starts_at timestamptz,
    ADD COLUMN IF NOT EXISTS ends_at timestamptz;

ALTER TABLE rule_versions
    ADD COLUMN IF NOT EXISTS starts_at timestamptz,
    ADD COLUMN IF NOT EXISTS ends_at timestamptz;

ALTER TABLE rules
    DROP CONSTRAINT IF EXISTS rules_schedule_window_check;
ALTER TABLE rules
    ADD CONSTRAINT rules_schedule_window_check CHECK (ends_at IS NULL OR starts_at IS NULL OR ends_at > starts_at);

ALTER TABLE rule_versions
    DROP CONSTRAINT IF EXISTS rule_versions_schedule_window_check;
ALTER TABLE rule_versions
    ADD CONSTRAINT rule_versions_schedule_window_check CHECK (ends_at IS NULL OR starts_at IS NULL OR ends_at > starts_at);

CREATE INDEX IF NOT EXISTS rules_schedule_idx ON rules (tenant_id, environment_id, status, starts_at, ends_at);
CREATE INDEX IF NOT EXISTS rule_versions_schedule_idx ON rule_versions (rule_id, status, starts_at, ends_at);
