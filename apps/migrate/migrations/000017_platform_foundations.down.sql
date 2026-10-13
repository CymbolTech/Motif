DROP TABLE IF EXISTS audit_log;
DROP TABLE IF EXISTS tenant_plans;
DROP TABLE IF EXISTS plans;
DROP INDEX IF EXISTS integration_credentials_scope_idx;
ALTER TABLE integration_credentials
    DROP COLUMN IF EXISTS created_by,
    DROP COLUMN IF EXISTS last_used_at,
    DROP COLUMN IF EXISTS label;
