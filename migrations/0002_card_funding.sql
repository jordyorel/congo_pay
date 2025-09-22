-- +migrate Up
ALTER TABLE transactions
    ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'completed';

CREATE INDEX IF NOT EXISTS idx_transactions_kind_status ON transactions(kind, status);

INSERT INTO accounts (id, code)
SELECT uuid_generate_v4(), 'suspense:card'
WHERE NOT EXISTS (SELECT 1 FROM accounts WHERE code = 'suspense:card');

-- +migrate Down
DROP INDEX IF EXISTS idx_transactions_kind_status;
ALTER TABLE transactions DROP COLUMN IF EXISTS status;
