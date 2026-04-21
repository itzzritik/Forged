---
title: Server DB Schema
applies_to:
  - server/internal/db/**
  - server/migrations/**
  - server/cmd/migrate/**
last_verified: 2026-04-21
stable: yes
---

# Server DB Schema

Postgres (pgx) backs `forged-server`. Five tables: `users`, `vaults`,
`devices`, `audit_log`, `auth_sessions`. Migration runner is a tiny
file-scan tool at `server/cmd/migrate/`. Despite plan references to
CockroachDB, production uses vanilla Postgres — `pgcrypto` extension
and `gen_random_uuid()`.

## Must know

- **Migration 005 dropped server-side password verification.** Removed
  columns: `users.master_password_hash`, `users.vault_unlock_attempts`,
  `users.vault_locked_until`. These were added in migration 004, shipped,
  and yanked. IF any new feature wants to verify a password server-side
  THEN it reverses zero-knowledge — reject at design time.
- **Migrations apply in lexical filename order**, tracked in
  `schema_migrations(version TEXT PK)`. The runner bootstraps by marking
  `001_init.sql` as applied when a pre-existing `users` table is detected
  (old deployments predate the tracking table). New migrations MUST keep
  numeric prefixes so ordering stays correct.
- **`000_drop.sql` is only executed under `migrate reset`** — a manual
  nuke, never part of the normal forward sweep. The runner explicitly
  skips it in the glob loop.
- **`vaults.user_id` is `UNIQUE`** — one vault per user. Multi-vault
  requires a schema break, not just an extra row.
- **Optimistic locking is `vaults.version`** (BIGINT). Push uses
  `WHERE version = expected_version` and returns the incremented value;
  no row updated → 409 Conflict. Pull/status return the vault row's
  `kdf_params` + `protected_symmetric_key` alongside the blob.
- **First-push sentinel: `expected_version = 0`** triggers an `ON
  CONFLICT ... WHERE vaults.version = 0` upsert. This guards the empty-
  vault race but means *legitimate* first push after a delete-then-recreate
  has no protection.
- **`devices.approved` auto-true for the first device** on a user, false
  for subsequent devices. No server UI to approve — the client must hit
  `/devices/{id}/approve`. A non-approved device can still call sync.
- **`audit_log` is append-only with no index on `user_id`.** `CleanupAuditLog`
  deletes older than 90 days; no pagination API for reads.
- **`auth_sessions.code` is the primary key AND the polling secret.** A
  10-minute `WHERE created_at > now() - interval '10 minutes'` guard
  lives in every query; no expiry column, no token-rotation.
- **`users.key_generation`** tracks password/KDF rotation intent but
  `ChangePassword` does not bump it (see `cli/vault.md`). Reserved for a
  future true Symmetric Key rotation.
- Connection pool: `MaxConns=5, MinConns=1, MaxConnLifetime=5m`. Fixed
  constants — no env config.

## Decisions

- Plain Postgres, no ORM, no sqlc. Hand-written SQL against `pgxpool`.
  Schema churn is slow; the complexity of codegen is not earned.
- File-sorted migration runner, no up/down. Forward-only. Rollback is a
  new forward migration.
- Server stores `kdf_params` and `protected_symmetric_key` alongside the
  blob so a fresh browser/device can prompt the user for their password
  without a full pull.
