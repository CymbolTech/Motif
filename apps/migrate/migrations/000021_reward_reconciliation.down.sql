DROP INDEX IF EXISTS reward_delivery_attempts_redemption_created_idx;
DROP INDEX IF EXISTS redemptions_reconciliation_idx;

ALTER TABLE reward_delivery_attempts DROP CONSTRAINT IF EXISTS reward_delivery_attempts_kind_check;
ALTER TABLE reward_delivery_attempts DROP CONSTRAINT IF EXISTS reward_delivery_attempts_status_check;

UPDATE reward_delivery_attempts SET status = 'started' WHERE status = 'pending';

ALTER TABLE reward_delivery_attempts
    ADD CONSTRAINT reward_delivery_attempts_status_check
    CHECK (status IN ('started', 'fulfilled', 'failed'));

ALTER TABLE reward_delivery_attempts
    DROP COLUMN IF EXISTS completed_at,
    DROP COLUMN IF EXISTS provider_state,
    DROP COLUMN IF EXISTS kind;

ALTER TABLE redemptions
    DROP COLUMN IF EXISTS last_provider_check_error,
    DROP COLUMN IF EXISTS last_provider_check_at,
    DROP COLUMN IF EXISTS reconciliation_locked_until,
    DROP COLUMN IF EXISTS reconciliation_attempts,
    DROP COLUMN IF EXISTS next_reconciliation_at;
