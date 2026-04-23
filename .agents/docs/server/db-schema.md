---
title: Server DB Schema
applies_to:
  - server/internal/db/**
  - server/migrations/**
  - server/cmd/migrate/**
last_verified: 2026-04-23
stable: yes
---

# Server DB Schema

Postgres stores users, vault blobs, devices, audit rows, short-lived auth sessions, and refresh-backed login sessions.

## Must know

- `users` no longer has any server-side password-verification fields.
- `vaults.user_id` is unique: one vault per user.
- `vaults.version` is the optimistic-lock column for sync.
- `auth_sessions` are short-lived browser/CLI handoff rows.
- `refresh_sessions` are the real long-lived login state. Rotation and revoke act on these rows.
- Migrations are forward-only and run in lexical order.

## Decisions

- Keep hand-written SQL and plain pgx.
- Keep one-vault-per-user until a real multi-vault design exists.
