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
last_verified: 2026-04-21
stable: partial
---

# Daemon

Long-running per-user process. Opens the vault, mlocks decrypted keys,
hosts `agent.sock` (SSH) and `ctl.sock` (internal IPC). Composes
keystore, activity log, sync bus, sensitive-auth broker, SSH routing.
Runs as a launchd / systemd user / Task Scheduler service, or in the
foreground via `forged daemon` (same code path).

## Must know

- **One daemon per user.** PID file + socket peer checks enforce it; a
  second live daemon aborts at startup.
- **`FORGED_MASTER_PASSWORD` is stored plaintext** in the launchd plist /
  systemd unit. Top open security gap — see
  `architecture/security-model.md`; hardening plan rewrites this.
- **IF the env var is unset AND stdin is not a TTY AND nothing is piped
  THEN startup blocks on stdin read.** launchd/systemd MUST set the env var.
- **Decrypted keys stay in daemon memory for its whole lifetime** once
  unlocked. `sensitiveauth` lock gates IPC only, does not re-encrypt
  memory.
- **Windows is not fully supported** — no peer-PID, no control-socket
  transport (see `cli/ipc.md`).
- **macOS legacy label `me.ritik.forged`** is detected and migrated to
  `me.ritik.forged.daemon` on service start; old plist removed.
- **`just dev` does NOT run `forged daemon` directly.** It runs
  `forged-dev-service install`, reusing the normal `InstallService` path.
- IF the vault cannot open THEN daemon exits before writing PID or opening
  sockets.
- Stale socket/PID detection handles hard-kill recovery; clean shutdown
  zeros key buffers, `munlock`s, closes the vault.
- Activity log is a 1000-entry in-memory ring; file logging rotates
  (lumberjack: 10 MB × 3 backups, 30 days).

## Decisions

- Long-running, not on-demand. SSH agent must answer instantly with keys
  in memory; per-request spin-up re-derives the Symmetric Key and defeats
  mlock.
- Service install lives inside the CLI binary. No external installer, no
  package postinstall. TUI doctor drives install and repair.
- Agent and control sockets stay split (see `architecture/overview.md`).
