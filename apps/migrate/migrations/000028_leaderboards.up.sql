-- A leaderboard is a stable, named ranking (for example "weekly") that runs
-- as a series of rounds. Manual boards get rounds from an operator; recurring
-- boards get rounds computed from their schedule the first time a round is
-- needed, so no background job has to fire at period boundaries.
CREATE TABLE IF NOT EXISTS leaderboards (
    id text PRIMARY KEY,
    tenant_id text NOT NULL,
    environment_id text NOT NULL,
    key text NOT NULL,
    name text NOT NULL,
    description text NOT NULL DEFAULT '',
    metric_type text NOT NULL CHECK (metric_type IN ('points', 'xp')),
    currency_id text REFERENCES currencies(id),
    schedule_type text NOT NULL CHECK (schedule_type IN ('manual', 'recurring')),
    interval_unit text CHECK (interval_unit IN ('day', 'week', 'month')),
    interval_count integer CHECK (interval_count BETWEEN 1 AND 366),
    anchor_at timestamptz,
    timezone text NOT NULL DEFAULT 'UTC',
    active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (tenant_id, environment_id) REFERENCES environments(tenant_id, id) ON DELETE CASCADE,
    UNIQUE (tenant_id, environment_id, key),
    CHECK ((metric_type = 'points') = (currency_id IS NOT NULL)),
    CHECK ((schedule_type = 'recurring') = (interval_unit IS NOT NULL AND interval_count IS NOT NULL AND anchor_at IS NOT NULL))
);

CREATE INDEX IF NOT EXISTS leaderboards_metric_idx ON leaderboards (tenant_id, environment_id, metric_type, active);

-- Rounds of one board never overlap. The application enforces that while
-- holding the board row lock, because an exclusion constraint would need the
-- btree_gist extension.
CREATE TABLE IF NOT EXISTS leaderboard_rounds (
    id text PRIMARY KEY,
    leaderboard_id text NOT NULL REFERENCES leaderboards(id) ON DELETE CASCADE,
    number integer NOT NULL CHECK (number > 0),
    name text NOT NULL,
    starts_at timestamptz NOT NULL,
    ends_at timestamptz,
    created_by text NOT NULL CHECK (created_by IN ('admin', 'schedule')),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (leaderboard_id, number),
    UNIQUE (leaderboard_id, starts_at),
    CHECK (ends_at IS NULL OR ends_at > starts_at)
);

CREATE INDEX IF NOT EXISTS leaderboard_rounds_board_idx ON leaderboard_rounds (leaderboard_id, starts_at DESC);

-- reached_at records when a score last increased. Earlier wins a tie.
CREATE TABLE IF NOT EXISTS leaderboard_scores (
    round_id text NOT NULL REFERENCES leaderboard_rounds(id) ON DELETE CASCADE,
    external_user_id text NOT NULL,
    score bigint NOT NULL CHECK (score >= 0),
    reached_at timestamptz NOT NULL,
    PRIMARY KEY (round_id, external_user_id)
);

CREATE INDEX IF NOT EXISTS leaderboard_scores_rank_idx ON leaderboard_scores (round_id, score DESC, reached_at, external_user_id);

UPDATE plans SET entitlements = entitlements || '{"max_leaderboards":10}'::jsonb
WHERE id = 'starter' AND NOT entitlements ? 'max_leaderboards';
