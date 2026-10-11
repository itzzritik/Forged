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
last_verified: 2026-10-11
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
- Parallel proxy requests share one refresh per refresh token (30 s, per server process). The API revokes the whole session family when a refresh token is used twice.
- Any 401 from `proxyToAPI` (no session, rejected refresh, upstream 401) also clears `forged_session` and `forged_logged_in`; otherwise `proxy.ts` sends `/login` straight back to `/dashboard` in a loop.
- A refresh that fails with 5xx or a network error is transient: it surfaces as a 500, never as a sign-out.
- Logout now revokes the refresh-backed server session best-effort before clearing cookies.
- `/login?code=...` must stay reachable even when the user is already logged in.
- No password-derived material belongs in the browser session cookie.

## Decisions

- Keep browser-assisted polling for CLI login.
- Keep the session in an HttpOnly encrypted cookie, not readable browser storage.
