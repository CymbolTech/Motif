UPDATE audit_log SET actor_type = 'platform_admin' WHERE actor_type IN ('super_admin', 'tenant_admin', 'access_token');
ALTER TABLE audit_log DROP CONSTRAINT IF EXISTS audit_log_actor_type_check;
ALTER TABLE audit_log ADD CONSTRAINT audit_log_actor_type_check
    CHECK (actor_type IN ('platform_admin', 'integration', 'worker', 'system'));

DROP TABLE IF EXISTS access_tokens;
