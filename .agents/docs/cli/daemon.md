---
title: Daemon
applies_to:
  - cli/internal/daemon/**
  - cli/cmd/forged/cmd/daemon.go
  - cli/internal/platform/mlock_*.go
  - cli/internal/platform/socket_unix.go
  - cli/internal/platform/peer*.go
depends_on:
  - architecture/security-model.md
  - cli/ipc.md
last_verified: 2026-04-23
stable: partial
---

# Daemon

The daemon is the long-running per-user process behind SSH agent access, IPC, sync, and sensitive-auth session state.

## Must know

- The daemon hosts two sockets:
  - `agent.sock` for SSH clients
  - `ctl.sock` for CLI/TUI control
- It now boots cold by default. Installed services and foreground `forged daemon` no longer depend on a stored plaintext master password.
- A live vault session exists only after sensitive auth or password fallback hydrates it.
- When the shared session is cleared, the daemon drops back to cold state.
- Sync only exists while account credentials are present and a live vault session is available.
- Windows support is still partial around socket transport and platform helpers.

## Decisions

- The daemon stays long-running even in cold state.
- Service install and repair stay inside the CLI, not a separate installer.
