CREATE TABLE IF NOT EXISTS rule_versions (
    id text PRIMARY KEY,
    rule_id text NOT NULL REFERENCES rules(id) ON DELETE CASCADE,
    version integer NOT NULL CHECK (version > 0),
    event_type text NOT NULL,
    conditions jsonb NOT NULL DEFAULT '{}'::jsonb,
    action jsonb NOT NULL,
    status text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published', 'archived')),
    created_at timestamptz NOT NULL DEFAULT now(),
    published_at timestamptz,
    UNIQUE (rule_id, version)
);

INSERT INTO rule_versions (id, rule_id, version, event_type, conditions, action, status, created_at, published_at)
SELECT r.id || ':v' || r.version, r.id, r.version, r.event_type, r.conditions, r.action, r.status, r.created_at, r.published_at
FROM rules r
ON CONFLICT (rule_id, version) DO NOTHING;

ALTER TABLE rule_executions ADD COLUMN IF NOT EXISTS rule_version_id text;

UPDATE rule_executions x
SET rule_version_id = r.id || ':v' || r.version
FROM rules r
WHERE x.rule_id = r.id AND x.rule_version_id IS NULL;

ALTER TABLE rule_executions
    ALTER COLUMN rule_version_id SET NOT NULL,
    ADD CONSTRAINT rule_executions_rule_version_fk FOREIGN KEY (rule_version_id) REFERENCES rule_versions(id) ON DELETE CASCADE;

CREATE UNIQUE INDEX IF NOT EXISTS rule_versions_one_published_idx
    ON rule_versions (rule_id) WHERE status = 'published';
CREATE INDEX IF NOT EXISTS rule_versions_rule_idx ON rule_versions (rule_id, version DESC);
