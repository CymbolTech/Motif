ALTER TABLE rule_executions DROP CONSTRAINT IF EXISTS rule_executions_rule_version_fk;
ALTER TABLE rule_executions DROP COLUMN IF EXISTS rule_version_id;
DROP TABLE IF EXISTS rule_versions;
