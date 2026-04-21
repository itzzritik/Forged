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
last_verified: 2026-04-21
stable: partial
---

# Web Auth Flow

Two login flows (web-only and CLI-assisted) converge on one callback
(`/api/auth/callback`) and one encrypted session cookie. CLI flow uses
device-code polling plus a `FORGE-<verification>` string the user confirms
before approving the browser callback. Server endpoints in `server/api.md`.

`.agents/plan/auth-hardening/` adds OAuth PKCE and replaces the long-lived
session with short access + rotating refresh.

## Must know

- **`/api/auth/callback` is the ONLY cookie setter.** Adding a second
  setter fragments the lifecycle and breaks logout.
- **`forged_logged_in` is NOT a security signal** — non-HttpOnly UI hint
  only. Auth gating always reads the encrypted `forged_session`.
- **Session cookie wraps the server JWT with AES-GCM under `AUTH_SECRET`**
  (Web Crypto, 12-byte IV prefix, base64). Decrypt returns `null` on any
  error. IF `AUTH_SECRET` < 32 bytes THEN web refuses to encrypt/decrypt
  — no fallback.
- **Logout has NO server-side revocation.** The JWT stays valid on
  `forged-server` until its 30-day exp. Web just clears cookies.
- **JWT `exp` is checked in `proxy.ts`, not in `/api/auth/callback`.** A
  callback with an expired token still sets the cookie; first gated
  request rejects.
- **CSP (`next.config.ts`) pins `connect-src`** to `'self'` and
  `https://forged-api.ritik.me`. A different `NEXT_PUBLIC_API_URL`
  silently breaks the verification fetch unless CSP is updated.
- **The verification string is the only cross-device binding.** No PKCE,
  no IP lock, no device binding on the CLI session today — a stolen code
  is defeated only by the user refusing to click "Continue".
- **`/login` with `?code=` must render for authenticated users too** so
  they can approve the CLI handoff. Do NOT auto-redirect them away.
- Cookie is `HttpOnly`, `SameSite=Lax`, `Secure` in prod, 30 days.
  Downgrading any enables XSS exfiltration or cross-site posting.
- CLI polls `/sessions/{code}` every 2s (exp backoff to 10s, 5 min deadline).
- IF the session cookie contains anything password-derived THEN
  zero-knowledge is broken. It only ever wraps the server JWT.
- Auth gating is in `web/src/proxy.ts` — matcher covers `/dashboard/**`,
  `/login`, `/auth/success`. Rate limiting lives on `forged-server`.

## Decisions

- Device-flow with polling, not a localhost redirect. `forged` never binds
  a local HTTP server — works under headless servers, strict firewalls,
  SSH forwarding. Do NOT add a localhost path without keeping polling
  default.
- Session rides in a HttpOnly, encrypted-at-rest cookie, not
  `localStorage` or a readable cookie. HttpOnly blocks XSS theft;
  encryption blocks replay without `AUTH_SECRET`.
- Web has no OAuth client of its own. Providers only know
  `forged-server`. Do NOT add provider buttons that bypass the server.
