---
title: Server HTTP API
applies_to:
  - server/cmd/forged-server/**
  - server/internal/api/**
  - server/internal/auth/**
  - server/internal/middleware/**
depends_on:
  - architecture/security-model.md
last_verified: 2026-10-13
stable: partial
---

# Server HTTP API

`forged-server` is an encrypted-blob store plus OAuth/session backend. It never sees plaintext vault contents or the master password.

## Must know

- Google and GitHub are the only login providers.
- CLI login is still browser-assisted polling, but completion now goes through a PKCE-style exchange step instead of handing out tokens from the poll result.
- Auth is now:
  - short-lived access token
  - rotating refresh token
  - server-side revoke on logout
- Public auth-session endpoints are rate-limited. Protected routes require bearer auth.
- Sync push uses optimistic locking and returns 409 on version mismatch. The client handles merge/retry. Only a missing vault row gives `has_vault:false` or a pull 404, and only a version mismatch gives 409; any other database error is a 500, so a database outage never looks like a missing vault to clients.
- Push never replaces an existing password wrap (KDF params plus protected key); only `POST /api/v1/vault/rekey` does, so a device holding an old wrap cannot revert a password change.
- `handleDevAuth` is still a real back door when `REDIRECT_BASE_URL` is unset. That must stay disabled in production.

## Decisions

- Keep stdlib `net/http`.
- Keep OAuth-only account auth. Do not add server-side password auth.
