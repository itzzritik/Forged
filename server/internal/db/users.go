package db

import (
	"context"
	"fmt"
	"time"
)

type User struct {
	ID            string    `json:"id"`
	Email         string    `json:"email"`
	AuthHash      string    `json:"-"`
	KeyGeneration int       `json:"key_generation"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (d *DB) CreateUser(ctx context.Context, email, authHash string) (User, error) {
	var u User
	err := d.Pool.QueryRow(ctx,
		`INSERT INTO users (email, auth_hash) VALUES ($1, $2)
		 RETURNING id, email, auth_hash, key_generation, created_at, updated_at`,
		email, authHash,
	).Scan(&u.ID, &u.Email, &u.AuthHash, &u.KeyGeneration, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return User{}, fmt.Errorf("creating user: %w", err)
	}
	return u, nil
}

func (d *DB) GetUserByEmail(ctx context.Context, email string) (User, error) {
	var u User
	err := d.Pool.QueryRow(ctx,
		`SELECT id, email, auth_hash, key_generation, created_at, updated_at
		 FROM users WHERE email = $1`,
		email,
	).Scan(&u.ID, &u.Email, &u.AuthHash, &u.KeyGeneration, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return User{}, fmt.Errorf("user not found: %w", err)
	}
	return u, nil
}

func (d *DB) GetUserByID(ctx context.Context, id string) (User, error) {
	var u User
	err := d.Pool.QueryRow(ctx,
		`SELECT id, email, auth_hash, key_generation, created_at, updated_at
		 FROM users WHERE id = $1`,
		id,
	).Scan(&u.ID, &u.Email, &u.AuthHash, &u.KeyGeneration, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return User{}, fmt.Errorf("user not found: %w", err)
	}
	return u, nil
}

func (d *DB) DeleteUser(ctx context.Context, id string) error {
	_, err := d.Pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, id)
	return err
}
