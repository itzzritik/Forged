ALTER TABLE vaults ADD COLUMN IF NOT EXISTS protected_symmetric_key TEXT;

ALTER TABLE users ADD COLUMN IF NOT EXISTS master_password_hash TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS vault_unlock_attempts INT DEFAULT 0;
ALTER TABLE users ADD COLUMN IF NOT EXISTS vault_locked_until TIMESTAMPTZ;
