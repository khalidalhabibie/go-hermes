ALTER TABLE users
    ADD CONSTRAINT chk_users_role
    CHECK (role IN ('user', 'admin'));

ALTER TABLE wallets
    ADD CONSTRAINT chk_wallets_balance_non_negative
    CHECK (balance >= 0),
    ADD CONSTRAINT chk_wallets_status
    CHECK (status IN ('ACTIVE'));

ALTER TABLE transactions
    ADD CONSTRAINT chk_transactions_type
    CHECK (type IN ('TOP_UP', 'TRANSFER')),
    ADD CONSTRAINT chk_transactions_status
    CHECK (status IN ('SUCCESS', 'FAILED', 'PENDING')),
    ADD CONSTRAINT chk_transactions_amount_positive
    CHECK (amount > 0),
    ADD CONSTRAINT chk_transactions_wallet_shape
    CHECK (
        (type = 'TOP_UP' AND source_wallet_id IS NULL AND destination_wallet_id IS NOT NULL)
        OR
        (
            type = 'TRANSFER'
            AND source_wallet_id IS NOT NULL
            AND destination_wallet_id IS NOT NULL
            AND source_wallet_id <> destination_wallet_id
        )
    );

ALTER TABLE ledger_entries
    ADD CONSTRAINT chk_ledger_entries_entry_type
    CHECK (entry_type IN ('DEBIT', 'CREDIT')),
    ADD CONSTRAINT chk_ledger_entries_amount_positive
    CHECK (amount > 0);

ALTER TABLE idempotency_records
    ADD CONSTRAINT chk_idempotency_records_status_code_non_negative
    CHECK (status_code >= 0);

ALTER TABLE webhook_deliveries
    ADD CONSTRAINT chk_webhook_deliveries_status
    CHECK (status IN ('PENDING', 'SUCCESS', 'FAILED', 'RETRYING')),
    ADD CONSTRAINT chk_webhook_deliveries_retry_count_non_negative
    CHECK (retry_count >= 0),
    ADD CONSTRAINT chk_webhook_deliveries_max_retry_positive
    CHECK (max_retry > 0),
    ADD CONSTRAINT chk_webhook_deliveries_retry_bounds
    CHECK (retry_count <= max_retry),
    ADD CONSTRAINT chk_webhook_deliveries_target_url_not_blank
    CHECK (char_length(trim(target_url)) > 0);

CREATE INDEX IF NOT EXISTS idx_transactions_initiated_by_user_id ON transactions(initiated_by_user_id);
CREATE INDEX IF NOT EXISTS idx_idempotency_records_lookup ON idempotency_records(user_id, endpoint, idempotency_key);
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_due ON webhook_deliveries(status, next_retry_at, created_at);

CREATE OR REPLACE FUNCTION prevent_ledger_entries_mutation()
RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'ledger entries are immutable';
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_ledger_entries_prevent_update ON ledger_entries;
CREATE TRIGGER trg_ledger_entries_prevent_update
    BEFORE UPDATE ON ledger_entries
    FOR EACH ROW
    EXECUTE FUNCTION prevent_ledger_entries_mutation();

DROP TRIGGER IF EXISTS trg_ledger_entries_prevent_delete ON ledger_entries;
CREATE TRIGGER trg_ledger_entries_prevent_delete
    BEFORE DELETE ON ledger_entries
    FOR EACH ROW
    EXECUTE FUNCTION prevent_ledger_entries_mutation();
