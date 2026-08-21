ALTER TABLE wallet_usage_callbacks
    ADD COLUMN IF NOT EXISTS failure_code VARCHAR(64) NOT NULL DEFAULT '';
