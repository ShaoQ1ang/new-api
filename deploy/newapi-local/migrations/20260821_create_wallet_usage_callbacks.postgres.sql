CREATE TABLE IF NOT EXISTS wallet_usage_callbacks (
    id BIGSERIAL PRIMARY KEY,
    api_request_id VARCHAR(128) NOT NULL,
    user_id BIGINT NOT NULL,
    business_order_no VARCHAR(128),
    usage_at_ms BIGINT NOT NULL,
    reserved_quota BIGINT NOT NULL,
    reserved_amount BIGINT NOT NULL,
    final_quota BIGINT,
    final_amount BIGINT,
    exchange_rate NUMERIC(20, 8) NOT NULL,
    status VARCHAR(32) NOT NULL,
    next_retry_at_ms BIGINT NOT NULL,
    retry_count BIGINT NOT NULL DEFAULT 0,
    last_error TEXT,
    reserved_at_ms BIGINT NOT NULL DEFAULT 0,
    finalized_at_ms BIGINT NOT NULL DEFAULT 0,
    created_at_ms BIGINT NOT NULL,
    updated_at_ms BIGINT NOT NULL,
    CONSTRAINT uk_wallet_usage_callbacks_request UNIQUE (api_request_id)
);

CREATE INDEX IF NOT EXISTS idx_wallet_usage_callbacks_user
    ON wallet_usage_callbacks (user_id);

CREATE INDEX IF NOT EXISTS idx_wallet_callback_due
    ON wallet_usage_callbacks (status, next_retry_at_ms);
