CREATE TABLE IF NOT EXISTS point_accounts (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id text NOT NULL,
    environment_id text NOT NULL,
    currency_id text NOT NULL REFERENCES currencies(id) ON DELETE CASCADE,
    external_user_id text NOT NULL,
    available_balance bigint NOT NULL DEFAULT 0 CHECK (available_balance >= 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (tenant_id, environment_id) REFERENCES environments(tenant_id, id) ON DELETE CASCADE,
    UNIQUE (tenant_id, environment_id, currency_id, external_user_id)
);

CREATE TABLE IF NOT EXISTS point_ledger_entries (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    account_id bigint NOT NULL REFERENCES point_accounts(id) ON DELETE RESTRICT,
    entry_type text NOT NULL CHECK (entry_type IN ('award', 'spend', 'expiration', 'conversion_credit', 'conversion_debit', 'reservation', 'release')),
    amount bigint NOT NULL CHECK (amount <> 0),
    balance_after bigint NOT NULL CHECK (balance_after >= 0),
    idempotency_key text NOT NULL,
    request_hash bytea NOT NULL,
    provenance jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (account_id, idempotency_key)
);

CREATE INDEX IF NOT EXISTS point_accounts_user_idx ON point_accounts (tenant_id, environment_id, external_user_id);
CREATE INDEX IF NOT EXISTS point_ledger_entries_account_idx ON point_ledger_entries (account_id, created_at DESC, id DESC);
