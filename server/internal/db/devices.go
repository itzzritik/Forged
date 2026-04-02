package db

import (
	"context"
	"fmt"
	"time"
)

type Device struct {
	ID              string    `json:"id"`
	UserID          string    `json:"user_id"`
	Name            string    `json:"name"`
	Platform        string    `json:"platform"`
	Hostname        string    `json:"hostname,omitempty"`
	DevicePublicKey string    `json:"device_public_key"`
	RegisteredAt    time.Time `json:"registered_at"`
	LastSeenAt      time.Time `json:"last_seen_at"`
	Approved        bool      `json:"approved"`
}

func (d *DB) CreateDevice(ctx context.Context, userID, name, platform, hostname, publicKey string) (Device, error) {
	hasDevices := false
	d.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM devices WHERE user_id = $1)`, userID).Scan(&hasDevices)

	approved := !hasDevices

	var dev Device
	err := d.Pool.QueryRow(ctx,
		`INSERT INTO devices (user_id, name, platform, hostname, device_public_key, approved)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, user_id, name, platform, hostname, device_public_key, registered_at, last_seen_at, approved`,
		userID, name, platform, hostname, publicKey, approved,
	).Scan(&dev.ID, &dev.UserID, &dev.Name, &dev.Platform, &dev.Hostname,
		&dev.DevicePublicKey, &dev.RegisteredAt, &dev.LastSeenAt, &dev.Approved)
	if err != nil {
		return Device{}, fmt.Errorf("creating device: %w", err)
	}
	return dev, nil
}

func (d *DB) ListDevices(ctx context.Context, userID string) ([]Device, error) {
	rows, err := d.Pool.Query(ctx,
		`SELECT id, user_id, name, platform, hostname, device_public_key, registered_at, last_seen_at, approved
		 FROM devices WHERE user_id = $1 ORDER BY registered_at`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var devices []Device
	for rows.Next() {
		var dev Device
		if err := rows.Scan(&dev.ID, &dev.UserID, &dev.Name, &dev.Platform, &dev.Hostname,
			&dev.DevicePublicKey, &dev.RegisteredAt, &dev.LastSeenAt, &dev.Approved); err != nil {
			return nil, err
		}
		devices = append(devices, dev)
	}
	return devices, rows.Err()
}

func (d *DB) ApproveDevice(ctx context.Context, userID, deviceID string) error {
	tag, err := d.Pool.Exec(ctx,
		`UPDATE devices SET approved = true WHERE id = $1 AND user_id = $2`,
		deviceID, userID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("device not found")
	}
	return nil
}

func (d *DB) DeleteDevice(ctx context.Context, userID, deviceID string) error {
	tag, err := d.Pool.Exec(ctx,
		`DELETE FROM devices WHERE id = $1 AND user_id = $2`,
		deviceID, userID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("device not found")
	}
	return nil
}

func (d *DB) TouchDevice(ctx context.Context, deviceID string) {
	d.Pool.Exec(ctx, `UPDATE devices SET last_seen_at = now() WHERE id = $1`, deviceID)
}

func (d *DB) AuditLog(ctx context.Context, userID, deviceID, action, ip string) {
	d.Pool.Exec(ctx,
		`INSERT INTO audit_log (user_id, device_id, action, ip_address) VALUES ($1, $2, $3, $4::inet)`,
		userID, nilIfEmpty(deviceID), action, nilIfEmpty(ip),
	)
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
