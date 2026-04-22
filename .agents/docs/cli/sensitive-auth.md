---
title: Sensitive Auth
applies_to:
  - cli/internal/sensitiveauth/**
  - cli/cmd/forged-auth/**
depends_on:
  - architecture/security-model.md
  - cli/daemon.md
  - cli/ipc.md
last_verified: 2026-04-22
stable: partial
---

# Sensitive Auth

Gate between IPC handlers and paths that expose private-key bytes.
Viewing a private key in the clear and exporting the whole vault go
through here. Public IPC metadata/listings still do not, but SSH agent
list/sign/signers can now trigger sensitiveauth when the daemon is cold
and needs to hydrate a live session.

Broker lives in the daemon. Native prompt delegated to the
`forged-auth` helper binary over line-delimited JSON stdio (macOS:
`LocalAuthentication`; Linux: `pkexec`; Windows: PowerShell + Hello).
Helper also subscribes to platform session-lock events and clears the
shared session on workstation lock.

Hardening plan (`.agents/plan/security-hardening/`) is reworking what
"lock" means at the crypto layer.

## Must know

- **Session clear today does NOT re-encrypt the vault on disk.** It drops
  the shared session and clears the live daemon session. The daemon then
  returns to cold state until the next successful auth.
- **Shared private-key session now has a 4-hour TTL.** Successful TUI
  auth or external-use auth refreshes the same shared window. Expiry
  clears both the broker session and the live daemon session.
- **External use now has its own auth action.** SSH auth, `SIGN_REQUEST`,
  and `SIGNERS` go through `ActionExternal`, not `ActionView`. That means
  they never get a master-password fallback prompt from the broker.
- **Foreground TUI idle can also clear the shared session.** After 4
  minutes with no key input, the TUI calls `sensitive-lock`, which drops
  the broker session and the live daemon session. This is a real relock,
  not just a local screen change.
- **Export token is separate from the view lease.** Random UUID, 1-minute
  TTL, single-use. `export-all` requires a fresh token even if the view
  session is active.
- **Linux "biometric" is actually `pkexec`** (user password). Windows is
  best-effort PowerShell + Hello.
- **macOS native auth disables LocalAuthentication reuse.** The helper
  and darwin fallback set Touch ID reuse duration to `0`, so a fresh TUI
  launch asks again instead of silently reusing a recent biometric auth.
- **IF the native helper is unavailable** (missing, crash, `unavailable`)
  **THEN behavior now depends on action class.** `view` / TUI launch may
  fall back to master password. `external` either follows
  `security.external_use_policy` on true unavailability or fails on a
  broken helper. It does NOT get a password prompt.
- **Broken is not unavailable anymore.** Transport/startup failures in
  the auth helper now map to `CapabilityBroken`. True helper responses
  map to `CapabilityUnavailableByPlatform` or
  `CapabilityUnavailableByEnv`. Only true unavailability can use the
  external-use policy.
- **Helper `status` is now a real capability probe.** TUI diagnostics no
  longer assume native auth is available. `forged-auth` answers a
  non-interactive `status` request with:
  - `ok`
  - `unavailable_by_platform`
  - `unavailable_by_environment`
  - `broken`
  and the TUI uses that for Manage/Doctor security state.
- **External-use policy lives in `config.toml`.** `security.external_use_policy`
  defaults to `deny`. When set to `allow`, and native auth is truly
  unavailable, the broker may hydrate from local unlock trust without a
  prompt. If local unlock trust cannot hydrate, external use still fails.
- **`AuthorizeForced` exists for fresh TUI launch.** It bypasses the
  broker's "shared session already active" short-circuit and makes the
  helper prompt again. This is how `forged` asks for Touch ID on every
  vault-backed launch without breaking the shared 4-hour session model
  for external use.
- **Successful master-password fallback now refreshes local unlock trust
  best-effort.** That path verifies the password by recovering the vault
  Symmetric Key, then tries to write:
  - `config/local-unlock.json`
  - `config/install.id`
  - secure-storage device key entry
  The daemon still attempts that refresh, and the foreground CLI now
  retries it locally after successful password fallback so the next TUI
  launch can use native auth. If refresh still fails, the current auth
  succeeds and a warning is logged.
- **Successful auth now hydrates a cold daemon session.** Native auth
  tries local enrollment first; master-password fallback hydrates through
  the password path. Broker invalidation now clears the live daemon
  session, not just a local gate bit.
- **If native auth succeeds but local enrollment cannot hydrate** (missing,
  expired, corrupt, or mismatched local unlock state), the broker now
  returns `PasswordRequired=true` with a distinct prompt instead of
  surfacing the raw hydration error. TUI/Manage can fall back to master
  password without incorrectly saying native auth was unavailable;
  external use still does not get a password prompt.
- **TUI launch now pre-checks for obvious missing local trust.** If
  `local-unlock.json` or `install.id` is absent, startup skips native
  auth and goes straight to the master-password screen.
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
- Shared session (not per-op prompts) because users view/copy keys in
  quick succession from the TUI and then often sign immediately after.
  Export token is the exception — single, auditable dump.
- Do NOT cache decrypted PEMs outside the vault store. The hardening plan
  will drop key material on lock; external stashes will break.
- Enrollment refresh is attached to real master-password verification, not
  a side-channel command. Do NOT add a second path that writes local
  unlock state without first recovering the vault Symmetric Key.
- The broker now owns both the shared-session timer and the daemon
  session-clear callback. If you add new unlock paths, wire both or the
  daemon will drift into stale "authorized but cold" state.
- `security.external_use_policy=allow` is only a bypass for true native
  unavailability. Do NOT apply it when the helper is broken.
- The broker is a UX/policy gate, NOT a second auth layer on IPC. Owner-only
  `ctl.sock` perms are the access control. See `cli/ipc.md`.
