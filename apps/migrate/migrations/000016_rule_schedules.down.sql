DROP INDEX IF EXISTS rule_versions_schedule_idx;
DROP INDEX IF EXISTS rules_schedule_idx;
ALTER TABLE rule_versions DROP CONSTRAINT IF EXISTS rule_versions_schedule_window_check;
ALTER TABLE rules DROP CONSTRAINT IF EXISTS rules_schedule_window_check;
ALTER TABLE rule_versions DROP COLUMN IF EXISTS starts_at, DROP COLUMN IF EXISTS ends_at;
ALTER TABLE rules DROP COLUMN IF EXISTS starts_at, DROP COLUMN IF EXISTS ends_at;
