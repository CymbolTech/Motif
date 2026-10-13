ALTER TABLE plans
    ADD COLUMN IF NOT EXISTS description text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS billing_interval text NOT NULL DEFAULT 'manual',
    ADD COLUMN IF NOT EXISTS price_cents bigint NOT NULL DEFAULT 0 CHECK (price_cents >= 0),
    ADD COLUMN IF NOT EXISTS updated_at timestamptz NOT NULL DEFAULT now();

ALTER TABLE plans
    DROP CONSTRAINT IF EXISTS plans_billing_interval_check;
ALTER TABLE plans
    ADD CONSTRAINT plans_billing_interval_check CHECK (billing_interval IN ('manual', 'monthly', 'yearly'));
