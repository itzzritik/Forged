package db

import (
	"context"
	"fmt"
	"time"
)

type Vault struct {
	ID              string    `json:"id"`
	UserID          string    `json:"user_id"`
	EncryptedBlob   []byte    `json:"-"`
	Version         int64     `json:"version"`
	UpdatedAt       time.Time `json:"updated_at"`
	UpdatedByDevice *string   `json:"updated_by_device,omitempty"`
}

func (d *DB) GetVault(ctx context.Context, userID string) (Vault, error) {
	var v Vault
	err := d.Pool.QueryRow(ctx,
		`SELECT id, user_id, encrypted_blob, version, updated_at, updated_by_device
		 FROM vaults WHERE user_id = $1`,
		userID,
	).Scan(&v.ID, &v.UserID, &v.EncryptedBlob, &v.Version, &v.UpdatedAt, &v.UpdatedByDevice)
	if err != nil {
		return Vault{}, fmt.Errorf("vault not found: %w", err)
	}
	return v, nil
}

func (d *DB) PushVault(ctx context.Context, userID string, blob []byte, expectedVersion int64, deviceID string) (int64, error) {
	var newVersion int64
	var devID *string
	if deviceID != "" {
		devID = &deviceID
	}

	if expectedVersion == 0 {
		err := d.Pool.QueryRow(ctx,
			`INSERT INTO vaults (user_id, encrypted_blob, version, updated_by_device)
			 VALUES ($1, $2, 1, $3)
			 ON CONFLICT (user_id) DO UPDATE SET
			   encrypted_blob = EXCLUDED.encrypted_blob,
			   version = vaults.version + 1,
			   updated_at = now(),
			   updated_by_device = EXCLUDED.updated_by_device
			 WHERE vaults.version = 0
			 RETURNING version`,
			userID, blob, devID,
		).Scan(&newVersion)
		if err != nil {
			return 0, fmt.Errorf("creating vault: %w", err)
		}
		return newVersion, nil
	}

	err := d.Pool.QueryRow(ctx,
		`UPDATE vaults SET
		   encrypted_blob = $1,
		   version = version + 1,
		   updated_at = now(),
		   updated_by_device = $2
		 WHERE user_id = $3 AND version = $4
		 RETURNING version`,
		blob, devID, userID, expectedVersion,
	).Scan(&newVersion)
	if err != nil {
		return 0, fmt.Errorf("version conflict: vault has been updated by another device")
	}
	return newVersion, nil
}
