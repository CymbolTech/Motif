ALTER TABLE rule_executions DROP CONSTRAINT IF EXISTS rule_executions_status_check;
ALTER TABLE rule_executions
    ADD COLUMN IF NOT EXISTS attempts integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    ADD COLUMN IF NOT EXISTS last_error text,
    ADD COLUMN IF NOT EXISTS started_at timestamptz,
    ADD COLUMN IF NOT EXISTS completed_at timestamptz,
    ADD COLUMN IF NOT EXISTS action_ordinal integer NOT NULL DEFAULT 0 CHECK (action_ordinal >= 0);

ALTER TABLE rule_executions
    ADD CONSTRAINT rule_executions_status_check CHECK (status IN ('pending', 'recorded', 'failed'));

CREATE INDEX IF NOT EXISTS rule_executions_status_idx ON rule_executions (status, created_at DESC);
