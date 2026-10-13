-- Reconcile conversions written before conversion credits/debits maintained lots.
-- Block concurrent point writes while the historical ledger is inspected and repaired.
LOCK TABLE point_conversions, point_accounts, point_ledger_entries, point_lots, point_lot_allocations IN SHARE ROW EXCLUSIVE MODE;

DO $$
DECLARE
    conversion_record record;
    entry_record record;
    lot_record record;
    expiry_days integer;
    allocated bigint;
    remaining bigint;
    portion bigint;
BEGIN
    -- Credits come first: an account can have received one conversion and
    -- subsequently used that currency as the source of another conversion.
    FOR conversion_record IN
        SELECT x.*, a.id AS account_id
        FROM point_conversions x
        JOIN point_accounts a ON a.tenant_id = x.tenant_id
            AND a.environment_id = x.environment_id
            AND a.external_user_id = x.external_user_id
            AND a.currency_id = x.target_currency_id
        ORDER BY x.created_at, x.id
    LOOP
        SELECT e.id, e.amount INTO entry_record
        FROM point_ledger_entries e
        WHERE e.account_id = conversion_record.account_id
            AND e.idempotency_key = 'conversion:' || conversion_record.id || ':credit'
            AND e.entry_type = 'conversion_credit';
        IF NOT FOUND OR entry_record.amount <> conversion_record.target_amount THEN
            RAISE EXCEPTION 'conversion % has no matching credit ledger entry', conversion_record.id;
        END IF;

        IF NOT EXISTS (SELECT 1 FROM point_lots WHERE award_entry_id = entry_record.id) THEN
            SELECT NULLIF((p.policy->>'expiry_days')::integer, 0) INTO expiry_days
            FROM currencies c
            JOIN currency_policy_versions p ON p.currency_id = c.id AND p.version = c.policy_version
            WHERE c.id = conversion_record.target_currency_id;
            INSERT INTO point_lots (account_id, award_entry_id, original_amount, remaining_amount, expires_at)
            VALUES (conversion_record.account_id, entry_record.id,
                    conversion_record.target_amount, conversion_record.target_amount,
                    CASE WHEN expiry_days IS NULL THEN NULL
                         ELSE conversion_record.created_at + make_interval(days => expiry_days) END);
        END IF;
    END LOOP;

    FOR conversion_record IN
        SELECT x.*, a.id AS account_id
        FROM point_conversions x
        JOIN point_accounts a ON a.tenant_id = x.tenant_id
            AND a.environment_id = x.environment_id
            AND a.external_user_id = x.external_user_id
            AND a.currency_id = x.source_currency_id
        ORDER BY x.created_at, x.id
    LOOP
        SELECT e.id, e.amount INTO entry_record
        FROM point_ledger_entries e
        WHERE e.account_id = conversion_record.account_id
            AND e.idempotency_key = 'conversion:' || conversion_record.id || ':debit'
            AND e.entry_type = 'conversion_debit';
        IF NOT FOUND OR entry_record.amount <> -conversion_record.source_amount THEN
            RAISE EXCEPTION 'conversion % has no matching debit ledger entry', conversion_record.id;
        END IF;

        SELECT COALESCE(sum(amount), 0) INTO allocated
        FROM point_lot_allocations WHERE ledger_entry_id = entry_record.id;
        remaining := conversion_record.source_amount - allocated;
        IF remaining < 0 THEN
            RAISE EXCEPTION 'conversion % has excess debit allocations', conversion_record.id;
        END IF;

        FOR lot_record IN
            SELECT id, remaining_amount FROM point_lots
            WHERE account_id = conversion_record.account_id AND remaining_amount > 0
            ORDER BY expires_at NULLS LAST, id FOR UPDATE
        LOOP
            EXIT WHEN remaining = 0;
            portion := LEAST(lot_record.remaining_amount, remaining);
            UPDATE point_lots SET remaining_amount = remaining_amount - portion WHERE id = lot_record.id;
            INSERT INTO point_lot_allocations (lot_id, ledger_entry_id, amount)
            VALUES (lot_record.id, entry_record.id, portion)
            ON CONFLICT (lot_id, ledger_entry_id)
            DO UPDATE SET amount = point_lot_allocations.amount + EXCLUDED.amount;
            remaining := remaining - portion;
        END LOOP;
        IF remaining > 0 THEN
            RAISE EXCEPTION 'conversion % cannot be backed by existing point lots', conversion_record.id;
        END IF;
    END LOOP;
END $$;
