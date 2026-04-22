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
last_verified: 2026-04-22
stable: partial
---

# Daemon

Long-running per-user process. Hosts `agent.sock` (SSH) and `ctl.sock`
(internal IPC), plus activity log, sync bus, sensitive-auth broker, and
SSH routing. It can now run in a **cold** state with no active vault
session, or an **active** state with the vault open, private keys
decrypted, and key buffers mlocked. Runs as a launchd / systemd user /
Task Scheduler service, or in the foreground via `forged daemon` (same
code path).

## Must know

- **One daemon per user.** PID file + socket peer checks enforce it; a
  second live daemon aborts at startup.
- **Daemon can now boot cold if no startup password is provided.** In
  that state sockets come up, but vault-backed IPC and agent operations
  stay locked until sensitiveauth hydrates a live session.
- **Installed service now starts cold by default.** launchd/systemd /
  Task Scheduler bootstrap no longer stores or passes the master
  password. Service repair and reinstall no longer prompt for it either.
- **Foreground `forged daemon` no longer prompts on a TTY.** It starts
  cold unless a password is explicitly supplied through
  `FORGED_MASTER_PASSWORD` or stdin piping / redirection.
- **Decrypted keys stay in daemon memory for its whole lifetime** once
  a shared session is active. Session expiry now clears the live daemon
  session, but the next hardening slices still need fuller lock/sleep
  invalidation and tighter in-memory lifetime guarantees.
- **Windows is not fully supported** — no peer-PID, no control-socket
  transport (see `cli/ipc.md`).
- **macOS legacy label `me.ritik.forged`** is detected and migrated to
  `me.ritik.forged.daemon` on service start; old plist removed.
- **`just dev` does NOT run `forged daemon` directly.** It runs
  `forged-dev-service install`, reusing the normal `InstallService` path.
- IF startup password is provided and the vault cannot open THEN daemon
  exits before writing PID or opening sockets. In cold-start mode the
  daemon still starts and waits for a later hydrate.
- Sync bus only initializes when an active vault session exists. Clearing
  the active session also clears the live sync bus reference.
- Stale socket/PID detection handles hard-kill recovery; clean shutdown
  zeros key buffers, `munlock`s, closes the vault.
- Activity log is a 1000-entry in-memory ring; file logging rotates
  (lumberjack: 10 MB × 3 backups, 30 days).

## Decisions

- Long-running, not on-demand. SSH agent must answer instantly with keys
  in memory; per-request spin-up re-derives the Symmetric Key and defeats
  mlock.
- Cold boot is now the default for both foreground and installed-service
  daemon startup. Active vault sessions are still fully in-memory once
  hydrated. The next hardening slices will shorten that lifetime with
  platform-complete lock/sleep invalidation and stricter policy.
- Service install lives inside the CLI binary. No external installer, no
  package postinstall. TUI doctor drives install and repair.
- Agent and control sockets stay split (see `architecture/overview.md`).
