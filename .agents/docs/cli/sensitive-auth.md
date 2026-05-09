---
title: Sensitive Auth
applies_to:
  - cli/internal/sensitiveauth/**
  - cli/cmd/forged-auth/**
depends_on:
  - architecture/security-model.md
  - cli/daemon.md
  - cli/ipc.md
last_verified: 2026-05-10
stable: partial
---

# Sensitive Auth

Sensitive auth is the gate for private-key use and live daemon-session hydrate. The broker is the single policy owner in the daemon; System Auth prompts come from `forged-auth`.

## Must know

- There is one shared active-use session for TUI access and external signing/auth. It lasts 4 hours unless the system locks/sleeps, the user locks Forged, or the daemon restarts.
- That session ends on:
  - expiry
  - system lock/sleep
  - explicit TUI idle lock
  - daemon restart
- On desktop, TUI unlock tries System Auth first and falls back to the universal master-password page on cancel, failure, unavailable System Auth, or missing device unlock.
- On desktop, external SSH/signing tries System Auth and denies on cancel/failure. It never falls back to master password.
- On headless machines, System Auth is treated as unavailable. The first successful master-password unlock creates file-backed local unlock trust; after that TUI and external SSH/signing hydrate from it without prompting.
- Export and change-password are always master-password-only. Export issues a short export token and does not rely on System Auth.
- Open TUI sessions relock after system lock/sleep and after 4 minutes of idle time.
- External System Auth prompts are single-flight with a short failure cooldown so parallel SSH/signing requests do not spam prompts.
- SSH route preparation normally uses public in-memory route data and does not prompt. If a cold daemon has no route cache, it may trigger external auth once to hydrate the vault before writing the route snippet.
- `broken` and `unavailable` are separate states. Unavailable means no System Auth path, usually headless; broken means the desktop System Auth path exists but failed unexpectedly.
- `Master Password Interval` is a local device policy because device unlock enrollment is per-device.
- Successful master-password fallback refreshes device unlock best-effort.

## Decisions

- System Auth stays in a helper binary, not inside the daemon.
- One shared session is the UX/security balance; export is the intentional exception.
