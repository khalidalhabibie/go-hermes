CREATE TABLE IF NOT EXISTS webhook_deliveries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_type VARCHAR(100) NOT NULL,
    transaction_id UUID REFERENCES transactions(id),
    transaction_ref VARCHAR(100),
    target_url VARCHAR(500) NOT NULL,
    payload JSONB NOT NULL,
    secret VARCHAR(255),
    status VARCHAR(20) NOT NULL,
    retry_count INTEGER NOT NULL DEFAULT 0,
    max_retry INTEGER NOT NULL DEFAULT 3,
    last_error TEXT,
    last_http_status INTEGER,
    next_retry_at TIMESTAMPTZ,
    delivered_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_status ON webhook_deliveries(status);
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_event_type ON webhook_deliveries(event_type);
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_transaction_id ON webhook_deliveries(transaction_id);
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_transaction_ref ON webhook_deliveries(transaction_ref);
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_next_retry_at ON webhook_deliveries(next_retry_at);
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_created_at ON webhook_deliveries(created_at DESC);
