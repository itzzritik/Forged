package db

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type User struct {
	ID            string    `json:"id"`
	Email         string    `json:"email"`
	Name          string    `json:"name,omitempty"`
	Provider      string    `json:"provider"`
	ProviderID    string    `json:"provider_id,omitempty"`
	KeyGeneration int       `json:"key_generation"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (d *DB) UpsertOAuthUser(ctx context.Context, email, name, provider, providerID string) (User, error) {
	var u User
	err := d.Pool.QueryRow(ctx,
		`INSERT INTO users (email, name, provider, provider_id)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (email) DO UPDATE SET
		   name = COALESCE(NULLIF($2, ''), users.name),
		   provider = $3,
		   provider_id = $4,
		   updated_at = now()
		 RETURNING id, email, name, provider, provider_id, key_generation, created_at, updated_at`,
		email, name, provider, providerID,
	).Scan(&u.ID, &u.Email, &u.Name, &u.Provider, &u.ProviderID, &u.KeyGeneration, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return User{}, fmt.Errorf("upserting user: %w", err)
	}
	return u, nil
}

func (d *DB) GetUserByID(ctx context.Context, id string) (User, error) {
	var u User
	err := d.Pool.QueryRow(ctx,
		`SELECT id, email, name, provider, provider_id, key_generation, created_at, updated_at
		 FROM users WHERE id = $1`,
		id,
	).Scan(&u.ID, &u.Email, &u.Name, &u.Provider, &u.ProviderID, &u.KeyGeneration, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return User{}, fmt.Errorf("user not found: %w", err)
	}
	return u, nil
}

func (d *DB) DeleteUser(ctx context.Context, id string) error {
	_, err := d.Pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, id)
	return err
}

func (d *DB) GetUserVaultAuth(ctx context.Context, userID string) (masterPasswordHash *string, protectedSymmetricKey *string, attempts int, lockedUntil *time.Time, err error) {
	var hash, protKey *string
	var att int
	var locked *time.Time
	err = d.Pool.QueryRow(ctx,
		`SELECT u.master_password_hash, v.protected_symmetric_key, u.vault_unlock_attempts, u.vault_locked_until
		 FROM users u LEFT JOIN vaults v ON v.user_id = u.id
		 WHERE u.id = $1`, userID).Scan(&hash, &protKey, &att, &locked)
	if err != nil {
		return nil, nil, 0, nil, fmt.Errorf("getting vault auth: %w", err)
	}
	return hash, protKey, att, locked, nil
}

func (d *DB) SetMasterPasswordHash(ctx context.Context, userID, hash string) error {
	_, err := d.Pool.Exec(ctx,
		"UPDATE users SET master_password_hash = $1 WHERE id = $2",
		hash, userID)
	return err
}

func (d *DB) IncrementUnlockAttempts(ctx context.Context, userID string) (int, error) {
	var attempts int
	err := d.Pool.QueryRow(ctx,
		"UPDATE users SET vault_unlock_attempts = vault_unlock_attempts + 1 WHERE id = $1 RETURNING vault_unlock_attempts",
		userID).Scan(&attempts)
	return attempts, err
}

func (d *DB) ResetUnlockAttempts(ctx context.Context, userID string) error {
	_, err := d.Pool.Exec(ctx,
		"UPDATE users SET vault_unlock_attempts = 0, vault_locked_until = NULL WHERE id = $1",
		userID)
	return err
}

func (d *DB) LockVaultUnlock(ctx context.Context, userID string, until time.Time) error {
	_, err := d.Pool.Exec(ctx,
		"UPDATE users SET vault_locked_until = $1, vault_unlock_attempts = 0 WHERE id = $2",
		until, userID)
	return err
}

func (d *DB) UpdateRekey(ctx context.Context, userID string, kdfParams json.RawMessage, protectedKey, masterPasswordHash string) error {
	_, err := d.Pool.Exec(ctx,
		`UPDATE vaults SET kdf_params = $1, protected_symmetric_key = $2 WHERE user_id = $3`,
		kdfParams, protectedKey, userID)
	if err != nil {
		return fmt.Errorf("updating vault rekey: %w", err)
	}
	_, err = d.Pool.Exec(ctx,
		`UPDATE users SET master_password_hash = $1, vault_unlock_attempts = 0, vault_locked_until = NULL WHERE id = $2`,
		masterPasswordHash, userID)
	return err
}

func (d *DB) CleanupAuditLog(ctx context.Context) (int64, error) {
	tag, err := d.Pool.Exec(ctx,
		"DELETE FROM audit_log WHERE created_at < now() - interval '90 days'")
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
