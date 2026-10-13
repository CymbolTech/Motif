DROP INDEX IF EXISTS rule_executions_status_idx;
ALTER TABLE rule_executions DROP CONSTRAINT IF EXISTS rule_executions_status_check;
ALTER TABLE rule_executions DROP COLUMN IF EXISTS attempts;
ALTER TABLE rule_executions DROP COLUMN IF EXISTS last_error;
ALTER TABLE rule_executions DROP COLUMN IF EXISTS started_at;
ALTER TABLE rule_executions DROP COLUMN IF EXISTS completed_at;
ALTER TABLE rule_executions DROP COLUMN IF EXISTS action_ordinal;
ALTER TABLE rule_executions ADD CONSTRAINT rule_executions_status_check CHECK (status IN ('recorded', 'failed'));
