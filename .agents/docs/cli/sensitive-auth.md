---
title: Sensitive Auth
applies_to:
  - cli/internal/sensitiveauth/**
  - cli/cmd/forged-auth/**
depends_on:
  - architecture/security-model.md
  - cli/daemon.md
  - cli/ipc.md
last_verified: 2026-04-23
stable: partial
---

# Sensitive Auth

Sensitive auth is the gate for private-key use and live daemon-session hydrate. The broker lives in the daemon; native prompts come from `forged-auth`.

## Must know

- There is one shared active-use session for TUI private-key access and external signing/auth.
- That session ends on:
  - expiry
  - system lock/sleep
  - explicit TUI idle lock
  - daemon restart
- Fresh TUI launch always asks for auth. If local unlock trust is missing, it goes straight to the master-password screen.
- Open TUI sessions relock after system lock/sleep and after 4 minutes of idle time.
- External use never gets master-password fallback. It either uses native auth or follows the local external-use policy when native auth is truly unavailable.
- `broken` and `unavailable` are separate states. The external-use policy applies only to true unavailability.
- `Master Password Interval` is a local device policy because local unlock trust is per-device.
- Successful master-password fallback refreshes local unlock trust best-effort.
- Export and change-password are always master-password-only.

## Decisions

- Native auth stays in a helper binary, not inside the daemon.
- One shared session is the UX/security balance; export is the intentional exception.
