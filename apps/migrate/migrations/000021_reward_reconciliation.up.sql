ALTER TABLE redemptions
    ADD COLUMN IF NOT EXISTS next_reconciliation_at timestamptz,
    ADD COLUMN IF NOT EXISTS reconciliation_attempts integer NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS reconciliation_locked_until timestamptz,
    ADD COLUMN IF NOT EXISTS last_provider_check_at timestamptz,
    ADD COLUMN IF NOT EXISTS last_provider_check_error text;

ALTER TABLE reward_delivery_attempts
    ADD COLUMN IF NOT EXISTS kind text NOT NULL DEFAULT 'delivery',
    ADD COLUMN IF NOT EXISTS provider_state text,
    ADD COLUMN IF NOT EXISTS completed_at timestamptz;

ALTER TABLE reward_delivery_attempts
    DROP CONSTRAINT IF EXISTS reward_delivery_attempts_status_check;

ALTER TABLE reward_delivery_attempts
    ADD CONSTRAINT reward_delivery_attempts_status_check
    CHECK (status IN ('started', 'pending', 'fulfilled', 'failed'));

ALTER TABLE reward_delivery_attempts
    DROP CONSTRAINT IF EXISTS reward_delivery_attempts_kind_check;

ALTER TABLE reward_delivery_attempts
    ADD CONSTRAINT reward_delivery_attempts_kind_check
    CHECK (kind IN ('delivery', 'reconciliation'));

UPDATE redemptions
SET next_reconciliation_at = updated_at
WHERE status = 'delivering' AND next_reconciliation_at IS NULL;

CREATE INDEX IF NOT EXISTS redemptions_reconciliation_idx
    ON redemptions (status, next_reconciliation_at, reconciliation_locked_until, updated_at)
    WHERE status = 'delivering';

CREATE INDEX IF NOT EXISTS reward_delivery_attempts_redemption_created_idx
    ON reward_delivery_attempts (redemption_id, created_at DESC, id DESC);
