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
last_verified: 2026-07-15
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
- A foreground startup password hydrates through the broker, so it establishes the same active session as an interactive password unlock.
- A delivered master-password authorization refreshes local unlock enrollment from the active vault session; a canceled or failed response does not change that durable trust.
- When the shared session is cleared, the daemon drops back to cold state.
- Session clear immediately cancels and detaches initial-link and sync-bus work before closing the vault. Stopped buses discard completion-state writes; old work drains separately, and a generation plus vault-identity fence prevents stale link or bus work from applying a vault merge or publishing a bus or sync-state metadata after its ownership is revoked. Revocation waits only for an already admitted bounded local commit.
- Account and sync-state transitions wait for detached work outside the live-session lock. Their credential and state-file commits use a validated, fingerprinted state snapshot without holding that lock; only the bounded pre-detach dirty transition is persisted while the live session is held so a crash cannot lose it.
- Sync initialization reserves a generation- and vault-fenced run while holding the live-session lock, then performs credential reads, state recovery/candidate preparation, and identity revalidation outside it. Its state work and activation save are run-gated outside that lock; a final dirty-marker check and bus publication remain brief session-owned steps. Clear, account changes, and unlink detach that run; stale workers cannot publish a bus or alter the current run's pending state.
- Sync only exists while account credentials are present and a live vault session is available.
- Local sync state preserves the three-way merge base and server version. Corrupt history is quarantined with a recovery marker, blocks sync and account-change state replacement, and is never auto-relinked or deleted as stale state.
- Account actions require the daemon's account-change protocol, repairing or restarting the managed service before sending versioned replace and clear commands. They never write credentials directly. Replacement and clear hold the credential lock through their sync-state transaction, so restore cannot publish state for the previous account mid-switch or logout. A manual or system lock does not wait for that transaction; an already admitted account transaction keeps sync paused until it completes, then initializes only from the current session and durable account state.
- Refreshing credentials for the same account and server retains its validated sync history; only a different-account transition stages that history away before linking.
- Logout commits local credentials and sync-state removal outside the live-session lock, then releases the credential lock before the bounded remote token revocation.
- While sync is active, learned SSH route proofs mark the vault dirty and the sync bus also runs low-frequency status checks.
- Service repair replaces any unmanaged `forged daemon` that still owns the runtime sockets before launchd/system service restart, and health checks only trust service sockets when the managed service PID matches the daemon PID file on platforms that expose it.
- Daemon status exposes a build id. Readiness treats a running daemon with a different or missing build id as degraded and repairs it by reinstalling/restarting the managed service.
- Linux user-service commands derive `XDG_RUNTIME_DIR` and `DBUS_SESSION_BUS_ADDRESS` when shells omit them, which is common in headless SSH or remote-editor sessions.
- macOS service start and removal boot out current and legacy launchd labels before deleting legacy plists, so an old KeepAlive job cannot respawn.
- Persistent Forged state lives under `~/.config/forged` on every OS. Auth/device trust lives under `~/.config/forged/auth`. Linux keeps runtime sockets under `/run/user/<uid>/forged`; macOS and Windows use `~/.config/forged/runtime` for runtime metadata, with Windows sockets using named pipes.
- Windows pipe names are opaque, domain-separated hashes of the current process token SID; startup fails before binding if that identity cannot be resolved.
- Windows deliberately starts no SSH route service. Startup migrates a uniquely marked Forged routing section in the private managed SSH config to agent-only form and removes local route artifacts; duplicate route markers or a missing generated `PermitLocalCommand` boundary block migration instead of being overwritten. Normal SSH-agent and commit-signing operations remain available.
- Windows support is still partial around socket transport and platform helpers.
- Windows service health reads numeric Task Scheduler state through COM instead of localized command output, and stop/restart/removal propagate control failures. Daemon PID liveness uses native process state; Unix uses signal-zero liveness.
- Shutdown is idempotent and two-phase: both listeners and their active connections close before any handler wait; native-auth waits are released without clearing the session; the vault closes only after admitted work finishes.
- A daemon publishes its PID only after both servers start and removes the PID file only while it still owns that record.
- Unix listeners never unlink a path before binding and remove it on close only while it is still the inode they created; stale cleanup refuses non-socket paths and inode replacements.
- Unix startup holds a persistent `daemon.lock` sidecar through shutdown and checks a live legacy PID before probing or removing socket paths; the lock file is never unlinked.

## Decisions

- The daemon stays long-running even in cold state.
- Service install and repair stay inside the CLI, not a separate installer.
