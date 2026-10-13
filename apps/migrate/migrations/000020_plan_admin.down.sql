ALTER TABLE plans DROP CONSTRAINT IF EXISTS plans_billing_interval_check;
ALTER TABLE plans
    DROP COLUMN IF EXISTS updated_at,
    DROP COLUMN IF EXISTS price_cents,
    DROP COLUMN IF EXISTS billing_interval,
    DROP COLUMN IF EXISTS description;
