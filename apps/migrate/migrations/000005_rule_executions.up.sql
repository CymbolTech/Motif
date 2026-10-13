CREATE TABLE IF NOT EXISTS rule_executions (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    rule_id text NOT NULL REFERENCES rules(id) ON DELETE CASCADE,
    event_id bigint NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    action_type text NOT NULL,
    action jsonb NOT NULL,
    status text NOT NULL DEFAULT 'recorded' CHECK (status IN ('recorded', 'failed')),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (rule_id, event_id)
);

CREATE INDEX IF NOT EXISTS rule_executions_created_idx ON rule_executions (created_at DESC);
