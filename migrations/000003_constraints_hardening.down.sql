DROP TRIGGER IF EXISTS trg_ledger_entries_prevent_delete ON ledger_entries;
DROP TRIGGER IF EXISTS trg_ledger_entries_prevent_update ON ledger_entries;
DROP FUNCTION IF EXISTS prevent_ledger_entries_mutation();

DROP INDEX IF EXISTS idx_webhook_deliveries_due;
DROP INDEX IF EXISTS idx_idempotency_records_lookup;
DROP INDEX IF EXISTS idx_transactions_initiated_by_user_id;

ALTER TABLE webhook_deliveries
    DROP CONSTRAINT IF EXISTS chk_webhook_deliveries_target_url_not_blank,
    DROP CONSTRAINT IF EXISTS chk_webhook_deliveries_retry_bounds,
    DROP CONSTRAINT IF EXISTS chk_webhook_deliveries_max_retry_positive,
    DROP CONSTRAINT IF EXISTS chk_webhook_deliveries_retry_count_non_negative,
    DROP CONSTRAINT IF EXISTS chk_webhook_deliveries_status;

ALTER TABLE idempotency_records
    DROP CONSTRAINT IF EXISTS chk_idempotency_records_status_code_non_negative;

ALTER TABLE ledger_entries
    DROP CONSTRAINT IF EXISTS chk_ledger_entries_amount_positive,
    DROP CONSTRAINT IF EXISTS chk_ledger_entries_entry_type;

ALTER TABLE transactions
    DROP CONSTRAINT IF EXISTS chk_transactions_wallet_shape,
    DROP CONSTRAINT IF EXISTS chk_transactions_amount_positive,
    DROP CONSTRAINT IF EXISTS chk_transactions_status,
    DROP CONSTRAINT IF EXISTS chk_transactions_type;

ALTER TABLE wallets
    DROP CONSTRAINT IF EXISTS chk_wallets_status,
    DROP CONSTRAINT IF EXISTS chk_wallets_balance_non_negative;

ALTER TABLE users
    DROP CONSTRAINT IF EXISTS chk_users_role;
