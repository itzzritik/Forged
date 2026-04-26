---
title: Daemon IPC
applies_to:
  - cli/internal/ipc/**
last_verified: 2026-04-27
stable: yes
---

# Daemon IPC

`ctl.sock` is a one-request control channel between `forged` and the daemon. One connection carries one request and one response.

## Must know

- Socket ownership and `0600` perms are the main access control. Sensitive operations still add broker checks on top.
- Vault-backed handlers can be called while the daemon is cold; they should return a locked error, not panic.
- `proto/ipc.md` is not current. Code is the source of truth for the command set.
- `sensitive-auth` takes an `action` and optional `force`. `force=true` is used for launch auth.
- `status` exposes enough sensitive state for the TUI to detect cold vs active session.
- Key list/view/export handlers ask the sync bus for a lightweight foreground refresh before reading local vault data.
- Hidden SSH route IPC prepares per-attempt snippets from `%C`, `%h`, `%p`, `%r`, and `%n`; prepare failures are quiet so the managed SSH config fails closed with no default identities.
- Windows IPC support is still incomplete.

## Decisions

- Keep the flat JSON-over-socket model. The surface is small enough that gRPC/codegen is not worth it.
