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

func (d *DB) UpdateRekey(ctx context.Context, userID string, kdfParams json.RawMessage, protectedKey string) error {
	_, err := d.Pool.Exec(ctx,
		`UPDATE vaults SET kdf_params = $1, protected_symmetric_key = $2, updated_at = now() WHERE user_id = $3`,
		kdfParams, protectedKey, userID)
	if err != nil {
		return fmt.Errorf("updating vault rekey: %w", err)
	}
	return nil
}

func (d *DB) CleanupAuditLog(ctx context.Context) (int64, error) {
	tag, err := d.Pool.Exec(ctx,
		"DELETE FROM audit_log WHERE created_at < now() - interval '90 days'")
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
