---
title: Sensitive Auth
applies_to:
  - cli/internal/sensitiveauth/**
  - cli/cmd/forged-auth/**
depends_on:
  - architecture/security-model.md
  - cli/daemon.md
  - cli/ipc.md
last_verified: 2026-04-21
stable: partial
---

# Sensitive Auth

Gate between IPC handlers and paths that expose private-key bytes.
Viewing a private key in the clear and exporting the whole vault go
through here. Public metadata, listings, and SSH agent signing do NOT.

Broker lives in the daemon. Native prompt delegated to the
`forged-auth` helper binary over line-delimited JSON stdio (macOS:
`LocalAuthentication`; Linux: `pkexec`; Windows: PowerShell + Hello).
Helper also subscribes to platform session-lock events and clears the
lease on workstation lock.

Hardening plan (`.agents/plan/security-hardening/`) is reworking what
"lock" means at the crypto layer.

## Must know

- **"Lock" today does NOT re-encrypt decrypted private keys.** It flips
  the broker lease. Daemon keeps decrypted keys in mlocked memory for its
  whole lifetime; SSH agent signing continues regardless of lock state.
- **View lease has no TTL.** Lives until explicit `sensitive-lock`,
  workstation lock, broker close, or daemon shutdown. A long laptop
  session stays "unlocked" indefinitely.
- **Export token is separate from the view lease.** Random UUID, 1-minute
  TTL, single-use. `export-all` requires a fresh token even if the view
  lease is active.
- **Linux "biometric" is actually `pkexec`** (user password). Windows is
  best-effort PowerShell + Hello.
- **IF the native helper is unavailable** (missing, crash, `unavailable`)
  **THEN `Authorize` returns `PasswordRequired=true`** with a prompt
  string — it does NOT implicitly grant. Caller falls back to master
  password via `sensitive-password`.
- **`view` has only two actions: `view` and `export`.** No per-key
  scoping. Broker does NOT gate SSH agent `Sign`.
- A legacy in-process fallback in `provider_darwin.go` writes a Swift
  script and `exec`s `swift`. Retained for environments where the helper
  binary is absent.
- IPC deadline extends to 5 minutes for `sensitive-auth` /
  `sensitive-password` because user may sit on the prompt.
- `sensitive-password` zeros the password buffer before returning,
  regardless of outcome.
- IF the broker is not wired THEN every sensitive command returns
  "unavailable" rather than falling through.

## Decisions

- Native auth is a long-lived helper binary, not inline cgo.
  `LocalAuthentication` requires a signed bundle on macOS, and isolating
  prompts keeps the daemon free of UI frameworks. Do NOT inline.
- View lease (not per-op prompts) because users view/copy keys in quick
  succession from the TUI. Export token is the exception — single,
  auditable dump.
- Do NOT cache decrypted PEMs outside the vault store. The hardening plan
  will drop key material on lock; external stashes will break.
- The broker is a UX/policy gate, NOT a second auth layer on IPC. Owner-only
  `ctl.sock` perms are the access control. See `cli/ipc.md`.
