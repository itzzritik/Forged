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
last_verified: 2026-07-14
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
- Session clear waits for admitted sync work to finish before zeroing and closing the vault.
- Sync only exists while account credentials are present and a live vault session is available.
- Account actions require the daemon's account-change protocol, repairing or restarting the managed service before sending versioned replace and clear commands. They never write credentials directly. Credential commit is quick; reconciliation continues as bounded, generation-checked background work.
- Logout commits local credential and sync-state removal under the session lock, then releases it before the bounded remote token revocation.
- While sync is active, learned SSH route proofs mark the vault dirty and the sync bus also runs low-frequency status checks.
- Service repair replaces any unmanaged `forged daemon` that still owns the runtime sockets before launchd/system service restart, and health checks only trust service sockets when the managed service PID matches the daemon PID file on platforms that expose it.
- Daemon status exposes a build id. Readiness treats a running daemon with a different or missing build id as degraded and repairs it by reinstalling/restarting the managed service.
- Linux user-service commands derive `XDG_RUNTIME_DIR` and `DBUS_SESSION_BUS_ADDRESS` when shells omit them, which is common in headless SSH or remote-editor sessions.
- macOS service start and removal boot out current and legacy launchd labels before deleting legacy plists, so an old KeepAlive job cannot respawn.
- Persistent Forged state lives under `~/.config/forged` on every OS. Auth/device trust lives under `~/.config/forged/auth`. Linux keeps runtime sockets under `/run/user/<uid>/forged`; macOS and Windows use `~/.config/forged/runtime` for runtime metadata, with Windows sockets using named pipes.
- Windows support is still partial around socket transport and platform helpers.
- Windows service health reads numeric Task Scheduler state through COM instead of localized command output, and stop/restart/removal propagate control failures. Daemon PID liveness uses native process state; Unix uses signal-zero liveness.
- Shutdown is idempotent and two-phase: both listeners and their active connections close before any handler wait; native-auth waits are released without clearing the session; the vault closes only after admitted work finishes.
- A daemon publishes its PID only after both servers start and removes the PID file only while it still owns that record.
- Unix listeners never unlink a path before binding and remove it on close only while it is still the inode they created; stale cleanup refuses non-socket paths and inode replacements.
- Unix startup holds a persistent `daemon.lock` sidecar through shutdown and checks a live legacy PID before probing or removing socket paths; the lock file is never unlinked.

## Decisions

- The daemon stays long-running even in cold state.
- Service install and repair stay inside the CLI, not a separate installer.
