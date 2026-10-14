package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

var (
	ErrVaultNotFound   = errors.New("vault not found")
	ErrVersionConflict = errors.New("version conflict: vault has been updated by another device")
)

type Vault struct {
	ID              string          `json:"id"`
	UserID          string          `json:"user_id"`
	EncryptedBlob   []byte          `json:"-"`
	Version         int64           `json:"version"`
	UpdatedAt       time.Time       `json:"updated_at"`
	UpdatedByDevice *string         `json:"updated_by_device,omitempty"`
	KDFParams             json.RawMessage `json:"kdf_params,omitempty"`
	ProtectedSymmetricKey *string         `json:"protected_symmetric_key,omitempty"`
}

func (d *DB) GetVault(ctx context.Context, userID string) (Vault, error) {
	var v Vault
	err := d.Pool.QueryRow(ctx,
		`SELECT id, user_id, encrypted_blob, version, updated_at, updated_by_device, kdf_params, protected_symmetric_key
		 FROM vaults WHERE user_id = $1`,
		userID,
	).Scan(&v.ID, &v.UserID, &v.EncryptedBlob, &v.Version, &v.UpdatedAt, &v.UpdatedByDevice, &v.KDFParams, &v.ProtectedSymmetricKey)
	if errors.Is(err, pgx.ErrNoRows) {
		return Vault{}, ErrVaultNotFound
	}
	if err != nil {
		return Vault{}, fmt.Errorf("reading vault: %w", err)
	}
	return v, nil
}

func (d *DB) PushVault(ctx context.Context, userID string, blob []byte, expectedVersion int64, deviceID string, kdfParams json.RawMessage, protectedSymmetricKey string) (int64, error) {
	var newVersion int64
	var devID *string
	if deviceID != "" {
		devID = &deviceID
	}

	if expectedVersion == 0 {
		err := d.Pool.QueryRow(ctx,
			`INSERT INTO vaults (user_id, encrypted_blob, version, updated_by_device, kdf_params, protected_symmetric_key)
			 VALUES ($1, $2, 1, $3, $4, $5)
			 ON CONFLICT (user_id) DO UPDATE SET
			   encrypted_blob = EXCLUDED.encrypted_blob,
			   version = vaults.version + 1,
			   updated_at = now(),
			   updated_by_device = EXCLUDED.updated_by_device,
			   kdf_params = EXCLUDED.kdf_params,
			   protected_symmetric_key = EXCLUDED.protected_symmetric_key
			 WHERE vaults.version = 0
			 RETURNING version`,
			userID, blob, devID, kdfParams, protectedSymmetricKey,
		).Scan(&newVersion)
		// No row means a vault already exists with a version, so the create lost the race.
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, ErrVersionConflict
		}
		if err != nil {
			return 0, fmt.Errorf("creating vault: %w", err)
		}
		return newVersion, nil
	}

	// Only rekey changes the password wrap; a push from a device still holding the old wrap
	// must not revert a password change. A push only fills a missing wrap.
	err := d.Pool.QueryRow(ctx,
		`UPDATE vaults SET
		   encrypted_blob = $1,
		   version = version + 1,
		   updated_at = now(),
		   updated_by_device = $2,
		   kdf_params = CASE WHEN COALESCE(protected_symmetric_key, '') = '' THEN $5 ELSE kdf_params END,
		   protected_symmetric_key = CASE WHEN COALESCE(protected_symmetric_key, '') = '' THEN $6 ELSE protected_symmetric_key END
		 WHERE user_id = $3 AND version = $4
		 RETURNING version`,
		blob, devID, userID, expectedVersion, kdfParams, protectedSymmetricKey,
	).Scan(&newVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrVersionConflict
	}
	if err != nil {
		return 0, fmt.Errorf("updating vault: %w", err)
	}
	return newVersion, nil
}
