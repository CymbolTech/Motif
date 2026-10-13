CREATE TABLE IF NOT EXISTS point_lots (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    account_id bigint NOT NULL REFERENCES point_accounts(id) ON DELETE RESTRICT,
    award_entry_id bigint NOT NULL UNIQUE REFERENCES point_ledger_entries(id) ON DELETE RESTRICT,
    original_amount bigint NOT NULL CHECK (original_amount > 0),
    remaining_amount bigint NOT NULL CHECK (remaining_amount >= 0 AND remaining_amount <= original_amount),
    expires_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS point_lot_allocations (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    lot_id bigint NOT NULL REFERENCES point_lots(id) ON DELETE RESTRICT,
    ledger_entry_id bigint NOT NULL REFERENCES point_ledger_entries(id) ON DELETE RESTRICT,
    amount bigint NOT NULL CHECK (amount > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (lot_id, ledger_entry_id)
);

CREATE INDEX IF NOT EXISTS point_lots_expiry_idx ON point_lots (expires_at, id) WHERE remaining_amount > 0;
CREATE INDEX IF NOT EXISTS point_lots_account_idx ON point_lots (account_id, expires_at, id);
