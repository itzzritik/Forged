---
title: Server HTTP API
applies_to:
  - server/cmd/forged-server/**
  - server/internal/api/**
  - server/internal/auth/**
  - server/internal/middleware/**
depends_on:
  - architecture/security-model.md
last_verified: 2026-04-23
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
- Sync push uses optimistic locking and returns 409 on version mismatch. The client handles merge/retry.
- `handleDevAuth` is still a real back door when `REDIRECT_BASE_URL` is unset. That must stay disabled in production.

## Decisions

- Keep stdlib `net/http`.
- Keep OAuth-only account auth. Do not add server-side password auth.
