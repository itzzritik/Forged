---
title: Web Auth Flow
applies_to:
  - web/src/app/login/**
  - web/src/app/auth/**
  - web/src/app/api/auth/**
  - web/src/lib/auth.ts
  - web/src/proxy.ts
depends_on:
  - server/api.md
  - architecture/security-model.md
last_verified: 2026-04-23
stable: partial
---

# Web Auth Flow

Web login and CLI-assisted login both finish through `/api/auth/callback` and one encrypted session cookie.

## Must know

- `/api/auth/callback` is the only place that should set the browser session cookie.
- `forged_session` now stores an encrypted session object:
  - access token + expiry
  - refresh token + expiry
  - user summary
- `forged_logged_in` is only a UI hint.
- API proxy paths can refresh access on demand and rewrite the cookie with rotated session state.
- Logout now revokes the refresh-backed server session best-effort before clearing cookies.
- `/login?code=...` must stay reachable even when the user is already logged in.
- No password-derived material belongs in the browser session cookie.

## Decisions

- Keep browser-assisted polling for CLI login.
- Keep the session in an HttpOnly encrypted cookie, not readable browser storage.
