---
title: Server HTTP API
applies_to:
  - server/cmd/forged-server/**
  - server/internal/api/**
  - server/internal/auth/**
  - server/internal/middleware/**
depends_on:
  - architecture/security-model.md
last_verified: 2026-04-21
stable: partial
---

# Server HTTP API

`forged-server` is a dumb encrypted-blob store plus OAuth provider under
`/api/v1/*`. Stdlib `net/http` with Go 1.22 method+path routing — no
framework. Payload detail in `proto/sync-api.md`; DB in
`server/db-schema.md`.

Routes: OAuth login (Google/GitHub, public), CLI device-flow sessions
(public, rate-limited), sync push/pull/status (authed), devices (authed),
account (authed), vault rekey (authed), `/health` (public), and a
dev-only `/api/v1/auth/dev` gated by `REDIRECT_BASE_URL` being unset.

`.agents/plan/auth-hardening/` adds OAuth PKCE and swaps long-lived JWTs
for short access + rotating refresh. Token lifetime and login handoff
shape are most likely to drift.

## Must know

- **`proto/sync-api.md` is stale.** Still documents `/auth/register` +
  `/auth/login` with server-side bcrypted "auth-derived key" (removed in
  migration 005) and `X-Vault-Version` + raw octet-stream bodies for
  push/pull. Shipped sync uses a JSON envelope carrying base64 `blob`,
  `kdf_params`, `protected_symmetric_key`, `expected_version`, `device_id`.
  Trust the handlers.
- **`handleDevAuth` is a back door** to any account — gated only by
  `REDIRECT_BASE_URL` being unset. Production MUST set it.
- **`clientIP` trusts `X-Forwarded-For` as-is.** No proxy whitelist. Rate
  limiting is spoofable unless the deployment terminates trusted proxies.
- **OAuth `state` carries the CLI session code in the clear.** An
  intercepted code completes login. PKCE is planned (`auth-hardening/`) —
  do not design features assuming state-only binding.
- **JWT is HS256 with 30-day exp and no refresh.** Revocation = delete
  the user. Short-lived access + rotating refresh is planned.
- **CORS is `Access-Control-Allow-Origin: *`.** Safe today because the
  bearer rides in `Authorization`, not a cookie. Adding cookies requires
  tightening the origin.
- Rate limiting is in-process per-IP, scoped to the three auth-session
  endpoints only. Sync/device/account have none at the server.
- Auth-session rows are single-use; poll responses set `Cache-Control:
  no-store`. GC goroutine sweeps every 5 minutes.
- IF a new `/api/v1/*` route is mutating THEN register on the `authed`
  mux. Only OAuth redirects, three session endpoints, `/health`, and
  dev-auth may be public.
- IF a request body contains the master password or any password-derived
  key THEN reject at design time — zero-knowledge invariant.
- IF a sync push's `expected_version` mismatches THEN return 409; client
  pulls + re-pushes. Do not overwrite.

## Decisions

- Stdlib `net/http` + Go 1.22 method routing. No chi/gin/echo. Keeps the
  binary small and the surface auditable.
- OAuth-only (Google, GitHub). Email/password removed with migration 005
  alongside `master_password_hash`. Do NOT reintroduce password auth —
  reverses zero-knowledge.
- JWT HS256 with shared `JWT_SECRET`, not JWE / asymmetric. Only the
  server and web proxy verify. Reconsider if a third verifier appears.
- Rate limiting scoped to auth-session endpoints — they're the only ones
  callable without a bearer.
