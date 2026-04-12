-- Zero-knowledge: server must never verify passwords
ALTER TABLE users DROP COLUMN IF EXISTS master_password_hash;
ALTER TABLE users DROP COLUMN IF EXISTS vault_unlock_attempts;
ALTER TABLE users DROP COLUMN IF EXISTS vault_locked_until;
