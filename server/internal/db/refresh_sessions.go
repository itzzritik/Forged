package db

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

var (
	ErrRefreshSessionNotFound = errors.New("refresh session not found")
	ErrRefreshSessionExpired  = errors.New("refresh session expired")
	ErrRefreshSessionRevoked  = errors.New("refresh session revoked")
	ErrRefreshSessionReplay   = errors.New("refresh session replay detected")
	ErrRefreshSessionInvalid  = errors.New("refresh session invalid")
)

type RefreshSession struct {
	ID           string
	UserID       string
	FamilyID     string
	SecretHash   []byte
	CreatedAt    time.Time
	LastUsedAt   *time.Time
	ExpiresAt    time.Time
	RotatedFrom  *string
	RevokedAt    *time.Time
	RevokeReason *string
}

func (d *DB) CreateRefreshSession(ctx context.Context, userID, familyID string, secretHash []byte, expiresAt time.Time, rotatedFrom *string) (RefreshSession, error) {
	var session RefreshSession
	var rotated string
	if rotatedFrom != nil {
		rotated = *rotatedFrom
	}
	err := d.Pool.QueryRow(ctx,
		`INSERT INTO refresh_sessions (user_id, family_id, secret_hash, expires_at, rotated_from)
		 VALUES (
		   $1,
		   COALESCE(NULLIF($2, '')::uuid, gen_random_uuid()),
		   $3,
		   $4,
		   NULLIF($5, '')::uuid
		 )
		 RETURNING id, user_id, family_id, secret_hash, created_at, last_used_at, expires_at, rotated_from, revoked_at, revoke_reason`,
		userID, familyID, secretHash, expiresAt, rotated,
	).Scan(
		&session.ID,
		&session.UserID,
		&session.FamilyID,
		&session.SecretHash,
		&session.CreatedAt,
		&session.LastUsedAt,
		&session.ExpiresAt,
		&session.RotatedFrom,
		&session.RevokedAt,
		&session.RevokeReason,
	)
	if err != nil {
		return RefreshSession{}, fmt.Errorf("Creating refresh session: %w", err)
	}
	return session, nil
}

func (d *DB) RotateRefreshSession(ctx context.Context, sessionID string, presentedSecretHash []byte, newSecretHash []byte, newExpiresAt time.Time) (RefreshSession, User, error) {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return RefreshSession{}, User{}, fmt.Errorf("Starting refresh rotation transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	session, user, err := loadRefreshSessionForUpdate(ctx, tx, sessionID)
	if err != nil {
		return RefreshSession{}, User{}, err
	}

	if session.RevokedAt != nil {
		if subtle.ConstantTimeCompare(session.SecretHash, presentedSecretHash) == 1 &&
			session.RevokeReason != nil &&
			(*session.RevokeReason == "rotated" || *session.RevokeReason == "replayed") {
			if err := revokeRefreshFamily(ctx, tx, session.FamilyID, "replayed"); err != nil {
				return RefreshSession{}, User{}, err
			}
			if err := tx.Commit(ctx); err != nil {
				return RefreshSession{}, User{}, fmt.Errorf("Committing refresh replay revoke: %w", err)
			}
			return RefreshSession{}, User{}, ErrRefreshSessionReplay
		}
		return RefreshSession{}, User{}, ErrRefreshSessionRevoked
	}

	now := time.Now().UTC()
	if !session.ExpiresAt.After(now) {
		if _, err := tx.Exec(ctx,
			`UPDATE refresh_sessions
			 SET revoked_at = COALESCE(revoked_at, now()),
			     revoke_reason = COALESCE(revoke_reason, 'expired')
			 WHERE id = $1`,
			session.ID,
		); err != nil {
			return RefreshSession{}, User{}, fmt.Errorf("Marking expired refresh session: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return RefreshSession{}, User{}, fmt.Errorf("Committing expired refresh session update: %w", err)
		}
		return RefreshSession{}, User{}, ErrRefreshSessionExpired
	}

	if subtle.ConstantTimeCompare(session.SecretHash, presentedSecretHash) != 1 {
		return RefreshSession{}, User{}, ErrRefreshSessionInvalid
	}

	var next RefreshSession
	err = tx.QueryRow(ctx,
		`INSERT INTO refresh_sessions (user_id, family_id, secret_hash, expires_at, rotated_from)
		 VALUES ($1, $2::uuid, $3, $4, $5::uuid)
		 RETURNING id, user_id, family_id, secret_hash, created_at, last_used_at, expires_at, rotated_from, revoked_at, revoke_reason`,
		session.UserID, session.FamilyID, newSecretHash, newExpiresAt, session.ID,
	).Scan(
		&next.ID,
		&next.UserID,
		&next.FamilyID,
		&next.SecretHash,
		&next.CreatedAt,
		&next.LastUsedAt,
		&next.ExpiresAt,
		&next.RotatedFrom,
		&next.RevokedAt,
		&next.RevokeReason,
	)
	if err != nil {
		return RefreshSession{}, User{}, fmt.Errorf("Creating rotated refresh session: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`UPDATE refresh_sessions
		 SET revoked_at = now(),
		     revoke_reason = 'rotated',
		     last_used_at = now()
		 WHERE id = $1`,
		session.ID,
	); err != nil {
		return RefreshSession{}, User{}, fmt.Errorf("Revoking rotated refresh session: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return RefreshSession{}, User{}, fmt.Errorf("Committing refresh rotation: %w", err)
	}

	return next, user, nil
}

func (d *DB) RevokeRefreshSession(ctx context.Context, sessionID string, presentedSecretHash []byte, reason string) error {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("Starting refresh revoke transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	session, _, err := loadRefreshSessionForUpdate(ctx, tx, sessionID)
	if err != nil {
		return err
	}

	if subtle.ConstantTimeCompare(session.SecretHash, presentedSecretHash) != 1 {
		return ErrRefreshSessionInvalid
	}

	if session.RevokedAt != nil {
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("Committing idempotent refresh revoke: %w", err)
		}
		return nil
	}

	if _, err := tx.Exec(ctx,
		`UPDATE refresh_sessions
		 SET revoked_at = now(),
		     revoke_reason = $2,
		     last_used_at = now()
		 WHERE id = $1`,
		session.ID, reason,
	); err != nil {
		return fmt.Errorf("Revoking refresh session: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("Committing refresh revoke: %w", err)
	}
	return nil
}

func (d *DB) CleanupRefreshSessions(ctx context.Context) (int64, error) {
	tag, err := d.Pool.Exec(ctx,
		`DELETE FROM refresh_sessions
		 WHERE expires_at < now() - interval '7 days'
		    OR (revoked_at IS NOT NULL AND revoked_at < now() - interval '7 days')`,
	)
	if err != nil {
		return 0, fmt.Errorf("Cleaning refresh sessions: %w", err)
	}
	return tag.RowsAffected(), nil
}

func loadRefreshSessionForUpdate(ctx context.Context, tx pgx.Tx, sessionID string) (RefreshSession, User, error) {
	var session RefreshSession
	var user User
	err := tx.QueryRow(ctx,
		`SELECT
		    rs.id,
		    rs.user_id,
		    rs.family_id,
		    rs.secret_hash,
		    rs.created_at,
		    rs.last_used_at,
		    rs.expires_at,
		    rs.rotated_from,
		    rs.revoked_at,
		    rs.revoke_reason,
		    u.id,
		    u.email,
		    u.name,
		    u.provider,
		    u.provider_id,
		    u.key_generation,
		    u.created_at,
		    u.updated_at
		 FROM refresh_sessions rs
		 JOIN users u ON u.id = rs.user_id
		 WHERE rs.id = $1::uuid
		 FOR UPDATE`,
		sessionID,
	).Scan(
		&session.ID,
		&session.UserID,
		&session.FamilyID,
		&session.SecretHash,
		&session.CreatedAt,
		&session.LastUsedAt,
		&session.ExpiresAt,
		&session.RotatedFrom,
		&session.RevokedAt,
		&session.RevokeReason,
		&user.ID,
		&user.Email,
		&user.Name,
		&user.Provider,
		&user.ProviderID,
		&user.KeyGeneration,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return RefreshSession{}, User{}, ErrRefreshSessionNotFound
	}
	if err != nil {
		return RefreshSession{}, User{}, fmt.Errorf("Loading refresh session: %w", err)
	}
	return session, user, nil
}

func revokeRefreshFamily(ctx context.Context, tx pgx.Tx, familyID string, reason string) error {
	if _, err := tx.Exec(ctx,
		`UPDATE refresh_sessions
		 SET revoked_at = COALESCE(revoked_at, now()),
		     revoke_reason = CASE
		         WHEN revoke_reason IS NULL THEN $2
		         ELSE revoke_reason
		     END
		 WHERE family_id = $1::uuid`,
		familyID, reason,
	); err != nil {
		return fmt.Errorf("Revoking refresh session family: %w", err)
	}
	return nil
}
