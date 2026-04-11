package db

import (
	"context"
	"time"
)

type AuthSession struct {
	Code         string
	Verification string
	Token        *string
	UserID       *string
	Email        *string
	Error        *string
	CreatedAt    time.Time
	CompletedAt  *time.Time
}

func (d *DB) CreateAuthSession(ctx context.Context, code, verification string) error {
	_, err := d.Pool.Exec(ctx,
		"INSERT INTO auth_sessions (code, verification) VALUES ($1, $2)",
		code, verification)
	return err
}

func (d *DB) GetAuthSession(ctx context.Context, code string) (*AuthSession, error) {
	var s AuthSession
	err := d.Pool.QueryRow(ctx,
		`SELECT code, verification, token, user_id, email, error, created_at, completed_at
		 FROM auth_sessions WHERE code = $1 AND created_at > now() - interval '10 minutes'`,
		code).Scan(&s.Code, &s.Verification, &s.Token, &s.UserID, &s.Email, &s.Error, &s.CreatedAt, &s.CompletedAt)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (d *DB) CompleteAuthSession(ctx context.Context, code, token, userID, email string) error {
	tag, err := d.Pool.Exec(ctx,
		`UPDATE auth_sessions SET token = $2, user_id = $3, email = $4, completed_at = now()
		 WHERE code = $1 AND completed_at IS NULL AND created_at > now() - interval '10 minutes'`,
		code, token, userID, email)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrSessionNotFound
	}
	return nil
}

func (d *DB) FailAuthSession(ctx context.Context, code, errMsg string) error {
	_, err := d.Pool.Exec(ctx,
		`UPDATE auth_sessions SET error = $2, completed_at = now()
		 WHERE code = $1 AND completed_at IS NULL`,
		code, errMsg)
	return err
}

func (d *DB) CleanupAuthSessions(ctx context.Context) (int64, error) {
	tag, err := d.Pool.Exec(ctx,
		"DELETE FROM auth_sessions WHERE created_at < now() - interval '10 minutes'")
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
