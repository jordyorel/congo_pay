-- Users table for identity service
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY,
    phone TEXT NOT NULL UNIQUE,
    tier TEXT NOT NULL,
    pin_hash BYTEA NOT NULL,
    device_id TEXT,
    token_version INT NOT NULL DEFAULT 0,
    last_login TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_users_phone ON users (phone);
CREATE INDEX IF NOT EXISTS idx_users_device_id ON users (device_id);
