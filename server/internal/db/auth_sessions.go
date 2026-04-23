package db

import (
	"context"
	"fmt"
	"time"
)

type AuthSession struct {
	Code            string
	Verification    string
	CodeChallenge   *string
	ChallengeMethod *string
	Token           *string
	UserID          *string
	Email           *string
	Error           *string
	ApprovedUserID  *string
	CreatedAt       time.Time
	ApprovedAt      *time.Time
	CompletedAt     *time.Time
}

func (d *DB) CreateAuthSession(ctx context.Context, code, verification, codeChallenge, challengeMethod string) error {
	_, err := d.Pool.Exec(ctx,
		`INSERT INTO auth_sessions (code, verification, code_challenge, challenge_method)
		 VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, ''))`,
		code, verification, codeChallenge, challengeMethod)
	return err
}

func (d *DB) GetAuthSession(ctx context.Context, code string) (*AuthSession, error) {
	var s AuthSession
	err := d.Pool.QueryRow(ctx,
		`SELECT code, verification, code_challenge, challenge_method, token, user_id, email, error, approved_user_id, created_at, approved_at, completed_at
		 FROM auth_sessions WHERE code = $1 AND created_at > now() - interval '10 minutes'`,
		code).Scan(&s.Code, &s.Verification, &s.CodeChallenge, &s.ChallengeMethod, &s.Token, &s.UserID, &s.Email, &s.Error, &s.ApprovedUserID, &s.CreatedAt, &s.ApprovedAt, &s.CompletedAt)
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

func (d *DB) ApproveAuthSession(ctx context.Context, code, userID string) error {
	tag, err := d.Pool.Exec(ctx,
		`UPDATE auth_sessions
		 SET approved_user_id = $2::uuid,
		     approved_at = now(),
		     error = NULL
		 WHERE code = $1
		   AND completed_at IS NULL
		   AND created_at > now() - interval '10 minutes'`,
		code, userID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrSessionNotFound
	}
	return nil
}

func (d *DB) ConsumeApprovedAuthSession(ctx context.Context, code string) (*AuthSession, error) {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("Starting auth-session exchange transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var s AuthSession
	err = tx.QueryRow(ctx,
		`SELECT code, verification, code_challenge, challenge_method, token, user_id, email, error, approved_user_id, created_at, approved_at, completed_at
		 FROM auth_sessions
		 WHERE code = $1
		   AND created_at > now() - interval '10 minutes'
		 FOR UPDATE`,
		code,
	).Scan(&s.Code, &s.Verification, &s.CodeChallenge, &s.ChallengeMethod, &s.Token, &s.UserID, &s.Email, &s.Error, &s.ApprovedUserID, &s.CreatedAt, &s.ApprovedAt, &s.CompletedAt)
	if err != nil {
		return nil, err
	}

	if s.ApprovedAt == nil || s.ApprovedUserID == nil || s.CompletedAt != nil {
		return nil, ErrSessionNotFound
	}

	if _, err := tx.Exec(ctx,
		`UPDATE auth_sessions
		 SET completed_at = now()
		 WHERE code = $1`,
		code,
	); err != nil {
		return nil, fmt.Errorf("Completing approved auth session: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("Committing auth-session exchange: %w", err)
	}
	return &s, nil
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
